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

package rower

import (
	"context"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/rower/marksman"
	"neui.dev/nekit/internal/chat/params"
)

func NewResponse(p params.Params, state api.Indexed[api.Content], fs api.FS) Response {
	content := make(map[int]streamer[api.IndexedEvent])
	for index, c := range state {
		switch c := c.(type) {
		case api.ContentText:
			content[index] = newTextRow(c.Text, fs)
		case api.ContentReasoning:
			if p.Off(params.Thinking) {
				continue
			}
			content[index] = newThinking(c.Thoughts, c.Summary)
		}
	}
	r := newRower(content, func(e api.IndexedEvent) streamer[api.IndexedEvent] {
		switch e := e.(type) {
		case api.EventText:
			return newTextRow(e.Text, fs)
		case api.EventThinking:
			if p.Off(params.Thinking) {
				return nil
			}
			return newThinking(
				map[int]string{
					e.Part: e.Text,
				},
				nil,
			)
		case api.EventSummary:
			if p.Off(params.Thinking) {
				return nil
			}
			return newThinking(
				nil,
				map[int]string{
					e.Part: e.Text,
				},
			)
		default:
			return nil
		}
	})
	return &response{
		rower: r,
	}
}

type Response = *response

type response struct {
	rower   *rower[api.IndexedEvent]
	mu      sync.Mutex
	started bool
	closed  bool
	pending deque.Deque[replacement]
	signal  *chan struct{}
}

func (r Response) IsEmpty() bool {
	return r.rower.IsEmpty()
}

func (r Response) Close() {
	r.push(r.rower.Close())
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.signal != nil {
		close(*r.signal)
		r.signal = nil
	}
}

func (r Response) On(e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		r.push(r.rower.Append(e))
	case api.EventThinking:
		r.push(r.rower.Append(e))
	case api.EventSummary:
		r.push(r.rower.Append(e))
	case api.EventClose:
		r.push(r.rower.Append(e))
	default:
	}
}

func (r Response) push(replacements []replacement) {
	if len(replacements) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		for _, rp := range replacements {
			rp.door.Static(context.Background(), rp.content)
		}
		return
	}
	for _, rp := range replacements {
		r.pending.PushBack(rp)
	}
	if r.signal != nil {
		close(*r.signal)
		r.signal = nil
	}
}

func (r Response) loop(ctx context.Context) {
	for ctx.Err() == nil {
		r.mu.Lock()
		if r.pending.Len() == 0 {
			if r.closed {
				r.mu.Unlock()
				return
			}
			ch := make(chan struct{})
			r.signal = &ch
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ch:
			}
			continue
		}
		next := r.pending.PopFront()
		r.mu.Unlock()
		done := next.door.Static(ctx, next.content)
		for ok := true; ok; {
			select {
			case <-ctx.Done():
				return
			case _, ok = <-done:
			}
		}
	}
}

func (r Response) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		r.mu.Lock()
		r.started = true
		closed := r.closed
		r.mu.Unlock()
		if err := cur.Comp(r.rower.Main()); err != nil {
			return err
		}
		if closed {
			return nil
		}
		return cur.Comp(doors.Go(r.loop))
	})
}

func newTextRow(text string, fs api.FS) *textRow {
	return &textRow{
		Marksman: marksman.New(text, 5*time.Second, fs),
		slot:     newSlot(textRunning()),
	}
}

type textRow struct {
	marksman.Marksman
	slot   *doors.Door
	closed bool
}

var _ streamer[api.IndexedEvent] = &textRow{}

func (t *textRow) Main() gox.Elem {
	return block{"Text", t.slot, gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "nechat-col nechat-prose"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Comp(t.Marksman); err != nil {
			return err
		}
		return cur.Close()
	})}.Main()
}

func (t *textRow) Append(e api.IndexedEvent) []replacement {
	switch e := e.(type) {
	case api.EventText:
		t.Marksman.Append(e.Text)
	case api.EventClose:
		return t.Close()
	}
	return nil
}

func (t *textRow) Close() []replacement {
	if t.closed {
		return nil
	}
	t.closed = true
	t.Marksman.Close()
	return []replacement{{t.slot, textDone()}}
}
