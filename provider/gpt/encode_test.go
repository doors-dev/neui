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

package gpt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func encodeBody(t *testing.T, model string, set settings, req api.Request) map[string]any {
	t.Helper()
	b, err := (&Provider{model: api.Model{Name: model}, set: set}).encode(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// sameJSON compares a body field against expected JSON, ignoring key order and
// number formatting.
func sameJSON(t *testing.T, got any, want string) {
	t.Helper()
	gotRaw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(gotRaw, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatalf("bad want JSON: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("\n got: %s\nwant: %s", gotRaw, want)
	}
}

func user(cs ...api.Content) api.Message {
	return api.Message{Role: api.RoleUser, Content: api.Contents(cs)}
}

func assistant(cs ...api.Content) api.Message {
	return api.Message{Role: api.RoleAssistant, Content: api.Contents(cs)}
}

func phaseMeta(phase string) json.RawMessage {
	raw, err := json.Marshal(messageMeta{Phase: phase})
	if err != nil {
		panic(err)
	}
	return lib.Meta(providerName, raw)
}

func ownMeta(item string) json.RawMessage {
	return lib.Meta(providerName, json.RawMessage(item))
}

func TestEncodeInput(t *testing.T) {
	tests := []struct {
		name     string
		model    string // the provider's model, testModel unless a case is about the name
		settings func(s *settings)
		req      api.Request
		want     string
	}{
		{
			name: "user text becomes a typed block array",
			req:  api.Request{Messages: []api.Message{user(api.ContentText{Text: "hi"})}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`,
		},
		{
			name: "an image rides inline as a data url with an explicit detail",
			req: api.Request{Messages: []api.Message{user(
				api.ContentText{Text: "what is this"},
				api.ContentImage{Data: []byte{0x89, 0x50}, Mime: "image/png"},
			)}},
			want: `[{"role":"user","content":[
				{"type":"input_text","text":"what is this"},
				{"type":"input_image","image_url":"data:image/png;base64,iVA=","detail":"high"}]}]`,
		},
		{
			name: "a document without a name gets one that matches its media type",
			req: api.Request{Messages: []api.Message{user(
				api.ContentDocument{Data: []byte("%PDF"), Mime: "application/pdf"},
			)}},
			want: `[{"role":"user","content":[
				{"type":"input_file","filename":"document.pdf","file_data":"data:application/pdf;base64,JVBERg=="}]}]`,
		},
		{
			// The service refuses a part whose data URL carries nothing, and a
			// user message is stored and replayed, so one stored block sent as a
			// part would be refused on every later turn of the thread rather than
			// on this one alone. The emptiness is read before the media type, so
			// neither refusal these arms have fires on a block that could not be
			// sent whatever it claimed to be.
			name: "an attachment with no bytes in it leaves a note rather than a part",
			req: api.Request{Messages: []api.Message{user(
				api.ContentImage{Mime: "image/png"},
				api.ContentImage{Mime: "image/svg+xml"},
				api.ContentDocument{Mime: "application/pdf"},
				api.ContentDocument{},
				api.ContentText{Text: "what is it"},
			)}},
			want: `[{"role":"user","content":[
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"[document omitted]"},
				{"type":"input_text","text":"[document omitted]"},
				{"type":"input_text","text":"what is it"}]}]`,
		},
		{
			name: "an attachment with no bytes in it leaves a note inside a result too",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{
					api.ContentImage{Mime: "image/png"},
					api.ContentDocument{Mime: "application/pdf"},
					api.ContentText{Text: "here"},
				}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"[document omitted]"},
				{"type":"input_text","text":"here"}]}]`,
		},
		{
			name: "a result image the service refuses by type or a result document with no type leaves a note",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{
					api.ContentImage{Data: []byte("<svg/>"), Mime: "image/svg+xml"},
					api.ContentDocument{Data: []byte("%PDF")},
					api.ContentText{Text: "here"},
				}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"[document omitted]"},
				{"type":"input_text","text":"here"}]}]`,
		},
		{
			name: "assistant text goes back as output_text parts, the only text part an assistant message takes",
			req: api.Request{Messages: []api.Message{
				user(api.ContentText{Text: "hi"}),
				assistant(api.ContentText{Text: "hello"}),
				user(api.ContentText{Text: "again"}),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"hi"}]},
				{"role":"assistant","content":[{"type":"output_text","text":"hello"}]},
				{"role":"user","content":[{"type":"input_text","text":"again"}]}]`,
		},
		{
			name: "an assistant text document goes as output_text and any other attachment leaves a note",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentDocument{Data: []byte("plain words"), Mime: " Text/Plain "},
					api.ContentDocument{Data: []byte(`{"a":1}`), Mime: string(api.DocumentJSON)},
					api.ContentDocument{Data: []byte("%PDF"), Mime: string(api.DocumentPDF)},
					api.ContentDocument{Data: []byte{0xff, 0xfe}, Mime: string(api.DocumentText)},
					api.ContentDocument{Mime: string(api.DocumentText)},
					api.ContentImage{Data: []byte{0x89, 0x50}, Mime: "image/png"},
				),
			}},
			want: `[{"role":"assistant","content":[
					{"type":"output_text","text":"plain words"},
					{"type":"output_text","text":"{\"a\":1}"},
					{"type":"output_text","text":"[document omitted]"},
					{"type":"output_text","text":"[document omitted]"},
					{"type":"output_text","text":"[document omitted]"},
					{"type":"output_text","text":"[image omitted]"}]}]`,
		},
		{
			name: "one text block is one part, and a call splits the run without joining anything",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "Let me check."},
					api.ContentText{Text: "Two things."},
					api.ContentToolUse{ID: "call_a", Name: "search", Input: json.RawMessage(`{}`)},
					api.ContentText{Text: "Found it."},
				),
			}},
			want: `[{"role":"assistant","content":[
					{"type":"output_text","text":"Let me check."},
					{"type":"output_text","text":"Two things."}]},
				{"type":"function_call","call_id":"call_a","name":"search","arguments":"{}"},
				{"role":"assistant","content":[
					{"type":"output_text","text":"Found it."}]}]`,
		},
		{
			name: "a tool call and its result are top level items, not blocks",
			req: api.Request{Messages: []api.Message{
				user(api.ContentText{Text: "weather?"}),
				assistant(
					api.ContentText{Text: "Looking."},
					api.ContentToolUse{ID: "call_a", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
				),
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "18C"}}}),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"weather?"}]},
				{"role":"assistant","content":[{"type":"output_text","text":"Looking."}]},
				{"type":"function_call","call_id":"call_a","name":"get_weather","arguments":"{\"city\":\"Paris\"}"},
				{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"18C"}]}]`,
		},
		{
			name: "a result written after text stays after it",
			req: api.Request{Messages: []api.Message{
				user(
					api.ContentText{Text: "here you go"},
					api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "18C"}}},
					api.ContentText{Text: "anything else?"},
				),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"here you go"}]},
				{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"18C"}]},
				{"role":"user","content":[{"type":"input_text","text":"anything else?"}]}]`,
		},
		{
			name: "a failed tool result says so in a part of its own, because there is no flag for it",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{
					ID:      "call_a",
					Content: api.UserContents{api.ContentText{Text: "boom"}},
					Error:   api.ToolErrRuntime,
				}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"Error:"},
				{"type":"input_text","text":"boom"}]}]`,
		},
		{
			name: "two text blocks in a tool result stay two parts",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{
					api.ContentText{Text: "first"},
					api.ContentText{Text: "second"},
				}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"first"},
				{"type":"input_text","text":"second"}]}]`,
		},
		{
			name: "a tool result carrying an image uses the same block array as one carrying only text",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{
					api.ContentText{Text: "here"},
					api.ContentImage{Data: []byte{0x01}, Mime: "image/jpeg"},
				}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"here"},
				{"type":"input_image","image_url":"data:image/jpeg;base64,AQ==","detail":"high"}]}]`,
		},
		{
			// Every kind the subset admits today has an output part here, so what
			// this branch is for is the kind a later version of this protocol
			// adds, and one cannot be declared outside protocol: the nil block
			// stands in for it. The note is what a block of a message leaves, and
			// the rest of the result still answers the call.
			name: "a result block this version has no part for leaves a note",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{
					api.ContentText{Text: "here"},
					nil,
				}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[
				{"type":"input_text","text":"here"},
				{"type":"input_text","text":"[content omitted]"}]}]`,
		},
		{
			// The result reporting err_json is what says the arguments are gone,
			// so the sentinel in Input is never read and never leaves the
			// process, and a call that was stored with no arguments at all
			// replays the same way.
			name: "arguments that never parsed replay as an empty object",
			req: api.Request{Messages: []api.Message{
				assistant(api.ContentToolUse{ID: "call_a", Name: "search", Input: api.DiscardedArguments}),
				user(api.ContentToolResult{ID: "call_a", Error: api.ToolErrJSON, Content: api.UserContents{
					api.ContentText{Text: api.DiscardedArgumentsError{Raw: `{"q`}.Error()},
				}}),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"search","arguments":"{}"},
				{"type":"function_call_output","call_id":"call_a","output":[
					{"type":"input_text","text":"Error:"},
					{"type":"input_text","text":"tool call arguments were not valid JSON, the call was not made, reissue it with complete arguments: {\"q"}]}]`,
		},
		{
			name: "the service's own encoding of a reasoning item goes back as it arrived",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{
						Model:   "gpt-5.6-sol",
						Summary: map[int]string{0: "thinking"},
						Meta:    ownMeta(`{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"SEALED"}`),
					},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "reasoning with no encoding of its own sends nothing, and the call it produced still goes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "thinking"}},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			// What is left in its place is the note: the summary is the
			// service's paraphrase of thinking it never showed and never
			// signed, so it does not go back as the model's own words. The call
			// is untouched, because nothing the adapter sends on a
			// function_call names the reasoning item the service would pair it
			// with, and the result answering it is in the message after this
			// one.
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "a reasoning item that cannot travel takes nothing beside it with it",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "unsealed"}},
					api.ContentToolUse{ID: "call_b", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"type":"function_call","call_id":"call_b","name":"t","arguments":"{}"}]`,
		},
		{
			name: "every reasoning item this provider issued goes back, the last of a turn included",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","encrypted_content":"ONE"}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_2","type":"reasoning","encrypted_content":"TWO"}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_3","type":"reasoning","encrypted_content":"THREE"}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"ONE"},
				{"id":"rs_2","type":"reasoning","encrypted_content":"TWO"},
				{"id":"rs_3","type":"reasoning","encrypted_content":"THREE"}]`,
		},
		{
			name: "reasoning that only preceded text goes back",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentText{Text: "the answer"},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"LAST"}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"},
				{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]},
				{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"LAST"}]`,
		},
		{
			name:     "with store on, a reasoning item goes back only when a call or a hosted tool follows it",
			settings: func(s *settings) { s.store = true },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[]}`)},
					api.ContentText{Text: "looking"},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_2","type":"reasoning","summary":[]}`)},
					api.ContentText{Text: "the answer"},
				),
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "ok"}}}),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[]},
				{"role":"assistant","content":[{"type":"output_text","text":"looking"}]},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]},
				{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"ok"}]}]`,
		},
		{
			// The reasoning items are the whole of what this mode removes. A
			// call and the result answering it are separate items in
			// consecutive messages, and a pass that sees one message would
			// strand the output by dropping the call.
			name:     "ReplayNever drops every reasoning item and nothing else",
			settings: func(s *settings) { s.replay = ReplayNever },
			req: api.Request{Messages: []api.Message{
				user(api.ContentText{Text: "weather?"}),
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
				),
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "18C"}}}),
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"ALSO"}`)},
					api.ContentText{Text: "It is 18C."},
				),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"weather?"}]},
				{"type":"function_call","call_id":"call_a","name":"get_weather","arguments":"{\"city\":\"Paris\"}"},
				{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"18C"}]},
				{"role":"assistant","content":[{"type":"output_text","text":"It is 18C."}]}]`,
		},
		{
			name: "reasoning carrying neither a summary nor an encoding of its own sends nothing, and its call goes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol"},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			// A block with nothing to send becomes no item at all, so the note
			// standing for it is a part like any other and the text either side
			// of it stays one message.
			name: "a reasoning block with nothing to send leaves the text either side of it one message",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "so far"},
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "thinking"}},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[
				{"type":"output_text","text":"so far"},
				{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			// A reasoning item names no model anywhere in it and its payload is
			// sealed against the organization rather than against a model, so
			// there is nothing here for a model comparison to be right about.
			name: "a reasoning item replays whatever model stamped it, and whether it was stamped at all",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.1", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Meta: ownMeta(`{"id":"rs_2","type":"reasoning","encrypted_content":"ALSO"}`)},
					api.ContentToolUse{ID: "call_b", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"id":"rs_2","type":"reasoning","encrypted_content":"ALSO"},
				{"type":"function_call","call_id":"call_b","name":"t","arguments":"{}"}]`,
		},
		{
			name: "a phase replays whichever model of this provider produced it",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "the answer", Meta: phaseMeta("commentary")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"the answer"}],"phase":"commentary"}]`,
		},
		{
			name:  "a phase replays under a gateway spelling of the model too",
			model: "openai/GPT-5.6-sol",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "the answer", Meta: phaseMeta("commentary")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"the answer"}],"phase":"commentary"}]`,
		},
		{
			// The label is on the block, so it travels with it wherever the block
			// sits, and a message ends where its phase stops applying.
			name: "phase labels the message the block it is on becomes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me look", Meta: phaseMeta("commentary")},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentText{Text: "the answer", Meta: phaseMeta("final_answer")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"let me look"}],"phase":"commentary"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"role":"assistant","content":[{"type":"output_text","text":"the answer"}],"phase":"final_answer"}]`,
		},
		{
			// A run is one message only as far as the blocks agree, because a
			// phase is a claim about the message and two of them cannot be one.
			name: "a change of phase inside a run ends the message",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me look", Meta: phaseMeta("commentary")},
					api.ContentText{Text: "still looking", Meta: phaseMeta("commentary")},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[
					{"type":"output_text","text":"let me look"},
					{"type":"output_text","text":"still looking"}],"phase":"commentary"},
				{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			// Meta another adapter wrote parses to nothing this one recognizes,
			// which claims no phase, which is what a block with no meta claims.
			name: "meta of a shape this adapter never wrote labels nothing",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "the answer", Meta: json.RawMessage(`"c3ln"`)},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			// An opaque payload is an output item the input array takes back, so
			// it goes out as the bytes that arrived, as a top level item that
			// ends the run of text around it.
			name: "an opaque payload goes back as the item it arrived as",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me search"},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "gpt.web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"let me search"}]},
				{"id":"ws_1","type":"web_search_call","status":"completed"},
				{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			// The finished item carried the field its fragments streamed, so the
			// parts are what the field looked like on the way there and nothing
			// puts them back.
			name: "the parts of an opaque payload are not spliced back into it",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentOpaque{
						Model:  "gpt-5.6-sol",
						Kind:   "gpt.code_interpreter_call",
						Data:   json.RawMessage(`{"id":"ci_1","type":"code_interpreter_call","code":"print(1)"}`),
						Deltas: []string{"print(", "1)"},
					},
				),
			}},
			want: `[{"id":"ci_1","type":"code_interpreter_call","code":"print(1)"}]`,
		},
		{
			// A tool the service ran is the item the thinking in front of it
			// produced, so the two pair the way a reasoning item and a function
			// call do and a turn that only searched replays its reasoning.
			name: "a hosted tool call carries the reasoning in front of it",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "gpt.web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"},
				{"id":"ws_1","type":"web_search_call","status":"completed"}]`,
		},
		{
			name: "a Claude signature as meta is another provider's and sends nothing, and the call goes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "claude-opus-5", Thoughts: map[int]string{0: "theirs"}, Meta: lib.Meta("claude", json.RawMessage(`"EqQBCkYIBRgC"`))},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "a redacted_thinking block as meta is another provider's",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "claude-opus-5", Meta: lib.Meta("claude", json.RawMessage(`{"type":"redacted_thinking","data":"EvwBCkYIA"}`))},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "a reasoning item stored before Meta named its provider is not this provider's",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"x_1","type":"reasoning","encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "Messages API server tool blocks are another provider's and send nothing",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentOpaque{Model: "claude-opus-5", Kind: "claude.server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"q"}}`)},
					api.ContentOpaque{Model: "claude-opus-5", Kind: "claude.web_search_tool_result", Data: json.RawMessage(`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]}`)},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			name: "an opaque payload whose type is not its kind is not this service's",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "gpt.web_search_call", Data: json.RawMessage(`{"id":"ci_1","type":"code_interpreter_call"}`)},
				),
			}},
			want: `[]`,
		},
		{
			name:  "a reasoning item another model of the family made goes back byte for byte",
			model: "gpt-5.5",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "with store off a reasoning item with no encrypted content sends nothing, and the call goes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "thinking"}, Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name:     "with store on a reasoning item replays by id",
			settings: func(s *settings) { s.store = true },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[]}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[]},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name:     "with store on a reasoning item with nothing after it is still not sent",
			settings: func(s *settings) { s.store = true },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "so far"},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[]}`)},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"output_text","text":"so far"}]}]`,
		},
		{
			name: "a commentary preamble between reasoning and its call keeps the pairing",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentText{Text: "let me look", Meta: phaseMeta("commentary")},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"},
				{"role":"assistant","content":[{"type":"output_text","text":"let me look"}],"phase":"commentary"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "two reasoning items before one call both go back",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"ONE"}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"TWO"}`)},
					api.ContentText{Text: "let me look"},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"ONE"},
				{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"TWO"},
				{"role":"assistant","content":[{"type":"output_text","text":"let me look"}]},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "reasoning last in a turn goes back",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
				),
			}},
			want: `[{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}]`,
		},
		{
			name: "a user image the service refuses by type and a user document with no type leave notes",
			req: api.Request{Messages: []api.Message{
				user(
					api.ContentImage{Data: []byte("BM"), Mime: "image/bmp"},
					api.ContentDocument{Data: []byte("%PDF")},
					api.ContentText{Text: "here"},
				),
			}},
			want: `[{"role":"user","content":[
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"[document omitted]"},
				{"type":"input_text","text":"here"}]}]`,
		},
		{
			name: "a docx goes as a file named for its type",
			req: api.Request{Messages: []api.Message{
				user(api.ContentDocument{Data: []byte("PK"), Mime: string(api.DocumentDOCX)}),
			}},
			want: `[{"role":"user","content":[{"type":"input_file","filename":"document.docx","file_data":"data:application/vnd.openxmlformats-officedocument.wordprocessingml.document;base64,UEs="}]}]`,
		},
		{
			name: "a result with no blocks says the tool returned no output",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a"}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"(the tool returned no output)"}]}]`,
		},
		{
			name: "a failed result with no blocks is only its error part",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Error: api.ToolErrRuntime}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"Error:"}]}]`,
		},
		{
			name: "a result whose only block is an empty image keeps the note rather than the empty-result text",
			req: api.Request{Messages: []api.Message{
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentImage{Mime: "image/png"}}}),
			}},
			want: `[{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"[image omitted]"}]}]`,
		},
		{
			name: "an attachment on an assistant turn leaves a note, because this wire has no part for one",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentImage{Data: []byte("x"), Mime: "image/png"},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[
				{"type":"output_text","text":"[image omitted]"},
				{"type":"output_text","text":"the answer"}]}]`,
		},
		{
			name: "reasoning and a payload in a user turn send nothing",
			req: api.Request{Messages: []api.Message{
				user(
					api.ContentReasoning{Model: "gpt-5.6-sol", Thoughts: map[int]string{0: "theirs"}},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "gpt.web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
					api.ContentText{Text: "carry on"},
				),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"carry on"}]}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set := defaultSettings()
			if tc.settings != nil {
				tc.settings(&set)
			}
			model := tc.model
			if model == "" {
				model = testModel
			}
			req, want := paired(t, tc.req, tc.want)
			body := encodeBody(t, model, set, req)
			sameJSON(t, body["input"], want)
		})
	}
}

