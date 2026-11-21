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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/internal/thread/response"
	"neui.dev/provider/lib"
)

const testModel = "gemini-4-hydrogen"

func mustNew(t *testing.T, conf Conf, opts ...Option) *Provider {
	t.Helper()
	p, err := New(conf, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func encoder(t *testing.T, opts ...Option) *Provider {
	t.Helper()
	return mustNew(t, Conf{Model: api.Model{Name: testModel}}, append([]Option{WithAPIKey("test-key"), WithBaseURL("http://gemini.invalid")}, opts...)...)
}

func encodeBody(t *testing.T, p *Provider, req api.Request) []byte {
	t.Helper()
	httpReq, _, err := p.request(context.Background(), req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body, err := io.ReadAll(httpReq.Body)
	if err != nil {
		t.Fatal(err)
	}
	httpReq.Body.Close()
	return body
}

func field(t *testing.T, body []byte, name string) string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	return string(fields[name])
}

func steps(t *testing.T, req api.Request, opts ...Option) string {
	t.Helper()
	return field(t, encodeBody(t, encoder(t, opts...), req), "input")
}

func wantJSON(t *testing.T, got, want string) {
	t.Helper()
	var g, w bytes.Buffer
	if err := json.Compact(&g, []byte(got)); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Compact(&w, []byte(want)); err != nil {
		t.Fatalf("want is not JSON: %v\n%s", err, want)
	}
	if g.String() != w.String() {
		t.Fatalf("JSON differs\n got %s\nwant %s", g.String(), w.String())
	}
}

func user(c ...api.Content) api.Message {
	return api.Message{Role: api.RoleUser, Content: c}
}

func assistant(c ...api.Content) api.Message {
	return api.Message{Role: api.RoleAssistant, Content: c}
}

func text(s string) api.ContentText {
	return api.ContentText{Text: s}
}

func call(id, name, input string) api.ContentToolUse {
	c := api.ContentToolUse{ID: id, Name: name}
	if input != "" {
		c.Input = json.RawMessage(input)
	}
	return c
}

func result(id string, failure api.ToolError, c ...api.UserContent) api.ContentToolResult {
	return api.ContentToolResult{ID: id, Content: c, Error: failure}
}

func turn(s string) api.Request {
	return api.Request{Messages: []api.Message{user(text(s))}}
}

const (
	dummy   = `{"type":"thought","signature":"skip_thought_signature_validator"}`
	ownMeta = `{"type":"thought","signature":"c2lnbmF0dXJl"}`
)

var ownThought = lib.Meta(providerName, json.RawMessage(ownMeta))

type levelCounter map[slog.Level]int

func (c levelCounter) Enabled(context.Context, slog.Level) bool { return true }

func (c levelCounter) Handle(_ context.Context, r slog.Record) error {
	c[r.Level]++
	return nil
}

func (c levelCounter) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c levelCounter) WithGroup(string) slog.Handler { return c }

func countLogs(t *testing.T) levelCounter {
	t.Helper()
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return counts
}

func TestName(t *testing.T) {
	if got := encoder(t).Name(); got != "gemini" {
		t.Fatalf("Name %q", got)
	}
}

func TestModel(t *testing.T) {
	m := api.Model{Name: "gemini-9-unknown", Context: 7}
	if got := mustNew(t, Conf{Model: m}).Model(); got != m {
		t.Fatalf("Model %+v, want %+v", got, m)
	}
	if got := mustNew(t, Conf{Model: Hydrogen4}).Model(); got != Hydrogen4 {
		t.Fatalf("Model %+v, want %+v", got, Hydrogen4)
	}
}

func TestDeclaredModels(t *testing.T) {
	for _, c := range []struct {
		m       api.Model
		name    string
		context int
		levels  []Effort
	}{
		{Hydrogen4, "gemini-4-hydrogen", 1_048_576, []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh}},
		{Neon4, "gemini-4-neon", 1_048_576, []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh}},
		{Carbon4, "gemini-4-carbon", 1_048_576, []Effort{EffortLow, EffortMedium, EffortHigh}},
		{Argon4, "gemini-4-argon", 1_048_576, []Effort{EffortLow, EffortMedium, EffortHigh}},
		{Flash38, "gemini-3.8-flash", 1_114_112, []Effort{EffortLow, EffortMedium, EffortHigh}},
		{FlashLite35, "gemini-3.5-flash-lite", 1_114_112, []Effort{EffortMinimal, EffortLow, EffortMedium, EffortHigh}},
	} {
		want := api.Model{Name: c.name, Context: c.context, ImagesPerRequest: 3_600, RequestBytes: 100_000_000, Inputs: api.ModalityText | api.ModalityImage | api.ModalityDocument}
		if c.m != want {
			t.Errorf("%s = %+v, want %+v", c.name, c.m, want)
		}
		r, ok := declared[c.name]
		if !ok {
			t.Errorf("%s is not declared", c.name)
			continue
		}
		if r.maxTokens != 65_536 || !slices.Equal(r.levels, c.levels) {
			t.Errorf("%s rules %+v, want max 65536 and levels %v", c.name, r, c.levels)
		}
	}
	if len(declared) != 6 {
		t.Errorf("%d declared models, want 6", len(declared))
	}
}

func TestDeclaredModelsAreMatchedByExactName(t *testing.T) {
	for _, name := range []string{"Gemini-4-Hydrogen", "gemini-4-hydrogen-001", "models/gemini-4-hydrogen", " gemini-4-hydrogen"} {
		if _, err := New(Conf{Model: api.Model{Name: name}}, WithEffort(EffortMinimal), WithMaxTokens(1_000_000)); err != nil {
			t.Errorf("%q was checked as a declared model: %v", name, err)
		}
	}
}

