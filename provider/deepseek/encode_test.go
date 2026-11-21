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
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func encodeBody(t *testing.T, p *Provider, req api.Request) *chatRequest {
	t.Helper()
	body, err := p.encode(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return body
}

func encodeJSON(t *testing.T, p *Provider, req api.Request) string {
	t.Helper()
	raw, err := json.Marshal(encodeBody(t, p, req))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func encodeError(t *testing.T, p *Provider, req api.Request) error {
	t.Helper()
	_, err := p.encode(req)
	if err == nil {
		t.Fatal("expected the turn to be refused")
	}
	if !strings.HasPrefix(err.Error(), "deepseek: ") {
		t.Fatalf("error %q does not name the provider", err)
	}
	return err
}

func text(s string) api.Contents {
	return api.Contents{api.ContentText{Text: s}}
}

// The system prompt is a leading message rather than a field, several text
// blocks in one message become one string, and streaming with usage is not
// optional because it is the only way the turn is priced.
func TestEncodeShape(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	body := encodeBody(t, p, api.Request{
		System: "be terse\nanswer in English",
		Messages: []api.Message{{
			Role:    api.RoleUser,
			Content: api.Contents{api.ContentText{Text: "one"}, api.ContentText{Text: "two"}},
		}},
	})
	if !body.Stream || body.StreamOptions == nil || !body.StreamOptions.IncludeUsage {
		t.Fatal("stream and stream_options.include_usage must both be on")
	}
	if len(body.Messages) != 2 {
		t.Fatalf("messages %+v", body.Messages)
	}
	if body.Messages[0].Role != "system" || body.Messages[0].Content.(string) != "be terse\nanswer in English" {
		t.Fatalf("system message %+v", body.Messages[0])
	}
	if body.Messages[1].Role != "user" || body.Messages[1].Content.(string) != "one\n\ntwo" {
		t.Fatalf("user message %+v", body.Messages[1])
	}
	if body.MaxTokens != 0 {
		t.Fatal("no cap was asked for and DeepSeek publishes no default, so none should be sent")
	}
}

// The cap is the Provider's and a request cannot carry one, so there is nothing
// to resolve between two of them. A value over the documented ceiling is the
// option's error rather than a number quietly lowered: New logs it and the knob
// keeps the value it had, which is a line at construction instead of a turn the
// caller believes was capped where it asked.
func TestEncodeMaxTokens(t *testing.T) {
	tests := []struct {
		name string
		opt  int
		want int
	}{
		{"unset", 0, 0},
		{"configured", 4096, 4096},
		{"at the documented ceiling", maxOutputTokens, maxOutputTokens},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"), WithMaxTokens(tc.opt))
			body := encodeBody(t, p, api.Request{
				Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			if body.MaxTokens != tc.want {
				t.Fatalf("max_tokens %d, want %d", body.MaxTokens, tc.want)
			}
		})
	}
}

// deepseek-v4-pro reads text and nothing else, and a turn carrying anything else
// is a thread that touched another provider or an adapter that never grew a
// path for it. The note goes out in the block's place: refusing would end
// the conversation over stored history the caller cannot rewrite, and dropping
// the block silently would tell the model a turn it cannot see never happened.
func TestEncodeNotesWhatItCannotSend(t *testing.T) {
	image := api.ContentImage{Data: []byte("x"), Mime: "image/png"}
	doc := api.ContentDocument{Data: []byte("x"), Mime: "application/pdf"}
	tests := []struct {
		name string
		msg  api.Message
		want string
	}{
		{"image in a user turn", api.Message{Role: api.RoleUser, Content: api.Contents{image}}, "[image omitted]"},
		{"document in a user turn", api.Message{Role: api.RoleUser, Content: api.Contents{doc}}, "[document omitted]"},
		// An attachment whose bytes are gone reaches the same arm rather than any
		// part of this wire, which is what keeps one stored block from failing
		// every later turn of a thread that is stored and replayed.
		{
			"an image with no bytes in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentImage{Mime: "image/png"}}},
			"[image omitted]",
		},
		{
			"a document with no bytes in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentDocument{Mime: "application/pdf"}}},
			"[document omitted]",
		},
		{
			"a text document with no bytes in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentDocument{Mime: "text/plain"}}},
			"[document omitted]",
		},
		{
			"a text document that is not UTF-8 in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentDocument{Data: []byte{0xff, 0xfe, 'a'}, Mime: "text/plain"}}},
			"[document omitted]",
		},
		{"image in an assistant turn", api.Message{Role: api.RoleAssistant, Content: api.Contents{image}}, "[image omitted]"},
		{"document in an assistant turn", api.Message{Role: api.RoleAssistant, Content: api.Contents{doc}}, "[document omitted]"},
		{
			"text document in an assistant turn goes as its text",
			api.Message{Role: api.RoleAssistant, Content: api.Contents{api.ContentDocument{Data: []byte("x"), Mime: " Text/Plain"}, api.ContentText{Text: "here"}}},
			"x\n\nhere",
		},
		{
			"tool call in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentToolUse{ID: "t1", Name: "x"}}},
			"[content omitted]",
		},
		{
			"reasoning in a user turn",
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentReasoning{Thoughts: map[int]string{0: "x"}}}},
			"[reasoning omitted]",
		},
		{
			"tool result in an assistant turn",
			api.Message{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolResult{ID: "t1"}}},
			"[content omitted]",
		},
		{
			"the rest of the turn survives beside the note",
			api.Message{Role: api.RoleUser, Content: api.Contents{image, api.ContentText{Text: "what is it"}}},
			"[image omitted]\n\nwhat is it",
		},
		{
			"an assistant note stands on its own line rather than against the model's words",
			api.Message{Role: api.RoleAssistant, Content: api.Contents{image, api.ContentText{Text: "here it is"}}},
			"[image omitted]\n\nhere it is",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
				Messages: []api.Message{tc.msg},
			})
			if len(body.Messages) != 1 {
				t.Fatalf("messages %d, want 1", len(body.Messages))
			}
			if got := body.Messages[0].Content.(string); got != tc.want {
				t.Fatalf("content %q, want %q", got, tc.want)
			}
		})
	}
}