// paired completes a case written around one item: a call no later result
// answers gets a result in a message of its own after the request, and a result
// no earlier call made gets a call in a message of its own before it, with the
// items they encode to around want.
func paired(t *testing.T, req api.Request, want string) (api.Request, string) {
	t.Helper()
	var before, after api.Contents
	var head, tail []any
	for i, m := range req.Messages {
		for j, c := range m.Content {
			switch c := c.(type) {
			case api.ContentToolUse:
				answered := false
				for k, n := range req.Messages[i:] {
					for l, d := range n.Content {
						r, is := d.(api.ContentToolResult)
						answered = answered || is && r.ID == c.ID && (k > 0 || l > j)
					}
				}
				if !answered {
					after = append(after, api.ContentToolResult{ID: c.ID, Content: api.UserContents{api.ContentText{Text: "ok"}}})
					tail = append(tail, map[string]any{"type": "function_call_output", "call_id": lib.CallID(c.ID), "output": []any{map[string]any{"type": "input_text", "text": "ok"}}})
				}
			case api.ContentToolResult:
				called := false
				for k, n := range req.Messages[:i+1] {
					for l, d := range n.Content {
						u, is := d.(api.ContentToolUse)
						called = called || is && u.ID == c.ID && (k < i || l < j)
					}
				}
				if !called {
					before = append(before, api.ContentToolUse{ID: c.ID, Name: "t"})
					head = append(head, map[string]any{"type": "function_call", "call_id": lib.CallID(c.ID), "name": "t", "arguments": "{}"})
				}
			}
		}
	}
	if len(before) > 0 {
		req.Messages = append([]api.Message{assistant(before...)}, req.Messages...)
	}
	if len(after) > 0 {
		req.Messages = append(slices.Clone(req.Messages), user(after...))
	}
	var items []any
	if err := json.Unmarshal([]byte(want), &items); err != nil {
		t.Fatalf("bad want JSON: %v", err)
	}
	out, err := json.Marshal(append(append(append([]any{}, head...), items...), tail...))
	if err != nil {
		t.Fatal(err)
	}
	return req, string(out)
}

