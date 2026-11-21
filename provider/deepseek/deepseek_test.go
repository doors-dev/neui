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

package deepseek

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

// DeepSeek publishes no raw SSE transcript anywhere, only SDK snippets, so
// every fixture below is written from the documented field names rather than
// recorded off the wire. They are shaped strictly to what the API reference
// lists and carry the fields it says are always present, including the ones
// this adapter drops.

// frame wraps a payload in the SSE framing DeepSeek uses: one data line, blank
// line separator, no event name.
func frame(payload string) string {
	return "data: " + payload + "\n\n"
}

const keepAlive = ": keep-alive\n\n"

func chunkFrame(delta string, finish string) string {
	reason := "null"
	if finish != "" {
		reason = `"` + finish + `"`
	}
	return frame(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,`+
		`"model":"deepseek-v4-pro","system_fingerprint":"fp_a1b2c3",`+
		`"choices":[{"index":0,"delta":%s,"logprobs":null,"finish_reason":%s}],"usage":null}`, delta, reason))
}

const (
	openingFrame = `{"role":"assistant","content":"","reasoning_content":null}`

	usageFrame = `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,` +
		`"model":"deepseek-v4-pro","choices":[],"usage":{"prompt_tokens":12000,"completion_tokens":950,` +
		`"total_tokens":12950,"prompt_cache_hit_tokens":9000,"prompt_cache_miss_tokens":3000,` +
		`"completion_tokens_details":{"reasoning_tokens":600}}}` + "\n\n"

	doneFrame = "data: [DONE]\n\n"
)

// serve starts a server running h and returns a Provider aimed at it.
func serve(t *testing.T, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	return serveModel(t, "deepseek-v4-pro", h, opts...)
}

// serveModel is serve for the tests that turn on which model was configured,
// since the model is no longer something a request can vary.
func serveModel(t *testing.T, model string, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	base := []Option{WithAPIKey("test-key"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client())}
	return mustNew(t, Conf{Model: api.Model{Name: model}}, append(base, opts...)...)
}

func mustNew(t *testing.T, conf Conf, opts ...Option) *Provider {
	t.Helper()
	p, err := New(conf, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// stringHandler writes the frames in order, flushing each so the client sees
// them as they are written rather than in one block at the end.
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

// drop closes the socket without finishing the response, which is what a
// mid stream disconnect looks like to the client.
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
	return drain(t, p.Stream(ctx, req))
}

func drain(t *testing.T, ch <-chan api.Event) []api.Event {
	t.Helper()
	var events []api.Event
	deadline := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-deadline:
			t.Fatal("stream did not close")
		}
	}
}

func turn(text string) api.Request {
	return api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: text}}}},
	}
}

// fold rebuilds the content blocks a stream produced, which is what
// protocol/event.go says the events are edits to. Nothing in this repo folds
// events yet, so a test that checks what replay sends has to do it here.
//
// Text and reasoning only, since those are the two streams this adapter assigns
// an index to; a tool call is assembled by hand where one is needed, because
// discarding an accumulation that never parsed is a rule of its own.
func fold(t *testing.T, events []api.Event) api.Contents {
	t.Helper()
	var model string
	var content api.Contents
	for _, e := range events {
		switch e := e.(type) {
		case api.EventStart:
			// The model the provider is configured with is what a stored block is
			// stamped with, and this event is the only thing that carries it.
			model = e.Model
		case api.EventText:
			if e.Index == len(content) {
				content = append(content, api.ContentText{})
			}
			b := content[e.Index].(api.ContentText)
			b.Text += e.Text
			content[e.Index] = b
		case api.EventThinking:
			if e.Index == len(content) {
				content = append(content, api.ContentReasoning{Model: model, Thoughts: map[int]string{}})
			}
			b := content[e.Index].(api.ContentReasoning)
			b.Thoughts[e.Part] += e.Text
			content[e.Index] = b
		case api.EventMeta:
			b := content[e.Index].(api.ContentReasoning)
			b.Meta = e.Data
			content[e.Index] = b
		}
	}
	return content
}

// trace renders an event as one comparable line, so a test asserts the whole
// sequence rather than that some event of some kind turned up.
func wantTrace(t *testing.T, events []api.Event, want ...string) {
	t.Helper()
	got := conform.DescribeAll(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] && !(strings.HasPrefix(want[i], "err ") && strings.HasPrefix(got[i], want[i]+" ")) {
			t.Fatalf("event %d: got %q, want %q\n got %v\nwant %v", i, got[i], want[i], got, want)
		}
	}
}

// Model answers from configuration alone, so it holds before any turn has run:
// a listed model carries its window, a caller's own literal reads back as
// written, and an unstated window reads as unknown rather than as no window.
func TestModel(t *testing.T) {
	if got := mustNew(t, Conf{Model: V4Pro}, WithAPIKey("k")).Model(); got != V4Pro || got.Context == 0 {
		t.Fatalf("Model %+v, want %+v", got, V4Pro)
	}
	own := api.Model{Name: "deepseek-v4-pro"}
	if got := mustNew(t, Conf{Model: own}, WithAPIKey("k")).Model(); got != own {
		t.Fatalf("Model %+v, want the caller's literal with an unknown window", got)
	}
	unknown := api.Model{Name: "my-flash-proxy"}
	if got := mustNew(t, Conf{Model: unknown}, WithAPIKey("k")).Model(); got.Inputs != 0 {
		t.Fatalf("an unknown id reads back Inputs %b, want zero, which sends every kind", got.Inputs)
	}
}

func TestProviderRefusesBytesMeasuredAsFitting(t *testing.T) {
	const declared = 1 << 20
	var sent int
	p := mustNew(t,
		Conf{Model: api.Model{Name: "deepseek-v4-pro", RequestBytes: declared}},
		WithAPIKey("test-key"),
		WithBaseURL(serverURL(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sent = len(body)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			io.WriteString(w, "payload too large")
		})),
	)
	events := collect(t, p, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "hi"}}}}})
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: a 413 is the request refused by its size (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if msg := e.Err.Error(); sent == 0 || !strings.Contains(msg, fmt.Sprint(sent)) || !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q names neither the %d bytes sent nor the %d declared: nothing points at the misconfiguration", msg, sent, declared)
	}
	var carried *APIError
	if !errors.As(e.Err, &carried) || carried.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error %v lost what the API answered", e.Err)
	}
}

func serverURL(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestUnmeasuredRequestTooLargeNamesNoLimit(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, "payload too large")
	})
	events := collect(t, p, turn("hi"))
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: the refusal is still one about bytes (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if msg := e.Err.Error(); strings.Contains(msg, "the 0 declared") {
		t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
	}
}

func TestContextOverflow(t *testing.T) {
	const overflow = "This model's maximum context length is 131072 tokens. However, you requested 131329 tokens " +
		"(123137 in the messages, 8192 in the completion). Please reduce the length of the messages or completion."
	tests := []struct {
		name   string
		status int
		body   string
		want   api.ErrCode
	}{
		{"the overflow, as the server wrote it", 400, `{"error":{"message":"` + overflow + `","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`, api.CodeContextLength},
		{"the same sentence in upper case", 400, `{"error":{"message":"` + strings.ToUpper(overflow) + `","type":"invalid_request_error"}}`, api.CodeContextLength},
		{"the same sentence in a body that is not json", 400, overflow, api.CodeContextLength},
		{"without the requested clause", 400, `{"error":{"message":"Input exceeds the maximum context length of this model."}}`, api.CodeContextLength},
		{"a context_length_exceeded code", 400, `{"error":{"message":"too long","code":"context_length_exceeded"}}`, api.CodeContextLength},
		{"a context_length_exceeded type", 400, `{"error":{"message":"too long","type":"context_length_exceeded"}}`, api.CodeContextLength},
		{"on a 413", 413, `{"error":{"message":"` + overflow + `"}}`, api.CodeContextLength},
		{"on a 422", 422, `{"error":{"message":"` + overflow + `"}}`, api.CodeContextLength},
		{"not on a 404", 404, `{"error":{"message":"` + overflow + `"}}`, api.CodeInvalid},
		{"an ordinary bad request", 400, `{"error":{"message":"Invalid request body format.","type":"invalid_request_error","code":"invalid_request_error"}}`, api.CodeInvalid},
		{"a refusal that only names the context", 400, `{"error":{"message":"Context caching is not supported for this model.","type":"invalid_request_error"}}`, api.CodeInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantTrace(t, events, "err "+string(tc.want))
		})
	}
}

