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
	"sync/atomic"

	"github.com/doors-dev/doors"
	"neui.dev/api"
)

type Message interface {
	Ack()
	Add()
}

func NewMessageInit(state api.ChatState, fs api.FS) MessageInit {
	return MessageInit{state, fs, nil}
}

type MessageInit struct {
	State   api.ChatState
	FS      api.FS
	message *message
}

func (m MessageInit) Enabled() bool {
	return m.State.ID != ""
}

func (m MessageInit) Ack() {
	m.message.ack()
}

func (m MessageInit) Add() {
	m.message.add()
}

type MessageReload struct {
	message *message
}

func (m MessageReload) Ack() {
	m.message.ack()
}

func (m MessageReload) Add() {
	m.message.add()
}

func NewMessageReload(op Operation) MessageReload {
	return MessageReload{op.intoMessage()}
}

type MessageEvent struct {
	Event   api.ChatEvent
	message *message
}

func (m MessageEvent) Ack() {
	m.message.ack()
}

func (m MessageEvent) Add() {
	m.message.add()
}

func newMessageEvent(m *message, event api.ChatEvent) MessageEvent {
	return MessageEvent{event, m}
}

type Operation = *operation

func NewOperation(ctx context.Context) Operation {
	return &operation{
		ch:  make(chan error, 1),
		ctx: ctx,
	}
}

type operation struct {
	ch  chan error
	ctx context.Context
}

func (o Operation) Receive() error {
	err, _ := <-o.ch
	return err
}

func (o Operation) Err(err error) {
	o.ch <- err
	close(o.ch)
}

func (o Operation) NoMessage() {
	close(o.ch)
}

func (o *operation) intoMessage() *message {
	if o == nil {
		return nil
	}
	release := func() {}
	if doors.HasInstance(o.ctx) {
		release = doors.HoldSettle(o.ctx)
	}
	close(o.ch)
	m := &message{release: release}
	m.counter.Add(1)
	return m
}

type message struct {
	counter atomic.Int32
	release func()
}

func (r *message) add() {
	if r == nil {
		return
	}
	if r.counter.Add(1) == 1 {
		panic("can't be raised from 0, order of clone/ack was violated")
	}
}

func (r *message) ack() {
	if r == nil {
		return
	}
	if r.counter.Add(-1) != 0 {
		return
	}
	r.release()
}