func TestEncodeTopLevel(t *testing.T) {
	base := api.Request{
		Messages: []api.Message{user(api.ContentText{Text: "hi"})},
	}

	t.Run("stateless defaults", func(t *testing.T) {
		body := encodeBody(t, testModel, defaultSettings(), base)
		if body["stream"] != true {
			t.Fatal("stream must be true")
		}
		if body["store"] != false {
			t.Fatal("store must be sent as false, the service default is true")
		}
		sameJSON(t, body["stream_options"], `{"include_obfuscation":false}`)
		sameJSON(t, body["include"], `["reasoning.encrypted_content"]`)
		if _, ok := body["max_output_tokens"]; ok {
			t.Fatal("max_output_tokens must be omitted when nothing set one")
		}
		if _, ok := body["reasoning"]; ok {
			t.Fatal("no reasoning block until a knob asks for one")
		}
	})

	t.Run("system prompts join into instructions", func(t *testing.T) {
		req := base
		req.System = "be terse\n\nanswer in English"
		body := encodeBody(t, testModel, defaultSettings(), req)
		if body["instructions"] != "be terse\n\nanswer in English" {
			t.Fatalf("instructions %q", body["instructions"])
		}
	})

	t.Run("tools are flat and the schema goes straight into parameters", func(t *testing.T) {
		req := base
		req.Tools = []api.Tool{
			{Name: "get_weather", Desc: "look up the weather", Schema: map[string]any{"type": "object"}},
			{Name: "now"},
		}
		body := encodeBody(t, testModel, defaultSettings(), req)
		sameJSON(t, body["tools"], `[
			{"type":"function","name":"get_weather","description":"look up the weather","parameters":{"type":"object"}},
			{"type":"function","name":"now","parameters":{"type":"object","properties":{},"additionalProperties":false}}]`)
	})

	t.Run("tool choice", func(t *testing.T) {
		cases := map[api.ToolChoice]any{
			api.ToolChoiceAuto: "auto",
			api.ToolChoiceNone: "none",
			api.ToolChoiceAny:  "required",
		}
		for choice, want := range cases {
			req := base
			req.ToolChoice = choice
			if got := encodeBody(t, testModel, defaultSettings(), req)["tool_choice"]; got != want {
				t.Fatalf("tool_choice for %q is %v, want %v", choice, got, want)
			}
		}
		req := base
		req.ToolChoice = "get_weather"
		sameJSON(t, encodeBody(t, testModel, defaultSettings(), req)["tool_choice"], `{"type":"function","name":"get_weather"}`)

		req.ToolChoice = ""
		if _, ok := encodeBody(t, testModel, defaultSettings(), req)["tool_choice"]; ok {
			t.Fatal("an unset choice must be omitted")
		}
	})

	t.Run("max_output_tokens is the cap the provider was configured with", func(t *testing.T) {
		set := defaultSettings()
		set.maxTokens = 4096
		if got := encodeBody(t, testModel, set, base)["max_output_tokens"]; got != float64(4096) {
			t.Fatalf("max_output_tokens %v", got)
		}
	})

	t.Run("an undeclared model gets sampling and no include", func(t *testing.T) {
		set := defaultSettings()
		temp := 0.2
		set.temperature = &temp
		body := encodeBody(t, "gpt-4o", set, base)
		if body["temperature"] != 0.2 {
			t.Fatalf("temperature %v, want 0.2", body["temperature"])
		}
		if _, ok := body["reasoning"]; ok {
			t.Fatal("a reasoning block was sent without a reasoning option")
		}
		if _, ok := body["include"]; ok {
			t.Fatal("encrypted reasoning was requested from an undeclared model")
		}
	})

	t.Run("effort none keeps sampling on a reasoning model", func(t *testing.T) {
		set := defaultSettings()
		temp, topP := 0.2, 0.9
		set.temperature, set.topP = &temp, &topP
		set.effort = string(EffortNone)
		body := encodeBody(t, testModel, set, base)
		if body["temperature"] != 0.2 || body["top_p"] != 0.9 {
			t.Fatalf("temperature %v, top_p %v, want 0.2 and 0.9", body["temperature"], body["top_p"])
		}
		sameJSON(t, body["reasoning"], `{"effort":"none"}`)
	})

	t.Run("an unknown model gets every option and no include", func(t *testing.T) {
		set := defaultSettings()
		temp := 0.2
		set.temperature = &temp
		set.effort = string(EffortHigh)
		body := encodeBody(t, "my-model", set, base)
		if body["temperature"] != 0.2 {
			t.Fatalf("temperature %v, want 0.2", body["temperature"])
		}
		sameJSON(t, body["reasoning"], `{"effort":"high"}`)
		if _, ok := body["include"]; ok {
			t.Fatal("encrypted reasoning was requested from a model not known to reason")
		}
	})
}