// deepseek-v4-pro takes a result as text only, so an attachment inside one goes
// the way an attachment in a message goes: the note in its place. Refusing
// delivers the image no more than the note does, and the result is the answer a
// call is waiting on, so failing it fails the conversation as well.
func TestEncodeNotesAttachmentsInAToolResult(t *testing.T) {
	image := api.ContentImage{Data: []byte("x"), Mime: "image/png"}
	doc := api.ContentDocument{Data: []byte("x"), Mime: "application/pdf"}
	tests := []struct {
		name string
		rc   api.UserContent
		want string
	}{
		{"image", image, "the chart:\n\n[image omitted]\n\nthat is all"},
		{"document", doc, "the chart:\n\n[document omitted]\n\nthat is all"},
		{"an image with no bytes", api.ContentImage{Mime: "image/png"}, "the chart:\n\n[image omitted]\n\nthat is all"},
		{"a document with no bytes", api.ContentDocument{Mime: "application/pdf"}, "the chart:\n\n[document omitted]\n\nthat is all"},
		{"a text document that is not UTF-8", api.ContentDocument{Data: []byte{0xff, 0xfe, 'a'}, Mime: "text/csv"}, "the chart:\n\n[document omitted]\n\nthat is all"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
				Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
					api.ContentToolResult{ID: "t1", Content: api.UserContents{
						api.ContentText{Text: "the chart:"},
						tc.rc,
						api.ContentText{Text: "that is all"},
					}},
				}}},
			})
			if len(body.Messages) != 1 {
				t.Fatalf("messages %+v", body.Messages)
			}
			m := body.Messages[0]
			if m.Role != "tool" || m.ToolCallID != "t1" {
				t.Fatalf("result message %+v", m)
			}
			if got := m.Content.(string); got != tc.want {
				t.Fatalf("content %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEveryDocumentMimeEncodes(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	asText := map[api.DocumentMime]bool{
		api.DocumentText: true, api.DocumentJSON: true, api.DocumentMarkdown: true, api.DocumentCSV: true,
		api.DocumentHTML: true, api.DocumentXML: true, api.DocumentYAML: true, api.DocumentRTF: true,
	}
	for _, mime := range []api.DocumentMime{
		api.DocumentPDF, api.DocumentText, api.DocumentJSON, api.DocumentMarkdown,
		api.DocumentCSV, api.DocumentHTML, api.DocumentXML, api.DocumentYAML,
		api.DocumentRTF, api.DocumentDOC, api.DocumentXLS, api.DocumentDOCX,
		api.DocumentXLSX, api.DocumentPPTX, "application/zip", " Text/Markdown ",
	} {
		doc := api.ContentDocument{Data: []byte("a,b\n1,2"), Mime: string(mime), Name: "a.bin"}
		want := "[document omitted]"
		if asText[mime] || mime == " Text/Markdown " {
			want = "a,b\n1,2"
		}
		body := encodeBody(t, p, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
			api.ContentToolResult{ID: "t1", Content: api.UserContents{doc}},
			api.ContentText{Text: "read it"},
			doc,
		}}}})
		if len(body.Messages) != 2 {
			t.Fatalf("%q: messages %+v", mime, body.Messages)
		}
		if got := body.Messages[0].Content.(string); got != want {
			t.Fatalf("%q: tool content %q, want %q", mime, got, want)
		}
		if got := body.Messages[1].Content.(string); got != "read it\n\n"+want {
			t.Fatalf("%q: user content %q, want %q", mime, got, "read it\n\n"+want)
		}
	}
}