func TestNewRefuses(t *testing.T) {
	for _, c := range []struct {
		name string
		conf Conf
		opts []Option
		want string
	}{
		{"no model", Conf{}, nil, "gemini: no model, set Conf.Model"},
		{"empty base", Conf{Model: Hydrogen4}, []Option{WithBaseURL("")}, "gemini: empty base URL"},
		{"unparseable base", Conf{Model: Hydrogen4}, []Option{WithBaseURL("://x")}, `gemini: base URL "://x": parse "://x": missing protocol scheme`},
		{"base without scheme", Conf{Model: Hydrogen4}, []Option{WithBaseURL("example.com")}, `gemini: base URL "example.com" needs a scheme and host`},
		{"nil client", Conf{Model: Hydrogen4}, []Option{WithHTTPClient(nil)}, "gemini: WithHTTPClient needs a client"},
		{"empty revision", Conf{Model: Hydrogen4}, []Option{WithRevision("")}, "gemini: WithRevision needs a revision"},
		{"negative max tokens", Conf{Model: Hydrogen4}, []Option{WithMaxTokens(-1)}, "gemini: max tokens -1 is negative"},
		{"minimal on hydrogen", Conf{Model: Hydrogen4}, []Option{WithEffort(EffortMinimal)}, `gemini: model gemini-4-hydrogen does not take thinking level "minimal"; it takes ["low" "medium" "high" "xhigh"]`},
		{"minimal on neon", Conf{Model: Neon4}, []Option{WithEffort(EffortMinimal)}, `gemini: model gemini-4-neon does not take thinking level "minimal"; it takes ["low" "medium" "high" "xhigh"]`},
		{"minimal on carbon", Conf{Model: Carbon4}, []Option{WithEffort(EffortMinimal)}, `gemini: model gemini-4-carbon does not take thinking level "minimal"; it takes ["low" "medium" "high"]`},
		{"minimal on argon", Conf{Model: Argon4}, []Option{WithEffort(EffortMinimal)}, `gemini: model gemini-4-argon does not take thinking level "minimal"; it takes ["low" "medium" "high"]`},
		{"minimal on 3.8 flash", Conf{Model: Flash38}, []Option{WithEffort(EffortMinimal)}, `gemini: model gemini-3.8-flash does not take thinking level "minimal"; it takes ["low" "medium" "high"]`},
		{"xhigh on carbon", Conf{Model: Carbon4}, []Option{WithEffort(EffortXHigh)}, `gemini: model gemini-4-carbon does not take thinking level "xhigh"; it takes ["low" "medium" "high"]`},
		{"xhigh on argon", Conf{Model: Argon4}, []Option{WithEffort(EffortXHigh)}, `gemini: model gemini-4-argon does not take thinking level "xhigh"; it takes ["low" "medium" "high"]`},
		{"xhigh on 3.8 flash", Conf{Model: Flash38}, []Option{WithEffort(EffortXHigh)}, `gemini: model gemini-3.8-flash does not take thinking level "xhigh"; it takes ["low" "medium" "high"]`},
		{"xhigh on 3.5 flash-lite", Conf{Model: FlashLite35}, []Option{WithEffort(EffortXHigh)}, `gemini: model gemini-3.5-flash-lite does not take thinking level "xhigh"; it takes ["minimal" "low" "medium" "high"]`},
		{"max on hydrogen", Conf{Model: Hydrogen4}, []Option{WithEffort("max")}, `gemini: model gemini-4-hydrogen does not take thinking level "max"; it takes ["low" "medium" "high" "xhigh"]`},
		{"max on 3.5 flash-lite", Conf{Model: FlashLite35}, []Option{WithEffort("max")}, `gemini: model gemini-3.5-flash-lite does not take thinking level "max"; it takes ["minimal" "low" "medium" "high"]`},
	} {
		_, err := New(c.conf, c.opts...)
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
	for _, m := range []api.Model{Hydrogen4, Neon4, Carbon4, Argon4, Flash38, FlashLite35} {
		want := "gemini: max tokens 65537 is over the 65536 model " + m.Name + " returns in one request"
		if _, err := New(Conf{Model: m}, WithMaxTokens(65_537)); err == nil || err.Error() != want {
			t.Errorf("%s: err %v, want %q", m.Name, err, want)
		}
		if _, err := New(Conf{Model: m}, WithMaxTokens(65_536)); err != nil {
			t.Errorf("%s refused the 65,536 ceiling: %v", m.Name, err)
		}
		for _, e := range declared[m.Name].levels {
			if _, err := New(Conf{Model: m}, WithEffort(e)); err != nil {
				t.Errorf("%s refused its level %s: %v", m.Name, e, err)
			}
		}
	}
	if _, err := New(Conf{Model: api.Model{Name: "gemini-9-unknown"}}, WithEffort("max"), WithMaxTokens(1_000_000)); err != nil {
		t.Errorf("an undeclared model was checked: %v", err)
	}
	if _, err := New(Conf{Model: Hydrogen4}, nil, WithAPIKey("k"), nil); err != nil {
		t.Errorf("a nil option was not skipped: %v", err)
	}
}

func TestDeclaredModelChecksRunInNew(t *testing.T) {
	for _, opts := range [][]Option{
		{WithEffort(EffortXHigh), WithMaxTokens(10)},
		{WithMaxTokens(10), WithEffort(EffortXHigh)},
	} {
		if _, err := New(Conf{Model: Carbon4}, opts...); err == nil {
			t.Error("xhigh on carbon was not refused")
		}
		if _, err := New(Conf{Model: Hydrogen4}, opts...); err != nil {
			t.Errorf("xhigh on hydrogen was refused: %v", err)
		}
	}
}

func TestStopSequencesReplace(t *testing.T) {
	body := encodeBody(t, encoder(t, WithStopSequences("a", "b"), WithStopSequences("c")), turn("hi"))
	wantJSON(t, field(t, body, "generation_config"), `{"stop_sequences":["c"],"thinking_summaries":"auto"}`)
}

func TestNilHTTPClientIsRefused(t *testing.T) {
	if _, err := New(Conf{Model: Hydrogen4}, WithHTTPClient(nil)); err == nil {
		t.Fatal("a nil client was accepted")
	}
	c := &http.Client{}
	p := mustNew(t, Conf{Model: Hydrogen4}, WithHTTPClient(c))
	if p.client != c {
		t.Fatal("the client was not kept")
	}
}

func TestAPIKeySource(t *testing.T) {
	counts := countLogs(t)
	t.Setenv("GEMINI_API_KEY", "env-key")
	if p := mustNew(t, Conf{Model: Hydrogen4}); p.key != "env-key" {
		t.Fatalf("key %q, want the environment's", p.key)
	}
	if p := mustNew(t, Conf{Model: Hydrogen4}, WithAPIKey("")); p.key != "" {
		t.Fatalf("key %q, want the explicit empty one", p.key)
	}
	t.Setenv("GEMINI_API_KEY", "")
	p := mustNew(t, Conf{Model: Hydrogen4})
	httpReq, _, err := p.request(context.Background(), turn("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if got, sent := httpReq.Header["X-Goog-Api-Key"]; sent {
		t.Fatalf("x-goog-api-key %q sent with no key", got)
	}
	if len(counts) != 0 {
		t.Fatalf("New logged %v", map[slog.Level]int(counts))
	}
}

func TestRequestWire(t *testing.T) {
	type seen struct {
		method, path, query string
		header              http.Header
		body                []byte
	}
	got := make(chan seen, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), body}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	for _, c := range []struct {
		opts     []Option
		revision string
	}{
		{nil, "2026-05-20"},
		{[]Option{WithRevision("2026-07-01")}, "2026-07-01"},
	} {
		p := mustNew(t, Conf{Model: Hydrogen4}, append([]Option{WithAPIKey("test-key"), WithBaseURL(srv.URL + "/")}, c.opts...)...)
		collect(t, p, api.Request{Tools: []api.Tool{{Name: "clock"}}, System: "be brief", Messages: []api.Message{user(text("hi"))}})
		s := <-got
		if s.method != http.MethodPost || s.path != "/v1/interactions" || s.query != "" {
			t.Fatalf("%s %s?%s, want POST /v1/interactions", s.method, s.path, s.query)
		}
		for k, v := range map[string]string{"Content-Type": "application/json", "Accept": "text/event-stream", "Api-Revision": c.revision, "X-Goog-Api-Key": "test-key"} {
			if s.header.Get(k) != v {
				t.Fatalf("%s %q, want %q", k, s.header.Get(k), v)
			}
		}
		dec := json.NewDecoder(bytes.NewReader(s.body))
		var keys []string
		dec.Token()
		for dec.More() {
			k, _ := dec.Token()
			keys = append(keys, k.(string))
			var skip json.RawMessage
			dec.Decode(&skip)
		}
		if want := []string{"model", "stream", "store", "tools", "system_instruction", "input", "generation_config"}; !slices.Equal(keys, want) {
			t.Fatalf("keys %v, want %v", keys, want)
		}
		if field(t, s.body, "stream") != "true" || field(t, s.body, "store") != "false" {
			t.Fatalf("body %s, want stream true and store false", s.body)
		}
	}
}

func TestSettings(t *testing.T) {
	for _, c := range []struct {
		name string
		opts []Option
		want string
	}{
		{"defaults", nil, `{"thinking_summaries":"auto"}`},
		{"summaries off", []Option{WithThoughtSummaries(false)}, `{"thinking_summaries":"none"}`},
		{"summaries back on", []Option{WithThoughtSummaries(false), WithThoughtSummaries(true)}, `{"thinking_summaries":"auto"}`},
		{"level", []Option{WithEffort(EffortMedium)}, `{"thinking_level":"medium","thinking_summaries":"auto"}`},
		{"empty level", []Option{WithEffort("")}, `{"thinking_summaries":"auto"}`},
		{"stop", []Option{WithStopSequences("END", "\n\n")}, `{"stop_sequences":["END","\n\n"],"thinking_summaries":"auto"}`},
		{"no stop", []Option{WithStopSequences()}, `{"thinking_summaries":"auto"}`},
		{"max tokens", []Option{WithMaxTokens(4096)}, `{"max_output_tokens":4096,"thinking_summaries":"auto"}`},
		{"zero max tokens", []Option{WithMaxTokens(0)}, `{"thinking_summaries":"auto"}`},
		{"all", []Option{WithMaxTokens(9), WithStopSequences("x"), WithEffort(EffortLow), WithThoughtSummaries(false)}, `{"max_output_tokens":9,"stop_sequences":["x"],"thinking_level":"low","thinking_summaries":"none"}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantJSON(t, field(t, encodeBody(t, encoder(t, c.opts...), turn("hi")), "generation_config"), c.want)
		})
	}
}

func TestTools(t *testing.T) {
	req := api.Request{Tools: []api.Tool{
		{Name: "clock"},
		{Name: "lookup", Desc: "Look <a> word & up.", Schema: map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string", "description": "<q>"}}}},
	}, Messages: []api.Message{user(text("hi"))}}
	body := encodeBody(t, encoder(t), req)
	if bytes.Contains(body, []byte(`\u003c`)) || !bytes.Contains(body, []byte("<a> word & up")) {
		t.Fatalf("HTML escaped: %s", body)
	}
	wantJSON(t, field(t, body, "tools"), `[
		{"type":"function","name":"clock","parameters":{"type":"object","properties":{}}},
		{"type":"function","name":"lookup","description":"Look <a> word & up.","parameters":{"properties":{"q":{"description":"<q>","type":"string"}},"type":"object"}}]`)
	if _, _, err := encoder(t).request(context.Background(), api.Request{Tools: []api.Tool{{Desc: "x"}}, Messages: []api.Message{user(text("hi"))}}); err == nil || err.Error() != "gemini: tool has no name" {
		t.Fatalf("err %v, want the nameless tool refused", err)
	}
	if _, _, err := encoder(t).request(context.Background(), api.Request{Tools: []api.Tool{{Name: "bad", Schema: map[string]any{"x": func() {}}}}, Messages: []api.Message{user(text("hi"))}}); err == nil || !strings.HasPrefix(err.Error(), `gemini: tool "bad" schema: `) {
		t.Fatalf("err %v, want the schema refused", err)
	}
	for _, c := range []struct {
		choice api.ToolChoice
		want   string
	}{
		{"", `{"thinking_summaries":"auto"}`},
		{api.ToolChoiceAuto, `{"thinking_summaries":"auto","tool_choice":"auto"}`},
		{api.ToolChoiceNone, `{"thinking_summaries":"auto","tool_choice":"none"}`},
		{api.ToolChoiceAny, `{"thinking_summaries":"auto","tool_choice":"any"}`},
		{"lookup", `{"thinking_summaries":"auto","tool_choice":{"allowed_tools":{"mode":"any","tools":["lookup"]}}}`},
	} {
		req.ToolChoice = c.choice
		wantJSON(t, field(t, encodeBody(t, encoder(t), req), "generation_config"), c.want)
	}
}

func TestRefusalsSendNothing(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	p := mustNew(t, Conf{Model: Hydrogen4}, WithAPIKey("k"), WithBaseURL(srv.URL))
	for _, c := range []struct {
		name string
		req  api.Request
		want string
	}{
		{"no messages", api.Request{}, "gemini: request has no messages"},
		{"bad role", api.Request{Messages: []api.Message{{Role: "system", Content: api.Contents{text("x")}}}}, `gemini: message 0 has role "system"`},
		{"discarded without err_json", api.Request{Messages: []api.Message{
			user(text("hi")),
			assistant(api.ContentToolUse{ID: "c1", Name: "lookup", Input: api.DiscardedArguments}),
			user(result("c1", api.ToolErrRuntime, text("no"))),
		}}, `gemini: tool call "c1" carries discarded arguments and no result answering it reports err_json`},
	} {
		events := collect(t, p, c.req)
		if len(events) != 1 {
			t.Fatalf("%s: %v, want one EventErr", c.name, conform.DescribeAll(events))
		}
		e, ok := events[0].(api.EventErr)
		if !ok || e.Code != api.CodeInvalid || e.Err.Error() != c.want {
			t.Fatalf("%s: %s, want EventErr invalid %q", c.name, conform.Describe(events[0]), c.want)
		}
	}
	if hits != 0 {
		t.Fatalf("%d requests reached the server", hits)
	}
}

func TestNotes(t *testing.T) {
	png := api.ContentImage{Data: []byte("\x89PNG"), Mime: "image/png"}
	for _, c := range []struct {
		name string
		msgs []api.Message
		want string
	}{
		{"empty text in a user message", []api.Message{user(text("a"), text(""), text("b"))},
			`[{"type":"user_input","content":[{"type":"text","text":"a"},{"type":"text","text":"[content omitted]"},{"type":"text","text":"b"}]}]`},
		{"whitespace text is sent", []api.Message{user(text(" \n"))},
			`[{"type":"user_input","content":[{"type":"text","text":" \n"}]}]`},
		{"image without bytes", []api.Message{user(api.ContentImage{Mime: "image/png"})},
			`[{"type":"user_input","content":[{"type":"text","text":"[image omitted]"}]}]`},
		{"image of a type the API does not take here", []api.Message{user(api.ContentImage{Data: []byte("BM"), Mime: "image/bmp"})},
			`[{"type":"user_input","content":[{"type":"text","text":"[image omitted]"}]}]`},
		{"document without bytes", []api.Message{user(api.ContentDocument{Mime: "application/pdf"})},
			`[{"type":"user_input","content":[{"type":"text","text":"[document omitted]"}]}]`},
		{"document of an unknown type", []api.Message{user(api.ContentDocument{Data: []byte("PK"), Mime: "application/zip"})},
			`[{"type":"user_input","content":[{"type":"text","text":"[document omitted]"}]}]`},
		{"text document that is not UTF-8", []api.Message{user(api.ContentDocument{Data: []byte{0xff, 0xfe}, Mime: "text/plain"})},
			`[{"type":"user_input","content":[{"type":"text","text":"[document omitted]"}]}]`},
		{"text document in a result that is not UTF-8", []api.Message{user(text("hi")), assistant(call("c1", "fetch", `{}`)), user(result("c1", api.ToolOk, api.ContentDocument{Data: []byte{0xff, 0xfe}, Mime: "text/markdown"}))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"c1","name":"fetch","arguments":{}},{"type":"function_result","name":"fetch","call_id":"c1","result":[{"type":"text","text":"[document omitted]"}]}]`},
		{"call in a user message", []api.Message{user(text("hi"), call("c1", "lookup", `{}`))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"},{"type":"text","text":"[content omitted]"}]}]`},
		{"reasoning in a user message sends nothing", []api.Message{user(api.ContentReasoning{Meta: ownThought})},
			`[{"type":"user_input","content":[{"type":"text","text":"[empty]"}]}]`},
		{"opaque in a user message sends nothing", []api.Message{user(api.ContentOpaque{Kind: "gemini.google_search_call", Data: json.RawMessage(`{"type":"google_search_call","id":"g"}`)})},
			`[{"type":"user_input","content":[{"type":"text","text":"[empty]"}]}]`},
		{"unknown type in a user message", []api.Message{user(text("a"), nil)},
			`[{"type":"user_input","content":[{"type":"text","text":"a"},{"type":"text","text":"[content omitted]"}]}]`},
		{"empty user message", []api.Message{user()},
			`[{"type":"user_input","content":[{"type":"text","text":"[empty]"}]}]`},
		{"empty assistant message", []api.Message{user(text("hi")), assistant(), user(text("again"))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"[empty]"}]},{"type":"user_input","content":[{"type":"text","text":"again"}]}]`},
		{"empty text in an assistant run before a call", []api.Message{user(text("hi")), assistant(text(""), call("c1", "lookup", `{"q":"x"}`)), user(result("c1", api.ToolOk, text("ok")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"model_output","content":[{"type":"text","text":"[content omitted]"}]},{"type":"function_call","id":"c1","name":"lookup","arguments":{"q":"x"}},{"type":"function_result","name":"lookup","call_id":"c1","result":[{"type":"text","text":"ok"}]}]`},
		{"call without a name", []api.Message{user(text("hi")), assistant(call("c1", "", `{}`)), user(result("c1", api.ToolOk, text("ok")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"c1","name":"` + lib.CallName("") + `","arguments":{}},{"type":"function_result","name":"` + lib.CallName("") + `","call_id":"c1","result":[{"type":"text","text":"ok"}]}]`},
		{"call without an id", []api.Message{user(text("hi")), assistant(call("", "lookup", `{}`)), user(result("", api.ToolOk, text("ok")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"` + lib.CallID("") + `","name":"lookup","arguments":{}},{"type":"function_result","name":"lookup","call_id":"` + lib.CallID("") + `","result":[{"type":"text","text":"ok"}]}]`},
		{"call whose input is not an object", []api.Message{user(text("hi")), assistant(call("c1", "lookup", `[1]`), call("c2", "lookup", `{"q":`)), user(result("c1", api.ToolOk, text("a")), result("c2", api.ToolOk, text("b")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"c1","name":"lookup","arguments":{"input":[1]}},{"type":"function_call","id":"c2","name":"lookup","arguments":{"input":"{\"q\":"}},{"type":"function_result","name":"lookup","call_id":"c1","result":[{"type":"text","text":"a"}]},{"type":"function_result","name":"lookup","call_id":"c2","result":[{"type":"text","text":"b"}]}]`},
		{"result in an assistant message", []api.Message{user(text("hi")), assistant(text("x"), result("c1", api.ToolOk, text("ok")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"x"},{"type":"text","text":"[content omitted]"}]}]`},
		{"result answering no call goes in place under a placeholder name", []api.Message{user(text("hi"), result("c9", api.ToolOk, text("ok")))},
			`[{"type":"function_result","name":"` + lib.CallName("") + `","call_id":"c9","result":[{"type":"text","text":"ok"}]},{"type":"user_input","content":[{"type":"text","text":"hi"}]}]`},
		{"result two turns after its call takes the call's name", []api.Message{user(text("hi")), assistant(call("c1", "lookup", `{}`)), user(text("wait")), assistant(text("ok")), user(result("c1", api.ToolOk, text("late")))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"c1","name":"lookup","arguments":{}},{"type":"user_input","content":[{"type":"text","text":"wait"}]},{"type":"model_output","content":[{"type":"text","text":"ok"}]},{"type":"function_result","name":"lookup","call_id":"c1","result":[{"type":"text","text":"late"}]}]`},
		{"result answering a call in a user message", []api.Message{user(call("c1", "lookup", `{}`), result("c1", api.ToolOk, text("ok")))},
			`[{"type":"user_input","content":[{"type":"text","text":"[content omitted]"},{"type":"text","text":"[content omitted]"}]}]`},
		{"reasoning without own meta sends nothing", []api.Message{user(text("hi")), assistant(api.ContentReasoning{Summary: api.Indexed[string]{0: "plan"}}, text("hello"))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"hello"}]}]`},
		{"foreign opaque sends nothing", []api.Message{user(text("hi")), assistant(api.ContentOpaque{Kind: "claude.web_search_tool_result", Data: json.RawMessage(`{"type":"web_search_tool_result"}`)}, text("x"))},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"x"}]}]`},
		{"unknown type in an assistant message", []api.Message{user(text("hi")), assistant(nil)},
			`[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"[content omitted]"}]}]`},
		{"image in an assistant message", []api.Message{user(text("draw")), assistant(text("here"), png)},
			`[{"type":"user_input","content":[{"type":"text","text":"draw"}]},{"type":"model_output","content":[{"type":"text","text":"here"},{"type":"image","data":"iVBORw==","mime_type":"image/png"}]}]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantJSON(t, steps(t, api.Request{Messages: c.msgs}), c.want)
		})
	}
}

