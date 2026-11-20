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

package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func mustNew(t *testing.T, conf Conf, opts ...Option) *Provider {
	t.Helper()
	p, err := New(conf, append([]Option{WithoutCache()}, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func withCache() Option {
	return func(p *Provider) error {
		p.noCache = false
		return nil
	}
}

func mustRefuse(t *testing.T, conf Conf, want string, opts ...Option) {
	t.Helper()
	_, err := New(conf, opts...)
	if err == nil {
		t.Fatalf("New succeeded, want a refusal mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not mention %q", err, want)
	}
}

func encodeBody(p *Provider, req api.Request) (*lib.Body, error) {
	w, err := p.encode(req)
	if err != nil {
		return nil, err
	}
	return lib.MarshalBody(w)
}

func mustEncode(t *testing.T, p *Provider, req api.Request) map[string]json.RawMessage {
	t.Helper()
	raw, err := encodeBody(p, req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw.Bytes(), &out); err != nil {
		t.Fatalf("encoded request is not an object: %v", err)
	}
	return out
}

func mustFail(t *testing.T, p *Provider, req api.Request, want string) {
	t.Helper()
	_, err := p.encode(req)
	if err == nil {
		t.Fatalf("encode succeeded, want a refusal mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not mention %q", err, want)
	}
}

func userText(s string) api.Message {
	return api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: s}}}
}

func TestEncodeRequestShape(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithMaxTokens(2048), WithEffort("medium"), WithThinking(Thinking{Type: "adaptive", Display: "summarized"}))
	got := mustEncode(t, p, api.Request{
		ToolChoice: api.ToolChoiceAny,
		Tools: []api.Tool{{
			Name:   "get_weather",
			Desc:   "current weather",
			Schema: map[string]any{"type": "object", "properties": map[string]any{}},
		}},
		System:   "be brief",
		Messages: []api.Message{userText("hi")},
	})

	for field, want := range map[string]string{
		"model":         `"` + testModel + `"`,
		"max_tokens":    `2048`,
		"stream":        `true`,
		"tool_choice":   `{"type":"any"}`,
		"system":        `[{"type":"text","text":"be brief"}]`,
		"messages":      `[{"role":"user","content":[{"type":"text","text":"hi"}]}]`,
		"thinking":      `{"type":"adaptive","display":"summarized","block_binding":{"prefix_mismatch_behavior":"drop_block"}}`,
		"output_config": `{"effort":"medium"}`,
		"tools":         `[{"name":"get_weather","description":"current weather","input_schema":{"properties":{},"type":"object"}}]`,
	} {
		if string(got[field]) != want {
			t.Errorf("%s = %s, want %s", field, got[field], want)
		}
	}
}

func TestEncodeToolChoice(t *testing.T) {
	for choice, want := range map[api.ToolChoice]string{
		"":                  "",
		api.ToolChoiceAuto:  `{"type":"auto"}`,
		api.ToolChoiceNone:  `{"type":"none"}`,
		api.ToolChoiceAny:   `{"type":"any"}`,
		api.ToolChoice("f"): `{"type":"tool","name":"f"}`,
	} {
		p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"))
		got := mustEncode(t, p, api.Request{
			ToolChoice: choice,
			Messages:   []api.Message{userText("hi")},
		})
		if string(got["tool_choice"]) != want {
			t.Errorf("tool_choice for %q = %s, want %s", choice, got["tool_choice"], want)
		}
	}
}

// Forcing a tool is refused only alongside manual extended thinking, and only on
// a declared model. Adaptive thinking accepts it, including on models that think
// by default.
func TestForcedToolChoiceWithExtendedThinking(t *testing.T) {
	req := api.Request{
		ToolChoice: api.ToolChoiceAny,
		Messages:   []api.Message{userText("hi")},
	}
	mustFail(t, mustNew(t, Conf{Model: Haiku45}, WithThinking(Thinking{Type: "enabled", Budget: 4096})), req, "extended thinking")
	if got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: "claude-sonnet-4-5"}}, WithThinking(Thinking{Type: "enabled", Budget: 4096})), req); string(got["tool_choice"]) != `{"type":"any"}` {
		t.Fatalf("unknown model: tool_choice = %s", got["tool_choice"])
	}

	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithThinking(Thinking{Type: "adaptive"}))
	if got := mustEncode(t, p, api.Request{
		ToolChoice: api.ToolChoiceAny,
		Messages:   []api.Message{userText("hi")},
	}); string(got["tool_choice"]) != `{"type":"any"}` {
		t.Fatalf("tool_choice = %s", got["tool_choice"])
	}
}

func TestForcedToolChoiceRefusedByModel(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-fable-5-1"} {
		for _, choice := range []api.ToolChoice{api.ToolChoiceAny, api.ToolChoice("f")} {
			mustFail(t, mustNew(t, Conf{Model: api.Model{Name: model}}), api.Request{ToolChoice: choice, Messages: []api.Message{userText("hi")}}, "refuses tool choice")
		}
		for choice, want := range map[api.ToolChoice]string{api.ToolChoiceAuto: `{"type":"auto"}`, api.ToolChoiceNone: `{"type":"none"}`} {
			got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: model}}), api.Request{ToolChoice: choice, Messages: []api.Message{userText("hi")}})
			if string(got["tool_choice"]) != want {
				t.Errorf("%s: tool_choice = %s, want %s", model, got["tool_choice"], want)
			}
		}
	}
	for _, model := range []string{"claude-opus-5", "claude-sonnet-5", "claude-fable-5", "claude-haiku-5-5", "claude-mythos-5-1", "anthropic.claude-opus-5-5"} {
		got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: model}}), api.Request{ToolChoice: api.ToolChoiceAny, Messages: []api.Message{userText("hi")}})
		if string(got["tool_choice"]) != `{"type":"any"}` {
			t.Errorf("%s: tool_choice = %s, want any", model, got["tool_choice"])
		}
	}
}

