// Managed by GoX v0.3.4

//line search.gox:1
package aside

import (
	"cmp"
	"context"
	"strconv"
	"strings"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nechat/anchor"
	"neui.dev/nechat/aside/controller"
	"neui.dev/nechat/aside/window"
	"neui.dev/nechat/icons"
	"neui.dev/nechat/notify"
)

const overview = 5

//line search.gox:22
func (p *Aside) overview(input string, open func(ctx context.Context, kind controller.SearchKind)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:24
		body := new(doors.Door)
	body.Inner(ctx, p.overviewBody(body, input, open))

//line search.gox:27
		__e = __c.Any(body); if __e != nil { return }
	return })
//line search.gox:28
}

//line search.gox:30
func (p *Aside) overviewBody(body *doors.Door, input string, open func(ctx context.Context, kind controller.SearchKind)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:32
		loc := p.Instance.Location()
	current := p.Instance.State().Get().Thread.ID
	var failed error
	threads, err := p.Instance.ThreadList(ctx, time.Time{}, time.Time{}, false, overview + 1, input)
	if err != nil {
		doors.Logger(ctx).Error("aside: search names", "err", err)
		failed = err
	}
	mine, err := p.Instance.RoundSearch(ctx, current, false, input, nil, false, false, overview + 1)
	if err != nil {
		doors.Logger(ctx).Error("aside: search current", "err", err)
		failed = err
	}
	others, err := p.Instance.RoundSearch(ctx, current, true, input, nil, false, false, overview + 1)
	if err != nil {
		doors.Logger(ctx).Error("aside: search others", "err", err)
		failed = err
	}

//line search.gox:51
		if len(threads) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:52
				__e = __c.Set("class", "nechat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Chats"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:53
			for _, t := range threads[:min(len(threads), overview)] {
//line search.gox:54
				__e = __c.Any(p.match(t)); if __e != nil { return }
			}
//line search.gox:56
			if len(threads) > overview {
//line search.gox:57
				__e = __c.Any(p.more(open, controller.SearchNames)); if __e != nil { return }
			}
		}
//line search.gox:60
		if len(mine) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:61
				__e = __c.Set("class", "nechat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Messages here"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:62
			for _, h := range mine[:min(len(mine), overview)] {
//line search.gox:63
				__e = __c.Any(p.hit(h, loc, false)); if __e != nil { return }
			}
//line search.gox:65
			if len(mine) > overview {
//line search.gox:66
				__e = __c.Any(p.more(open, controller.SearchMessagesCurrent)); if __e != nil { return }
			}
		}
//line search.gox:69
		if len(others) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:70
				__e = __c.Set("class", "nechat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Other messages"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:71
			for _, h := range others[:min(len(others), overview)] {
//line search.gox:72
				__e = __c.Any(p.hit(h, loc, true)); if __e != nil { return }
			}
//line search.gox:74
			if len(others) > overview {
//line search.gox:75
				__e = __c.Any(p.more(open, controller.SearchMessagesAll)); if __e != nil { return }
			}
		}
//line search.gox:78
		if failed != nil {
//line search.gox:79
			__e = __c.Any(retry(body)); if __e != nil { return }
//line search.gox:80
		} else if len(threads) == 0 && len(mine) == 0 && len(others) == 0 {
//line search.gox:81
			__e = __c.Any(empty("No chats or messages match", "Try fewer words")); if __e != nil { return }
		}
	return })
//line search.gox:83
}

//line search.gox:85
func (p *Aside) noResults(kind controller.SearchKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:87
		text := "No chats or messages match"
	switch kind {
	case controller.SearchNames:
		text = "No chats match"
	case controller.SearchMessagesCurrent:
		text = "No messages in this chat match"
	case controller.SearchMessagesAll:
		text = "No messages in other chats match"
	}

//line search.gox:97
		__e = __c.Any(empty(text, "")); if __e != nil { return }
	return })
//line search.gox:98
}

//line search.gox:100
func retry(door *doors.Door) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:101
			__e = __c.Set("class", "nechat-side-retry"); if __e != nil { return }
