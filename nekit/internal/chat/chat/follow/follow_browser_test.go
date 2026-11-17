//go:build browser

package follow_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/go-rod/rod"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit/internal/chat/chat/follow"
	"neui.dev/nekit/internal/chat/css"
)

func div(attrs map[string]any, children ...any) gox.Elem {
	return func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		for name, value := range attrs {
			if err := cur.Set(name, value); err != nil {
				return err
			}
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Many(children...); err != nil {
			return err
		}
		return cur.Close()
	}
}

func tall(px int) gox.Elem {
	return div(map[string]any{"style": fmt.Sprintf("height: %dpx", px)})
}

type position struct {
	Top float64
	End float64
}

type log struct {
	t      *testing.T
	page   *rod.Page
	ctx    context.Context
	answer *doors.Door
	hidden *doors.Door
}

func openLog(t *testing.T, pinned bool) *log {
	t.Helper()
	l := &log{t: t, answer: new(doors.Door), hidden: new(doors.Door)}
	user := []any{div(map[string]any{"class": "nechat-bubble"}, "the question")}
	if pinned {
		user = append([]any{div(map[string]any{"class": "nechat-span"})}, user...)
	}
	rendered := make(chan context.Context, 1)
	c := stack.Browser(t, stack.Host{Provider: testkit.NewProvider(api.Model{}), Page: func(cur gox.Cursor) error {
		select {
		case rendered <- doors.InstanceContext(cur.Context()):
		default:
		}
		return cur.Many(
			css.Style(css.Theme{}, css.Theme{}, css.Layout{}),
			div(map[string]any{"class": "nechat-log nechat-log_scroll", "style": "height: 400px"},
				div(map[string]any{"class": "nechat-section"},
					div(map[string]any{"class": "nechat-turn"}, tall(1200)),
					div(map[string]any{"class": "nechat-turn"},
						div(map[string]any{"class": "nechat-user"}, user...),
						div(map[string]any{"class": "nechat-agent"}, l.answer),
					),
				),
				div(map[string]any{"class": "nechat-section", "hidden": true}, l.hidden),
			),
		)
	}})
	l.ctx = <-rendered
	l.page = c.Page.Timeout(time.Minute)
	l.page.MustWaitLoad()
	return l
}

func (l *log) scroll(top float64) float64 {
	l.t.Helper()
	return l.page.MustEval(`(top) => {
		const log = document.querySelector(".nechat-log")
		log.scrollTo({ top, behavior: "instant" })
		return log.scrollTop
	}`, top).Num()
}

func (l *log) add(door **doors.Door, px int) position {
	l.t.Helper()
	next := new(doors.Door)
	applied := 0
	for err := range (*door).Static(l.ctx, gox.Elem(func(cur gox.Cursor) error {
		return cur.Many(follow.Mark(), tall(px), next)
	})) {
		if err != nil {
			l.t.Fatalf("the block did not land: %v", err)
		}
		applied++
	}
	if applied != 2 {
		l.t.Fatal("the block was not applied to the page")
	}
	*door = next
	settled := l.page.MustEval(`() => new Promise((done) => {
		const log = document.querySelector(".nechat-log")
		let top = NaN
		let since = 0
		const tick = (now) => {
			if (document.querySelector(".nechat-follow") || log.scrollTop !== top) {
				top = log.scrollTop
				since = now
			} else if (now - since > 300) {
				done({ top, end: log.scrollHeight - log.clientHeight })
				return
			}
			requestAnimationFrame(tick)
		}
		requestAnimationFrame(tick)
	})`)
	return position{Top: settled.Get("top").Num(), End: settled.Get("end").Num()}
}

func TestFollowStaysPutWhenScrolledUpDuringAShortPinnedAnswer(t *testing.T) {
	l := openLog(t, true)
	pinned := l.scroll(1e9)
	up := l.scroll(pinned - 200)
	if p := l.add(&l.answer, 40); p.Top != up {
		t.Fatalf("a block moved a reader scrolled up from the pinned position: %v -> %v (end %v)", up, p.Top, p.End)
	}
}

func TestFollowResumesAtThePinnedPosition(t *testing.T) {
	l := openLog(t, true)
	pinned := l.scroll(1e9)
	l.scroll(pinned - 200)
	l.add(&l.answer, 40)
	l.scroll(1e9)
	var p position
	for range 10 {
		p = l.add(&l.answer, 60)
	}
	if p.End < pinned+200 || p.Top != p.End {
		t.Fatalf("the reader back at the pinned position (%v) was left at %v while the answer grew to %v", pinned, p.Top, p.End)
	}
}

func TestFollowKeepsUpWithALongAnswer(t *testing.T) {
	l := openLog(t, true)
	pinned := l.scroll(1e9)
	var p position
	for i := range 16 {
		p = l.add(&l.answer, 60)
		if p.Top != p.End {
			t.Fatalf("block %d left the reader at %v, the answer ends at %v", i, p.Top, p.End)
		}
	}
	if p.End < pinned+400 {
		t.Fatalf("the answer ends at %v, want it well past the pinned position %v", p.End, pinned)
	}
}

func TestFollowIgnoresAMarkInAHiddenSection(t *testing.T) {
	l := openLog(t, false)
	up := l.scroll(100)
	if p := l.add(&l.hidden, 40); p.Top != up {
		t.Fatalf("a mark in a hidden section moved the reader: %v -> %v (end %v)", up, p.Top, p.End)
	}
}
