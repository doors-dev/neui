package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/tools"
)

type Run interface {
	Context() context.Context
	TurnContext() context.Context
	PauseTurn()
	ResumeTurn()
	PageContext() context.Context
	Library() Library
	Notify(script uint, data json.RawMessage, idle bool)
	Script() uint
	NotificationsCap() int
	Seed(context.Context, gox.Comp)
	GC() error
	Rearm() error
	Config() api.IslandConfig
	AckHandover()
	Store() doors.Store
}

type State = *state

func NewState(run Run) State {
	applied := make(chan struct{})
	s := &state{
		run:     run,
		widgets: map[uint]*placedWidget{},
		slots:   make(map[uint]*slot),
		applied: applied,
	}
	doors.OnBatch(run.PageContext(), func(context.Context) {
		close(applied)
	})
	return s
}

type anyWidget interface {
	Update(json.RawMessage) (func(ctx context.Context), error)
	IsControl() bool
	gox.Comp
}

type children struct {
	slots   []*slot
	widgets []*placedWidget
	layouts int
}

type slot struct {
	door     doors.Door
	holders  int
	released bool
	children
}

type placedWidget struct {
	name     string
	door     doors.Door
	widget   anyWidget
	holders  int
	released bool
}

type submit struct {
	id      uint
	data    any
	release func()
}

type state struct {
	run                Run
	widgets            map[uint]*placedWidget
	slots              map[uint]*slot
	nextID             uint
	items              int
	ops                deque.Deque[func(ctx context.Context)]
	notifications      bytes.Buffer
	notificationsCount int
	mu                 sync.Mutex
	hook               *chan struct{}
	stash              *submit
	handover           bool
	root               slot
	ch                 chan string
	task               common.Task
	seeded             bool
	applied            chan struct{}
}

func (s *state) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stash != nil {
		s.stash.release()
		s.stash = nil
	}
}

