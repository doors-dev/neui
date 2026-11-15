package thread

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/doors-dev/doors"
	"maps"
	"sync"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
	"neui.dev/internal/thread/response"
	"neui.dev/internal/utils"
)

var registry sync.Map

var ErrLinked = errors.New("thread: already linked to this page")

func getThread(id api.ThreadID) *thread {
	if entry, ok := registry.Load(id); ok {
		return entry.(*thread)
	}
	t := &thread{
		id:    id,
		ch:    make(chan Command),
		close: make(chan struct{}),
	}
	if entry, loaded := registry.LoadOrStore(id, t); loaded {
		return entry.(*thread)
	}
	go t.run()
	return t
}

func NewLink(backend *common.Backend, page page.Page) Link {
	response := NewResultResponse[Link]()
repeat:
	thread := getThread(api.ThreadID(utils.NewID()))
	select {
	case <-thread.close:
		goto repeat
	case thread.ch <- commandLink{backend, page, false, nil, nil, response}:
	}
	link, err := response.Receive()
	if err != nil {
		panic("there should be no way it panics")
	}
	return link
}

func OpenLink(id api.ThreadID, backend *common.Backend, page page.Page, meta any, op Operation) (Link, error) {
	response := NewResultResponse[Link]()
repeat:
	thread := getThread(id)
	select {
	case <-thread.close:
		goto repeat
	case thread.ch <- commandLink{backend, page, true, meta, op, response}:
	}
	return response.Receive()
}

func Rename(ctx context.Context, storage api.MessageStorage, id api.ThreadID, name string) error {
	op := NewOperation(ctx)
repeat:
	thread := getThread(id)
	select {
	case <-thread.close:
		goto repeat
	case thread.ch <- commandThreadRename{ctx, storage, name, op}:
	}
	return op.Receive()
}

func Delete(ctx context.Context, storage api.MessageStorage, id api.ThreadID) error {
	op := NewOperation(ctx)
repeat:
	thread := getThread(id)
	select {
	case <-thread.close:
		goto repeat
	case thread.ch <- commandThreadDelete{ctx, storage, op}:
	}
	return op.Receive()
}

type thread struct {
	id      api.ThreadID
	link    *link
	ch      chan Command
	close   chan struct{}
	replay  replay.Replay
	input   api.Input
	pending bool
	state   interface {
		tick() bool
		dump(s *api.ChatState)
	}
	compaction compaction
}

func (t *thread) run() {
	defer func() {
		registry.Delete(t.id)
		close(t.close)
	}()
	t.switchStateWait()
	for t.state.tick() {
	}
}

func (t *thread) criticalErr(m *message, err error) {
	if t.link == nil {
		return
	}
	t.link.send(newMessageEvent(m, api.ChatEventCriticalError{Err: err}))
	t.pending = false
	t.compaction.stop()
	t.link.dispose()
	t.link = nil
	t.replay = nil
	t.switchStateWait()
}

func (t *thread) getState() api.ChatState {
	s := api.ChatState{
		ID: t.id,
		Input: api.ChatInput{
			Input:   t.input,
			Pending: t.pending,
		},
	}
	if t.link != nil {
		s.Tasks = t.link.taskList()
	}
	s.Usage = t.replay.Usage()
	s.Name = t.replay.Name()
	s.Compaction = t.compaction.isActive()
	s.CanCompact = t.canCompact()
	t.state.dump(&s)
	return s
}

func (t *thread) taskResult(id uint, output any, err error) (bool, error) {
	task, ok := t.link.taskRemove(id)
	if !ok {
		return false, nil
	}
	attrs := maps.Clone(task.attrs)
	if attrs == nil {
		attrs = map[string]any{}
	}
	maps.Copy(attrs, map[string]any{
		"task_id": task.id,
		"module":  task.module,
		"name":    task.name,
	})
	if task.comment != "" {
		attrs["comment"] = task.comment
	}
	if err != nil {
		attrs["error"] = err.Error()
	}
	if err := t.replay.Record(
		task.origin,
		"task_result",
		attrs,
		output,
	); err != nil {
		return true, err
	}
	t.link.send(newMessageEvent(nil, api.ChatEventTasks{Tasks: t.link.taskList()}))
	return true, nil
}

