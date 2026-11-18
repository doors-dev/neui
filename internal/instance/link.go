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

package instance

import (
	"context"
	"sync"
	"sync/atomic"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/thread"
)

func newLink(i *instance) Link {
	ctx, cancel := context.WithCancel(i.ctx)
	return &link{
		inst:   i,
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan thread.Message),
	}
}

type Link = *link

type link struct {
	initialized bool
	inst        *instance
	backend     *common.Backend
	thread      api.ThreadID
	ctx         context.Context
	cancel      context.CancelFunc
	ch          chan thread.Message
	once        sync.Once
	dead        atomic.Bool
}

func (l Link) Stop(ctx context.Context) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Stop(op)
	})
}

func (l Link) Send(ctx context.Context, input api.Input) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Send(op, input)
	})
}

func (l Link) Unsend(ctx context.Context) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Unsend(op)
	})
}

func (l Link) Input(ctx context.Context, input api.Input) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Input(op, input)
	})
}

func (l Link) Retry(ctx context.Context) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Retry(op)
	})
}

func (l Link) Compact(ctx context.Context) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Compact(op)
	})
}

func (l Link) Rewind(ctx context.Context, round api.RoundID, input api.Input) error {
	return l.commandUplink(ctx, func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Rewind(op, round, input)
	})
}

func (l Link) Model() api.Model {
	if l.backend == nil {
		return api.Model{}
	}
	return l.backend.Provider().Model()
}

func (l Link) Config() api.ThreadConfig {
	if l.backend == nil {
		return api.ThreadConfig{}
	}
	return l.backend.Config()
}

func (l Link) RoundBefore(ctx context.Context, start api.RoundID, limit int) ([]api.Round, error) {
	if l.backend == nil || l.dead.Load() || l.ctx.Err() != nil {
		return nil, api.ErrUnlinked
	}
	return l.backend.Storage().RoundBefore(ctx, l.thread, start, limit)
}

func (l Link) RoundAfter(ctx context.Context, start api.RoundID, limit int) ([]api.Round, error) {
	if l.backend == nil || l.dead.Load() || l.ctx.Err() != nil {
		return nil, api.ErrUnlinked
	}
	return l.backend.Storage().RoundAfter(ctx, l.thread, start, limit)
}

func (l Link) Close() {
	l.once.Do(func() {
		l.cancel()
		l.inst.send(commandUnlink{l})
	})
}

func (l Link) commandUplink(ctx context.Context, make func(uplink thread.Link, op thread.Operation) thread.Command) error {
	op := thread.NewOperation(ctx)
	cmd := commandUplink{
		link:      l,
		Operation: op,
		make:      make,
	}
	if err := l.inst.send(cmd); err != nil {
		return err
	}
	return op.Receive()
}

func (l *link) disconnect() {
	close(l.ch)
}

func (l Link) Ch() <-chan thread.Message {
	return l.ch
}

func (l *link) prepeare(m thread.Message) (thread.Message, bool) {
	switch m.(type) {
	case thread.MessageInit:
		return m, !l.initialized
	case thread.MessageReload:
		return m, true
	}
	return m, l.initialized
}

func (l *link) send(m thread.Message) {
	m, send := l.prepeare(m)
	if !send {
		return
	}
	m.Add()
	select {
	case <-l.ctx.Done():
		m.Ack()
	case l.ch <- m:
		l.initialized = true
	}
}