// Tool results are their own role, one message each, and they come out before
// any user text in the same message: a result has to follow the assistant turn
// that asked for it with nothing in between.
func TestEncodeToolResults(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	body := encodeBody(t, p, api.Request{
		Messages: []api.Message{{
			Role: api.RoleUser,
			Content: api.Contents{
				api.ContentText{Text: "and now?"},
				api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "24C"}}},
				api.ContentToolResult{ID: "call_b", Content: api.UserContents{api.ContentText{Text: "boom"}}, Error: api.ToolErrRuntime},
			},
		}},
	})
	if len(body.Messages) != 3 {
		t.Fatalf("messages %+v", body.Messages)
	}
	if body.Messages[0].Role != "tool" || body.Messages[0].ToolCallID != "call_a" || body.Messages[0].Content.(string) != "24C" {
		t.Fatalf("first result %+v", body.Messages[0])
	}
	// There is no error flag on a tool message, so a failed result says so in
	// the text or the model never learns it failed.
	if body.Messages[1].Content.(string) != "Error: boom" {
		t.Fatalf("failed result %q", body.Messages[1].Content.(string))
	}
	if body.Messages[2].Role != "user" || body.Messages[2].Content.(string) != "and now?" {
		t.Fatalf("trailing user message %+v", body.Messages[2])
	}
}

func TestEncodeEmptyToolResult(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	body := encodeBody(t, p, api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
			api.ContentToolResult{ID: "call_a"},
			api.ContentToolResult{ID: "call_b", Error: api.ToolErrRuntime},
		}}},
	})
	if len(body.Messages) != 2 {
		t.Fatalf("messages %+v", body.Messages)
	}
	if got := body.Messages[0].Content.(string); got != lib.EmptyResult {
		t.Fatalf("empty result %q", got)
	}
	if got := body.Messages[1].Content.(string); got != "Error: " {
		t.Fatalf("empty failed result %q", got)
	}
}

// An assistant turn is one message with sibling fields, not a block array: the
// thinking, the text and the calls all belong to the same object. A message
// carrying several text or several reasoning blocks came from a caller that
// merged turns, since one turn of this adapter produces one of each. Text joins
// with the blank line every role here joins with; reasoning joins with nothing,
// into the one reasoning_content the wire has per message, since putting the
// fragments of one block back together is a different operation.
func TestEncodeAssistantCollapsesToOneMessage(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	body := encodeBody(t, p, api.Request{
		Tools: []api.Tool{{Name: "get_weather"}, {Name: "get_time"}},
		Messages: []api.Message{{
			Role: api.RoleAssistant,
			Content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "first I think"}},
				api.ContentText{Text: "let me look"},
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: ", then some more"}},
				api.ContentText{Text: "at this"},
				api.ContentToolUse{ID: "call_a", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
				api.ContentToolUse{ID: "call_b", Name: "get_time"},
			},
		}},
	})
	if len(body.Messages) != 1 {
		t.Fatalf("messages %+v", body.Messages)
	}
	m := body.Messages[0]
	if m.Role != "assistant" || m.Content.(string) != "let me look\n\nat this" {
		t.Fatalf("message %+v", m)
	}
	if m.ReasoningContent != "first I think, then some more" {
		t.Fatalf("reasoning_content %q", m.ReasoningContent)
	}
	if len(m.ToolCalls) != 2 {
		t.Fatalf("tool_calls %+v", m.ToolCalls)
	}
	if m.ToolCalls[0].Type != "function" || m.ToolCalls[0].Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("first call %+v", m.ToolCalls[0])
	}
	// Nothing means the call takes no arguments; the field is still required to
	// be JSON.
	if m.ToolCalls[1].Function.Arguments != "{}" {
		t.Fatalf("a call with no arguments encoded as %q", m.ToolCalls[1].Function.Arguments)
	}
}

// The discard has to survive storage, which is where a conversation actually
// lives between turns. api.DiscardedArguments is a marker held in memory and
// never written, so the field is absent rather than null and reloads as a nil
// slice rather than as the four bytes of the literal; what survives the round
// trip is the api.ToolErrJSON on the result answering the call.
func TestEncodeDiscardedArgumentsSurviveStorage(t *testing.T) {
	stored, err := json.Marshal(api.Message{
		Role:    api.RoleAssistant,
		Content: api.Contents{api.ContentToolUse{ID: "call_trunc", Name: "search", Input: api.DiscardedArguments}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), `"input"`) {
		t.Fatalf("stored as %s; arguments that were discarded have to be absent, not null", stored)
	}
	var loaded api.Message
	if err := json.Unmarshal(stored, &loaded); err != nil {
		t.Fatal(err)
	}
	if input := loaded.Content[0].(api.ContentToolUse).Input; input != nil {
		t.Fatalf("a discarded accumulation reloaded as %s, want nothing", input)
	}
	body := encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
		Messages: []api.Message{loaded, {Role: api.RoleUser, Content: api.Contents{
			api.ContentToolResult{ID: "call_trunc", Content: api.UserContents{api.ContentText{Text: "no"}}, Error: api.ToolErrJSON},
		}}},
	})
	if got := body.Messages[0].ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("a discarded call reloaded from storage replays as %q, want %q", got, "{}")
	}
}