func TestEncodeLogsNothingForAnOmission(t *testing.T) {
	counts := countLogs(t)
	steps(t, api.Request{Messages: []api.Message{
		user(text(""), api.ContentImage{Mime: "image/bmp"}, api.ContentDocument{Mime: "application/zip", Data: []byte("x")}, result("x", api.ToolOk)),
		assistant(api.ContentReasoning{Meta: json.RawMessage(`"claude-sig"`)}, call("", "x", ""), text("ok")),
	}})
	if len(counts) != 0 {
		t.Fatalf("an omission logged %v", map[slog.Level]int(counts))
	}
}

func TestEncodeLogsOnlyAnUnknownTypeOnce(t *testing.T) {
	counts := countLogs(t)
	req := api.Request{Messages: []api.Message{user(text("hi"), nil), assistant(nil)}}
	for range 3 {
		steps(t, req)
	}
	if counts[slog.LevelWarn] != 0 || counts[slog.LevelError] > 1 {
		t.Fatalf("three encodes logged %v, want no warnings and at most one error", map[slog.Level]int(counts))
	}
}

func TestImageMimes(t *testing.T) {
	for in, want := range map[string]string{
		"image/png": "image/png", "image/jpeg": "image/jpeg", "image/webp": "image/webp", "image/gif": "image/gif",
		"image/jpg": "image/jpeg", " IMAGE/PNG ": "image/png", "Image/Jpg": "image/jpeg",
	} {
		got := steps(t, api.Request{Messages: []api.Message{user(api.ContentImage{Data: []byte{1, 2, 3}, Mime: in})}})
		wantJSON(t, got, `[{"type":"user_input","content":[{"type":"image","data":"AQID","mime_type":"`+want+`"}]}]`)
	}
}

