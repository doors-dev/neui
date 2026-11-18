package core

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/doors-dev/gox"
	"modernc.org/quickjs"
	"neui.dev/api"
)

type uiRun struct {
	Run
	vm *quickjs.VM
}

func (r *uiRun) TurnContext() context.Context { return context.Background() }
func (r *uiRun) Library() Library             { return NewLibrary() }
func (r *uiRun) End()                         { r.vm.Interrupt() }

func newUI(t *testing.T) *quickjs.VM {
	t.Helper()
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vm.Close() })
	s := &state{run: &uiRun{vm: vm}}
	if err := s.RegisterTools(vm); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.Eval(UIJS, quickjs.EvalGlobal); err != nil {
		t.Fatal(err)
	}
	return vm
}

func TestEndHandsNoControlBack(t *testing.T) {
	vm := newUI(t)
	var ran []string
	if err := vm.RegisterFunc("ran", func(where string) { ran = append(ran, where) }, false); err != nil {
		t.Fatal(err)
	}
	_, err := vm.Eval(`try { ui.end() } catch (e) { ran("catch") } finally { ran("finally") }
ran("after")`, quickjs.EvalGlobal)
	var qe *quickjs.Error
	if !errors.As(err, &qe) || qe.Message != "interrupted" {
		t.Fatalf("the script ended with %v, want it interrupted", err)
	}
	if len(ran) != 0 {
		t.Fatalf("code after ui.end() ran: %v", ran)
	}
}

func TestReservedNamesRejected(t *testing.T) {
	keys, err := newUI(t).Eval(`JSON.stringify(Object.keys(ui))`, quickjs.EvalGlobal)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal([]byte(keys.(string)), &names); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, "end") {
		t.Fatalf("ui has %v of its own, want end among them", names)
	}
	for _, name := range names {
		l := NewLibrary()
		for kind, err := range map[string]error{
			"view": l.View(name, "", func(context.Context, struct{}) (api.Widget[struct{}], error) { return nil, nil }),
			"control": l.Control(name, "", func(context.Context, struct{}, func(context.Context, struct{}) bool) (api.Widget[struct{}], error) {
				return nil, nil
			}),
			"layout": l.Layout(name, "", func(struct{}, []gox.Comp) gox.Comp { return nil }),
		} {
			if !errors.Is(err, errNameReserved) {
				t.Fatalf("a %s named %s registered with %v, want it rejected as reserved", kind, name, err)
			}
		}
	}
}

type drawRun struct {
	Run
	notes []string
}

func (r *drawRun) TurnContext() context.Context { return context.Background() }
func (r *drawRun) Library() Library             { return NewLibrary() }
func (r *drawRun) Note(note string) {
	r.notes = append(r.notes, note)
}

func drawUI(t *testing.T, drawn string, script string) (string, *drawRun) {
	t.Helper()
	run := &drawRun{}
	return drawUIOn(t, &state{run: run}, drawn, script), run
}

func drawUIOn(t *testing.T, s *state, drawn string, script string) string {
	t.Helper()
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vm.Close() })
	if err := s.RegisterTools(vm); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.Eval(`Object.defineProperty(globalThis, "__draw", {value: (ms) => (`+drawn+`), configurable: true})`, quickjs.EvalGlobal); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.Eval(UIJS, quickjs.EvalGlobal); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Eval(script, quickjs.EvalGlobal)
	if err != nil {
		t.Fatal(err)
	}
	return got.(string)
}

func TestDrawFallsBackToOutputWithOneNotePerRun(t *testing.T) {
	got, run := drawUI(t, `{id: 3, output: {row: 5, name: "a"}}`,
		`const ev = ui.draw(); const again = ui.draw(); JSON.stringify([ev.row, ev.row, ev.name, again.name, ev.missing === undefined, ev.id, ev.output.row])`)
	if want := `[5,5,"a","a",true,3,5]`; got != want {
		t.Fatalf("the reads gave %s, want %s", got, want)
	}
	if want := []string{"Your script read .row on what ui.draw() returned, but the control's fields are under .output. The script kept running and got .output.row instead. In future scripts, read .output.<field>. You won't get any more notifications of this type this run."}; !slices.Equal(run.notes, want) {
		t.Fatalf("notes %q, want %q", run.notes, want)
	}
}

