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

package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const (
	kindThought = iota
	kindOutput
	kindCall
	kindOpaque
	kindIgnore
)

var kindNames = [...]string{"thought", "model_output", "function_call", "opaque", "ignored"}

type step struct {
	kind    int
	idx     int
	sawArgs bool
	args    json.RawMessage
	data    map[string]json.RawMessage
}

type converter struct {
	model    string
	base     string
	sent     int
	declared int
	started  bool
	id       string
	steps    map[int]*step
	next     int
	seen     map[string]bool
	usage    api.EventUsage
	hasUsage bool
	events   []api.Event
	head     frameHead
	typ      typeHead
	ev       streamEvent
	body     stepBody
	delta    deltaBody
}

func newConverter(model, base string, sent, declared int) *converter {
	return &converter{model: model, base: base, sent: sent, declared: declared, steps: map[int]*step{}, seen: map[string]bool{}}
}

func (c *converter) emit(e api.Event) {
	c.events = append(c.events, e)
}

func (c *converter) take() int {
	i := c.next
	c.next++
	return i
}

func (c *converter) fail(err error) ([]api.Event, bool) {
	c.emit(api.EventErr{Code: api.CodeNetwork, Err: err})
	return c.events, false
}

func (c *converter) Convert(f lib.Frame) ([]api.Event, bool) {
	c.events = c.events[:0]
	if string(f.Data) == "[DONE]" {
		return c.fail(errors.New("gemini: the stream ended without interaction.completed"))
	}
	c.head = frameHead{}
	if err := json.Unmarshal(f.Data, &c.head); err != nil {
		return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
	}
	if c.head.EventType == "" {
		return c.fail(fmt.Errorf("gemini: %q frame without an event_type", f.Event))
	}
	if !c.started && c.head.EventType != "error" && c.head.EventType != "interaction.created" {
		return c.fail(fmt.Errorf("gemini: the stream opened with %q, not interaction.created", c.head.EventType))
	}
	switch c.head.EventType {
	case "error", "interaction.created", "interaction.status_update", "interaction.completed", "step.start", "step.delta", "step.stop":
		c.ev = streamEvent{}
		if err := json.Unmarshal(f.Data, &c.ev); err != nil {
			return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
		}
	default:
		return nil, true
	}
	ev := &c.ev
	switch ev.EventType {
	case "error":
		c.emit(c.inBandError(ev.Error))
		return c.events, false
	case "interaction.created":
		if c.started {
			return c.fail(errors.New("gemini: a second interaction.created"))
		}
		if ev.Interaction == nil {
			return c.fail(errors.New("gemini: interaction.created without an interaction"))
		}
		c.started = true
		c.id = ev.Interaction.ID
		c.emit(api.EventStart{ID: c.id, Model: c.model})
		c.readUsage(ev.Interaction.Usage)
		return c.events, true
	case "interaction.status_update":
		c.readUsage(ev.Usage)
		return c.events, true
	case "interaction.completed":
		if ev.Interaction == nil {
			return c.fail(errors.New("gemini: interaction.completed without an interaction"))
		}
		c.readUsage(ev.Interaction.Usage)
		c.emit(c.endFor(ev.Interaction.Status))
		return c.events, false
	}
	if ev.Index == nil {
		return c.fail(fmt.Errorf("gemini: %s without an index", ev.EventType))
	}
	at := *ev.Index
	s := c.steps[at]
	switch ev.EventType {
	case "step.start":
		if s != nil {
			return c.fail(fmt.Errorf("gemini: step.start for open step %d", at))
		}
		c.typ = typeHead{}
		if err := json.Unmarshal(ev.Step, &c.typ); err != nil || c.typ.Type == "" {
			return c.fail(fmt.Errorf("gemini: step.start %d without a step type", at))
		}
		s = &step{idx: -1}
		c.steps[at] = s
		switch c.typ.Type {
		case "thought", "model_output", "function_call":
			c.body = stepBody{}
			if err := json.Unmarshal(ev.Step, &c.body); err != nil {
				return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
			}
		}
		switch c.typ.Type {
		case "thought":
			s.kind = kindThought
			for _, part := range c.body.Summary {
				if part.Type == "text" && part.Text != "" {
					if s.idx < 0 {
						s.idx = c.take()
					}
					c.emit(api.EventSummary{Index: s.idx, Text: part.Text})
				}
			}
			if c.body.Signature != "" {
				c.signature(s, c.body.Signature)
			}
		case "model_output":
			s.kind = kindOutput
			for _, part := range c.body.Content {
				if part.Type == "text" && part.Text != "" {
					if s.idx < 0 {
						s.idx = c.take()
					}
					c.emit(api.EventText{Index: s.idx, Text: part.Text})
				}
			}
		case "function_call":
			if a := bytes.TrimSpace(c.body.Arguments); len(a) > 0 && a[0] != '{' && string(a) != "null" {
				return c.fail(fmt.Errorf("gemini: step.start %d with arguments that are not an object", at))
			}
			s.kind = kindCall
			s.idx = c.take()
			id := c.body.ID
			if id == "" || c.seen[id] {
				id = fmt.Sprintf("%s_call_%d", c.base, s.idx)
			}
			c.seen[id] = true
			s.args = c.body.Arguments
			c.emit(api.EventTool{Index: s.idx, ID: id, Name: c.body.Name})
		case "user_input", "function_result":
			s.kind = kindIgnore
		default:
			s.kind = kindOpaque
			if err := json.Unmarshal(ev.Step, &s.data); err != nil {
				return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
			}
			s.idx = c.take()
			c.emit(api.EventOpaque{Index: s.idx, Kind: opaquePrefix + c.typ.Type, Data: bytes.Clone(ev.Step)})
		}
	case "step.delta":
		if s == nil {
			return c.fail(fmt.Errorf("gemini: step.delta for step %d, which never started", at))
		}
		c.typ = typeHead{}
		if err := json.Unmarshal(ev.Delta, &c.typ); err != nil || c.typ.Type == "" {
			return c.fail(fmt.Errorf("gemini: step.delta %d without a delta type", at))
		}
		switch s.kind {
		case kindIgnore:
			return c.events, true
		case kindOpaque:
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(ev.Delta, &fields); err != nil {
				return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
			}
			for k, v := range fields {
				if k != "type" {
					s.data[k] = v
				}
			}
			data, err := lib.MarshalRaw(s.data)
			if err != nil {
				return c.fail(fmt.Errorf("gemini: assembling step %d: %w", at, err))
			}
			c.emit(api.EventOpaque{Index: s.idx, Data: data})
			return c.events, true
		}
		var want int
		switch c.typ.Type {
		case "text":
			want = kindOutput
		case "thought_summary", "thought_signature":
			want = kindThought
		case "arguments_delta":
			want = kindCall
		default:
			return c.events, true
		}
		if s.kind != want {
			return c.fail(fmt.Errorf("gemini: %s delta on a %s step at %d", c.typ.Type, kindNames[s.kind], at))
		}
		c.delta = deltaBody{}
		if err := json.Unmarshal(ev.Delta, &c.delta); err != nil {
			return c.fail(fmt.Errorf("gemini: stream frame did not parse: %w", err))
		}
		d := &c.delta
		switch d.Type {
		case "text":
			if d.Text != "" {
				if s.idx < 0 {
					s.idx = c.take()
				}
				c.emit(api.EventText{Index: s.idx, Text: d.Text})
			}
		case "thought_summary":
			if d.Content != nil && d.Content.Type == "text" && d.Content.Text != "" {
				if s.idx < 0 {
					s.idx = c.take()
				}
				c.emit(api.EventSummary{Index: s.idx, Text: d.Content.Text})
			}
		case "thought_signature":
			if d.Signature != "" {
				c.signature(s, d.Signature)
			}
		case "arguments_delta":
			if d.Arguments != "" {
				s.sawArgs = true
				c.emit(api.EventTool{Index: s.idx, Args: d.Arguments})
			}
		}
	case "step.stop":
		if s == nil {
			return c.fail(fmt.Errorf("gemini: step.stop for step %d, which never started", at))
		}
		delete(c.steps, at)
		switch s.kind {
		case kindCall:
			if !s.sawArgs {
				args := "{}"
				if trimmed := bytes.TrimSpace(s.args); len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed) && !bytes.Equal(trimmed, emptyObject) {
					args = string(trimmed)
				}
				c.emit(api.EventTool{Index: s.idx, Args: args})
			}
			c.emit(api.EventClose{Index: s.idx})
		case kindOpaque:
			c.emit(api.EventClose{Index: s.idx})
		case kindThought, kindOutput:
			if s.idx >= 0 {
				c.emit(api.EventClose{Index: s.idx})
			}
		}
	}
	return c.events, true
}

