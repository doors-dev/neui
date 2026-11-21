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

package generic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

const testModel = "qwen/qwen3.8-27b"

func frame(payload string) string {
	return "data: " + payload + "\n\n"
}

func chunkFrame(delta string, finish string) string {
	reason := "null"
	if finish != "" {
		reason = `"` + finish + `"`
	}
	return frame(fmt.Sprintf(`{"id":"gen-1","object":"chat.completion.chunk","created":1795183087,"model":"qwen/qwen3.8-27b","provider":"Wafer",`+
		`"choices":[{"index":0,"delta":%s,"finish_reason":%s,"native_finish_reason":%s}]}`, delta, reason, reason))
}

const (
	openingFrame = `{"role":"assistant","content":""}`

	usageFrame = `data: {"id":"gen-1","object":"chat.completion.chunk","created":1795183087,"model":"qwen/qwen3.8-27b","provider":"Wafer",` +
		`"choices":[{"index":0,"delta":{"content":"","role":"assistant"},"finish_reason":"stop","native_finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":440,"completion_tokens":60,"total_tokens":500,"cost":0.0001,"is_byok":false,` +
		`"prompt_tokens_details":{"cached_tokens":64,"cache_write_tokens":0},"completion_tokens_details":{"reasoning_tokens":15}}}` + "\n\n"

	doneFrame = "data: [DONE]\n\n"
)

func serve(t *testing.T, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: srv.URL}, append([]Option{WithAPIKey("test-key"), WithHTTPClient(srv.Client())}, opts...)...)
}

func mustNew(t *testing.T, conf Conf, opts ...Option) *Provider {
	t.Helper()
	p, err := New(conf, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func stringHandler(frames ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flush(w)
		for _, f := range frames {
			io.WriteString(w, f)
			flush(w)
		}
	}
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func drop(w http.ResponseWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	conn.Close()
}

func collect(t *testing.T, p *Provider, req api.Request) []api.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var events []api.Event
	for e := range p.Stream(ctx, req) {
		events = append(events, e)
	}
	if ctx.Err() != nil {
		t.Fatalf("stream did not close within 10s: %v", conform.DescribeAll(events))
	}
	return events
}

func turn(text string) api.Request {
	return api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: text}}}}}
}

func wantEvents(t *testing.T, events []api.Event, want ...string) {
	t.Helper()
	got := conform.DescribeAll(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence:\n got %q\nwant %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] && !(strings.HasPrefix(want[i], "err ") && strings.HasPrefix(got[i], want[i]+" ")) {
			t.Fatalf("event %d: got %q, want %q\n got %q\nwant %q", i, got[i], want[i], got, want)
		}
	}
}

func TestModel(t *testing.T) {
	m := api.Model{Name: testModel, Context: 1_024_000, ImagesPerRequest: 8}
	if got := mustNew(t, Conf{Model: m, BaseURL: "http://127.0.0.1:8080/v1"}).Model(); !reflect.DeepEqual(got, m) {
		t.Fatalf("Model %+v, want %+v", got, m)
	}
	if got := mustNew(t, Conf{Model: m, BaseURL: "http://127.0.0.1:8080/v1"}).Name(); got != "generic" {
		t.Fatalf("Name %q", got)
	}
}

