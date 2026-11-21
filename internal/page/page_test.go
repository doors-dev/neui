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

package page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/zeebo/xxh3"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
	"neui.dev/internal/schema"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/thread"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type state struct {
	Path   string
	Island bool
	Tools  []struct{ Name string }
	Inputs []struct {
		ID     string
		Kind   string
		Desc   string
		Rights string
	}
	Data []struct {
		Text   string
		Object any
	}
	Children []struct {
		Name   string
		Island bool
	}
}

type diff struct {
	Removed bool
	Tools   *struct {
		Added   []struct{ Name string }
		Removed []string
	}
	Inputs *struct {
		Added   []struct{ ID string }
		Removed []string
	}
	Data []struct {
		Text   string
		Object any
	}
	Children *struct {
		Added   []struct{ Name string }
		Removed []string
	}
}

type report struct {
	Diff    *diff
	Changes json.RawMessage
}

type toolOutput struct {
	Output any
	report
}

func empty(d *diff) bool {
	return d != nil && reflect.ValueOf(*d).IsZero()
}

type refusal struct {
	Diff  *diff
	Space *state
}

func shop() gox.Elem {
	items := doors.NewSource(1)
	return func(cur gox.Cursor) error {
		if err := cur.Comp(neui.Data{Text: "the shop"}); err != nil {
			return err
		}
		if err := cur.Comp(neui.Tool[struct{}, string]{Name: "checkout", Desc: "starts the checkout", Handler: func(context.Context, struct{}) (string, error) {
			return "checking out", nil
		}}); err != nil {
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
			if err := (neui.Use{Desc: "the order total", Rights: neui.R}).Proxy(cur, func(cur gox.Cursor) error {
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
			return (neui.Use{Desc: "pays the order", Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
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
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "restock", Desc: "puts one more item in the cart", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				items.Mutate(ctx, func(n int) int { return n + 1 })
				return "restocked", nil
			}}); err != nil {
				return err
			}
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

func refusedWith(t *testing.T, r api.ContentToolResult) refusal {
	t.Helper()
	var keys map[string]json.RawMessage
	failedWith(t, r, "not done", &keys)
	if len(keys) != 1 || (keys["diff"] == nil && keys["space"] == nil) {
		t.Fatalf("call %s was refused with %#v, want diff or space alone", r.ID, r.Content)
	}
	var out refusal
	testkit.Decode(t, api.ContentToolResult{ID: r.ID, Content: r.Content[1:]}, &out)
	return out
}

func failedWith(t *testing.T, r api.ContentToolResult, message string, v any) {
	t.Helper()
	if r.Error == api.ToolOk || len(r.Content) != 2 {
		t.Fatalf("call %s answered %#v, want an error with its message and its content", r.ID, r.Content)
	}
	if text, ok := r.Content[0].(api.ContentText); !ok || !strings.HasPrefix(text.Text, message) {
		t.Fatalf("call %s failed with %#v, want the message %q first", r.ID, r.Content, message)
	}
	testkit.Decode(t, api.ContentToolResult{ID: r.ID, Content: r.Content[1:]}, v)
}

func plain(t *testing.T, r api.ContentToolResult, want string) {
	t.Helper()
	refused(t, r, want)
	if len(r.Content) != 1 {
		t.Fatalf("call %s failed with %#v, want the message alone", r.ID, r.Content)
	}
}

func inputID(kind, typ, desc, name, value string) string {
	return fmt.Sprintf("%08x", uint32(xxh3.HashString(kind+"\x00"+typ+"\x00"+desc+"\x00"+name+"\x00"+value)))
}

func call(id string, name string, args any) api.Event {
	raw, _ := json.Marshal(args)
	return api.EventTool{ID: id, Name: name, Args: string(raw)}
}

func round(calls ...api.Event) testkit.Turn {
	events := []api.Event{api.EventStart{ID: "round", Model: "fake"}}
	for i, c := range calls {
		c := c.(api.EventTool)
		c.Index = i
		events = append(events, c, api.EventClose{Index: i})
	}
	return testkit.Events(append(events, api.EventOk{Code: api.CodeTurn})...)
}

func TestSyncSpace(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		round(call("root", "page__sync", map[string]any{"path": ""}), call("cart", "page__sync", map[string]any{"path": "cart"})),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	var out bytes.Buffer
	for _, r := range testkit.Results(p.Request(t, 1)) {
		var raw json.RawMessage
		testkit.Decode(t, r, &raw)
		if err := json.Indent(&out, raw, "", "  "); err != nil {
			t.Fatal(err)
		}
		out.WriteString("\n")
	}
	testkit.Golden(t, out.Bytes())
}

func TestTreeShowsSpacesAndCountsOnly(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		round(call("root", "page__tree", map[string]any{}), call("cart", "page__tree", map[string]any{"path": "cart", "depth": 0})),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	var got []string
	for _, r := range testkit.Results(p.Request(t, 1)) {
		var raw json.RawMessage
		testkit.Decode(t, r, &raw)
		got = append(got, string(raw))
	}
	want := []string{
		`{"name":"","desc":"","tools":1,"data":1,"children":[{"name":"cart","desc":"the cart","tools":1,"inputs":2,"data":1},{"name":"notes","desc":"the notes","tools":1,"data":1}]}`,
		`{"name":"cart","desc":"the cart","tools":1,"inputs":2,"data":1}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tree answered\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

type logs struct {
	sync.Mutex
	bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.Buffer.Write(p)
}

func (l *logs) String() string {
	l.Lock()
	defer l.Unlock()
	return l.Buffer.String()
}

func TestSpaceNamedFromHash(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__tree", map[string]any{}), testkit.Say("done"))
	c := stack.Page(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		return (neui.Space{Name: "!!!", Desc: "odd"}).Proxy(cur, func(cur gox.Cursor) error { return nil })
	}})
	c.Send("look")
	c.Wait()
	var root struct{ Children []struct{ Name string } }
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &root)
	if name := "x" + utils.NameHash("!!!"); len(root.Children) != 1 || root.Children[0].Name != name {
		t.Fatalf("a space named !!! shows as %+v, want it named %s from its hash", root.Children, name)
	}
}

func TestRenderFailsOnEmptyNameAndUnencodableData(t *testing.T) {
	for name, el := range map[string]gox.Elem{
		"empty name": func(cur gox.Cursor) error {
			return (neui.Space{Desc: "nameless"}).Proxy(cur, func(gox.Cursor) error { return nil })
		},
		"unencodable data": neui.Data{Text: "a channel", Object: make(chan int)}.Main(),
	} {
		app := doors.NewApp(func(context.Context, doors.Request) gox.Elem {
			return func(cur gox.Cursor) error {
				p := page.NewPage(doors.InstanceContext(cur.Context()))
				return doors.Ctx(context.WithValue(cur.Context(), common.KeySpace, p.Space())).Proxy(cur, el)
			}
		}, doors.WithID(doors.IDRand()), doors.WithLogger(slog.New(slog.DiscardHandler)))
		response := httptest.NewRecorder()
		app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code == http.StatusOK {
			t.Fatalf("the page with %s rendered", name)
		}
	}
}

func TestInputElementCarriesNeid(t *testing.T) {
	app := doors.NewApp(func(context.Context, doors.Request) gox.Elem {
		return func(cur gox.Cursor) error {
			p := page.NewPage(doors.InstanceContext(cur.Context()))
			return doors.Ctx(context.WithValue(cur.Context(), common.KeySpace, p.Space())).Proxy(cur, func(cur gox.Cursor) error {
				return (neui.Use{Desc: "the note"}).Proxy(cur, func(cur gox.Cursor) error {
					if err := cur.InitVoid("input"); err != nil {
						return err
					}
					return cur.Submit()
				})
			})
		}
	}, doors.WithID(doors.IDRand()), doors.WithLogger(slog.New(slog.DiscardHandler)))
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	src := regexp.MustCompile(`data-d0d-id="[^"]*" src="([^"]+)"`).FindStringSubmatch(body)
	if !strings.Contains(body, `<input data-neid="`) || src == nil {
		t.Fatalf("the page rendered %s, want the input keyed by data-neid and its script", body)
	}
	script := httptest.NewRecorder()
	app.ServeHTTP(script, httptest.NewRequest(http.MethodGet, src[1], nil))
	if !strings.Contains(script.Body.String(), `[data-neid="`) || strings.Contains(script.Body.String(), "data-kid") {
		t.Fatalf("the input's script is %s, want it to find the element by data-neid", script.Body.String())
	}
}

func TestReadIsChecked(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__read", map[string]any{"path": "cart", "id": "unread"}), testkit.Say("done"))
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("read the total")
	c.Wait()
	if r := refusedWith(t, testkit.Results(p.Request(t, 1))[0]); r.Space == nil || r.Space.Path != "cart" {
		t.Fatalf("the read without a sync was refused with %+v, want the whole cart", r)
	}
}

func TestActionWithoutReadRefusedWithSpace(t *testing.T) {
	add := map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}
	p := testkit.NewProvider(api.Model{},
		round(call("tree", "page__tree", map[string]any{}), call("add", "page__tool", add)),
		testkit.Call("page__tool", add),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("add an item")
	c.Wait()
	r := refusedWith(t, testkit.Results(p.Request(t, 1))[1])
	if r.Diff != nil || r.Space == nil || r.Space.Path != "cart" || len(r.Space.Tools) != 1 || len(r.Space.Inputs) != 2 {
		t.Fatalf("the add after only a tree was refused with %+v, want the whole cart", r)
	}
	var added toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &added)
	if added.Output != "added" || added.Diff == nil || len(added.Diff.Data) != 1 || added.Diff.Data[0].Object != float64(2) || added.Diff.Inputs != nil {
		t.Fatalf("the add after the refusal answered %+v, want it run with the new items count", added)
	}
}

func TestChangeByOtherCallRefusedWithDiff(t *testing.T) {
	add := map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}
	p := testkit.NewProvider(api.Model{},
		round(call("cart", "page__sync", map[string]any{"path": "cart"}), call("notes", "page__sync", map[string]any{"path": "notes"})),
		testkit.Call("page__tool", map[string]any{"path": "notes", "tool": "restock", "arg": map[string]any{}}),
		testkit.Call("page__tool", add),
		testkit.Call("page__tool", add),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("restock and add")
	c.Wait()
	var restocked toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &restocked)
	if restocked.Output != "restocked" || !empty(restocked.Diff) || string(restocked.Changes) != `[{"path":"cart","changed":["data"]}]` {
		t.Fatalf("the restock answered %+v with changes %s, want cart's data listed", restocked, restocked.Changes)
	}
	r := refusedWith(t, testkit.Results(p.Request(t, 3))[0])
	if r.Space != nil || r.Diff == nil || len(r.Diff.Data) != 1 || r.Diff.Data[0].Object != float64(2) || r.Diff.Tools != nil {
		t.Fatalf("the add after the restock was refused with %+v, want the data diff", r)
	}
	var added toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &added)
	if added.Output != "added" || len(added.Diff.Data) != 1 || added.Diff.Data[0].Object != float64(3) {
		t.Fatalf("the retried add answered %+v, want it run", added)
	}
}

func TestChildChangeDoesNotRefuseParent(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		round(call("root", "page__sync", map[string]any{"path": ""}), call("cart", "page__sync", map[string]any{"path": "cart"})),
		testkit.Call("page__tool", map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}),
		testkit.Call("page__tool", map[string]any{"path": "", "tool": "checkout", "arg": map[string]any{}}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("add and check out")
	c.Wait()
	var out toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &out)
	if out.Output != "checking out" || !empty(out.Diff) || out.Changes != nil {
		t.Fatalf("the checkout after a change inside cart answered %+v, want it run with nothing changed", out)
	}
}

func TestRelinkEndsAnInFlightCall(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			for _, in := range s.Inputs {
				if in.Desc == "pays the order" {
					return testkit.Call("page__click", map[string]any{"path": "cart", "id": in.ID})
				}
			}
			t.Errorf("the cart synced as %+v, want the pay button", s)
			return testkit.Say("lost")
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("pay")
	ends := 0
	c.Until(func(e api.ChatEvent) bool {
		if _, ok := e.(api.ChatEventRoundEnd); ok {
			ends++
		}
		return ends == 2
	})
	c.Relink()
	c.Wait()
	plain(t, testkit.Results(p.Request(t, 2))[0], "the chat moved to a new page, by a reload or in another tab, which ended what ran on the old page")
}

func TestInputRightsRefused(t *testing.T) {
	inputs := map[string]string{}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			for _, in := range s.Inputs {
				inputs[in.Desc+" "+in.Rights] = in.ID
			}
			return round(
				call("write-total", "page__write", map[string]any{"path": "cart", "id": inputs["the order total R"], "value": "9"}),
				call("click-total", "page__click", map[string]any{"path": "cart", "id": inputs["the order total R"]}),
				call("read-pay", "page__read", map[string]any{"path": "cart", "id": inputs["pays the order W"]}),
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
	for i, call := range []string{"write", "click", "read"} {
		refused(t, results[i], fmt.Sprintf("%q on input ", call))
		refused(t, results[i], "rights do not allow")
	}
}

func radio(name string, value string, changes chan<- string) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Use{Desc: "size " + value}).Proxy(cur, func(cur gox.Cursor) error {
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
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__sync", map[string]any{"path": "sizes"}), testkit.Say("done"))
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
	var s state
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &s)
	var got []string
	for _, in := range s.Inputs {
		got = append(got, in.Kind+" "+in.Desc+" "+in.ID)
	}
	want := []string{"radio size s " + inputID("radio", "radio", "size s", "", "s"), "radio size m " + inputID("radio", "radio", "size m", "", "m"), "radio size l " + inputID("radio", "radio", "size l", "size", "")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the radios synced as %q, want %q: an unnamed radio's id has its value, a named group's leaves it out", got, want)
	}
}

type stuck struct {
	Count int `json:"count"`
}

func (s stuck) Error() string {
	return "stuck"
}

func (s stuck) Content() api.UserContent {
	return api.ContentText{Text: fmt.Sprintf("stuck at %d", s.Count)}
}

func (s stuck) ContentType() string {
	return "text"
}

type tangled struct {
	stuck
	Wire chan int
}

func (tangled) Error() string {
	return "tangled"
}

type spread struct {
	Parts []string `json:"parts"`
}

func (s spread) Error() string {
	return "spread"
}

func (s spread) Contents() []api.UserContent {
	var out []api.UserContent
	for _, part := range s.Parts {
		out = append(out, api.ContentText{Text: part})
	}
	return out
}

func board() gox.Elem {
	open := doors.NewSource(false)
	count := doors.NewSource(0)
	toggle := func(ctx context.Context, _ struct{}) (string, error) {
		open.Mutate(ctx, func(o bool) bool { return !o })
		return "toggled", nil
	}
	space := func(cur gox.Cursor, name string, desc string, content ...any) error {
		return (neui.Space{Name: name, Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
			for _, c := range content {
				if err := cur.Any(c); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return func(cur gox.Cursor) error {
		if err := space(cur, "controls", "the controls", neui.Tool[struct{}, string]{Name: "toggle", Desc: "toggles the dialog and counts", Handler: func(ctx context.Context, in struct{}) (string, error) {
			count.Mutate(ctx, func(n int) int { return n + 1 })
			return toggle(ctx, in)
		}}); err != nil {
			return err
		}
		fail := func(name string, err error) neui.Tool[struct{}, string] {
			return neui.Tool[struct{}, string]{Name: name, Desc: "counts and fails", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				count.Mutate(ctx, func(n int) int { return n + 1 })
				return "", err
			}}
		}
		spill := neui.Tool[struct{}, float64]{Name: "spill", Desc: "counts and answers what cannot be encoded", Handler: func(ctx context.Context, _ struct{}) (float64, error) {
			count.Mutate(ctx, func(n int) int { return n + 1 })
			return math.NaN(), nil
		}}
		if err := space(cur, "counter", "the counter", fail("jam", errors.New("jammed")), fail("stick", fmt.Errorf("jam: %w", stuck{Count: 7})), fail("tangle", tangled{Wire: make(chan int)}), fail("spread", spread{Parts: []string{"a", "b"}}), spill, count.Bind(func(n int) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Text: "count", Object: n}) }
		})); err != nil {
			return err
		}
		if err := space(cur, "list", "the list", neui.Tool[struct{}, string]{Name: "noop", Desc: "does nothing", Handler: func(context.Context, struct{}) (string, error) {
			return "nothing", nil
		}}, count.Bind(func(n int) gox.Elem {
			return func(cur gox.Cursor) error {
				return space(cur, "row", "the row", neui.Data{Text: "row", Object: n})
			}
		})); err != nil {
			return err
		}
		return space(cur, "panel", "the panel", neui.Tool[struct{}, string]{Name: "toggle", Desc: "toggles the dialog", Handler: toggle}, gox.Elem(func(cur gox.Cursor) error {
			return space(cur, "dialogs", "the dialog list", neui.Tool[struct{}, string]{Name: "noop", Desc: "does nothing", Handler: func(context.Context, struct{}) (string, error) {
				return "nothing", nil
			}})
		}), open.Bind(func(o bool) gox.Elem {
			return func(cur gox.Cursor) error {
				if !o {
					return nil
				}
				return space(cur, "dialog", "the dialog", neui.Tool[struct{}, string]{Name: "close", Desc: "closes the dialog", Handler: func(ctx context.Context, _ struct{}) (string, error) {
					open.Mutate(ctx, func(bool) bool { return false })
					return "closed", nil
				}}, gox.Elem(func(cur gox.Cursor) error {
					if err := space(cur, "body", "the body", neui.Tool[struct{}, string]{Name: "noop", Desc: "does nothing", Handler: func(context.Context, struct{}) (string, error) {
						return "nothing", nil
					}}); err != nil {
						return err
					}
					return space(cur, "footer", "the footer")
				}))
			}
		}))
	}
}

