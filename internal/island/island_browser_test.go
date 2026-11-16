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
	"github.com/go-rod/rod/lib/proto"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
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
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.column({})); var r = ui.draw(1); app.mark({N: r === null ? 1 : 0}); ui.notify({drawn: r})`)
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

func TestIslandUIEndCarriesValuesSinceLastDraw(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.column({})); ui.notify(1); ui.draw(1); ui.notify(2); ui.end()`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
		testkit.Say("ended"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	carries(t, recorded(t, c, p, `kind="notifications_list"`), `script="1"`, `>[1]</task_notify>`)
	carries(t, recorded(t, c, p, "<task_result "), `>{"ended":"the island's app ended: its script called ui.end()","notifications":[2]}</task_result>`)
}

func pressSynced(t *testing.T, then func(input string) testkit.Turn) testkit.Turn {
	return testkit.Then(func(r api.Request) testkit.Turn {
		var s struct{ Inputs []struct{ ID string } }
		testkit.Decode(t, testkit.Results(r)[0], &s)
		if len(s.Inputs) != 1 {
			t.Errorf("the press space synced as %+v, want one input", s)
			return testkit.Say("lost")
		}
		return then(s.Inputs[0].ID)
	})
}

func pressScript(t *testing.T, shown <-chan string, then func(input string) testkit.Turn) *testkit.Provider {
	return testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var b = ui.press({label: "go", value: "(n) => app.mark({N: n}) * 2"}); ui.root.set(b); var r = ui.draw(); ui.notify(r); ui.draw(1)`)
		}),
		shownThen(t, shown, testkit.Call("page__sync", map[string]any{"path": "board.screen.press_1"})),
		pressSynced(t, then),
		testkit.Say("pressed"),
	)
}

func TestIslandSubmitAnswersDraw(t *testing.T) {
	marks := make(chan int, 1)
	shown := make(chan string, 1)
	p := pressScript(t, shown, func(string) testkit.Turn {
		return testkit.Call("page__tool", map[string]any{"path": "board.screen.press_1", "tool": "press", "arg": map[string]any{}})
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
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"output":42}]`)
}

func TestIslandControlClickSubmits(t *testing.T) {
	marks := make(chan int, 1)
	shown := make(chan string, 1)
	p := pressScript(t, shown, func(input string) testkit.Turn {
		return testkit.Call("page__click", map[string]any{"path": "board.screen.press_1", "id": input})
	})
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("press it")
	c.Wait()
	if r := testkit.Results(p.Request(t, 4))[0]; r.Error != api.ToolOk {
		t.Fatalf("the click answered %#v, want the press handler run", r.Content)
	}
	if n := <-marks; n != 21 {
		t.Fatalf("value called app.mark with %d, want 21", n)
	}
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"output":42}]`)
}

func TestIslandInterruptHandsOverTheUI(t *testing.T) {
	marks := make(chan int)
	shown := make(chan string, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var b = ui.press({label: "wait", value: "(n) => n"}); ui.root.set(b); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Call("page__island_interrupt", map[string]any{"path": "board", "code": `app.mark({N: 2}); ui.notify({kept: b.id})`})),
		testkit.Say("continued"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("change the board")
	c.Wait()
	var s started
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &s)
	if s != (started{Task: 1, Script: 2}) {
		t.Fatalf("island_interrupt answered %+v, want task 1 script 2", s)
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

func TestIslandAppendWaitsBehindADrawLoopAndInterruptTakesOver(t *testing.T) {
	marks := make(chan int)
	shown := make(chan string, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `var b = ui.press({label: "wait", value: "(n) => n"}); ui.root.set(b); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Call("page__island_append", map[string]any{"path": "board", "code": `app.mark({N: 1})`})),
		testkit.Call("page__island_interrupt", map[string]any{"path": "board", "code": `app.mark({N: 2}); ui.notify({kept: b.id})`}),
		testkit.Say("interrupted"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(marks, shown)})
	c.Send("change the board")
	c.Wait()
	for i, want := range []started{{Task: 1, Script: 2}, {Task: 1, Script: 3}} {
		var s started
		testkit.Decode(t, testkit.Results(p.Request(t, 3+i))[0], &s)
		if s != want {
			t.Fatalf("call %d answered %+v, want %+v", i+1, s, want)
		}
	}
	select {
	case n := <-marks:
		if n != 2 {
			t.Fatalf("the first mark is %d, want 2 from the interrupting part: the appended part waits behind the draw loop and is dropped", n)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the interrupting part never ran")
	}
	carries(t, recorded(t, c, p, `kind="notifications_list"`), `script="3"`, `[{"kept":1}]`)
}

