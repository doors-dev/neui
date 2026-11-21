// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/testkit"
	"neui.dev/internal/tools"
)

type callTestRow struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type callTestParams struct {
	Total Call[callTestRow, float64] `json:"total" description:"the row's total"`
}

func newCallTestRun(t *testing.T, config api.IslandConfig, lib Library) (*run, *testkit.Task, context.Context) {
	t.Helper()
	c := testkit.NewController(t, testkit.Setup{})
	if _, err := c.Task("", nil, nil); err != nil {
		t.Fatal(err)
	}
	task := c.Tasks()[0]
	ctx, cancel := context.WithCancelCause(context.Background())
	r := &run{
		island: &island{lib: lib},
		config: common.IslandConfigDefaults(config),
		fs:     c.FS(),
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

func reported(r *run) int {
	r.runner.mu.Lock()
	defer r.runner.mu.Unlock()
	return len(r.runner.reported)
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
	if strings.Contains(raw, "$ref") || strings.Contains(raw, "x-neui-call") || strings.Contains(raw, "allOf") || strings.Contains(raw, "function") {
		t.Fatalf("call schema in raw: %s", raw)
	}
	var doc struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(s.Raw(), &doc); err != nil {
		t.Fatal(err)
	}
	total := doc.Properties["total"]
	if len(total) != 2 || total["type"] != "string" || total["description"] != "the row's total" {
		t.Fatalf("total: %v\nraw: %s", total, raw)
	}
}

type callTestStep struct {
	Search Call[string, []callTestRow] `json:"search" description:"finds rows"`
}

type callTestSeries struct {
	Steps []Call[any, callTestStep]  `json:"steps"`
	Total Call[callTestRow, float64] `json:"total" description:"the row's total"`
}

func TestCallSpec(t *testing.T) {
	s, err := schema.NewSchema[callTestSeries]()
	if err != nil {
		t.Fatal(err)
	}
	spec, defs := s.Root(), s.Defs()
	type function struct {
		Arg     json.RawMessage `json:"arg"`
		Returns json.RawMessage `json:"returns"`
	}
	type call struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Function    function `json:"function"`
	}
	var doc struct {
		Properties struct {
			Steps struct {
				Items call `json:"items"`
			} `json:"steps"`
			Total call `json:"total"`
		} `json:"properties"`
		Definitions json.RawMessage `json:"definitions"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	total := doc.Properties.Total
	if doc.Definitions != nil || total.Type != "string" || total.Description != "the row's total" ||
		string(total.Function.Arg) != `{"$ref":"#/definitions/IslandCallTestRow"}` || string(total.Function.Returns) != `{"type":"number"}` {
		t.Fatalf("total: %s", spec)
	}
	step := doc.Properties.Steps.Items
	if step.Type != "string" || string(step.Function.Arg) != `{"description":"any JSON value"}` || string(step.Function.Returns) != `{"$ref":"#/definitions/IslandCallTestStep"}` {
		t.Fatalf("steps: %s", spec)
	}
	var stepDef struct {
		Properties struct {
			Search call `json:"search"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(defs["IslandCallTestStep"], &stepDef); err != nil {
		t.Fatal(err)
	}
	search := stepDef.Properties.Search
	if search.Description != "finds rows" || string(search.Function.Arg) != `{"type":"string"}` || string(search.Function.Returns) != `{"items":{"$ref":"#/definitions/IslandCallTestRow"},"type":"array"}` {
		t.Fatalf("search: %s", defs["IslandCallTestStep"])
	}
	if len(defs) != 2 || !strings.Contains(string(defs["IslandCallTestRow"]), `"sku"`) {
		t.Fatalf("defs: %v", defs)
	}
	if strings.Contains(string(s.Raw()), "function") {
		t.Fatalf("validation schema: %s", s.Raw())
	}
	if _, err := s.DecodeString(`{"steps":["() => ({search: (q) => []})"],"total":"(row) => row.qty"}`); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if _, err := s.DecodeString(`{"steps":[{}],"total":"(row) => row.qty"}`); err == nil {
		t.Fatal("object accepted as source")
	}
}

type callTestPointers struct {
	List  []*Call[int, int]          `json:"list"`
	Named map[string]*Call[int, int] `json:"named"`
}

func TestCallSpecNilPointer(t *testing.T) {
	s, err := schema.NewSchema[callTestPointers]()
	if err != nil {
		t.Fatal(err)
	}
	if spec := s.Root(); strings.Count(string(spec), `"function":{"arg":{"type":"integer"},"returns":{"type":"integer"}}`) != 2 {
		t.Fatalf("spec: %s", spec)
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
	task.WaitNotes(t, 0)
	if n := reported(r); n != 0 {
		t.Fatalf("%d reports for calls that did not fail by their function", n)
	}
}

func TestCallThrow(t *testing.T) {
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, NewLibrary())
	throws := callFrom[int, int](t, "(a) => { throw new Error('boom ' + a) }")
	for range 3 {
		if _, err := throws.Call(ctx, 7); err == nil || !strings.Contains(err.Error(), "boom 7") {
			t.Fatalf("throw: %v", err)
		}
	}
	notes := task.WaitNotes(t, 1)
	if n := reported(r); n != 1 {
		t.Fatalf("%d reports for one repeated failure", n)
	}
	var data map[string]string
	if err := json.Unmarshal(notes[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if notes[0].Meta["kind"] != "call_error" || notes[0].Meta["note"] != callErrorNote || !strings.Contains(data["error"], "boom 7") || data["src"] != string(throws) {
		t.Fatalf("note: %v", notes[0])
	}
	if log := readLog(t, r); strings.Count(log, "ERROR function: ") != 3 || !strings.Contains(log, "boom 7") {
		t.Fatalf("log: %s", log)
	}

	r.runner.reset()
	if _, err := throws.Call(ctx, 7); err == nil {
		t.Fatal("no error after reset")
	}
	task.WaitNotes(t, 2)

	r.runner.reset()
	for i := range fnReportsCap + 4 {
		c := callFrom[int, int](t, fmt.Sprintf("(a) => { throw new Error('cap %d') }", i))
		if _, err := c.Call(ctx, 1); err == nil {
			t.Fatalf("cap %d: no error", i)
		}
	}
	task.WaitNotes(t, 2+fnReportsCap)
	if n := reported(r); n != fnReportsCap {
		t.Fatalf("%d reports past the cap of %d", n, fnReportsCap)
	}
	r.runner.reset()
	if _, err := throws.Call(ctx, 1); err == nil {
		t.Fatal("no error after cap reset")
	}
	task.WaitNotes(t, 3+fnReportsCap)
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
	for _, n := range task.WaitNotes(t, len(cases)) {
		if n.Meta["kind"] != "call_error" {
			t.Fatalf("note: %v", n)
		}
	}
}

func TestCallAppErrorReachesFunctionInFull(t *testing.T) {
	long := strings.Repeat("x", 300) + "END"
	lib := NewLibrary()
	if err := fetch(lib, "fail", func(ctx context.Context, _ struct{}) (int, error) {
		return 0, errors.New(long)
	}); err != nil {
		t.Fatal(err)
	}
	_, _, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	seen := callFrom[int, string](t, `(a) => { try { app.fail({}) } catch (e) { return e.name + ' ' + e.message + ' ' + ("value" in e) } }`)
	if got, err := seen.Call(ctx, 1); err != nil || got != "TypeError "+long+" false" {
		t.Fatalf("got %q, %v", got, err)
	}
}

type imageError struct{ api.Image }

func (imageError) Error() string { return "render failed" }

type blocksError []api.UserContent

func (blocksError) Error() string { return "render failed" }

func (b blocksError) Contents() []api.UserContent { return b }

func TestCallAppContentError(t *testing.T) {
	lib := NewLibrary()
	if err := fetch(lib, "image", func(ctx context.Context, _ struct{}) (int, error) {
		return 0, imageError{api.Image{Data: []byte{1, 2}, Mime: api.ImagePNG}}
	}); err != nil {
		t.Fatal(err)
	}
	if err := fetch(lib, "blocks", func(ctx context.Context, _ struct{}) (int, error) {
		return 0, blocksError{api.ContentText{Text: "a"}}
	}); err != nil {
		t.Fatal(err)
	}
	_, _, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	seen := callFrom[int, string](t, "(a) => JSON.stringify([app.image, app.blocks].map(f => { try { f({}) } catch (e) { return [e.name, e.message, e.stack.length > 0, e.value] } }))")
	if got, err := seen.Call(ctx, 1); err != nil || got != `[["TypeError","render failed",true,{"data":"AQI=","mime":"image/png"}],["TypeError","render failed",true,[{"type":"text","text":"a"}]]]` {
		t.Fatalf("got %q, %v", got, err)
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
	task.WaitNotes(t, 1)
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
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
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
	task.WaitNotes(t, 0)
	if n := reported(r); n != 0 {
		t.Fatalf("%d reports for calls that did not fail by their function", n)
	}
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
	task.WaitNotes(t, 0)
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
	task.WaitNotes(t, 0)
	if n := reported(r); n != 0 {
		t.Fatalf("%d reports for calls that did not fail by their function", n)
	}
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

func TestCallCannotLeaveStateOnApp(t *testing.T) {
	lib := NewLibrary()
	if err := fetch(lib, "price", func(ctx context.Context, sku string) (float64, error) { return 1, nil }); err != nil {
		t.Fatal(err)
	}
	_, _, ctx := newCallTestRun(t, api.IslandConfig{}, lib)
	if _, err := callFrom[int, int](t, "(a) => { app.price.leak = a; return a }").Call(ctx, 1); err == nil {
		t.Fatal("a property set on an app function was accepted")
	}
	got, err := callFrom[int, string](t, "(a) => typeof app.price.leak").Call(ctx, 1)
	if err != nil || got != "undefined" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func fetch[I, O any](lib Library, name string, f func(ctx context.Context, input I) (O, error)) error {
	tool, err := tools.NewTool(name, "", f, false)
	if err != nil {
		return err
	}
	return lib.Fetch(tool)
}

func TestReportWidget(t *testing.T) {
	r, task, ctx := newCallTestRun(t, api.IslandConfig{}, NewLibrary())
	form := context.WithValue(ctx, common.KeyIslandWidget, common.IslandWidget{Name: "form", ID: 3})
	Report(form, errors.New("query failed"))
	Report(form, errors.New("query failed"))
	Report(context.WithValue(ctx, common.KeyIslandWidget, common.IslandWidget{Name: "table", ID: 4}), errors.New("query failed"))
	Report(context.Background(), errors.New("no island"))
	notes := task.WaitNotes(t, 2)
	widgets := map[string]map[string]any{}
	for _, n := range notes {
		var data map[string]any
		if err := json.Unmarshal(n.Data, &data); err != nil {
			t.Fatal(err)
		}
		if n.Meta["kind"] != "widget_error" || n.Meta["note"] != widgetErrorNote || data["error"] != "query failed" {
			t.Fatalf("note: %v %v", n.Meta, data)
		}
		widgets[fmt.Sprint(data["widget"])] = data
	}
	if form, table := widgets["form"], widgets["table"]; form["id"] != float64(3) || table["id"] != float64(4) {
		t.Fatalf("the notes name %v, want form #3 and table #4", widgets)
	}
	if n := reported(r); n != 2 {
		t.Fatalf("%d reports, want 2", n)
	}
	if log := readLog(t, r); strings.Count(log, "ERROR widget ") != 3 || !strings.Contains(log, "widget form #3: query failed") {
		t.Fatalf("log: %s", log)
	}
}
