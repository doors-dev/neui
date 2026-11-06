package page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type tree struct {
	PageSeq uint
	Space   struct {
		Seq      uint
		Children map[string]struct {
			Seq    uint
			Inputs []struct {
				ID     uint
				Name   string
				Rights string
			}
		}
	}
}

type changed struct {
	PageSeq uint
	Spaces  []struct {
		Path string
		Seq  uint
		Data []struct {
			Text   string
			Object any
		}
	}
}

type toolOutput struct {
	Output  string
	Seq     uint
	Changes changed
}

func shop() gox.Elem {
	items := doors.NewSource(1)
	return func(cur gox.Cursor) error {
		if err := cur.Comp(neui.Data{Text: "the shop"}); err != nil {
			return err
		}
		if err := (neui.Space{Name: "cart", Desc: "the cart"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "add", Desc: "adds an item", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				items.Mutate(ctx, func(n int) int { return n + 1 })
				return "added", nil
			}}); err != nil {
				return err
			}
			if err := cur.Any(items.Bind(func(n int) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Text: "items", Object: n}) }
			})); err != nil {
				return err
			}
			if err := (neui.Input{Desc: "the order total", Rights: neui.R}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				if err := cur.Set("name", "total"); err != nil {
					return err
				}
				return cur.Submit()
			}); err != nil {
				return err
			}
			return (neui.Input{Desc: "pays the order", Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init("button"); err != nil {
					return err
				}
				if err := cur.Set("name", "pay"); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text("Pay"); err != nil {
					return err
				}
				return cur.Close()
			})
		}); err != nil {
			return err
		}
		return (neui.Space{Name: "notes", Desc: "the notes"}).Proxy(cur, func(cur gox.Cursor) error {
			return cur.Comp(neui.Data{Text: "a note"})
		})
	}
}

func refused(t *testing.T, r api.ContentToolResult, want string) {
	t.Helper()
	if r.Error == api.ToolOk {
		t.Fatalf("call %s succeeded with %#v, want it refused with %q", r.ID, r.Content, want)
	}
	if text, ok := r.Content[0].(api.ContentText); !ok || !strings.Contains(text.Text, want) {
		t.Fatalf("call %s refused with %#v, want %q", r.ID, r.Content, want)
	}
}

func synced(t *testing.T, then func(s tree) testkit.Turn) testkit.Turn {
	return testkit.Then(func(r api.Request) testkit.Turn {
		var s tree
		testkit.Decode(t, testkit.Results(r)[0], &s)
		return then(s)
	})
}

func TestToolBeforeSyncFails(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__tool", map[string]any{"path": "cart", "seq": 1, "tool": "add", "arg": map[string]any{}}),
		testkit.Call("page__changes", map[string]any{"sincePageSeq": 0}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("add an item")
	c.Wait()
	refused(t, testkit.Results(p.Request(t, 1))[0], "has no sync")
	refused(t, testkit.Results(p.Request(t, 2))[0], "has no sync")
}

func TestSyncTree(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}), testkit.Say("done"))
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	var raw json.RawMessage
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &raw)
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, out.Bytes())
}

