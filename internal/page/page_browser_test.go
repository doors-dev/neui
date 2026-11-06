//go:build browser

package page_test

import (
	"context"
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
	Seq   uint
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
	var seq uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		synced(t, func(s tree) testkit.Turn {
			o := s.Space.Children["order"]
			seq = o.Seq
			for _, in := range o.Inputs {
				inputs[in.Name] = in.ID
			}
			return testkit.Call("page__write", map[string]any{"path": "order", "seq": seq, "id": inputs["qty"], "value": "3"})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var w hookOutput
			testkit.Decode(t, testkit.Results(r)[0], &w)
			seq = w.Seq
			return testkit.Call("page__read", map[string]any{"path": "order", "seq": seq, "id": inputs["qty"]})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__click", map[string]any{"path": "order", "seq": seq, "id": inputs["place"]})
		}),
		testkit.Say("placed"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: order(changes, clicks)})
	c.Send("order three")
	c.Wait()
	var written hookOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &written)
	if written.Hooks != 1 || written.Seq == 0 {
		t.Fatalf("the write answered %+v, want the change handler and the space's seq", written)
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
	var seq uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		synced(t, func(s tree) testkit.Turn {
			sizes := s.Space.Children["sizes"]
			if len(sizes.Inputs) != 1 {
				t.Errorf("the sizes synced as %+v, want one radio group", sizes.Inputs)
				return testkit.Say("lost")
			}
			group, seq = sizes.Inputs[0].ID, sizes.Seq
			return testkit.Call("page__write", map[string]any{"path": "sizes", "seq": seq, "id": group, "value": "s"})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var w hookOutput
			testkit.Decode(t, testkit.Results(r)[0], &w)
			seq = w.Seq
			return testkit.Call("page__tool", map[string]any{"path": "sizes", "seq": seq, "tool": "drop", "arg": map[string]any{}})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var d toolOutput
			testkit.Decode(t, testkit.Results(r)[0], &d)
			seq = d.Seq
			return testkit.Call("page__write", map[string]any{"path": "sizes", "seq": seq, "id": group, "value": "m"})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var w hookOutput
			testkit.Decode(t, testkit.Results(r)[0], &w)
			return testkit.Call("page__read", map[string]any{"path": "sizes", "seq": w.Seq, "id": group})
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
	if dropped.Output != "dropped" {
		t.Fatalf("the drop answered %+v", dropped)
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
