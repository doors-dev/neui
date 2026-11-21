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

package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const doneSentinel = "[DONE]"

var (
	metaReasoning        = json.RawMessage(`{"provider":"compat","data":"reasoning"}`)
	metaReasoningContent = json.RawMessage(`{"provider":"compat","data":"reasoning_content"}`)
)

type converter struct {
	model     string
	id        string
	started   bool
	next      int
	text      int
	reasoning int
	tools     map[int]int
	ids       map[string]bool
	real      map[int]string
	stopped   bool
	finish    string
	failure   api.Event
	sent      int
	declared  int
	k         chunk
	events    []api.Event
}

func newConverter(model string, sent int, declared int) *converter {
	return &converter{
		model:     model,
		text:      -1,
		reasoning: -1,
		tools:     make(map[int]int),
		ids:       make(map[string]bool),
		real:      make(map[int]string),
		sent:      sent,
		declared:  declared,
	}
}

func (c *converter) Convert(f lib.Frame) ([]api.Event, bool) {
	if string(f.Data) == doneSentinel {
		if c.failure != nil {
			return []api.Event{c.failure}, false
		}
		if !c.stopped {
			return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: errors.New("compat: [DONE] without a finish_reason")}}, false
		}
		code := api.CodeUnknown
		switch c.finish {
		case "stop", "tool_calls", "function_call":
			return []api.Event{api.EventOk{Code: api.CodeTurn}}, false
		case "length":
			code = api.CodeCapped
		case "content_filter":
			code = api.CodeRefused
		case "abort", "error":
			code = api.CodeNetwork
		}
		return []api.Event{api.EventErr{Code: code, Err: &APIError{Provider: providerName, Code: c.finish, RequestID: c.id, Message: "the turn ended on this finish_reason"}}}, false
	}
	c.k = chunk{}
	c.events = c.events[:0]
	if err := json.Unmarshal(f.Data, &c.k); err != nil {
		if c.failure != nil {
			return []api.Event{c.failure}, false
		}
		return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: fmt.Errorf("compat: stream frame did not parse: %w", err)}}, false
	}
	if !c.started {
		c.started = true
		c.id = c.k.ID
		c.events = append(c.events, api.EventStart{ID: c.k.ID, Model: c.model})
	}
	for i := range c.k.Choices {
		ch := &c.k.Choices[i]
		if ch.Index != 0 || c.failure != nil {
			continue
		}
		d := &ch.Delta
		thought, meta := d.Reasoning, metaReasoning
		if thought == "" {
			thought, meta = d.ReasoningContent, metaReasoningContent
		}
		if thought != "" {
			if c.reasoning < 0 {
				c.reasoning = c.next
				c.next++
				c.events = append(c.events, api.EventThinking{Index: c.reasoning, Text: thought}, api.EventMeta{Index: c.reasoning, Data: meta})
			} else {
				c.events = append(c.events, api.EventThinking{Index: c.reasoning, Text: thought})
			}
		}
		if d.Content != "" {
			if c.text < 0 {
				c.text = c.next
				c.next++
			}
			c.events = append(c.events, api.EventText{Index: c.text, Text: d.Content})
		}
		for j := range d.ToolCalls {
			t := &d.ToolCalls[j]
			if t.Index == nil {
				return append(c.events, api.EventErr{Code: api.CodeNetwork, Err: errors.New("compat: tool call fragment without an index")}), false
			}
			e := api.EventTool{}
			index, seen := c.tools[*t.Index]
			switch {
			case !seen:
				index = c.next
				c.next++
				c.tools[*t.Index] = index
				c.real[*t.Index] = t.ID
				e.ID = t.ID
				if e.ID == "" || c.ids[e.ID] {
					base := c.id
					if base == "" {
						base = providerName
					}
					e.ID = fmt.Sprintf("%s_call_%d", base, index)
				}
				c.ids[e.ID] = true
			case t.ID == "" || t.ID == c.real[*t.Index]:
			case c.real[*t.Index] != "":
				return append(c.events, api.EventErr{Code: api.CodeNetwork, Err: fmt.Errorf("compat: tool call %d named %q after %q", *t.Index, t.ID, c.real[*t.Index])}), false
			default:
				c.real[*t.Index] = t.ID
				if !c.ids[t.ID] {
					c.ids[t.ID] = true
					e.ID = t.ID
				}
			}
			e.Index = index
			if t.Function != nil {
				e.Name, e.Args = t.Function.Name, t.Function.Arguments
			}
			if e.ID != "" || e.Name != "" || e.Args != "" {
				c.events = append(c.events, e)
			}
		}
		if ch.FinishReason != nil && *ch.FinishReason != "" {
			c.stopped = true
			c.finish = *ch.FinishReason
		}
	}
	if u := c.k.Usage; u != nil {
		var cached, written, reasoning int
		if u.PromptDetails != nil {
			cached, written = max(u.PromptDetails.Cached, 0), max(u.PromptDetails.CacheWrite, 0)
		}
		if u.CompletionDetails != nil {
			reasoning = max(min(u.CompletionDetails.Reasoning, u.Completion), 0)
		}
		c.events = append(c.events, api.EventUsage{
			Input:      max(u.Prompt-cached-written, 0),
			Output:     max(u.Completion-reasoning, 0),
			Reasoning:  reasoning,
			CacheRead:  cached,
			CacheWrite: written,
		})
	}
	if c.failure == nil && len(c.k.Error) > 0 && string(c.k.Error) != "null" {
		e := &APIError{Provider: providerName, RequestID: c.id}
		status := readError(e, c.k.Error)
		if e.Message == "" {
			e.Message = lib.ErrorMessage(c.k.Error)
		}
		c.failure = api.EventErr{Code: errorCode(status, e), Err: e}
	}
	return c.events, true
}

func (c *converter) End(err error) []api.Event {
	if h, ok := err.(*lib.HTTPError); ok {
		e := &APIError{Provider: providerName, Status: h.Status, RetryAfter: lib.RetryAfter(h.Header), RequestID: h.Header.Get("X-Request-Id")}
		if e.RequestID == "" {
			e.RequestID = h.Header.Get("X-Generation-Id")
		}
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(h.Body, &envelope) == nil && len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			readError(e, envelope.Error)
		} else {
			readError(e, h.Body)
		}
		if e.Message == "" {
			e.Message = lib.ErrorMessage(h.Body)
		}
		code := errorCode(h.Status, e)
		if code == api.CodeTooLarge {
			return []api.Event{api.EventErr{Code: code, Err: lib.MeasuredTooLarge(c.model, c.sent, c.declared, e)}}
		}
		return []api.Event{api.EventErr{Code: code, Err: e}}
	}
	switch {
	case err == context.Canceled, err == context.DeadlineExceeded:
		return nil
	case c.failure != nil:
		return []api.Event{c.failure}
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return nil
	}
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}
