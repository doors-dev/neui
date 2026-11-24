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

package island_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/replay"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit"
)

type started struct {
	Task   uint
	Script uint
}

type pressParams struct {
	Label string
	Value neui.IslandCall[int, int]
}

type press struct {
	params pressParams
	shown  chan<- string
	submit func(ctx context.Context, n int) bool
}

func (p press) Update(context.Context, struct{}) {}

func (p press) press(ctx context.Context) (int, error) {
	n, err := p.params.Value.Call(ctx, 21)
	if err != nil {
		return 0, err
	}
	if !p.submit(ctx, n) {
		return 0, errors.New("no draw took the press")
	}
	return n, nil
}

func (p press) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		doors.OnBatch(cur.Context(), func(context.Context) {
			select {
			case p.shown <- p.params.Label:
			default:
			}
		})
		if err := cur.Comp(neui.Tool[struct{}, int]{Name: "press", Desc: "presses the button", Handler: func(ctx context.Context, _ struct{}) (int, error) {
			return p.press(ctx)
		}}); err != nil {
			return err
		}
		return (neui.Use{Desc: p.params.Label, Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("button"); err != nil {
				return err
			}
			if err := cur.Set("name", "press"); err != nil {
				return err
			}
			if err := cur.Modify(doors.AClick{On: func(ctx context.Context, _ doors.RequestPointer) bool {
				p.press(ctx)
				return false
			}}); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Text(p.params.Label); err != nil {
				return err
			}
			return cur.Close()
		})
	}
}

func board(marks chan<- int, shown chan<- string) gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := nekit.IslandRegisterLayout(l); err != nil {
					return err
				}
				if err := l.Tool("mark", "marks a number", func(ctx context.Context, in struct{ N int }) (int, error) {
					marks <- in.N
					return in.N, nil
				}); err != nil {
					return err
				}
				return l.Control("press", "a button that answers with value(21)", func(_ context.Context, p pressParams, submit func(ctx context.Context, n int) bool) (api.Widget[struct{}], error) {
					return press{params: p, shown: shown, submit: submit}, nil
				})
			},
		})
	}
}

func revised(t *testing.T, then func(revision string) testkit.Turn) testkit.Turn {
	return testkit.Then(func(r api.Request) testkit.Turn {
		revision, _, _ := specText(t, testkit.Results(r)[0])
		return then(revision)
	})
}

func start(revision string, code string) testkit.Turn {
	return testkit.Call("page__island_begin", map[string]any{"path": "board", "revision": revision, "code": code, "comment": "the board app"})
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

func specText(t *testing.T, r api.ContentToolResult) (string, string, string) {
	t.Helper()
	if r.Error != api.ToolOk || len(r.Content) != 1 {
		t.Fatalf("island_spec answered %#v, want one text", r)
	}
	text, _ := r.Content[0].(api.ContentText)
	head, body, ok := strings.Cut(text.Text, "\n\n")
	line, _, _ := strings.Cut(head, "\n")
	revision, found := strings.CutPrefix(line, "revision ")
	if !ok || !found || !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(revision) {
		t.Fatalf("island_spec answered %q, want a revision line, a blank line and the spec", text.Text)
	}
	return revision, head, body
}

func fullBoard() gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: nekit.IslandRegisterAll,
		})
	}
}

func lastText(req api.Request) string {
	content := req.Messages[len(req.Messages)-1].Content
	if text, ok := content[len(content)-1].(api.ContentText); ok {
		return text.Text
	}
	return ""
}

func lastTexts(req api.Request) []string {
	var texts []string
	for _, content := range req.Messages[len(req.Messages)-1].Content {
		if text, ok := content.(api.ContentText); ok {
			texts = append(texts, text.Text)
		}
	}
	return texts
}

func recorded(t *testing.T, c *stack.Chat, p *testkit.Provider, want string) string {
	t.Helper()
	for {
		for _, r := range p.Requests() {
			for _, text := range lastTexts(r) {
				if strings.Contains(text, want) {
					return text
				}
			}
		}
		c.Wait()
	}
}

