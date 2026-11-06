// Managed by GoX v0.3.3

//line days.gox:1
package aside

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"sync"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/aside/controller"
	"neui.dev/kchat/aside/window"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/notify"
)

const margin = 12

const probe = 100

type days struct {
	mu sync.Mutex
	aside *Aside
	window controller.Window
	save func(ctx context.Context, w controller.Window)
	name string
	pinned api.ThreadID
	active api.ThreadID
}

//line days.gox:34
func (d *days) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line days.gox:35
		__e = __c.Any(&window.WindowList{
		Margin: margin,
		Driver: d,
		InitialWindow: d.window,
		SaveWindow: d.save,
	}); if __e != nil { return }
	return })
//line days.gox:41
}

func (d *days) Compare(a string, b string) int {
	x, _ := strconv.ParseInt(a, 10, 64)
	y, _ := strconv.ParseInt(b, 10, 64)
	return cmp.Compare(x, y)
}

func (d *days) loc() *time.Location {
	return d.aside.Instance.Location()
}

func (d *days) today() time.Time {
	return todayIn(d.loc())
}

func (d *days) Sections(ctx context.Context, from string, inclusive bool, asc bool, limit int) ([]window.Section, bool, error) {
	loc := d.loc()
	today := todayIn(loc)
	at, _ := strconv.ParseInt(from, 10, 64)
	bound := time.Time{}
	if at != 0 {
		bound = time.Unix(at, 0).In(loc)
		if asc != inclusive {
			bound = bound.AddDate(0, 0, 1)
		}
	}
	keys := []time.Time{}
	rows := map[time.Time][]api.Thread{}
	cut := map[time.Time]bool{}
	ended := true
	walk: for {
		var threads []api.Thread
		var err error
		if asc {
			threads, err = d.aside.Instance.ThreadList(ctx, bound, time.Time{}, true, probe, d.name)
		} else {
			threads, err = d.aside.Instance.ThreadList(ctx, time.Time{}, bound, false, probe, d.name)
		}
		if err != nil {
			return nil, false, err
		}
		for _, t := range threads {
			y, m, dd := t.Updated.In(loc).Date()
			k := time.Date(y, m, dd, 0, 0, 0, 0, loc)
			if len(keys) == 0 || !keys[len(keys) - 1].Equal(k) {
				if len(keys) == limit {
					ended = false
					break walk
				}
				keys = append(keys, k)
			}
			rows[k] = append(rows[k], t)
		}
		if len(threads) < probe || len(keys) == 0 {
			break
		}
		last := keys[len(keys) - 1]
		cut[last] = true
		if asc {
			bound = last.AddDate(0, 0, 1)
		} else {
			bound = last
		}
	}
	if d.name == "" {
		if asc {
			reach := at == 0 || today.Unix() > at || (inclusive && today.Unix() == at)
			if reach && (len(keys) == 0 || !keys[len(keys) - 1].Equal(today)) {
				if len(keys) == limit {
					ended = false
				} else {
					keys = append(keys, today)
				}
			}
		} else if at == 0 && (len(keys) == 0 || !keys[0].Equal(today)) {
			keys = slices.Insert(keys, 0, today)
			if len(keys) > limit {
				keys = keys[:limit]
				ended = false
			}
		}
	}
	out := make([]window.Section, 0, len(keys))
	for _, k := range keys {
		var threads []api.Thread
		if !cut[k] {
			threads = rows[k]
			if asc {
				slices.Reverse(threads)
			}
		}
		out = append(out, window.Section{
			Key: strconv.FormatInt(k.Unix(), 10),
			Content: &day{
				days: d,
				start: k,
				end: k.AddDate(0, 0, 1),
				body: new(doors.Door),
				seeded: !cut[k],
				rows: threads,
			},
		})
	}
	return out, ended, nil
}

type day struct {
	days *days
	start time.Time
	end time.Time
	body *doors.Door
	seeded bool
	rows []api.Thread
}

//line days.gox:157
func (d *day) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line days.gox:159
		d.body.Inner(ctx, d.content())