func TestImageMimeChecked(t *testing.T) {
	user := func(c api.Content) api.Request {
		return api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{c, api.ContentText{Text: "what is it"}}}}}
	}
	for _, mime := range []string{"", "image/bmp", "application/pdf"} {
		got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), user(api.ContentImage{Data: []byte{1}, Mime: mime}))
		want := `{"role":"user","content":[{"type":"text","text":"[image omitted]"},{"type":"text","text":"what is it"}]}`
		if !strings.Contains(string(got["messages"]), want) {
			t.Fatalf("mime %q: messages =\n%s\nwant to contain\n%s", mime, got["messages"], want)
		}
	}
	for mime, sent := range map[string]string{"image/png": "image/png", "image/jpeg": "image/jpeg", "image/gif": "image/gif", "image/webp": "image/webp", " IMAGE/PNG ": "image/png", "image/jpg": "image/jpeg"} {
		got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), user(api.ContentImage{Data: []byte{1}, Mime: mime}))
		if want := `"media_type":"` + sent + `"`; !strings.Contains(string(got["messages"]), want) {
			t.Fatalf("mime %q: messages = %s, want to contain %s", mime, got["messages"], want)
		}
	}

	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("look"),
			{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "toolu_1", Name: "shot", Input: json.RawMessage(`{}`)}}},
			{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
				api.ContentImage{Data: []byte{1}, Mime: "image/bmp"},
				api.ContentImage{Data: []byte{1}},
				api.ContentImage{Data: []byte{1}, Mime: "image/png"},
			}}}},
		},
	})
	msgs := string(got["messages"])
	want := `"content":[{"type":"text","text":"[image omitted]"},{"type":"text","text":"[image omitted]"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AQ=="}}]`
	if !strings.Contains(msgs, want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", msgs, want)
	}
}

func TestEncodeContentBlocks(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"))
	got := mustEncode(t, p, api.Request{
		Messages: []api.Message{{
			Role: api.RoleUser,
			Content: api.Contents{
				api.ContentText{Text: "look"},
				api.ContentImage{Data: []byte{0xff, 0xd8}, Mime: "image/jpeg"},
				api.ContentDocument{Data: []byte("%PDF-1.4"), Mime: "application/pdf", Name: "r.pdf"},
				api.ContentDocument{Data: []byte("plain"), Mime: "text/plain"},
			},
		}},
	})
	want := `[{"role":"user","content":[` +
		`{"type":"text","text":"look"},` +
		`{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"/9g="}},` +
		`{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0xLjQ="},"title":"r.pdf"},` +
		`{"type":"document","source":{"type":"text","media_type":"text/plain","data":"plain"}}]}]`
	if string(got["messages"]) != want {
		t.Fatalf("messages =\n%s\nwant\n%s", got["messages"], want)
	}
}

func TestEncodeRefusesARequestWithNoMessages(t *testing.T) {
	mustFail(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{}, "no messages")
}

func TestStoredContentTheAPIRefusesIsANote(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}})
	for name, c := range map[string]struct {
		msg  api.Message
		want string
	}{
		"result with no id": {
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{Content: api.UserContents{api.ContentText{Text: "72F"}}}}},
			`{"role":"user","content":[{"type":"text","text":"[content omitted]"}]}`,
		},
		"call with no name": {
			api.Message{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "toolu_1", Input: json.RawMessage(`{}`)}}},
			`{"role":"assistant","content":[{"type":"text","text":"[content omitted]"}]}`,
		},
		"call with invalid arguments": {
			api.Message{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "toolu_1", Name: "weather", Input: json.RawMessage(`{"city":`)}}},
			`{"role":"assistant","content":[{"type":"text","text":"[content omitted]"}]}`,
		},
		"assistant blocks in a user message": {
			api.Message{Role: api.RoleUser, Content: api.Contents{
				api.ContentText{Text: "hi"},
				api.ContentToolUse{ID: "toolu_1", Name: "weather", Input: json.RawMessage(`{}`)},
				api.ContentReasoning{Meta: json.RawMessage(`"sig"`)},
				api.ContentOpaque{Kind: "server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1"}`)},
			}},
			`{"role":"user","content":[{"type":"text","text":"hi"},{"type":"text","text":"[content omitted]"},{"type":"text","text":"[reasoning omitted]"},{"type":"text","text":"[content omitted]"}]}`,
		},
		"message with no blocks": {
			api.Message{Role: api.RoleAssistant},
			`{"role":"assistant","content":[{"type":"text","text":"[empty]"}]}`,
		},
	} {
		got := mustEncode(t, p, api.Request{Messages: []api.Message{userText("hi"), c.msg}})
		if !strings.Contains(string(got["messages"]), c.want) {
			t.Errorf("%s: messages =\n%s\nwant to contain\n%s", name, got["messages"], c.want)
		}
	}
	got := mustEncode(t, p, api.Request{Messages: []api.Message{
		userText("hi"),
		{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "toolu_1", Input: json.RawMessage(`{}`)}}},
		{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{api.ContentText{Text: "72F"}}}}},
	}})
	if strings.Contains(string(got["messages"]), "tool_result") || !strings.HasSuffix(string(got["messages"]), `{"role":"user","content":[{"type":"text","text":"[content omitted]"}]}]`) {
		t.Errorf("the result answering a call that went out as a note was sent:\n%s", got["messages"])
	}
}

func TestToolResultsGoFirstInAUserMessage(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, withCache())
	result := func(id string) api.Content {
		return api.ContentToolResult{ID: id, Content: api.UserContents{api.ContentText{Text: id}}}
	}
	call := func(id string) api.Content {
		return api.ContentToolUse{ID: id, Name: "weather", Input: json.RawMessage(`{}`)}
	}
	got := mustEncode(t, p, api.Request{Messages: []api.Message{
		userText("hi"),
		{Role: api.RoleAssistant, Content: api.Contents{call("a"), call("b")}},
		{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "before"}, result("a"), api.ContentText{Text: "between"}, result("b")}},
	}})
	want := `{"role":"user","content":[` +
		`{"type":"tool_result","tool_use_id":"a","content":[{"type":"text","text":"a"}],"is_error":false},` +
		`{"type":"tool_result","tool_use_id":"b","content":[{"type":"text","text":"b"}],"is_error":false},` +
		`{"type":"text","text":"before"},` +
		`{"cache_control":{"type":"ephemeral"},"type":"text","text":"between"}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
	got = mustEncode(t, p, api.Request{Messages: []api.Message{
		userText("hi"),
		{Role: api.RoleAssistant, Content: api.Contents{call("a"), call("b")}},
		{Role: api.RoleUser, Content: api.Contents{result("a"), result("b")}},
	}})
	want = `{"type":"tool_result","tool_use_id":"a","content":[{"type":"text","text":"a"}],"is_error":false},` +
		`{"cache_control":{"type":"ephemeral"},"type":"tool_result","tool_use_id":"b","content":[{"type":"text","text":"b"}],"is_error":false}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

func TestTextGoesOutWithoutHTMLEscapes(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		System:   "a<b>&c",
		Messages: []api.Message{userText("x<y>&z")},
	})
	if string(got["system"]) != `[{"type":"text","text":"a<b>&c"}]` || !strings.Contains(string(got["messages"]), `"text":"x<y>&z"`) {
		t.Fatalf("system = %s, messages = %s, want <, > and & as they are", got["system"], got["messages"])
	}
}

// A message whose every block was another provider's still goes out, as the note
// each of those blocks left. Failing the request instead would end a thread on
// stored history the caller cannot rewrite, and dropping the message would take
// the turn out of the conversation.
func TestEncodeEmptiedMessageStillEncodes(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "..."}, Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
			}},
			userText("again"),
		},
	})
	want := `{"role":"assistant","content":[{"type":"text","text":"[reasoning omitted]"}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

// Every content type that cannot be sent leaves a note naming what was there,
// and the message keeps whatever else it held.
func TestEncodeNotesWhatItCannotSend(t *testing.T) {
	const model = "claude-opus-5"
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "theirs"}, Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
				api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
				api.ContentText{Text: "answer"},
			}},
		},
	})
	want := `{"role":"assistant","content":[` +
		`{"type":"text","text":"[reasoning omitted]"},` +
		`{"type":"text","text":"[content omitted]"},` +
		`{"type":"text","text":"answer"}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

// A tool result is a block of the user turn on this API, so which message it was
// written in is the one part of the caller's arrangement that cannot survive:
// one written into an assistant message leaves a note, and moving it into a turn
// the caller never wrote is not the adapter's to do.
func TestToolResultOutsideAUserMessageIsANote(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("weather?"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
					api.ContentText{Text: "72F"},
				}},
				api.ContentText{Text: "72 degrees."},
			}},
		},
	})
	want := `{"role":"assistant","content":[` +
		`{"type":"text","text":"[content omitted]"},` +
		`{"type":"text","text":"72 degrees."}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

func TestEncodeToolUseAndResult(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("weather?"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolUse{ID: "toolu_1", Name: "get", Input: json.RawMessage(`{"city":"SF"}`)},
				// Arguments that never parsed were discarded, so there are
				// none left to send; the result below classifies the call as
				// one whose arguments never formed JSON, which is what licenses
				// the empty object and tells the model what happened.
				api.ContentToolUse{ID: "toolu_2", Name: "get", Input: api.DiscardedArguments},
				// A call the provider sent no arguments for. It takes none,
				// and the empty object is the only shape this API has for that.
				api.ContentToolUse{ID: "toolu_3", Name: "now"},
			}},
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
					api.ContentText{Text: "72F"},
				}},
				api.ContentToolResult{ID: "toolu_2", Error: api.ToolErrJSON, Content: api.UserContents{
					api.ContentText{Text: api.DiscardedArgumentsError{Raw: `{"cit`}.Error()},
				}},
				api.ContentToolResult{ID: "toolu_3", Error: api.ToolErrRuntime, Content: api.UserContents{
					api.ContentText{Text: "the clock is broken"},
				}},
			}},
		},
	})
	discarded, _ := json.Marshal(api.DiscardedArgumentsError{Raw: `{"cit`}.Error())
	for _, want := range []string{
		`{"type":"tool_use","id":"toolu_1","name":"get","input":{"city":"SF"}}`,
		`{"type":"tool_use","id":"toolu_2","name":"get","input":{}}`,
		`{"type":"tool_use","id":"toolu_3","name":"now","input":{}}`,
		`{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"72F"}],"is_error":false}`,
		`{"type":"tool_result","tool_use_id":"toolu_2","content":[{"type":"text","text":` + string(discarded) + `}],"is_error":true}`,
		`{"type":"tool_result","tool_use_id":"toolu_3","content":[{"type":"text","text":"the clock is broken"}],"is_error":true}`,
	} {
		if !strings.Contains(string(got["messages"]), want) {
			t.Errorf("messages missing %s\ngot %s", want, got["messages"])
		}
	}
}