func (c *converter) signature(s *step, sig string) {
	if s.idx < 0 {
		s.idx = c.take()
		c.emit(api.EventThinking{Index: s.idx})
	}
	meta, _ := lib.MarshalRaw(thought{Type: "thought", Signature: sig})
	c.emit(api.EventMeta{Index: s.idx, Data: lib.Meta(providerName, meta)})
}

func (c *converter) readUsage(u *usage) {
	if u == nil {
		return
	}
	e := api.EventUsage{
		Input:     max(0, u.TotalInputTokens+u.TotalToolUseTokens-u.TotalCachedTokens),
		CacheRead: max(0, u.TotalCachedTokens),
		Output:    max(0, u.TotalOutputTokens),
		Reasoning: max(0, u.TotalThoughtTokens),
	}
	if c.hasUsage && e == c.usage {
		return
	}
	c.usage, c.hasUsage = e, true
	c.emit(e)
}

func (c *converter) endFor(status string) api.Event {
	ended := func(code api.ErrCode) api.Event {
		return api.EventErr{Code: code, Err: &APIError{Provider: providerName, Code: status, Message: "the turn ended on this status"}}
	}
	switch status {
	case "completed", "requires_action":
		if len(c.steps) > 0 {
			return api.EventErr{Code: api.CodeNetwork, Err: fmt.Errorf("gemini: interaction.completed with %d steps still open", len(c.steps))}
		}
		return api.EventOk{Code: api.CodeTurn}
	case "incomplete", "requires_continuation":
		return ended(api.CodeCapped)
	case "":
		return api.EventErr{Code: api.CodeNetwork, Err: errors.New("gemini: interaction.completed without a status")}
	}
	return ended(api.CodeUnknown)
}

func (c *converter) End(err error) []api.Event {
	if h, ok := err.(*lib.HTTPError); ok {
		return []api.Event{c.httpError(h)}
	}
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return nil
	case err == context.Canceled, err == context.DeadlineExceeded:
		return nil
	}
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}
