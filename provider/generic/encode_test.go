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
	"log/slog"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func provider(t *testing.T, opts ...Option) *Provider {
	t.Helper()
	return mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: "http://127.0.0.1:8080/v1"}, opts...)
}

func encodeJSON(t *testing.T, p *Provider, req api.Request) string {
	t.Helper()
	body, err := p.encode(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	raw, err := lib.MarshalRaw(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func messagesJSON(t *testing.T, p *Provider, msgs ...api.Message) string {
	t.Helper()
	body, err := p.encode(api.Request{Messages: msgs})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	raw, err := lib.MarshalRaw(body.Messages)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
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

func reasoning(meta string, thoughts ...string) api.ContentReasoning {
	r := api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{}}
	for i, s := range thoughts {
		r.Thoughts[i] = s
	}
	if meta != "" {
		r.Meta = json.RawMessage(meta)
	}
	return r
}

func TestEncodeShape(t *testing.T) {
	p := provider(t, WithEffort(EffortLow), WithMaxTokens(4096), WithTemperature(0.6), WithTopP(0.95), WithStopSequences("END"))
	got := encodeJSON(t, p, api.Request{
		System: "Be brief.",
		Tools: []api.Tool{
			{Name: "get_weather", Desc: "Weather by city.", Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}},
			{Name: "now"},
		},
		ToolChoice: api.ToolChoiceAuto,
		Messages:   []api.Message{user(text("hi <b> & you"))},
	})
	want := `{"model":"qwen/qwen3.8-27b","tools":[{"type":"function","function":{"name":"get_weather","description":"Weather by city.","parameters":{"properties":{"city":{"type":"string"}},"type":"object"}}},` +
		`{"type":"function","function":{"name":"now","parameters":{"properties":{},"type":"object"}}}],"tool_choice":"auto",` +
		`"messages":[{"role":"system","content":"Be brief."},{"role":"user","content":"hi <b> & you"}],` +
		`"stream":true,"stream_options":{"include_usage":true},"max_tokens":4096,"reasoning_effort":"low","temperature":0.6,"top_p":0.95,"stop":["END"]}`
	if got != want {
		t.Fatalf("body\n%s\nwant\n%s", got, want)
	}
	if got := encodeJSON(t, provider(t), turn("hi")); got != `{"model":"qwen/qwen3.8-27b","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true}}` {
		t.Fatalf("default body %s", got)
	}
}

func TestEncodeToolChoice(t *testing.T) {
	for choice, want := range map[api.ToolChoice]string{
		api.ToolChoiceAuto: `"auto"`,
		api.ToolChoiceNone: `"none"`,
		api.ToolChoiceAny:  `"required"`,
		"get_weather":      `{"type":"function","function":{"name":"get_weather"}}`,
	} {
		got := encodeJSON(t, provider(t, WithEffort(EffortHigh)), api.Request{Tools: []api.Tool{{Name: "get_weather"}}, ToolChoice: choice, Messages: []api.Message{user(text("hi"))}})
		if !strings.Contains(got, `"tool_choice":`+want+`,`) {
			t.Errorf("%q: %s", choice, got)
		}
		if got := encodeJSON(t, provider(t), api.Request{ToolChoice: choice, Messages: []api.Message{user(text("hi"))}}); strings.Contains(got, "tool_choice") {
			t.Errorf("%q with no tools: %s", choice, got)
		}
	}
}

func TestEncodeRefuses(t *testing.T) {
	p := provider(t)
	for name, req := range map[string]api.Request{
		"no messages":     {},
		"a nameless tool": {Tools: []api.Tool{{Desc: "x"}}, Messages: []api.Message{user(text("hi"))}},
		"a system role":   {Messages: []api.Message{{Role: "system", Content: api.Contents{text("x")}}}},
		"discarded arguments with no failed result": {Messages: []api.Message{
			user(text("go")),
			assistant(api.ContentToolUse{ID: "c1", Name: "now", Input: api.DiscardedArguments}),
			user(api.ContentToolResult{ID: "c1", Content: api.UserContents{text("x")}}),
		}},
	} {
		if _, err := p.encode(req); err == nil || !strings.HasPrefix(err.Error(), "generic: ") {
			t.Errorf("%s: encode returned %v", name, err)
		}
		events := collect(t, p, req)
		wantEvents(t, events, "err invalid")
	}
}

func TestEncodeToolTurns(t *testing.T) {
	p := provider(t)
	got := messagesJSON(t, p,
		user(text("weather?")),
		assistant(text("Checking."), api.ContentToolUse{ID: "c1", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)}, api.ContentToolUse{ID: "c2", Name: "now"}),
		user(text("and hurry"), api.ContentToolResult{ID: "c2", Content: api.UserContents{text("noon")}}, api.ContentToolResult{ID: "c1", Content: api.UserContents{text("12C"), text("clear")}, Error: api.ToolErrRuntime}),
		assistant(api.ContentToolUse{ID: "c3", Name: "now", Input: json.RawMessage(`{}`)}),
		user(api.ContentToolResult{ID: "c3"}, api.ContentToolResult{ID: "c1", Content: api.UserContents{text("late")}}),
	)
	want := `[{"role":"user","content":"weather?"},` +
		`{"role":"assistant","content":"Checking.","tool_calls":[{"id":"c1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}},{"id":"c2","type":"function","function":{"name":"now","arguments":"{}"}}]},` +
		`{"role":"tool","content":"noon","tool_call_id":"c2"},{"role":"tool","content":"Error: 12C\n\nclear","tool_call_id":"c1"},{"role":"user","content":"and hurry"},` +
		`{"role":"assistant","content":null,"tool_calls":[{"id":"c3","type":"function","function":{"name":"now","arguments":"{}"}}]},` +
		`{"role":"tool","content":"(the tool returned no output)","tool_call_id":"c3"},{"role":"user","content":"[content omitted]"}]`
	if got != want {
		t.Fatalf("messages\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeDiscardedArguments(t *testing.T) {
	got := messagesJSON(t, provider(t),
		user(text("go")),
		assistant(api.ContentToolUse{ID: "c1", Name: "lookup", Input: api.DiscardedArguments}, api.ContentToolUse{ID: "c2", Name: "lookup", Input: json.RawMessage(`{"q":`)}),
		user(api.NewContentToolResult("c1", nil, api.DiscardedArgumentsError{Raw: `{"q":`}), api.ContentToolResult{ID: "c2", Error: api.ToolErrJSON}),
	)
	if strings.Count(got, `"arguments":"{}"`) != 2 || strings.Contains(got, "discarded-arguments") {
		t.Fatalf("messages %s", got)
	}
}

func TestEncodeResultsPairPositionally(t *testing.T) {
	got := messagesJSON(t, provider(t),
		user(api.ContentToolResult{ID: "c0", Content: api.UserContents{text("orphan")}}),
		assistant(api.ContentToolUse{ID: "c1", Name: "now"}, api.ContentToolUse{Name: "now"}, api.ContentToolUse{ID: "c3"}),
		user(text("first")),
		user(api.ContentToolResult{ID: "c1", Content: api.UserContents{text("late")}}, api.ContentToolResult{Content: api.UserContents{text("no id")}}),
		assistant(api.ContentToolUse{Name: "now"}),
		user(api.ContentToolResult{Content: api.UserContents{text("no id")}}),
	)
	want := `[{"role":"user","content":"[content omitted]"},` +
		`{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"now","arguments":"{}"}},` +
		`{"id":"` + lib.CallID("") + `","type":"function","function":{"name":"now","arguments":"{}"}},` +
		`{"id":"c3","type":"function","function":{"name":"` + lib.CallName("") + `","arguments":"{}"}}]},` +
		`{"role":"user","content":"first"},{"role":"user","content":"[content omitted]\n\n[content omitted]"},` +
		`{"role":"assistant","content":null,"tool_calls":[{"id":"` + lib.CallID("") + `","type":"function","function":{"name":"now","arguments":"{}"}}]},` +
		`{"role":"tool","content":"no id","tool_call_id":"` + lib.CallID("") + `"}]`
	if got != want {
		t.Fatalf("messages\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeReasoningReplay(t *testing.T) {
	got := messagesJSON(t, provider(t),
		user(text("hi")),
		assistant(reasoning(`{"provider":"generic","data":"reasoning"}`, "Say", " hi."), text("Hi")),
		user(text("again"), reasoning(`{"provider":"generic","data":"reasoning"}`, "user side")),
		assistant(reasoning(`{"provider":"generic","data":"reasoning_content"}`, "Again."), api.ContentToolUse{ID: "c1", Name: "now"}),
		user(api.ContentToolResult{ID: "c1", Content: api.UserContents{text("noon")}}),
		assistant(reasoning(`{"provider":"deepseek"}`, "foreign"), reasoning("", "no meta"), reasoning(`"c2ln"`, "signed elsewhere"), text("It is noon.")),
		user(text("thanks")),
		assistant(reasoning(`{"provider":"generic","data":"reasoning"}`, "only thought")),
	)
	want := `[{"role":"user","content":"hi"},{"role":"assistant","content":"Hi","reasoning":"Say hi."},{"role":"user","content":"again"},` +
		`{"role":"assistant","content":null,"reasoning_content":"Again.","tool_calls":[{"id":"c1","type":"function","function":{"name":"now","arguments":"{}"}}]},` +
		`{"role":"tool","content":"noon","tool_call_id":"c1"},{"role":"assistant","content":"It is noon."},{"role":"user","content":"thanks"},` +
		`{"role":"assistant","content":"[empty]","reasoning":"only thought"}]`
	if got != want {
		t.Fatalf("messages\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeEmptyMessages(t *testing.T) {
	got := messagesJSON(t, provider(t), user(), assistant(reasoning(`{"provider":"gpt"}`, "x")), user(reasoning("", "y")))
	if got != `[{"role":"user","content":"[empty]"},{"role":"assistant","content":"[empty]"},{"role":"user","content":"[empty]"}]` {
		t.Fatalf("messages %s", got)
	}
}

func TestEncodeMedia(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	got := messagesJSON(t, provider(t),
		user(
			text("look"),
			api.ContentImage{Data: png, Mime: " IMAGE/JPG "},
			api.ContentImage{Data: png, Mime: "image/bmp"},
			api.ContentImage{Mime: "image/png"},
			api.ContentDocument{Data: []byte("a,b\n1,2"), Mime: "Text/CSV", Name: "t.csv"},
			api.ContentDocument{Data: []byte("%PDF-1.4"), Mime: "application/pdf"},
			api.ContentDocument{Data: []byte{0xff, 0xfe}, Mime: "text/plain"},
			api.ContentDocument{Mime: "text/plain"},
		),
		assistant(api.ContentImage{Data: png, Mime: "image/png"}, api.ContentDocument{Data: []byte("# notes"), Mime: "text/markdown"}, api.ContentDocument{Data: []byte("x"), Mime: "application/zip"}, api.ContentToolUse{ID: "c1", Name: "shot"}),
		user(api.ContentToolResult{ID: "c1", Content: api.UserContents{api.ContentImage{Data: png, Mime: "image/png"}, api.ContentDocument{Data: []byte("{}"), Mime: "application/json"}}, Error: api.ToolErrRuntime}),
	)
	want := `[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"data:image/jpg;base64,iVBORw0KGgo="}},` +
		`{"type":"text","text":"[image omitted]"},{"type":"text","text":"[image omitted]"},{"type":"text","text":"a,b\n1,2"},` +
		`{"type":"text","text":"[document omitted]"},{"type":"text","text":"[document omitted]"},{"type":"text","text":"[document omitted]"}]},` +
		`{"role":"assistant","content":"[image omitted]\n\n# notes\n\n[document omitted]","tool_calls":[{"id":"c1","type":"function","function":{"name":"shot","arguments":"{}"}}]},` +
		`{"role":"tool","content":[{"type":"text","text":"Error:"},{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}},{"type":"text","text":"{}"}],"tool_call_id":"c1"}]`
	if got != want {
		t.Fatalf("messages\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeNotesOtherBlocks(t *testing.T) {
	opaque := api.ContentOpaque{Kind: "gpt.web_search_call", Data: json.RawMessage(`{"type":"web_search_call"}`)}
	got := messagesJSON(t, provider(t),
		user(api.ContentToolUse{ID: "c1", Name: "now"}, opaque),
		assistant(opaque, api.ContentToolResult{ID: "c1"}),
	)
	if got != `[{"role":"user","content":"[content omitted]"},{"role":"assistant","content":"[content omitted]"}]` {
		t.Fatalf("messages %s", got)
	}
}

func TestReplayReproducesTheTurn(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"reasoning_content":"Need the time."}`, ""),
		chunkFrame(`{"content":"Let me check."}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"now","arguments":"{}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("time?"))
	var content api.Contents
	var meta json.RawMessage
	var thought, said string
	for _, e := range events {
		switch e := e.(type) {
		case api.EventThinking:
			thought += e.Text
		case api.EventMeta:
			meta = e.Data
		case api.EventText:
			said += e.Text
		}
	}
	content = api.Contents{api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{0: thought}, Meta: meta}, text(said), api.ContentToolUse{ID: "call_1", Name: "now", Input: json.RawMessage(`{}`)}}
	got := messagesJSON(t, p, user(text("time?")), assistant(content...), user(api.ContentToolResult{ID: "call_1", Content: api.UserContents{text("noon")}}))
	want := `[{"role":"user","content":"time?"},{"role":"assistant","content":"Let me check.","reasoning_content":"Need the time.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"now","arguments":"{}"}}]},{"role":"tool","content":"noon","tool_call_id":"call_1"}]`
	if got != want {
		t.Fatalf("replay\n%s\nwant\n%s", got, want)
	}
}

type levelCounter map[slog.Level]*int

func (c levelCounter) Enabled(context.Context, slog.Level) bool { return true }

func (c levelCounter) Handle(_ context.Context, r slog.Record) error {
	if n, ok := c[r.Level]; ok {
		*n++
	}
	return nil
}

func (c levelCounter) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c levelCounter) WithGroup(string) slog.Handler { return c }

type strange struct{ api.ContentText }

func TestEncodeLogging(t *testing.T) {
	var errs, others int
	counter := levelCounter{slog.LevelError: &errs, slog.LevelWarn: &others, slog.LevelInfo: &others, slog.LevelDebug: &others}
	prev := slog.Default()
	slog.SetDefault(slog.New(counter))
	defer slog.SetDefault(prev)
	p := provider(t)
	messagesJSON(t, p, user(api.ContentImage{Mime: "image/png"}, api.ContentDocument{Mime: "application/zip", Data: []byte("x")}), assistant(api.ContentImage{}))
	if errs+others != 0 {
		t.Fatalf("an omission logged %d errors and %d others", errs, others)
	}
	for range 3 {
		got := messagesJSON(t, p, user(strange{}), assistant(strange{}))
		if got != `[{"role":"user","content":"[content omitted]"},{"role":"assistant","content":"[content omitted]"}]` {
			t.Fatalf("messages %s", got)
		}
	}
	if errs > 1 || others != 0 {
		t.Fatalf("an unknown type logged %d errors and %d others, want at most one error", errs, others)
	}
}