func TestToolStaleSeqRefused(t *testing.T) {
	var first uint
	var added toolOutput
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		synced(t, func(s tree) testkit.Turn {
			first = s.Space.Children["cart"].Seq
			return testkit.Call("page__tool", map[string]any{"path": "cart", "seq": first, "tool": "add", "arg": map[string]any{}})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			testkit.Decode(t, testkit.Results(r)[0], &added)
			return testkit.Call("page__tool", map[string]any{"path": "cart", "seq": first, "tool": "add", "arg": map[string]any{}})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__tool", map[string]any{"path": "cart", "seq": added.Seq, "tool": "add", "arg": map[string]any{}})
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("add two items")
	c.Wait()
	if added.Output != "added" || added.Seq <= first {
		t.Fatalf("the first add answered %+v, want output added and a seq past %d", added, first)
	}
	refused(t, testkit.Results(p.Request(t, 3))[0], "seq is not this space's current seq")
	var again toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &again)
	if again.Output != "added" || again.Seq <= added.Seq {
		t.Fatalf("the add with the newer seq answered %+v, want output added and a seq past %d", again, added.Seq)
	}
}

func TestChangesSinceSeq(t *testing.T) {
	var since uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		synced(t, func(s tree) testkit.Turn {
			since = s.PageSeq
			return testkit.Call("page__tool", map[string]any{"path": "cart", "seq": s.Space.Children["cart"].Seq, "tool": "add", "arg": map[string]any{}})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__changes", map[string]any{"sincePageSeq": since})
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("add an item")
	c.Wait()
	var added toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &added)
	var changes changed
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &changes)
	if len(changes.Spaces) != 1 || changes.Spaces[0].Path != "cart" || changes.Spaces[0].Seq != added.Seq {
		t.Fatalf("changes since %d list %+v, want only cart at seq %d", since, changes.Spaces, added.Seq)
	}
	data := changes.Spaces[0].Data
	if len(data) != 1 || data[0].Text != "items" || data[0].Object != float64(2) {
		t.Fatalf("the changed cart carries data %+v, want items at 2", data)
	}
	if changes.PageSeq <= since {
		t.Fatalf("changes answered pageSeq %d, want past %d", changes.PageSeq, since)
	}
}

func TestInputRightsRefused(t *testing.T) {
	inputs := map[string]uint{}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		synced(t, func(s tree) testkit.Turn {
			cart := s.Space.Children["cart"]
			seq := cart.Seq
			for _, in := range cart.Inputs {
				inputs[in.Name+" "+in.Rights] = in.ID
			}
			return testkit.Events(
				api.EventStart{ID: "rights", Model: "fake"},
				api.EventTool{Index: 0, ID: "write-total", Name: "page__write", Args: fmt.Sprintf(`{"path":"cart","seq":%d,"id":%d,"value":"9"}`, seq, inputs["total R"])},
				api.EventClose{Index: 0},
				api.EventTool{Index: 1, ID: "click-total", Name: "page__click", Args: fmt.Sprintf(`{"path":"cart","seq":%d,"id":%d}`, seq, inputs["total R"])},
				api.EventClose{Index: 1},
				api.EventTool{Index: 2, ID: "read-pay", Name: "page__read", Args: fmt.Sprintf(`{"path":"cart","seq":%d,"id":%d}`, seq, inputs["pay W"])},
				api.EventClose{Index: 2},
				api.EventOk{Code: api.CodeTurn},
			)
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("pay")
	c.Wait()
	if len(inputs) != 2 {
		t.Fatalf("sync listed inputs %v, want total with R and pay with W", inputs)
	}
	results := testkit.Results(p.Request(t, 2))
	if len(results) != 3 {
		t.Fatalf("the rights round answered %d calls, want 3", len(results))
	}
	for _, r := range results {
		refused(t, r, "rights do not allow")
	}
}

func radio(name string, value string, changes chan<- string) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Input{Desc: "size " + value}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.InitVoid("input"); err != nil {
				return err
			}
			if err := cur.Set("type", "radio"); err != nil {
				return err
			}
			if name != "" {
				if err := cur.Set("name", name); err != nil {
					return err
				}
			}
			if err := cur.Set("value", value); err != nil {
				return err
			}
			if err := cur.Modify(doors.AChange{On: func(ctx context.Context, r doors.RequestChange) bool {
				changes <- value
				return false
			}}); err != nil {
				return err
			}
			return cur.Submit()
		})
	}
}

func TestUnnamedRadiosAreSeparateInputs(t *testing.T) {
	changes := make(chan string, 4)
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}), testkit.Say("done"))
	c := stack.Page(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		return (neui.Space{Name: "sizes", Desc: "the sizes"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, r := range []gox.Elem{radio("", "s", changes), radio("", "m", changes), radio("size", "l", changes), radio("size", "xl", changes)} {
				if err := cur.Any(r); err != nil {
					return err
				}
			}
			return nil
		})
	}})
	c.Send("look")
	c.Wait()
	var s struct {
		Space struct {
			Children map[string]struct {
				Inputs []struct {
					ID   uint
					Kind string
					Name string
					Desc string
				}
			}
		}
	}
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &s)
	var got []string
	ids := map[uint]bool{}
	for _, in := range s.Space.Children["sizes"].Inputs {
		got = append(got, in.Kind+" "+in.Name+" "+in.Desc)
		ids[in.ID] = true
	}
	want := []string{"radio  size s", "radio  size m", "radio size size l"}
	if fmt.Sprint(got) != fmt.Sprint(want) || len(ids) != 3 {
		t.Fatalf("the radios synced as %q, want %q with distinct ids", got, want)
	}
}