func TestIslandEndEndsTask(t *testing.T) {
	shown := make(chan string, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.press({label: "wait", value: "(n) => n"})); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Call("page__island_end", map[string]any{"path": "board"})),
		testkit.Say("ended"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(make(chan int, 1), shown)})
	c.Send("close the board")
	events := c.Wait()
	if r := testkit.Results(p.Request(t, 3))[0]; r.Error != api.ToolOk {
		t.Fatalf("island_end failed with %#v", r.Content)
	}
	carries(t, lastText(p.Request(t, 3)), `<task_result `, `task_id="1"`)
	if trace := stack.Trace(events); trace[len(trace)-1] != "RoundEnd(ok,final)" {
		t.Fatalf("the end round traced %v", trace)
	}
}

type inputs struct {
	Inputs []struct {
		ID   string
		Desc string
	}
}

func screenOf(t *testing.T, results []api.ContentToolResult) (string, []string) {
	t.Helper()
	var labels []string
	for _, r := range results[1:] {
		if r.Error != api.ToolOk {
			continue
		}
		var child inputs
		testkit.Decode(t, r, &child)
		for _, in := range child.Inputs {
			labels = append(labels, in.Desc)
		}
	}
	return statusOf(t, results[0]), labels
}

func syncScreen(children ...string) testkit.Turn {
	events := []api.Event{api.EventStart{ID: "screen", Model: "fake"}}
	for i, path := range append([]string{"board.screen"}, children...) {
		events = append(events, api.EventTool{Index: i, ID: path, Name: "page__sync", Args: `{"path":"` + path + `"}`}, api.EventClose{Index: i})
	}
	return testkit.Events(append(events, api.EventOk{Code: api.CodeTurn})...)
}

func TestIslandOvertakeStatusNamesNewTaskBeforeItsFirstDraw(t *testing.T) {
	marks := make(chan int)
	shown := make(chan string, 1)
	var boardAt string
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			boardAt = revision
			return start(revision, `ui.root.set(ui.press({label: "old", value: "(n) => n"})); while (true) { ui.draw() }`)
		}),
		shownThen(t, shown, testkit.Then(func(api.Request) testkit.Turn {
			return start(boardAt, `app.mark({N: 1}); ui.root.set(ui.press({label: "new", value: "(n) => n"})); while (true) { ui.draw() }`)
		})),
		syncScreen("board.screen.press_1"),
		testkit.Say("seen the old screen"),
		syncScreen("board.screen.press_1"),
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
	status, screen := screenOf(t, testkit.Results(p.Request(t, 4)))
	if !strings.Contains(status, "task 2") {
		t.Fatalf("before the new run's first draw the status is %q, want it to name task 2", status)
	}
	if got := screen; !slices.Equal(got, []string{"old"}) {
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
	status, screen = screenOf(t, testkit.Results(p.Request(t, 6)))
	if !strings.Contains(status, "task 2") {
		t.Fatalf("after the new run's first draw the status is %q, want it to name task 2", status)
	}
	if got := screen; !slices.Equal(got, []string{"new"}) {
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
				if err := l.Layout("column", neisland.ColumnDesc, neisland.Column); err != nil {
					return err
				}
				return neisland.RegisterButton(l)
			},
		})
	}
}