func TestDrawNoteStaysSilentInLaterParts(t *testing.T) {
	run := &drawRun{}
	s := &state{run: run}
	drawUIOn(t, s, `{id: 3, output: {row: 5}}`, `String(ui.draw().row)`)
	if got := drawUIOn(t, s, `{id: 3, output: {name: "a"}}`, `String(ui.draw().name)`); got != "a" {
		t.Fatalf("a later part read %s, want a", got)
	}
	if len(run.notes) != 1 {
		t.Fatalf("notes %q, want only the first part's", run.notes)
	}
}

func TestDrawNoteNamesANonIdentifierKey(t *testing.T) {
	_, run := drawUI(t, `{id: 3, output: {"first name": "b"}}`, `String(ui.draw()["first name"])`)
	if want := []string{`Your script read ["first name"] on what ui.draw() returned, but the control's fields are under .output. The script kept running and got .output["first name"] instead. In future scripts, read .output.<field>. You won't get any more notifications of this type this run.`}; !slices.Equal(run.notes, want) {
		t.Fatalf("notes %q, want %q", run.notes, want)
	}
}

func TestDrawKeepsItsOwnKeys(t *testing.T) {
	got, run := drawUI(t, `{id: 3, output: {id: 9, output: "inner", toString: 1, row: 1}}`,
		`const ev = ui.draw(); JSON.stringify([ev.id, ev.output.id, ev.output.output, typeof ev.toString, ev.hasOwnProperty("row")])`)
	if want := `[3,9,"inner","function",false]`; got != want {
		t.Fatalf("the reads gave %s, want %s", got, want)
	}
	if len(run.notes) != 0 {
		t.Fatalf("own keys noted %q", run.notes)
	}
}

func TestDrawSerializesAsBefore(t *testing.T) {
	got, run := drawUI(t, `{id: 3, output: {row: 5}}`,
		`const ev = ui.draw(); JSON.stringify([JSON.stringify(ev), Object.keys(ev), "row" in ev, "id" in ev, {...ev}, Object.entries(ev)])`)
	if want := `["{\"id\":3,\"output\":{\"row\":5}}",["id","output"],false,true,{"id":3,"output":{"row":5}},[["id",3],["output",{"row":5}]]]`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if len(run.notes) != 0 {
		t.Fatalf("serializing noted %q", run.notes)
	}
}

func TestDrawGivesNoFallbackForANonObjectOutput(t *testing.T) {
	for _, output := range []string{`"text"`, `5`, `[1, 2]`, `null`, `true`} {
		got, run := drawUI(t, `{id: 3, output: `+output+`}`,
			`const ev = ui.draw(); JSON.stringify([ev.length, ev[0], ev.toFixed, ev.row].map(v => v === undefined))`)
		if want := `[true,true,true,true]`; got != want {
			t.Fatalf("output %s: undefined reads %s, want %s", output, got, want)
		}
		if len(run.notes) != 0 {
			t.Fatalf("output %s noted %q", output, run.notes)
		}
	}
}

func TestDrawDestructuringFallsBack(t *testing.T) {
	got, run := drawUI(t, `{id: 3, output: {row: 5}}`,
		`const {id, row, output, missing} = ui.draw(); JSON.stringify([id, row, output, missing === undefined])`)
	if want := `[3,5,{"row":5},true]`; got != want {
		t.Fatalf("destructuring gave %s, want %s", got, want)
	}
	if want := []string{"Your script read .row on what ui.draw() returned, but the control's fields are under .output. The script kept running and got .output.row instead. In future scripts, read .output.<field>. You won't get any more notifications of this type this run."}; !slices.Equal(run.notes, want) {
		t.Fatalf("notes %q, want %q", run.notes, want)
	}
}

func TestDrawTimeoutGivesNull(t *testing.T) {
	got, run := drawUI(t, `null`, `String(ui.draw(5))`)
	if got != "null" {
		t.Fatalf("a timed-out draw gave %s, want null", got)
	}
	if len(run.notes) != 0 {
		t.Fatalf("a timeout noted %q", run.notes)
	}
}
