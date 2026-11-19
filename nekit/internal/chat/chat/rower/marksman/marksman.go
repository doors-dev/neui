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
		buffer:   new(bytes.Buffer),
		fs:       fs,
		interval: interval,
	}
	m.parser = parser.New(m.block)
	m.parser.Write(initial)
	return m
}

type Marksman = *marksman

type marksman struct {
	mu         sync.Mutex
	closed     bool
	fs         api.FS
	buffer     *bytes.Buffer
	interval   time.Duration
	parser     parser.Parser
	cursor     *doors.Door
	signal     *chan struct{}
	dirty      bool
	lastUpdate time.Time
}

func (m Marksman) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	m.parser.Close()
	if m.signal != nil {
		close(*m.signal)
		m.signal = nil
	}
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
	m.parser.Write(chunk)
	m.dirty = true
	if m.signal != nil {
		close(*m.signal)
		m.signal = nil
	}
}

func (m *marksman) block(b []byte) {
	m.buffer.Write(b)
}

func (m *marksman) loop(ctx context.Context) {
	m.lastUpdate = time.Now()
	for ctx.Err() == nil {
		m.mu.Lock()
		buf := m.readBuffer()
		if len(buf) == 0 {
			if m.closed {
				m.mu.Unlock()
				m.cursor.Static(ctx, nil)
				return
			}
			ch := make(chan struct{})
			m.signal = &ch
			dirty := m.dirty
			m.mu.Unlock()
			var timer <-chan time.Time
			if dirty {
				next := m.lastUpdate.Add(m.interval)
				dur := max(time.Until(next), 0)
				timer = time.After(dur)
			}
			select {
			case <-ctx.Done():
				return
			case <-timer:
				m.update(ctx)
			case <-ch:
			}
			continue
		}
		m.mu.Unlock()
		m.append(ctx, buf)
	}
}

func (m *marksman) update(ctx context.Context) {
	m.mu.Lock()
	if m.buffer.Len() != 0 {
		m.mu.Unlock()
		return
	}
	m.dirty = false
	partial, has := m.parser.Partial()
	if !has {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	done := m.cursor.Inner(ctx, gox.Elem(func(cur gox.Cursor) error {
		return cur.Bytes(partial)
	}))
	for ok := true; ok; {
		select {
		case <-ctx.Done():
			return
		case _, ok = <-done:
		}
	}
	m.lastUpdate = time.Now()
}

func (m *marksman) append(ctx context.Context, buf []byte) {
	newCursor := new(doors.Door)
	prevCursor := m.cursor
	m.cursor = newCursor
	done := prevCursor.Static(ctx, gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Any(follow.Mark()); err != nil {
			return err
		}
		if err := cur.Comp(chunk(m.fs, buf)); err != nil {
			return err
		}
		return cur.Comp(newCursor)
	}))
	for ok := true; ok; {
		select {
		case <-ctx.Done():
			return
		case _, ok = <-done:
		}
	}
	m.lastUpdate = time.Now()
}

func (m Marksman) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if buf := m.readBuffer(); len(buf) != 0 {
			if err := cur.Comp(chunk(m.fs, buf)); err != nil {
				return err
			}
		}
		if m.closed {
			return nil
		}
		m.cursor = new(doors.Door)
		if err := cur.Comp(doors.Go(m.loop)); err != nil {
			return err
		}
		return cur.Comp(m.cursor)
	})
}

func (m *marksman) readBuffer() []byte {
	if m.buffer.Len() == 0 {
		return nil
	}
	content := m.buffer.Bytes()
	m.buffer = &bytes.Buffer{}
	return content
}
