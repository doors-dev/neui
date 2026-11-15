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
	"neui.dev/internal/replay"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
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
		return (neui.Input{Desc: p.params.Label, Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
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
				if err := neisland.RegisterLayout(l); err != nil {
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
	return testkit.Call("page__island_start", map[string]any{"path": "board", "revision": revision, "code": code, "comment": "the board app"})
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
			Library: neisland.RegisterAll,
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
			api.EventTool{Index: 3, ID: "spec-part-2", Name: "page__island_spec", Args: `{"path":"board","part":2}`},
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
	refused(t, results[1], "part 2, the spec has 1 in all")
}

func TestIslandSpecKeepsDefinitionsAtTheEnd(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	_, _, spec := specText(t, testkit.Results(p.Request(t, 1))[0])
	head, defs, ok := strings.Cut(spec, "\nDefinitions\n")
	if !ok || strings.Contains(head, `"definitions"`) {
		t.Fatalf("the spec has definitions outside its Definitions section:\n%s", spec)
	}
	_, body, _ := strings.Cut(spec, "\nRuntime\n")
	for _, ref := range strings.Split(body, `"$ref":"#/definitions/`)[1:] {
		name, _, _ := strings.Cut(ref, `"`)
		if !strings.Contains("\n"+defs, "\n  "+name+" ") {
			t.Fatalf("%s is not under Definitions:\n%s", name, spec)
		}
	}
	t.Logf("island_spec of neisland.RegisterAll: %d bytes", len(spec))
}

func TestIslandSpecComesInParts(t *testing.T) {
	full := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: full, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	_, _, whole := specText(t, testkit.Results(full.Request(t, 1))[0])
	model := api.Model{Name: "fake", Context: 31_250}
	limit := replay.RoundItemBudget(common.ConfigDefaults(api.ThreadConfig{}), model) / 2
	parts := 0
	var p *testkit.Provider
	p = testkit.NewProvider(model,
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			_, head, _ := specText(t, testkit.Results(r)[0])
			fmt.Sscanf(head, "revision %s\npart 1 of %d", new(string), &parts)
			turns := make([]testkit.Turn, 0, parts+1)
			for part := 2; part <= parts+1; part++ {
				turns = append(turns, testkit.Events(
					api.EventStart{ID: "parts", Model: "fake"},
					api.EventTool{Index: 0, ID: fmt.Sprint("spec-part-", part), Name: "page__island_spec", Args: fmt.Sprintf(`{"path":"board","part":%d}`, part)},
					api.EventClose{Index: 0},
					api.EventOk{Code: api.CodeTurn},
				))
			}
			p.Push(append(turns[1:], testkit.Say("done"))...)
			return turns[0]
		}),
	)
	c = stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	if parts < 3 {
		t.Fatalf("a %d-byte spec under a %d-byte part size came in %d parts, want 3 or more", len(whole), limit, parts)
	}
	var results []api.ContentToolResult
	for i := 1; i <= parts+1; i++ {
		results = append(results, testkit.Results(p.Request(t, i))...)
	}
	if len(results) != parts+1 {
		t.Fatalf("%d parts read, want %d and one past the end", len(results), parts+1)
	}
	refused(t, results[parts], fmt.Sprintf("part %d, the spec has %d in all", parts+1, parts))
	joined := ""
	for i, r := range results[:parts] {
		revision, head, body := specText(t, r)
		want := fmt.Sprintf("revision %s\npart %d of %d: call island_spec with part %d for the rest", revision, i+1, parts, i+2)
		if i == parts-1 {
			want = fmt.Sprintf("revision %s\npart %d of %d", revision, parts, parts)
		}
		if head != want {
			t.Fatalf("part %d opens with %q, want %q", i+1, head, want)
		}
		if size := len(head) + 2 + len(body); size > limit || strings.Contains(body, "<truncated>") {
			t.Fatalf("part %d takes %d bytes, over the %d-byte part size", i+1, size, limit)
		}
		if !strings.HasSuffix(body, "\n") {
			t.Fatalf("part %d ends inside a line:\n%s", i+1, body)
		}
		joined += body
	}
	notify := regexp.MustCompile(`\d+ bytes of ui.notify`)
	if notify.ReplaceAllString(joined, "") != notify.ReplaceAllString(whole, "") {
		t.Fatalf("the parts joined differ from the whole spec:\n%s", joined)
	}
}

func TestIslandSpecItemOverPartRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 1_000},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	refused(t, testkit.Results(p.Request(t, 1))[0], "intro takes")
}

func TestIslandStartReturnsTask(t *testing.T) {
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
		t.Fatalf("island_start answered %+v, want task 1 script 1", s)
	}
	select {
	case n := <-marks:
		if n != 3 {
			t.Fatalf("the script marked %d, want 3", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the script never called app.mark")
	}
	c.Wait()
	notify := lastText(p.Request(t, 3))
	for _, want := range []string{`<task_notify `, `kind="notifications_list"`, `path="board"`, `script="1"`, `task_id="1"`, `[{"marked":3}]`} {
		if !strings.Contains(notify, want) {
			t.Fatalf("the script's end notified %q, want it to carry %s", notify, want)
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
	c.Wait()
	record := lastText(p.Request(t, 3))
	for _, want := range []string{`kind="script_error"`, `script="1"`, "boom"} {
		if !strings.Contains(record, want) {
			t.Fatalf("the throw reported %q, want it to carry %s", record, want)
		}
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
	c.Wait()
	notify := lastText(p.Request(t, 3))
	for _, want := range []string{`"TypeError create: ui.wall: ` + long + `"`, `"TypeError fail: ` + long + `"`} {
		if !strings.Contains(notify, want) {
			t.Fatalf("the script saw %q, want it to carry %s", notify, want)
		}
	}
}

func TestIslandStartStaleRevisionRefused(t *testing.T) {
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

func TestIslandStartSyntaxErrorRefused(t *testing.T) {
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

func TestIslandStartRefusedAfterSpecChange(t *testing.T) {
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
	refused(t, testkit.Results(p.Request(t, 6))[0], "no such space")
	if slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("a refused start reported a task: %v", stack.Trace(events))
	}
}

func TestIslandScriptReadsForItself(t *testing.T) {
	page := func(cur gox.Cursor) error {
		if err := (neui.Space{Name: "cart", Desc: "the cart"}).Proxy(cur, func(cur gox.Cursor) error {
			return cur.Comp(neui.Tool[struct{}, string]{Name: "add", Desc: "adds an item", Handler: func(context.Context, struct{}) (string, error) {
				return "added", nil
			}})
		}); err != nil {
			return err
		}
		return cur.Any(board(make(chan int, 1), nil))
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "cart"}),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var seen = []
try { seen.push(page__tool({path: "cart", tool: "add", arg: {}}).output) } catch (e) { seen.push(Object.keys(e.value).join(",")) }
ui.notify(seen)`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("add from the board")
	c.Wait()
	c.Wait()
	if notify := lastText(p.Request(t, 4)); !strings.Contains(notify, `[["space"]]`) {
		t.Fatalf("the island script's add notified %q, want it refused with the space: the agent's read is not the script's", notify)
	}
}

func endBoard(marks chan<- int, screens chan<- string, success bool) gox.Elem {
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
				return l.Tool("mark", "marks a number", func(ctx context.Context, in struct{ N int }) (int, error) {
					marks <- in.N
					return in.N, nil
				})
			},
		}
		if success {
			island.Success = func(func(context.Context)) gox.Elem { return shown("done") }
		}
		return cur.Comp(island)
	}
}

func endScreen(t *testing.T, screens <-chan string, success bool) string {
	t.Helper()
	want, status := "idle", "no app is running in this island"
	if success {
		want, status = "done", "the island's app, task 1, has ended and the island shows its end state"
	}
	select {
	case got := <-screens:
		if got != want {
			t.Fatalf("the ended app shows %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("the ended app never showed %q", want)
	}
	return status
}

func TestIslandEndEndsAppAsStopDoes(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprintf("success %v", success), func(t *testing.T) {
			marks := make(chan int, 4)
			screens := make(chan string, 8)
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, `app.mark({N: 1})
ui.notify("dropped")
try { ui.end() } catch (e) { app.mark({N: 2}) } finally { app.mark({N: 3}) }
app.mark({N: 4})`)
				}),
				testkit.Say("started"),
				testkit.Say("ended"),
				testkit.Call("page__sync", map[string]any{"path": "board.screen"}),
				testkit.Say("seen"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: endBoard(marks, screens, success)})
			for len(screens) > 0 {
				<-screens
			}
			c.Send("open the board")
			c.Wait()
			c.Wait()
			ended := lastText(p.Request(t, 3))
			for _, want := range []string{`<task_result `, `task_id="1"`, "the island's app ended: its script called ui.end()"} {
				if !strings.Contains(ended, want) {
					t.Fatalf("ui.end() reported %q, want it to carry %s", ended, want)
				}
			}
			want := endScreen(t, screens, success)
			c.Send("look at the board")
			c.Wait()
			if status := statusOf(t, testkit.Results(p.Request(t, 5))[0]); !strings.HasPrefix(status, want) {
				t.Fatalf("the status after the end is %q, want %q", status, want)
			}
			if n := <-marks; n != 1 || len(marks) != 0 {
				t.Fatalf("the script marked %d and %d more, want only 1: code after ui.end() ran", n, len(marks))
			}
			for i, r := range p.Requests() {
				if strings.Contains(lastText(r), "dropped") {
					t.Fatalf("request %d carries %q, want the values queued before ui.end() dropped as island_stop drops them", i, lastText(r))
				}
			}
		})
	}
}

func TestIslandStopEndsAppWithItsOwnResult(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprintf("success %v", success), func(t *testing.T) {
			marks := make(chan int, 1)
			screens := make(chan string, 8)
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, `app.mark({N: 1})`)
				}),
				testkit.Call("page__island_stop", map[string]any{"path": "board"}),
				testkit.Say("stopped"),
				testkit.Call("page__sync", map[string]any{"path": "board.screen"}),
				testkit.Say("seen"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: endBoard(marks, screens, success)})
			for len(screens) > 0 {
				<-screens
			}
			c.Send("close the board")
			c.Wait()
			if r := testkit.Results(p.Request(t, 3))[0]; r.Error != api.ToolOk {
				t.Fatalf("island_stop failed with %#v", r.Content)
			}
			stopped := lastText(p.Request(t, 3))
			for _, want := range []string{`<task_result `, `task_id="1"`, "the island's app was stopped"} {
				if !strings.Contains(stopped, want) {
					t.Fatalf("island_stop reported %q, want it to carry %s", stopped, want)
				}
			}
			want := endScreen(t, screens, success)
			c.Send("look at the board")
			c.Wait()
			if status := statusOf(t, testkit.Results(p.Request(t, 5))[0]); !strings.HasPrefix(status, want) {
				t.Fatalf("the status after the end is %q, want %q", status, want)
			}
		})
	}
}

func TestIslandSpecCoversScriptPartsAndEnd(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	_, _, spec := specText(t, testkit.Results(p.Request(t, 1))[0])
	_, runtime, _ := strings.Cut(spec, "\nRuntime\n")
	if !strings.Contains(runtime, "\n- When a script part completes execution, the app keeps its last drawn screen and waits for the next script. Buttons do nothing until you send the next script with island_continue. To close the app from a script, call ui.end() explicitly.\n") {
		t.Fatalf("the spec's runtime does not carry the script part line:\n%s", spec)
	}
	if system := p.Request(t, 0).System; !strings.Contains(system, "\n- ui.end() ends the app as island_stop does") {
		t.Fatalf("the island guide does not list ui.end():\n%s", system)
	}
}
