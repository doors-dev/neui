package js

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js/task"
	"neui.dev/internal/testkit"
	"neui.dev/internal/tools"
)

func write(t *testing.T, fs api.FS, name, content string) {
	t.Helper()
	w, err := fs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, fs api.FS, name string) string {
	t.Helper()
	rc, err := fs.ReadAll(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRun(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "calc.js", "console.log('hey');\nreturn 21 * 2;")
	m := &module{runtime: getRuntime(context.Background())}
	res, err := m.run(ctx, RunArg{File: "calc.js"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "42" || res.Async || res.Log != "calc.js.log" {
		t.Fatalf("res = %+v", res)
	}
	log := read(t, fs, "calc.js.log")
	if !strings.Contains(log, "INFO hey") || strings.Count(log, "\n") != 1 {
		t.Fatalf("log = %q", log)
	}
	res, err = m.run(ctx, RunArg{File: "calc.js"})
	if err != nil {
		t.Fatal(err)
	}
	if log := read(t, fs, "calc.js.log"); strings.Count(log, "\n") != 1 {
		t.Fatalf("log not truncated: %q", log)
	}
	if _, err := m.run(ctx, RunArg{File: "calc.js", AppendLogs: true}); err != nil {
		t.Fatal(err)
	}
	if log := read(t, fs, "calc.js.log"); strings.Count(log, "\n") != 2 {
		t.Fatalf("log not appended: %q", log)
	}
}

func TestRunThrow(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "bad.js", `throw new Error("boom");`)
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{File: "bad.js"}); !errors.Is(err, errScript) || !strings.Contains(err.Error(), `log file "bad.js.log"`) {
		t.Fatalf("err = %v", err)
	}
	if log := read(t, fs, "bad.js.log"); !strings.Contains(log, "boom") {
		t.Fatalf("log = %q", log)
	}
}

func TestRunTimeout(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "spin.js", "while (true) {}")
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{File: "spin.js", Timeout: 200}); !errors.Is(err, task.ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTimeoutCancelsTool(t *testing.T) {
	seen := make(chan error, 1)
	wait := tools.NewModule("wait", "")
	if _, err := wait.Add("block", "", func(ctx context.Context, _ struct{}) (struct{}, error) {
		<-ctx.Done()
		seen <- context.Cause(ctx)
		return struct{}{}, context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	ctx := testkit.NewController(t, testkit.Setup{Modules: []common.Module{wait}}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	start := time.Now()
	_, err := m.run(ctx, RunArg{Code: "try { wait__block({}); } catch (e) {}\nwhile (true) {}", Timeout: 200})
	if !errors.Is(err, task.ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timed out run took %v", elapsed)
	}
	if cause := <-seen; !errors.Is(cause, task.ErrTimeout) {
		t.Fatalf("tool ctx cause = %v", cause)
	}
}

func TestRunToolErrorReachesScriptInFull(t *testing.T) {
	long := strings.Repeat("x", 300) + "END"
	fail := tools.NewModule("fail", "")
	if _, err := fail.Add("long", "", func(ctx context.Context, _ struct{}) (struct{}, error) {
		return struct{}{}, errors.New(long)
	}); err != nil {
		t.Fatal(err)
	}
	ctx := testkit.NewController(t, testkit.Setup{Modules: []common.Module{fail}}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	res, err := m.run(ctx, RunArg{Code: "try { fail__long({}); } catch (e) { return [e instanceof TypeError, e.message]; }"})
	if err != nil {
		t.Fatal(err)
	}
	var got []any
	if err := json.Unmarshal(res.Output, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != true || got[1] != "long: "+long {
		t.Fatalf("output = %s", res.Output)
	}
}

func TestRunMissing(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{File: "ghost.js"}); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStopIdle(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.stop(ctx, struct{}{}); err == nil {
		t.Fatal("stop with nothing running succeeded")
	}
}

func TestRunWaitsForMemory(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	spin := "const end = Date.now() + 300;\nwhile (Date.now() < end) {}\nreturn 1;"
	write(t, fs, "a.js", spin)
	write(t, fs, "b.js", spin)
	for _, c := range []struct {
		total  int
		serial bool
	}{{4 << 20, true}, {8 << 20, false}} {
		seed := NewSeed(Config{DefaultMemory: 4 << 20, WorkerMemory: 4 << 20, TotalMemory: c.total})
		rt := getRuntime(context.WithValue(context.Background(), ConfigKey{}, seed))
		start := time.Now()
		errs := make(chan error, 2)
		for _, name := range []string{"a.js", "b.js"} {
			go func() {
				m := &module{runtime: rt}
				_, err := m.run(ctx, RunArg{File: name, Memory: 100})
				errs <- err
			}()
		}
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}
		elapsed := time.Since(start)
		if c.serial && elapsed < 550*time.Millisecond || !c.serial && elapsed >= 500*time.Millisecond {
			t.Fatalf("total %d MiB: both runs took %v", c.total>>20, elapsed)
		}
	}
}

func TestRunArg(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "echo.js", "return {arg, kind: typeof arg};")
	m := &module{runtime: getRuntime(context.Background())}
	for _, c := range []struct {
		arg  string
		want string
	}{
		{"", `{"arg":null,"kind":"object"}`},
		{`{"a":[1,"x y"]}`, `{"arg":{"a":[1,"x y"]},"kind":"object"}`},
		{`"hi"`, `{"arg":"hi","kind":"string"}`},
	} {
		res, err := m.run(ctx, RunArg{File: "echo.js", Arg: json.RawMessage(c.arg)})
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal(res.Output, &got); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal([]byte(c.want), &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("arg %s: output = %s", c.arg, res.Output)
		}
	}
}

func TestRunSkipsCanceledQueued(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "spin.js", "const end = Date.now() + 1000;\nwhile (Date.now() < end) {}\nreturn 1;")
	write(t, fs, "quick.js", "return 2;")
	seed := NewSeed(Config{DefaultMemory: 4 << 20, WorkerMemory: 8 << 20, TotalMemory: 8 << 20})
	rt := getRuntime(context.WithValue(context.Background(), ConfigKey{}, seed))
	spinner := &module{runtime: rt}
	spun := make(chan error, 1)
	go func() {
		_, err := spinner.run(ctx, RunArg{File: "spin.js"})
		spun <- err
	}()
	time.Sleep(100 * time.Millisecond)
	big := &module{runtime: rt}
	stopped := make(chan error, 1)
	go func() {
		_, err := big.run(ctx, RunArg{File: "quick.js", Memory: 8})
		stopped <- err
	}()
	time.Sleep(100 * time.Millisecond)
	if _, err := big.stop(ctx, struct{}{}); err != nil {
		t.Fatal(err)
	}
	if err := <-stopped; err == nil || !strings.Contains(err.Error(), "stopped before finishing") {
		t.Fatalf("canceled run err = %v", err)
	}
	if big.running != nil {
		t.Fatal("canceled run still marked running")
	}
	small := &module{runtime: rt}
	res, err := small.run(ctx, RunArg{File: "quick.js"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "2" {
		t.Fatalf("res = %+v", res)
	}
	select {
	case err := <-spun:
		t.Fatalf("small run waited for the spinner to finish: %v", err)
	default:
	}
	if err := <-spun; err != nil {
		t.Fatal(err)
	}
}

func TestRunCode(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	m := &module{runtime: getRuntime(context.Background())}
	res, err := m.run(ctx, RunArg{Code: "console.log('hey');\nreturn 21 * 2;"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "42" || res.Log != "inline.log" {
		t.Fatalf("res = %+v", res)
	}
	if got := read(t, fs, "inline.log"); !strings.Contains(got, "INFO hey") {
		t.Fatalf("log = %q", got)
	}
	res, err = m.run(ctx, RunArg{Code: "return arg.n + 1;", Arg: json.RawMessage(`{"n":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "2" {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunFileXorCode(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "calc.js", "return 1;")
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{File: "calc.js", Code: "return 1;"}); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("both set: err = %v", err)
	}
	if _, err := m.run(ctx, RunArg{}); err == nil || !strings.Contains(err.Error(), "neither") {
		t.Fatalf("neither set: err = %v", err)
	}
}

func TestRunBackToBackNotBusy(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	for range 200 {
		if _, err := m.run(ctx, RunArg{Code: "return 1;"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunCanceledCall(t *testing.T) {
	base := testkit.NewController(t, testkit.Setup{}).Ctx()
	ctx, cancel := context.WithCancelCause(base)
	m := &module{runtime: getRuntime(context.Background())}
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel(api.ErrCanceledByUser)
	}()
	start := time.Now()
	_, err := m.run(ctx, RunArg{Code: "while (true) {}"})
	if !errors.Is(err, api.ErrCanceledByUser) {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("canceled run took %v", elapsed)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running != nil {
		t.Fatal("canceled run still marked running")
	}
}

func TestRunUnserializableResult(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	for _, code := range []string{"return () => 1;", "return Symbol('x');", "return {toJSON() {}};"} {
		_, err := m.run(ctx, RunArg{Code: code})
		if !errors.Is(err, errScript) || errors.Is(err, errRuntime) {
			t.Fatalf("%s: err = %v", code, err)
		}
	}
}

func TestRunArgProtoKey(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	res, err := m.run(ctx, RunArg{Code: "return Object.keys(arg);", Arg: json.RawMessage(`{"__proto__":{"x":1},"y":2}`)})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != `["__proto__","y"]` {
		t.Fatalf("output = %s", res.Output)
	}
}

func TestConfigNonPositiveTakesDefaults(t *testing.T) {
	got := Config{-1, -1, -1, -1, -1, -1, -1, -1}
	got.initDefaults()
	want := Config{}
	want.initDefaults()
	if got != want {
		t.Fatalf("config = %+v, want %+v", got, want)
	}
}

func TestRunHugeLimitsApplyTheLimit(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background())}
	res, err := m.run(ctx, RunArg{
		Code:    "const end = Date.now() + 50;\nwhile (Date.now() < end) {}\nreturn 1;",
		Timeout: 18446744073710,
		Memory:  17592186044417,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "1" {
		t.Fatalf("res = %+v", res)
	}
}

func TestExecuteSetupTimeout(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(task.ErrTimeout)
	rt := getRuntime(context.Background())
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Close()
	_, err = rt.execute(vm, task.Input{
		Params:  task.Params{Name: "../escape", Script: "return 1;", Controller: c},
		Context: ctx,
	})
	if !errors.Is(err, task.ErrTimeout) || errors.Is(err, errRuntime) {
		t.Fatalf("err = %v", err)
	}
}

func TestTaskNameAfterCancel(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	done := make(chan struct{})
	w, _, cancel := task.NewTask(task.Params{Name: "a.js", Controller: c}, func() { close(done) }, time.Hour)
	cancel(errStopped)
	for {
		if w.Name() != "a.js" {
			t.Fatalf("name = %q", w.Name())
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func TestConsoleLogsUnserializableValues(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{Code: "console.log(undefined, Symbol('s'), 1); return null;"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, fs, "inline.log"); !strings.Contains(got, "INFO undefined Symbol(s) 1\n") {
		t.Fatalf("log = %q", got)
	}
}

func TestRunRefusesAnOversizedScript(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "big.js", "return 1;\n//"+strings.Repeat("x", scriptLimit))
	m := &module{runtime: getRuntime(context.Background())}
	if _, err := m.run(ctx, RunArg{File: "big.js"}); err == nil || !strings.Contains(err.Error(), "4 MiB script size limit") {
		t.Fatalf("err = %v", err)
	}
	write(t, fs, "ok.js", "return 1;\n//"+strings.Repeat("x", scriptLimit-100))
	if _, err := m.run(ctx, RunArg{File: "ok.js"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesWhileRunningBeforeReading(t *testing.T) {
	ctx := testkit.NewController(t, testkit.Setup{}).Ctx()
	m := &module{runtime: getRuntime(context.Background()), running: &running{name: "other.js"}}
	if _, err := m.run(ctx, RunArg{File: "missing.js"}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("err = %v", err)
	}
}

func TestLogKeepsTheNewestMegabyte(t *testing.T) {
	fs := testkit.NewController(t, testkit.Setup{}).FS()
	entry := strings.Repeat("x", 100<<10)
	for i := range 25 {
		if err := Log(fs, "a.log", "INFO", entry+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	got := read(t, fs, "a.log")
	if len(got) > logCap {
		t.Fatalf("log is %d bytes", len(got))
	}
	first, _, _ := strings.Cut(got, "\n")
	if !strings.HasSuffix(first, "INFO earlier output dropped") {
		t.Fatalf("first line %q", first)
	}
	if !strings.HasSuffix(got, "xy\n") || strings.Count(got, "\n") < 6 {
		t.Fatalf("tail lost: %d lines", strings.Count(got, "\n"))
	}
}

func TestClearLogQuotaIsNotARuntimeFailure(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{MaxFiles: 1}})
	ctx, fs := c.Ctx(), c.FS()
	write(t, fs, "a.js", "return 1;")
	m := &module{runtime: getRuntime(context.Background())}
	_, err := m.run(ctx, RunArg{File: "a.js"})
	if !errors.Is(err, api.ErrQuota) || errors.Is(err, errRuntime) {
		t.Fatalf("err = %v", err)
	}
}