func TestReportListsWhatChanged(t *testing.T) {
	tool := func(path string, name string) map[string]any {
		return map[string]any{"path": path, "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(api.Model{},
		round(call("controls", "page__sync", map[string]any{"path": "controls"}), call("list", "page__sync", map[string]any{"path": "list"})),
		testkit.Call("page__tool", tool("controls", "toggle")),
		testkit.Call("page__sync", map[string]any{"path": "panel.dialog"}),
		testkit.Call("page__tool", tool("panel.dialog", "close")),
		testkit.Call("page__sync", map[string]any{"path": "panel"}),
		testkit.Call("page__tool", tool("panel", "toggle")),
		testkit.Call("page__tool", tool("list", "noop")),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board()})
	c.Send("toggle")
	c.Wait()
	output := func(i int) toolOutput {
		var out toolOutput
		r := testkit.Results(p.Request(t, i))[0]
		if r.Error != api.ToolOk {
			t.Fatalf("request %d answered %#v", i, r.Content)
		}
		testkit.Decode(t, r, &out)
		return out
	}
	toggled := output(2)
	if !empty(toggled.Diff) || string(toggled.Changes) != `[{"path":"counter","changed":["data"]},{"path":"list.row","changed":["data"]},{"path":"panel.dialog","added":true,"desc":"the dialog"}]` {
		t.Fatalf("the toggle answered diff %+v and changes %s", toggled.Diff, toggled.Changes)
	}
	closed := output(4)
	if closed.Diff == nil || !closed.Diff.Removed || closed.Changes != nil {
		t.Fatalf("the close answered diff %+v and changes %s, want the dialog removed", closed.Diff, closed.Changes)
	}
	reopened := output(6)
	if reopened.Diff == nil || reopened.Diff.Children == nil || len(reopened.Diff.Children.Added) != 1 || reopened.Diff.Children.Added[0].Name != "dialog" || reopened.Changes != nil {
		t.Fatalf("the panel's toggle answered diff %+v and changes %s, want the dialog in the diff only", reopened.Diff, reopened.Changes)
	}
	if noop := output(7); noop.Output != "nothing" {
		t.Fatalf("the list's noop after its row re-rendered answered %+v", noop)
	}
}

