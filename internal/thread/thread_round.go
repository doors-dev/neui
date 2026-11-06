package thread

import (
	"context"
	"errors"
	"github.com/doors-dev/doors"

	"neui.dev/api"
	"neui.dev/internal/replay"
	"neui.dev/internal/thread/response"
)

type threadRound struct {
	thread   *thread
	round    api.Round
	ctx      context.Context
	cancel   context.CancelCauseFunc
	ch       <-chan api.Event
	response response.Builder
}

func (s *threadRound) tick() bool {
	select {
	case e, contunue := <-s.ch:
		if !contunue {
			s.finalize()
			return true
		}
		e, contunue = s.response.On(e)
		switch e := e.(type) {
		case api.IndexedEvent:
			s.thread.link.send(newMessageEvent(nil, api.ChatEventIndexed{Event: e}))
		case api.EventUsage:
			s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundUsage{Round: s.round.ID, Usage: e}))
		}
		if !contunue {
			s.cancel(nil)
			s.finalize()
		}
	case cmd := <-s.thread.ch:
		s.on(cmd)
	}
	return true
}

func (s *threadRound) dump(state *api.ChatState) {
	round := s.round
	round.Response = s.response.State()
	state.Round = &round
}

func (s *threadRound) on(cmd any) {
	switch cmd := cmd.(type) {
	case commandLink:
		if cmd.backend != s.thread.link.backend {
			defer s.thread.switchStateWait()
			s.cancel(api.ErrUnlinked)
			if _, err := s.abort(); err != nil {
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
		s.cancel(api.ErrUnlinked)
		if _, err := s.abort(); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.disconnect()
	case commandStop:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		defer s.thread.switchStateWait()
		wasPending := s.thread.pending
		s.thread.pending = false
		s.cancel(api.ErrCanceledByUser)
		usage, err := s.abort()
		if err != nil {
			s.thread.criticalErr(cmd.intoMessage(), err)
			return
		}
		if wasPending {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
		}
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundAbort, Error: "", Usage: usage, Final: true}))
		s.thread.applyHeldCompaction()
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
		round, replay, removed, err := s.thread.replay.Rewind(cmd.round, cmd.input)
		if errors.Is(err, api.ErrWrongState) {
			cmd.Err(err)
			return
		}
		wasPending := s.thread.pending
		s.thread.pending = false
		s.cancel(api.ErrCanceledByUser)
		if s.thread.compaction.cut(cmd.round) {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}}))
		}
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
		s.thread.compaction.held = &cmd
	default:
		s.thread.on(cmd)
	}
}

func (s *threadRound) abort() (api.EventUsage, error) {
	usage, content := s.response.Abort()
	return usage, s.thread.replay.RoundAbort(usage, content)
}

func (s *threadRound) finalize() {
	s.cancel(nil)
	resp := s.response.Flush()
	if code, ok := resp.Ok(); ok {
		s.ok(code, resp.Content, resp.Usage)
	} else if code, ok := resp.Err(); ok {
		s.err(code, resp.Cause, resp.Content, resp.Usage)
	} else {
		panic("unexpected response type")
	}
}

func (s *threadRound) ok(code api.OkCode, content api.Indexed[api.Content], usage api.EventUsage) {
	switch code {
	case api.CodeTurn:
	default:
		panic("unknown ok code")
	}
	origin, err := s.thread.replay.RoundOk(usage, content)
	if err != nil {
		s.thread.criticalErr(nil, err)
		return
	}
	cause := s.thread.replay.NeedCompaction()
	if cause != replay.CompactionNone {
		if _, err := s.thread.compact(); err != nil {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: compaction", "thread", s.thread.id, "err", err)
		}
	}
	toolCalls := filterTools(content)
	if len(toolCalls) != 0 {
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: origin, State: api.RoundOk, Error: "", Usage: usage, Final: false}))
		s.thread.applyHeldCompaction()
		if s.thread.replay == nil {
			return
		}
		round := s.round
		round.State = api.RoundOk
		round.Response = content
		s.thread.switchStateTools(round, toolCalls)
		return
	}
	s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: origin, State: api.RoundOk, Error: "", Usage: usage, Final: !s.thread.replay.IsPending()}))
	s.thread.applyHeldCompaction()
	if s.thread.replay == nil {
		return
	}
	if !s.thread.pending && !s.thread.replay.IsPending() {
		s.thread.switchStateWait()
		return
	}
	if cause == replay.CompactionBytes && s.thread.compaction.isActive() {
		round := s.round
		round.State = api.RoundOk
		round.Response = content
		s.thread.switchStateCompact(round, usage, content)
		return
	}
	round, err := s.thread.switchStateRound()
	if err != nil {
		s.thread.criticalErr(nil, err)
		return
	}
	s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID}))
}

func (s *threadRound) err(code api.ErrCode, cause error, content api.Indexed[api.Content], usage api.EventUsage) {
	switch code {
	case api.CodeAccess,
		api.CodeBusy,
		api.CodeNetwork,
		api.CodeUnknown,
		api.CodeCapped:
		if code != api.CodeCapped {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: turn failed", "thread", s.thread.id, "code", code, "err", cause)
		}
		if err := s.thread.replay.RoundError(errors.New(string(code)), usage, content); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundError, Error: string(code), Usage: usage, Final: true}))
	case api.CodeInvalid:
		doors.Logger(s.thread.link.backend.Context()).Error("thread: turn failed", "thread", s.thread.id, "code", code, "err", cause)
		if err := s.thread.replay.RoundReject(code, usage, content); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundError, Error: string(code), Usage: usage, Final: true}))
	case api.CodeContextLength:
		if held := s.thread.compaction.held; held != nil {
			s.thread.compaction.held = nil
			s.thread.switchStateCompact(s.round, usage, content).on(*held)
			return
		}
		if _, err := s.thread.compact(); err != nil {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: compaction", "thread", s.thread.id, "err", err)
		}
		if s.thread.compaction.isActive() {
			s.thread.switchStateCompact(s.round, usage, content)
			return
		}
		if err := s.thread.replay.RoundError(errors.New(string(code)), usage, content); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundError, Error: string(code), Usage: usage, Final: true}))
	case api.CodeTooLarge:
		if !errors.Is(cause, api.ErrMeasuredTooLarge) && s.thread.replay.Tighten() {
			round, truncate, messages, err := s.thread.replay.Retry()
			if err != nil {
				s.thread.criticalErr(nil, err)
				return
			}
			s.thread.switchStateRoundReplay(round, messages)
			s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: truncate}))
			return
		}
		if err := s.thread.replay.RoundReject(code, usage, content); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundError, Error: string(code), Usage: usage, Final: true}))
	case api.CodeRefused:
		if err := s.thread.replay.RoundRefuse(usage, content); err != nil {
			s.thread.criticalErr(nil, err)
			return
		}
		s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundEnd{Round: s.round.ID, State: api.RoundRefuse, Error: "", Usage: usage, Final: true}))
	default:
		panic("unknown errror code")
	}
	wasPending := s.thread.pending
	s.thread.pending = false
	if wasPending {
		s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
	}
	s.thread.applyHeldCompaction()
	s.thread.switchStateWait()
}