// TestDiscardedArgumentsNeedTheirResult is the pairing assertion. A call whose
// arguments never formed JSON carries the marker and nothing sendable, and the
// result classifying it as err_json is what licenses the empty object standing
// in for arguments that are gone. A thread that lost that classification is
// refused rather than sent, because {} there replays as a call that legitimately
// took none, on this turn and every later one.
func TestDiscardedArgumentsNeedTheirResult(t *testing.T) {
	mustFail(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("weather?"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolUse{ID: "toolu_1", Name: "get", Input: api.DiscardedArguments},
			}},
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Error: api.ToolErrRuntime, Content: api.UserContents{
					api.ContentText{Text: "the tool blew up"},
				}},
			}},
		},
	}, "discarded arguments")
}

// A result takes text, images and the documents a message takes here, and a
// block it cannot carry leaves the note every other unsendable block leaves.
// Refusing the request would not deliver the document either, and it would take
// the rest of the result with it, leaving the model waiting on a call nothing
// answered.
func TestToolResultNotesWhatItCannotCarry(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("read it"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolUse{ID: "toolu_1", Name: "read", Input: json.RawMessage(`{}`)},
			}},
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
					api.ContentText{Text: "the report"},
					api.ContentDocument{Data: []byte("%PDF-1.4"), Mime: "application/pdf", Name: "r.pdf"},
					api.ContentDocument{Data: []byte("15 degrees"), Mime: string(api.DocumentMarkdown)},
					api.ContentDocument{Data: []byte("PK"), Mime: string(api.DocumentDOCX), Name: "r.docx"},
				}},
			}},
		},
	})
	want := `{"type":"tool_result","tool_use_id":"toolu_1","content":[` +
		`{"type":"text","text":"the report"},` +
		`{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERi0xLjQ="},"title":"r.pdf"},` +
		`{"type":"document","source":{"type":"text","media_type":"text/plain","data":"15 degrees"}},` +
		`{"type":"text","text":"[document omitted]"}],"is_error":false}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

// An attachment whose bytes are gone never becomes a source. This API refuses a
// source with nothing in it, and a user message and a tool result are both stored
// and replayed, so one sent as a block would fail every later turn of the thread
// rather than this one alone. The emptiness is read before the mime type, so
// neither refusal this arm has fires on a block that could not be sent whatever
// it claimed to be.
func TestEmptyAttachmentIsANote(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentImage{Mime: "image/png"},
				api.ContentImage{},
				api.ContentDocument{Mime: "application/pdf", Name: "r.pdf"},
				api.ContentDocument{Mime: "application/zip"},
				api.ContentText{Text: "what is it"},
			}},
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolUse{ID: "toolu_1", Name: "read", Input: json.RawMessage(`{}`)},
			}},
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
					api.ContentImage{Mime: "image/png"},
					api.ContentDocument{Mime: "application/pdf"},
					api.ContentText{Text: "the report"},
				}},
			}},
		},
	})
	msgs := string(got["messages"])
	for _, want := range []string{
		`{"role":"user","content":[` +
			`{"type":"text","text":"[image omitted]"},` +
			`{"type":"text","text":"[image omitted]"},` +
			`{"type":"text","text":"[document omitted]"},` +
			`{"type":"text","text":"[document omitted]"},` +
			`{"type":"text","text":"what is it"}]}`,
		`{"type":"tool_result","tool_use_id":"toolu_1","content":[` +
			`{"type":"text","text":"[image omitted]"},` +
			`{"type":"text","text":"[document omitted]"},` +
			`{"type":"text","text":"the report"}],"is_error":false}`,
	} {
		if !strings.Contains(msgs, want) {
			t.Fatalf("messages =\n%s\nwant to contain\n%s", msgs, want)
		}
	}
	if strings.Contains(msgs, `"source"`) {
		t.Fatalf("an attachment with nothing in it went out as a source:\n%s", msgs)
	}
}

// A text block with nothing but whitespace in it never becomes a text block on
// the wire. This API refuses one outright, and the refusal is not a failure one
// turn absorbs: the block is stored, so it goes back out on every later round of
// the thread and earns the identical 400 until somebody rewinds. A third party
// MCP server answering {"type":"text","text":""} is the whole of what it takes
// to reach it.
//
// A result that arrived with no blocks at all is the same case one step over: it
// would encode as an empty content array, which this API does not document
// accepting, so it takes the note too and the result still answers its call.
func TestEmptyTextIsANote(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentText{},
				api.ContentText{Text: " \n\t"},
				api.ContentText{Text: "what is it"},
			}},
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentToolUse{ID: "toolu_1", Name: "read", Input: json.RawMessage(`{}`)},
				api.ContentToolUse{ID: "toolu_2", Name: "read", Input: json.RawMessage(`{}`)},
				api.ContentText{Text: "  "},
			}},
			{Role: api.RoleUser, Content: api.Contents{
				api.ContentToolResult{ID: "toolu_1", Content: api.UserContents{
					api.ContentText{},
					api.ContentText{Text: "72F"},
				}},
				api.ContentToolResult{ID: "toolu_2"},
			}},
		},
	})
	msgs := string(got["messages"])
	for _, want := range []string{
		`{"role":"user","content":[` +
			`{"type":"text","text":"[content omitted]"},` +
			`{"type":"text","text":"[content omitted]"},` +
			`{"type":"text","text":"what is it"}]}`,
		`{"type":"text","text":"[content omitted]"}]}`,
		`{"type":"tool_result","tool_use_id":"toolu_1","content":[` +
			`{"type":"text","text":"[content omitted]"},` +
			`{"type":"text","text":"72F"}],"is_error":false}`,
		`{"type":"tool_result","tool_use_id":"toolu_2","content":[` +
			`{"type":"text","text":"(the tool returned no output)"}],"is_error":false}`,
	} {
		if !strings.Contains(msgs, want) {
			t.Fatalf("messages =\n%s\nwant to contain\n%s", msgs, want)
		}
	}
	if strings.Contains(msgs, `"text":""`) || strings.Contains(msgs, `"text":" \n\t"`) {
		t.Fatalf("a text block this API refuses went out, and it is stored, so every later turn sends it again:\n%s", msgs)
	}
	if strings.Contains(msgs, `"content":[]`) {
		t.Fatalf("a result went out with an empty content array:\n%s", msgs)
	}
}

func TestReplayReasoningAndOpaque(t *testing.T) {
	const model = "claude-opus-5"
	sealed := json.RawMessage(`{"type":"redacted_thinking","data":"EvwBCkYIA"}`)
	signed := json.RawMessage(`"sig2"`)
	summarized := json.RawMessage(`"sig3"`)
	result := json.RawMessage(`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]}`)
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				// An envelope with nothing readable in it is what a model whose
				// thinking display is omitted returns, and dropping one of those
				// is the classic way to break a tool use turn.
				api.ContentReasoning{Model: model, Meta: json.RawMessage(`"sig1"`)},
				// The readable half goes into the envelope built around the
				// signature.
				api.ContentReasoning{Model: model, Thoughts: map[int]string{0: "because"}, Meta: signed},
				// A turn configured for summarized display fills the other field
				// out of the same wire field, so that is the one that goes into
				// it. The signature was issued over what the API sent.
				api.ContentReasoning{Model: model, Summary: map[int]string{0: "in short"}, Meta: summarized},
				// The other shape meta holds here: a redacted_thinking block is
				// bytes this API sent, so it goes back as they are.
				api.ContentReasoning{Model: model, Meta: sealed},
				api.ContentOpaque{Model: "claude-sonnet-5", Kind: "web_search_tool_result", Data: result},
				api.ContentOpaque{Model: model, Kind: "compaction", Data: json.RawMessage(`{"id":"cmp_1","type":"compaction","encrypted_content":"x"}`)},
				api.ContentText{Text: "answer"},
			}},
		},
	})
	want := `{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"","signature":"sig1"},` +
		`{"type":"thinking","thinking":"because","signature":"sig2"},` +
		`{"type":"thinking","thinking":"in short","signature":"sig3"},` +
		string(sealed) + `,` +
		string(result) + `,` +
		`{"type":"text","text":"[content omitted]"},` +
		`{"type":"text","text":"answer"}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