func TestAgentAndScriptsShareReads(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Call("js__run", map[string]any{"code": `page__sync({path: "notes"})
try { return ["ran", page__tool({path: "cart", tool: "add", arg: {}}).output] } catch (e) { return [e.message, Object.keys(e.value).join(",")] }`, "comment": "adds"}),
		testkit.Call("page__tool", map[string]any{"path": "notes", "tool": "restock", "arg": map[string]any{}}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("add from a script")
	c.Wait()
	var script struct{ Result []string }
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &script)
	if fmt.Sprint(script.Result) != "[ran added]" {
		t.Fatalf("a script acting on the agent's read saw %q, want the add run", script.Result)
	}
	r := testkit.Results(p.Request(t, 3))[0]
	if r.Error != api.ToolOk {
		t.Fatalf("the agent's restock on a script's read answered %#v, want it run", r.Content)
	}
	var restocked toolOutput
	testkit.Decode(t, r, &restocked)
	if restocked.Output != "restocked" {
		t.Fatalf("the agent's restock on a script's read answered %+v", restocked)
	}
}

func TestScriptsShareReads(t *testing.T) {
	script := func(code string) testkit.Turn {
		return testkit.Call("js__run", map[string]any{"code": code, "comment": "adds"})
	}
	p := testkit.NewProvider(api.Model{},
		script(`page__sync({path: "cart"}); return "synced"`),
		script(`try { return ["ran", page__tool({path: "cart", tool: "add", arg: {}}).output] } catch (e) { return [e.message, Object.keys(e.value).join(",")] }`),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("add from two scripts")
	c.Wait()
	var second struct{ Result []string }
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &second)
	if fmt.Sprint(second.Result) != "[ran added]" {
		t.Fatalf("a script acting on the read of an earlier script saw %q, want the add run", second.Result)
	}
}

