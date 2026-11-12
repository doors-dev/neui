package island_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
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

type spec struct {
	Seq  uint
	Spec string
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
				l.Head(neisland.Style())
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
			return testkit.Call("page__island_spec", map[string]any{"path": "board"})
		}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	var s spec
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &s)
	if s.Seq != synced {
		t.Fatalf("island_spec reports seq %d, sync reported %d", s.Seq, synced)
	}
	for _, name := range []string{"mark", "press", "column", "row", "grid"} {
		if !strings.Contains(s.Spec, name) {
			t.Fatalf("the spec does not list %s:\n%s", name, s.Spec)
		}
	}
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