// The pairing api.ContentToolUse states is an invariant, so the marker reaching
// an encoder with no failed result answering it is a thread that was assembled
// wrong rather than a call to flatten: sending {} would replay as a legitimate
// call that takes no arguments, and the model would read a tool it never named
// arguments for as one it did.
func TestEncodeRejectsDiscardedArgumentsWithNoFailedResult(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	call := api.Message{
		Role:    api.RoleAssistant,
		Content: api.Contents{api.ContentToolUse{ID: "call_trunc", Name: "search", Input: api.DiscardedArguments}},
	}
	err := encodeError(t, p, api.Request{Messages: []api.Message{call}})
	if !strings.Contains(err.Error(), "call_trunc") || !strings.Contains(err.Error(), "discarded") {
		t.Fatalf("error %q names neither the call nor what is wrong with it", err)
	}
	// With the result present the marker is never read: the empty object goes
	// out and the tool message is what tells the model the call was not made.
	body := encodeBody(t, p, api.Request{Messages: []api.Message{call, {Role: api.RoleUser, Content: api.Contents{
		api.ContentToolResult{ID: "call_trunc", Content: api.UserContents{api.ContentText{Text: "not valid JSON"}}, Error: api.ToolErrJSON},
	}}}})
	if got := body.Messages[0].ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("a discarded call replays as %q, want %q", got, "{}")
	}
	if got := body.Messages[1].Content.(string); got != "Error: not valid JSON" {
		t.Fatalf("result content %q does not report the discard", got)
	}
}

func TestStoredContentTheServerRefusesIsANote(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	for name, c := range map[string]struct {
		msg  api.Message
		want string
	}{
		"call with no name": {
			api.Message{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "call_a", Input: json.RawMessage(`{}`)}}},
			`{"role":"assistant","content":"[content omitted]"}`,
		},
		"foreign reasoning": {
			api.Message{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Meta: json.RawMessage(`"sig"`), Thoughts: map[int]string{0: "sealed"}},
				api.ContentReasoning{Summary: map[int]string{0: "a summary"}},
				api.ContentText{Text: "answer"},
			}},
			`{"role":"assistant","content":"[reasoning omitted]\n\n[reasoning omitted]\n\nanswer"}`,
		},
		"result with no id": {
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{Content: api.UserContents{api.ContentText{Text: "72F"}}}}},
			`{"role":"user","content":"[content omitted]"}`,
		},
	} {
		raw := encodeJSON(t, p, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}, c.msg}})
		if !strings.Contains(raw, c.want) {
			t.Errorf("%s: body\n%s\nwant to contain\n%s", name, raw, c.want)
		}
	}
	raw := encodeJSON(t, p, api.Request{Messages: []api.Message{
		{Role: api.RoleUser, Content: text("hi")},
		{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "call_a", Input: json.RawMessage(`{}`)}}},
		{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "72F"}}}}},
	}})
	if strings.Contains(raw, `"role":"tool"`) || !strings.HasSuffix(raw, `{"role":"user","content":"[content omitted]"}],"stream":true,"stream_options":{"include_usage":true},"thinking":{"type":"enabled"}}`) {
		t.Errorf("the result answering a call that went out as a note was sent:\n%s", raw)
	}
}

func TestEncodeRefusesARequestWithNoMessages(t *testing.T) {
	err := encodeError(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{})
	if !strings.Contains(err.Error(), "no messages") {
		t.Fatalf("error %q", err)
	}
}

// A turn that only called tools sends an explicit null, which the assistant
// message schema documents and which omitting would not reproduce.
func TestEncodeAssistantContentNull(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	raw := encodeJSON(t, p, api.Request{
		Messages: []api.Message{{
			Role:    api.RoleAssistant,
			Content: api.Contents{api.ContentToolUse{ID: "call_a", Name: "x", Input: json.RawMessage(`{}`)}},
		}},
	})
	if !strings.Contains(raw, `"role":"assistant","content":null`) {
		t.Fatalf("body %s", raw)
	}
}