func TestIslandSpecListsLibrary(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Events(
			api.EventStart{ID: "spec", Model: "fake"},
			api.EventTool{Index: 0, ID: "tree", Name: "page__tree", Args: `{}`},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "sync", Name: "page__sync", Args: `{"path":"board"}`},
			api.EventClose{Index: 1},
			api.EventTool{Index: 2, ID: "spec-whole", Name: "page__island_spec", Args: `{"path":"board"}`},
			api.EventClose{Index: 2},
			api.EventTool{Index: 3, ID: "spec-page-2", Name: "page__island_spec", Args: `{"path":"board","page":2}`},
			api.EventClose{Index: 3},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 4 {
		t.Fatalf("the spec round answered %d calls, want 4", len(results))
	}
	var tree struct {
		Children []struct {
			Name   string
			Island bool
		}
	}
	testkit.Decode(t, results[0], &tree)
	if len(tree.Children) != 1 || tree.Children[0].Name != "board" || !tree.Children[0].Island {
		t.Fatalf("tree shows %+v, want board as an island", tree)
	}
	var synced struct {
		Path   string
		Island bool
	}
	testkit.Decode(t, results[1], &synced)
	if synced.Path != "board" || !synced.Island {
		t.Fatalf("sync shows %+v, want board as an island", synced)
	}
	results = results[2:]
	revision, head, body := specText(t, results[0])
	if head != "revision "+revision {
		t.Fatalf("island_spec opens with %q, want the revision alone: the spec fits one result", head)
	}
	for _, name := range []string{"mark", "press", "column", "row", "grid"} {
		if !strings.Contains(body, name) {
			t.Fatalf("the spec does not list %s:\n%s", name, body)
		}
	}
	refused(t, results[1], "page 2, the spec has 1 in all")
}

func TestIslandSpecSchemaKeepsDefinitionsAtTheEnd(t *testing.T) {
	var catalog string
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			revision, _, spec := specText(t, testkit.Results(r)[0])
			catalog = spec
			return testkit.Call("page__island_spec_schema", map[string]any{"path": "board", "revision": revision, "items": []string{"ui.form", "ui.table", "ui.markdown"}})
		}),
		testkit.Call("page__island_spec_schema", map[string]any{"path": "board", "revision": "0000000000000000", "items": []string{"ui.form"}}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	if strings.Contains(catalog, "\n  params ") || !strings.Contains(catalog, "\nui.form  ") {
		t.Fatalf("the catalog carries schemas or misses ui.form:\n%s", catalog)
	}
	r := testkit.Results(p.Request(t, 2))[0]
	if r.Error != api.ToolOk || len(r.Content) != 1 {
		t.Fatalf("island_spec_schema answered %#v", r)
	}
	schema := r.Content[0].(api.ContentText).Text
	head, defs, ok := strings.Cut(schema, "\nDefinitions\n")
	if !ok || strings.Contains(head, `"definitions"`) || !strings.HasPrefix(head, "ui.form\n") || !strings.Contains(head, "\nui.markdown\n") {
		t.Fatalf("the schema has definitions outside its Definitions section or misses an item:\n%s", schema)
	}
	for _, ref := range strings.Split(schema, `"$ref":"#/definitions/`)[1:] {
		name, _, _ := strings.Cut(ref, `"`)
		if !strings.Contains("\n"+defs, "\n  "+name+" ") {
			t.Fatalf("%s is not under Definitions:\n%s", name, schema)
		}
	}
	refused(t, testkit.Results(p.Request(t, 3))[0], "read island_spec again")
	t.Logf("catalog %d bytes, schema of ui.form, ui.table and ui.markdown %d bytes", len(catalog), len(schema))
}

