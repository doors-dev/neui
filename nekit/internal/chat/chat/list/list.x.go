// Managed by GoX v0.4.0

//line list.gox:1
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
	"errors"
	"slices"
	"sync"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/list/round"
	"neui.dev/nekit/internal/chat/chat/rower"
	"neui.dev/nekit/internal/chat/params"
)

const margin = 20
const seamGap = 4 * time.Hour

type List = *list

func New(params params.Params, inst api.Instance, thread api.ChatThread, rewinder round.Rewinder, id api.ThreadID, active *api.Round, anchor api.RoundID, fail func(context.Context, error)) List {
	l := &list{
		params: params,
		id: id,
		rewinder: rewinder,
		thread: thread,
		inst: inst,
		fail: fail,
		lastRound: doors.NewSource(api.NoRoundID),
	}
	l.lastSection.entry.id = api.NoRoundID
	l.once = doors.Once(l.main(active, anchor))
	return l
}

type list struct {
	mu sync.Mutex
	ctx context.Context
	params params.Params
	lastRound doors.Source[api.RoundID]
	rewinder round.Rewinder
	thread api.ChatThread
	inst api.Instance
	fail func(context.Context, error)
	id api.ThreadID
	history deque.Deque[entry]
	lastSection struct {
		entry entry
		setter *doors.Setter
		chips rower.Chips
		live round.LiveRound
		roundAppender *doors.Door
		hidden bool
	}
	sectionAppender *doors.Door
	greeting *doors.Door
	historyPrepender *doors.Door
	historyAppender *doors.Door
	seq int
	batch int
	scrollState struct {
		first api.RoundID
		last api.RoundID
		synced bool
		spawned bool
		topReached bool
		bottomReached bool
	}
	once gox.Elem
}

func (l List) Main() gox.Elem {
	return l.once
}

//line list.gox:93
func (l List) main(active *api.Round, anchor api.RoundID) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:95
		seed, err := l.seed(ctx, active, anchor)

//line list.gox:97
		if err == nil {
//line list.gox:98
			__e = __c.Any(seed); if __e != nil { return }
		} else  {
//line list.gox:101
			doors.Logger(ctx).Error("list", "err", err)

//line list.gox:103
			__e = __c.Any(doors.Go(func(ctx context.Context) {
			l.fail(ctx, err)
		})); if __e != nil { return }
		}
	return })
//line list.gox:107
}