// TestThinkingReplayIsAllOrNothing pins the decision as a property of the
// message rather than of the block. Keeping part of a message's thinking is what
// this API refuses; keeping none of it costs tokens and continuity and nothing
// else. Each block that stays behind leaves a note, so the turn tells the model
// it thought without telling it what.
func TestThinkingReplayIsAllOrNothing(t *testing.T) {
	const model = "claude-opus-5"
	signed := json.RawMessage(`"sig"`)
	sealed := json.RawMessage(`{"type":"redacted_thinking","data":"EvwBCkYIA"}`)

	tests := []struct {
		name    string
		spoiler api.Content
	}{
		// Another provider's sealed payload is no block of this API's, so it
		// does not travel; the blocks beside it cannot travel without it.
		{"another provider", api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)}},
		{"an empty signature", api.ContentReasoning{Model: model, Thoughts: map[int]string{0: "x"}, Meta: json.RawMessage(`""`)}},
		// A block with no meta has no signature, and this API rejects unsigned
		// thinking, so it can never be sent and the rest goes with it.
		{"no meta", api.ContentReasoning{Model: model, Thoughts: map[int]string{0: "unsigned"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
				Messages: []api.Message{
					userText("go"),
					{Role: api.RoleAssistant, Content: api.Contents{
						api.ContentReasoning{Model: model, Thoughts: map[int]string{0: "mine"}, Meta: signed},
						api.ContentReasoning{Model: model, Meta: sealed},
						tc.spoiler,
						api.ContentText{Text: "answer"},
					}},
				},
			})
			want := `{"role":"assistant","content":[` +
				`{"type":"text","text":"[reasoning omitted]"},` +
				`{"type":"text","text":"[reasoning omitted]"},` +
				`{"type":"text","text":"[reasoning omitted]"},` +
				`{"type":"text","text":"answer"}]}`
			if !strings.Contains(string(got["messages"]), want) {
				t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
			}
		})
	}
}

