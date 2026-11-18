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

package marksman

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/follow"
	"neui.dev/nekit/internal/chat/chat/rower/marksman/parser"
)

func New(initial string, interval time.Duration, fs api.FS) Marksman {
	m := &marksman{
		initialBuffer: new(bytes.Buffer),
		interval:      interval,
		fs:            fs,
	}
	m.parser = parser.New(m.block)
	m.parser.Write(initial)
	return m
}

type Marksman = *marksman

type marksman struct {
	mu            sync.Mutex
	ctx           context.Context
	closed        bool
	fs            api.FS
	initialBuffer *bytes.Buffer
	parser        parser.Parser
	cursor        *doors.Door
	interval      time.Duration
	partialPushed bool
	timer         *time.Timer
}

func (m *marksman) Append(chunk string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(chunk) == 0 {
		return
	}
	if m.closed {
		return
	}
	m.partialPushed = false
	m.parser.Write(chunk)
	if m.timer == nil && m.rendered() {
		m.armTimer()
	}
}

func (m Marksman) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.stopTimer()
	m.closed = true
	m.parser.Close()
	if m.cursor != nil {
		m.cursor.Static(m.ctx, nil)
		m.cursor = nil
	}

}

func (m *marksman) stopTimer() {
	if m.timer == nil {
		return
	}
	m.timer.Stop()
	m.timer = nil
}

func (m *marksman) armTimer() {
	var timer *time.Timer
	timer = time.AfterFunc(m.interval, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.ctx.Err() != nil {
			return
		}
		if m.timer != timer {
			return
		}
		m.timer = nil
		if m.partialPushed {
			return
		}
		partial, has := m.parser.Partial()
		if !has {
			return
		}
		m.partialPushed = true
		m.cursor.Inner(m.ctx, gox.Elem(func(cur gox.Cursor) error {
			return cur.Bytes(partial)
		}))
	})
	m.timer = timer
}

func (m *marksman) block(b []byte) {
	if !m.rendered() {
		m.initialBuffer.Write(b)
		return
	}
	m.stopTimer()
	newCursor := new(doors.Door)
	prevCursor := m.cursor
	m.cursor = newCursor
	prevCursor.Static(m.ctx, gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Any(follow.Mark()); err != nil {
			return err
		}
		if err := chunk(m.fs, b)(cur); err != nil {
			return err
		}
		return cur.Comp(newCursor)
	}))

}

func (m Marksman) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.rendered() {
			return errors.New("marker can be only rendered once, create new one")
		}
		initial := m.readInitalBuffer()
		if len(initial) != 0 {
			if err := chunk(m.fs, initial)(cur); err != nil {
				return err
			}
		}
		if m.closed {
			return nil
		}
		m.ctx = doors.DetachedContext(cur.Context())
		m.cursor = new(doors.Door)
		if partial, has := m.parser.Partial(); has {
			m.partialPushed = true
			m.cursor.Inner(cur.Context(), gox.Elem(func(cur gox.Cursor) error {
				return cur.Bytes(partial)
			}))
		}
		return cur.Comp(m.cursor)
	})

}

func (m *marksman) rendered() bool {
	return m.initialBuffer == nil
}

func (m *marksman) readInitalBuffer() []byte {
	initial := m.initialBuffer.Bytes()
	m.initialBuffer = nil
	return initial
}