//line days.gox:161
		__e = __c.Any(d.body); if __e != nil { return }
	return })
//line days.gox:162
}

//line days.gox:164
func (d *day) content() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line days.gox:166
		today := d.days.name == "" && d.start.Equal(d.days.today())
	label := ""
	switch {
	case today:
		label = "Today"
	case d.start.Year() == time.Now().In(d.days.loc()).Year():
		label = d.start.Format("January 2")
	default:
		label = d.start.Format("January 2, 2006")
	}
	d.days.mu.Lock()
	threads, seeded := d.rows, d.seeded
	d.rows, d.seeded = nil, false
	d.days.mu.Unlock()
	failed := false
	if !seeded {
		var err error
		threads, err = d.days.aside.Instance.ThreadList(ctx, d.start, d.end, false, 0, d.days.name)
		if err != nil {
			doors.Logger(ctx).Error("aside: day", "day", label, "err", err)
			failed = true
		}
	}
	var all map[api.ThreadID]api.InstanceChat
	all, _ = d.days.aside.Instance.Chats().ReadAndSub(ctx, func(ctx context.Context, m map[api.ThreadID]api.InstanceChat) bool {
		if !today && !failed {
			same := true
			for _, t := range threads {
				if m[t.ID] != all[t.ID] {
					same = false
					break
				}
			}
			if same {
				return false
			}
		}
		d.body.Reload(ctx)
		return false
	})
	fresh := false
	pinned := api.ThreadID("")
	if today {
		active, latest := api.ThreadID(""), api.ThreadID("")
		var at time.Time
		for id, chat := range all {
			if chat.InstanceID == d.days.aside.Instance.ID() {
				active = id
				fresh = chat.State.Thread.New
			}
			if chat.State.Thread.Updated.After(at) {
				at = chat.State.Thread.Updated
				latest = id
			}
		}
		d.days.mu.Lock()
		if d.days.active != active {
			d.days.pinned = ""
		}
		d.days.active = active
		if active != "" && active == latest {
			d.days.pinned = active
		}
		pinned = d.days.pinned
		d.days.mu.Unlock()
		if pinned != "" {
			i := slices.IndexFunc(threads, func(t api.Thread) bool {
				return t.ID == pinned
			})
			if i > 0 {
				t := threads[i]
				copy(threads[1:i + 1], threads[:i])
				threads[0] = t
			}
		}
	}

//line days.gox:243
		if today || len(threads) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line days.gox:244
				__e = __c.Set("class", "kchat-side-label"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line days.gox:244
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
//line days.gox:246
		if today {
//line days.gox:247
			if fresh {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line days.gox:248
					__e = __c.Set("class", "kchat-row"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line days.gox:249
						__e = __c.Set("class", "kchat-thread kchat-thread_active"); if __e != nil { return }
//line days.gox:249
						__e = __c.Set("aria-current", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line days.gox:250
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line days.gox:250
							__e = __c.Any(icons.Plus()); if __e != nil { return }
						return })); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line days.gox:251
							__e = __c.Set("class", "kchat-thread-name"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("New chat"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line days.gox:255
					__e = __c.Set("class", "kchat-row"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line days.gox:256
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						d.days.aside.Instance.ThreadNew(ctx)
						rp.After(doors.ActionEmit[any]{Name: "opened"})
						return false
					},
					OnError: notify.OnError,
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line days.gox:264
							__e = __c.Set("class", "kchat-thread kchat-thread_new"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line days.gox:265
							__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line days.gox:265
								__e = __c.Any(icons.Plus()); if __e != nil { return }
							return })); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
//line days.gox:266
								__e = __c.Set("class", "kchat-thread-name"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("New chat"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
//line days.gox:271
		for _, t := range threads {
//line days.gox:272
			__e = __c.Any(&row{aside: d.days.aside, thread: t, instance: all[t.ID].InstanceID, day: d.body}); if __e != nil { return }
		}
	return })
//line days.gox:274
}