// Reasoning goes back on every assistant turn of a request that declares tools,
// including turns that called none, which is where the server requires it and
// answers a 400 without it, and nowhere in a request without tools, which is
// where the server ignores it.
//
// Nothing about a model decides this. Both models follow the same rule, and a
// stamp the fold layer never wrote would only turn a required field into a
// rejected request.
func TestEncodeReasoningReplay(t *testing.T) {
	tests := []struct {
		name    string
		noTools bool
		content api.Contents
		want    string
	}{
		{
			name: "a tool call turn replays it",
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "mine"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "mine",
		},
		{
			name: "another model's stamp decides nothing",
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-flash", Thoughts: map[int]string{0: "theirs"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "theirs",
		},
		{
			name: "neither does no stamp at all",
			content: api.Contents{
				api.ContentReasoning{Thoughts: map[int]string{0: "nobody's"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "nobody's",
		},
		{
			name: "a turn that only spoke replays it when the request has tools",
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "mine"}},
				api.ContentText{Text: "hi"},
			},
			want: "mine",
		},
		{
			name:    "a request without tools sends none",
			noTools: true,
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "mine"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "",
		},
		{
			// reasoning_content is the thinking, this adapter never produces a
			// summary, and there is no second field here a paraphrase could go
			// in, so one on a stored block is dropped rather than sent as the
			// model's own thinking.
			name: "a summary has no field on this wire",
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Summary: map[int]string{0: "in short"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "",
		},
		{
			name: "thinking another provider signed is not replayed as this model's",
			content: api.Contents{
				api.ContentReasoning{Model: "claude-opus-5", Thoughts: map[int]string{0: "theirs"}, Meta: json.RawMessage(`"sig"`)},
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "mine"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "mine",
		},
		{
			name: "reasoning another provider summarized is not replayed either",
			content: api.Contents{
				api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "in short"}, Thoughts: map[int]string{0: "theirs"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "",
		},
		{
			name: "several blocks join into one field",
			content: api.Contents{
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "first"}},
				api.ContentText{Text: "hi"},
				api.ContentReasoning{Model: "deepseek-v4-pro", Thoughts: map[int]string{0: "second"}},
				api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
			},
			want: "firstsecond",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := api.Request{
				Tools: []api.Tool{{Name: "t"}},
				Messages: []api.Message{{
					Role:    api.RoleAssistant,
					Content: tc.content,
				}},
			}
			if tc.noTools {
				req.Tools = nil
			}
			body := encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req)
			if got := body.Messages[0].ReasoningContent; got != tc.want {
				t.Fatalf("reasoning_content %q, want %q", got, tc.want)
			}
		})
	}
}

// This adapter issues no opaque blocks, since DeepSeek runs no tools of its own
// and seals nothing, so one met on replay has nowhere to go on this wire and
// leaves a note. The note stands apart from the model's own text, because an
// assistant turn's text joins with the blank line every other role's joins
// with.
func TestEncodeNotesOpaque(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	body := encodeBody(t, p, api.Request{
		Messages: []api.Message{{
			Role: api.RoleAssistant,
			Content: api.Contents{
				api.ContentOpaque{
					Kind:   "web_search_call",
					Data:   json.RawMessage(`{"x":1}`),
					Deltas: []string{`{"q":`, `"go"}`},
				},
				api.ContentText{Text: "hi"},
			},
		}},
	})
	if len(body.Messages) != 1 {
		t.Fatalf("messages %d, want 1", len(body.Messages))
	}
	if got := body.Messages[0].Content.(string); got != "[content omitted]\n\nhi" {
		t.Fatalf("content %q", got)
	}
}

func TestEncodeTools(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	schema := map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}
	body := encodeBody(t, p, api.Request{
		Tools:    []api.Tool{{Name: "get_weather", Desc: "look it up", Schema: schema}},
		Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	})
	if len(body.Tools) != 1 {
		t.Fatalf("tools %+v", body.Tools)
	}
	fn := body.Tools[0].Function
	if body.Tools[0].Type != "function" || fn.Name != "get_weather" || fn.Description != "look it up" {
		t.Fatalf("tool %+v", body.Tools[0])
	}
	if _, ok := fn.Parameters.(map[string]any); !ok {
		t.Fatalf("schema went through as %T", fn.Parameters)
	}
}