func TestIslandSpecComesInPages(t *testing.T) {
	wide := func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				for n := range 12 {
					if err := l.Tool(fmt.Sprint("mark", n), strings.Repeat("marks a number ", 20), func(_ context.Context, in struct{ N int }) (int, error) {
						return in.N, nil
					}); err != nil {
						return err
					}
				}
				return nil
			},
		})
	}
	full := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: full, Page: wide})
	c.Send("open the board")
	c.Wait()
	_, _, whole := specText(t, testkit.Results(full.Request(t, 1))[0])
	model := api.Model{Name: "fake", Context: 6_250}
	limit := replay.RoundItemBudget(common.ConfigDefaults(api.ThreadConfig{}), model) / 2
	pages := 0
	first := ""
	var p *testkit.Provider
	p = testkit.NewProvider(model,
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			revision, head, _ := specText(t, testkit.Results(r)[0])
			first = revision
			fmt.Sscanf(head, "revision %s\npage 1 of %d", new(string), &pages)
			turns := make([]testkit.Turn, 0, pages+1)
			for page := 2; page <= pages+1; page++ {
				turns = append(turns, testkit.Events(
					api.EventStart{ID: "pages", Model: "fake"},
					api.EventTool{Index: 0, ID: fmt.Sprint("spec-page-", page), Name: "page__island_spec", Args: fmt.Sprintf(`{"path":"board","page":%d}`, page)},
					api.EventClose{Index: 0},
					api.EventOk{Code: api.CodeTurn},
				))
			}
			p.Push(append(turns[1:], start(revision, `var started = true`), testkit.Say("done"))...)
			return turns[0]
		}),
	)
	c = stack.Page(t, stack.Host{Provider: p, Page: wide})
	c.Send("open the board")
	c.Wait()
	if pages < 3 {
		t.Fatalf("a %d-byte spec under a %d-byte page size came in %d pages, want 3 or more", len(whole), limit, pages)
	}
	var results []api.ContentToolResult
	for i := 1; i <= pages+1; i++ {
		results = append(results, testkit.Results(p.Request(t, i))...)
	}
	if len(results) != pages+1 {
		t.Fatalf("%d pages read, want %d and one past the end", len(results), pages+1)
	}
	refused(t, results[pages], fmt.Sprintf("page %d, the spec has %d in all: no such page of the spec", pages+1, pages))
	joined := ""
	for i, r := range results[:pages] {
		revision, head, body := specText(t, r)
		if revision != first {
			t.Fatalf("page %d carries revision %s, page 1 %s, want every page to carry the whole spec's", i+1, revision, first)
		}
		want := fmt.Sprintf("revision %s\npage %d of %d: call island_spec with page %d for the rest", revision, i+1, pages, i+2)
		if i == pages-1 {
			want = fmt.Sprintf("revision %s\npage %d of %d", revision, pages, pages)
		}
		if head != want {
			t.Fatalf("page %d opens with %q, want %q", i+1, head, want)
		}
		if size := len(head) + 2 + len(body); size > limit || strings.Contains(body, "<truncated>") {
			t.Fatalf("page %d takes %d bytes, over the %d-byte page size", i+1, size, limit)
		}
		if !strings.HasSuffix(body, "\n") {
			t.Fatalf("page %d ends inside a line:\n%s", i+1, body)
		}
		joined += body
	}
	if joined != whole {
		t.Fatalf("the pages joined differ from the whole spec:\n%s", joined)
	}
	if r := testkit.Results(p.Request(t, pages+2))[0]; r.Error != api.ToolOk {
		t.Fatalf("island_begin with the pages' revision answered %#v, want it accepted", r.Content)
	}
}

func TestIslandSpecItemOverPageRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 1_000},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	refused(t, testkit.Results(p.Request(t, 1))[0], "logs and files takes")
	refused(t, testkit.Results(p.Request(t, 1))[0], "a page holds")
	refused(t, testkit.Results(p.Request(t, 1))[0], "larger than a page can hold")
}

func TestIslandBeginReturnsTask(t *testing.T) {
	marks := make(chan int, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.notify({marked: app.mark({N: 3})})`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(marks, nil)})
	c.Send("open the board")
	events := c.Wait()
	if !slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("the start reported no task: %v", stack.Trace(events))
	}
	var s started
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &s)
	if s != (started{Task: 1, Script: 1}) {
		t.Fatalf("island_begin answered %+v, want task 1 script 1", s)
	}
	select {
	case n := <-marks:
		if n != 3 {
			t.Fatalf("the script marked %d, want 3", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the script never called app.mark")
	}
	notify := recorded(t, c, p, `kind="notifications_list"`)
	for _, want := range []string{`<task_notify `, `path="board"`, `script="1"`, `task_id="1"`, `[{"marked":3}]`} {
		if !strings.Contains(notify, want) {
			t.Fatalf("the script's end notified %q, want it to carry %s", notify, want)
		}
	}
}

func TestIslandAppendQueuesBehindTheRunningPart(t *testing.T) {
	held := make(chan struct{}, 1)
	release := make(chan struct{})
	page := func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				return l.Tool("hold", "holds until released", func(ctx context.Context, _ struct{}) (int, error) {
					held <- struct{}{}
					select {
					case <-release:
						return 0, nil
					case <-ctx.Done():
						return 0, context.Cause(ctx)
					}
				})
			},
		})
	}
	appended := func(code string) testkit.Turn {
		return testkit.Call("page__island_append", map[string]any{"path": "board", "code": code})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var order = [1]; app.hold({}); order.push(2)`)
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			select {
			case <-held:
			case <-time.After(5 * time.Second):
				t.Error("the first part never reached app.hold")
			}
			return appended(`order.push(3)`)
		}),
		testkit.Then(func(api.Request) testkit.Turn { return appended(`order.push(4); ui.notify(order)`) }),
		testkit.Then(func(api.Request) testkit.Turn {
			close(release)
			return testkit.Say("appended")
		}),
		testkit.Say("noted"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("open the board")
	c.Wait()
	for i, want := range []started{{Task: 1, Script: 2}, {Task: 1, Script: 3}} {
		var s started
		testkit.Decode(t, testkit.Results(p.Request(t, 3+i))[0], &s)
		if s != want {
			t.Fatalf("island_append %d answered %+v while the first part held, want %+v", i+1, s, want)
		}
	}
	notify := recorded(t, c, p, `kind="notifications_list"`)
	for _, want := range []string{`script="3"`, `>[[1,2,3,4]]</task_notify>`} {
		if !strings.Contains(notify, want) {
			t.Fatalf("the appended parts notified %q, want them run in order after the first part, carrying %s", notify, want)
		}
	}
}

