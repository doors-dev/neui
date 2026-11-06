//go:build browser

package island_test

import (
	"context"
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
	"neui.dev/ui"
)

func shownThen(t *testing.T, shown <-chan string, then testkit.Turn) testkit.Turn {
	return testkit.Then(func(api.Request) testkit.Turn {
		select {
		case <-shown:
		case <-time.After(10 * time.Second):
			t.Error("the press was never drawn")
		}
		return then
	})
}

func carries(t *testing.T, record string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(record, w) {
			t.Fatalf("the record %q does not carry %s", record, w)
		}
	}
}

func TestIslandDrawReturnsAfterTimeout(t *testing.T) {
	marks := make(chan int)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `ui.root.set(ui.column({})); var r = ui.draw(1); app.mark({N: r === null ? 1 : 0}); ui.notify({drawn: r})`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, nil)})
	c.Send("open the board")
	c.Wait()
	select {
	case n := <-marks:
		if n != 1 {
			t.Fatalf("ui.draw(1) returned a user action, want null")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ui.draw(1) never returned")
	}
	c.Wait()
	carries(t, lastText(p.Request(t, 3)), `<task_notify `, `kind="notifications_list"`, `script="1"`, `[{"drawn":null}]`)
}

func pressSynced(t *testing.T, then func(seq uint, input uint) testkit.Turn) testkit.Turn {
	return testkit.Then(func(r api.Request) testkit.Turn {
		var s struct {
			Space struct {
				Seq    uint
				Inputs []struct{ ID uint }
			}
		}
		testkit.Decode(t, testkit.Results(r)[0], &s)
		if len(s.Space.Inputs) != 1 {
			t.Errorf("the press space synced as %+v, want one input", s.Space)
			return testkit.Say("lost")
		}
		return then(s.Space.Seq, s.Space.Inputs[0].ID)
	})
}

func pressScript(t *testing.T, shown <-chan string, then func(seq uint, input uint) testkit.Turn) *testkit.Provider {
	return testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `var b = ui.press({label: "go", value: "(n) => app.mark({N: n}) * 2"}); ui.root.set(b); var r = ui.draw(); ui.notify(r); ui.draw(1)`)
		}),
		shownThen(t, shown, testkit.Call("page__sync", map[string]any{"path": "board.screen.press_1", "depth": 0})),
		pressSynced(t, then),
		testkit.Say("pressed"),
	)
}

func TestIslandSubmitAnswersDraw(t *testing.T) {
	marks := make(chan int, 1)
	shown := make(chan string, 1)
	p := pressScript(t, shown, func(seq uint, _ uint) testkit.Turn {
		return testkit.Call("page__tool", map[string]any{"path": "board.screen.press_1", "seq": seq, "tool": "press", "arg": map[string]any{}})
	})
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("press it")
	c.Wait()
	var pressed struct{ Output int }
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &pressed)
	if pressed.Output != 42 {
		t.Fatalf("the press answered %+v, want value(21) = 42", pressed)
	}
	if n := <-marks; n != 21 {
		t.Fatalf("value called app.mark with %d, want 21", n)
	}
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"data":42}]`)
}

func TestIslandControlClickSubmits(t *testing.T) {
	marks := make(chan int, 1)
	shown := make(chan string, 1)
	p := pressScript(t, shown, func(seq uint, input uint) testkit.Turn {
		return testkit.Call("page__click", map[string]any{"path": "board.screen.press_1", "seq": seq, "id": input})
	})
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("press it")
	c.Wait()
	var clicked struct{ Hooks int }
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &clicked)
	if clicked.Hooks != 1 {
		t.Fatalf("the click triggered %d hooks, want the press handler", clicked.Hooks)
	}
	if n := <-marks; n != 21 {
		t.Fatalf("value called app.mark with %d, want 21", n)
	}
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"data":42}]`)
}

func TestIslandInterruptHandsOverTheUI(t *testing.T) {
	marks := make(chan int)
	shown := make(chan string, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `var b = ui.press({label: "wait", value: "(n) => n"}); ui.root.set(b); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Call("page__island_continue", map[string]any{"path": "board", "code": `app.mark({N: 2}); ui.notify({kept: b.id})`, "interrupt": true})),
		testkit.Say("continued"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("change the board")
	c.Wait()
	var s started
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &s)
	if s != (started{Task: 1, Script: 2}) {
		t.Fatalf("island_continue answered %+v, want task 1 script 2", s)
	}
	select {
	case <-marks:
	case <-time.After(10 * time.Second):
		t.Fatal("the interrupting script never ran")
	}
	c.Wait()
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `kind="notifications_list"`, `script="2"`, `[{"kept":1}]`)
	for i, r := range p.Requests() {
		for _, m := range r.Messages {
			for _, block := range m.Content {
				if text, ok := block.(api.ContentText); ok && strings.Contains(text.Text, "script_error") {
					t.Fatalf("request %d reports %q: the handed-over script must stop quietly", i, text.Text)
				}
			}
		}
	}
}

func TestIslandStopEndsTask(t *testing.T) {
	shown := make(chan string, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `ui.root.set(ui.press({label: "wait", value: "(n) => n"})); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Call("page__island_stop", map[string]any{"path": "board"})),
		testkit.Say("stopped"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(make(chan int, 1), shown)})
	c.Send("close the board")
	events := c.Wait()
	if r := testkit.Results(p.Request(t, 3))[0]; r.Error != api.ToolOk {
		t.Fatalf("island_stop failed with %#v", r.Content)
	}
	carries(t, lastText(p.Request(t, 3)), `<task_result `, `task_id="1"`)
	if trace := stack.Trace(events); trace[len(trace)-1] != "RoundEnd(ok,final)" {
		t.Fatalf("the stop round traced %v", trace)
	}
}