func (s *state) RegisterTools(vm *quickjs.VM) error {
	m := tools.NewModule("", "")
	_, err := m.Add("release", "", s.toolRelease)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("create", "", s.toolCreate)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("update", "", s.toolUpdate)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("createSlot", "", s.toolCreateSlot)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("releaseSlot", "", s.toolReleaseSlot)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("notify", "", s.toolNotify)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("setSlot", "", s.toolSetSlot)
	if err != nil {
		panic(err)
	}
	_, err = m.Add("widgets", "", func(ctx context.Context, _ struct{}) ([]string, error) {
		return s.run.Library().widgetNames(), nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add("layouts", "", func(ctx context.Context, _ struct{}) ([]string, error) {
		return s.run.Library().layoutNames(), nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add("validateLayout", "", func(ctx context.Context, arg validateLayoutArg) (struct{}, error) {
		return struct{}{}, s.run.Library().validateLayout(arg.Name, arg.Params)
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add("draw", "", s.toolDraw)
	if err != nil {
		panic(err)
	}
	err = js.RegisterTools(vm, s.run.TurnContext, "__", m.List())
	return err
}

func (s *state) toolRelease(ctx context.Context, id uint) (struct{}, error) {
	w, ok := s.widgets[id]
	if !ok {
		return struct{}{}, nil
	}
	delete(s.widgets, id)
	w.released = true
	if w.holders == 0 {
		s.dropWidget(w)
	}
	return struct{}{}, nil
}

type createArg struct {
	Name   string
	Params json.RawMessage
}

func (s *state) toolCreate(ctx context.Context, arg createArg) (uint, error) {
	if err := s.fit(1); err != nil {
		return 0, err
	}
	s.nextID++
	id := s.nextID
	widget, err := s.run.Library().newWidget(arg.Name, arg.Params, func(ctx context.Context, output any) bool {
		s.mu.Lock()
		if s.run.Context().Err() != nil || s.hook == nil {
			s.mu.Unlock()
			return false
		}
		hook := *s.hook
		s.hook = nil
		s.stash = &submit{id: id, data: output, release: doors.HoldSettle(ctx)}
		close(hook)
		s.mu.Unlock()
		return true
	})
	if err != nil {
		return 0, err
	}
	p := &placedWidget{name: arg.Name, widget: widget}
	p.door.Outer(ctx, widgetSpace{name: arg.Name, id: id, widget: widget})
	s.items++
	s.widgets[id] = p
	return id, nil
}

func (s *state) toolCreateSlot(ctx context.Context, arg struct{}) (uint, error) {
	if err := s.fit(1); err != nil {
		return 0, err
	}
	s.nextID++
	id := s.nextID
	s.items++
	s.slots[id] = &slot{}
	return id, nil
}

func (s *state) fit(n int) error {
	if s.items+n > s.run.Config().MaxItems {
		return errItemLimit
	}
	return nil
}

func (s *state) toolReleaseSlot(ctx context.Context, id uint) (struct{}, error) {
	c, ok := s.slots[id]
	if !ok {
		return struct{}{}, nil
	}
	delete(s.slots, id)
	c.released = true
	if c.holders == 0 {
		s.dropSlot(c)
	}
	return struct{}{}, nil
}

func (s *state) dropWidget(w *placedWidget) {
	s.items--
}

func (r *state) dropSlot(s *slot) {
	r.items -= 1 + s.layouts
	for _, c := range s.slots {
		c.holders--
		if c.holders == 0 && c.released {
			r.dropSlot(c)
		}
	}
	for _, w := range s.widgets {
		w.holders--
		if w.holders == 0 && w.released {
			r.dropWidget(w)
		}
	}
}

func (s *state) toolSetSlot(ctx context.Context, arg setSlotArg) (struct{}, error) {
	if s.ops.Len()+s.notificationsCount >= s.run.Config().MaxOps {
		return struct{}{}, errOpsLimit
	}
	apply, err := s.updateSlot(arg.ID, arg.Content)
	if err != nil {
		return struct{}{}, err
	}
	s.ops.PushBack(apply)
	return struct{}{}, nil
}

type validateLayoutArg struct {
	Name   string
	Params json.RawMessage
}

type updateArg struct {
	ID    uint
	Patch json.RawMessage
}

func (s *state) toolUpdate(ctx context.Context, arg updateArg) (struct{}, error) {
	if s.ops.Len()+s.notificationsCount >= s.run.Config().MaxOps {
		return struct{}{}, errOpsLimit
	}
	apply, err := s.updateWidget(arg.ID, arg.Patch)
	if err != nil {
		return struct{}{}, err
	}
	s.ops.PushBack(apply)
	return struct{}{}, nil
}

type Result struct {
	ID        uint `json:"id"`
	Data      any  `json:"data"`
	Interrupt bool `json:"interrupt,omitempty"`
}

func (s *state) toolNotify(ctx context.Context, data json.RawMessage) (struct{}, error) {
	if s.ops.Len()+s.notificationsCount >= s.run.Config().MaxOps {
		return struct{}{}, errOpsLimit
	}
	prev := s.notifications.Len()
	if prev == 0 {
		s.notifications.WriteByte('[')
	} else {
		s.notifications.WriteByte(',')
	}
	json.HTMLEscape(&s.notifications, data)
	if limit := s.run.NotificationsCap(); limit > 0 && s.notifications.Len()+1 > limit {
		s.notifications.Truncate(prev)
		return struct{}{}, fmt.Errorf("%d bytes: %w", limit, errNotificationsLimit)
	}
	s.notificationsCount++
	return struct{}{}, nil
}

func (s *state) toolDraw(ctx context.Context, ms int) (*Result, error) {
	if s.applied != nil {
		s.run.PauseTurn()
		select {
		case <-ctx.Done():
			return &Result{Interrupt: true}, nil
		case <-s.applied:
		}
		s.run.ResumeTurn()
	}
	if err := s.run.GC(); err != nil {
		return nil, err
	}
	s.run.PauseTurn()
	s.mu.Lock()
	handover := s.handover
	s.handover = false
	ch := make(chan struct{})
	if !handover {
		s.hook = &ch
	}
	prevStash := s.stash
	s.stash = nil
	s.mu.Unlock()
	s.send(prevStash, !handover && ms <= 0 && !s.hasControl())
	if handover {
		s.run.AckHandover()
		return &Result{Interrupt: true}, nil
	}
	var timeout <-chan time.Time
	if ms > 0 {
		timeout = time.After(time.Duration(ms) * time.Millisecond)
	}
	select {
	case <-ctx.Done():
		return &Result{Interrupt: true}, nil
	case <-timeout:
	case <-ch:
	}
	if err := s.run.Rearm(); err != nil {
		return nil, err
	}
	return s.collect()
}

func (s *state) collect() (*Result, error) {
	s.mu.Lock()
	s.hook = nil
	if s.stash != nil {
		r := &Result{
			ID:   s.stash.id,
			Data: s.stash.data,
		}
		s.stash.data = nil
		s.mu.Unlock()
		return r, nil
	}
	if s.handover {
		s.handover = false
		s.mu.Unlock()
		s.run.AckHandover()
		return &Result{Interrupt: true}, nil
	}
	s.mu.Unlock()
	return nil, nil
}

func (s *state) ResetHandover() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handover = false
}

func (s *state) ArmHandover() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handover = true
	if s.hook == nil {
		return
	}
	hook := *s.hook
	s.hook = nil
	close(hook)
}

func (s *state) hasControl() bool {
	seen := map[*slot]bool{}
	stack := []*slot{&s.root}
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[next] {
			continue
		}
		seen[next] = true
		for _, w := range next.widgets {
			if w.widget.IsControl() {
				return true
			}
		}
		stack = append(stack, next.slots...)
	}
	return false
}

func (s *state) FlushNotifications(idle bool) {
	if s.notificationsCount == 0 && !idle {
		return
	}
	if s.notificationsCount == 0 {
		s.notifications.WriteByte('[')
	}
	s.notifications.WriteByte(']')
	data := json.RawMessage(s.notifications.Bytes())
	s.notifications = bytes.Buffer{}
	s.notificationsCount = 0
	s.run.Notify(s.run.Script(), data, idle)
}

func (s *state) send(prevStash *submit, idle bool) {
	applied := make(chan struct{})
	s.applied = applied
	doors.OnBatch(s.run.PageContext(), func(context.Context) {
		close(applied)
	}, func(ctx context.Context) {
		ctx = context.WithValue(ctx, common.KeyIslandStore, s.run.Store())
		if prevStash != nil {
			doors.OnSettle(ctx, func(context.Context) {
				prevStash.release()
			})
		}
		if !s.seeded {
			s.seeded = true
			s.run.Seed(ctx, s.seed())
		}
		for f := range s.ops.IterPopFront() {
			f(ctx)
		}
	})
	s.FlushNotifications(idle)
}

const (
	kindSlot   = "slot"
	kindLayout = "layout"
	kindWidget = "widget"
)

type content struct {
	Kind    string          `json:"kind"`
	ID      uint            `json:"id"`
	Name    string          `json:"name"`
	Params  json.RawMessage `json:"params"`
	Content []content       `json:"content"`
}

type setSlotArg struct {
	ID      uint      `json:"id"`
	Content []content `json:"content"`
}

type node struct {
	content []gox.Comp
}

func (n *node) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		for _, c := range n.content {
			if err := cur.Comp(c); err != nil {
				return err
			}
		}
		return nil
	}
}

func (s *state) render(content content, owner *children) (gox.Comp, error) {
	switch content.Kind {
	case kindSlot:
		child, ok := s.slots[content.ID]
		if !ok {
			return nil, fmt.Errorf("slot %d: %w", content.ID, errSlotNotFound)
		}
		if !slices.Contains(owner.slots, child) {
			owner.slots = append(owner.slots, child)
		}
		return &child.door, nil
	case kindWidget:
		widget, ok := s.widgets[content.ID]
		if !ok {
			return nil, fmt.Errorf("widget %d: %w", content.ID, errWidgetNotFound)
		}
		if !slices.Contains(owner.widgets, widget) {
			owner.widgets = append(owner.widgets, widget)
		}
		return &widget.door, nil
	case kindLayout:
		owner.layouts++
		innerContents := make([]gox.Comp, len(content.Content))
		for i, c := range content.Content {
			inner, err := s.render(c, owner)
			if err != nil {
				return nil, err
			}
			innerContents[i] = inner
		}
		layout, err := s.run.Library().newLayout(content.Name, content.Params, innerContents)
		if err != nil {
			return nil, err
		}
		return layout, nil
	default:
		return nil, fmt.Errorf("content kind %q: %w", content.Kind, errContentKind)
	}
}

func (s *state) updateSlot(id uint, content []content) (func(ctx context.Context), error) {
	target, ok := s.slots[id]
	if !ok {
		if id != 0 {
			return nil, fmt.Errorf("slot %d: %w", id, errSlotNotFound)
		}
		target = &s.root
	}
	var children children
	innerContents := make([]gox.Comp, len(content))
	for i, c := range content {
		inner, err := s.render(c, &children)
		if err != nil {
			return nil, err
		}
		innerContents[i] = inner
	}
	seen := map[*slot]bool{}
	stack := slices.Clone(children.slots)
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if next == target {
			return nil, fmt.Errorf("slot %d: %w", id, errSlotCycle)
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		stack = append(stack, next.slots...)
	}
	if err := s.fit(children.layouts - target.layouts); err != nil {
		return nil, err
	}
	s.items += children.layouts - target.layouts
	for _, c := range children.slots {
		c.holders++
	}
	for _, w := range children.widgets {
		w.holders++
	}
	for _, c := range target.slots {
		c.holders--
		if c.holders == 0 && c.released {
			s.dropSlot(c)
		}
	}
	for _, w := range target.widgets {
		w.holders--
		if w.holders == 0 && w.released {
			s.dropWidget(w)
		}
	}
	target.children = children
	door := &target.door
	return func(ctx context.Context) {
		door.Inner(ctx, gox.Elem(func(cur gox.Cursor) error {
			for _, c := range innerContents {
				if err := cur.Comp(c); err != nil {
					return err
				}
			}
			return nil
		}))
	}, nil
}

func (s *state) updateWidget(id uint, patch json.RawMessage) (func(ctx context.Context), error) {
	widget, ok := s.widgets[id]
	if !ok {
		return nil, fmt.Errorf("widget %d: %w", id, errWidgetNotFound)
	}
	apply, err := widget.widget.Update(patch)
	if err != nil {
		return nil, fmt.Errorf("ui.%s #%d update: %w", widget.name, id, err)
	}
	return apply, nil
}

func (s *state) seed() gox.Elem {
	return func(cur gox.Cursor) error {
		for _, e := range s.run.Library().head {
			if err := cur.Any(e); err != nil {
				return err
			}
		}
		return cur.Comp(&s.root.door)
	}
}

type widgetSpace struct {
	name   string
	id     uint
	widget anyWidget
}

func (w widgetSpace) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		ctx, err := getSpace(cur.Context()).AddSpace(cur.Context(), fmt.Sprintf("%s_%d", w.name, w.id), "")
		if err != nil {
			return err
		}
		return doors.Ctx(ctx).Proxy(cur, w.widget.Main())
	}
}