func TestIslandScriptThrowReportsError(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `throw new Error("boom")`)
		}),
		testkit.Say("started"),
		testkit.Say("seen"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	record := recorded(t, c, p, `kind="script_error"`)
	for _, want := range []string{`script="1"`, "boom", "send what should still run with island_append", "nothing has been drawn yet"} {
		if !strings.Contains(record, want) {
			t.Fatalf("the throw reported %q, want it to carry %s", record, want)
		}
	}
	if strings.Contains(record, "last_action") {
		t.Fatalf("a throw before any user action reported %q, want no last_action", record)
	}
}

func TestIslandPartWithoutDrawReportsTheEmptyScreen(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var loaded = 1`)
		}),
		testkit.Say("started"),
		testkit.Say("seen"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	record := recorded(t, c, p, `kind="script_note"`)
	if !strings.Contains(record, "has not drawn yet, so the user does not see it") {
		t.Fatalf("a part that ended without drawing reported %q, want the empty-screen note", record)
	}
}

func TestIslandHostErrorReachesScriptInFull(t *testing.T) {
	long := strings.Repeat("x", 300) + "END"
	page := func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := l.Tool("fail", "fails", func(context.Context, struct{}) (struct{}, error) {
					return struct{}{}, errors.New(long)
				}); err != nil {
					return err
				}
				return l.View("wall", "fails to build", func(context.Context, struct{}) (api.Widget[struct{}], error) {
					return nil, errors.New(long)
				})
			},
		})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `const seen = []
for (const f of [() => ui.wall({}), () => app.fail({})]) {
	try { f() } catch (e) { seen.push(e.name + " " + e.message) }
}
ui.notify(seen)`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("open the board")
	c.Wait()
	notify := recorded(t, c, p, `kind="notifications_list"`)
	for _, want := range []string{`"TypeError ui.wall: ` + long + `"`, `"TypeError ` + long + `"`} {
		if !strings.Contains(notify, want) {
			t.Fatalf("the script saw %q, want it to carry %s", notify, want)
		}
	}
}

func TestIslandBeginStaleRevisionRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(strings.Repeat("0", 16), `ui.notify(1)`)
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	events := c.Wait()
	refused(t, testkit.Results(p.Request(t, 2))[0], "read island_spec again")
	if slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("a refused start reported a task: %v", stack.Trace(events))
	}
}

func TestIslandBeginSyntaxErrorRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var (`)
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	events := c.Wait()
	refused(t, testkit.Results(p.Request(t, 2))[0], "SyntaxError")
	if slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("a refused start reported a task: %v", stack.Trace(events))
	}
}

