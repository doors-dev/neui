package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/schema"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/tools"
)

type callTestTask struct {
	mu      sync.Mutex
	notices []map[string]any
}

func (t *callTestTask) ID() uint           { return 1 }
func (t *callTestTask) Ok(json.RawMessage) {}
func (t *callTestTask) Err(error)          {}
func (t *callTestTask) Notify(data json.RawMessage, meta map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	meta["data"] = string(data)
	t.notices = append(t.notices, meta)
}

func (t *callTestTask) list() []map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]map[string]any(nil), t.notices...)
}

func (t *callTestTask) wait(tb testing.TB, n int) []map[string]any {
	tb.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(t.list()) < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)
	notices := t.list()
	if len(notices) != n {
		tb.Fatalf("want %d notices, got %d: %v", n, len(notices), notices)
	}
	return notices
}

type callTestRow struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type callTestParams struct {
	Total Call[callTestRow, float64] `json:"total" description:"the row's total"`
}

func newCallTestRun(t *testing.T, config api.IslandConfig, lib Library) (*run, *callTestTask, context.Context) {
	t.Helper()
	storage, err := local.NewFileStorage()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Remove() })
	ws, err := storage.WorkspaceOpen("thread")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	fs := filesystem.New("thread", ws, storage, api.FilesConfig{})
	ctx, cancel := context.WithCancelCause(context.Background())
	task := &callTestTask{}
	r := &run{
		island: &island{lib: lib},
		config: common.IslandConfigDefaults(config),
		fs:     fs,
		log:    logFile("app"),
		task:   task,
		ctx:    ctx,
		cancel: cancel,
		store:  doors.NewStore(),
	}
	r.runner = newRunner(r)
	r.store.Save(runnerKey{}, r.runner)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		r.runner.run()
	}()
	t.Cleanup(func() {
		cancel(context.Canceled)
		<-stopped
	})
	return r, task, context.WithValue(context.Background(), common.KeyIslandStore, r.store)
}

