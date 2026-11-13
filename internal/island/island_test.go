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

type tree struct {
	Space struct {
		Children map[string]struct {
			Seq    uint
			Island bool
		}
	}
}

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

func boardSeq(t *testing.T, then func(seq uint) testkit.Turn) testkit.Turn {
	return testkit.Then(func(r api.Request) testkit.Turn {
		var s tree
		testkit.Decode(t, testkit.Results(r)[0], &s)
		b := s.Space.Children["board"]
		if !b.Island {
			t.Errorf("sync shows board as %+v, want an island", b)
		}
		return then(b.Seq)
	})
}

func start(seq uint, code string) testkit.Turn {
	return testkit.Call("page__island_start", map[string]any{"path": "board", "seq": seq, "code": code, "comment": "the board app"})
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

func specText(t *testing.T, r api.ContentToolResult) (uint, string, string) {
	t.Helper()
	if r.Error != api.ToolOk || len(r.Content) != 1 {
		t.Fatalf("island_spec answered %#v, want one text", r)
	}
	text, _ := r.Content[0].(api.ContentText)
	head, body, ok := strings.Cut(text.Text, "\n\n")
	var seq uint
	if _, err := fmt.Sscanf(head, "seq %d", &seq); !ok || err != nil {
		t.Fatalf("island_spec answered %q, want a seq line, a blank line and the spec", text.Text)
	}
	return seq, head, body
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
	var synced uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			synced = seq
			return testkit.Events(
				api.EventStart{ID: "spec", Model: "fake"},
				api.EventTool{Index: 0, ID: "spec-whole", Name: "page__island_spec", Args: `{"path":"board"}`},
				api.EventClose{Index: 0},
				api.EventTool{Index: 1, ID: "spec-part-2", Name: "page__island_spec", Args: `{"path":"board","part":2}`},
				api.EventClose{Index: 1},
				api.EventOk{Code: api.CodeTurn},
			)
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	results := testkit.Results(p.Request(t, 2))
	if len(results) != 2 {
		t.Fatalf("the spec round answered %d calls, want 2", len(results))
	}
	seq, head, body := specText(t, results[0])
	if seq != synced || head != fmt.Sprintf("seq %d", synced) {
		t.Fatalf("island_spec opens with %q, sync reported seq %d and the spec fits one result", head, synced)
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
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	_, _, spec := specText(t, testkit.Results(p.Request(t, 2))[0])
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
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: full, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	_, _, whole := specText(t, testkit.Results(full.Request(t, 2))[0])
	model := api.Model{Name: "fake", Context: 200_000}
	limit := replay.ResultTextCap(common.ConfigDefaults(api.ThreadConfig{}), model)
	parts := 0
	p := testkit.NewProvider(model,
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			_, head, _ := specText(t, testkit.Results(r)[0])
			fmt.Sscanf(head, "seq %d\npart 1 of %d", new(uint), &parts)
			events := []api.Event{api.EventStart{ID: "parts", Model: "fake"}}
			for part := 2; part <= parts+1; part++ {
				events = append(events,
					api.EventTool{Index: part - 2, ID: fmt.Sprint("spec-part-", part), Name: "page__island_spec", Args: fmt.Sprintf(`{"path":"board","part":%d}`, part)},
					api.EventClose{Index: part - 2},
				)
			}
			return testkit.Events(append(events, api.EventOk{Code: api.CodeTurn})...)
		}),
		testkit.Say("done"),
	)
	c = stack.Page(t, stack.Host{Provider: p, Page: fullBoard()})
	c.Send("open the board")
	c.Wait()
	if parts < 3 {
		t.Fatalf("a %d-byte spec under a %d-byte result cap came in %d parts, want 3 or more", len(whole), limit, parts)
	}
	results := append(testkit.Results(p.Request(t, 2)), testkit.Results(p.Request(t, 3))...)
	if len(results) != parts+1 {
		t.Fatalf("%d parts read, want %d and one past the end", len(results), parts+1)
	}
	refused(t, results[parts], fmt.Sprintf("part %d, the spec has %d in all", parts+1, parts))
	joined := ""
	for i, r := range results[:parts] {
		seq, head, body := specText(t, r)
		want := fmt.Sprintf("seq %d\npart %d of %d: call island_spec with part %d for the rest", seq, i+1, parts, i+2)
		if i == parts-1 {
			want = fmt.Sprintf("seq %d\npart %d of %d", seq, parts, parts)
		}
		if head != want {
			t.Fatalf("part %d opens with %q, want %q", i+1, head, want)
		}
		if size := len(head) + 2 + len(body); size > limit || strings.Contains(body, "<truncated>") {
			t.Fatalf("part %d takes %d bytes, over the %d-byte result cap", i+1, size, limit)
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
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 200_000},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Config: api.ThreadConfig{ToolCallLimit: 1000}, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	refused(t, testkit.Results(p.Request(t, 2))[0], "intro takes")
}

func TestIslandStartReturnsTask(t *testing.T) {
	marks := make(chan int, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `ui.notify({marked: app.mark({N: 3})})`)
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
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `throw new Error("boom")`)
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
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `const seen = []
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

func TestIslandStartStaleSeqRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq+1, `ui.notify(1)`)
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	events := c.Wait()
	refused(t, testkit.Results(p.Request(t, 2))[0], "the island space's seq does not match")
	if slices.Contains(stack.Trace(events), "Tasks") {
		t.Fatalf("a refused start reported a task: %v", stack.Trace(events))
	}
}

func TestIslandStartSyntaxErrorRefused(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `var (`)
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