func TestIslandBeginRefusesWhatPrecheckFinds(t *testing.T) {
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"empty screen", "var f = ui.form({ fields: [{ type: \"text\", label: \"Name\", name: \"name\" }] })\nvar action = ui.draw()",
			[]string{"line 2, column 14: TypeError: ui.draw(): the screen is empty, as no part sets ui.root"}},
		{"id literal and output field", "var f = ui.form({ fields: [{ type: \"number\", label: \"Qty\", name: \"new_qty\" }] })\nui.root.set(f)\nvar action = ui.draw()\nif (action.id === \"submit\") ui.notify(action.output.new_qty)",
			[]string{
				"line 4, column 5: TypeError: action.id: a control's id is a number set when it is created, never a literal such as \"submit\": compare it with that control, such as form.id",
				"line 4, column 39: TypeError: action.output.new_qty: no control's output has new_qty; the outputs have only data, row, rows, submit, value",
			}},
		{"field type", "ui.root.set(ui.form({ fields: [{ type: \"hidden\", name: \"id\" }] }))",
			[]string{"ui.form params at '/fields/0/type': value must be one of 'text', 'email', 'tel', 'url', 'password', 'number', 'range', 'date', 'time', 'datetime-local', 'month', 'week', 'textarea', 'select', 'search', 'checkbox', 'file', 'group', 'list', 'series'"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, c.code)
				}),
				testkit.Say("done"),
			)
			chat := stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
			chat.Send("open the board")
			chat.Wait()
			for _, want := range c.want {
				refused(t, testkit.Results(p.Request(t, 2))[0], want)
			}
		})
	}
}

func TestIslandBeginRefusedAfterSpecChange(t *testing.T) {
	mode := doors.NewSource(0)
	page := func(cur gox.Cursor) error {
		if err := cur.Comp(neui.Tool[struct{}, string]{Name: "next", Desc: "changes the board", Handler: func(ctx context.Context, _ struct{}) (string, error) {
			mode.Mutate(ctx, func(m int) int { return m + 1 })
			return "changed", nil
		}}); err != nil {
			return err
		}
		return cur.Any(mode.Bind(func(m int) gox.Elem {
			return func(cur gox.Cursor) error {
				if m == 2 {
					return nil
				}
				return cur.Comp(neui.Island{
					Name:        "board",
					Desc:        "a board",
					Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
					Template: func(slot gox.Comp) gox.Elem {
						return func(cur gox.Cursor) error { return cur.Comp(slot) }
					},
					Library: func(l neui.Library) error {
						if m == 0 {
							return nil
						}
						return l.Tool("mark", "marks a number", func(_ context.Context, in struct{ N int }) (int, error) {
							return in.N, nil
						})
					},
				})
			}
		}))
	}
	next := map[string]any{"path": "", "tool": "next", "arg": map[string]any{}}
	var first string
	p := testkit.NewProvider(api.Model{},
		testkit.Events(
			api.EventStart{ID: "look", Model: "fake"},
			api.EventTool{Index: 0, ID: "sync", Name: "page__sync", Args: `{"path":""}`},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "spec", Name: "page__island_spec", Args: `{"path":"board"}`},
			api.EventClose{Index: 1},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Then(func(r api.Request) testkit.Turn {
			first, _, _ = specText(t, testkit.Results(r)[1])
			return testkit.Call("page__tool", next)
		}),
		testkit.Then(func(api.Request) testkit.Turn { return start(first, `var x = 1`) }),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			if revision == first {
				t.Errorf("the extended library kept revision %s", revision)
			}
			return testkit.Call("page__tool", next)
		}),
		testkit.Then(func(api.Request) testkit.Turn { return start(first, `var x = 1`) }),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("open the board")
	events := c.Wait()
	refused(t, testkit.Results(p.Request(t, 3))[0], "read island_spec again")
	refused(t, testkit.Results(p.Request(t, 6))[0], "no space ")
	if slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("a refused start reported a task: %v", stack.Trace(events))
	}
}

func TestIslandScriptHasNoPageOrJSTools(t *testing.T) {
	code := `ui.notify([typeof page__sync, typeof page__tool, typeof page__click, typeof js__run])`
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, code)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(make(chan int, 1), nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionModule(js.Module)}})
	c.Send("probe the board")
	c.Wait()
	if notify := recorded(t, c, p, `kind="notifications_list"`); !strings.Contains(notify, `[["undefined","undefined","undefined","undefined"]]`) {
		t.Fatalf("the island script saw %q, want no page or js tools", notify)
	}
}

func TestIslandSpecListsScriptTools(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("read"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(make(chan int, 1), nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionModule(js.Module), common.OptionModule(timemod.Module)}})
	c.Send("read the board")
	c.Wait()
	_, _, spec := specText(t, testkit.Results(p.Request(t, 1))[0])
	if !strings.Contains(spec, "\nYour tools\nThe script has time__*.\n") || strings.Contains(spec, "page__*") || strings.Contains(spec, "js__*") {
		t.Fatalf("the spec lists the script's tools as:\n%s", spec)
	}
}