func TestEveryDocumentMimeEncodes(t *testing.T) {
	asText := map[api.DocumentMime]bool{
		api.DocumentText: true, api.DocumentJSON: true, api.DocumentMarkdown: true, api.DocumentHTML: true,
		api.DocumentXML: true, api.DocumentYAML: true, api.DocumentRTF: true,
	}
	for _, mime := range []api.DocumentMime{
		api.DocumentPDF, api.DocumentText, api.DocumentJSON, api.DocumentMarkdown,
		api.DocumentCSV, api.DocumentHTML, api.DocumentXML, api.DocumentYAML,
		api.DocumentRTF, api.DocumentDOC, api.DocumentXLS, api.DocumentDOCX,
		api.DocumentXLSX, api.DocumentPPTX,
	} {
		doc := api.ContentDocument{Data: []byte("words"), Mime: string(mime), Name: "a.bin"}
		words := `{"type":"text","text":"words"}`
		want := `{"type":"document","data":"d29yZHM=","mime_type":"` + string(mime) + `"}`
		if asText[mime] {
			want = words
		}
		got := steps(t, api.Request{Messages: []api.Message{
			user(doc),
			assistant(doc, call("c1", "fetch", `{}`)),
			user(result("c1", api.ToolOk, doc)),
		}})
		wantJSON(t, got, `[{"type":"user_input","content":[`+want+`]},`+dummy+`,{"type":"model_output","content":[`+want+`]},{"type":"function_call","id":"c1","name":"fetch","arguments":{}},{"type":"function_result","name":"fetch","call_id":"c1","result":[`+want+`]}]`)
	}
}

