package thread

import (
	"errors"
	"github.com/doors-dev/doors"
	"time"

	"neui.dev/api"
	"neui.dev/internal/replay"
)

type threadWait struct {
	thread *thread
}

func (s threadWait) tick() bool {
	var ttl <-chan time.Time
	if s.thread.link == nil {
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		ttl = timer.C
	}
	select {
	case <-ttl:
		return false
	case cmd := <-s.thread.ch:
		s.on(cmd)
		return true
	}
}

func (s threadWait) dump(*api.ChatState) {}

func (s threadWait) on(cmd any) {
	switch cmd := cmd.(type) {
	case commandLink:
		if s.thread.link == nil || cmd.backend != s.thread.link.backend {
			link, err := s.thread.connect(cmd.backend, cmd.page, cmd.load, cmd.meta)
			if err != nil {
				cmd.Err(err)
				return
			}
			cmd.Ok(link)
			return
		}
		if s.thread.link.page == cmd.page && s.thread.link.linkCtx.Err() == nil {
			s.thread.link.meta = cmd.meta
			s.thread.link.send(NewMessageReload(cmd.op))
			cmd.Err(ErrLinked)
			return
		}
		s.thread.pending = false
		s.thread.link = s.thread.link.inherit(cmd.page, cmd.meta)
		cmd.Ok(s.thread.link.link())
	case commandUnlink:
		if s.thread.link != cmd.link {
			return
		}
		s.thread.disconnect()
	case commandRecord:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		created, err := s.thread.replay.Create()
		if err != nil {
			cmd.Err(err)
			s.thread.criticalErr(nil, err)
			return
		}
		if created {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventThreadCreated{}))
		}
		if err := s.thread.replay.Record(
			cmd.origin,
			cmd.kind,
			cmd.attrs,
			cmd.data,
		); err != nil {
			cmd.Err(err)
			s.thread.criticalErr(nil, err)
			return
		}
		round, err := s.thread.switchStateRound()
		if err != nil {
			cmd.Err(err)
			s.thread.criticalErr(nil, err)
			return
		}
		cmd.NoMessage()
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	case commandTaskResult:
		if s.thread.link == nil || s.thread.link.linkTasks != cmd.linkTasks {
			return
		}
		found, err := s.thread.taskResult(cmd.id, cmd.output, cmd.err)
		if err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		if !found || !s.thread.replay.IsPending() {
			return
		}
		round, err := s.thread.switchStateRound()
		if err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	case commandTaskNotify:
		if s.thread.link == nil || s.thread.link.linkTasks != cmd.linkTasks {
			return
		}
		task, ok := s.thread.link.taskGet(cmd.id)
		if !ok {
			return
		}
		if err := s.thread.taskNotify(task, cmd.data, cmd.meta); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		if !s.thread.replay.IsPending() {
			return
		}
		round, err := s.thread.switchStateRound()
		if err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	case commandSend:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
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
		s.thread.input = cmd.input
		if s.thread.input.Len() == 0 && !s.thread.replay.IsPending() {
			cmd.Err(api.ErrWrongState)
			return
		}
		s.thread.pending = true
		round, err := s.thread.switchStateRound()
		if err != nil {
			s.thread.criticalErr(cmd.intoMessage(), err)
			return
		}
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	case commandRewind:
		if cmd.link != s.thread.link {
			cmd.Err(api.ErrUnlinked)
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
		round, replay, removed, err := s.thread.replay.Rewind(cmd.round, cmd.input)
		if errors.Is(err, api.ErrWrongState) {
			cmd.Err(err)
			return
		}
		wasPending := s.thread.pending
		s.thread.pending = false
		if s.thread.compaction.cut(cmd.round) {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}, Final: false, CanCompact: s.thread.canCompact()}))
		}
		if err != nil {
			s.thread.criticalErr(cmd.intoMessage(), err)
			return
		}
		if s.thread.link.taskCut(cmd.round) {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventTasks{Tasks: s.thread.link.taskList()}))
		}
		s.thread.rewindAttachments(removed, cmd.input)
		s.thread.link.pageModule.Reset()
		if err := s.thread.sweepDrafts(); err != nil {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: draft sweep failed", "thread", s.thread.id, "err", err)
		}
		s.thread.switchStateRoundReplay(round, replay)
		if wasPending {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
		}
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventRoundStart{Round: round, Truncate: cmd.round, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	case commandRetry:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !s.thread.replay.IsRetriable() {
			cmd.Err(api.ErrWrongState)
			return
		}
		round, truncate, replay, err := s.thread.replay.Retry()
		if err != nil {
			s.thread.criticalErr(cmd.intoMessage(), err)
			return
		}
		s.thread.switchStateRoundReplay(round, replay)
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventRoundStart{Round: round, Truncate: truncate, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
	default:
		s.thread.on(cmd)
	}
}
