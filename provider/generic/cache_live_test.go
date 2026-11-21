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

//go:build live

package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/internal/thread/response"
)

type streamTee struct {
	n int
}

func (s *streamTee) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	s.n++
	if err != nil || resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return resp, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, os.WriteFile(filepath.Join("testdata", "streams", fmt.Sprintf("openrouter-qwen-cache-%d.sse", s.n)), data, 0o644)
}

func TestLiveCacheRead(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}
	client := &http.Client{}
	if f := flag.Lookup("record"); f != nil && f.Value.String() == "true" {
		client.Transport = &streamTee{}
	}
	p, err := New(Conf{Model: api.Model{Name: "qwen/qwen3.8-27b"}, BaseURL: "https://openrouter.ai/api/v1"},
		WithAPIKey(key), WithHTTPClient(client), WithEffort(EffortNone), WithMaxTokens(16),
		WithExtraBody(map[string]any{"provider": map[string]any{"order": []string{"wafer"}, "allow_fallbacks": false}}))
	if err != nil {
		t.Fatal(err)
	}
	var system strings.Builder
	fmt.Fprintf(&system, "Ledger run %d.\n", time.Now().UnixNano())
	for i := range 360 {
		fmt.Fprintf(&system, "Record %04d: the ledger keeps entry %d of %d in its original order.\n", i, i*7, 360*7)
	}
	req := api.Request{System: system.String(), Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "Reply with the single word ok."}}}}}
	var usage [2]api.EventUsage
	for i := range usage {
		if i > 0 {
			time.Sleep(5 * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		var events []api.Event
		for e := range p.Stream(ctx, req) {
			events = append(events, e)
			if u, ok := e.(api.EventUsage); ok {
				usage[i] = u
			}
		}
		cancel()
		if len(events) == 0 {
			t.Fatalf("request %d: the stream closed with no event", i+1)
		}
		if _, ok := events[len(events)-1].(api.EventOk); !ok {
			t.Fatalf("request %d: %v", i+1, conform.DescribeAll(events))
		}
		t.Logf("request %d: %s", i+1, conform.Describe(usage[i]))
	}
	if prompt := usage[0].Input + usage[0].CacheRead + usage[0].CacheWrite; prompt < 8_000 || prompt > 14_000 {
		t.Fatalf("the first request took %d prompt tokens, want about 10k", prompt)
	}
	if usage[1].CacheRead == 0 {
		t.Fatalf("the second identical request read nothing from the cache: %s", conform.Describe(usage[1]))
	}
}

type bodyTee struct {
	bodies []string
}

func (b *bodyTee) RoundTrip(r *http.Request) (*http.Response, error) {
	data, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	b.bodies = append(b.bodies, string(data))
	r.Body = io.NopCloser(bytes.NewReader(data))
	return http.DefaultTransport.RoundTrip(r)
}

func liveToolLoop(t *testing.T, model string, system string, opts ...Option) ([2]api.EventUsage, api.Message, []string) {
	t.Helper()
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}
	tee := &bodyTee{}
	p, err := New(Conf{Model: api.Model{Name: model}, BaseURL: "https://openrouter.ai/api/v1"},
		append([]Option{WithAPIKey(key), WithHTTPClient(&http.Client{Transport: tee}), WithEffort(EffortLow), WithMaxTokens(2048)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	tools := []api.Tool{{Name: "lookup", Desc: "Returns the code of an item.", Schema: map[string]any{"type": "object", "properties": map[string]any{"item": map[string]any{"type": "string"}}, "required": []string{"item"}}}}
	msgs := []api.Message{
		user(text("What is the code of item beta?")),
		assistant(api.ContentToolUse{ID: "call_beta", Name: "lookup", Input: json.RawMessage(`{"item":"beta"}`)}),
		user(api.ContentToolResult{ID: "call_beta", Content: api.UserContents{text("beta: 23. Ledger note: codes are stable for the day; every lookup is logged with the caller, the time and the item, and the ledger keeps the entries in their original order.")}}),
		assistant(text("The code of item beta is 23.")),
		user(text("Now look up the code of item alpha, add the code of beta to it, and reply with only the sum.")),
	}
	var usage [2]api.EventUsage
	var reply api.Message
	for i := range usage {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		b := response.NewBulder()
		var events []api.Event
		for e := range p.Stream(ctx, api.Request{System: system, Tools: tools, Messages: msgs}) {
			events = append(events, e)
			if u, ok := e.(api.EventUsage); ok {
				usage[i] = u
			}
			if _, ok := b.On(e); !ok {
				t.Fatalf("builder refused %s", conform.Describe(e))
			}
		}
		cancel()
		if len(events) == 0 {
			t.Fatalf("request %d: the stream closed with no event", i+1)
		}
		if _, ok := events[len(events)-1].(api.EventOk); !ok {
			t.Fatalf("request %d: %v", i+1, conform.DescribeAll(events))
		}
		t.Logf("request %d: %s", i+1, conform.Describe(usage[i]))
		if i > 0 {
			for _, c := range reply.Content {
				o, ok := c.(api.ContentOpaque)
				if !ok {
					continue
				}
				var fields struct {
					Fields map[string]json.RawMessage `json:"fields"`
				}
				if err := json.Unmarshal(o.Data, &fields); err != nil {
					t.Fatal(err)
				}
				for name, value := range fields.Fields {
					if !strings.Contains(tee.bodies[1], fmt.Sprintf("%q:%s", name, value)) {
						t.Fatalf("the second request does not carry %s unchanged:\n%s", name, tee.bodies[1])
					}
				}
				t.Logf("fields sent back: %s", strings.Join(slices.Sorted(maps.Keys(fields.Fields)), ", "))
			}
			break
		}
		reply = assistant(b.Flush().Content.Ordered()...)
		var results api.Contents
		for _, c := range reply.Content {
			if u, ok := c.(api.ContentToolUse); ok {
				results = append(results, api.ContentToolResult{ID: u.ID, Content: api.UserContents{text("alpha: 58")}})
			}
		}
		if len(results) == 0 {
			t.Fatalf("the first turn called no tool: %v", conform.DescribeAll(events))
		}
		msgs = append(msgs, reply, user(results...))
	}
	return usage, reply, tee.bodies
}

func TestLiveCacheMarker(t *testing.T) {
	var system strings.Builder
	fmt.Fprintf(&system, "Item codes are only known through the lookup tool; never guess them. Ledger run %d.\n", time.Now().UnixNano())
	for i := range 150 {
		fmt.Fprintf(&system, "Record %04d: the ledger keeps entry %d of %d in its original order.\n", i, i*7, 150*7)
	}
	usage, _, bodies := liveToolLoop(t, "anthropic/claude-haiku-5.5", system.String(), WithCacheMarker(map[string]any{"cache_control": map[string]any{"type": "ephemeral"}}))
	if strings.Count(bodies[0], `"cache_control"`) != 2 || strings.Count(bodies[1], `"cache_control"`) != 2 {
		t.Fatalf("the bodies do not carry two markers each:\n%s\n%s", bodies[0], bodies[1])
	}
	if usage[0].CacheWrite == 0 {
		t.Fatalf("the first request wrote nothing to the cache: %s", conform.Describe(usage[0]))
	}
	if usage[1].CacheRead < usage[0].CacheWrite {
		t.Fatalf("the second request read %d tokens from the cache, less than the %d the first wrote", usage[1].CacheRead, usage[0].CacheWrite)
	}
}

func TestLiveOpaqueFields(t *testing.T) {
	for _, model := range []string{"anthropic/claude-haiku-5.5", "openai/gpt-5-mini", "google/gemini-3.5-flash-lite", "qwen/qwen3.8-27b"} {
		t.Run(strings.ReplaceAll(model, "/", "_"), func(t *testing.T) {
			_, reply, _ := liveToolLoop(t, model, "Item codes are only known through the lookup tool; never guess them.")
			kept := false
			for _, c := range reply.Content {
				o, ok := c.(api.ContentOpaque)
				kept = kept || ok && o.Kind == "generic.fields" && o.Model == model
			}
			if !kept {
				t.Fatalf("the first turn kept no fields: %+v", reply.Content)
			}
		})
	}
}