func TestResults(t *testing.T) {
	png := api.ContentImage{Data: []byte{1}, Mime: "image/png"}
	got := steps(t, api.Request{Messages: []api.Message{
		user(text("go")),
		assistant(call("a", "alpha", `{"x":1}`), call("b", "beta", ""), call("c", "gamma", `{"y":2}`), call("d", "delta", `{}`)),
		user(
			text("before"),
			result("b", api.ToolErrRuntime, text("down")),
			result("a", api.ToolOk, text("t"), png),
			result("c", api.ToolOk),
			text("after"),
			result("d", api.ToolOk, text("")),
		),
	}})
	wantJSON(t, got, `[
		{"type":"user_input","content":[{"type":"text","text":"go"}]},
		`+dummy+`,
		{"type":"function_call","id":"a","name":"alpha","arguments":{"x":1}},
		{"type":"function_call","id":"b","name":"beta","arguments":{}},
		{"type":"function_call","id":"c","name":"gamma","arguments":{"y":2}},
		{"type":"function_call","id":"d","name":"delta","arguments":{}},
		{"type":"function_result","name":"beta","call_id":"b","result":[{"type":"text","text":"down"}],"is_error":true},
		{"type":"function_result","name":"alpha","call_id":"a","result":[{"type":"text","text":"t"},{"type":"image","data":"AQ==","mime_type":"image/png"}]},
		{"type":"function_result","name":"gamma","call_id":"c","result":[{"type":"text","text":"`+lib.EmptyResult+`"}]},
		{"type":"function_result","name":"delta","call_id":"d","result":[{"type":"text","text":"[content omitted]"}]},
		{"type":"user_input","content":[{"type":"text","text":"before"},{"type":"text","text":"after"}]}]`)
	got = steps(t, api.Request{Messages: []api.Message{
		user(text("go")),
		assistant(api.ContentToolUse{ID: "a", Name: "alpha", Input: api.DiscardedArguments}),
		user(api.NewContentToolResult("a", nil, api.DiscardedArgumentsError{})),
	}})
	if !strings.Contains(got, `"arguments":{}`) || !strings.Contains(got, `"is_error":true`) {
		t.Fatalf("a discarded call went out as %s", got)
	}
}

