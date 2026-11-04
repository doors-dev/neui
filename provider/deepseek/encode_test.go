package deepseek

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"neui.dev/api"
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

func encodeError(t *testing.T, p *Provider, req api.Request) *RequestError {
	t.Helper()
	_, err := p.encode(req)
	if err == nil {
		t.Fatal("expected the turn to be refused")
	}
	reqErr, ok := err.(*RequestError)
	if !ok {
		t.Fatalf("err is %T, want *RequestError", err)
	}
	return reqErr
}

func text(s string) api.Contents {
	return api.Contents{api.ContentText{Text: s}}
}

// The system prompt is a leading message rather than a field, several text
// blocks in one message become one string, and streaming with usage is not
// optional because it is the only way the turn is priced.
func TestEncodeShape(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
	if body.Messages[0].Role != "system" || *body.Messages[0].Content != "be terse\nanswer in English" {
		t.Fatalf("system message %+v", body.Messages[0])
	}
	if body.Messages[1].Role != "user" || *body.Messages[1].Content != "one\n\ntwo" {
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
		{"over the documented ceiling", 1_000_000, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"), WithMaxTokens(tc.opt))
			body := encodeBody(t, p, api.Request{
				Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			if body.MaxTokens != tc.want {
				t.Fatalf("max_tokens %d, want %d", body.MaxTokens, tc.want)
			}
		})
	}
	if err := WithMaxTokens(maxOutputTokens + 1)(&Provider{}); err == nil {
		t.Fatal("a cap over the documented ceiling has to be reported, not adjusted")
	}
}

// This wire carries text and nothing else, and a turn carrying anything else is
// a thread that touched another provider or an adapter that never grew a path
// for it. The note goes out in the block's place: refusing would end
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
		{"image in an assistant turn", api.Message{Role: api.RoleAssistant, Content: api.Contents{image}}, "[image omitted]"},
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
			body := encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
				Messages: []api.Message{tc.msg},
			})
			if len(body.Messages) != 1 {
				t.Fatalf("messages %d, want 1", len(body.Messages))
			}
			if got := *body.Messages[0].Content; got != tc.want {
				t.Fatalf("content %q, want %q", got, tc.want)
			}
		})
	}
}

// This wire takes a result as a bare string, so an attachment inside one goes
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
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
			if got := *m.Content; got != tc.want {
				t.Fatalf("content %q, want %q", got, tc.want)
			}
		})
	}
}

// Tool results are their own role, one message each, and they come out before
// any user text in the same message: a result has to follow the assistant turn
// that asked for it with nothing in between.
func TestEncodeToolResults(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
	if body.Messages[0].Role != "tool" || body.Messages[0].ToolCallID != "call_a" || *body.Messages[0].Content != "24C" {
		t.Fatalf("first result %+v", body.Messages[0])
	}
	// There is no error flag on a tool message, so a failed result says so in
	// the text or the model never learns it failed.
	if *body.Messages[1].Content != "Error: boom" {
		t.Fatalf("failed result %q", *body.Messages[1].Content)
	}
	if body.Messages[2].Role != "user" || *body.Messages[2].Content != "and now?" {
		t.Fatalf("trailing user message %+v", body.Messages[2])
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
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
	if m.Role != "assistant" || *m.Content != "let me look\n\nat this" {
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
	body := encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
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
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
	if got := *body.Messages[1].Content; got != "Error: not valid JSON" {
		t.Fatalf("result content %q does not report the discard", got)
	}
}

// A call with no name names no tool, so there is nothing for the model to make
// sense of and nothing for the caller to dispatch. Sending "name":"" would ask
// DeepSeek to decide what that meant.
func TestEncodeRejectsNamelessToolCall(t *testing.T) {
	err := encodeError(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
		Messages: []api.Message{{
			Role:    api.RoleAssistant,
			Content: api.Contents{api.ContentToolUse{ID: "call_a", Input: json.RawMessage(`{}`)}},
		}},
	})
	if !strings.Contains(err.Error(), "call_a") || !strings.Contains(err.Error(), "no name") {
		t.Fatalf("error %q names neither the call nor what is wrong with it", err)
	}
}

// A turn that only called tools sends an explicit null, which the assistant
// message schema documents and which omitting would not reproduce.
func TestEncodeAssistantContentNull(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
			body := encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req)
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
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
	if got := *body.Messages[0].Content; got != "[content omitted]\n\nhi" {
		t.Fatalf("content %q", got)
	}
}

