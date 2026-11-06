package island_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type screenStatus struct {
	Space struct {
		Data []struct {
			Text   string
			Object any
		}
	}
}

func statusOf(t *testing.T, r api.ContentToolResult) string {
	t.Helper()
	var s screenStatus
	testkit.Decode(t, r, &s)
	for _, d := range s.Space.Data {
		if d.Text == "status" {
			text, _ := d.Object.(string)
			return text
		}
	}
	t.Fatalf("the screen synced as %+v, want a status", s.Space)
	return ""
}

type hold struct {
	held      chan<- error
	release   <-chan struct{}
	placeheld chan<- struct{}
}

func (h hold) board() gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name: "board",
			Desc: "a board",
			Placeholder: func(cur gox.Cursor) error {
				select {
				case h.placeheld <- struct{}{}:
				default:
				}
				return cur.Text("idle")
			},
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				return l.Tool("hold", "holds until released", func(ctx context.Context, _ struct{}) (int, error) {
					<-ctx.Done()
					h.held <- context.Cause(ctx)
					<-h.release
					return 0, nil
				})
			},
		})
	}
}

func TestIslandOvertakeWithCanceledCallShowsPlaceholderOnceOldRunEnds(t *testing.T) {
	held := make(chan error, 1)
	release := make(chan struct{})
	placeheld := make(chan struct{}, 8)
	var board uint
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			board = seq
			return start(seq, `app.hold({})`)
		}),
		testkit.Say("started"),
		testkit.Then(func(api.Request) testkit.Turn { return start(board, `ui.notify(2)`) }),
		testkit.Say("noted"),
		testkit.Call("page__sync", map[string]any{"path": "board.screen", "depth": 0}),
		testkit.Call("page__island_continue", map[string]any{"path": "board", "code": `ui.notify(3)`}),
		testkit.Say("seen"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: hold{held: held, release: release, placeheld: placeheld}.board()})
	for len(placeheld) > 0 {
		<-placeheld
	}
	c.Send("open the board")
	c.Wait()
	c.Send("replace the app")
	select {
	case cause := <-held:
		if cause == nil || !strings.Contains(cause.Error(), "replaced by a new one") {
			t.Fatalf("the old run's call ended with %v, want the overtake", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the new start never overtook the old run")
	}
	c.Stop()
	c.Until(func(e api.ChatEvent) bool {
		r, ok := e.(api.ChatEventToolResults)
		return ok && r.Final
	})
	if len(p.Requests()) != 4 {
		t.Fatalf("the stopped start made %d requests, want 4", len(p.Requests()))
	}
	select {
	case <-placeheld:
		t.Fatal("the placeholder showed while the old run still held")
	default:
	}
	close(release)
	c.Wait()
	ended := lastText(p.Request(t, 4))
	for _, want := range []string{`<task_result `, `task_id="1"`, "replaced by a new one"} {
		if !strings.Contains(ended, want) {
			t.Fatalf("the old run's end reported %q, want it to carry %s", ended, want)
		}
	}
	select {
	case <-placeheld:
	case <-time.After(5 * time.Second):
		t.Fatal("the placeholder never showed after the old run ended")
	}
	c.Send("look at the board")
	c.Wait()
	if status := statusOf(t, testkit.Results(p.Request(t, 6))[0]); !strings.HasPrefix(status, "no app is running") {
		t.Fatalf("the status after the old run ended is %q, want no app", status)
	}
	refused(t, testkit.Results(p.Request(t, 7))[0], "no app is running in this island")
}

func TestIslandTurnDeadlineEndsBlockingAppCall(t *testing.T) {
	held := make(chan error, 1)
	release := make(chan struct{})
	close(release)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "", "depth": 1}),
		boardSeq(t, func(seq uint) testkit.Turn {
			return start(seq, `app.hold({}); ui.notify("after")`)
		}),
		testkit.Say("started"),
		testkit.Say("seen"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Island: api.IslandConfig{TurnTime: 200 * time.Millisecond}, Page: hold{held: held, release: release, placeheld: make(chan struct{}, 8)}.board()})
	c.Send("open the board")
	c.Wait()
	select {
	case cause := <-held:
		if cause == nil || !strings.Contains(cause.Error(), "went that long without ui.draw") {
			t.Fatalf("the blocking call ended with %v, want the turn deadline", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the turn deadline never ended the blocking call")
	}
	c.Wait()
	record := lastText(p.Request(t, 3))
	for _, want := range []string{`kind="script_error"`, `script="1"`, "went that long without ui.draw"} {
		if !strings.Contains(record, want) {
			t.Fatalf("the deadline reported %q, want it to carry %s", record, want)
		}
	}
	if strings.Contains(record, "<task_result") || strings.Contains(record, "after") {
		t.Fatalf("the deadline ended the app or let the script go on: %q", record)
	}
}
