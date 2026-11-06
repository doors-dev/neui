package thread

import (
	"errors"

	"github.com/doors-dev/doors"

	"neui.dev/api"
)

type threadCompact struct {
	thread  *thread
	round   api.Round
	usage   api.EventUsage
	content api.Indexed[api.Content]
}

func (s *threadCompact) tick() bool {
	s.on(<-s.thread.ch)
	return true
}

func (s *threadCompact) dump(state *api.ChatState) {
	round := s.round
	round.Response = s.content
	state.Round = &round
}

func (s *threadCompact) on(cmd any) {
	switch cmd := cmd.(type) {
	case commandLink:
		if cmd.backend != s.thread.link.backend {
			defer s.thread.switchStateWait()
			if err := s.abort(); err != nil {
				s.thread.criticalErr(nil, err)
			}
			link, err := s.thread.connect(cmd.backend, cmd.page, true, cmd.meta)
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
		defer s.thread.switchStateWait()
		if err := s.abort(); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.disconnect()
	case commandStop:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !s.thread.replay.IsRetriable() {
			cmd.Err(api.ErrWrongState)
			return
		}
		cmd.NoMessage()
		s.thread.compaction.stop()
		wasPending := s.thread.pending
		s.thread.pending = false
		if wasPending {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}}))
		if err := s.abort(); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundAbort, Error: "", Usage: s.usage, Final: true}))
		s.thread.switchStateWait()
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
		if inputBytes > cmd.link.backend.Config().InputBytes {
			cmd.Err(api.ErrInputTooLarge)
			return
		}
		wasPending := s.thread.pending
		s.thread.pending = false
		if s.thread.compaction.cut(cmd.round) {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}}))
		}
		round, replay, removed, err := s.thread.replay.Rewind(cmd.round, cmd.input)
		if err != nil {
			s.thread.criticalErr(cmd.intoMessage(), err)
			return
		}
		if s.thread.link.taskCut(cmd.round) {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventTasks{Tasks: s.thread.link.taskList()}))
		}
		s.thread.rewindAttachments(removed, cmd.input)
		if err := s.thread.sweepDrafts(); err != nil {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: draft sweep failed", "thread", s.thread.id, "err", err)
		}
		s.thread.switchStateRoundReplay(round, replay)
		if wasPending {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
		}
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventRoundStart{Round: round, Truncate: cmd.round}))
	case commandApplyCompaction:
		if cmd.seq != s.thread.compaction.seq {
			return
		}
		applied := s.thread.applyCompaction(cmd)
		if s.thread.replay == nil {
			return
		}
		if !s.thread.replay.IsRetriable() {
			if !s.thread.pending && !s.thread.replay.IsPending() {
				s.thread.switchStateWait()
				return
			}
			round, err := s.thread.switchStateRound()
			if err != nil {
				s.thread.criticalErr(nil, err)
				return
			}
			s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID}))
			return
		}
		if !applied {
			if err := s.thread.replay.RoundError(errors.New(string(api.CodeContextLength)), s.usage, s.content); err != nil {
				s.thread.criticalErr(nil, err)
				return
			}
			s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundError, Error: string(api.CodeContextLength), Usage: s.usage, Final: true}))
			s.thread.switchStateWait()
			return
		}
		round, truncate, replay, err := s.thread.replay.Retry()
		if err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.switchStateRoundReplay(round, replay)
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: truncate}))
	default:
		s.thread.on(cmd)
	}
}

func (s *threadCompact) abort() error {
	if !s.thread.replay.IsRetriable() {
		return nil
	}
	return s.thread.replay.RoundAbort(s.usage, s.content)
}