func TestEncodeToolChoice(t *testing.T) {
	tests := []struct {
		choice api.ToolChoice
		want   string
	}{
		{"", ""},
		{api.ToolChoiceAuto, `"tool_choice":"auto"`},
		{api.ToolChoiceNone, `"tool_choice":"none"`},
		{api.ToolChoiceAny, `"tool_choice":"required"`},
		{"get_weather", `"tool_choice":{"type":"function","function":{"name":"get_weather"}}`},
	}
	for _, tc := range tests {
		t.Run(string(tc.choice), func(t *testing.T) {
			raw := encodeJSON(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"), WithEffort(EffortNone)), api.Request{
				ToolChoice: tc.choice,
				Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			if tc.want == "" {
				if strings.Contains(raw, "tool_choice") {
					t.Fatalf("an unset choice must leave the field off the wire: %s", raw)
				}
				return
			}
			if !strings.Contains(raw, tc.want) {
				t.Fatalf("body %s does not contain %s", raw, tc.want)
			}
		})
	}
}

// DeepSeek answers a forced choice in thinking mode with a 400, so on a
// declared model it is refused here with a sentence saying how to get one.
// Choices that force nothing still go out, and a model not declared here is sent
// the forced choice as it is.
func TestEncodeForcedToolChoiceInThinkingMode(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	for _, choice := range []api.ToolChoice{api.ToolChoiceAny, "get_weather"} {
		t.Run(string(choice), func(t *testing.T) {
			err := encodeError(t, p, api.Request{
				ToolChoice: choice,
				Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			if !strings.Contains(err.Error(), "WithEffort(EffortNone)") {
				t.Fatalf("error %q", err)
			}
		})
	}
	if err := encodeError(t, mustNew(t, Conf{Model: Flash}, WithAPIKey("k"), WithEffort(EffortHigh)), api.Request{
		ToolChoice: api.ToolChoiceAny,
		Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	}); !strings.Contains(err.Error(), "WithEffort(EffortNone)") {
		t.Fatalf("error %q", err)
	}
	encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v5"}}, WithAPIKey("k")), api.Request{
		ToolChoice: api.ToolChoiceAny,
		Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	})
	for _, choice := range []api.ToolChoice{"", api.ToolChoiceAuto, api.ToolChoiceNone} {
		t.Run(string(choice), func(t *testing.T) {
			encodeBody(t, p, api.Request{
				ToolChoice: choice,
				Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
		})
	}
}

func TestEncodeToolValidation(t *testing.T) {
	base := api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	}
	t.Run("no name", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{}}
		if err := encodeError(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req); !strings.Contains(err.Error(), "no name") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("names are not checked", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{Name: "get weather"}, {Name: strings.Repeat("a", 200)}, {Name: "twice"}, {Name: "twice"}}
		if got := len(encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req).Tools); got != 4 {
			t.Fatalf("%d tools declared, want all 4 as written", got)
		}
	})
	t.Run("no count limit", func(t *testing.T) {
		req := base
		for i := range 200 {
			req.Tools = append(req.Tools, api.Tool{Name: "t" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
		}
		encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req)
	})
}