func TestEncodeTools(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
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
			raw := encodeJSON(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"), WithThinking(false)), api.Request{
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

// DeepSeek answers a forced choice in thinking mode with a 400, so it is
// refused here with a sentence saying how to get one. Choices that force
// nothing still go out.
func TestEncodeForcedToolChoiceInThinkingMode(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"))
	for _, choice := range []api.ToolChoice{api.ToolChoiceAny, "get_weather"} {
		t.Run(string(choice), func(t *testing.T) {
			err := encodeError(t, p, api.Request{
				ToolChoice: choice,
				Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
			if !strings.Contains(err.Error(), "WithThinking(false)") {
				t.Fatalf("error %q", err)
			}
		})
	}
	for _, choice := range []api.ToolChoice{"", api.ToolChoiceAuto, api.ToolChoiceNone} {
		t.Run(string(choice), func(t *testing.T) {
			encodeBody(t, p, api.Request{
				ToolChoice: choice,
				Messages:   []api.Message{{Role: api.RoleUser, Content: text("hi")}},
			})
		})
	}
}

// The server enforces these too, with a 400 that does not say which tool was
// wrong.
func TestEncodeToolValidation(t *testing.T) {
	base := api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: text("hi")}},
	}
	t.Run("name characters", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{Name: "get weather"}}
		if err := encodeError(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req); !strings.Contains(err.Error(), "get weather") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("name length", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{Name: strings.Repeat("a", maxToolNameLen+1)}}
		if err := encodeError(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req); !strings.Contains(err.Error(), "128") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("name at the limit", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{Name: strings.Repeat("a", maxToolNameLen)}}
		encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req)
	})
	t.Run("duplicate name", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{{Name: "get_weather"}, {Name: "get_weather"}}
		if err := encodeError(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req); !strings.Contains(err.Error(), "twice") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("no count limit", func(t *testing.T) {
		req := base
		for i := range 200 {
			req.Tools = append(req.Tools, api.Tool{Name: "t" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
		}
		encodeBody(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), req)
	})
	t.Run("no model", func(t *testing.T) {
		if err := encodeError(t, New(Conf{}, WithAPIKey("k")), api.Request{}); !strings.Contains(err.Error(), "no model") {
			t.Fatalf("error %q", err)
		}
	})
}

// Thinking is on by default and the field goes out explicitly, so a change to
// the server default cannot quietly change what a turn does. Effort and
// temperature reach the model with thinking on and off respectively, and top_p
// with thinking on for flash and off for v4-pro, so each is omitted elsewhere
// rather than sent to be ignored. Effort is a top level field; the thinking
// object has only a type.
func TestEncodeThinkingAndSampling(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		opts    []Option
		want    []string
		notWant []string
	}{
		{
			name:    "default",
			opts:    nil,
			want:    []string{`"thinking":{"type":"enabled"}`},
			notWant: []string{"reasoning_effort", "temperature", "top_p"},
		},
		{
			name:    "temperature with thinking on",
			opts:    []Option{WithTemperature(0.2), WithTopP(0.9)},
			want:    []string{`"thinking":{"type":"enabled"}`},
			notWant: []string{"temperature", "top_p"},
		},
		{
			name: "temperature with thinking off",
			opts: []Option{WithThinking(false), WithTemperature(0.2), WithTopP(0.9)},
			want: []string{`"thinking":{"type":"disabled"}`, `"temperature":0.2`, `"top_p":0.9`},
		},
		{
			name:    "flash honors top_p with thinking on",
			model:   "deepseek-flash",
			opts:    []Option{WithTemperature(0.2), WithTopP(0.97)},
			want:    []string{`"thinking":{"type":"enabled"}`, `"top_p":0.97`},
			notWant: []string{"temperature"},
		},
		{
			name:  "a legacy flash id too",
			model: "deepseek-v4-flash",
			opts:  []Option{WithTopP(0.97)},
			want:  []string{`"top_p":0.97`},
		},
		{
			name:    "flash fixes top_p with thinking off",
			model:   "deepseek-flash",
			opts:    []Option{WithThinking(false), WithTemperature(0.2), WithTopP(0.9)},
			want:    []string{`"thinking":{"type":"disabled"}`, `"temperature":0.2`},
			notWant: []string{"top_p"},
		},
		{
			name: "reasoning effort is top level",
			opts: []Option{WithReasoningEffort(EffortMax)},
			want: []string{`"thinking":{"type":"enabled"},"reasoning_effort":"max"`},
		},
		{
			name: "low effort",
			opts: []Option{WithReasoningEffort(EffortLow)},
			want: []string{`"reasoning_effort":"low"`},
		},
		{
			name:    "no effort with thinking off",
			opts:    []Option{WithThinking(false), WithReasoningEffort(EffortMax)},
			want:    []string{`"thinking":{"type":"disabled"}`},
			notWant: []string{"reasoning_effort"},
		},
		{
			name: "stop and user id",
			opts: []Option{WithStop("STOP"), WithUserID("tenant-7")},
			want: []string{`"stop":["STOP"]`, `"user_id":"tenant-7"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := tc.model
			if model == "" {
				model = "deepseek-v4-pro"
			}
			raw := encodeJSON(t, New(Conf{Model: api.Model{Name: model}}, append([]Option{WithAPIKey("k")}, tc.opts...)...), api.Request{
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

// A user_id over the documented 512 bytes is the option's error, and the id it
// had stays.
func TestWithUserIDLimit(t *testing.T) {
	p := New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k"), WithUserID("tenant-7"), WithUserID(strings.Repeat("a", maxUserIDBytes+1)))
	if p.userID != "tenant-7" {
		t.Fatalf("user id %q", p.userID)
	}
	if err := WithUserID(strings.Repeat("a", maxUserIDBytes))(&Provider{}); err != nil {
		t.Fatalf("an id at the limit was refused: %v", err)
	}
}

// Both deprecated penalties are documented as no longer supported, so a field
// for either would encode a knob the server ignores.
func TestEncodeNeverSendsDeprecatedPenalties(t *testing.T) {
	raw := encodeJSON(t, New(Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
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