func TestIslandAppFunctionIsATool(t *testing.T) {
	marks := make(chan int, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "board.app"}),
		testkit.Call("page__tool", map[string]any{"path": "board.app", "tool": "mark", "arg": map[string]any{"N": 7}}),
		testkit.Say("marked"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(marks, nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("mark 7")
	c.Wait()
	var out struct{ Output int }
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &out)
	if out.Output != 7 || <-marks != 7 {
		t.Fatalf("the agent's mark returned %d, want the island's app function to run as a tool of its space", out.Output)
	}
}

func TestIslandAppFunctionNeedsARead(t *testing.T) {
	marks := make(chan int, 2)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__tool", map[string]any{"path": "board.app", "tool": "mark", "arg": map[string]any{"N": 7}}),
		testkit.Call("page__sync", map[string]any{"path": "board.app"}),
		testkit.Call("page__tool", map[string]any{"path": "board.app", "tool": "mark", "arg": map[string]any{"N": 7}}),
		testkit.Say("marked"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(marks, nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("mark 7")
	c.Wait()
	refused(t, testkit.Results(p.Request(t, 1))[0], "not done: this space had not been read: sync it, then call again")
	if len(marks) != 1 {
		t.Fatalf("mark ran %d times, want once: the refused call ran too", len(marks))
	}
	var out struct{ Output int }
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &out)
	if out.Output != 7 || <-marks != 7 {
		t.Fatalf("the call after the sync answered %d, want mark 7 to run", out.Output)
	}
}

func endBoard(held chan<- struct{}, screens chan<- string, success bool) gox.Elem {
	shown := func(label string) gox.Elem {
		return func(cur gox.Cursor) error {
			select {
			case screens <- label:
			default:
			}
			return cur.Text(label)
		}
	}
	return func(cur gox.Cursor) error {
		island := neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: shown("idle"),
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				return l.Tool("hold", "holds until the app ends", func(ctx context.Context, _ struct{}) (int, error) {
					held <- struct{}{}
					<-ctx.Done()
					return 0, context.Cause(ctx)
				})
			},
		}
		if success {
			island.Success = func(func(context.Context)) gox.Elem { return shown("done") }
		}
		return cur.Comp(island)
	}
}

type endView struct {
	result string
	screen string
	sync   api.ContentToolResult
}

func syncOnEnd(screens <-chan string, ended chan<- endView) testkit.Turn {
	var view endView
	return testkit.Then(func(r api.Request) testkit.Turn {
		if results := testkit.Results(r); len(results) == 1 && results[0].ID == "call-page__sync" {
			view.sync = results[0]
			ended <- view
			return testkit.Say("seen")
		}
		if text := lastText(r); strings.Contains(text, "<task_result ") {
			view.result = text
			select {
			case view.screen = <-screens:
			default:
			}
			return testkit.Call("page__sync", map[string]any{"path": "board.screen"})
		}
		return testkit.Say("noted")
	})
}

func endedAs(t *testing.T, ended <-chan endView, success bool, body string) {
	t.Helper()
	var view endView
	select {
	case view = <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("no sync followed the app's task_result")
	}
	if !strings.Contains(view.result, `task_id="1"`) || !strings.HasSuffix(view.result, ">"+body+"</task_result>") {
		t.Fatalf("the app's end reported %q, want task 1 with %s", view.result, body)
	}
	want, status := "idle", "no app is running in this island"
	if success {
		want, status = "done", "the island's app, task 1, has ended and the island shows its end screen"
	}
	if view.screen != want {
		t.Fatalf("the island showed %q when the task_result reached the agent, want %q", view.screen, want)
	}
	if got := statusOf(t, view.sync); !strings.HasPrefix(got, status) {
		t.Fatalf("a sync right after the task_result shows the status %q, want %q", got, status)
	}
}