func TestRefusalsFollowTheLastReadByAnyone(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		round(call("cart", "page__sync", map[string]any{"path": "cart"}), call("notes", "page__sync", map[string]any{"path": "notes"})),
		testkit.Call("page__tool", map[string]any{"path": "notes", "tool": "restock", "arg": map[string]any{}}),
		testkit.Call("js__run", map[string]any{"code": `const act = (path, tool) => { try { return ["ran", page__tool({path: path, tool: tool, arg: {}}).output] } catch (e) { return [e.message, Object.keys(e.value).join(",")] } }
return [act("cart", "add"), act("", "checkout")]`, "comment": "adds and checks out"}),
		round(call("add", "page__tool", map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}), call("checkout", "page__tool", map[string]any{"path": "", "tool": "checkout", "arg": map[string]any{}})),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("restock, add and check out")
	c.Wait()
	var script struct{ Result [][]string }
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &script)
	if len(script.Result) != 2 || len(script.Result[0]) != 2 || script.Result[0][0] != "not done: the space changed since your last read. diff shows what changed and brings your read up to date: check it and call again directly, no sync needed" || script.Result[0][1] != "diff" {
		t.Fatalf("a script's add on the cart the agent read before the restock saw %q, want it refused with the diff", script.Result)
	}
	if len(script.Result[1]) != 2 || script.Result[1][0] != "not done: this space had not been read. space shows it as it is now and counts as a read: check it and call again directly, no sync needed" || script.Result[1][1] != "space" {
		t.Fatalf("a script's checkout on the root nobody read saw %q, want it refused with the space", script.Result[1])
	}
	for i, want := range []string{"added", "checking out"} {
		r := testkit.Results(p.Request(t, 4))[i]
		if r.Error != api.ToolOk {
			t.Fatalf("the agent's call %s after the script's refusals answered %#v, want it run", r.ID, r.Content)
		}
		var out toolOutput
		testkit.Decode(t, r, &out)
		if out.Output != want {
			t.Fatalf("the agent's call %s answered %+v, want %q", r.ID, out, want)
		}
	}
}