func (t *thread) taskNotify(task linkTask, data json.RawMessage, meta map[string]any) error {
	attrs := maps.Clone(task.attrs)
	if attrs == nil {
		attrs = map[string]any{}
	}
	maps.Copy(attrs, meta)
	maps.Copy(attrs, map[string]any{
		"task_id": task.id,
		"module":  task.module,
		"name":    task.name,
	})
	if task.comment != "" {
		attrs["comment"] = task.comment
	}
	return t.replay.Record(task.origin, "task_notify", attrs, data)
}

func (t *thread) on(cmd any) {
	switch cmd := cmd.(type) {
	case commandInit:
		if t.link != cmd.link {
			return
		}
		state := t.getState()
		state.Meta = t.link.meta
		t.link.meta = nil
		t.link.send(NewMessageInit(state, t.link.fs))
	case commandInput:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if t.pending {
			cmd.Err(api.ErrWrongState)
			return
		}
		inputBytes := 0
		for _, c := range cmd.input.Content() {
			if text, ok := c.(api.ContentText); ok {
				inputBytes += len(text.Text)
			}
		}
		if inputBytes > replay.RoundItemBudget(cmd.link.backend.Config(), cmd.link.backend.Provider().Model()) {
			cmd.Err(api.ErrInputTooLarge)
			return
		}
		t.input = cmd.input
		cmd.NoMessage()
	case commandStop:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		cmd.Err(api.ErrWrongState)
	case commandUnsend:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !t.pending {
			cmd.Err(api.ErrWrongState)
			return
		}
		t.pending = false
		t.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventInputPending{Pending: false}))
	case commandSend:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if t.pending {
			cmd.Err(api.ErrWrongState)
			return
		}
		inputBytes := 0
		for _, c := range cmd.input.Content() {
			if text, ok := c.(api.ContentText); ok {
				inputBytes += len(text.Text)
			}
		}
		if inputBytes > replay.RoundItemBudget(cmd.link.backend.Config(), cmd.link.backend.Provider().Model()) {
			cmd.Err(api.ErrInputTooLarge)
			return
		}
		t.input = cmd.input
		if t.input.Len() == 0 {
			cmd.Err(api.ErrWrongState)
			return
		}
		t.pending = true
		t.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventInputPending{Pending: true}))
	case commandTask:
		if t.link == nil || t.link.linkTasks != cmd.linkTasks {
			cmd.Err(context.Canceled)
			return
		}
		cmd.Err(errors.New("task can be issued only during tool calls round"))
	case commandRecord:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if err := t.replay.Record(
			cmd.origin,
			cmd.kind,
			cmd.attrs,
			cmd.data,
		); err != nil {
			cmd.Err(err)
			t.criticalErr(nil, err)
			return
		}
		cmd.NoMessage()
	case commandRename:
		if t.link == nil || t.link.linkTasks != cmd.link.linkTasks {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if err := t.replay.Rename(cmd.name); err != nil {
			cmd.Err(err)
			t.criticalErr(nil, err)
			return
		}
		t.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventName{Name: cmd.name}))
	case commandThreadRename:
		if t.replay == nil {
			for retried := false; ; retried = true {
				thread, err := cmd.storage.ThreadGet(cmd.ctx, t.id)
				if err != nil {
					cmd.Err(err)
					return
				}
				_, err = cmd.storage.ThreadRename(cmd.ctx, t.id, thread.Seq, cmd.name)
				if errors.Is(err, api.ErrDrift) && !retried {
					continue
				}
				if err != nil {
					cmd.Err(err)
					return
				}
				cmd.NoMessage()
				return
			}
		}
		if !t.replay.IsLoaded() {
			cmd.Err(api.ErrNotFound)
			return
		}
		if err := t.replay.Rename(cmd.name); err != nil {
			cmd.Err(err)
			t.criticalErr(nil, err)
			return
		}
		t.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventName{Name: cmd.name}))
	case commandThreadDelete:
		if err := cmd.storage.ThreadDelete(cmd.ctx, t.id); err != nil {
			cmd.Err(err)
			return
		}
		if t.link == nil {
			cmd.NoMessage()
			return
		}
		t.criticalErr(cmd.intoMessage(), api.ErrNotFound)
	case commandTaskResult:
		if t.link == nil || t.link.linkTasks != cmd.linkTasks {
			return
		}
		if _, err := t.taskResult(cmd.id, cmd.output, cmd.err); err != nil {
			t.criticalErr(nil, err)
			return
		}
	case commandTaskNotify:
		if t.link == nil || t.link.linkTasks != cmd.linkTasks {
			return
		}
		task, ok := t.link.taskGet(cmd.id)
		if !ok {
			return
		}
		if err := t.taskNotify(task, cmd.data, cmd.meta); err != nil {
			t.criticalErr(nil, err)
			return
		}
	case commandRetry:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		cmd.Err(api.ErrWrongState)
	case commandApplyCompaction:
		t.applyCompaction(cmd)
	case commandCompact:
		if t.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !t.canCompact() {
			cmd.Err(api.ErrWrongState)
			return
		}
		if _, err := t.compact(false); err != nil {
			cmd.Err(err)
			return
		}
		cmd.NoMessage()
	case commandAskDone:
		cmd.Operation.NoMessage()
	case commandAsk:
		cmd.Err(api.ErrWrongState)
	default:
		panic("unknown command")
	}
}