func TestEncodeRejections(t *testing.T) {
	tests := []struct {
		name    string
		model   string // the provider's model, empty where that is what the case rejects
		req     api.Request
		wantSub string
	}{
		{
			// The result answering it says the call succeeded, so the arguments
			// the model sent are unaccounted for, and an empty object in their
			// place would replay as a legitimate call that takes none.
			name:  "discarded arguments no result reports as discarded",
			model: testModel,
			req: api.Request{Messages: []api.Message{
				assistant(api.ContentToolUse{ID: "call_a", Name: "search", Input: api.DiscardedArguments}),
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "18C"}}}),
			}},
			wantSub: "err_json",
		},
		{
			name:  "a tool with no name",
			model: testModel,
			req: api.Request{Tools: []api.Tool{{Desc: "x"}},
				Messages: []api.Message{user(api.ContentText{Text: "hi"})}},
			wantSub: "no name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&Provider{model: api.Model{Name: tc.model}, set: defaultSettings()}).encode(tc.req)
			if err == nil {
				t.Fatal("want an error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSub)
			}
		})
	}
}

func TestDeclaredModelsAreMatchedByExactName(t *testing.T) {
	for _, m := range []api.Model{GPT56Sol, GPT56Luna, GPT6Astra, GPT6Sol, GPT6Luna, GPT61Sol, GPT61Terra} {
		if _, ok := declared[m.Name]; !ok {
			t.Errorf("%q is not declared", m.Name)
		}
	}
	if len(declared) != 7 {
		t.Errorf("%d models declared, want 7", len(declared))
	}
	for _, m := range []string{
		"gpt-5.6", "gpt-5", "gpt-5.1-codex-max", "gpt-5.4-2026-01-01", "o1", "o3-mini", "codex-mini-latest",
		"gpt-4o", "gpt-4.1", "gpt-3.5-turbo", "chatgpt-4o-latest", "gpt-5-chat-latest",
		"gpt-6-sol-2026-11-01", "my-model", "", "openai/gpt-6-sol", "GPT-5.6-LUNA",
	} {
		if _, ok := declared[m]; ok {
			t.Errorf("%q is declared, want unknown", m)
		}
	}
}