func TestRewindClearsReads(t *testing.T) {
	started := make(chan struct{}, 1)
	streaming := make(chan struct{}, 1)
	hold := tools.NewModule("hold", "")
	if _, err := hold.AddRaw("run", "holds until canceled", schema.ObjectRawSchema, schema.AnyRawSchema, func(ctx context.Context, _ json.RawMessage) (any, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, context.Cause(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		name  string
		turn  testkit.Turn
		enter func(c *stack.Chat)
	}{
		{"waiting", testkit.Say("later"), func(c *stack.Chat) { c.Wait() }},
		{"streaming", testkit.Then(func(api.Request) testkit.Turn {
			streaming <- struct{}{}
			return testkit.Hang(api.EventStart{ID: "hang", Model: "stub"})
		}), func(*stack.Chat) { <-streaming }},
		{"tools", testkit.Call("hold__run", map[string]any{}), func(*stack.Chat) { <-started }},
		{"compacting", testkit.Events(api.EventStart{ID: "full", Model: "stub"}, api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")}), func(c *stack.Chat) {
			c.Until(func(e api.ChatEvent) bool {
				compacting, ok := e.(api.ChatEventCompacting)
				return ok && compacting.Running
			})
		}},
	} {
		t.Run(state.name, func(t *testing.T) {
			p := testkit.NewProvider(api.Model{Name: "stub"},
				testkit.Call("page__sync", map[string]any{"path": "cart"}),
				testkit.Say("synced"),
				state.turn,
				testkit.Call("page__tool", map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}),
				testkit.Say("done"),
			)
			p.OnCompaction(testkit.Hang(api.EventStart{ID: "compact", Model: "stub"}))
			c := stack.Page(t, stack.Host{Provider: p, Page: shop(), Options: []common.Option{common.OptionModule(func(context.Context) common.Module { return hold })}})
			c.Send("look")
			c.Wait()
			c.Send("wait")
			events := c.Until(func(e api.ChatEvent) bool {
				_, ok := e.(api.ChatEventRoundStart)
				return ok
			})
			second := events[len(events)-1].(api.ChatEventRoundStart).Round.ID
			state.enter(c)
			c.Rewind(second, "add")
			c.Until(func(e api.ChatEvent) bool {
				start, ok := e.(api.ChatEventRoundStart)
				return ok && start.Truncate == second
			})
			c.Wait()
			requests := p.Requests()
			if r := refusedWith(t, testkit.Results(requests[len(requests)-1])[0]); r.Space == nil {
				t.Fatalf("the add after a rewind while %s was refused with %+v, want the whole space", state.name, r)
			}
		})
	}
}

func TestCompactionClearsReads(t *testing.T) {
	add := map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}
	p := testkit.NewProvider(api.Model{Name: "stub"},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Say("synced"),
		testkit.Say("later"),
		testkit.Call("page__tool", add),
		testkit.Say("done"),
	)
	p.OnCompaction(testkit.Say("the summary"))
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	c.Send("wait")
	c.Wait()
	if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Compact(op) }); err != nil {
		t.Fatalf("compact: %v", err)
	}
	c.Until(func(e api.ChatEvent) bool {
		compacting, ok := e.(api.ChatEventCompacting)
		return ok && !compacting.Running
	})
	c.Send("add")
	c.Wait()
	if !testkit.IsCompaction(p.Request(t, 3)) {
		t.Fatal("request 3 is not the compaction")
	}
	if got := refusedWith(t, testkit.Results(p.Request(t, 5))[0]); got.Space == nil {
		t.Fatalf("the add after the compaction was refused with %+v, want the whole space", got)
	}
}

func big() gox.Elem {
	grown := doors.NewSource("")
	return func(cur gox.Cursor) error {
		return (neui.Space{Name: "big", Desc: "a big space"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "dump", Desc: "dumps", Handler: func(context.Context, struct{}) (string, error) {
				return strings.Repeat("x", 3000), nil
			}}); err != nil {
				return err
			}
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "grow", Desc: "grows", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				grown.Mutate(ctx, func(string) string { return strings.Repeat("y", 3000) })
				return "grown", nil
			}}); err != nil {
				return err
			}
			return cur.Any(grown.Bind(func(s string) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Object: s}) }
			}))
		})
	}
}

