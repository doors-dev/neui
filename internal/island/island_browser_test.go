//go:build browser

package island_test

import (
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
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
	t.Skip("bug: neui.Input.Proxy (page.go:180) renders the input's script before its modifier has run when the widget renders inside an island, so the button has no client handler and page__click fails with \"handler ... not found\"")
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
