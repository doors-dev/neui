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

package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"neui.dev/api"
)

var ErrClosed = errors.New("thread: block already closed")
var ErrMismatch = errors.New("thread: event does not belong")

func closedErr(kind string, e api.IndexedEvent) error {
	return fmt.Errorf("%w: %T arrived for %s block %d", ErrClosed, e, kind, e.At())
}

func mismatchErr(kind string, e api.IndexedEvent) error {
	return fmt.Errorf("%w: %s block %d cannot take %T", ErrMismatch, kind, e.At(), e)
}

func openErr(e api.IndexedEvent) error {
	return fmt.Errorf("%w: %T cannot open a block at %d", ErrMismatch, e, e.At())
}

type block interface {
	append(e api.IndexedEvent) error
	assemble(final bool) api.Content
}

func newBlock(model string, e api.IndexedEvent) (block, error) {
	switch e := e.(type) {
	case api.EventText:
		block := &textBlock{}
		return block, block.append(e)
	case api.EventTool:
		block := &toolBlock{}
		return block, block.append(e)
	case api.EventThinking:
		block := &reasoningBlock{
			model:     model,
			thoughts:  make(map[int]*strings.Builder),
			summaries: make(map[int]*strings.Builder),
		}
		return block, block.append(e)
	case api.EventSummary:
		block := &reasoningBlock{
			model:     model,
			thoughts:  make(map[int]*strings.Builder),
			summaries: make(map[int]*strings.Builder),
		}
		return block, block.append(e)
	case api.EventOpaque:
		block := &opaqueBlock{
			model: model,
		}
		return block, block.append(e)
	}
	return nil, openErr(e)
}

type textBlock struct {
	closed bool
	meta   json.RawMessage
	text   strings.Builder
}

func (t *textBlock) assemble(final bool) api.Content {
	return api.ContentText{
		Text: t.text.String(),
		Meta: t.meta,
	}
}

func (t *textBlock) append(e api.IndexedEvent) error {
	if t.closed {
		return closedErr("text", e)
	}
	switch e := e.(type) {
	case api.EventText:
		if e.Text != "" {
			t.text.WriteString(e.Text)
		}
	case api.EventClose:
		t.closed = true
	case api.EventMeta:
		if len(e.Data) != 0 {
			t.meta = e.Data
		}
	default:
		return mismatchErr("text", e)
	}
	return nil
}

type toolBlock struct {
	closed bool
	meta   json.RawMessage
	id     string
	name   strings.Builder
	args   bytes.Buffer
}

func (t *toolBlock) assemble(final bool) api.Content {
	input := json.RawMessage(t.args.Bytes())
	if t.args.Len() == 0 {
		input = nil
	} else if !json.Valid(input) {
		if final {
			slog.Warn("thread: tool call arguments discarded, never formed valid JSON", "id", t.id, "name", t.name.String(), "args", t.args.String())
		}
		input = api.DiscardedArguments
	}
	return api.ContentToolUse{
		ID:    t.id,
		Name:  t.name.String(),
		Input: input,
		Meta:  t.meta,
	}
}

func (t *toolBlock) append(e api.IndexedEvent) error {
	if t.closed {
		return closedErr("tool", e)
	}
	switch e := e.(type) {
	case api.EventTool:
		if e.ID != "" {
			t.id = e.ID
		}
		if e.Name != "" {
			t.name.WriteString(e.Name)
		}
		if e.Args != "" {
			t.args.WriteString(e.Args)
		}
	case api.EventClose:
		t.closed = true
	case api.EventMeta:
		if len(e.Data) != 0 {
			t.meta = e.Data
		}
	default:
		return mismatchErr("tool", e)
	}
	return nil
}

type reasoningBlock struct {
	closed    bool
	model     string
	meta      json.RawMessage
	thoughts  map[int]*strings.Builder
	summaries map[int]*strings.Builder
}

func (r *reasoningBlock) assemble(final bool) api.Content {
	return api.ContentReasoning{
		Model:    r.model,
		Summary:  partsText(r.summaries),
		Thoughts: partsText(r.thoughts),
		Meta:     r.meta,
	}
}

func (r *reasoningBlock) append(e api.IndexedEvent) error {
	if r.closed {
		return closedErr("reasoning", e)
	}
	switch e := e.(type) {
	case api.EventThinking:
		if e.Text != "" {
			if _, has := r.thoughts[e.Part]; !has {
				r.thoughts[e.Part] = new(strings.Builder)
			}
			r.thoughts[e.Part].WriteString(e.Text)
		}
	case api.EventSummary:
		if e.Text != "" {
			if _, has := r.summaries[e.Part]; !has {
				r.summaries[e.Part] = new(strings.Builder)
			}
			r.summaries[e.Part].WriteString(e.Text)
		}
	case api.EventClose:
		r.closed = true
	case api.EventMeta:
		if len(e.Data) != 0 {
			r.meta = e.Data
		}
	default:
		return mismatchErr("reasoning", e)
	}
	return nil
}

type opaqueBlock struct {
	closed bool
	model  string
	kind   string
	data   json.RawMessage
	deltas []string
}

func (o *opaqueBlock) assemble(final bool) api.Content {
	return api.ContentOpaque{
		Model:  o.model,
		Kind:   o.kind,
		Data:   o.data,
		Deltas: o.deltas,
	}
}

func (o *opaqueBlock) append(e api.IndexedEvent) error {
	if o.closed {
		return closedErr("opaque", e)
	}
	switch e := e.(type) {
	case api.EventOpaque:
		if e.Kind != "" {
			o.kind = e.Kind
		}
		if len(e.Data) != 0 {
			o.data = e.Data
		}
		if e.Delta != "" {
			o.deltas = append(o.deltas, e.Delta)
		}
	case api.EventClose:
		o.closed = true
	default:
		return mismatchErr("opaque", e)
	}
	return nil
}

func partsText(parts map[int]*strings.Builder) map[int]string {
	if len(parts) == 0 {
		return nil
	}
	text := make(map[int]string, len(parts))
	for key := range parts {
		text[key] = parts[key].String()
	}
	return text
}