func TestProviderOptions(t *testing.T) {
	build := func(t *testing.T, m api.Model, opts ...Option) *Provider {
		t.Helper()
		p, err := New(Conf{Model: m}, opts...)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return p
	}

	t.Run("the key comes from the environment", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "env-key")
		if p := build(t, api.Model{Name: testModel}); p.apiKey != "env-key" {
			t.Fatalf("apiKey %q", p.apiKey)
		}
	})

	t.Run("a missing key still builds, with an empty key", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "")
		if p := build(t, api.Model{Name: testModel}); p.apiKey != "" {
			t.Fatalf("apiKey %q, want empty", p.apiKey)
		}
	})

	t.Run("an explicitly empty key means no header", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "env-key")
		if p := build(t, api.Model{Name: testModel}, WithAPIKey("")); p.apiKey != "" {
			t.Fatalf("apiKey %q, want empty", p.apiKey)
		}
	})

	t.Run("options land on the settings the turn is built from", func(t *testing.T) {
		p := build(t, api.Model{Name: "gpt-4o"}, WithAPIKey("k"), nil, WithTemperature(0.7), WithMaxTokens(2048))
		if p.set.temperature == nil || *p.set.temperature != 0.7 {
			t.Fatalf("temperature %v", p.set.temperature)
		}
		if p.set.maxTokens != 2048 {
			t.Fatalf("maxTokens %d", p.set.maxTokens)
		}
	})

	t.Run("an invalid option fails New", func(t *testing.T) {
		for name, o := range map[string]Option{
			"temperature": WithTemperature(3),
			"nan":         WithTemperature(math.NaN()),
			"top_p nan":   WithTopP(math.NaN()),
			"base url":    WithBaseURL("no-scheme"),
			"replay":      WithReasoningReplay(ReplayMode(9)),
		} {
			if p, err := New(Conf{Model: GPT56Sol}, WithAPIKey("k"), o); err == nil || p != nil {
				t.Errorf("%s: New returned %v, %v, want only an error", name, p, err)
			}
		}
	})

	t.Run("New refuses sampling on a declared reasoning model", func(t *testing.T) {
		for name, tc := range map[string]struct {
			model string
			opts  []Option
		}{
			"temperature on a reasoning model": {testModel, []Option{WithTemperature(0.2)}},
			"top_p on a reasoning model":       {testModel, []Option{WithTopP(0.9), WithEffort(EffortHigh)}},
		} {
			if p, err := New(Conf{Model: api.Model{Name: tc.model}}, append([]Option{WithAPIKey("k")}, tc.opts...)...); err == nil || p != nil {
				t.Errorf("%s: New returned %v, %v, want only an error", name, p, err)
			}
		}
		build(t, api.Model{Name: testModel}, WithAPIKey("k"), WithTemperature(0.2), WithEffort(EffortNone))
	})

	t.Run("New checks no combination on an unknown model", func(t *testing.T) {
		for _, name := range []string{"my-model", "gpt-4o", "o3", "gpt-5.6", "openai/gpt-5.6-sol"} {
			p := build(t, api.Model{Name: name}, WithAPIKey("k"), WithTemperature(0.2), WithTopP(0.9),
				WithEffort(EffortHigh), WithReasoningSummary(SummaryAuto), WithReasoningMode(ReasoningPro), WithReasoningContext(ContextAllTurns))
			if p.set.temperature == nil || p.set.topP == nil || p.set.effort != string(EffortHigh) || p.set.summary != string(SummaryAuto) ||
				p.set.reasoningMode != string(ReasoningPro) || p.set.reasoningContext != string(ContextAllTurns) {
				t.Fatalf("%s: settings %+v, want every option kept", name, p.set)
			}
		}
	})

	t.Run("an empty model name fails New", func(t *testing.T) {
		if p, err := New(Conf{}, WithAPIKey("k")); err == nil || p != nil {
			t.Fatalf("New returned %v, %v, want only an error", p, err)
		}
	})

	t.Run("max tokens: zero is the default, negative and under the minimum fail", func(t *testing.T) {
		if p := build(t, GPT56Sol, WithAPIKey("k"), WithMaxTokens(0)); p.set.maxTokens != 0 {
			t.Fatalf("maxTokens %d, want no cap", p.set.maxTokens)
		}
		for _, n := range []int{-1, 4, maxOutputTokensFloor - 1} {
			if _, err := New(Conf{Model: GPT56Sol}, WithAPIKey("k"), WithMaxTokens(n)); err == nil {
				t.Errorf("WithMaxTokens(%d) built", n)
			}
		}
		build(t, GPT56Sol, WithAPIKey("k"), WithMaxTokens(maxOutputTokensFloor))
	})

	t.Run("a cap over a declared model's output limit fails New", func(t *testing.T) {
		for _, m := range []api.Model{GPT56Sol, GPT56Luna, GPT6Astra, GPT6Sol, GPT6Luna, GPT61Sol, GPT61Terra} {
			if _, err := New(Conf{Model: m}, WithAPIKey("k"), WithMaxTokens(128_001)); err == nil {
				t.Errorf("%s took 128,001 output tokens", m.Name)
			}
			build(t, m, WithAPIKey("k"), WithMaxTokens(128_000))
		}
		for _, name := range []string{"gpt-7", "openai/gpt-6.1-sol", "GPT-6.1-SOL"} {
			if p := build(t, api.Model{Name: name}, WithAPIKey("k"), WithMaxTokens(256_000)); p.set.maxTokens != 256_000 {
				t.Fatalf("%s: maxTokens %d, want an undeclared model left unchecked", name, p.set.maxTokens)
			}
		}
	})

	t.Run("effort values a declared model leaves out fail New", func(t *testing.T) {
		refused := map[api.Model][]Effort{
			GPT56Sol:   {EffortMinimal},
			GPT56Luna:  {EffortMinimal},
			GPT6Astra:  {EffortMinimal, EffortNone},
			GPT6Sol:    {EffortMinimal},
			GPT6Luna:   {EffortMinimal},
			GPT61Sol:   {EffortMinimal, EffortNone},
			GPT61Terra: {EffortMinimal},
		}
		for m, efforts := range refused {
			for _, e := range efforts {
				if _, err := New(Conf{Model: m}, WithAPIKey("k"), WithEffort(e)); err == nil {
					t.Errorf("%s took effort %q", m.Name, e)
				}
			}
			for _, e := range []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax} {
				if _, err := New(Conf{Model: m}, WithAPIKey("k"), WithEffort(e)); err != nil {
					t.Errorf("%s refused effort %q: %v", m.Name, e, err)
				}
			}
		}
		for _, m := range []api.Model{GPT6Sol, GPT61Terra} {
			build(t, m, WithAPIKey("k"), WithEffort(EffortNone))
		}
		build(t, api.Model{Name: "gpt-5"}, WithAPIKey("k"), WithEffort(EffortMinimal))
		build(t, api.Model{Name: "openai/gpt-6.1-sol"}, WithAPIKey("k"), WithEffort(EffortNone))
	})

	t.Run("the model reads back as configured, window included", func(t *testing.T) {
		if got := build(t, GPT56Sol, WithAPIKey("k")).Model(); got != GPT56Sol || got.Context != 1_050_000 {
			t.Fatalf("Model %+v", got)
		}
		own := api.Model{Name: testModel}
		if got := build(t, own, WithAPIKey("k")).Model(); got != own {
			t.Fatalf("Model %+v, want the literal with an unknown window", got)
		}
	})
}

