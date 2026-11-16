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