func TestIslandButtonWithAgentIsClickableAndSubmitsItsValue(t *testing.T) {
	drawn := make(chan struct{}, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.column({}, ui.button({text: "Go", value: "v1", agent: "submits go"}), ui.button({text: "Plain", value: "v2"}))); ui.notify(ui.draw()); ui.draw(1)`)
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			select {
			case <-drawn:
			case <-time.After(10 * time.Second):
				t.Error("the buttons were never drawn")
			}
			return syncScreen("board.screen.button_1", "board.screen.button_2")
		}),
		testkit.Then(func(r api.Request) testkit.Turn {
			results := testkit.Results(r)
			var agent, plain inputs
			testkit.Decode(t, results[1], &agent)
			testkit.Decode(t, results[2], &plain)
			if len(agent.Inputs) != 1 || agent.Inputs[0].Desc != "submits go" || len(plain.Inputs) != 0 {
				t.Errorf("the buttons synced as %+v and %+v, want only the agent one as an input", agent, plain)
				return testkit.Say("lost")
			}
			return testkit.Call("page__click", map[string]any{"path": "board.screen.button_1", "id": agent.Inputs[0].ID})
		}),
		testkit.Say("clicked"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: buttons(drawn)})
	c.Send("click go")
	c.Wait()
	if r := testkit.Results(p.Request(t, 4))[0]; r.Error != api.ToolOk {
		t.Fatalf("the click answered %#v, want the button's handler run", r.Content)
	}
	carries(t, lastText(p.Request(t, 4)), `<task_notify `, `script="1"`, `[{"id":1,"output":"v1"}]`)
}

const actionNote = `note="the part stopped at this error while handling the user&#39;s action in last_action, which was not carried out; any parts queued behind it were dropped, listed in dropped; the app runs on: send the fix with island_append, and in it carry out that action or ask the user to redo it"`

const plainErrorNote = `note="the part stopped at this error and any parts queued behind it were dropped, listed in dropped; the app runs on: send what should still run with island_append"`

func userClick(t *testing.T, c *stack.Chat, text string) {
	t.Helper()
	button, err := c.Page.Timeout(10*time.Second).ElementR("button", "^"+text+"$")
	if err != nil {
		t.Fatalf("the %s button never reached the page: %v", text, err)
	}
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatalf("the user's click on %s failed: %v", text, err)
	}
}

func lacks(t *testing.T, record string, unwanted string) {
	t.Helper()
	if strings.Contains(record, unwanted) {
		t.Fatalf("the record %q carries %s", record, unwanted)
	}
}

func throwBoard(t *testing.T, code string) (*testkit.Provider, *stack.Chat) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn { return start(revision, code) }),
		testkit.Say("started"),
		testkit.Say("seen"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	return p, c
}

func TestIslandThrowWhileHandlingAnActionReportsIt(t *testing.T) {
	p, c := throwBoard(t, `ui.root.set(ui.column({}, ui.press({label: "keep", value: "(n) => n"}), ui.press({label: "save", value: "(n) => n * 2"}))); ui.draw(); throw new Error("save failed")`)
	userClick(t, c, "save")
	carries(t, recorded(t, c, p, `kind="script_error"`), `script="1"`, "save failed", `last_action="{&#34;id&#34;:2,&#34;output&#34;:42}"`, actionNote)
}

func TestIslandThrowAfterDrawingAgainLeavesTheActionOut(t *testing.T) {
	p, c := throwBoard(t, `ui.root.set(ui.press({label: "save", value: "(n) => n"})); ui.draw(); ui.draw(1); throw new Error("later")`)
	userClick(t, c, "save")
	record := recorded(t, c, p, `kind="script_error"`)
	carries(t, record, `script="1"`, "later", plainErrorNote)
	lacks(t, record, "last_action")
}

func TestIslandThrowInALaterPartLeavesTheEarlierActionOut(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.press({label: "save", value: "(n) => n"})); ui.notify(ui.draw())`)
		}),
		testkit.Say("started"),
		testkit.Call("page__island_append", map[string]any{"path": "board", "code": `throw new Error("later")`}),
		testkit.Say("appended"),
		testkit.Say("seen"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: board(make(chan int, 1), nil)})
	c.Send("open the board")
	c.Wait()
	userClick(t, c, "save")
	carries(t, recorded(t, c, p, `kind="notifications_list"`), `script="1"`, `>[{"id":1,"output":21}]</task_notify>`)
	record := recorded(t, c, p, `kind="script_error"`)
	carries(t, record, `script="2"`, "later", plainErrorNote)
	lacks(t, record, "last_action")
}

type fillParams struct {
	Size int
}

type fill struct {
	size   int
	submit func(ctx context.Context, s string) bool
}

func (f fill) Update(context.Context, struct{}) {}

func (f fill) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		if err := cur.Init("button"); err != nil {
			return err
		}
		if err := cur.Modify(doors.AClick{On: func(ctx context.Context, _ doors.RequestPointer) bool {
			f.submit(ctx, strings.Repeat("x", f.size))
			return false
		}}); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Text("fill"); err != nil {
			return err
		}
		return cur.Close()
	}
}

func fillBoard(cur gox.Cursor) error {
	return cur.Comp(neui.Island{
		Name:        "board",
		Desc:        "a board",
		Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
		Template: func(slot gox.Comp) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(slot) }
		},
		Library: func(l neui.Library) error {
			return l.Control("fill", "a button that answers with size x's", func(_ context.Context, p fillParams, submit func(ctx context.Context, s string) bool) (api.Widget[struct{}], error) {
				return fill{size: p.Size, submit: submit}, nil
			})
		},
	})
}

func TestIslandThrowWhileHandlingAnOversizedActionKeepsItsID(t *testing.T) {
	p := testkit.NewProvider(api.Model{Name: "fake", Context: 200_000},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.fill({size: 40000})); ui.draw(); throw new Error("save failed")`)
		}),
		testkit.Say("started"),
		testkit.Say("seen"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: gox.Elem(fillBoard)})
	c.Send("open the board")
	c.Wait()
	userClick(t, c, "fill")
	record := recorded(t, c, p, `kind="script_error"`)
	carries(t, record, `script="1"`, "save failed", `last_action="{&#34;id&#34;:1,&#34;output_omitted&#34;:true}"`, actionNote)
	lacks(t, record, "xxxx")
}