func TestNewRefuses(t *testing.T) {
	model := api.Model{Name: testModel}
	base := "http://127.0.0.1:8080/v1"
	tests := []struct {
		name string
		conf Conf
		opts []Option
		want string
	}{
		{"no model", Conf{BaseURL: base}, nil, "no model"},
		{"no base URL", Conf{Model: model}, nil, "no base URL"},
		{"a base URL without a scheme", Conf{Model: model, BaseURL: "127.0.0.1:8080/v1"}, nil, "base URL"},
		{"a base URL without a host", Conf{Model: model, BaseURL: "http:///v1"}, nil, "no scheme or host"},
		{"a base URL that does not parse", Conf{Model: model, BaseURL: "http://[::1"}, nil, "base URL"},
		{"negative max tokens", Conf{Model: model, BaseURL: base}, []Option{WithMaxTokens(-1)}, "negative"},
		{"temperature below range", Conf{Model: model, BaseURL: base}, []Option{WithTemperature(-0.1)}, "range"},
		{"temperature above range", Conf{Model: model, BaseURL: base}, []Option{WithTemperature(2.1)}, "range"},
		{"temperature NaN", Conf{Model: model, BaseURL: base}, []Option{WithTemperature(math.NaN())}, "range"},
		{"top_p of zero", Conf{Model: model, BaseURL: base}, []Option{WithTopP(0)}, "range"},
		{"top_p above one", Conf{Model: model, BaseURL: base}, []Option{WithTopP(1.1)}, "range"},
		{"top_p NaN", Conf{Model: model, BaseURL: base}, []Option{WithTopP(math.NaN())}, "range"},
		{"a nil client", Conf{Model: model, BaseURL: base}, []Option{WithHTTPClient(nil)}, "client"},
		{"an extra field with no name", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"": 1})}, "no name"},
		{"an extra model", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"model": "other"})}, `"model", set Conf.Model`},
		{"an extra max_tokens", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"max_tokens": 10})}, `"max_tokens", use WithMaxTokens`},
		{"extra messages", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"messages": []any{}})}, `"messages"`},
		{"extra stream options", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"stream_options": map[string]any{"continuous_usage_stats": true}})}, `"stream_options"`},
		{"an extra value that does not encode", Conf{Model: model, BaseURL: base}, []Option{WithExtraBody(map[string]any{"cache_prompt": true, "hook": func() {}})}, `"hook"`},
		{"a marker field with no name", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"": 1})}, "no name"},
		{"a marker type", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"type": "x"})}, `"type"`},
		{"a marker text", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"text": "x"})}, `"text"`},
		{"a marker image_url", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"image_url": "x"})}, `"image_url"`},
		{"a marker function", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"function": "x"})}, `"function"`},
		{"a marker value that does not encode", Conf{Model: model, BaseURL: base}, []Option{WithCacheMarker(map[string]any{"cache_control": func() {}})}, `cache marker field "cache_control"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.conf, tc.opts...)
			if err == nil || p != nil {
				t.Fatalf("New returned %v, %v; want an error", p, err)
			}
			if !strings.HasPrefix(err.Error(), "generic: ") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
	p := mustNew(t, Conf{Model: model, BaseURL: base + "/"}, nil, WithEffort("whatever"), WithTemperature(0), WithTopP(1), WithMaxTokens(0), WithStopSequences("a"), WithStopSequences("b", "c"))
	if p.base != base || p.effort != "whatever" || *p.temperature != 0 || *p.topP != 1 || strings.Join(p.stop, ",") != "b,c" {
		t.Fatalf("options not kept as given: %+v", p)
	}
}

func TestRequestWire(t *testing.T) {
	var path, auth, accept, contentType string
	var authSent bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		path, accept, contentType = r.URL.Path, r.Header.Get("Accept"), r.Header.Get("Content-Type")
		auth, authSent = r.Header.Get("Authorization"), r.Header["Authorization"] != nil
		stringHandler(chunkFrame(`{"content":"hi"}`, "stop"), doneFrame)(w, r)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()
	collect(t, mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: srv.URL + "/api/v1/"}, WithAPIKey("sk-1")), turn("hi"))
	if path != "/api/v1/chat/completions" || auth != "Bearer sk-1" || accept != "text/event-stream" || contentType != "application/json" {
		t.Fatalf("path %q auth %q accept %q content type %q", path, auth, accept, contentType)
	}
	for _, opts := range [][]Option{nil, {WithAPIKey("")}} {
		collect(t, mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: srv.URL}, opts...), turn("hi"))
		if authSent {
			t.Fatalf("Authorization %q sent with no key", auth)
		}
	}
}

func TestProviderRefusesBytesMeasuredAsFitting(t *testing.T) {
	const declared = 1 << 20
	var sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent = len(body)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, "payload too large")
	}))
	defer srv.Close()
	p := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: declared}, BaseURL: srv.URL})
	events := collect(t, p, turn("hi"))
	wantEvents(t, events, "err too_large")
	e := events[0].(api.EventErr)
	if msg := e.Err.Error(); sent == 0 || !strings.Contains(msg, fmt.Sprint(sent)) || !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q names neither the %d bytes sent nor the %d declared", msg, sent, declared)
	}
	var carried *APIError
	if !errors.As(e.Err, &carried) || carried.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error %v lost what the server answered", e.Err)
	}
}

func TestUnmeasuredRequestTooLargeNamesNoLimit(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, "payload too large")
	})
	events := collect(t, p, turn("hi"))
	wantEvents(t, events, "err too_large")
	if msg := events[0].(api.EventErr).Err.Error(); strings.Contains(msg, "the 0 declared") {
		t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
	}
}

func TestRequestOverTheDeclaredBytesIsNotSent(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	p := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: 50}, BaseURL: srv.URL})
	events := collect(t, p, turn(strings.Repeat("a", 100)))
	wantEvents(t, events, "err too_large")
	_, fits, err := mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: srv.URL}).request(context.Background(), turn("hi"))
	if err != nil {
		t.Fatal(err)
	}
	p = mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: fits}, BaseURL: srv.URL}, WithExtraBody(map[string]any{"cache_prompt": true}))
	events = collect(t, p, turn("hi"))
	wantEvents(t, events, "err too_large")
	if msg := events[0].(api.EventErr).Err.Error(); !strings.Contains(msg, fmt.Sprint(fits+len(`,"cache_prompt":true`))) {
		t.Fatalf("error %q does not count the extra fields", msg)
	}
	if hits != 0 {
		t.Fatalf("%d requests reached the server", hits)
	}
}

func capture(t *testing.T, bodies *[]string, opts ...Option) *Provider {
	t.Helper()
	return serve(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(body))
		stringHandler(chunkFrame(`{"content":"ok"}`, "stop"), doneFrame)(w, r)
	}, opts...)
}

func TestExtraBody(t *testing.T) {
	routing := map[string]any{
		"provider":             map[string]any{"order": []string{"wafer"}, "allow_fallbacks": false},
		"cache_prompt":         true,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}
	const fields = `"cache_prompt":true,"chat_template_kwargs":{"enable_thinking":false},"provider":{"allow_fallbacks":false,"order":["wafer"]}`
	var bodies []string
	collect(t, capture(t, &bodies, WithStopSequences("END"), WithExtraBody(routing)), turn("hi"))
	if want := `{"model":"qwen/qwen3.8-27b","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true},"stop":["END"],` + fields + `}`; bodies[0] != want {
		t.Fatalf("body\n%s\nwant\n%s", bodies[0], want)
	}

	bodies = nil
	collect(t, capture(t, &bodies, WithExtraBody(map[string]any{"response_format": json.RawMessage(`{ "type" : "json_object", "b": "<a&b>", "a": 1 }`)})), turn("hi"))
	if !strings.HasSuffix(bodies[0], `"stream_options":{"include_usage":true},"response_format":{"type":"json_object","b":"<a&b>","a":1}}`) {
		t.Fatalf("raw value not kept as given: %s", bodies[0])
	}

	bodies = nil
	collect(t, capture(t, &bodies, WithExtraBody(routing), WithExtraBody(map[string]any{"seed": 7})), turn("hi"))
	collect(t, capture(t, &bodies, WithExtraBody(routing), WithExtraBody(nil)), turn("hi"))
	if !strings.HasSuffix(bodies[0], `{"include_usage":true},"seed":7}`) || !strings.HasSuffix(bodies[1], `{"include_usage":true}}`) {
		t.Fatalf("a later WithExtraBody did not replace the earlier one:\n%s\n%s", bodies[0], bodies[1])
	}

	bodies = nil
	collect(t, capture(t, &bodies, WithExtraBody(map[string]any{"cache_control": map[string]any{"type": "ephemeral"}})), turn("hi"))
	if want := `{"model":"qwen/qwen3.8-27b","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true},"cache_control":{"type":"ephemeral"}}`; bodies[0] != want {
		t.Fatalf("top-level marker\n%s\nwant\n%s", bodies[0], want)
	}

	bodies = nil
	host := map[string]any{"provider": map[string]any{"order": []string{"wafer"}}}
	p := capture(t, &bodies, WithExtraBody(host))
	host["provider"].(map[string]any)["order"] = []string{"parasail"}
	host["seed"] = 1
	collect(t, p, turn("hi"))
	if !strings.HasSuffix(bodies[0], `,"provider":{"order":["wafer"]}}`) {
		t.Fatalf("a change to the host's map after New reached the body: %s", bodies[0])
	}

	bodies = nil
	p = capture(t, &bodies, WithEffort(EffortLow), WithExtraBody(routing))
	thread := []api.Message{user(text("weather in Paris?"))}
	tools := []api.Tool{{Name: "get_weather", Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}}
	for _, next := range [][]api.Message{
		{assistant(reasoning(`{"provider":"generic","data":"reasoning"}`, "Call it."), text("\n\n"), api.ContentToolUse{ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city": "Paris"}`)}),
			user(api.ContentToolResult{ID: "call_1", Content: api.UserContents{text("12C")}})},
		{assistant(reasoning(`{"provider":"generic","data":"reasoning"}`, "Answer."), text("12C in Paris.")), user(text("and Rome?"))},
		nil,
	} {
		collect(t, p, api.Request{System: "Be brief.", Tools: tools, Messages: thread})
		thread = append(thread, next...)
	}
	tail := `],"stream":true,"stream_options":{"include_usage":true},"reasoning_effort":"low",` + fields + `}`
	for i, body := range bodies {
		if !strings.HasSuffix(body, tail) {
			t.Fatalf("body %d ends differently:\n%s\nwant the tail\n%s", i, body, tail)
		}
		if i > 0 && !strings.HasPrefix(body, strings.TrimSuffix(bodies[i-1], tail)) {
			t.Fatalf("body %d does not start with body %d up to its last message:\n%s\n%s", i, i-1, bodies[i-1], body)
		}
	}
	if len(bodies) != 3 {
		t.Fatalf("%d bodies", len(bodies))
	}
}
