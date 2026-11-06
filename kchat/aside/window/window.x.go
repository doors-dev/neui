// Managed by GoX v0.3.3

//line window.gox:1
package window

import (
	"context"
	"slices"
	"sync"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"neui.dev/kchat/aside/controller"
	"neui.dev/kchat/notify"
)

type Section struct {
	Key string
	Content gox.Comp
}

type Driver interface {
	Compare(a string, b string) int
	Sections(ctx context.Context, from string, inclusive bool, asc bool, limit int) ([]Section, bool, error)
}

type slot struct {
	key string
	batch int
	after any
	door *doors.Door
	content gox.Comp
}

func (s *slot) remove(ctx context.Context) {
	s.door.Static(ctx, nil)
}

//line window.gox:38
func (s *slot) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line window.gox:40
		var batch any
	if s.batch != 0 {
		batch = s.batch
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line window.gox:45
			__e = __c.Set("class", "kchat-day"); if __e != nil { return }
//line window.gox:45
			__e = __c.Set("data-id", s.key); if __e != nil { return }
//line window.gox:45
			__e = __c.Set("data-batch", batch); if __e != nil { return }
//line window.gox:45
			__e = __c.Set("data-pending", s.batch != 0); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line window.gox:46
			__e = __c.Any(s.content); if __e != nil { return }
//line window.gox:47
			__e = __c.Any(s.after); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line window.gox:49
}

type layout struct {
	prepender gox.Comp
	history []*slot
	appender gox.Comp
}

//line window.gox:57
func (l *layout) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line window.gox:58
		__e = __c.Any(l.prepender); if __e != nil { return }
//line window.gox:59
		for _, s := range l.history {
//line window.gox:60
			__e = __c.Any(s.door); if __e != nil { return }
		}
//line window.gox:62
		__e = __c.Any(l.appender); if __e != nil { return }
	return })
//line window.gox:63
}

type WindowList struct {
	InitialWindow controller.Window
	SaveWindow func(context.Context, controller.Window)
	Driver Driver
	Margin int
	mu *sync.Mutex
	ctx context.Context
	history deque.Deque[*slot]
	prepender *doors.Door
	appender *doors.Door
	first string
	last string
	batch int
	scrollState struct {
		first string
		last string
		synced bool
		spawned bool
		newestReached bool
		oldestReached bool
	}
}

//line window.gox:88
func (w WindowList) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line window.gox:90
		body := new(doors.Door)
	body.Inner(ctx, w.body(body))

//line window.gox:93
		__e = __c.Any(body); if __e != nil { return }
	return })
//line window.gox:94
}

//line window.gox:96
func (w WindowList) body(door *doors.Door) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line window.gox:98
		w.mu = new(sync.Mutex)
	s, err := w.seed(ctx)

