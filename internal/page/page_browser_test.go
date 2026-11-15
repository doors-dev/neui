//go:build browser

package page_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type readOutput struct {
	Value any
	Label string
}

type hookOutput struct {
	Hooks int
	report
}

func order(changes chan<- string, clicks chan<- struct{}) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Space{Name: "order", Desc: "the order"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := (neui.Input{Desc: "how many"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				if err := cur.Set("name", "qty"); err != nil {
					return err
				}
				if err := cur.Modify(doors.AChange{On: func(ctx context.Context, r doors.RequestChange) bool {
					changes <- r.Event().Value
					return false
				}}); err != nil {
					return err
				}
				return cur.Submit()
			}); err != nil {
				return err
			}
			return (neui.Input{Desc: "places the order", Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init("button"); err != nil {
					return err
				}
				if err := cur.Set("name", "place"); err != nil {
					return err
				}
				if err := cur.Modify(doors.AClick{On: func(ctx context.Context, r doors.RequestPointer) bool {
					clicks <- struct{}{}
					return false
				}}); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text("Place"); err != nil {
					return err
				}
				return cur.Close()
			})
		})
	}
}

func TestInputDispatchReachesHandlers(t *testing.T) {
	changes := make(chan string, 1)
	clicks := make(chan struct{}, 1)
	inputs := map[string]uint{}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "order"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			for _, in := range s.Inputs {
				inputs[in.Desc] = in.ID
			}
			return testkit.Call("page__write", map[string]any{"path": "order", "id": inputs["how many"], "value": "3"})
		}),
		testkit.Call("page__read", map[string]any{"path": "order", "id": 1}),
		testkit.Call("page__click", map[string]any{"path": "order", "id": 2}),
		testkit.Say("placed"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: order(changes, clicks)})
	c.Send("order three")
	c.Wait()
	var written hookOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &written)
	if written.Hooks != 1 || written.Diff != nil || written.Changes != nil {
		t.Fatalf("the write answered %+v, want the change handler and nothing changed", written)
	}
	if inputs["how many"] != 1 || inputs["places the order"] != 2 {
		t.Fatalf("sync listed inputs %v, want how many as 1 and places the order as 2", inputs)
	}
	select {
	case v := <-changes:
		if v != "3" {
			t.Fatalf("the change handler saw %q, want 3", v)
		}
	default:
		t.Fatal("the write returned before its change handler ran")
	}
	var read readOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &read)
	if read.Value != "3" {
		t.Fatalf("the read answered %+v, want the written 3", read)
	}
	var clicked hookOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &clicked)
	if clicked.Hooks != 1 {
		t.Fatalf("the click answered %+v, want the click handler", clicked)
	}
	select {
	case <-clicks:
	default:
		t.Fatal("the click returned before its handler ran")
	}
}

func doored(d *doors.Door, content gox.Elem) gox.Elem {
	return func(cur gox.Cursor) error {
		return d.Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("div"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Any(content); err != nil {
				return err
			}
			return cur.Close()
		})
	}
}

func TestRadioGroupStaysWritableAfterFirstMemberDoorIsRemoved(t *testing.T) {
	changes := make(chan string, 4)
	sizes := func(cur gox.Cursor) error {
		first := new(doors.Door)
		return (neui.Space{Name: "sizes", Desc: "the sizes"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "drop", Desc: "removes the small size", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				for err := range first.Inner(ctx, nil) {
					if err != nil {
						return "", err
					}
				}
				return "dropped", nil
			}}); err != nil {
				return err
			}
			if err := cur.Any(doored(first, radio("size", "s", changes))); err != nil {
				return err
			}
			return cur.Any(doored(new(doors.Door), radio("size", "m", changes)))
		})
	}
	var group uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "sizes"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var sizes state
			testkit.Decode(t, testkit.Results(r)[0], &sizes)
			if len(sizes.Inputs) != 1 {
				t.Errorf("the sizes synced as %+v, want one radio group", sizes.Inputs)
				return testkit.Say("lost")
			}
			group = sizes.Inputs[0].ID
			return testkit.Call("page__write", map[string]any{"path": "sizes", "id": group, "value": "s"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__tool", map[string]any{"path": "sizes", "tool": "drop", "arg": map[string]any{}})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__write", map[string]any{"path": "sizes", "id": group, "value": "m"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__read", map[string]any{"path": "sizes", "id": group})
		}),
		testkit.Say("picked"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: sizes})
	c.Send("pick medium")
	c.Wait()
	var before hookOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &before)
	if before.Hooks != 1 || <-changes != "s" {
		t.Fatalf("the write before the drop answered %+v, want the small radio's change handler", before)
	}
	var dropped toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &dropped)
	if dropped.Output != "dropped" || dropped.Diff != nil {
		t.Fatalf("the drop answered %+v, want the group kept", dropped)
	}
	var after hookOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &after)
	if after.Hooks != 1 {
		t.Fatalf("the write after the drop answered %+v, want the medium radio's change handler", after)
	}
	select {
	case v := <-changes:
		if v != "m" {
			t.Fatalf("the write after the drop reached %q, want m", v)
		}
	default:
		t.Fatal("the write after the drop returned before its change handler ran")
	}
	var read struct {
		Value   any
		Options []struct{ Value string }
	}
	testkit.Decode(t, testkit.Results(p.Request(t, 5))[0], &read)
	if read.Value != "m" || len(read.Options) != 1 {
		t.Fatalf("the read after the drop answered %+v, want m as the only option", read)
	}
}