func (t *thread) disconnect() {
	t.pending = false
	t.compaction.stop()
	t.link.dispose()
	t.link = nil
	t.replay = nil
}

func (t *thread) connect(backend *common.Backend, page page.Page, load bool, meta any) (Link, error) {
	t.pending = false
	t.compaction.stop()
	if t.link != nil {
		t.link.dispose()
		t.link = nil
		t.replay = nil
	}
	link := newLink(t, backend, page, meta)
	replay := replay.NewReplay(backend.Context(), t.id, backend.Storage(), backend.Config(), backend.Provider().Model())
	if load {
		if err := replay.Load(); err != nil {
			link.dispose()
			return Link{}, err
		}
	}
	t.link = link
	t.replay = replay
	if load {
		if err := t.sweepDrafts(); err != nil {
			doors.Logger(backend.Context()).Error("thread: draft sweep failed", "thread", t.id, "err", err)
		}
	}
	return t.link.link(), nil

}

func (t *thread) switchStateRound() (api.Round, error) {
	input := api.Input{}
	if t.pending {
		input = t.input
		t.input = api.Input{}
		t.pending = false
	}
	t.commitAttachments(input)
	created, err := t.replay.Create()
	if err != nil {
		return api.Round{}, err
	}
	if created {
		t.link.send(newMessageEvent(nil, api.ChatEventThreadCreated{}))
	}
	round, replay, err := t.replay.RoundInit(input)
	if err != nil {
		return api.Round{}, err
	}
	if input.Len() != 0 {
		if err := t.sweepDrafts(); err != nil {
			doors.Logger(t.link.backend.Context()).Error("thread: draft sweep failed", "thread", t.id, "err", err)
		}
	}
	t.switchStateRoundReplay(round, replay)
	return round, nil
}