//line window.gox:101
		if err == nil {
//line window.gox:102
			__e = __c.Any(s); if __e != nil { return }
//line window.gox:103
			__e = __c.Any(w.script()); if __e != nil { return }
		} else  {
//line window.gox:106
			doors.Logger(ctx).Error("window: seed", "err", err)

			__e = __c.Init("div"); if __e != nil { return }
			{
//line window.gox:108
				__e = __c.Set("class", "kchat-side-retry"); if __e != nil { return }
//line window.gox:108
				__e = __c.Set("role", "alert"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Couldn't load."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line window.gox:110
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
//line window.gox:117
						__e = __c.Set("type", "button"); if __e != nil { return }
//line window.gox:117
						__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Retry"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line window.gox:120
}

func (w *WindowList) seed(ctx context.Context) (gox.Comp, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	w.ctx = doors.DetachedContext(ctx)
	w.first, w.last = w.InitialWindow.First, w.InitialWindow.Last
	w.prepender = new(doors.Door)
	w.appender = new(doors.Door)
	w.scrollState.synced = true
	w.scrollState.newestReached = true
	above := []Section{}
	if w.last != "" {
		newer, ended, err := w.Driver.Sections(ctx, w.last, false, true, w.Margin)
		if err != nil {
			return nil, err
		}
		slices.Reverse(newer)
		above = newer
		w.scrollState.newestReached = ended
	}
	sections := []Section{}
	if w.last == "" {
		fresh, ended, err := w.Driver.Sections(ctx, "", true, false, 2 * w.Margin)
		if err != nil {
			return nil, err
		}
		sections = fresh
		w.scrollState.oldestReached = ended
	} else {
		from := w.last
		take := true
		beyond := 0
		for {
			fresh, ended, err := w.Driver.Sections(ctx, from, take, false, w.Margin)
			if err != nil {
				return nil, err
			}
			for _, s := range fresh {
				sections = append(sections, s)
				if w.Driver.Compare(s.Key, w.first) < 0 {
					beyond++
				}
			}
			if ended {
				w.scrollState.oldestReached = true
				break
			}
			if len(fresh) == 0 || beyond >= w.Margin {
				break
			}
			from = sections[len(sections) - 1].Key
			take = false
		}
	}
	for _, s := range above {
		w.history.PushBack(w.intoSlot(w.ctx, s, 0, nil))
	}
	for _, s := range sections {
		w.history.PushBack(w.intoSlot(w.ctx, s, 0, nil))
	}
	if w.scrollState.newestReached && w.history.Len() != 0 && w.history.Front().key == w.last {
		w.first, w.last = "", ""
	}
	return doors.Once(&layout{
		prepender: w.prepender,
		history: slices.Collect(w.history.Iter()),
		appender: w.appender,
	}), nil
}

func (w *WindowList) intoSlot(ctx context.Context, s Section, batch int, after any) *slot {
	slot := &slot{
		key: s.Key,
		batch: batch,
		after: after,
		door: new(doors.Door),
		content: s.Content,
	}
	slot.door.Outer(ctx, doors.Once(slot))
	return slot
}

func (w *WindowList) updateRange(ctx context.Context, first string, last string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if first == w.scrollState.first && last == w.scrollState.last {
		return
	}
	w.scrollState.first = first
	w.scrollState.last = last
	if w.SaveWindow != nil {
		w.SaveWindow(ctx, controller.Window{First: first, Last: last})
	}
	w.scrollState.synced = false
	if w.scrollState.spawned {
		return
	}
	w.scrollState.spawned = true
	go w.syncLoop()
}

func (w *WindowList) syncLoop() {
	for {
		applied := make(chan struct{})
		more := false
		doors.OnBatch(w.ctx, func(context.Context) {
			close(applied)
		}, func(ctx context.Context) {
			w.mu.Lock()
			more = w.sync(ctx)
			if !more {
				w.scrollState.spawned = false
			}
			w.mu.Unlock()
		})
		if !more {
			return
		}
		<-applied
	}
}

func (w *WindowList) sync(ctx context.Context) (more bool) {
	if w.scrollState.synced {
		return
	}
	first, last := w.scrollState.first, w.scrollState.last
	if first == "" {
		w.scrollState.synced = true
		return
	}
	newer, older := 0, 0
	for s := range w.history.Iter() {
		if w.Driver.Compare(s.key, last) > 0 {
			newer++
		}
		if w.Driver.Compare(s.key, first) < 0 {
			older++
		}
	}
	wantNewer, wantOlder := w.Margin, w.Margin
	if w.scrollState.newestReached && newer < w.Margin {
		wantOlder += w.Margin - newer
	}
	if w.scrollState.oldestReached && older < w.Margin {
		wantNewer += w.Margin - older
	}
	if older < wantOlder && !w.scrollState.oldestReached && w.history.Len() != 0 {
		anchor := w.history.Back().key
		w.mu.Unlock()
		sections, ended, err := w.Driver.Sections(ctx, anchor, false, false, wantOlder - older)
		w.mu.Lock()
		if err != nil {
			doors.Logger(ctx).Error("window: load older", "err", err)
			notify.Error(ctx, err)
			w.scrollState.first = ""
			w.scrollState.last = ""
			w.scrollState.synced = true
			return
		}
		w.batch++
		w.scrollState.oldestReached = ended || len(sections) == 0
		fresh := make([]*slot, 0, len(sections))
		for i, s := range sections {
			var after any
			if i == len(sections) - 1 {
				after = restoreScroll(w.batch, "bottom")
			}
			fresh = append(fresh, w.intoSlot(ctx, s, w.batch, after))
		}
		for _, s := range fresh {
			w.history.PushBack(s)
		}
		if len(fresh) != 0 {
			old := w.appender
			w.appender = new(doors.Door)
			old.Static(ctx, doors.Once(&layout{
				history: fresh,
				appender: w.appender,
			}))
		}
		return true
	}
	if newer < wantNewer && !w.scrollState.newestReached && w.history.Len() != 0 {
		anchor := w.history.Front().key
		w.mu.Unlock()
		sections, ended, err := w.Driver.Sections(ctx, anchor, false, true, wantNewer - newer)
		w.mu.Lock()
		if err != nil {
			doors.Logger(ctx).Error("window: load newer", "err", err)
			notify.Error(ctx, err)
			w.scrollState.first = ""
			w.scrollState.last = ""
			w.scrollState.synced = true
			return
		}
		w.batch++
		w.scrollState.newestReached = ended || len(sections) == 0
		fresh := make([]*slot, 0, len(sections))
		for i, s := range slices.Backward(sections) {
			var after any
			if i == len(sections) - 1 {
				after = restoreScroll(w.batch, "top")
			}
			fresh = append(fresh, w.intoSlot(ctx, s, w.batch, after))
		}
		for _, f := range slices.Backward(fresh) {
			w.history.PushFront(f)
		}
		if len(fresh) != 0 {
			old := w.prepender
			w.prepender = new(doors.Door)
			old.Static(ctx, doors.Once(&layout{
				prepender: w.prepender,
				history: fresh,
			}))
		}
		return true
	}
	for newer > wantNewer {
		w.history.PopFront().remove(ctx)
		newer--
		w.scrollState.newestReached = false
	}
	for older > wantOlder {
		w.history.PopBack().remove(ctx)
		older--
		w.scrollState.oldestReached = false
	}
	w.scrollState.synced = true
	return
}

//line window.gox:355
func (w *WindowList) script() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line window.gox:356
		__e = (doors.AHook[syncMsg]{
		Name: "sync",
		Scope: &doors.ScopeRate{Tick: 100 * time.Millisecond},
		Race: doors.RaceStrict,
		On: func(ctx context.Context, r doors.RequestHook[syncMsg]) (any, bool) {
			msg := r.Data()
			w.updateRange(ctx, msg.First, msg.Last)
			return nil, false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("script"); if __e != nil { return }
			{
//line window.gox:365
				__e = __c.Set("data:first", w.first); if __e != nil { return }
//line window.gox:365
				__e = __c.Set("data:last", w.last); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const script = document.currentScript\n\t\tconst body = script.closest(\".kchat-side-body\")\n\t\tconst seedFirst = $data(\"first\")\n\t\tconst seedLast = $data(\"last\")\n\t\tlet sent = seedLast ? `${seedFirst}:${seedLast}` : \"\"\n\t\tconst widget = body.closest(\".kchat-widget\")\n\t\tlet reported = false\n\t\tconst report = () => {\n\t\t\tconst b = body.getBoundingClientRect()\n\t\t\tlet first = \"\"\n\t\t\tlet last = \"\"\n\t\t\tfor (const s of body.querySelectorAll(\".kchat-day[data-id]\")) {\n\t\t\t\tif (s.hasAttribute(\"data-pending\")) continue\n\t\t\t\tconst r = s.getBoundingClientRect()\n\t\t\t\tif (r.height === 0 || r.bottom <= b.top || r.top >= b.bottom) continue\n\t\t\t\tif (last === \"\") last = s.dataset.id\n\t\t\t\tfirst = s.dataset.id\n\t\t\t}\n\t\t\tif (first === \"\") return\n\t\t\tconst key = `${first}:${last}`\n\t\t\tif (key === sent) return\n\t\t\tsent = key\n\t\t\t$hook(\"sync\", { first, last }).then(\n\t\t\t\t() => {\n\t\t\t\t\treported = false\n\t\t\t\t},\n\t\t\t\t(err) => {\n\t\t\t\t\tsent = \"\"\n\t\t\t\t\tif (reported || !(err instanceof HookErr) || !err.network()) return\n\t\t\t\t\treported = true\n\t\t\t\t\twidget.dispatchEvent(new CustomEvent(\"kchat_error\", { detail: err.kind }))\n\t\t\t\t},\n\t\t\t)\n\t\t}\n\t\tconst edge = (e) => {\n\t\t\tconst top = e.deltaY < 0 && body.scrollTop <= 0\n\t\t\tconst bottom =\n\t\t\t\te.deltaY > 0 && body.scrollTop + body.clientHeight >= body.scrollHeight - 1\n\t\t\tif (!top && !bottom) return\n\t\t\tsent = \"\"\n\t\t\treport()\n\t\t}\n\t\t$sys.clean(() => {\n\t\t\tbody.removeEventListener(\"scroll\", report)\n\t\t\tbody.removeEventListener(\"wheel\", edge)\n\t\t})\n\t\tawait $sys.ready()\n\t\trequestAnimationFrame(() => {\n\t\t\tconst top = body.querySelector(`.kchat-day[data-id=\"${seedLast}\"]`)\n\t\t\tif (seedLast && top) {\n\t\t\t\tconst base = body.scrollTop - body.getBoundingClientRect().top\n\t\t\t\tconst t = top.getBoundingClientRect()\n\t\t\t\tlet lo = t.top + base\n\t\t\t\tlet hi = t.bottom + base\n\t\t\t\tconst bottom = body.querySelector(`.kchat-day[data-id=\"${seedFirst}\"]`)\n\t\t\t\tif (bottom) {\n\t\t\t\t\tconst f = bottom.getBoundingClientRect()\n\t\t\t\t\tlo = Math.max(lo, f.top + base - body.clientHeight)\n\t\t\t\t\thi = Math.min(hi, f.bottom + base - body.clientHeight)\n\t\t\t\t}\n\t\t\t\tbody.scrollTop = lo <= hi ? (lo + hi) / 2 : t.top + base\n\t\t\t}\n\t\t\tbody.addEventListener(\"scroll\", report, { passive: true })\n\t\t\tbody.addEventListener(\"wheel\", edge, { passive: true })\n\t\t\treport()\n\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line window.gox:433
}

type syncMsg struct {
	First string `json:"first"`
	Last string `json:"last"`
}

//line window.gox:440
func restoreScroll(batch int, edge string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line window.gox:441
			__e = __c.Set("data:batch", batch); if __e != nil { return }
//line window.gox:441
			__e = __c.Set("data:edge", edge); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const script = document.currentScript\n\t\tconst body = script.closest(\".kchat-side-body\")\n\t\tconst top = body.scrollTop\n\t\tconst h0 = body.scrollHeight\n\t\tfor (const s of body.querySelectorAll(\n\t\t\t`.kchat-day[data-batch=\"${$data(\"batch\")}\"]`,\n\t\t)) {\n\t\t\ts.removeAttribute(\"data-pending\")\n\t\t}\n\t\tconst grew = body.scrollHeight - h0\n\t\tif ($data(\"edge\") === \"top\") {\n\t\t\tbody.scrollTo({ top: top + grew, behavior: \"instant\" })\n\t\t} else {\n\t\t\tbody.scrollTo({ top, behavior: \"instant\" })\n\t\t}\n\t\tbody.dispatchEvent(new Event(\"scroll\"))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line window.gox:459
}