// TestRequestWire checks what actually goes out: the path, the headers and a
// body the service would accept.
func TestRequestWire(t *testing.T) {
	type captured struct {
		path   string
		auth   string
		accept string
		org    string
		body   map[string]any
	}
	got := make(chan captured, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		got <- captured{
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			accept: r.Header.Get("Accept"),
			org:    r.Header.Get("OpenAI-Organization"),
			body:   m,
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		fmt.Fprint(w, frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`))
	}))
	defer srv.Close()

	p := testProvider(t, srv.URL, WithOrganization("org_1"), WithEffort("low"), WithReasoningSummary("auto"))
	collect(t, p, textRequest())

	c := <-got
	if c.path != "/responses" {
		t.Fatalf("path %q, want /responses", c.path)
	}
	if c.auth != "Bearer test-key" {
		t.Fatalf("Authorization %q", c.auth)
	}
	if c.accept != "text/event-stream" {
		t.Fatalf("Accept %q", c.accept)
	}
	if c.org != "org_1" {
		t.Fatalf("OpenAI-Organization %q", c.org)
	}
	if c.body["model"] != "gpt-5.6-sol" {
		t.Fatalf("model %v", c.body["model"])
	}
	sameJSON(t, c.body["reasoning"], `{"effort":"low","summary":"auto"}`)
}

// The declared caps have to be this service's own. Images go at detail "high",
// which the docs fit within 2,500 patches of 32x32 pixels and 2048 pixels a
// side on gpt-5.6, with a 65,535-pixel side on gpt-6-astra; the models with no
// published sizing take the gpt-5.6 limits. A tool-result image is capped at a
// 20,971,520-character data URL, which ImageBytes declares as the raw size that
// encodes to it; a request at 512 MB.
func TestModelLimits(t *testing.T) {
	const pixels = 2_500 * 32 * 32
	for _, m := range []api.Model{GPT56Sol, GPT56Luna, GPT6Astra, GPT6Sol, GPT6Luna, GPT61Sol, GPT61Terra} {
		if m.ImagePixels != pixels {
			t.Errorf("%s declares %d pixels in an image, want the %d of the high-detail patch budget", m.Name, m.ImagePixels, pixels)
		}
		side := 2048
		if m == GPT6Astra {
			side = 65_535
		}
		if m.ImageSide != side {
			t.Errorf("%s declares a %d pixel side, want %d", m.Name, m.ImageSide, side)
		}
		if m.ImagesPerRequest != 1500 {
			t.Errorf("%s declares %d images in a request, want the documented 1500", m.Name, m.ImagesPerRequest)
		}
		if m.RequestBytes != 512_000_000 {
			t.Errorf("%s declares %d bytes a request, want the documented 512 MB", m.Name, m.RequestBytes)
		}
		if m.Context != 1_050_000 {
			t.Errorf("%s declares a %d token context, want the documented 1,050,000", m.Name, m.Context)
		}
		for _, mime := range []string{"image/jpeg", "image/png", "image/gif", "image/webp"} {
			if url := dataURL(mime, make([]byte, m.ImageBytes)); len(url) > 20_971_520 {
				t.Errorf("%s: a %s of the declared %d bytes is a %d-character data URL, over the documented 20,971,520", m.Name, mime, m.ImageBytes, len(url))
			}
			if url := dataURL(mime, make([]byte, m.ImageBytes+3)); len(url) <= 20_971_520 {
				t.Errorf("%s: ImageBytes %d leaves room under the data URL limit for %s", m.Name, m.ImageBytes, mime)
			}
		}
	}
}

func TestOpaqueKindsTheStreamEmitsReplay(t *testing.T) {
	for _, kind := range []string{
		"file_search_call", "web_search_call", "code_interpreter_call",
		"image_generation_call", "mcp_call", "mcp_list_tools", "custom_tool_call", "compaction",
	} {
		item := `{"type":"` + kind + `"}`
		body := encodeBody(t, testModel, defaultSettings(), api.Request{Messages: []api.Message{
			assistant(api.ContentOpaque{Kind: providerName + "." + kind, Data: json.RawMessage(item)}),
		}})
		raw, _ := json.Marshal(body["input"])
		if string(raw) != `[`+item+`]` {
			t.Errorf("opaque kind %q the stream emits did not replay: %s", kind, raw)
		}
	}
}

func TestOpaqueKindsOfOtherProvidersSendNothing(t *testing.T) {
	for _, c := range []api.ContentOpaque{
		{Kind: "compaction", Data: json.RawMessage(`{"id":"cmp_1","type":"compaction"}`)},
		{Kind: "claude.compaction", Data: json.RawMessage(`{"type":"compaction"}`)},
		{Kind: "gpt.compaction", Data: json.RawMessage(`{"type":"web_search_call"}`)},
		{Kind: "gpt.unknown_call", Data: json.RawMessage(`{"type":"unknown_call"}`)},
	} {
		body := encodeBody(t, testModel, defaultSettings(), api.Request{Messages: []api.Message{assistant(c, api.ContentText{Text: "ok"})}})
		sameJSON(t, body["input"], `[{"role":"assistant","content":[{"type":"output_text","text":"ok"}]}]`)
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
	counts := levelCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counts))
	t.Cleanup(func() { slog.SetDefault(prev) })
	req := api.Request{Messages: []api.Message{
		user(api.ContentImage{Data: []byte("BM"), Mime: "image/bmp"}, nil),
		assistant(api.ContentReasoning{Model: "claude-opus-5", Meta: json.RawMessage(`"sig"`)}, api.ContentText{Text: "ok"}),
	}}
	for range 2 {
		body := encodeBody(t, testModel, defaultSettings(), req)
		sameJSON(t, body["input"], `[{"role":"user","content":[
			{"type":"input_text","text":"[image omitted]"},
			{"type":"input_text","text":"[content omitted]"}]},
			{"role":"assistant","content":[
			{"type":"output_text","text":"ok"}]}]`)
	}
	if counts[slog.LevelWarn] != 0 || counts[slog.LevelError] > 1 {
		t.Fatalf("two encodes logged %v, want no warnings and at most one error", map[slog.Level]int(counts))
	}
}

func TestEveryDocumentMimeEncodes(t *testing.T) {
	for _, mime := range []api.DocumentMime{
		api.DocumentPDF, api.DocumentText, api.DocumentJSON, api.DocumentMarkdown,
		api.DocumentCSV, api.DocumentHTML, api.DocumentXML, api.DocumentYAML,
		api.DocumentRTF, api.DocumentDOC, api.DocumentXLS, api.DocumentDOCX,
		api.DocumentXLSX, api.DocumentPPTX,
	} {
		body := encodeBody(t, testModel, defaultSettings(), api.Request{Messages: []api.Message{
			user(api.ContentDocument{Data: []byte("x"), Mime: string(mime)}),
		}})
		if raw, _ := json.Marshal(body["input"]); !strings.Contains(string(raw), `"type":"input_file"`) {
			t.Fatalf("%s: input %s carries no file", mime, raw)
		}
	}
}

func TestDataURLMatchesConcatenation(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 5, 57, 1000, 1 << 16, 1<<20 + 1} {
		data := make([]byte, n)
		for i := range data {
			data[i] = byte(i*7 + i>>8)
		}
		for _, mime := range []string{"image/png", "application/pdf", ""} {
			want := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
			if got := dataURL(mime, data); got != want {
				t.Fatalf("%d bytes of %q: the data URL differs from the concatenated one", n, mime)
			}
		}
	}
}

func BenchmarkDataURL(b *testing.B) {
	data := make([]byte, 1<<20)
	b.ReportAllocs()
	for b.Loop() {
		dataURL("image/png", data)
	}
}

func TestToolSchemaDropsFormatsStrictModeRejects(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"data":{"type":"string","format":"base64"},"at":{"type":"string","format":"date-time"},"format":{"type":"string"}}}`)
	body := encodeBody(t, testModel, defaultSettings(), api.Request{
		Tools:    []api.Tool{{Name: "put", Desc: "x", Schema: schema}},
		Messages: []api.Message{user(api.ContentText{Text: "hi"})},
	})
	params, _ := json.Marshal(body["tools"].([]any)[0].(map[string]any)["parameters"])
	want := `{"properties":{"at":{"format":"date-time","type":"string"},"data":{"type":"string"},"format":{"type":"string"}},"type":"object"}`
	if string(params) != want {
		t.Fatalf("parameters went out as %s, want %s", params, want)
	}
}

func TestDeclaredModelChecksRunInNew(t *testing.T) {
	for name, opts := range map[string][]Option{
		"max tokens over the ceiling": {WithMaxTokens(maxOutputTokensCeiling + 1)},
		"effort the model refuses":    {WithEffort(EffortMinimal)},
		"sampling before the effort":  {WithTemperature(0.2), WithEffort(EffortHigh)},
	} {
		if p, err := New(Conf{Model: GPT56Sol}, append(opts, WithAPIKey("k"))...); err == nil || p != nil || !strings.HasPrefix(err.Error(), "gpt: ") {
			t.Errorf("%s: New returned %v, %v, want only an error", name, p, err)
		}
		if _, err := New(Conf{Model: api.Model{Name: "openai/gpt-5.6-sol"}}, append(opts, WithAPIKey("k"))...); err != nil {
			t.Errorf("%s: an undeclared model was checked: %v", name, err)
		}
	}
	if p, err := New(Conf{Model: GPT56Sol}, WithEffort(EffortMinimal), WithEffort(EffortHigh), WithMaxTokens(maxOutputTokensCeiling)); err != nil || p.set.effort != "high" {
		t.Errorf("the last effort is the one checked: %v", err)
	}
}

func TestStoredContentTheServiceRefusesIsANote(t *testing.T) {
	body := encodeBody(t, testModel, defaultSettings(), api.Request{Messages: []api.Message{
		user(api.ContentText{Text: "hi"}),
		assistant(api.ContentToolUse{ID: "call_1", Input: json.RawMessage(`{}`)}),
		user(api.ContentToolResult{ID: "call_1", Content: api.UserContents{api.ContentText{Text: "72F"}}}, api.ContentToolResult{Content: api.UserContents{api.ContentText{Text: "lost"}}}),
	}})
	sameJSON(t, body["input"], `[
		{"role":"user","content":[{"type":"input_text","text":"hi"}]},
		{"type":"function_call","call_id":"call_1","name":"`+lib.CallName("")+`","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_text","text":"72F"}]},
		{"role":"user","content":[{"type":"input_text","text":"[content omitted]"}]}
	]`)
}

func TestEncodeRefusesARequestWithNoMessages(t *testing.T) {
	if _, err := (&Provider{model: api.Model{Name: testModel}, set: defaultSettings()}).encode(api.Request{}); err == nil || !strings.Contains(err.Error(), "gpt: request has no messages") {
		t.Fatalf("error %v, want a refusal naming the missing messages", err)
	}
}

func TestCallsAndResultsPairByIDInInputOrder(t *testing.T) {
	for _, c := range []struct {
		name string
		msgs []api.Message
		want string
	}{
		{"a result with no call before it is a note", []api.Message{
			user(api.ContentToolResult{ID: "call_9", Content: api.UserContents{api.ContentText{Text: "x"}}}),
		}, `[{"role":"user","content":[{"type":"input_text","text":"[content omitted]"}]}]`},
		{"a call no later result answers is a note", []api.Message{
			user(api.ContentText{Text: "go"}),
			assistant(api.ContentToolUse{ID: "call_1", Name: "t"}),
		}, `[{"role":"user","content":[{"type":"input_text","text":"go"}]},{"role":"assistant","content":[{"type":"output_text","text":"[content omitted]"}]}]`},
		{"a result before its call in one message is a note, and the call stays unanswered", []api.Message{
			user(api.ContentToolResult{ID: "call_1", Content: api.UserContents{api.ContentText{Text: "x"}}}, api.ContentToolUse{ID: "call_1", Name: "t"}),
		}, `[{"role":"user","content":[{"type":"input_text","text":"[content omitted]"},{"type":"input_text","text":"[content omitted]"}]}]`},
		{"a call in a user message answered two messages later is carried", []api.Message{
			user(api.ContentText{Text: "go"}, api.ContentToolUse{ID: "call.1:x", Name: "functions.t"}),
			assistant(api.ContentText{Text: "waiting"}),
			user(api.ContentToolResult{ID: "call.1:x", Content: api.UserContents{api.ContentText{Text: "x"}}}),
		}, `[{"role":"user","content":[{"type":"input_text","text":"go"}]},{"type":"function_call","call_id":"` + lib.CallID("call.1:x") + `","name":"functions_t","arguments":"{}"},` +
			`{"role":"assistant","content":[{"type":"output_text","text":"waiting"}]},{"type":"function_call_output","call_id":"` + lib.CallID("call.1:x") + `","output":[{"type":"input_text","text":"x"}]}]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			sameJSON(t, encodeBody(t, testModel, defaultSettings(), api.Request{Messages: c.msgs})["input"], c.want)
		})
	}
}