func TestEncodeEffortAndSampling(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		opts    []Option
		want    []string
		notWant []string
	}{
		{"pro unset", "deepseek-v4-pro", nil, []string{`"thinking":{"type":"enabled"}`}, []string{"reasoning_effort", "temperature", "top_p"}},
		{"flash unset", "deepseek-flash", nil, []string{`"thinking":{"type":"enabled"}`}, []string{"reasoning_effort", "temperature", "top_p"}},
		{"pro none", "deepseek-v4-pro", []Option{WithEffort(EffortNone)}, []string{`"thinking":{"type":"disabled"}`}, []string{"reasoning_effort", "temperature", "top_p"}},
		{"flash none", "deepseek-flash", []Option{WithEffort(EffortNone)}, []string{`"thinking":{"type":"disabled"}`}, []string{"reasoning_effort", "temperature", "top_p"}},
		{"pro low", "deepseek-v4-pro", []Option{WithEffort(EffortLow)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"low"`}, nil},
		{"flash low", "deepseek-flash", []Option{WithEffort(EffortLow)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"low"`}, nil},
		{"pro high", "deepseek-v4-pro", []Option{WithEffort(EffortHigh)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"high"`}, nil},
		{"flash high", "deepseek-flash", []Option{WithEffort(EffortHigh)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"high"`}, nil},
		{"pro max", "deepseek-v4-pro", []Option{WithEffort(EffortMax)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"max"`}, nil},
		{"flash max", "deepseek-flash", []Option{WithEffort(EffortMax)}, []string{`"thinking":{"type":"enabled"},"reasoning_effort":"max"`}, nil},
		{
			"pro none with temperature and top_p", "deepseek-v4-pro",
			[]Option{WithEffort(EffortNone), WithTemperature(0.2), WithTopP(0.9)},
			[]string{`"thinking":{"type":"disabled"}`, `"temperature":0.2`, `"top_p":0.9`},
			[]string{"reasoning_effort"},
		},
		{
			"flash none with temperature", "deepseek-flash",
			[]Option{WithEffort(EffortNone), WithTemperature(0.2)},
			[]string{`"thinking":{"type":"disabled"}`, `"temperature":0.2`},
			[]string{"reasoning_effort", "top_p"},
		},
		{"flash unset with top_p", "deepseek-flash", []Option{WithTopP(0.97)}, []string{`"thinking":{"type":"enabled"}`, `"top_p":0.97`}, []string{"temperature", "reasoning_effort"}},
		{"flash max with top_p", "deepseek-flash", []Option{WithEffort(EffortMax), WithTopP(0.97)}, []string{`"reasoning_effort":"max"`, `"top_p":0.97`}, []string{"temperature"}},
		{"a legacy flash id with top_p", "deepseek-v4-flash", []Option{WithTopP(0.97)}, []string{`"top_p":0.97`}, nil},
		{"stop and user id", "deepseek-v4-pro", []Option{WithStopSequences("STOP"), WithUserID("tenant-7")}, []string{`"stop":["STOP"]`, `"user_id":"tenant-7"`}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := encodeJSON(t, mustNew(t, Conf{Model: api.Model{Name: tc.model}}, append([]Option{WithAPIKey("k")}, tc.opts...)...), api.Request{
				Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			for _, want := range tc.want {
				if !strings.Contains(raw, want) {
					t.Fatalf("body %s does not contain %s", raw, want)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(raw, notWant) {
					t.Fatalf("body %s contains %s", raw, notWant)
				}
			}
		})
	}
}

func TestEncodeFlashImages(t *testing.T) {
	flash := mustNew(t, Conf{Model: Flash}, WithAPIKey("k"))
	for _, mime := range []string{"image/jpeg", "image/png", "image/gif", "image/webp", " IMAGE/PNG ", "image/jpg"} {
		t.Run(mime, func(t *testing.T) {
			raw := encodeJSON(t, flash, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
				api.ContentText{Text: "what is it"},
				api.ContentImage{Data: []byte("img"), Mime: mime},
			}}}})
			want := `"content":[{"type":"text","text":"what is it"},{"type":"image_url","image_url":{"url":"data:` +
				strings.ToLower(strings.TrimSpace(mime)) + `;base64,aW1n"}}]`
			if !strings.Contains(raw, want) {
				t.Fatalf("body %s does not contain %s", raw, want)
			}
		})
	}
	notes := []struct {
		name string
		p    *Provider
		img  api.ContentImage
	}{
		{"pro reads no images", mustNew(t, Conf{Model: V4Pro}, WithAPIKey("k")), api.ContentImage{Data: []byte("img"), Mime: "image/png"}},
		{"an unknown id reads no images", mustNew(t, Conf{Model: api.Model{Name: "my-flash-proxy"}}, WithAPIKey("k")), api.ContentImage{Data: []byte("img"), Mime: "image/png"}},
		{"an unsupported type", flash, api.ContentImage{Data: []byte("img"), Mime: "image/bmp"}},
		{"no bytes", flash, api.ContentImage{Mime: "image/png"}},
	}
	for _, tc := range notes {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeBody(t, tc.p, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{tc.img, api.ContentText{Text: "what is it"}}}}})
			if got, ok := body.Messages[0].Content.(string); !ok || got != "[image omitted]\n\nwhat is it" {
				t.Fatalf("content %#v", body.Messages[0].Content)
			}
		})
	}
	t.Run("an image in an assistant turn", func(t *testing.T) {
		body := encodeBody(t, flash, api.Request{Messages: []api.Message{{Role: api.RoleAssistant, Content: api.Contents{api.ContentImage{Data: []byte("img"), Mime: "image/png"}}}}})
		if got, ok := body.Messages[0].Content.(string); !ok || got != "[image omitted]" {
			t.Fatalf("content %#v", body.Messages[0].Content)
		}
	})
	t.Run("a document stays a note", func(t *testing.T) {
		raw := encodeJSON(t, flash, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
			api.ContentDocument{Data: []byte("%PDF-"), Mime: "application/pdf"},
			api.ContentImage{Data: []byte("img"), Mime: "image/png"},
		}}}})
		if !strings.Contains(raw, `"content":[{"type":"text","text":"[document omitted]"},{"type":"image_url"`) {
			t.Fatalf("body %s", raw)
		}
	})
	t.Run("a tool result", func(t *testing.T) {
		raw := encodeJSON(t, flash, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
			api.ContentToolResult{ID: "t1", Error: api.ToolErrRuntime, Content: api.UserContents{
				api.ContentText{Text: "the chart:"},
				api.ContentImage{Data: []byte("img"), Mime: "image/png"},
			}},
		}}}})
		want := `{"role":"tool","content":[{"type":"text","text":"Error:"},{"type":"text","text":"the chart:"},` +
			`{"type":"image_url","image_url":{"url":"data:image/png;base64,aW1n"}}],"tool_call_id":"t1"}`
		if !strings.Contains(raw, want) {
			t.Fatalf("body %s does not contain %s", raw, want)
		}
	})
}

// Both deprecated penalties are documented as no longer supported, so a field
// for either would encode a knob the server ignores.
func TestEncodeNeverSendsDeprecatedPenalties(t *testing.T) {
	raw := encodeJSON(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	})
	for _, field := range []string{"frequency_penalty", "presence_penalty", "response_format", "strict"} {
		if strings.Contains(raw, field) {
			t.Fatalf("body %s carries %s", raw, field)
		}
	}
}