func TestResultsFitTheRoundItemBudget(t *testing.T) {
	tool := func(name string) map[string]any {
		return map[string]any{"path": "big", "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 2000},
		testkit.Call("page__sync", map[string]any{"path": "big"}),
		testkit.Call("page__tool", tool("dump")),
		testkit.Call("page__tool", tool("grow")),
		testkit.Call("page__tool", tool("dump")),
		testkit.Call("page__tool", tool("dump")),
		testkit.Call("page__sync", map[string]any{"path": "big"}),
		testkit.Call("js__run", map[string]any{"code": `page__sync({path: "big"}); return page__tool({path: "big", tool: "dump", arg: {}}).output.length`, "comment": "dumps"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: big(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("dump")
	c.Wait()
	result := func(i int) api.ContentToolResult { return testkit.Results(p.Request(t, i))[0] }
	if r := result(1); r.Error != api.ToolOk {
		t.Fatalf("the sync answered %#v", r.Content)
	}
	var dumped toolOutput
	testkit.Decode(t, result(2), &dumped)
	if note, _ := dumped.Output.(string); !strings.HasPrefix(note, "Result omitted: it was 2.9 KB") {
		t.Fatalf("the dump answered %+v, want the omission note in output", dumped)
	}
	var grew toolOutput
	testkit.Decode(t, result(3), &grew)
	if grew.Output != "grown" || grew.Diff != nil || grew.Changes != nil {
		t.Fatalf("the grow answered %+v, want only its output: the diff does not fit", grew)
	}
	for i := 4; i <= 5; i++ {
		plain(t, result(i), "not done: this space has not been read as it is now, and it is too large")
	}
	refused(t, result(6), "too large for a result")
	var scripted struct{ Result int }
	testkit.Decode(t, result(7), &scripted)
	if scripted.Result != 3000 {
		t.Fatalf("the script's dump answered %d bytes, want 3000", scripted.Result)
	}
}

func TestLightModeTexts(t *testing.T) {
	tool := func(name string) map[string]any {
		return map[string]any{"path": "big", "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 2000},
		testkit.Call("page__sync", map[string]any{"path": "big"}),
		testkit.Call("page__tool", tool("dump")),
		testkit.Call("page__tool", tool("grow")),
		testkit.Call("page__tool", tool("dump")),
		testkit.Call("page__sync", map[string]any{"path": "big"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: big(), Options: []common.Option{common.OptionLightMode(true)}})
	c.Send("dump")
	c.Wait()
	result := func(i int) api.ContentToolResult { return testkit.Results(p.Request(t, i))[0] }
	var synced struct {
		Tools []struct{ Name, Desc string }
	}
	testkit.Decode(t, result(1), &synced)
	if len(synced.Tools) != 2 || synced.Tools[0].Desc != "dumps" || synced.Tools[1].Desc != "grows" {
		t.Fatalf("the sync listed %+v, want the tools' descriptions alone", synced.Tools)
	}
	var dumped toolOutput
	testkit.Decode(t, result(2), &dumped)
	if note, _ := dumped.Output.(string); note != "Result omitted: it was 2.9 KB, more than a turn's tool results can hold. Ask for less." {
		t.Fatalf("the dump answered %+v, want the light omission note in output", dumped)
	}
	plain(t, result(4), "not done: this space has not been read as it is now, and it is too large to show in a result: tell the user this part of the page is out of your reach")
	refused(t, result(5), "the space is too large for a result: tell the user this part of the page is out of your reach")
	request := p.Request(t, 0)
	for _, tool := range request.Tools {
		if strings.HasPrefix(tool.Name, "page__island") || strings.Contains(tool.Desc, "Output schema") {
			t.Fatalf("light mode offers %s: %s", tool.Name, tool.Desc)
		}
	}
	if len(request.Tools) != 7 {
		t.Fatalf("light mode offers %d page tools, want 7", len(request.Tools))
	}
	for i := 1; i <= 5; i++ {
		for _, c := range result(i).Content {
			if text, _ := c.(api.ContentText); strings.Contains(text.Text, "script") || strings.Contains(text.Text, "Output schema") {
				t.Fatalf("result %d mentions scripts or output schemas: %s", i, text.Text)
			}
		}
	}
}

func TestFailuresFitTheRoundItemBudget(t *testing.T) {
	model := api.Model{Name: "fake", Context: 2000}
	budget := replay.RoundItemBudget(common.ConfigDefaults(api.ThreadConfig{}), model)
	burst := doors.NewSource("")
	loud := doors.NewSource(0)
	edge := strings.Repeat("e", budget-10-len(`{"space":{"path":"edge","desc":"the edge","data":[{"object":""}]}}`))
	shout := strings.Repeat("l", budget-20)
	space := func(cur gox.Cursor, name string, content ...any) error {
		return (neui.Space{Name: name, Desc: "the " + name}).Proxy(cur, func(cur gox.Cursor) error {
			for _, c := range content {
				if err := cur.Any(c); err != nil {
					return err
				}
			}
			return nil
		})
	}
	page := func(cur gox.Cursor) error {
		if err := space(cur, "burst", neui.Tool[struct{}, string]{Name: "burst", Desc: "grows past a result and fails", Handler: func(ctx context.Context, _ struct{}) (string, error) {
			burst.Mutate(ctx, func(string) string { return strings.Repeat("y", 3000) })
			return "", errors.New("burst")
		}}, burst.Bind(func(s string) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Object: s}) }
		})); err != nil {
			return err
		}
		if err := space(cur, "heavy", neui.Tool[struct{}, string]{Name: "heavy", Desc: "fails with content past a result", Handler: func(context.Context, struct{}) (string, error) {
			return "", spread{Parts: []string{strings.Repeat("z", 3000)}}
		}}); err != nil {
			return err
		}
		if err := space(cur, "loud", neui.Tool[struct{}, string]{Name: "loud", Desc: "counts and fails at length", Handler: func(ctx context.Context, _ struct{}) (string, error) {
			loud.Mutate(ctx, func(n int) int { return n + 1 })
			return "", errors.New(shout)
		}}, neui.Tool[struct{}, string]{Name: "quiet", Desc: "does nothing", Handler: func(context.Context, struct{}) (string, error) {
			return "quiet", nil
		}}, loud.Bind(func(n int) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Object: n}) }
		})); err != nil {
			return err
		}
		return space(cur, "edge", neui.Data{Object: edge})
	}
	tool := func(path string, name string) map[string]any {
		return map[string]any{"path": path, "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(model,
		testkit.Call("page__sync", map[string]any{"path": "burst"}),
		testkit.Call("page__tool", tool("burst", "burst")),
		testkit.Call("page__sync", map[string]any{"path": "heavy"}),
		testkit.Call("page__tool", tool("heavy", "heavy")),
		testkit.Call("page__sync", map[string]any{"path": "loud"}),
		testkit.Call("page__tool", tool("loud", "loud")),
		testkit.Call("page__tool", tool("loud", "quiet")),
		testkit.Call("page__tool", tool("edge", "noop")),
		testkit.Call("page__tool", tool("edge", "noop")),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("fail")
	c.Wait()
	result := func(i int) api.ContentToolResult { return testkit.Results(p.Request(t, i))[0] }
	plain(t, result(2), "burst")
	var heavy toolOutput
	failedWith(t, result(4), "spread", &heavy)
	if note, _ := heavy.Output.(string); !strings.HasPrefix(note, "Result omitted: it was 2.9 KB") || !empty(heavy.Diff) {
		t.Fatalf("the heavy failure carried %+v, want the omission note in output and the diff", heavy)
	}
	plain(t, result(6), shout)
	if r := refusedWith(t, result(7)); r.Diff == nil || len(r.Diff.Data) != 1 || r.Diff.Data[0].Object != float64(1) {
		t.Fatalf("the call after the loud failure was refused with %+v, want the count it did not see", r)
	}
	for i := 8; i <= 9; i++ {
		plain(t, result(i), "not done: this space has not been read as it is now, and it is too large")
	}
}

func TestFailedCallReports(t *testing.T) {
	tool := func(name string) map[string]any {
		return map[string]any{"path": "counter", "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "counter"}),
		testkit.Call("page__tool", tool("jam")),
		testkit.Call("page__tool", tool("jam")),
		testkit.Call("page__tool", tool("stick")),
		testkit.Call("page__tool", tool("tangle")),
		testkit.Call("page__tool", tool("spread")),
		testkit.Call("page__tool", tool("spill")),
		testkit.Call("page__tool", map[string]any{"path": "counter", "tool": "jam", "arg": 5}),
		testkit.Call("js__run", map[string]any{"code": `page__sync({path: "counter"})
return ["jam", "stick"].map(tool => { try { page__tool({path: "counter", tool, arg: {}}) } catch (e) { return [e.message, Object.keys(e.value).sort().join(","), e.value.output ?? null] } })`, "comment": "jams"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("jam")
	c.Wait()
	for i, want := range []struct {
		message string
		keys    string
		output  any
	}{
		{"jammed", "changes,diff", nil},
		{"jammed", "changes,diff", nil},
		{"jam: stuck", "changes,diff,output", map[string]any{"count": float64(7)}},
		{"tangled", "changes,diff", nil},
		{"spread", "changes,diff,output", map[string]any{"parts": []any{"a", "b"}}},
		{"json: unsupported value: NaN", "changes,diff", nil},
	} {
		r := testkit.Results(p.Request(t, i+2))[0]
		var keys map[string]json.RawMessage
		failedWith(t, r, want.message, &keys)
		var out toolOutput
		failedWith(t, r, want.message, &out)
		if got := strings.Join(slices.Sorted(maps.Keys(keys)), ","); got != want.keys || !reflect.DeepEqual(out.Output, want.output) {
			t.Fatalf("call %d failed with %s, want keys %s and output %v", i+1, r.Content[1], want.keys, want.output)
		}
		if out.Diff == nil || len(out.Diff.Data) != 1 || out.Diff.Data[0].Object != float64(i+1) || string(out.Changes) != `[{"path":"list.row","changed":["data"]}]` {
			t.Fatalf("call %d failed with %+v and changes %s, want the count at %d and the row listed", i+1, out, out.Changes, i+1)
		}
	}
	plain(t, testkit.Results(p.Request(t, 8))[0], schema.ErrInvalidArguments.Error())
	var scripted struct{ Result json.RawMessage }
	testkit.Decode(t, testkit.Results(p.Request(t, 9))[0], &scripted)
	if want := `[["jammed","changes,diff",null],["jam: stuck","changes,diff,output",{"count":7}]]`; string(scripted.Result) != want {
		t.Fatalf("the script caught %s, want the message in e.message and the call's result in e.value", scripted.Result)
	}
}

func TestPathWithEmptySegmentIsNoSpace(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		round(
			call("cart", "page__sync", map[string]any{"path": "cart"}),
			call("sync", "page__sync", map[string]any{"path": "cart."}),
			call("tool", "page__tool", map[string]any{"path": "cart.", "tool": "add", "arg": map[string]any{}}),
			call("lead", "page__sync", map[string]any{"path": ".cart"}),
		),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	results := testkit.Results(p.Request(t, 1))
	if results[0].Error != api.ToolOk {
		t.Fatalf("the sync of cart answered %#v", results[0].Content)
	}
	for _, r := range results[1:] {
		plain(t, r, `space "" under`)
		refused(t, r, "no such space")
	}
}

func TestReopenedDialogActsWithoutRead(t *testing.T) {
	tool := func(path string, name string) map[string]any {
		return map[string]any{"path": path, "tool": name, "arg": map[string]any{}}
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "controls"}),
		testkit.Call("page__tool", tool("controls", "toggle")),
		testkit.Call("page__sync", map[string]any{"path": "panel.dialog"}),
		testkit.Call("page__tool", tool("panel.dialog", "close")),
		testkit.Call("page__tool", tool("panel.dialog", "close")),
		testkit.Call("page__tool", tool("controls", "toggle")),
		testkit.Call("page__tool", tool("panel.dialog", "close")),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board()})
	c.Send("toggle")
	c.Wait()
	var closed toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &closed)
	if closed.Diff == nil || !closed.Diff.Removed {
		t.Fatalf("the close answered %+v, want the dialog removed", closed)
	}
	refused(t, testkit.Results(p.Request(t, 5))[0], "no such space")
	r := testkit.Results(p.Request(t, 7))[0]
	var again toolOutput
	testkit.Decode(t, r, &again)
	if r.Error != api.ToolOk || again.Output != "closed" {
		t.Fatalf("the close after the reopen answered %#v, want it run on the read from before", r.Content)
	}
}

func TestStoredReadDropsReadsOfGoneChildren(t *testing.T) {
	tool := func(path string, name string) map[string]any {
		return map[string]any{"path": path, "tool": name, "arg": map[string]any{}}
	}
	for _, store := range []struct {
		name  string
		after testkit.Turn
	}{
		{"sync", testkit.Call("page__sync", map[string]any{"path": "panel"})},
		{"report", testkit.Call("page__tool", tool("panel", "toggle"))},
	} {
		t.Run(store.name, func(t *testing.T) {
			closer := testkit.Call("page__tool", tool("controls", "toggle"))
			if store.name == "report" {
				closer = testkit.Call("page__sync", map[string]any{"path": "panel"})
			}
			p := testkit.NewProvider(api.Model{},
				round(call("controls", "page__sync", map[string]any{"path": "controls"}), call("dialogs", "page__sync", map[string]any{"path": "panel.dialogs"})),
				testkit.Call("page__tool", tool("controls", "toggle")),
				round(call("dialog", "page__sync", map[string]any{"path": "panel.dialog"}), call("body", "page__sync", map[string]any{"path": "panel.dialog.body"})),
				closer,
				store.after,
				testkit.Call("page__tool", tool("controls", "toggle")),
				round(call("close", "page__tool", tool("panel.dialog", "close")), call("body", "page__tool", tool("panel.dialog.body", "noop")), call("dialogs", "page__tool", tool("panel.dialogs", "noop"))),
				testkit.Say("done"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: board()})
			c.Send("toggle")
			c.Wait()
			results := testkit.Results(p.Request(t, 7))
			for _, r := range results[:2] {
				if got := refusedWith(t, r); got.Space == nil {
					t.Fatalf("call %s was refused with %+v, want the whole space", r.ID, got)
				}
			}
			var out toolOutput
			testkit.Decode(t, results[2], &out)
			if results[2].Error != api.ToolOk || out.Output != "nothing" {
				t.Fatalf("the sibling's noop answered %#v, want it run", results[2].Content)
			}
		})
	}
}

func TestInputIdsFollowContent(t *testing.T) {
	logged := &logs{}
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logged, nil)))
	t.Cleanup(func() { slog.SetDefault(logger) })
	redraws := doors.NewSource(0)
	button := func(desc string, attrs ...any) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Use{Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
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
	link := func(href string) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Use{Desc: "opens"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init("a"); err != nil {
					return err
				}
				if err := cur.Set("href", href); err != nil {
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
		return (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "redraw", Desc: "redraws the form", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				redraws.Mutate(ctx, func(n int) int { return n + 1 })
				return "redrawn", nil
			}}); err != nil {
				return err
			}
			for _, el := range []gox.Elem{
				button("the note", "name", "note", "value", "typed"),
				button("picks", "type", "checkbox", "name", "pick", "value", "a"),
				button("picks", "type", "checkbox", "name", "pick", "value", "b"),
				button("deletes", "type", "submit"),
				button("deletes", "type", "submit"),
				button("queries", "type", "text", "name", "q"),
				button("queries", "type", "search", "name", "q"),
				link("/a"),
				link("/b"),
				redraws.Bind(func(n int) gox.Elem {
					return func(cur gox.Cursor) error {
						if err := cur.Any(button("saves", "type", "submit", "name", "save", "value", "now")); err != nil {
							return err
						}
						return cur.Comp(neui.Data{Text: "redraws", Object: n})
					}
				}),
			} {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		})
	}
	deletes := inputID("button", "submit", "deletes", "", "")
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		testkit.Call("page__tool", map[string]any{"path": "form", "tool": "redraw", "arg": map[string]any{}}),
		testkit.Call("page__click", map[string]any{"path": "form", "id": deletes}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: form})
	c.Send("look")
	c.Wait()
	var s state
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &s)
	var got []string
	for _, in := range s.Inputs {
		got = append(got, in.Desc+" "+in.ID)
	}
	want := []string{
		"the note " + inputID("input", "text", "the note", "note", ""),
		"picks " + inputID("input", "checkbox", "picks", "pick", "a"),
		"picks " + inputID("input", "checkbox", "picks", "pick", "b"),
		"deletes " + deletes,
		"queries " + inputID("input", "text", "queries", "q", ""),
		"queries " + inputID("input", "search", "queries", "q", ""),
		"opens " + inputID("link", "link", "opens", "", "/a"),
		"opens " + inputID("link", "link", "opens", "", "/b"),
		"saves " + inputID("button", "submit", "saves", "save", "now"),
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the form synced inputs %q, want %q", got, want)
	}
	var redrawn toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &redrawn)
	if redrawn.Diff == nil || redrawn.Diff.Inputs != nil || len(redrawn.Diff.Data) != 1 {
		t.Fatalf("the redraw answered %+v, want only the data changed: the redrawn button keeps its id", redrawn.Diff)
	}
	plain(t, testkit.Results(p.Request(t, 3))[0], fmt.Sprintf(`input %q in space "form": not done: more than one input`, deletes))
	for _, want := range []string{"level=ERROR", "space=form", "id=" + deletes, "desc=deletes"} {
		if !strings.Contains(logged.String(), want) {
			t.Fatalf("the refused click logged %q, want an error with %s", logged.String(), want)
		}
	}
}