func (l List) seed(ctx context.Context, active *api.Round, anchor api.RoundID) (gox.Comp, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx = doors.DetachedContext(ctx)
	cursor := api.NoRoundID
	if active != nil {
		cursor = active.ID
	}
	var tail []api.Round
	if active == nil || active.Input.Len() == 0 {
		loaded, _, err := l.loadBefore(ctx, cursor, 1)
		if err != nil {
			return nil, err
		}
		if len(loaded) != 0 {
			tail = loaded[0]
		}
	}
	stop := cursor
	if len(tail) != 0 {
		stop = tail[0].ID
	}
	head := api.NoRoundID
	var sections [][]api.Round
	topEnded, bottomEnded := false, true
	if anchor != api.NoRoundID {
		above, aboveEnded, err := l.loadBefore(ctx, anchor + 1, margin + 1)
		if err != nil {
			doors.Logger(ctx).Error("list: anchor resolve", "round", anchor, "err", err)
		} else if len(above) != 0 {
			at := above[len(above) - 1][0].ID
			below, belowEnded, err := l.loadAfter(ctx, at - 1, margin + 1, stop, false)
			if err != nil {
				doors.Logger(ctx).Error("list: anchor load", "round", at, "err", err)
			} else {
				head = at
				sections = append(above[0:len(above) - 1], below...)
				topEnded, bottomEnded = aboveEnded, belowEnded
			}
		}
	}
	if head == api.NoRoundID {
		loaded, ended, err := l.loadBefore(ctx, stop, 2 * margin)
		if err != nil {
			return nil, err
		}
		sections, topEnded, bottomEnded = loaded, ended, true
	}
	l.scrollState.first = api.NoRoundID
	l.scrollState.last = api.NoRoundID
	l.scrollState.synced = true
	l.scrollState.topReached = topEnded
	l.scrollState.bottomReached = bottomEnded
	l.sectionAppender = new(doors.Door)
	l.sectionAppender.Inner(ctx, seedScroll(head))
	l.historyPrepender = new(doors.Door)
	l.historyAppender = new(doors.Door)
	if active != nil {
		l.lastRound.Update(ctx, active.ID)
	} else if len(tail) != 0 {
		l.lastRound.Update(ctx, tail[len(tail) - 1].ID)
	}
	lastID := api.NoRoundID
	var lastHead time.Time
	if active != nil {
		lastID = active.ID
		lastHead = active.Created
	}
	if len(tail) != 0 {
		lastID = tail[0].ID
		lastHead = tail[0].Created
	}
	for i, s := range sections {
		var next time.Time
		if i + 1 < len(sections) {
			next = sections[i + 1][0].Created
		} else if bottomEnded {
			next = lastHead
		}
		l.history.PushBack(l.intoHistory(ctx, s, next, 0, nil))
	}
	if active != nil || len(tail) != 0 {
		l.activate(ctx, lastID, lastHead, tail, active, new(doors.Door), head == api.NoRoundID)
	}
	if active == nil && len(tail) == 0 && len(sections) == 0 && !l.params.Off(params.Greeting) {
		l.greeting = new(doors.Door)
		l.greeting.Inner(ctx, greeting(l.params.GreetingTitle, l.params.GreetingBody))
	}
	s := &seed{
		prepender: l.historyPrepender,
		historyAppender: l.historyAppender,
		appender: l.sectionAppender,
		history: slices.Collect(l.history.Iter()),
	}
	if l.lastSection.entry.door != nil {
		s.last = l.lastSection.entry
	}
	if l.greeting != nil {
		s.greeting = l.greeting
	}
	return doors.Once(s), nil
}

func (l List) ToolsDone(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastSection.chips == nil {
		return
	}
	l.lastSection.chips.Done(ctx)
}

func (l List) IndexedEvent(ctx context.Context, e api.IndexedEvent) {
	live := l.lastSection.live
	if live == nil {
		return
	}
	live.On(ctx, e)
}

func (l List) RoundDone(ctx context.Context, state api.RoundState, err string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sectionAppender == nil {
		return
	}
	live := l.lastSection.live
	l.lastSection.live = nil
	live.Done(ctx, state, err)
	if !live.Tools().HasCalls() {
		l.lastSection.chips = nil
		return
	}
	if live.IsErr() {
		live.Tools().Done(ctx)
		live.Tools().Reload(ctx)
		l.lastSection.chips = nil
		return
	}
	if live.IsEmpty() && l.lastSection.chips != nil {
		l.lastSection.chips.Merge(ctx, live.Tools())
		return
	}
	l.lastSection.chips = live.Head(ctx)
}

