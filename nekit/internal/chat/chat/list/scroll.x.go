// Managed by GoX v0.4.0

//line scroll.gox:1
// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package list

import (
	"context"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/notify"
)

func (l *list) updateRange(ctx context.Context, first api.RoundID, last api.RoundID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if first == l.scrollState.first && last == l.scrollState.last {
		return
	}
	l.scrollState.first = first
	l.scrollState.last = last
	l.scrollState.synced = false
	if l.scrollState.spawned {
		return
	}
	l.scrollState.spawned = true
	go l.syncLoop()
}

func (l *list) syncLoop() {
	for {
		applied := make(chan struct{})
		more := false
		doors.OnBatch(l.ctx, func(context.Context) {
			close(applied)
		}, func(ctx context.Context) {
			l.mu.Lock()
			more = l.sync(ctx)
			if !more {
				l.scrollState.spawned = false
			}
			l.mu.Unlock()
		})
		if !more {
			return
		}
		<-applied
	}
}

func (l *list) sync(ctx context.Context) (more bool) {
	if l.scrollState.synced {
		return
	}
	first, last := l.scrollState.first, l.scrollState.last
	if first == api.NoRoundID {
		l.scrollState.synced = true
		return
	}
	before, after := 0, 0
	for e := range l.history.Iter() {
		if e.id < first {
			before++
		}
		if e.id > last {
			after++
		}
	}
	wantBefore, wantAfter := margin, margin
	if l.scrollState.topReached && before < margin {
		wantAfter += margin - before
	}
	if l.scrollState.bottomReached && after < margin {
		wantBefore += margin - after
	}
	if before < wantBefore && !l.scrollState.topReached {
		seq := l.seq
		anchor, head := l.lastSection.entry.id, l.lastSection.entry.head
		if l.history.Len() != 0 {
			anchor, head = l.history.Front().id, l.history.Front().head
		}
		l.mu.Unlock()
		sections, ended, err := l.loadBefore(ctx, anchor, wantBefore - before)
		l.mu.Lock()
		if seq != l.seq {
			return true
		}
		if err != nil {
			doors.Logger(ctx).Error("list: load before", "err", err)
			notify.Error(ctx, err)
			l.scrollState.first = api.NoRoundID
			l.scrollState.last = api.NoRoundID
			l.scrollState.synced = true
			return
		}
		l.seq++
		l.batch++
		l.scrollState.topReached = ended || len(sections) == 0
		rounds := make([]entry, 0, len(sections))
		for i, s := range sections {
			var after any
			if i == 0 {
				after = restoreScroll(l.batch, "top")
			}
			next := head
			if i + 1 < len(sections) {
				next = sections[i + 1][0].Created
			}
			rounds = append(rounds, l.intoHistory(ctx, s, next, l.batch, after))
		}
		for i := len(rounds) - 1; i >= 0; i-- {
			l.history.PushFront(rounds[i])
		}
		if len(rounds) != 0 {
			old := l.historyPrepender
			l.historyPrepender = new(doors.Door)
			old.Static(ctx, doors.Once(&seed{
				prepender: l.historyPrepender,
				history: rounds,
			}))
		}
		return true
	}
	if after < wantAfter && !l.scrollState.bottomReached && l.history.Len() != 0 {
		seq := l.seq
		anchor := l.history.Back().id
		tail := l.lastSection.entry.id
		l.mu.Unlock()
		sections, ended, err := l.loadAfter(ctx, anchor, wantAfter - after, tail, true)
		l.mu.Lock()
		if seq != l.seq {
			return true
		}
		if err != nil {
			doors.Logger(ctx).Error("list: load after", "err", err)
			notify.Error(ctx, err)
			l.scrollState.first = api.NoRoundID
			l.scrollState.last = api.NoRoundID
			l.scrollState.synced = true
			return
		}
		l.seq++
		l.batch++
		if ended || len(sections) == 0 {
			l.scrollState.bottomReached = true
			l.showTail(ctx)
		}
		rounds := make([]entry, 0, len(sections))
		for i, s := range sections {
			var after any
			var next time.Time
			if i == len(sections) - 1 {
				after = restoreScroll(l.batch, "bottom")
				if ended {
					next = l.lastSection.entry.head
				}
			} else {
				next = sections[i + 1][0].Created
			}
			rounds = append(rounds, l.intoHistory(ctx, s, next, l.batch, after))
		}
		for _, e := range rounds {
			l.history.PushBack(e)
		}
		if len(rounds) != 0 {
			old := l.historyAppender
			l.historyAppender = new(doors.Door)
			old.Static(ctx, doors.Once(&seed{
				history: rounds,
				appender: l.historyAppender,
			}))
		}
		return true
	}
	for before > wantBefore {
		l.history.PopFront().remove(ctx)
		before--
		l.scrollState.topReached = false
	}
	for after > wantAfter {
		l.history.PopBack().remove(ctx)
		after--
		if l.scrollState.bottomReached {
			l.scrollState.bottomReached = false
			l.hideTail(ctx)
		}
	}
	l.scrollState.synced = true
	return
}

func (l *list) hideTail(ctx context.Context) {
	l.lastSection.hidden = true
	l.setTail(ctx, "hidden", true)
}

func (l *list) showTail(ctx context.Context) {
	l.lastSection.hidden = false
	l.setTail(ctx, "hidden", false)
}