func TestStoredDigestsMatchARecount(t *testing.T) {
	var root page.Space
	var stale []string
	n := doors.NewSource(0)
	bump := func(req api.Request) testkit.Turn {
		stale = append(stale, page.StaleDigests(root)...)
		return testkit.Call("page__tool", map[string]any{"path": "s", "tool": "bump", "arg": map[string]any{}})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "s"}),
		testkit.Then(bump),
		testkit.Then(bump),
		testkit.Then(bump),
		testkit.Then(func(api.Request) testkit.Turn {
			stale = append(stale, page.StaleDigests(root)...)
			return testkit.Say("done")
		}),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		root = cur.Context().Value(common.KeySpace).(page.Space)
		return (neui.Space{Name: "s", Desc: "the space"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "bump", Desc: "bumps", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				n.Mutate(ctx, func(n int) int { return n + 1 })
				return "bumped", nil
			}}); err != nil {
				return err
			}
			return cur.Any(n.Bind(func(n int) gox.Elem {
				return func(cur gox.Cursor) error {
					if n%2 == 1 {
						return nil
					}
					for _, el := range []any{
						neui.Tool[struct{}, string]{Name: "twin", Desc: "first", Handler: func(context.Context, struct{}) (string, error) { return "first", nil }},
						neui.Tool[struct{}, string]{Name: "twin", Desc: "second", Handler: func(context.Context, struct{}) (string, error) { return "second", nil }},
						neui.Data{Text: "twice"},
						neui.Data{Text: "twice"},
						radio("size", "s", make(chan string, 1)),
						radio("size", "m", make(chan string, 1)),
						gox.Elem(func(cur gox.Cursor) error {
							return (neui.Use{Desc: "the note"}).Proxy(cur, func(cur gox.Cursor) error {
								if err := cur.InitVoid("input"); err != nil {
									return err
								}
								return cur.Submit()
							})
						}),
					} {
						if err := cur.Any(el); err != nil {
							return err
						}
					}
					return (neui.Space{Name: "c", Desc: "a child"}).Proxy(cur, func(gox.Cursor) error { return nil })
				}
			}))
		})
	}})
	c.Send("bump")
	c.Wait()
	if stale != nil {
		t.Fatalf("spaces %q stored digests that differ from a recount", stale)
	}
}