// A turn whose finish reason arrived and whose socket then died before [DONE]
// lost its usage frame and its sentinel. The connection died, so the channel
// closes with no terminal event whatever the reason said.
func TestStreamDisconnectAfterTheFinishReasonIsSilent(t *testing.T) {
	for _, finish := range []string{"stop", "tool_calls", "length", "content_filter"} {
		t.Run(finish, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, chunkFrame(openingFrame, ""))
				io.WriteString(w, chunkFrame(`{"content":"done"}`, ""))
				io.WriteString(w, chunkFrame(`{}`, finish))
				flush(w)
				drop(w)
			})
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantTrace(t, events,
				"start chatcmpl-1 deepseek-v4-pro",
				`text 0 "done"`,
			)
		})
	}
}

func TestModelDeclarations(t *testing.T) {
	for _, m := range []api.Model{Flash, V4Pro} {
		if m.Context != 1_048_576 || m.RequestBytes != 48*1024*1024 {
			t.Errorf("%s declares context %d and request bytes %d", m.Name, m.Context, m.RequestBytes)
		}
	}
	if Flash.ImagePixels != 1300*1300 || Flash.ImageSide != 8192 || Flash.ImageBytes != 24*1024*1024 || Flash.ImagesPerRequest != 600 {
		t.Errorf("Flash image fields %+v", Flash)
	}
	if V4Pro.ImagePixels != 0 || V4Pro.ImageSide != 0 || V4Pro.ImageBytes != 0 || V4Pro.ImagesPerRequest != 0 {
		t.Errorf("V4Pro declares image capacity it has no path for: %+v", V4Pro)
	}
	if Flash.Inputs != api.ModalityText|api.ModalityImage || V4Pro.Inputs != api.ModalityText {
		t.Errorf("Flash reads %b and V4Pro reads %b, want text and images, and text", Flash.Inputs, V4Pro.Inputs)
	}
}