//line search.gox:101
			__e = __c.Set("role", "alert"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Couldn't load."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:103
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				door.Reload(ctx)
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line search.gox:110
					__e = __c.Set("type", "button"); if __e != nil { return }
//line search.gox:110
					__e = __c.Set("class", "nechat-btn nechat-btn_2nd"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Retry"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:112
}

//line search.gox:114
func empty(text string, hint string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:115
			__e = __c.Set("class", "nechat-side-empty"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line search.gox:116
				__e = __c.Any(text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:117
			if hint != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line search.gox:118
					__e = __c.Set("class", "nechat-side-empty-hint"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line search.gox:118
					__e = __c.Any(hint); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:121
}

//line search.gox:123
func (p *Aside) more(open func(ctx context.Context, kind controller.SearchKind), kind controller.SearchKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:124
			__e = __c.Set("class", "nechat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line search.gox:125
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				open(ctx, kind)
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line search.gox:132
					__e = __c.Set("class", "nechat-thread nechat-thread_new"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line search.gox:133
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line search.gox:133
						__e = __c.Any(icons.ArrowRight()); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:134
						__e = __c.Set("class", "nechat-thread-name"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Show more"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:137
}

//line search.gox:139
func (p *Aside) hit(h api.Hit, loc *time.Location, named bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:140
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			if err := p.Instance.ThreadOpen(ctx, h.Thread, anchor.New(h.Round)); err != nil {
				doors.Logger(ctx).Error("aside: open", "thread", h.Thread, "err", err)
				notify.Error(ctx, err)
				return false
			}
			rp.After(doors.ActionEmit[any]{Name: "opened"})
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line search.gox:152
				__e = __c.Set("class", "nechat-hit"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line search.gox:153
				if named {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:154
						__e = __c.Set("class", "nechat-hit-head"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:155
						__e = __c.Any(p.hitName(h)); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line search.gox:156
							__e = __c.Set("class", "nechat-date"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line search.gox:156
							__e = __c.Any(shortDate(h.Created, loc)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:158
						__e = __c.Set("class", "nechat-hit-snippet"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:158
						__e = __c.Any(h.Snippet); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:160
						__e = __c.Set("class", "nechat-date nechat-date_over"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:160
						__e = __c.Any(shortDate(h.Created, loc)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:161
						__e = __c.Set("class", "nechat-hit-snippet nechat-hit-snippet_under"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:161
						__e = __c.Any(h.Snippet); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line search.gox:164
}

//line search.gox:166
func (p *Aside) hitName(h api.Hit) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:168
		t, err := p.Instance.ThreadGet(ctx, h.Thread)
	if err != nil {
		doors.Logger(ctx).Error("aside: hit thread", "thread", h.Thread, "err", err)
	}
	n := t.Name
	if n == "" {
		n = "Untitled"
	}

		__e = __c.Init("span"); if __e != nil { return }
		{
//line search.gox:177
			__e = __c.Set("class", "nechat-hit-name"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line search.gox:177
			__e = __c.Any(n); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:178
}

type hits struct {
	aside *Aside
	window controller.Window
	save func(ctx context.Context, w controller.Window)
	thread api.ThreadID
	exclude bool
	query string
}

//line search.gox:189
func (h *hits) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:190
		__e = __c.Any(&window.List{
		Margin: margin,
		Driver: h,
		InitialWindow: h.window,
		SaveWindow: h.save,
	}); if __e != nil { return }
	return })
//line search.gox:196
}

func (h *hits) Compare(a string, b string) int {
	x, y := hitCursor(a), hitCursor(b)
	if x == nil || y == nil {
		return strings.Compare(a, b)
	}
	if c := x.Created.Compare(y.Created); c != 0 {
		return c
	}
	if c := strings.Compare(string(x.Thread), string(y.Thread)); c != 0 {
		return c
	}
	return cmp.Compare(x.Round, y.Round)
}

func (h *hits) Sections(ctx context.Context, from string, inclusive bool, asc bool, limit int) ([]window.Section, bool, error) {
	found, err := h.aside.Instance.RoundSearch(ctx, h.thread, h.exclude, h.query, hitCursor(from), inclusive, asc, limit)
	if err != nil {
		return nil, false, err
	}
	out := make([]window.Section, 0, len(found))
	for _, f := range found {
		out = append(out, window.Section{
			Key: strconv.FormatInt(f.Created.UnixNano(), 10) + ":" + strconv.Itoa(int(f.Round)) + ":" + string(f.Thread),
			Content: &hitRow{hits: h, hit: f},
		})
	}
	return out, len(found) < limit, nil
}

func hitCursor(key string) *api.Cursor {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return nil
	}
	at, _ := strconv.ParseInt(parts[0], 10, 64)
	round, _ := strconv.Atoi(parts[1])
	return &api.Cursor{
		Created: time.Unix(0, at),
		Thread: api.ThreadID(parts[2]),
		Round: api.RoundID(round),
	}
}

type hitRow struct {
	hits *hits
	hit api.Hit
}

//line search.gox:246
func (r *hitRow) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:247
		__e = __c.Any(r.hits.aside.hit(r.hit, r.hits.aside.Instance.Location(), r.hits.exclude)); if __e != nil { return }
	return })
//line search.gox:248
}

func shortDate(t time.Time, loc *time.Location) string {
	u := t.In(loc)
	if u.Year() == time.Now().In(loc).Year() {
		return u.Format("Jan 2")
	}
	return u.Format("Jan 2, 2006")
}

func todayIn(loc *time.Location) time.Time {
	now := time.Now().In(loc)
	y, m, day := now.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, loc)
}

type names struct {
	aside *Aside
	window controller.Window
	save func(ctx context.Context, w controller.Window)
	query string
}

//line search.gox:271
func (n *names) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:272
		__e = __c.Any(&days{aside: n.aside, window: n.window, save: n.save, name: n.query, empty: n.aside.noResults(controller.SearchNames)}); if __e != nil { return }
	return })
//line search.gox:273
}