type screenTree struct {
	Space struct {
		Data []struct {
			Text   string
			Object any
		}
		Children map[string]struct {
			Seq    uint
			Inputs []struct {
				ID   uint
				Desc string
			}
		}
	}
}

func screenOf(t *testing.T, r api.ContentToolResult) (string, screenTree) {
	t.Helper()
	var s screenTree
	testkit.Decode(t, r, &s)
	for _, d := range s.Space.Data {
		if d.Text == "status" {
			text, _ := d.Object.(string)
			return text, s
		}
	}
	t.Fatalf("the screen synced as %+v, want a status", s.Space)
	return "", s
}

func labels(s screenTree) []string {
	var all []string
	for _, child := range s.Space.Children {
		for _, in := range child.Inputs {
			all = append(all, in.Desc)
		}
	}
	return all
}

func TestIslandOvertakeStatusNamesNewTaskBeforeItsFirstDraw(t *testing.T) {
	marks := make(chan int)
	shown := make(chan string, 1)
	var boardAt uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			boardAt = seq
			return start(seq, `ui.root.set(ui.press({label: "old", value: "(n) => n"})); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Then(func(api.Request) testkit.Turn {
			return start(boardAt, `app.mark({N: 1}); ui.root.set(ui.press({label: "new", value: "(n) => n"})); while (true) { ui.draw() }`)
		})),
		testkit.Call("page__sync", map[string]any{"path": "board.screen", "depth": 1}),
		testkit.Say("seen the old screen"),
		testkit.Call("page__sync", map[string]any{"path": "board.screen", "depth": 1}),
		testkit.Say("seen the new screen"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("replace the app")
	c.Wait()
	var s started
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &s)
	if s != (started{Task: 2, Script: 1}) {
		t.Fatalf("the overtaking start answered %+v, want task 2 script 1", s)
	}
	status, screen := screenOf(t, testkit.Results(p.Request(t, 4))[0])
	if !strings.Contains(status, "task 2") {
		t.Fatalf("before the new run's first draw the status is %q, want it to name task 2", status)
	}
	if got := labels(screen); !slices.Equal(got, []string{"old"}) {
		t.Fatalf("before the new run's first draw the screen shows %v, want the old press", got)
	}
	select {
	case <-marks:
	case <-time.After(10 * time.Second):
		t.Fatal("the new run never reached app.mark")
	}
	select {
	case label := <-shown:
		if label != "new" {
			t.Fatalf("the new run drew %q, want new", label)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the new run never drew")
	}
	c.Send("look again")
	c.Wait()
	status, screen = screenOf(t, testkit.Results(p.Request(t, 6))[0])
	if !strings.Contains(status, "task 2") {
		t.Fatalf("after the new run's first draw the status is %q, want it to name task 2", status)
	}
	if got := labels(screen); !slices.Equal(got, []string{"new"}) {
		t.Fatalf("after the new run's first draw the screen shows %v, want the new press", got)
	}
}

func buttons(drawn chan<- struct{}) gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error {
					doors.OnBatch(cur.Context(), func(context.Context) {
						select {
						case drawn <- struct{}{}:
						default:
						}
					})
					return cur.Comp(slot)
				}
			},
			Library: func(l neui.Library) error {
				if err := l.Layout("column", ui.ColumnDesc, ui.Column); err != nil {
					return err
				}
				l.Head(ui.Style())
				return l.Control("button", "a button", ui.NewButton)
			},
		})
	}
}

func TestIslandButtonWithAgentIsClickableAndSubmitsItsValue(t *testing.T) {
	drawn := make(chan struct{}, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `ui.root.set(ui.column({}, ui.button({text: "Go", value: "v1", agent: "submits go"}), ui.button({text: "Plain", value: "v2"}))); ui.notify(ui.draw()); ui.draw(1)`)
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			select {
			case <-drawn:
			case <-time.After(10 * time.Second):
				t.Error("the buttons were never drawn")
			}
			return testkit.Call("page__sync", map[string]any{"path": "board.screen", "depth": 1})
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			_, s := screenOf(t, testkit.Results(r)[0])
			agent, plain := s.Space.Children["button_1"], s.Space.Children["button_2"]
			if len(agent.Inputs) != 1 || agent.Inputs[0].Desc != "submits go" || len(plain.Inputs) != 0 {
				t.Errorf("the buttons synced as %+v, want only the agent one as an input", s.Space.Children)
				return testkit.Say("lost")
			}
			return testkit.Call("page__click", map[string]any{"path": "board.screen.button_1", "seq": agent.Seq, "id": agent.Inputs[0].ID})
		}),
		testkit.Say("clicked"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: buttons(drawn)})
	c.Send("click go")
	c.Wait()
	var clicked struct{ Hooks int }
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &clicked)
	if clicked.Hooks != 1 {
		t.Fatalf("the click triggered %d hooks, want the button's handler", clicked.Hooks)
	}
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"data":"v1"}]`)
}