// A message that cannot replay its own thinking says nothing about the message
// beside it: the decision is taken per message, and the rejected sequence is the
// one inside a single assistant turn.
func TestThinkingReplayIsPerMessage(t *testing.T) {
	const model = "claude-opus-5"
	signed := json.RawMessage(`"sig"`)
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
				api.ContentText{Text: "first"},
			}},
			userText("again"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: model, Thoughts: map[int]string{0: "mine"}, Meta: signed},
				api.ContentText{Text: "second"},
			}},
		},
	})
	msgs := string(got["messages"])
	if !strings.Contains(msgs, `{"role":"assistant","content":[{"type":"text","text":"[reasoning omitted]"},{"type":"text","text":"first"}]}`) {
		t.Fatalf("the first turn kept thinking it should have left behind:\n%s", msgs)
	}
	if !strings.Contains(msgs, `{"type":"thinking","thinking":"mine","signature":"sig"},{"type":"text","text":"second"}`) {
		t.Fatalf("the second turn lost thinking the first turn's blocks decided nothing about:\n%s", msgs)
	}
}

func TestOpaqueStaysByteExact(t *testing.T) {
	// Deliberately not in the order or spacing Go would marshal, because the
	// point is that nothing here re-encodes it.
	sealed := json.RawMessage(`{"tool_use_id":"srvtoolu_1","type":"web_search_tool_result","content":[1,2,3]}`)
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentOpaque{Model: "claude-opus-5", Kind: "web_search_tool_result", Data: sealed},
				api.ContentText{Text: "x"},
			}},
		},
	})
	if !strings.Contains(string(got["messages"]), string(sealed)) {
		t.Fatalf("sealed payload was rewritten:\n%s", got["messages"])
	}
}

// The fragments of a provider run tool's arguments were kept beside the payload
// rather than spliced into it, so this is where they go back in.
func TestOpaquePartsGoBackIntoThePayload(t *testing.T) {
	data := json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}`)
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentOpaque{
					Model:  testModel,
					Kind:   "server_tool_use",
					Data:   data,
					Deltas: []string{`{"query":`, `"x"}`},
				},
				api.ContentText{Text: "answer"},
			}},
		},
	})
	want := `{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"x"}}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

