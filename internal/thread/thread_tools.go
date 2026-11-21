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

package thread

import (
	"context"
	"errors"
	"github.com/doors-dev/doors"
	"maps"

	"neui.dev/api"
	"neui.dev/internal/replay"
)

type threadTools struct {
	round   api.Round
	thread  *thread
	cancel  context.CancelCauseFunc
	stopped bool
	ch      <-chan api.ContentToolResult
	results map[string]api.ContentToolResult
	ask     *api.ChatAsk
}

func (s *threadTools) tick() bool {
	select {
	case r, ok := <-s.ch:
		if !ok {
			s.finalize()
			return true
		}
		s.results[r.ID] = r
	case cmd := <-s.thread.ch:
		s.on(cmd)
	}
	return true
}

func (s *threadTools) dump(state *api.ChatState) {
	round := s.round
	round.Results = maps.Clone(s.results)
	state.Round = &round
	state.Ask = s.ask
}

func (s *threadTools) on(cmd any) {
	switch cmd := cmd.(type) {
	case commandAskDone:
		if s.ask != cmd.ask {
			cmd.Operation.NoMessage()
			return
		}
		s.ask = nil
		s.thread.link.send(newMessageEvent(cmd.intoMessage(), api.ChatEventAskDone{}))
	case commandAsk:
		if s.thread.link.linkTasks != cmd.link.linkTasks {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if s.ask != nil {
			s.ask.Cancel(context.Background())
		}
		s.ask = cmd.ask
		cmd.Ok(struct{}{})
		s.thread.link.send(newMessageEvent(nil, api.ChatEventAsk{Ask: *s.ask}))
	case commandLink:
		if cmd.backend != s.thread.link.backend {
			defer s.thread.switchStateWait()
			s.cancel(api.ErrUnlinked)
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
		s.thread.disconnect()
	case commandStop:
		if cmd.link != s.thread.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		cmd.NoMessage()
		s.stop()
	case commandRewind:
		if cmd.link != s.thread.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		s.stop()
		if s.ask != nil {
			s.thread.link.send(newMessageEvent(nil, api.ChatEventAskDone{}))
			s.ask = nil
		}
		s.thread.rewind(cmd)
	case commandTask:
		if s.thread.link == nil || s.thread.link.linkTasks != cmd.linkTasks {
			cmd.Err(context.Canceled)
			return
		}
		if cmd.origin != s.round.ID {
			cmd.Err(errors.New("task can be issued only during the tool calls round of its tool"))
			return
		}
		cmd.Ok(s.thread.link.taskAdd(cmd.origin, cmd.module, cmd.name, cmd.comment, cmd.attrs, cmd.cancel))
		s.thread.link.send(newMessageEvent(nil, api.ChatEventTasks{Tasks: s.thread.link.taskList()}))
	case commandApplyCompaction:
		if cmd.seq != s.thread.compaction.seq {
			return
		}
		s.thread.compaction.held = &cmd
	default:
		s.thread.on(cmd)
	}
}

func (s *threadTools) stop() {
	s.cancel(api.ErrCanceledByUser)
	s.stopped = true
	wasPending := s.thread.pending
	s.thread.pending = false
	if wasPending {
		s.thread.link.send(newMessageEvent(nil, api.ChatEventInputPending{Pending: false}))
	}
}

func (s *threadTools) finalize() {
	if s.ask != nil {
		s.thread.link.send(newMessageEvent(nil, api.ChatEventAskDone{}))
		s.ask = nil
	}
	if err := s.thread.link.fs.TempWipe(); err != nil {
		doors.Logger(s.thread.link.backend.Context()).Error("thread: scratch wipe failed", "thread", s.thread.id, "err", err)
	}
	if err := s.thread.replay.RoundResult(s.results); err != nil {
		s.thread.criticalErr(nil, err)
		return
	}
	s.thread.link.send(newMessageEvent(nil, api.ChatEventToolResults{Round: s.round.ID, Results: s.results, Final: s.stopped || !s.thread.replay.IsPending()}))
	s.thread.applyHeldCompaction()
	if s.thread.replay == nil {
		return
	}
	if s.stopped || (!s.thread.pending && !s.thread.replay.IsPending()) {
		s.thread.switchStateWait()
		return
	}
	if s.thread.replay.NeedCompaction() == replay.CompactionBytes {
		if _, err := s.thread.compact(false); err != nil {
			doors.Logger(s.thread.link.backend.Context()).Error("thread: compaction", "thread", s.thread.id, "err", err)
		}
		if s.thread.compaction.isActive() {
			held := s.round
			held.Results = maps.Clone(s.results)
			s.thread.switchStateCompact(held, "", nil, api.EventUsage{}, held.Response)
			return
		}
	}
	round, err := s.thread.switchStateRound()
	if err != nil {
		s.thread.criticalErr(nil, err)
		return
	}
	s.thread.link.send(newMessageEvent(nil, api.ChatEventRoundStart{Round: round, Truncate: api.NoRoundID, Usage: s.thread.replay.Usage(), CanCompact: s.thread.canCompact()}))
}