func TestFamily(t *testing.T) {
	for name, want := range map[string]family{
		"deepseek-flash":               familyFlash,
		"deepseek-v4-flash":            familyFlash,
		"deepseek-v4-flash-vision-exp": familyFlash,
		"deepseek-v4-pro":              familyPro,
		"my-flash-proxy":               familyUnknown,
	} {
		if got := mustNew(t, Conf{Model: api.Model{Name: name}}, WithAPIKey("k")).family; got != want {
			t.Errorf("%s: family %d, want %d", name, got, want)
		}
	}
}

func TestNewRefuses(t *testing.T) {
	flash := Conf{Model: Flash}
	pro := Conf{Model: V4Pro}
	key := WithAPIKey("k")
	tests := []struct {
		name string
		conf Conf
		opts []Option
		want string
	}{
		{"no model", Conf{}, []Option{key}, "no model"},
		{"unknown effort", pro, []Option{key, WithEffort("medium")}, `effort "medium"`},
		{"temperature with thinking on by default", pro, []Option{key, WithTemperature(0.2)}, "EffortNone"},
		{"temperature with an effort", flash, []Option{key, WithEffort(EffortHigh), WithTemperature(0.2)}, "EffortNone"},
		{"temperature below range", pro, []Option{key, WithEffort(EffortNone), WithTemperature(-0.1)}, "range"},
		{"temperature above range", pro, []Option{key, WithEffort(EffortNone), WithTemperature(2.1)}, "range"},
		{"top_p on pro with thinking on", pro, []Option{key, WithTopP(0.9)}, "ignores top_p"},
		{"top_p on pro with an effort", pro, []Option{key, WithEffort(EffortLow), WithTopP(0.9)}, "ignores top_p"},
		{"top_p on flash with thinking off", flash, []Option{key, WithEffort(EffortNone), WithTopP(0.97)}, "fixes top_p"},
		{"top_p of zero", flash, []Option{key, WithTopP(0)}, "range"},
		{"top_p above one", flash, []Option{key, WithTopP(1.1)}, "range"},
		{"negative max tokens", pro, []Option{key, WithMaxTokens(-1)}, "negative"},
		{"max tokens over the ceiling", pro, []Option{key, WithMaxTokens(maxOutputTokens + 1)}, "ceiling"},
		{"a user id over the limit", pro, []Option{key, WithUserID(strings.Repeat("a", maxUserIDBytes+1))}, "512"},
		{"an empty base URL", pro, []Option{key, WithBaseURL("")}, "base URL"},
		{"a nil client", pro, []Option{key, WithHTTPClient(nil)}, "client"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.conf, tc.opts...)
			if err == nil || p != nil {
				t.Fatalf("New returned %v, %v; want an error", p, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
	for _, opts := range [][]Option{
		{key, WithEffort(EffortNone), WithTemperature(0), WithTopP(1)},
		{key, WithEffort(EffortNone), WithTemperature(2)},
		{key, WithMaxTokens(0)},
		{key, WithMaxTokens(maxOutputTokens)},
		{key, WithUserID(strings.Repeat("a", maxUserIDBytes))},
	} {
		mustNew(t, pro, opts...)
	}
	mustNew(t, flash, key, WithTopP(0.97))
	mustNew(t, flash, key, nil)
	mustNew(t, flash, key, WithEffort(EffortMax), WithTopP(0.97))
	own := mustNew(t, Conf{Model: api.Model{Name: "my-flash-proxy"}}, key, WithEffort("medium"), WithTemperature(0.2), WithTopP(0.9), WithMaxTokens(maxOutputTokens+1))
	if own.effort != "medium" || own.temperature == nil || own.topP == nil || own.maxTokens != maxOutputTokens+1 {
		t.Errorf("an unknown model lost an option: %+v", own)
	}
}

func TestNilHTTPClientIsRefused(t *testing.T) {
	p := &Provider{client: http.DefaultClient}
	if err := WithHTTPClient(nil)(p); err == nil {
		t.Fatalf("WithHTTPClient(nil) was accepted")
	}
	if p.client != http.DefaultClient {
		t.Fatalf("a refused nil client replaced the one already set")
	}
}

func TestUnparseableErrorBodyIsCut(t *testing.T) {
	e := apiError(502, nil, []byte(strings.Repeat("é", 2000)))
	if n := len([]rune(e.Message)); n != 512+1 {
		t.Fatalf("message has %d runes, want %d", n, 512+1)
	}
}

func TestAPIKeySource(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "env-key")
	if p := mustNew(t, Conf{Model: V4Pro}); p.key != "env-key" {
		t.Fatalf("key %q, want the environment's", p.key)
	}
	if p := mustNew(t, Conf{Model: V4Pro}, WithAPIKey("")); p.key != "" {
		t.Fatalf("key %q, want the explicit empty one", p.key)
	}
	t.Setenv("DEEPSEEK_API_KEY", "")
	p := mustNew(t, Conf{Model: V4Pro})
	if p.key != "" {
		t.Fatalf("key %q, want none", p.key)
	}
	httpReq, _, err := p.request(context.Background(), turn("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if got, sent := httpReq.Header["Authorization"]; sent {
		t.Fatalf("Authorization %q sent with no key", got)
	}
}