// A splice that cannot be done leaves the note every other unsendable block
// leaves, whichever half of the stored block went bad. The payload never goes
// out with the placeholder input it opened with, because a server tool call
// replayed with arguments it never had reads as one the model made, and failing
// the request would end the thread on stored history the caller cannot rewrite.
func TestOpaqueThatCannotBeSplicedIsANote(t *testing.T) {
	const model = "claude-opus-5"
	tests := []struct {
		name   string
		data   json.RawMessage
		deltas []string
	}{
		// The one a live turn reaches: cut inside a server tool's arguments.
		{"fragments that never parsed",
			json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}`),
			[]string{`{"query":`}},
		{"a payload that is not an object",
			json.RawMessage(`["server_tool_use"]`),
			[]string{`{"query":"x"}`}},
		{"a payload that stops mid value",
			json.RawMessage(`{"type":"server_tool_use","input":{`),
			[]string{`{"query":"x"}`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
				Messages: []api.Message{
					userText("go"),
					{Role: api.RoleAssistant, Content: api.Contents{
						api.ContentOpaque{Model: model, Kind: "server_tool_use", Data: tc.data, Deltas: tc.deltas},
						api.ContentText{Text: "answer"},
					}},
				},
			})
			want := `{"role":"assistant","content":[` +
				`{"type":"text","text":"[content omitted]"},` +
				`{"type":"text","text":"answer"}]}`
			msgs := string(got["messages"])
			if !strings.Contains(msgs, want) {
				t.Fatalf("messages =\n%s\nwant to contain\n%s", msgs, want)
			}
			if strings.Contains(msgs, "server_tool_use") {
				t.Fatalf("a payload that could not be spliced went out anyway:\n%s", msgs)
			}
		})
	}
}

func TestSpliceFieldKeepsEveryOtherByte(t *testing.T) {
	block := json.RawMessage(`{"z":[1,2],"input":{},"a":"a \"quoted\" thing","type":"server_tool_use"}`)
	out, err := spliceField(block, "input", json.RawMessage(`{"q":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"z":[1,2],"input":{"q":"v"},"a":"a \"quoted\" thing","type":"server_tool_use"}`
	if string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
	// A payload with no such field has nothing to splice, so it goes back as it
	// stands: only a block that streams a field collects fragments of one, and
	// every block of this API that streams one opened with it.
	if out, err := spliceField(block, "nowhere", json.RawMessage(`1`)); err != nil || string(out) != string(block) {
		t.Fatalf("splicing into a field that is not there gave (%s, %v), want the payload unchanged", out, err)
	}
	// Both refusals below are the caller's to turn into a note, which is what
	// TestOpaqueThatCannotBeSplicedIsANote pins; here they are only the report
	// that the bytes were not what they were stored as.
	if _, err := spliceField(block, "input", json.RawMessage(`{"q":`)); err == nil {
		t.Fatalf("splicing in a fragment that never became JSON succeeded")
	}
	if _, err := spliceField(json.RawMessage(`[]`), "input", json.RawMessage(`1`)); err == nil {
		t.Fatalf("splicing into a payload that is not an object succeeded")
	}
}

func TestCacheMarkers(t *testing.T) {
	req := api.Request{
		Tools:    []api.Tool{{Name: "a"}, {Name: "b"}},
		System:   "one\ntwo",
		Messages: []api.Message{userText("first"), userText("second")},
	}
	p, err := New(Conf{Model: api.Model{Name: testModel}}, WithCacheTTL(CacheTTL1H))
	if err != nil {
		t.Fatal(err)
	}
	got := mustEncode(t, p, req)
	mark := `"cache_control":{"type":"ephemeral","ttl":"1h"}`
	if tools := string(got["tools"]); strings.Contains(tools, "cache_control") {
		t.Fatalf("tools carry a marker beside the system one: %s", tools)
	}
	system := string(got["system"])
	if want := `{"cache_control":{"type":"ephemeral","ttl":"1h"},"type":"text","text":"one\ntwo"}`; !strings.Contains(system, want) {
		t.Fatalf("system = %s, want to contain %s", system, want)
	}
	messages := string(got["messages"])
	if strings.Count(messages, mark) != 1 || strings.Index(messages, mark) < strings.Index(messages, `"text":"first"`) {
		t.Fatalf("messages = %s, want one marker on the last message", messages)
	}

	req.System = ""
	got = mustEncode(t, p, req)
	tools := string(got["tools"])
	if strings.Count(tools, mark) != 1 || strings.Index(tools, mark) < strings.Index(tools, `"name":"b"`) {
		t.Fatalf("tools = %s, want one marker on the last tool", tools)
	}

	p, err = New(Conf{Model: api.Model{Name: testModel}}, WithoutCache())
	if err != nil {
		t.Fatal(err)
	}
	req.System = "one"
	for name, raw := range mustEncode(t, p, req) {
		if strings.Contains(string(raw), "cache_control") {
			t.Fatalf("%s carries a marker with caching off: %s", name, raw)
		}
	}
}

// A marker on a replayed block would change bytes the API checks for having
// been modified, so it goes on the last block that this adapter authored.
func TestCacheMarkerAvoidsReplayedBlocks(t *testing.T) {
	sealed := json.RawMessage(`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]}`)
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, withCache())
	got := mustEncode(t, p, api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentText{Text: "answer"},
				api.ContentReasoning{Model: "claude-opus-5", Thoughts: map[int]string{0: "t"}, Meta: json.RawMessage(`"s"`)},
				api.ContentOpaque{Model: "claude-opus-5", Kind: "web_search_tool_result", Data: sealed},
			}},
		},
	})
	messages := string(got["messages"])
	if !strings.Contains(messages, string(sealed)) {
		t.Fatalf("sealed payload was rewritten: %s", messages)
	}
	if !strings.Contains(messages, `{"cache_control":{"type":"ephemeral"},"type":"text","text":"answer"}`) {
		t.Fatalf("the marker did not land on the text block: %s", messages)
	}
	if strings.Count(messages, `"cache_control"`) != 1 {
		t.Fatalf("%d markers, want 1: %s", strings.Count(messages, `"cache_control"`), messages)
	}
}

func TestToolSchemaDefaults(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Tools:    []api.Tool{{Name: "noargs"}},
		Messages: []api.Message{userText("hi")},
	})
	if want := `"input_schema":{"type":"object","properties":{}}`; !strings.Contains(string(got["tools"]), want) {
		t.Fatalf("tools = %s, want to contain %s", got["tools"], want)
	}
	got = mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Tools:    []api.Tool{{Name: "array", Schema: map[string]any{"type": "array"}}},
		Messages: []api.Message{userText("hi")},
	})
	if want := `"input_schema":{"type":"array"}`; !strings.Contains(string(got["tools"]), want) {
		t.Fatalf("tools = %s, want the schema sent as it is: %s", got["tools"], want)
	}
}

func TestRequestOverModelRequestBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a body over the model's byte limit reached the API")
	}))
	t.Cleanup(srv.Close)
	over := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: 4096}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	req := api.Request{Messages: []api.Message{userText(strings.Repeat("x", 8192))}}
	body, err := encodeBody(over, req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	events := collect(t, over, req)
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if msg := e.Err.Error(); !strings.Contains(msg, fmt.Sprint(len(body.Bytes()))) || !strings.Contains(msg, "4096") {
		t.Fatalf("error %q names neither the %d bytes encoded nor the 4096 byte limit", msg, len(body.Bytes()))
	}

	sends := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(sends.Close)
	under := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: 1 << 20}}, WithAPIKey("k"), WithBaseURL(sends.URL))
	events = collect(t, under, req)
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	if e := events[0].(api.EventErr); e.Code != api.CodeNetwork {
		t.Fatalf("code = %q, want %q: a body under the limit has to be sent", e.Code, api.CodeNetwork)
	}
}

func TestProviderRefusesBytesMeasuredAsFitting(t *testing.T) {
	const declared = 1 << 20
	var sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent = len(body)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, "<html>413 Request Entity Too Large</html>")
	}))
	t.Cleanup(srv.Close)
	p := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: declared}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	events := collect(t, p, api.Request{Messages: []api.Message{userText("hi")}})
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
	msg := e.Err.Error()
	if sent == 0 {
		t.Fatalf("the server read no body, so there is no measured size to report")
	}
	if !strings.Contains(msg, fmt.Sprint(sent)) || !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q names neither the %d bytes sent nor the %d declared: nothing points at the misconfiguration", msg, sent, declared)
	}
	var carried *APIError
	if !errors.As(e.Err, &carried) || carried.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error %v lost what the API answered", e.Err)
	}

	small := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: 64}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	events = collect(t, small, api.Request{Messages: []api.Message{userText(strings.Repeat("x", 4096))}})
	refused, ok := events[0].(api.EventErr)
	if !ok || refused.Code != api.CodeTooLarge {
		t.Fatalf("a body over the declared limit: got %#v, want api.CodeTooLarge", events[0])
	}
}

func TestInbandRequestTooLargeIsMeasured(t *testing.T) {
	const declared = 1 << 20
	srv := httptest.NewServer(sseServer(sse(
		"message_start", fixStart,
		"error", `{"type":"error","error":{"type":"request_too_large","message":"too big"}}`,
	)))
	t.Cleanup(srv.Close)
	p := mustNew(t, Conf{Model: api.Model{Name: testModel, RequestBytes: declared}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	events := collect(t, p, api.Request{Messages: []api.Message{userText("hi")}})
	last, ok := events[len(events)-1].(api.EventErr)
	if !ok {
		t.Fatalf("the turn ended with %#v, want api.EventErr", events[len(events)-1])
	}
	if last.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: request_too_large is the in band spelling of the 413 (err %v)", last.Code, api.CodeTooLarge, last.Err)
	}
	if msg := last.Err.Error(); !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q does not name the %d declared", msg, declared)
	}
}

func TestUnmeasuredRequestTooLargeNamesNoLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, `{"type":"error","error":{"type":"request_too_large","message":"too big"}}`)
	}))
	t.Cleanup(srv.Close)
	inband := httptest.NewServer(sseServer(sse(
		"message_start", fixStart,
		"error", `{"type":"error","error":{"type":"request_too_large","message":"too big"}}`,
	)))
	t.Cleanup(inband.Close)

	for _, url := range []string{srv.URL, inband.URL} {
		p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithBaseURL(url))
		events := collect(t, p, api.Request{Messages: []api.Message{userText("hi")}})
		last, ok := events[len(events)-1].(api.EventErr)
		if !ok {
			t.Fatalf("the turn ended with %#v, want api.EventErr", events[len(events)-1])
		}
		if last.Code != api.CodeTooLarge {
			t.Fatalf("code = %q, want %q: the refusal is still one about bytes (err %v)", last.Code, api.CodeTooLarge, last.Err)
		}
		if msg := last.Err.Error(); strings.Contains(msg, "the 0 declared") {
			t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
		}
		var carried *APIError
		if !errors.As(last.Err, &carried) {
			t.Fatalf("error %v lost what the API answered", last.Err)
		}
	}
}

func TestName(t *testing.T) {
	if got := mustNew(t, Conf{Model: api.Model{Name: testModel}}).Name(); got != providerName {
		t.Fatalf("Name = %q, want %q", got, providerName)
	}
}

// Model reads back as configured: a listed model with its window, a caller's
// literal as written, where zero is unknown rather than a window of zero.
func TestDeclaredModels(t *testing.T) {
	for _, m := range []api.Model{Fable51, Opus55, Sonnet55, Haiku55, Sonnet5} {
		if m.Context != 1_000_000 || m.ImagePixels != 4784*28*28 || m.ImageSide != 2576 || m.ImagesPerRequest != 600 {
			t.Errorf("%s: %+v, want the 1M window, the high resolution tier and 600 images", m.Name, m)
		}
	}
	if Haiku45.Context != 200_000 || Haiku45.ImagePixels != 1568*28*28 || Haiku45.ImageSide != 1568 || Haiku45.ImagesPerRequest != 100 {
		t.Errorf("%s: %+v, want the 200k window, the standard tier and 100 images", Haiku45.Name, Haiku45)
	}
	for _, m := range []api.Model{Fable51, Opus55, Sonnet55, Haiku55, Sonnet5, Haiku45} {
		if m.RequestBytes != 32_000_000 || m.ImageBytes != 10_000_000*3/4 {
			t.Errorf("%s: request %d, image %d bytes, want 32 MB and the raw size of 10 MB base64", m.Name, m.RequestBytes, m.ImageBytes)
		}
	}
}

func TestNewRefusesInvalidSettings(t *testing.T) {
	model := Conf{Model: api.Model{Name: testModel}}
	mustRefuse(t, Conf{}, "no name")
	mustRefuse(t, model, "claude: empty base URL", WithBaseURL(""))
	mustRefuse(t, model, "claude: base URL", WithBaseURL("no-scheme"))
	if got := mustNew(t, model, WithStopSequences("a", "b"), WithStopSequences("c")).stopSeqs; len(got) != 1 || got[0] != "c" {
		t.Errorf("stop sequences %q, want the last call's alone", got)
	}
	mustRefuse(t, model, "needs a client", WithHTTPClient(nil))
	mustRefuse(t, model, "negative", WithMaxTokens(-1))
	mustRefuse(t, model, "needs a version", WithVersion(""))
	if got := mustNew(t, model, WithMaxTokens(100), WithMaxTokens(0)).maxToken; got != defaultMaxTokens {
		t.Errorf("WithMaxTokens(0) left %d, want the default %d", got, defaultMaxTokens)
	}

	haiku := Conf{Model: Haiku45}
	mustRefuse(t, haiku, "only with enabled", WithThinking(Thinking{Type: ThinkingAdaptive, Budget: 2048}))
	mustRefuse(t, haiku, "only with enabled", WithThinking(Thinking{Budget: 2048}))
	mustRefuse(t, haiku, "under the 1024", WithThinking(Thinking{Type: ThinkingEnabled, Budget: 1023}))
	mustRefuse(t, haiku, "not below max tokens", WithMaxTokens(4096), WithThinking(Thinking{Type: ThinkingEnabled, Budget: 4096}))
	mustNew(t, haiku, WithMaxTokens(4096), WithThinking(Thinking{Type: ThinkingEnabled, Budget: 4095}))
	mustNew(t, haiku, WithMaxTokens(4096), WithBeta("interleaved-thinking-2025-05-14"), WithThinking(Thinking{Type: ThinkingEnabled, Budget: 8192}))
	mustRefuse(t, haiku, "needs thinking that is on", WithThinking(Thinking{Display: DisplaySummarized}))
	mustRefuse(t, haiku, "needs thinking that is on", WithThinking(Thinking{Type: ThinkingDisabled, Display: DisplayOmitted}))

	enabled := WithThinking(Thinking{Type: ThinkingEnabled, Budget: 2048})
	disabled := WithThinking(Thinking{Type: ThinkingDisabled})
	adaptive := WithThinking(Thinking{Type: ThinkingAdaptive})
	for _, m := range []api.Model{Fable51, Opus55, Sonnet55} {
		mustRefuse(t, Conf{Model: m}, "refuses enabled thinking", enabled)
		mustRefuse(t, Conf{Model: m}, "refuses disabled thinking", disabled)
		mustNew(t, Conf{Model: m}, adaptive, WithEffort(EffortMax), WithMaxTokens(128_000))
		mustRefuse(t, Conf{Model: m}, "over the 128000", WithMaxTokens(128_001))
	}
	for _, m := range []api.Model{Haiku55, Sonnet5} {
		mustRefuse(t, Conf{Model: m}, "refuses enabled thinking", enabled)
		mustNew(t, Conf{Model: m}, disabled, WithEffort(EffortHigh))
	}
	mustNew(t, Conf{Model: Sonnet5}, disabled, WithEffort(EffortMax))
	mustNew(t, Conf{Model: Haiku55}, disabled)
	mustRefuse(t, Conf{Model: Haiku55}, "at effort xhigh", disabled, WithEffort(EffortXHigh))
	mustRefuse(t, Conf{Model: Haiku55}, "at effort max", disabled, WithEffort(EffortMax))
	mustNew(t, Conf{Model: Haiku55}, adaptive, WithEffort(EffortMax))

	mustRefuse(t, Conf{Model: Haiku45}, "refuses adaptive thinking", adaptive)
	mustRefuse(t, Conf{Model: Haiku45}, "takes no effort", WithEffort(EffortLow))
	mustRefuse(t, Conf{Model: Haiku45}, "over the 64000", WithMaxTokens(64_001))
	mustNew(t, Conf{Model: Haiku45}, enabled, WithMaxTokens(64_000))

	own := Conf{Model: api.Model{Name: Opus55.Name}}
	mustRefuse(t, own, "refuses disabled thinking", disabled)
	mustNew(t, Conf{Model: api.Model{Name: "claude-opus-4-6"}}, disabled, WithEffort(EffortMax), WithMaxTokens(1_000_000))
	mustNew(t, model, WithThinking(Thinking{Type: ThinkingAdaptive, Budget: 512, Display: DisplaySummarized}))
	mustNew(t, model, WithMaxTokens(4096), WithThinking(Thinking{Type: ThinkingEnabled, Budget: 8192}))
	mustNew(t, model, WithThinking(Thinking{Type: ThinkingDisabled, Display: DisplayOmitted}))
}

func TestDocumentInAssistantMessageIsANote(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), api.Request{
		Messages: []api.Message{
			userText("hi"),
			{Role: api.RoleAssistant, Content: api.Contents{api.ContentDocument{Data: []byte("%PDF"), Mime: string(api.DocumentPDF)}, api.ContentText{Text: "here"}}},
		},
	})
	msgs := string(got["messages"])
	if strings.Contains(msgs, `"type":"document"`) {
		t.Fatalf("a document went out in an assistant message:\n%s", msgs)
	}
	if !strings.Contains(msgs, `{"role":"assistant","content":[{"type":"text","text":"[`) {
		t.Fatalf("no note in place of the document:\n%s", msgs)
	}
}

func TestModel(t *testing.T) {
	var p api.Provider = mustNew(t, Conf{Model: Haiku45})
	if got := p.Model(); got != Haiku45 || got.Context != 200_000 {
		t.Fatalf("Model = %+v, want %+v", got, Haiku45)
	}
	own := api.Model{Name: testModel}
	if got := mustNew(t, Conf{Model: own}).Model(); got != own {
		t.Fatalf("Model = %+v, want the literal with an unknown window", got)
	}
}

func TestThinkingAnotherClaudeModelWroteReplays(t *testing.T) {
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: "claude-opus-5-5"}}), api.Request{
		Messages: []api.Message{
			userText("go"),
			{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: "claude-opus-5", Thoughts: map[int]string{0: "earlier"}, Meta: json.RawMessage(`"sig"`)},
				api.ContentReasoning{Model: "claude-opus-5", Meta: json.RawMessage(`{"type":"redacted_thinking","data":"EvwBCkYIA"}`)},
				api.ContentText{Text: "answer"},
			}},
		},
	})
	want := `{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"earlier","signature":"sig"},` +
		`{"type":"redacted_thinking","data":"EvwBCkYIA"},` +
		`{"type":"text","text":"answer"}]}`
	if !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
	}
}