func TestIslandUIEndEndsAppAsIslandEndDoes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		success bool
		code    string
		body    string
	}{
		{"notified with success", true, `ui.notify({kept: 1}); ui.notify("two"); ui.end()`, `{"ended":"the island's app ended normally: its script called ui.end(). If you did not expect it now, the user most likely ended it, for example with one of the app's buttons","notifications":[{"kept":1},"two"]}`},
		{"silent with placeholder", false, `ui.end()`, `{"ended":"the island's app ended normally: its script called ui.end(). If you did not expect it now, the user most likely ended it, for example with one of the app's buttons"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screens := make(chan string, 8)
			ended := make(chan endView, 1)
			reply := syncOnEnd(screens, ended)
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, tc.code)
				}),
				reply,
				reply,
				reply,
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: endBoard(make(chan struct{}, 1), screens, tc.success)})
			for len(screens) > 0 {
				<-screens
			}
			c.Send("open the board")
			endedAs(t, ended, tc.success, tc.body)
			for i, r := range p.Requests() {
				for _, text := range lastTexts(r) {
					if strings.Contains(text, "<task_notify ") {
						t.Fatalf("request %d carries %q, want the values notified before ui.end() in its task_result only", i, text)
					}
				}
			}
		})
	}
}

func TestIslandEndEndsAppWithItsOwnResult(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprintf("success %v", success), func(t *testing.T) {
			held := make(chan struct{}, 1)
			screens := make(chan string, 8)
			ended := make(chan endView, 1)
			reply := syncOnEnd(screens, ended)
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, `ui.notify("dropped"); app.hold({})`)
				}),
				testkit.Then(func(api.Request) testkit.Turn {
					select {
					case <-held:
					case <-time.After(5 * time.Second):
						t.Error("the script never reached app.hold")
					}
					return testkit.Call("page__island_end", map[string]any{"path": "board"})
				}),
				reply,
				reply,
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: endBoard(held, screens, success)})
			for len(screens) > 0 {
				<-screens
			}
			c.Send("close the board")
			endedAs(t, ended, success, `"the island's app was ended with island_end"`)
			if r := testkit.Results(p.Request(t, 3))[0]; r.ID != "call-page__island_end" || r.Error != api.ToolOk {
				t.Fatalf("island_end answered %#v, want it to succeed", r)
			}
			for i, r := range p.Requests() {
				for _, text := range lastTexts(r) {
					if strings.Contains(text, "dropped") {
						t.Fatalf("request %d carries %q, want the values queued before island_end dropped", i, text)
					}
				}
			}
		})
	}
}

func TestIslandRelinkEndsAppWithTheValuesSinceItsLastDraw(t *testing.T) {
	const relinked = "the chat moved to a new page, by a reload or in another tab, which ended the app; begin a new one with island_begin if it is still needed"
	const queued = `ui.root.set(ui.column({})); ui.notify(2); app.mark({N: 1}); ui.draw()`
	relink := func(c *stack.Chat, _ api.Task) { c.Relink() }
	for _, tc := range []struct {
		name string
		code string
		end  func(c *stack.Chat, task api.Task)
		want string
	}{
		{"relinked while the draw waits", `ui.root.set(ui.column({})); app.mark({N: 1}); ui.draw()`, relink,
			`<task_result comment="the board app" error="` + relinked + `" log="board.island.log" module="page" name="island_begin" path="board" task_id="1">null</task_result>`},
		{"relinked with values notified since the last draw", queued, relink,
			`<task_result comment="the board app" error="` + relinked + `" log="board.island.log" module="page" name="island_begin" path="board" task_id="1">{"notifications":[2]}</task_result>`},
		{"canceled by the user with values notified since the last draw", queued, func(_ *stack.Chat, task api.Task) { task.Cancel(api.ErrCanceledByUser) },
			`<task_result comment="the board app" error="canceled by user" log="board.island.log" module="page" name="island_begin" path="board" task_id="1">null</task_result>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marks := make(chan int, 1)
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, tc.code)
				}),
				testkit.Say("started"),
				testkit.Say("noted"),
				testkit.Say("ended"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: board(marks, nil)})
			c.Send("open the board")
			var task api.Task
			for _, e := range c.Wait() {
				if e, ok := e.(api.ChatEventTasks); ok && len(e.Tasks) == 1 {
					task = e.Tasks[0]
				}
			}
			if task.Cancel == nil {
				t.Fatal("the start listed no task")
			}
			select {
			case <-marks:
			case <-time.After(5 * time.Second):
				t.Fatal("the script never reached its draw")
			}
			tc.end(c, task)
			if got := recorded(t, c, p, "<task_result "); got != tc.want {
				t.Fatalf("the app's end reached the agent as\n%s\nwant\n%s", got, tc.want)
			}
			for i, r := range p.Requests() {
				for _, text := range lastTexts(r) {
					if strings.Contains(text, "<task_notify ") && strings.Contains(text, ">[2]</task_notify>") {
						t.Fatalf("request %d carries %q, want the values queued since the last draw in the task_result or dropped", i, text)
					}
				}
			}
		})
	}
}