func (l List) RoundStart(ctx context.Context, r api.Round, truncate api.RoundID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sectionAppender == nil {
		return
	}
	if l.greeting != nil {
		l.greeting.Unmount(ctx)
		l.greeting = nil
	}
	l.lastRound.Update(ctx, r.ID)
	reset := false
	if truncate != api.NoRoundID && l.lastSection.entry.door != nil && truncate > l.lastSection.entry.id {
		sections, _, err := l.loadBefore(ctx, r.ID, 1)
		if err != nil {
			doors.Logger(ctx).Error("list: tail reload", "err", err)
			reset = true
		} else {
			var tail []api.Round
			if len(sections) != 0 {
				tail = sections[0]
			}
			l.activate(ctx, l.lastSection.entry.id, l.lastSection.entry.head, tail, &r, l.lastSection.entry.door, true)
			return
		}
	}
	if reset || r.Input.Len() != 0 || l.lastSection.roundAppender == nil {
		l.seq++
		atBottom := !reset && l.scrollState.bottomReached && (l.scrollState.last == api.NoRoundID || l.scrollState.last == l.lastSection.entry.id)
		if truncate != api.NoRoundID {
			if l.lastSection.entry.door != nil && l.lastSection.entry.id >= truncate {
				l.lastSection.entry.remove(ctx)
				l.lastSection.entry = entry{id: api.NoRoundID}
			}
			for l.history.Len() != 0 && l.history.Back().id >= truncate {
				l.history.PopBack().remove(ctx)
			}
		}
		if l.lastSection.entry.door != nil {
			if atBottom {
				if label := l.seamLabel(l.lastSection.entry.head, r.Created); label != "" {
					l.lastSection.roundAppender.Static(ctx, seam(label))
				}
				l.history.PushBack(l.lastSection.entry)
			} else {
				l.lastSection.entry.remove(ctx)
			}
		}
		if !atBottom {
			for l.history.Len() != 0 {
				l.history.PopFront().remove(ctx)
			}
			sections, ended, err := l.loadBefore(ctx, r.ID, 2 * margin)
			if err != nil {
				doors.Logger(ctx).Error("list: reset", "err", err)
			}
			for i, s := range sections {
				next := r.Created
				if i + 1 < len(sections) {
					next = sections[i + 1][0].Created
				}
				l.history.PushBack(l.intoHistory(ctx, s, next, 0, nil))
			}
			l.scrollState.topReached = ended
			l.scrollState.bottomReached = true
			l.scrollState.first = api.NoRoundID
			l.scrollState.last = api.NoRoundID
			l.scrollState.synced = true
		}
		l.activate(ctx, r.ID, r.Created, nil, &r, new(doors.Door), true)
		newAppender, oldAppender := new(doors.Door), l.sectionAppender
		l.sectionAppender = newAppender
		l.historyAppender = new(doors.Door)
		s := &seed{
			historyAppender: l.historyAppender,
			appender: newAppender,
			last: l.lastSection.entry,
		}
		if !atBottom {
			l.historyPrepender = new(doors.Door)
			s.prepender = l.historyPrepender
			s.history = slices.Collect(l.history.Iter())
			newAppender.Inner(ctx, scrollBottom())
		}
		oldAppender.Static(ctx, doors.Once(s))
		if atBottom {
			l.scrollState.synced = false
			if !l.scrollState.spawned {
				l.scrollState.spawned = true
				go l.syncLoop()
			}
		}
		return
	}
	l.lastSection.live = round.NewLiveRound(
		ctx,
		l.params,
		l.thread,
		l.rewinder,
		l.lastRound,
		r,
		true,
		l.lastSection.chips,
	)
	newAppender, oldAppender := new(doors.Door), l.lastSection.roundAppender
	l.lastSection.roundAppender = newAppender
	oldAppender.Static(ctx, doors.Once(&seed{
		appender: newAppender,
		last: l.lastSection.live,
	}))
}

func (l List) activate(ctx context.Context, id api.RoundID, head time.Time, tail []api.Round, active *api.Round, door *doors.Door, span bool) {
	var rounds []round.Round
	l.lastSection.chips = nil
	if len(tail) != 0 {
		rounds, l.lastSection.chips = l.intoRounds(tail, span && active != nil)
	}
	if active != nil {
		chain := l.lastSection.chips
		if active.Input.Len() != 0 {
			chain = nil
		}
		l.lastSection.live = round.NewLiveRound(
			ctx,
			l.params,
			l.thread,
			l.rewinder,
			l.lastRound,
			*active,
			span,
			chain,
		)
	}
	l.lastSection.entry = entry{id: id, head: head, door: door}
	l.lastSection.setter = new(doors.Setter)
	l.lastSection.roundAppender = new(doors.Door)
	l.lastSection.hidden = !l.scrollState.bottomReached
	door.Outer(ctx, doors.Once(&liveSection{
		id: id,
		rounds: rounds,
		live: l.lastSection.live,
		setter: l.lastSection.setter,
		appender: l.lastSection.roundAppender,
		hidden: l.lastSection.hidden,
	}))
}

func (l List) intoHistory(ctx context.Context, s []api.Round, next time.Time, batch int, after any) entry {
	rounds, _ := l.intoRounds(s, false)
	e := entry{id: s[0].ID, head: s[0].Created, door: new(doors.Door)}
	e.door.Outer(ctx, doors.Once(&section{
		id: e.id,
		batch: batch,
		rounds: rounds,
		seam: l.seamLabel(e.head, next),
		after: after,
	}))
	return e
}