func TestDocumentsInAMessage(t *testing.T) {
	user := func(c api.Content) api.Request {
		return api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{c, api.ContentText{Text: "read it"}}}}}
	}
	for _, mime := range []api.DocumentMime{
		api.DocumentText, api.DocumentMarkdown, api.DocumentCSV, api.DocumentHTML,
		api.DocumentXML, api.DocumentJSON, api.DocumentYAML, api.DocumentRTF,
	} {
		got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), user(api.ContentDocument{Data: []byte("# a,b"), Mime: string(mime)}))
		want := `{"type":"document","source":{"type":"text","media_type":"text/plain","data":"# a,b"}}`
		if !strings.Contains(string(got["messages"]), want) {
			t.Fatalf("%s: messages =\n%s\nwant to contain\n%s", mime, got["messages"], want)
		}
	}
	for _, c := range []api.ContentDocument{
		{Data: []byte("PK"), Mime: string(api.DocumentDOCX)},
		{Data: []byte("PK"), Mime: string(api.DocumentXLSX)},
		{Data: []byte("PK"), Mime: string(api.DocumentPPTX)},
		{Data: []byte("x"), Mime: string(api.DocumentDOC)},
		{Data: []byte("x"), Mime: string(api.DocumentXLS)},
		{Data: []byte("PK"), Mime: "application/zip"},
		{Data: []byte{0xff, 0xfe, 'a'}, Mime: string(api.DocumentMarkdown)},
	} {
		p := mustNew(t, Conf{Model: api.Model{Name: testModel}})
		first := mustEncode(t, p, user(c))
		second := mustEncode(t, p, user(c))
		want := `{"role":"user","content":[{"type":"text","text":"[document omitted]"},{"type":"text","text":"read it"}]}`
		if !strings.Contains(string(first["messages"]), want) {
			t.Fatalf("%s: messages =\n%s\nwant to contain\n%s", c.Mime, first["messages"], want)
		}
		if string(first["messages"]) != string(second["messages"]) {
			t.Fatalf("%s: a second turn encoded differently:\n%s\n%s", c.Mime, first["messages"], second["messages"])
		}
	}
	got := mustEncode(t, mustNew(t, Conf{Model: api.Model{Name: testModel}}), user(api.ContentDocument{Data: []byte("%PDF"), Mime: string(api.DocumentPDF)}))
	if want := `{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBERg=="}}`; !strings.Contains(string(got["messages"]), want) {
		t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
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

func TestEncodeLogsOnlyAnUnknownTypeOnce(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"))
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })
	req := api.Request{Messages: []api.Message{
		{Role: api.RoleUser, Content: api.Contents{api.ContentImage{Data: []byte{1}, Mime: "image/bmp"}, nil}},
		{Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "toolu_1", Name: "read", Input: json.RawMessage(`{}`)}}},
		{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: "toolu_1"}}},
	}}
	for range 2 {
		got := mustEncode(t, p, req)
		for _, want := range []string{
			`{"role":"user","content":[{"type":"text","text":"[image omitted]"},{"type":"text","text":"[content omitted]"}]}`,
			`"content":[{"type":"text","text":"(the tool returned no output)"}]`,
		} {
			if !strings.Contains(string(got["messages"]), want) {
				t.Fatalf("messages =\n%s\nwant to contain\n%s", got["messages"], want)
			}
		}
	}
	if counts[slog.LevelWarn] != 0 || counts[slog.LevelError] > 1 {
		t.Fatalf("two encodes logged %v, want no warnings and at most one error", map[slog.Level]int(counts))
	}
}

func TestAPIKeySource(t *testing.T) {
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })

	t.Setenv("ANTHROPIC_API_KEY", "env-key")
	if p := mustNew(t, Conf{Model: api.Model{Name: testModel}}); p.key != "env-key" {
		t.Fatalf("key %q, want the environment's", p.key)
	}
	if p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("")); p.key != "" {
		t.Fatalf("key %q, want the explicit empty one", p.key)
	}
	t.Setenv("ANTHROPIC_API_KEY", "")
	mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey(""))
	if counts[slog.LevelWarn] != 0 {
		t.Fatalf("an explicit key warned %d times", counts[slog.LevelWarn])
	}
	if p := mustNew(t, Conf{Model: api.Model{Name: testModel}}); p.key != "" {
		t.Fatalf("key %q, want none", p.key)
	}
	if len(counts) != 0 {
		t.Fatalf("a missing key logged %v, want nothing", map[slog.Level]int(counts))
	}
}