func TestReadReportsDisabled(t *testing.T) {
	field := func(desc string, attrs ...any) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Input{Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				for i := 0; i < len(attrs); i += 2 {
					if err := cur.Set(attrs[i].(string), attrs[i+1]); err != nil {
						return err
					}
				}
				return cur.Submit()
			})
		}
	}
	size := func(cur gox.Cursor) error {
		return (neui.Input{Desc: "the size"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("select"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			for _, o := range []string{"s", "m"} {
				if err := cur.Init("option"); err != nil {
					return err
				}
				if err := cur.Set("value", o); err != nil {
					return err
				}
				if err := cur.Set("disabled", o == "m"); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text(o); err != nil {
					return err
				}
				if err := cur.Close(); err != nil {
					return err
				}
			}
			return cur.Close()
		})
	}
	page := func(cur gox.Cursor) error {
		return (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, el := range []gox.Elem{
				field("the name", "disabled", true),
				size,
				field("the color", "type", "radio", "name", "color", "value", "red"),
				field("the color", "type", "radio", "name", "color", "value", "blue", "disabled", true),
			} {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		round(
			call("name", "page__read", map[string]any{"path": "form", "id": 1}),
			call("size", "page__read", map[string]any{"path": "form", "id": 2}),
			call("color", "page__read", map[string]any{"path": "form", "id": 3}),
		),
		testkit.Say("read"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: page})
	c.Send("read the form")
	c.Wait()
	var got []string
	for _, r := range testkit.Results(p.Request(t, 2)) {
		var raw json.RawMessage
		testkit.Decode(t, r, &raw)
		got = append(got, string(raw))
	}
	want := []string{
		`{"value":"","disabled":true}`,
		`{"value":"s","options":[{"value":"s","label":"s","selected":true},{"value":"m","label":"m","selected":false,"disabled":true}]}`,
		`{"options":[{"value":"red","label":"","selected":false},{"value":"blue","label":"","selected":false,"disabled":true}]}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the reads answered\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInputCallsReportChanges(t *testing.T) {
	edits := doors.NewSource(0)
	bump := func(ctx context.Context) { edits.Mutate(ctx, func(n int) int { return n + 1 }) }
	count := func() gox.Elem {
		return edits.Bind(func(n int) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Text: "edits", Object: n}) }
		})
	}
	field := func(desc string, tag string, hook gox.Modify) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Input{Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init(tag); err != nil {
					return err
				}
				if err := cur.Modify(hook); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				return cur.Close()
			})
		}
	}
	form := func(cur gox.Cursor) error {
		if err := (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, el := range []gox.Elem{
				field("the name", "input", doors.AChange{On: func(ctx context.Context, _ doors.RequestChange) bool { bump(ctx); return false }}),
				field("saves", "button", doors.AClick{On: func(ctx context.Context, _ doors.RequestPointer) bool { bump(ctx); return false }}),
				field("the note", "textarea", doors.AFocus{On: func(ctx context.Context, _ doors.RequestFocus) bool { bump(ctx); return false }}),
				count(),
			} {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return (neui.Space{Name: "summary", Desc: "the summary"}).Proxy(cur, func(cur gox.Cursor) error { return cur.Any(count()) })
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		testkit.Call("page__write", map[string]any{"path": "form", "id": 1, "value": "Ann"}),
		testkit.Call("page__click", map[string]any{"path": "form", "id": 2}),
		testkit.Call("page__focus", map[string]any{"path": "form", "id": 3}),
		testkit.Say("done"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: form})
	c.Send("fill the form")
	c.Wait()
	for i, call := range []string{"write", "click", "focus"} {
		var out report
		r := testkit.Results(p.Request(t, i+2))[0]
		if r.Error != api.ToolOk {
			t.Fatalf("the %s answered %#v", call, r.Content)
		}
		testkit.Decode(t, r, &out)
		if out.Diff == nil || len(out.Diff.Data) != 1 || out.Diff.Data[0].Object != float64(i+1) || string(out.Changes) != `[{"path":"summary","changed":["data"]}]` {
			t.Fatalf("the %s answered diff %+v and changes %s, want edits at %d in the form and the summary listed", call, out.Diff, out.Changes, i+1)
		}
	}
}