func (l List) seamLabel(prev, next time.Time) string {
	if prev.IsZero() || next.IsZero() {
		return ""
	}
	loc := l.inst.Location()
	from, to := prev.In(loc), next.In(loc)
	fromYear, fromMonth, fromDay := from.Date()
	toYear, toMonth, toDay := to.Date()
	sameDay := fromYear == toYear && fromMonth == toMonth && fromDay == toDay
	if sameDay && to.Sub(from) < seamGap {
		return ""
	}
	nowYear, nowMonth, nowDay := time.Now().In(loc).Date()
	label := ""
	switch {
	case toYear == nowYear && toMonth == nowMonth && toDay == nowDay:
		label = "Today"
	case toYear == nowYear:
		label = to.Format("January 2")
	default:
		label = to.Format("January 2, 2006")
	}
	return label + " · " + to.Format("15:04")
}

type entry struct {
	id api.RoundID
	head time.Time
	door *doors.Door
}

func (e entry) remove(ctx context.Context) {
	e.door.Static(ctx, nil)
}

//line list.gox:451
func (e entry) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:452
		__e = __c.Any(e.door); if __e != nil { return }
	return })
//line list.gox:453
}

func (l List) loadAfter(ctx context.Context, cursor api.RoundID, num int, stop api.RoundID, strip bool) ([][]api.Round, bool, error) {
	sections := make([][]api.Round, 0, num)
	var buffer []api.Round
	var ended bool
	page := max(num, margin)
	outer: for {
		rounds, err := l.thread.RoundAfter(ctx, cursor, page)
		if err != nil {
			return nil, false, err
		}
		if len(rounds) == 0 {
			ended = true
			break
		}
		cursor = rounds[len(rounds) - 1].ID
		for _, r := range rounds {
			if strip && r.Input.Len() == 0 {
				continue
			}
			strip = false
			if r.ID == stop {
				ended = true
				break outer
			}
			if len(buffer) == 0 || r.Input.Len() == 0 {
				buffer = append(buffer, r)
				continue
			}
			sections = append(sections, buffer)
			buffer = nil
			if len(sections) == num {
				break outer
			}
			buffer = append(buffer, r)
		}
		if len(rounds) < page {
			ended = true
			break
		}
	}
	if len(buffer) != 0 {
		sections = append(sections, buffer)
	}
	return sections, ended, nil
}

func (l List) loadBefore(ctx context.Context, cursor api.RoundID, num int) ([][]api.Round, bool, error) {
	sections := make([][]api.Round, 0, num)
	var buffer []api.Round
	var ended bool
	page := max(num, margin)
	outer: for {
		rounds, err := l.thread.RoundBefore(ctx, cursor, page)
		if err != nil {
			if errors.Is(err, api.ErrNotFound) && cursor == api.NoRoundID {
				return nil, true, nil
			}
			return nil, false, err
		}
		if len(rounds) == 0 {
			ended = true
			break
		}
		cursor = rounds[0].ID
		end := len(rounds)
		for i, r := range slices.Backward(rounds) {
			if r.Input.Len() == 0 {
				continue
			}
			slice := slices.Clone(rounds[i:end])
			if len(buffer) != 0 {
				slice = append(slice, buffer...)
				buffer = nil
			}
			end = i
			sections = append(sections, slice)
			if len(sections) == num {
				buffer = nil
				break outer
			}
		}
		buffer = append(slices.Clone(rounds[0:end]), buffer...)
		if len(rounds) < page {
			ended = true
			break
		}
	}
	if ended && len(buffer) != 0 {
		sections = append(sections, buffer)
	}
	slices.Reverse(sections)
	return sections, ended, nil
}

func (l *list) intoRounds(section []api.Round, span bool) ([]round.Round, rower.Chips) {
	var head rower.Chips
	rounds := make([]round.Round, 0, len(section))
	for _, r := range section {
		chain := head
		if r.Input.Len() != 0 {
			chain = nil
		}
		next := round.NewRound(
			l.ctx,
			l.params,
			l.thread,
			l.rewinder,
			l.lastRound,
			r,
			chain,
			span,
		)
		if next.IsEmpty() && head != nil && r.State == api.RoundOk {
			if next.Tools().HasCalls() {
				head.Merge(l.ctx, next.Tools())
			} else {
				head = nil
			}
			continue
		}
		rounds = append(rounds, next)
		if r.State == api.RoundOk && next.Tools().HasCalls() {
			head = next.Head(l.ctx)
		} else {
			head = nil
		}
	}
	return rounds, head
}

