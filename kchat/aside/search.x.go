// Managed by GoX v0.3.3

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
	"neui.dev/kchat/anchor"
	"neui.dev/kchat/aside/controller"
	"neui.dev/kchat/aside/window"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/notify"
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
	if failed != nil {
		notify.Error(ctx, failed)
	}

//line search.gox:54
		if len(threads) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:55
				__e = __c.Set("class", "kchat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Chats"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:56
			for _, t := range threads[:min(len(threads), overview)] {
//line search.gox:57
				__e = __c.Any(p.match(t)); if __e != nil { return }
			}
//line search.gox:59
			if len(threads) > overview {
//line search.gox:60
				__e = __c.Any(p.more(open, controller.SearchNames)); if __e != nil { return }
			}
		}
//line search.gox:63
		if len(mine) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:64
				__e = __c.Set("class", "kchat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Messages here"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:65
			for _, h := range mine[:min(len(mine), overview)] {
//line search.gox:66
				__e = __c.Any(p.hit(h, loc, false)); if __e != nil { return }
			}
//line search.gox:68
			if len(mine) > overview {
//line search.gox:69
				__e = __c.Any(p.more(open, controller.SearchMessagesCurrent)); if __e != nil { return }
			}
		}
//line search.gox:72
		if len(others) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line search.gox:73
				__e = __c.Set("class", "kchat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Other messages"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:74
			for _, h := range others[:min(len(others), overview)] {
//line search.gox:75
				__e = __c.Any(p.hit(h, loc, true)); if __e != nil { return }
			}
//line search.gox:77
			if len(others) > overview {
//line search.gox:78
				__e = __c.Any(p.more(open, controller.SearchMessagesAll)); if __e != nil { return }
			}
		}
//line search.gox:81
		if failed != nil {
//line search.gox:82
			__e = __c.Any(retry(body)); if __e != nil { return }
//line search.gox:83
		} else if len(threads) == 0 && len(mine) == 0 && len(others) == 0 {
//line search.gox:84
			__e = __c.Any(empty("No chats or messages match", "Try fewer words")); if __e != nil { return }
		}
	return })
//line search.gox:86
}

//line search.gox:88
func (p *Aside) noResults(kind controller.SearchKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:90
		text := "No chats or messages match"
	switch kind {
	case controller.SearchNames:
		text = "No chats match"
	case controller.SearchMessagesCurrent:
		text = "No messages in this chat match"
	case controller.SearchMessagesAll:
		text = "No messages in other chats match"
	}

//line search.gox:100
		__e = __c.Any(empty(text, "")); if __e != nil { return }
	return })
//line search.gox:101
}

//line search.gox:103
func retry(door *doors.Door) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:104
			__e = __c.Set("class", "kchat-side-retry"); if __e != nil { return }
//line search.gox:104
			__e = __c.Set("role", "alert"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Couldn't load."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:106
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
//line search.gox:113
					__e = __c.Set("type", "button"); if __e != nil { return }
//line search.gox:113
					__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Retry"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:115
}

//line search.gox:117
func empty(text string, hint string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:118
			__e = __c.Set("class", "kchat-side-empty"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line search.gox:119
				__e = __c.Any(text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line search.gox:120
			if hint != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line search.gox:121
					__e = __c.Set("class", "kchat-side-empty-hint"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line search.gox:121
					__e = __c.Any(hint); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:124
}

//line search.gox:126
func (p *Aside) more(open func(ctx context.Context, kind controller.SearchKind), kind controller.SearchKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line search.gox:127
			__e = __c.Set("class", "kchat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line search.gox:128
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
//line search.gox:135
					__e = __c.Set("class", "kchat-thread kchat-thread_new"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line search.gox:136
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line search.gox:136
						__e = __c.Any(icons.ArrowRight()); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:137
						__e = __c.Set("class", "kchat-thread-name"); if __e != nil { return }
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
//line search.gox:140
}

//line search.gox:142
func (p *Aside) hit(h api.Hit, loc *time.Location, named bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:143
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			if err := p.Instance.ThreadOpen(ctx, h.Thread, anchor.NewAnchor(h.Round)); err != nil {
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
//line search.gox:155
				__e = __c.Set("class", "kchat-hit"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line search.gox:156
				if named {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:157
						__e = __c.Set("class", "kchat-hit-head"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:158
						__e = __c.Any(p.hitName(h)); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line search.gox:159
							__e = __c.Set("class", "kchat-date"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line search.gox:159
							__e = __c.Any(shortDate(h.Created, loc)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:161
						__e = __c.Set("class", "kchat-hit-snippet"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:161
						__e = __c.Any(h.Snippet); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:163
						__e = __c.Set("class", "kchat-date kchat-date_over"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:163
						__e = __c.Any(shortDate(h.Created, loc)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line search.gox:164
						__e = __c.Set("class", "kchat-hit-snippet kchat-hit-snippet_under"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line search.gox:164
						__e = __c.Any(h.Snippet); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line search.gox:167
}

//line search.gox:169
func (p *Aside) hitName(h api.Hit) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:171
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
//line search.gox:180
			__e = __c.Set("class", "kchat-hit-name"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line search.gox:180
			__e = __c.Any(n); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line search.gox:181
}

type hits struct {
	aside *Aside
	window controller.Window
	save func(ctx context.Context, w controller.Window)
	thread api.ThreadID
	exclude bool
	query string
}

//line search.gox:192
func (h *hits) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:193
		__e = __c.Any(&window.WindowList{
		Margin: margin,
		Driver: h,
		InitialWindow: h.window,
		SaveWindow: h.save,
	}); if __e != nil { return }
	return })
//line search.gox:199
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

//line search.gox:249
func (r *hitRow) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:250
		__e = __c.Any(r.hits.aside.hit(r.hit, r.hits.aside.Instance.Location(), r.hits.exclude)); if __e != nil { return }
	return })
//line search.gox:251
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

//line search.gox:274
func (n *names) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line search.gox:275
		__e = __c.Any(&days{aside: n.aside, window: n.window, save: n.save, name: n.query}); if __e != nil { return }
	return })
//line search.gox:276
}
