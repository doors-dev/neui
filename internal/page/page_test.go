package page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/zeebo/xxh3"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/page"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/thread"
)

type state struct {
	Path   string
	Island bool
	Tools  []struct{ Name string }
	Inputs []struct {
		ID     uint
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
		Added   []struct{ ID uint }
		Removed []uint
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

type refusal struct {
	Refused string
	Diff    *diff
	Space   *state
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
	if r.Error == api.ToolOk {
		t.Fatalf("call %s succeeded with %#v, want it refused", r.ID, r.Content)
	}
	var out refusal
	testkit.Decode(t, r, &out)
	if !strings.HasPrefix(out.Refused, "not done") {
		t.Fatalf("call %s failed with %#v, want a refusal", r.ID, r.Content)
	}
	return out
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

func TestTreeShowsSpacesOnly(t *testing.T) {
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
		`{"name":"","desc":"","children":[{"name":"cart","desc":"the cart"},{"name":"notes","desc":"the notes"}]}`,
		`{"name":"cart","desc":"the cart"}`,
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
	logged := &logs{}
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logged, nil)))
	t.Cleanup(func() { slog.SetDefault(logger) })
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__tree", map[string]any{}), testkit.Say("done"))
	c := stack.Page(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		return (neui.Space{Name: "!!!", Desc: "odd"}).Proxy(cur, func(cur gox.Cursor) error { return nil })
	}})
	c.Send("look")
	c.Wait()
	var root struct{ Children []struct{ Name string } }
	testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &root)
	name := fmt.Sprintf("space_%016x", xxh3.HashString("!!!"))
	if len(root.Children) != 1 || root.Children[0].Name != name {
		t.Fatalf("a space named !!! shows as %+v, want it named from its hash", root.Children)
	}
	if !strings.Contains(logged.String(), "level=WARN") || !strings.Contains(logged.String(), "as="+name) {
		t.Fatalf("the naming logged %q, want a warning with the hash name", logged.String())
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

func TestReadIsChecked(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("page__read", map[string]any{"path": "cart", "id": 1}), testkit.Say("done"))
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
	if restocked.Output != "restocked" || restocked.Diff != nil || string(restocked.Changes) != `[{"path":"cart","changed":["data"]}]` {
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
	if out.Output != "checking out" || out.Diff != nil || out.Changes != nil {
		t.Fatalf("the checkout after a change inside cart answered %+v, want it run with nothing changed", out)
	}
}

func TestInputRightsRefused(t *testing.T) {
	inputs := map[string]uint{}
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
	ids := map[uint]bool{}
	for _, in := range s.Inputs {
		got = append(got, in.Kind+" "+in.Desc)
		ids[in.ID] = true
	}
	want := []string{"radio size s", "radio size m", "radio size l"}
	if fmt.Sprint(got) != fmt.Sprint(want) || len(ids) != 3 {
		t.Fatalf("the radios synced as %q, want %q with distinct ids", got, want)
	}
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
		if err := space(cur, "counter", "the counter", count.Bind(func(n int) gox.Elem {
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
		return space(cur, "panel", "the panel", neui.Tool[struct{}, string]{Name: "toggle", Desc: "toggles the dialog", Handler: toggle}, open.Bind(func(o bool) gox.Elem {
			return func(cur gox.Cursor) error {
				if !o {
					return nil
				}
				return space(cur, "dialog", "the dialog", neui.Tool[struct{}, string]{Name: "close", Desc: "closes the dialog", Handler: func(ctx context.Context, _ struct{}) (string, error) {
					open.Mutate(ctx, func(bool) bool { return false })
					return "closed", nil
				}}, gox.Elem(func(cur gox.Cursor) error {
					if err := space(cur, "body", "the body"); err != nil {
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
	if toggled.Diff != nil || string(toggled.Changes) != `[{"path":"counter","changed":["data"]},{"path":"list.row","changed":["data"]},{"path":"panel","changed":["children"]},{"path":"panel.dialog","added":true,"desc":"the dialog"}]` {
		t.Fatalf("the toggle answered diff %+v and changes %s", toggled.Diff, toggled.Changes)
	}
	closed := output(4)
	if closed.Diff == nil || !closed.Diff.Removed || string(closed.Changes) != `[{"path":"panel","changed":["children"]}]` {
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

func TestScriptsReadForThemselves(t *testing.T) {
	add := map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}
	script := func(code string) testkit.Turn {
		return testkit.Call("js__run", map[string]any{"code": code, "comment": "adds"})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		script(`const out = []
try { page__tool({path: "cart", tool: "add", arg: {}}) } catch (e) { out.push(Object.keys(e.value).join(",")) }
out.push(page__tool({path: "cart", tool: "add", arg: {}}).output)
return out`),
		testkit.Call("page__tool", add),
		script(`try { page__tool({path: "cart", tool: "add", arg: {}}) } catch (e) { return [e.message, Object.keys(e.value).join(",")] }`),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop(), Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("add from a script")
	c.Wait()
	var first struct{ Result []string }
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &first)
	if fmt.Sprint(first.Result) != "[refused,space added]" {
		t.Fatalf("the script saw %q, want its first add refused with the space and its second run", first.Result)
	}
	if r := refusedWith(t, testkit.Results(p.Request(t, 3))[0]); r.Space == nil {
		t.Fatalf("the agent's add after the script's read was refused with %+v, want the whole space", r)
	}
	var second struct{ Result []string }
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &second)
	if len(second.Result) != 2 || !strings.HasPrefix(second.Result[0], "tool: not done") || second.Result[1] != "refused,space" {
		t.Fatalf("a second script saw %q, want the add refused with the space", second.Result)
	}
}

func TestRewindClearsReads(t *testing.T) {
	add := map[string]any{"path": "cart", "tool": "add", "arg": map[string]any{}}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Say("synced"),
		testkit.Say("later"),
		testkit.Call("page__tool", add),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: shop()})
	c.Send("look")
	c.Wait()
	c.Send("wait")
	second := c.Wait()[0].(api.ChatEventRoundStart).Round.ID
	c.Rewind(second, "add")
	c.Wait()
	if r := refusedWith(t, testkit.Results(p.Request(t, 4))[0]); r.Space == nil {
		t.Fatalf("the add after the rewind was refused with %+v, want the whole space", r)
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
		if r := refusedWith(t, result(i)); r.Diff != nil || r.Space != nil || !strings.Contains(r.Refused, "too large") {
			t.Fatalf("dump %d after the unread grow was refused with %+v, want the reason alone", i-3, r)
		}
	}
	refused(t, result(6), "too large for a result")
	var scripted struct{ Result int }
	testkit.Decode(t, result(7), &scripted)
	if scripted.Result != 3000 {
		t.Fatalf("the script's dump answered %d bytes, want 3000", scripted.Result)
	}
}