func TestEmptyThreadsSendInput(t *testing.T) {
	for _, msgs := range [][]api.Message{
		{user()},
		{user(text(""))},
		{user(), assistant(), user()},
	} {
		got := steps(t, api.Request{Messages: msgs})
		if got == "" || got == "[]" || got == "null" {
			t.Fatalf("%v sent input %s", msgs, got)
		}
	}
}

func TestSignatures(t *testing.T) {
	for _, c := range []struct {
		name string
		msgs []api.Message
		want string
	}{
		{"own meta replayed in place and the summary never sent", []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Summary: api.Indexed[string]{0: "secret plan"}, Meta: ownThought}, text("hello")),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + ownMeta + `,{"type":"model_output","content":[{"type":"text","text":"hello"}]}]`},
		{"foreign metas", []api.Message{
			user(text("hi")),
			assistant(
				api.ContentReasoning{Meta: json.RawMessage(`"claude-signature"`)},
				api.ContentReasoning{Meta: json.RawMessage(`{"type":"redacted_thinking","data":"x"}`)},
				api.ContentReasoning{Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"x"}`)},
				api.ContentReasoning{Meta: json.RawMessage(`{"type":"thought","signature":""}`)},
				api.ContentReasoning{Meta: json.RawMessage(ownMeta)},
				api.ContentReasoning{Meta: lib.Meta(providerName, json.RawMessage(`{"type":"thought","signature":""}`))},
				api.ContentReasoning{Meta: lib.Meta("claude", json.RawMessage(`"claude-signature"`))},
				text("hello")),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},{"type":"model_output","content":[{"type":"text","text":"hello"}]}]`},
		{"own calls get no dummy", []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Meta: ownThought}, call("a", "x", `{}`), call("b", "x", `{}`)),
			user(result("a", api.ToolOk, text("1")), result("b", api.ToolOk, text("2"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + ownMeta + `,{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_call","id":"b","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]},{"type":"function_result","name":"x","call_id":"b","result":[{"type":"text","text":"2"}]}]`},
		{"one dummy for parallel calls", []api.Message{
			user(text("hi")),
			assistant(call("a", "x", `{}`), call("b", "x", `{}`)),
			user(result("a", api.ToolOk, text("1")), result("b", api.ToolOk, text("2"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_call","id":"b","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]},{"type":"function_result","name":"x","call_id":"b","result":[{"type":"text","text":"2"}]}]`},
		{"the dummy opens a foreign turn that has text before its call", []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Meta: json.RawMessage(`"claude-signature"`)}, text("Checking."), call("a", "x", `{}`)),
			user(result("a", api.ToolOk, text("1"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"model_output","content":[{"type":"text","text":"Checking."}]},{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]}]`},
		{"text between calls goes after the last one", []api.Message{
			user(text("hi")),
			assistant(text("first"), call("a", "x", `{}`), text("and"), call("b", "x", `{}`), text("last")),
			user(result("a", api.ToolOk, text("1")), result("b", api.ToolOk, text("2"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"model_output","content":[{"type":"text","text":"first"}]},{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_call","id":"b","name":"x","arguments":{}},{"type":"model_output","content":[{"type":"text","text":"and"},{"type":"text","text":"last"}]},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]},{"type":"function_result","name":"x","call_id":"b","result":[{"type":"text","text":"2"}]}]`},
		{"a call with array input goes wrapped between the others", []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Meta: ownThought}, call("a", "x", `{}`), call("n", "x", `[1]`), api.ContentReasoning{Meta: ownThought}, call("b", "x", `{}`)),
			user(result("a", api.ToolOk, text("1")), result("n", api.ToolOk, text("0")), result("b", api.ToolOk, text("2"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + ownMeta + `,{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_call","id":"n","name":"x","arguments":{"input":[1]}},` + ownMeta + `,{"type":"function_call","id":"b","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]},{"type":"function_result","name":"x","call_id":"n","result":[{"type":"text","text":"0"}]},{"type":"function_result","name":"x","call_id":"b","result":[{"type":"text","text":"2"}]}]`},
		{"a dummy in a previous turn too", []api.Message{
			user(text("hi")),
			assistant(call("a", "x", `{}`)),
			user(result("a", api.ToolOk, text("1"))),
			assistant(text("done")),
			user(text("thanks")),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]},{"type":"model_output","content":[{"type":"text","text":"done"}]},{"type":"user_input","content":[{"type":"text","text":"thanks"}]}]`},
		{"an own thought after text does not open the turn", []api.Message{
			user(text("hi")),
			assistant(text("Checking."), api.ContentReasoning{Meta: ownThought}, call("a", "x", `{}`)),
			user(result("a", api.ToolOk, text("1"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"model_output","content":[{"type":"text","text":"Checking."}]},` + ownMeta + `,{"type":"function_call","id":"a","name":"x","arguments":{}},{"type":"function_result","name":"x","call_id":"a","result":[{"type":"text","text":"1"}]}]`},
		{"a nameless call gets the dummy and a placeholder name", []api.Message{
			user(text("hi")),
			assistant(call("a", "", `{}`), text("x")),
			user(result("a", api.ToolOk, text("1"))),
		}, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},` + dummy + `,{"type":"function_call","id":"a","name":"` + lib.CallName("") + `","arguments":{}},{"type":"model_output","content":[{"type":"text","text":"x"}]},{"type":"function_result","name":"` + lib.CallName("") + `","call_id":"a","result":[{"type":"text","text":"1"}]}]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantJSON(t, steps(t, api.Request{Messages: c.msgs}), c.want)
		})
	}
}

func TestEncodingIsStableAsTheThreadGrows(t *testing.T) {
	msgs := []api.Message{
		user(text("hi")),
		assistant(api.ContentReasoning{Meta: lib.Meta("claude", json.RawMessage(`"claude"`))}, text("Checking."), call("a", "x", `{"k": 1}`)),
		user(result("a", api.ToolOk, text("1"))),
		assistant(api.ContentReasoning{Meta: ownThought}, text("done")),
	}
	short := steps(t, api.Request{Messages: msgs})
	long := steps(t, api.Request{Messages: append(slices.Clone(msgs), user(text("more")))})
	if again := steps(t, api.Request{Messages: msgs}); again != short {
		t.Fatalf("two encodes differ\n%s\n%s", short, again)
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(short, "]")) {
		t.Fatalf("the grown thread does not start with the old one\n%s\n%s", short, long)
	}
}

func TestOpaque(t *testing.T) {
	own := `{"arguments":{"queries":["cats"]},"id":"gs_1","signature":"c2ln","type":"google_search_call"}`
	got := steps(t, api.Request{Messages: []api.Message{
		user(text("hi")),
		assistant(
			api.ContentOpaque{Kind: "gemini.google_search_call", Data: json.RawMessage(own)},
			api.ContentOpaque{Kind: "web_search_tool_result", Data: json.RawMessage(`{"type":"web_search_tool_result","tool_use_id":"s"}`)},
			api.ContentOpaque{Kind: "file_search_call", Data: json.RawMessage(`{"id":"fs_1","type":"file_search_call"}`)},
			api.ContentOpaque{Kind: "gemini.file_search_call", Data: json.RawMessage(`{"id":"fs_1","type":"google_search_call"}`)},
			api.ContentOpaque{Kind: "gemini.", Data: json.RawMessage(`{"type":""}`)},
			text("found"),
		),
	}})
	wantJSON(t, got, `[{"type":"user_input","content":[{"type":"text","text":"hi"}]},`+own+`,{"type":"model_output","content":[{"type":"text","text":"found"}]}]`)
	if !strings.Contains(got, own) {
		t.Fatalf("own step not replayed byte for byte: %s", got)
	}
}

func TestReplayReproducesTheTurnByteForByte(t *testing.T) {
	data, err := os.ReadFile("testdata/streams/capture-fc_one.sse")
	if err != nil {
		t.Fatal(err)
	}
	events := collect(t, serve(t, sseServer(string(data))), turn("What is the weather in Paris right now? Use the tool."))
	conform.CheckGrammar(t, events)
	b := response.NewBulder()
	for _, e := range events {
		if _, ok := b.On(e); !ok {
			t.Fatalf("builder refused %s", conform.Describe(e))
		}
	}
	content := b.Flush().Content.Ordered()
	var callID string
	for _, c := range content {
		if u, ok := c.(api.ContentToolUse); ok {
			callID = u.ID
		}
	}
	got := steps(t, api.Request{Messages: []api.Message{
		user(text("What is the weather in Paris right now? Use the tool.")),
		{Role: api.RoleAssistant, Content: content},
		user(result(callID, api.ToolOk, text(`{"temp_c":14}`))),
	}})
	wantJSON(t, got, `[{"type":"user_input","content":[{"type":"text","text":"What is the weather in Paris right now? Use the tool."}]},
		{"type":"thought","signature":"c2NydWJiZWQ="},
		{"type":"function_call","id":"`+callID+`","name":"get_weather","arguments":{"city":"Paris"}},
		{"type":"function_result","name":"get_weather","call_id":"`+callID+`","result":[{"type":"text","text":"{\"temp_c\":14}"}]}]`)
	if !strings.Contains(got, `"arguments":{"city":"Paris"}`) || !strings.Contains(got, `{"type":"thought","signature":"c2NydWJiZWQ="}`) {
		t.Fatalf("the replay is not byte for byte: %s", got)
	}
}

func TestRequestOverModelRequestBytes(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	p := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: 10}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	events := collect(t, p, turn("hello there"))
	if len(events) != 1 || hits != 0 {
		t.Fatalf("%v with %d requests sent, want one EventErr and none", conform.DescribeAll(events), hits)
	}
	if e, ok := events[0].(api.EventErr); !ok || e.Code != api.CodeTooLarge || !errors.Is(e.Err, errTooLarge) {
		t.Fatalf("%s, want EventErr too_large", conform.Describe(events[0]))
	}
}

func TestRefusalBySizeNamesTheSizesOnlyWhenMeasured(t *testing.T) {
	for _, c := range []struct {
		declared int
		measured bool
	}{{1 << 20, true}, {0, false}} {
		var sent int
		p := serve(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sent = len(body)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			io.WriteString(w, "payload too large")
		})
		p.model.RequestBytes = c.declared
		events := collect(t, p, turn("hi"))
		e, ok := events[len(events)-1].(api.EventErr)
		if !ok || e.Code != api.CodeTooLarge {
			t.Fatalf("declared %d: %s, want too_large", c.declared, conform.Describe(events[len(events)-1]))
		}
		msg := e.Err.Error()
		if c.measured && (sent == 0 || !strings.Contains(msg, fmt.Sprintf("sent %d bytes", sent)) || !strings.Contains(msg, fmt.Sprintf("the %d declared", c.declared))) {
			t.Fatalf("error %q names neither the %d bytes sent nor the %d declared", msg, sent, c.declared)
		}
		if !c.measured && strings.Contains(msg, "the 0 declared") {
			t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
		}
		var a *APIError
		if !errors.As(e.Err, &a) || a.Status != http.StatusRequestEntityTooLarge || a.Message != "payload too large" {
			t.Fatalf("declared %d: err %v carries no 413 APIError", c.declared, e.Err)
		}
	}
}