type section struct {
	id api.RoundID
	batch int
	rounds []round.Round
	seam string
	after any
}

//line list.gox:593
func (s *section) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:595
		var batch any
	if s.batch != 0 {
		batch = s.batch
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line list.gox:600
			__e = __c.Set("class", "nechat-section"); if __e != nil { return }
//line list.gox:600
			__e = __c.Set("data-id", int(s.id)); if __e != nil { return }
//line list.gox:600
			__e = __c.Set("data-batch", batch); if __e != nil { return }
//line list.gox:600
			__e = __c.Set("data-pending", s.batch != 0); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line list.gox:601
			for _, r := range s.rounds {
//line list.gox:602
				__e = __c.Any(r); if __e != nil { return }
			}
//line list.gox:604
			if s.seam != "" {
//line list.gox:605
				__e = __c.Any(seam(s.seam)); if __e != nil { return }
			}
//line list.gox:607
			__e = __c.Any(s.after); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line list.gox:609
}

//line list.gox:611
func seam(label string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line list.gox:612
			__e = __c.Set("class", "nechat-seam"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line list.gox:612
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line list.gox:613
}

type liveSection struct {
	id api.RoundID
	rounds []round.Round
	live round.LiveRound
	setter *doors.Setter
	appender *doors.Door
	hidden bool
}

//line list.gox:624
func (s *liveSection) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:626
		var setter gox.Modify = gox.ModifyFunc(func(ctx context.Context, tag string, attrs gox.Attrs) error {
		return nil
	})
	if s.setter != nil {
		setter = s.setter
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line list.gox:633
			__e = __c.Set("class", "nechat-section"); if __e != nil { return }
//line list.gox:633
			__e = __c.Modify(setter); if __e != nil { return }
//line list.gox:633
			__e = __c.Set("data-id", int(s.id)); if __e != nil { return }
//line list.gox:633
			__e = __c.Set("hidden", s.hidden); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line list.gox:634
			for _, r := range s.rounds {
//line list.gox:635
				__e = __c.Any(r); if __e != nil { return }
			}
//line list.gox:637
			if s.live != nil {
//line list.gox:638
				__e = __c.Any(s.live); if __e != nil { return }
			}
//line list.gox:640
			__e = __c.Any(s.appender); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line list.gox:642
}

type seed struct {
	prepender gox.Comp
	historyAppender gox.Comp
	appender gox.Comp
	history []entry
	last gox.Comp
	greeting gox.Comp
}

//line list.gox:653
func (s *seed) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:654
		__e = __c.Any(s.prepender); if __e != nil { return }
//line list.gox:655
		for _, d := range s.history {
//line list.gox:656
			__e = __c.Any(d); if __e != nil { return }
		}
//line list.gox:658
		__e = __c.Any(s.historyAppender); if __e != nil { return }
//line list.gox:659
		__e = __c.Any(s.last); if __e != nil { return }
//line list.gox:660
		__e = __c.Any(s.greeting); if __e != nil { return }
//line list.gox:661
		__e = __c.Any(s.appender); if __e != nil { return }
	return })
//line list.gox:662
}

//line list.gox:664
func greeting(title any, body any) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line list.gox:666
		text, isText := body.(string)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line list.gox:668
			__e = __c.Set("class", "nechat-greet"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line list.gox:669
			if title != nil && title != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line list.gox:670
					__e = __c.Set("class", "nechat-greet-title"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line list.gox:670
					__e = __c.Any(title); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line list.gox:672
			if isText && text != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line list.gox:673
					__e = __c.Set("class", "nechat-greet-body nechat-greet-body_text"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line list.gox:673
					__e = __c.Any(text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line list.gox:674
			} else if !isText && body != nil {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line list.gox:675
					__e = __c.Set("class", "nechat-greet-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line list.gox:675
					__e = __c.Any(body); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line list.gox:678
}