func (l *list) setTail(ctx context.Context, name string, value any) {
	if l.lastSection.setter == nil {
		return
	}
	doors.Call(ctx, l.lastSection.setter.Set(name, value))
}

//line scroll.gox:222
func seedScroll(head api.RoundID) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line scroll.gox:223
			__e = __c.Set("data:head", int(head)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const log = document.currentScript.closest(\".nechat-log\")\n\t\tconst target = log.querySelector(`.nechat-section[data-id=\"${$data(\"head\")}\"]`)\n\t\tif (target) {\n\t\t\tconst base = log.scrollTop - log.getBoundingClientRect().top\n\t\t\tlog.scrollTo({\n\t\t\t\ttop: target.getBoundingClientRect().top + base,\n\t\t\t\tbehavior: \"instant\",\n\t\t\t})\n\t\t} else if (!log.querySelector(\".nechat-span\")) {\n\t\t\tlog.scrollTo({ top: log.scrollHeight, behavior: \"instant\" })\n\t\t}\n\t\trequestAnimationFrame(() => log.removeAttribute(\"data-seeding\"))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line scroll.gox:237
}

//line scroll.gox:239
func restoreScroll(batch int, edge string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line scroll.gox:240
			__e = __c.Set("data:batch", batch); if __e != nil { return }
//line scroll.gox:240
			__e = __c.Set("data:edge", edge); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const script = document.currentScript\n\t\tconst log = script.closest(\".nechat-log\")\n\t\tconst top = log.scrollTop\n\t\tconst h0 = log.scrollHeight\n\t\tfor (const s of log.querySelectorAll(\n\t\t\t`.nechat-section[data-batch=\"${$data(\"batch\")}\"]`,\n\t\t)) {\n\t\t\ts.removeAttribute(\"data-pending\")\n\t\t}\n\t\tconst grew = log.scrollHeight - h0\n\t\tif ($data(\"edge\") === \"top\") {\n\t\t\tlog.scrollTo({ top: top + grew, behavior: \"instant\" })\n\t\t} else {\n\t\t\tlog.scrollTo({ top, behavior: \"instant\" })\n\t\t}\n\t\tlog.dispatchEvent(new Event(\"scroll\"))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line scroll.gox:258
}

//line scroll.gox:260
func scrollBottom() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const log = document.currentScript.closest(\".nechat-log\")\n\t\tlog.scrollTo({ top: log.scrollHeight, behavior: \"instant\" })"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line scroll.gox:265
}

type syncMsg struct {
	First api.RoundID `json:"first"`
	Last api.RoundID `json:"last"`
}

//line scroll.gox:272
func (l List) Script() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line scroll.gox:273
		__e = (doors.AHook[syncMsg]{
		Name: "sync",
		Scope: &doors.ScopeRate{Tick: 100 * time.Millisecond},
		Race: doors.RaceStrict,
		On: func(ctx context.Context, r doors.RequestHook[syncMsg]) (any, bool) {
			msg := r.Data()
			l.updateRange(ctx, msg.First, msg.Last)
			return nil, false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const script = document.currentScript\n\t\tconst log = script.closest(\".nechat-log\")\n\t\tconst widget = log.closest(\".nechat-widget\")\n\t\tlet sent = \"\"\n\t\tlet reported = false\n\t\tconst report = () => {\n\t\t\tconst b = log.getBoundingClientRect()\n\t\t\tlet first = Infinity\n\t\t\tlet last = -Infinity\n\t\t\tfor (const s of log.querySelectorAll(\".nechat-section[data-id]\")) {\n\t\t\t\tconst r = s.getBoundingClientRect()\n\t\t\t\tif (r.height === 0 || r.bottom <= b.top || r.top >= b.bottom) continue\n\t\t\t\tconst id = Number(s.dataset.id)\n\t\t\t\tfirst = Math.min(first, id)\n\t\t\t\tlast = Math.max(last, id)\n\t\t\t}\n\t\t\tif (first === Infinity) return\n\t\t\tconst key = `${first}:${last}`\n\t\t\tif (key === sent) return\n\t\t\tsent = key\n\t\t\t$hook(\"sync\", { first, last }).then(\n\t\t\t\t() => {\n\t\t\t\t\treported = false\n\t\t\t\t},\n\t\t\t\t(err) => {\n\t\t\t\t\tsent = \"\"\n\t\t\t\t\tif (reported || !(err instanceof HookErr) || !err.network()) return\n\t\t\t\t\treported = true\n\t\t\t\t\twidget.dispatchEvent(new CustomEvent(\"nechat_error\", { detail: err.kind }))\n\t\t\t\t},\n\t\t\t)\n\t\t}\n\t\tconst edge = (e) => {\n\t\t\tconst top = e.deltaY < 0 && log.scrollTop <= 0\n\t\t\tconst bottom =\n\t\t\t\te.deltaY > 0 && log.scrollTop + log.clientHeight >= log.scrollHeight - 1\n\t\t\tif (!top && !bottom) return\n\t\t\tsent = \"\"\n\t\t\treport()\n\t\t}\n\t\tawait $sys.ready()\n\t\trequestAnimationFrame(() => {\n\t\t\tlog.addEventListener(\"scroll\", report, { passive: true })\n\t\t\tlog.addEventListener(\"wheel\", edge, { passive: true })\n\t\t\treport()\n\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line scroll.gox:330
}