func callFrom[I, O any](t *testing.T, src string) Call[I, O] {
	t.Helper()
	var c Call[I, O]
	data, _ := json.Marshal(src)
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func readLog(t *testing.T, r *run) string {
	t.Helper()
	rc, err := r.fs.ReadAll(context.Background(), r.log)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCallSchema(t *testing.T) {
	s, err := schema.NewSchema[callTestParams]()
	if err != nil {
		t.Fatal(err)
	}
	raw := string(s.Raw())
	if strings.Contains(raw, "$ref") || strings.Contains(raw, "definitions") || strings.Contains(raw, "$defs") {
		t.Fatalf("not inlined: %s", raw)
	}
	var doc struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(s.Raw(), &doc); err != nil {
		t.Fatal(err)
	}
	total := doc.Properties["total"]
	in, _ := schema.NewSchema[callTestRow]()
	out, _ := schema.NewSchema[float64]()
	want := "function as JS source: (arg: " + string(in.Raw()) + ") => " + string(out.Raw())
	if total.Type != "string" || total.Title != want || total.Description != "the row's total" {
		t.Fatalf("total: %+v\nraw: %s", total, raw)
	}
	if !strings.Contains(total.Title, `"sku"`) || !strings.Contains(total.Title, `"number"`) {
		t.Fatalf("title: %s", total.Title)
	}
}

func TestCallDecode(t *testing.T) {
	s, err := schema.NewSchema[callTestParams]()
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.DecodeString(`{"total":"(row) => row.qty * 2"}`)
	if err != nil || p.Total != "(row) => row.qty * 2" {
		t.Fatalf("valid: %q, %v", p.Total, err)
	}
	var direct callTestParams
	if err := json.Unmarshal([]byte(`{"total":"function (row) { return row.qty }"}`), &direct); err != nil {
		t.Fatalf("function expression: %v", err)
	}
	for _, src := range []string{"(row) => {", "row) => row", "1; 2", "(row) => { with (row) { return qty } }", "(row) => 010"} {
		data, _ := json.Marshal(map[string]string{"total": src})
		if _, err := s.Decode(data); err == nil || !strings.Contains(err.Error(), "line ") {
			t.Fatalf("%q: %v", src, err)
		}
	}
	if _, err := s.DecodeString(`{"total":5}`); err == nil {
		t.Fatal("number accepted as source")
	}
}

func TestCallSuccess(t *testing.T) {
	lib := NewLibrary()
	var seen context.Context
	if err := fetch(lib, "price", func(ctx context.Context, sku string) (float64, error) {
		seen = ctx
		if sku != "a" {
			return 0, fmt.Errorf("unknown sku %s", sku)
		}
		return 2.5, nil
	}); err != nil {
		t.Fatal(err)
	}
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	total := callFrom[callTestRow, float64](t, "(row) => { console.log('row', row.sku); return app.price(row.sku) * row.qty }")
	for range 2 {
		got, err := total.Call(ctx, callTestRow{SKU: "a", Qty: 4})
		if err != nil || got != 10 {
			t.Fatalf("got %v, %v", got, err)
		}
	}
	if seen == nil {
		t.Fatal("app function not called")
	}
	if _, ok := seen.Deadline(); !ok {
		t.Fatal("app function ctx has no FnTime deadline")
	}
	if seen.Value(common.KeyIslandStore) != r.store {
		t.Fatal("app function ctx is not the caller's")
	}
	nothing := callFrom[int, *int](t, "(a) => undefined")
	if got, err := nothing.Call(ctx, 1); err != nil || got != nil {
		t.Fatalf("undefined: %v, %v", got, err)
	}
	isolated := callFrom[int, string](t, "(a) => typeof ui + typeof __log + typeof app.price")
	if got, err := isolated.Call(ctx, 1); err != nil || got != "undefinedundefinedfunction" {
		t.Fatalf("globals: %q, %v", got, err)
	}
	if log := readLog(t, r); !strings.Contains(log, "INFO row a") {
		t.Fatalf("log: %s", log)
	}
	task.wait(t, 0)
}

func TestCallThrow(t *testing.T) {
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, NewLibrary())
	throws := callFrom[int, int](t, "(a) => { throw new Error('boom ' + a) }")
	for range 3 {
		if _, err := throws.Call(ctx, 7); err == nil || !strings.Contains(err.Error(), "boom 7") {
			t.Fatalf("throw: %v", err)
		}
	}
	notices := task.wait(t, 1)
	var data map[string]string
	if err := json.Unmarshal([]byte(notices[0]["data"].(string)), &data); err != nil {
		t.Fatal(err)
	}
	if notices[0]["kind"] != "call_error" || !strings.Contains(data["error"], "boom 7") || data["src"] != string(throws) {
		t.Fatalf("notice: %v", notices[0])
	}
	if log := readLog(t, r); strings.Count(log, "ERROR function: ") != 3 || !strings.Contains(log, "boom 7") {
		t.Fatalf("log: %s", log)
	}

	r.runner.reset()
	if _, err := throws.Call(ctx, 7); err == nil {
		t.Fatal("no error after reset")
	}
	task.wait(t, 2)

	r.runner.reset()
	for i := range fnReportsCap + 4 {
		c := callFrom[int, int](t, fmt.Sprintf("(a) => { throw new Error('cap %d') }", i))
		if _, err := c.Call(ctx, 1); err == nil {
			t.Fatalf("cap %d: no error", i)
		}
	}
	task.wait(t, 2+fnReportsCap)
	r.runner.reset()
	if _, err := throws.Call(ctx, 1); err == nil {
		t.Fatal("no error after cap reset")
	}
	task.wait(t, 3+fnReportsCap)
}

func TestCallFailures(t *testing.T) {
	_, task, ctx := newCallTestRun(t, api.IslandConfig{FnTime: 200 * time.Millisecond}, NewLibrary())
	cases := []struct {
		src  string
		want error
		text string
	}{
		{src: "(a) => { while (true) {} }", want: errFnTime},
		{src: "42", text: "the source is not a function"},
		{src: "(a) => () => 1", want: errFnResult},
		{src: "(a) => Symbol('s')", want: errFnResult},
		{src: "(a) => 'text'", text: "schema"},
		{src: "(a) => null"},
	}
	for _, c := range cases {
		call := callFrom[int, int](t, c.src)
		start := time.Now()
		_, err := call.Call(ctx, 1)
		if err == nil {
			t.Fatalf("%s: no error", c.src)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Fatalf("%s: want %v, got %v", c.src, c.want, err)
		}
		if c.text != "" && !strings.Contains(err.Error(), c.text) {
			t.Fatalf("%s: want %q in %v", c.src, c.text, err)
		}
		if time.Since(start) > 2*time.Second {
			t.Fatalf("%s: took %s", c.src, time.Since(start))
		}
	}
	notices := task.wait(t, len(cases))
	for _, n := range notices {
		if n["kind"] != "call_error" {
			t.Fatalf("notice: %v", n)
		}
	}
}

func TestCallAppFunctionTime(t *testing.T) {
	lib := NewLibrary()
	if err := fetch(lib, "wait", func(ctx context.Context, _ struct{}) (int, error) {
		<-ctx.Done()
		return 0, context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	_, task, ctx := newCallTestRun(t, api.IslandConfig{FnTime: 200 * time.Millisecond}, lib)
	call := callFrom[int, int](t, "(a) => app.wait()")
	start := time.Now()
	if _, err := call.Call(ctx, 1); err == nil {
		t.Fatal("no error")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("took %s", took)
	}
	task.wait(t, 1)
}

func TestCallCallerCancel(t *testing.T) {
	lib := NewLibrary()
	started := make(chan struct{}, 1)
	if err := fetch(lib, "wait", func(ctx context.Context, _ struct{}) (int, error) {
		started <- struct{}{}
		<-ctx.Done()
		return 0, context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	_, task, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	call := callFrom[int, int](t, "(a) => app.wait()")

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := call.Call(canceled, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}

	mid, cancel := context.WithCancel(ctx)
	go func() {
		<-started
		cancel()
	}()
	if _, err := call.Call(mid, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid: %v", err)
	}

	quick := callFrom[int, int](t, "(a) => a + 1")
	if got, err := quick.Call(ctx, 1); err != nil || got != 2 {
		t.Fatalf("after cancel: %v, %v", got, err)
	}
	task.wait(t, 0)
}

func TestCallCallerCancelBusyLoop(t *testing.T) {
	_, task, ctx := newCallTestRun(t, api.IslandConfig{FnTime: 10 * time.Second}, NewLibrary())
	loop := callFrom[int, int](t, "(a) => { for (;;) {} }")
	mid, cancel := context.WithCancel(ctx)
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	if _, err := loop.Call(mid, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid: %v", err)
	}
	quick := callFrom[int, int](t, "(a) => a + 1")
	if got, err := quick.Call(ctx, 1); err != nil || got != 2 {
		t.Fatalf("after cancel: %v, %v", got, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("runner stayed busy for %s after cancel", elapsed)
	}
	task.wait(t, 0)
}

func TestCallAppEnded(t *testing.T) {
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, NewLibrary())
	slow := callFrom[int, int](t, "(a) => { while (true) {} }")
	done := make(chan error, 1)
	go func() {
		_, err := slow.Call(ctx, 1)
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	r.cancel(errStopped)
	select {
	case err := <-done:
		if !errors.Is(err, errStopped) {
			t.Fatalf("ended: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("call outlived its app")
	}
	quick := callFrom[int, int](t, "(a) => a")
	if _, err := quick.Call(ctx, 1); !errors.Is(err, errStopped) {
		t.Fatalf("after end: %v", err)
	}
	task.wait(t, 0)
}

func TestCallAppGlobalsHidden(t *testing.T) {
	lib := NewLibrary()
	if err := fetch(lib, "price", func(context.Context, string) (float64, error) { return 1, nil }); err != nil {
		t.Fatal(err)
	}
	_, _, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	call := callFrom[int, string](t, "(a) => typeof __fetches + typeof __fetch__price")
	if got, err := call.Call(ctx, 1); err != nil || got != "undefinedundefined" {
		t.Fatalf("globals: %q, %v", got, err)
	}
}

func TestCallNoStore(t *testing.T) {
	call := callFrom[int, int](t, "(a) => a")
	if _, err := call.Call(context.Background(), 1); err == nil {
		t.Fatal("no error without a store")
	}
}

func TestCallIsolation(t *testing.T) {
	r, _, ctx := newCallTestRun(t, api.IslandConfig{}, NewLibrary())
	leaks := []string{
		"(a) => { globalThis.leak = a; return a }",
		"(a) => { Array.prototype.leak = a; return a }",
		"(a) => { JSON.stringify = () => '0'; return a }",
	}
	for _, src := range leaks {
		if _, err := callFrom[int, int](t, src).Call(ctx, 1); err == nil {
			t.Fatalf("%s: no error", src)
		}
	}
	check := callFrom[int, string](t, "(a) => typeof globalThis.leak + ' ' + typeof [].leak + ' ' + JSON.stringify(a)")
	got, err := check.Call(ctx, 7)
	if err != nil || got != "undefined undefined 7" {
		t.Fatalf("got %q, %v", got, err)
	}
	vm := r.runner.vm
	if _, err := callFrom[int, int](t, "(a) => a + 1").Call(ctx, 1); err != nil || r.runner.vm != vm {
		t.Fatalf("vm not reused: %v", err)
	}
}

func fetch[I, O any](lib Library, name string, f func(ctx context.Context, input I) (O, error)) error {
	tool, err := tools.NewTool(name, "", f)
	if err != nil {
		return err
	}
	return lib.Fetch(tool)
}