func TestPageToolDuringRelinkReportsTheRelink(t *testing.T) {
	started := make(chan struct{}, 1)
	pg := func(cur gox.Cursor) error {
		return (neui.Space{Name: "box", Desc: "the box"}).Proxy(cur, func(cur gox.Cursor) error {
			return cur.Comp(neui.Tool[struct{}, string]{Name: "hold", Desc: "holds", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				started <- struct{}{}
				<-ctx.Done()
				return "", ctx.Err()
			}})
		})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "box"}),
		testkit.Call("page__tool", map[string]any{"path": "box", "tool": "hold", "arg": map[string]any{}}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: gox.Elem(pg)})
	c.Send("hold")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the tool handler never started")
	}
	c.Relink()
	c.Wait()
	r := testkit.Results(p.Request(t, 2))[0]
	plain(t, r, "the chat moved to a new page, by a reload or in another tab, which ended what ran on the old page")
}

type describedRows struct {
	_    struct{} `description:"the rows; truncated: call again"`
	Rows []string `json:"rows"`
}

func TestLightModeShowsPageToolResultDescriptions(t *testing.T) {
	page := func(cur gox.Cursor) error {
		return (neui.Space{Name: "sheet", Desc: "a sheet"}).Proxy(cur, func(cur gox.Cursor) error {
			return cur.Comp(neui.Tool[struct{}, describedRows]{Name: "rows", Desc: "reads rows", Handler: func(context.Context, struct{}) (describedRows, error) {
				return describedRows{}, nil
			}})
		})
	}
	for _, light := range []bool{false, true} {
		p := testkit.NewProvider(api.Model{}, testkit.Call("page__sync", map[string]any{"path": "sheet"}), testkit.Say("done"))
		c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionLightMode(light)}})
		c.Send("read")
		c.Wait()
		var synced struct {
			Tools []struct{ Name, Desc string }
		}
		testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &synced)
		want := "reads rows\nOutput schema: {\"description\":\"the rows; truncated: call again\","
		if light {
			want = "reads rows\nResult: the rows; truncated: call again"
		}
		if len(synced.Tools) != 1 || !strings.HasPrefix(synced.Tools[0].Desc, want) || light && synced.Tools[0].Desc != want {
			t.Fatalf("light %v: the sync listed %+v, want the description %q", light, synced.Tools, want)
		}
	}
}