func TestIslandGuideIsReadWithIslandGuide(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_guide", map[string]any{}),
		testkit.Call("js__run", map[string]any{"code": `try { return page__island_guide({}) } catch (e) { return e.message }`, "comment": "reads the guide"}),
		testkit.Say("done"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(make(chan int, 1), nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionModule(js.Module), common.OptionModule(timemod.Module)}})
	c.Send("open the board")
	c.Wait()
	system := p.Request(t, 0).System
	if !strings.Contains(system, "\n\nIslands\n\nAn island is a space where you run your own app for the user, ") || !strings.Contains(system, "Before your first app, read island_guide. Before starting an app in an island, read its spec with island_spec and the schemas of the items you use with island_spec_schema; ") {
		t.Fatalf("the page module does not introduce islands and send the agent to island_guide and island_spec:\n%s", system)
	}
	headings := []string{"How an app runs", "The screen", "Views and controls", "App functions and your tools", "The typical loop", "Reading the reference"}
	for _, text := range append([]string{"What to build", "ui.draw", "ui.root", "island_append"}, headings...) {
		if strings.Contains(system, text) {
			t.Fatalf("the page module's system text carries the guide's %q:\n%s", text, system)
		}
	}
	r := testkit.Results(p.Request(t, 1))[0]
	if r.Error != api.ToolOk || len(r.Content) != 1 {
		t.Fatalf("island_guide answered %#v, want one text", r)
	}
	guide := r.Content[0].(api.ContentText).Text
	if !strings.HasPrefix(guide, "What to build\nBuild what the request needs, ") || !strings.HasSuffix(guide, "assigning over an inherited property such as err.name throws.\n") {
		t.Fatalf("island_guide does not answer with the guide alone, from what to build to reading the reference:\n%s", guide)
	}
	last := 0
	for _, heading := range headings {
		at := strings.Index(guide, "\n\n"+heading+"\n")
		if at < last {
			t.Fatalf("the guide has no %q section after the one before it:\n%s", heading, guide)
		}
		last = at
	}
	for _, text := range []string{"revision ", "Logs and files", "board.island", "time__*", "the script has", "below"} {
		if strings.Contains(guide, text) {
			t.Fatalf("the guide carries %q, which belongs to an island's spec:\n%s", text, guide)
		}
	}
	var script struct{ Result string }
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &script)
	if script.Result != "island tools are not accessible from a script" {
		t.Fatalf("a script's island_guide answered %q, want it refused", script.Result)
	}
	t.Logf("system text: %d bytes; island_guide: %d bytes", len(system), len(guide))
}

func TestIslandSpecLeavesTheGuideToIslandGuide(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	page := func(cur gox.Cursor) error { return cur.Any(board(make(chan int, 1), nil)) }
	c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionModule(js.Module), common.OptionModule(timemod.Module)}})
	c.Send("open the board")
	c.Wait()
	_, _, spec := specText(t, testkit.Results(p.Request(t, 1))[0])
	own, catalog, ok := strings.Cut(spec, "\n\nControls: c = ui.<name>(params); c.update(patch)\n")
	if !ok || !strings.HasPrefix(catalog, "ui.press  a button that answers with value(21)\n\nLayouts: ui.<name>(params, ...children)\n") || !strings.HasSuffix(catalog, "\nApp functions: app.<name>(input) -> result\napp.mark  marks a number\n") {
		t.Fatalf("the spec does not end with the island's catalog:\n%s", spec)
	}
	if want := "Read island_guide first if you have not in this conversation.\n\nYour tools\nThe script has time__*.\n\nLogs and files\nconsole.log, info, warn and error append to the workspace file board.island.log, which is emptied when an app starts. Views and controls keep their files, such as uploads, under the workspace dir board.island, also emptied when an app starts: copy the files you want to keep elsewhere in the workspace."; own != want {
		t.Fatalf("before its catalog the spec reads:\n%s\nwant:\n%s", own, want)
	}
	for _, heading := range []string{"What to build", "How an app runs", "The screen", "Views and controls", "App functions and your tools", "The typical loop", "Reading the reference"} {
		if strings.Contains("\n"+spec, "\n"+heading+"\n") {
			t.Fatalf("the spec still carries the guide's %q section:\n%s", heading, spec)
		}
	}
	for _, text := range []string{"ui.draw", "ui.root", "island_append", "Controls are what the user acts on", "Views show something", "Layouts arrange content", "Limits"} {
		if strings.Contains(spec, text) {
			t.Fatalf("the spec still carries %q, which the guide says:\n%s", text, spec)
		}
	}
	t.Logf("island_spec of the board: %d bytes", len(spec))
}
