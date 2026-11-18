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
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
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
	rower *rower[api.IndexedEvent]
}

func (r Response) IsEmpty() bool {
	return r.rower.IsEmpty()
}

func (r Response) Close() {
	r.rower.Close()
}

func (r Response) On(e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		r.rower.Append(e)
	case api.EventThinking:
		r.rower.Append(e)
	case api.EventSummary:
		r.rower.Append(e)
	case api.EventClose:
		r.rower.Append(e)
	default:
	}
}

func (r Response) Main() gox.Elem {
	return r.rower.Main()
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

func (t *textRow) Append(e api.IndexedEvent) {
	switch e := e.(type) {
	case api.EventText:
		t.Marksman.Append(e.Text)
	case api.EventClose:
		t.Close()
	}
}

func (t *textRow) Close() {
	if t.closed {
		return
	}
	t.closed = true
	t.Marksman.Close()
	t.slot.Static(context.Background(), textDone())
}
