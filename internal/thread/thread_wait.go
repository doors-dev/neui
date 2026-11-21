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
		s.thread.rewind(cmd)
	case commandRetry:
		if s.thread.link != cmd.link {
			cmd.Err(api.ErrUnlinked)
			return
		}
		if !s.thread.replay.IsRetriable() {
			cmd.Err(api.ErrWrongState)
			return
		}
		round, truncate, replay, err := s.thread.replay.Retry(false)
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