// The endpoint has no version segment and takes exactly two headers.
func TestRequestWireDetails(t *testing.T) {
	var (
		mu     sync.Mutex
		path   string
		header http.Header
		body   []byte
	)
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		path = r.URL.Path
		header = r.Header.Clone()
		body, _ = io.ReadAll(r.Body)
		mu.Unlock()
		stringHandler(chunkFrame(`{}`, "stop"), doneFrame)(w, r)
	})
	req := turn("hi")
	req.Tools = []api.Tool{{Name: "get_weather", Schema: map[string]any{"type": "object"}}}
	req.ToolChoice = api.ToolChoiceAuto
	req.System = "be terse"
	collect(t, p, req)

	mu.Lock()
	defer mu.Unlock()
	if path != "/chat/completions" {
		t.Fatalf("path %q, want /chat/completions with no version segment", path)
	}
	if got := header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization %q", got)
	}
	if got := header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type %q", got)
	}
	var sent chatRequest
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	if sent.Model != "deepseek-v4-pro" || !sent.Stream {
		t.Fatalf("body %s", body)
	}
	// Field order follows the cache hierarchy api.Request documents:
	// tools, then the system prompt, which lives at the head of messages here,
	// then everything a turn does not cache on. The cache only hits on a prefix
	// that matches a persisted unit exactly, so the order is worth holding.
	for _, pair := range [][2]string{{`"model"`, `"tools"`}, {`"tools"`, `"tool_choice"`}, {`"tool_choice"`, `"messages"`}} {
		first, second := strings.Index(string(body), pair[0]), strings.Index(string(body), pair[1])
		if first < 0 || second < 0 || first > second {
			t.Fatalf("%s must precede %s: %s", pair[0], pair[1], body)
		}
	}
	if i := strings.Index(string(body), `"be terse"`); i < 0 || i > strings.Index(string(body), `"hi"`) {
		t.Fatalf("the system prompt must lead the messages: %s", body)
	}
}

type levelCounter map[slog.Level]int

func (c levelCounter) Enabled(context.Context, slog.Level) bool { return true }

func (c levelCounter) Handle(_ context.Context, r slog.Record) error {
	c[r.Level]++
	return nil
}

func (c levelCounter) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c levelCounter) WithGroup(string) slog.Handler { return c }

func TestEncodeLogsNothingForAnOmission(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })
	req := api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
		api.ContentImage{Data: []byte{1}, Mime: "image/png"},
		api.ContentDocument{Data: []byte("PK"), Mime: string(api.DocumentDOCX)},
		api.ContentText{Text: "look"},
	}}}}
	for range 2 {
		if got := encodeJSON(t, p, req); !strings.Contains(got, `[image omitted]\n\n[document omitted]\n\nlook`) {
			t.Fatalf("body %s does not carry the notes", got)
		}
	}
	if len(counts) != 0 {
		t.Fatalf("two encodes logged %v, want nothing", map[slog.Level]int(counts))
	}
}

func TestEncodeLogsAnUnknownTypeOnce(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })
	req := api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{nil, api.ContentText{Text: "look"}}}}}
	for range 2 {
		if got := encodeJSON(t, p, req); !strings.Contains(got, `[content omitted]\n\nlook`) {
			t.Fatalf("body %s does not carry the note", got)
		}
	}
	if counts[slog.LevelWarn] != 0 || counts[slog.LevelError] > 1 {
		t.Fatalf("two encodes logged %v, want no warnings and at most one error", map[slog.Level]int(counts))
	}
}

func TestFlashImageURLMatchesConcatenation(t *testing.T) {
	flash := mustNew(t, Conf{Model: Flash}, WithAPIKey("k"))
	for _, n := range []int{1, 2, 3, 4, 5, 57, 1000, 1<<20 + 1} {
		data := make([]byte, n)
		for i := range data {
			data[i] = byte(i*7 + i>>8)
		}
		part, ok := flash.image(api.ContentImage{Data: data, Mime: " Image/PNG "}).(imagePart)
		if !ok {
			t.Fatalf("%d bytes: the image was noted instead of sent", n)
		}
		if want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data); part.ImageURL.URL != want {
			t.Fatalf("%d bytes: the data URL differs from the concatenated one", n)
		}
	}
}

func BenchmarkFlashImageURL(b *testing.B) {
	flash, err := New(Conf{Model: Flash}, WithAPIKey("k"))
	if err != nil {
		b.Fatal(err)
	}
	image := api.ContentImage{Data: make([]byte, 1<<20), Mime: "image/png"}
	b.ReportAllocs()
	for b.Loop() {
		flash.image(image)
	}
}
