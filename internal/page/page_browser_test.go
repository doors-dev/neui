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
