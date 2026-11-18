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

package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"neui.dev/api"
)

const compactionMarker = "the conversation above is being compacted"

type Turn func(ctx context.Context, req api.Request, out chan<- api.Event)

type Provider struct {
	mu          sync.Mutex
	model       api.Model
	turns       []Turn
	compactions []Turn
	requests    []api.Request
}

func NewProvider(model api.Model, turns ...Turn) *Provider {
	if reflect.ValueOf(model).IsZero() {
		model = api.Model{Name: "fake", Context: 1 << 30, RequestBytes: 1 << 40}
	}
	return &Provider{model: model, turns: turns}
}

func (p *Provider) Name() string { return "fake" }

func (p *Provider) Model() api.Model { return p.model }

func (p *Provider) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	queue := &p.turns
	what := "turn"
	if IsCompaction(req) {
		queue = &p.compactions
		what = "compaction turn"
	}
	turn := Events(api.EventErr{Code: api.CodeUnknown, Err: fmt.Errorf("fake provider: no %s %d", what, len(p.requests))})
	if len(*queue) > 0 {
		turn = (*queue)[0]
		*queue = (*queue)[1:]
	}
	p.mu.Unlock()
	out := make(chan api.Event)
	go func() {
		defer close(out)
		turn(ctx, req, out)
	}()
	return out
}

func (p *Provider) Push(turns ...Turn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.turns = append(p.turns, turns...)
}

func (p *Provider) OnCompaction(turns ...Turn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.compactions = append(p.compactions, turns...)
}

func (p *Provider) Requests() []api.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]api.Request(nil), p.requests...)
}

func (p *Provider) Request(t *testing.T, i int) api.Request {
	t.Helper()
	requests := p.Requests()
	if i >= len(requests) {
		t.Fatalf("fake provider: request %d asked, %d made", i, len(requests))
	}
	return requests[i]
}

func Say(text string) Turn {
	return Events(
		api.EventStart{ID: "say", Model: "fake"},
		api.EventText{Index: 0, Text: text},
		api.EventClose{Index: 0},
		api.EventUsage{Input: 1, Output: 1},
		api.EventOk{Code: api.CodeTurn},
	)
}

func Call(name string, args any) Turn {
	data, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return Events(
		api.EventStart{ID: "call", Model: "fake"},
		api.EventTool{Index: 0, ID: "call-" + name, Name: name, Args: string(data)},
		api.EventClose{Index: 0},
		api.EventUsage{Input: 1, Output: 1},
		api.EventOk{Code: api.CodeTurn},
	)
}

func Events(events ...api.Event) Turn {
	return func(ctx context.Context, req api.Request, out chan<- api.Event) {
		for _, e := range events {
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
		}
	}
}

func Hang(events ...api.Event) Turn {
	return func(ctx context.Context, req api.Request, out chan<- api.Event) {
		Events(events...)(ctx, req, out)
		<-ctx.Done()
	}
}

func Then(f func(req api.Request) Turn) Turn {
	return func(ctx context.Context, req api.Request, out chan<- api.Event) {
		f(req)(ctx, req, out)
	}
}

func Results(req api.Request) []api.ContentToolResult {
	if len(req.Messages) == 0 {
		return nil
	}
	var results []api.ContentToolResult
	for _, c := range req.Messages[len(req.Messages)-1].Content {
		if r, ok := c.(api.ContentToolResult); ok {
			results = append(results, r)
		}
	}
	return results
}

func Decode(t *testing.T, r api.ContentToolResult, v any) {
	t.Helper()
	for _, c := range r.Content {
		if text, ok := c.(api.ContentText); ok {
			if err := json.Unmarshal([]byte(text.Text), v); err != nil {
				t.Fatalf("tool result %s: %v in %q", r.ID, err, text.Text)
			}
			return
		}
	}
	t.Fatalf("tool result %s carries no text: %#v", r.ID, r.Content)
}

func IsCompaction(req api.Request) bool {
	if len(req.Messages) == 0 {
		return false
	}
	content := req.Messages[len(req.Messages)-1].Content
	if len(content) == 0 {
		return false
	}
	text, ok := content[len(content)-1].(api.ContentText)
	return ok && strings.Contains(text.Text, compactionMarker)
}