func (t *thread) switchStateRoundReplay(round api.Round, replay []api.Message) {
	ctx, cancel := context.WithCancelCause(t.link.ctx)
	ch := t.link.backend.Provider().Stream(ctx, api.Request{
		System:   t.link.system(),
		Messages: replay,
		Tools:    t.link.tools(),
	})
	t.state = &threadRound{
		thread:   t,
		round:    round,
		ctx:      ctx,
		cancel:   cancel,
		ch:       ch,
		response: response.NewBulder(),
	}
}

func (t *thread) switchStateTools(round api.Round, calls []api.ContentToolUse) {
	ch, cancel := t.link.useTools(round.ID, calls)
	t.state = &threadTools{
		round:   round,
		thread:  t,
		ch:      ch,
		cancel:  cancel,
		results: make(map[string]api.ContentToolResult, len(calls)),
	}
}

func (t *thread) switchStateWait() {
	t.state = threadWait{thread: t}
}

func (t *thread) switchStateCompact(round api.Round, usage api.EventUsage, content api.Indexed[api.Content]) *threadCompact {
	state := &threadCompact{
		thread:  t,
		round:   round,
		usage:   usage,
		content: content,
	}
	t.state = state
	return state
}

func (t *thread) applyCompaction(cmd commandApplyCompaction) bool {
	if t.replay == nil {
		return false
	}
	compaction, ok := t.compaction.report(cmd.seq)
	if !ok {
		return false
	}
	if code, isErr := cmd.resp.Err(); isErr && code != api.CodeCapped {
		if code == api.CodeContextLength && compaction.Half() > 0 {
			t.compaction.chunk = compaction.Half()
			if _, err := t.compact(true); err != nil {
				t.criticalErr(nil, err)
				return false
			}
			if t.compaction.isActive() {
				return false
			}
			t.compaction.chunk = 0
		}
		t.replay.CompactionFailed()
		t.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}, Final: false, CanCompact: t.canCompact()}))
		switch code {
		case api.CodeAccess, api.CodeBusy, api.CodeNetwork, api.CodeUnknown, api.CodeInvalid:
			doors.Logger(t.link.backend.Context()).Error("thread: compaction failed", "thread", t.id, "code", code, "err", cmd.resp.Cause)
		}
		return false
	}
	err := t.replay.ApplyCompaction(compaction, cmd.resp.Content, cmd.resp.Usage)
	if errors.Is(err, replay.ErrEmptySummary) {
		t.replay.CompactionFailed()
		t.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}, Final: false, CanCompact: t.canCompact()}))
		return false
	}
	if err != nil {
		t.criticalErr(nil, err)
		return false
	}
	t.link.pageModule.Reset()
	usage := t.replay.Usage()
	t.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: usage, Final: false, CanCompact: t.canCompact()}))
	return true
}

func (t *thread) applyHeldCompaction() {
	held := t.compaction.held
	if held == nil {
		return
	}
	t.compaction.held = nil
	t.applyCompaction(*held)
}

func (t *thread) canCompact() bool {
	return !t.compaction.isActive() && t.replay.CanCompact()
}

func (t *thread) compact(force bool) (bool, error) {
	if t.compaction.isActive() {
		return false, nil
	}
	c, m, err := t.replay.PrepareCompaction(force, t.compaction.chunk)
	if err != nil {
		return false, err
	}
	if len(m) == 0 {
		return false, nil
	}
	ctx, seq := t.compaction.start(t.link.backend.Context(), c)
	t.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: true, Usage: api.EventUsage{}, Final: false, CanCompact: t.canCompact()}))
	events := t.link.backend.Provider().Stream(ctx, api.Request{
		System:   t.link.system(),
		Messages: m,
		Tools:    t.link.tools(),
	})
	go func() {
		b := response.NewBulder()
		for e := range events {
			b.On(e)
		}
		if ctx.Err() != nil {
			return
		}
		r := b.Flush()
		select {
		case <-t.close:
			return
		case t.ch <- commandApplyCompaction{
			resp: r,
			seq:  seq,
		}:
		}
	}()
	return true, nil
}
