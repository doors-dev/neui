//go:build browser

package island_test

import (
	"context"
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

type holderParams struct {
	Label string
}

type holder struct {
	label   string
	settled chan<- struct{}
	submit  func(ctx context.Context, s string) bool
}

func (h holder) Update(context.Context, struct{}) {}

func (h holder) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		if err := cur.Comp(neui.Tool[struct{}, string]{Name: "press", Desc: "presses the button", Handler: func(ctx context.Context, _ struct{}) (string, error) {
			if !h.submit(ctx, h.label) {
				return "", nil
			}
			return h.label, nil
		}}); err != nil {
			return err
		}
		return (neui.Input{Desc: h.label, Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("button"); err != nil {
				return err
			}
			if err := cur.Modify(doors.AClick{
				Scope:     neui.IslandScope(cur.Context()),
				Indicator: doors.IndicateAttr("aria-busy", "true"),
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					if h.submit(ctx, h.label) {
						doors.OnSettle(ctx, func(context.Context) {
							select {
							case h.settled <- struct{}{}:
							default:
							}
						})
					}
					return false
				},
			}); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Text(h.label); err != nil {
				return err
			}
			return cur.Close()
		})
	}
}

func holdBoard(settled chan<- struct{}) gox.Elem {
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
				return l.Control("hold", "a button that answers with its label", func(_ context.Context, p holderParams, submit func(ctx context.Context, s string) bool) (api.Widget[struct{}], error) {
					return holder{label: p.Label, settled: settled, submit: submit}, nil
				})
			},
		})
	}
}

func busyNow(t *testing.T, c *stack.Chat, label string, want bool) {
	t.Helper()
	stack.Eventually(t, 5*time.Second, func() (bool, string) {
		button, err := c.Page.Timeout(time.Second).ElementR("button", "^"+label+"$")
		if err != nil {
			return false, err.Error()
		}
		v, err := button.Attribute("aria-busy")
		if err != nil {
			return false, err.Error()
		}
		return (v != nil && *v == "true") == want, "aria-busy " + func() string {
			if v == nil {
				return "absent"
			}
			return *v
		}()
	}, "the button's aria-busy")
}

func TestIslandUserSubmitStaysPendingUntilTheNextDraw(t *testing.T) {
	settled := make(chan struct{}, 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.hold({label: "go"})); ui.notify(ui.draw())`)
		}),
		testkit.Say("started"),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: holdBoard(settled)})
	c.Send("open the board")
	c.Wait()
	userClick(t, c, "go")
	carries(t, recorded(t, c, p, `kind="notifications_list"`), `[{"id":1,"output":"go"}]`)
	select {
	case <-settled:
		t.Fatal("the user's submit settled when the part ended, want it pending until the next draw")
	case <-time.After(time.Second):
	}
	busyNow(t, c, "go", true)
	p.Push(testkit.Call("page__island_append", map[string]any{"path": "board", "code": `ui.draw(1)`}), testkit.Say("appended"))
	c.Send("go on")
	c.Wait()
	select {
	case <-settled:
	case <-time.After(10 * time.Second):
		t.Fatal("the user's submit never settled after the next draw")
	}
	busyNow(t, c, "go", false)
}

func TestIslandPageToolSubmitReturnsWhenThePartEndsWithoutADraw(t *testing.T) {
	took := make(chan time.Duration, 1)
	var pressed time.Time
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__island_spec", map[string]any{"path": "board"}),
		revised(t, func(revision string) testkit.Turn {
			return start(revision, `ui.root.set(ui.hold({label: "go"})); ui.notify(ui.draw())`)
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			time.Sleep(time.Second)
			return testkit.Call("page__sync", map[string]any{"path": "board.screen.hold_1"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			pressed = time.Now()
			return testkit.Call("page__tool", map[string]any{"path": "board.screen.hold_1", "tool": "press", "arg": map[string]any{}})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			took <- time.Since(pressed)
			return testkit.Say("pressed")
		}),
		testkit.Say("noted"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: holdBoard(make(chan struct{}, 1))})
	c.Send("press go")
	c.Wait()
	select {
	case d := <-took:
		if d > 2*time.Second {
			t.Fatalf("the press returned after %v, want it back once the part ends", d)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the press never returned to the agent")
	}
	carries(t, recorded(t, c, p, `kind="notifications_list"`), `[{"id":1,"output":"go"}]`)
}
