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
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/rower/marksman"
	"neui.dev/nekit/internal/chat/params"
)

func NewResponse(ctx context.Context, p params.Params, state api.Indexed[api.Content], fs api.FS, lead Chips) Response {
	r := &response{
		params: p,
		lead:   lead,
	}
	content := make(map[int]streamer[api.IndexedEvent])
	for _, index := range slices.Sorted(maps.Keys(state)) {
		switch c := state[index].(type) {
		case api.ContentText:
			if strings.TrimSpace(c.Text) == "" {
				continue
			}
			content[index] = newTextRow(c.Text, fs)
			r.hasText = true
			if r.afterText != nil {
				r.afterText.Close(ctx)
				r.afterText = nil
			}
		case api.ContentReasoning:
			if p.Off(params.Thinking) {
				continue
			}
			chips := lead
			if r.hasText {
				if r.afterText == nil {
					r.afterText = NewChips(p, nil, false)
					content[index] = chipsRow{r.afterText}
				}
				chips = r.afterText
			}
			for _, part := range slices.Sorted(maps.Keys(c.Summary)) {
				chips.Reason(ctx, api.EventSummary{Index: index, Part: part, Text: c.Summary[part]})
			}
			for _, part := range slices.Sorted(maps.Keys(c.Thoughts)) {
				chips.Reason(ctx, api.EventThinking{Index: index, Part: part, Text: c.Thoughts[part]})
			}
		}
	}
	r.rower = newRower(content, func(e api.IndexedEvent) streamer[api.IndexedEvent] {
		text, ok := e.(api.EventText)
		if !ok || strings.TrimSpace(text.Text) == "" {
			return nil
		}
		return newTextRow(text.Text, fs)
	})
	return r
}

type Response = *response

type response struct {
	params    params.Params
	rower     *rower[api.IndexedEvent]
	lead      Chips
	hasText   bool
	afterText Chips
	mu        sync.Mutex
	started   bool
	closed    bool
	pending   deque.Deque[replacement]
	signal    *chan struct{}
}

func (r Response) IsEmpty() bool {
	return r.rower.IsEmpty()
}

func (r Response) AfterText() Chips {
	return r.afterText
}

func (r Response) Close(ctx context.Context) {
	r.push(r.rower.Close())
	if r.afterText != nil {
		r.afterText.Close(ctx)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.signal != nil {
		close(*r.signal)
		r.signal = nil
	}
}

func (r Response) On(ctx context.Context, e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		if strings.TrimSpace(e.Text) != "" {
			r.hasText = true
			if r.afterText != nil {
				r.afterText.Close(ctx)
				r.afterText = nil
			}
		}
		r.push(r.rower.Append(e))
	case api.EventThinking, api.EventSummary:
		if r.params.Off(params.Thinking) {
			return
		}
		if !r.hasText {
			r.lead.Reason(ctx, e)
			return
		}
		if r.afterText == nil {
			r.afterText = NewChips(r.params, nil, false)
			r.push(r.rower.Insert(e.At(), chipsRow{r.afterText}))
		}
		r.afterText.Reason(ctx, e)
	case api.EventClose:
		r.push(r.rower.Append(e))
		r.lead.Reason(ctx, e)
		if r.afterText != nil {
			r.afterText.Reason(ctx, e)
		}
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

type chipsRow struct {
	Chips
}

func (chipsRow) Append(api.IndexedEvent) []replacement {
	return nil
}

func (chipsRow) Close() []replacement {
	return nil
}
