package gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"neui.dev/api"
)

func encodeBody(t *testing.T, model string, set settings, req api.Request) map[string]any {
	t.Helper()
	b, err := encodeRequest(model, set, req)
	if err != nil {
		t.Fatalf("encodeRequest: %v", err)
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

// phaseMeta is what this adapter stores on a text block whose message announced
// a phase, written the way the stream writes it.
func phaseMeta(model, phase string) json.RawMessage {
	raw, err := json.Marshal(messageMeta{Model: model, Phase: phase})
	if err != nil {
		panic(err)
	}
	return raw
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
				{"type":"input_image","image_url":"data:image/png;base64,iVA=","detail":"auto"}]}]`,
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
			name: "assistant text goes back as a typed block array, the same as the user role",
			req: api.Request{Messages: []api.Message{
				user(api.ContentText{Text: "hi"}),
				assistant(api.ContentText{Text: "hello"}),
				user(api.ContentText{Text: "again"}),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"hi"}]},
				{"role":"assistant","content":[{"type":"input_text","text":"hello"}]},
				{"role":"user","content":[{"type":"input_text","text":"again"}]}]`,
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
					{"type":"input_text","text":"Let me check."},
					{"type":"input_text","text":"Two things."}]},
				{"type":"function_call","call_id":"call_a","name":"search","arguments":"{}"},
				{"role":"assistant","content":[
					{"type":"input_text","text":"Found it."}]}]`,
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
				{"role":"assistant","content":[{"type":"input_text","text":"Looking."}]},
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
				{"type":"input_image","image_url":"data:image/jpeg;base64,AQ==","detail":"auto"}]}]`,
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
						Meta:    json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"SEALED"}`),
					},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "reasoning with no encoding of its own is left out, and the call it produced still goes",
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
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"[reasoning omitted]"}]},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			name: "a reasoning item that cannot travel takes nothing beside it with it",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "unsealed"}},
					api.ContentToolUse{ID: "call_b", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"role":"assistant","content":[{"type":"input_text","text":"[reasoning omitted]"}]},
				{"type":"function_call","call_id":"call_b","name":"t","arguments":"{}"}]`,
		},
		{
			// The last of them has nothing behind it to be its required
			// following item, which is the one direction of the pairing that
			// names an id this adapter sends. The two in front of it are each
			// followed by an item and go.
			name:     "a reasoning item with nothing following it is not sent",
			settings: func(s *settings) { s.replay = ReplayAlways },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"ONE"}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_2","type":"reasoning","encrypted_content":"TWO"}`)},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_3","type":"reasoning","encrypted_content":"THREE"}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"ONE"},
				{"id":"rs_2","type":"reasoning","encrypted_content":"TWO"}]`,
		},
		{
			name: "reasoning that only preceded text is left out by default",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
		},
		{
			name:     "ReplayAlways keeps it",
			settings: func(s *settings) { s.replay = ReplayAlways },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"},
				{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
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
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
				),
				user(api.ContentToolResult{ID: "call_a", Content: api.UserContents{api.ContentText{Text: "18C"}}}),
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"ALSO"}`)},
					api.ContentText{Text: "It is 18C."},
				),
			}},
			want: `[{"role":"user","content":[{"type":"input_text","text":"weather?"}]},
				{"type":"function_call","call_id":"call_a","name":"get_weather","arguments":"{\"city\":\"Paris\"}"},
				{"type":"function_call_output","call_id":"call_a","output":[{"type":"input_text","text":"18C"}]},
				{"role":"assistant","content":[{"type":"input_text","text":"It is 18C."}]}]`,
		},
		{
			name:     "reasoning at the end of a turn has no following item to be paired with",
			settings: func(s *settings) { s.replay = ReplayAlways },
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "so far"},
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`)},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"so far"}]}]`,
		},
		{
			name: "reasoning carrying neither a summary nor an encoding of its own leaves its note, and its call goes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.6-sol"},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"[reasoning omitted]"}]},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"}]`,
		},
		{
			// A block with nothing to send becomes no item at all, so the note
			// standing for it is a part like any other and the text either side
			// of it stays one message.
			name: "a reasoning block with nothing to send leaves a part rather than splitting the message",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "so far"},
					api.ContentReasoning{Model: "gpt-5.6-sol", Summary: map[int]string{0: "thinking"}},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[
				{"type":"input_text","text":"so far"},
				{"type":"input_text","text":"[reasoning omitted]"},
				{"type":"input_text","text":"the answer"}]}]`,
		},
		{
			// A reasoning item names no model anywhere in it and its payload is
			// sealed against the organization rather than against a model, so
			// there is nothing here for a model comparison to be right about.
			name: "a reasoning item replays whatever model stamped it, and whether it was stamped at all",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentReasoning{Model: "gpt-5.1", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`)},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentReasoning{Meta: json.RawMessage(`{"id":"rs_2","type":"reasoning","encrypted_content":"ALSO"}`)},
					api.ContentToolUse{ID: "call_b", Name: "t", Input: json.RawMessage(`{}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"id":"rs_2","type":"reasoning","encrypted_content":"ALSO"},
				{"type":"function_call","call_id":"call_b","name":"t","arguments":"{}"}]`,
		},
		{
			// The phase is this adapter's own invention rather than something the
			// service issued, and it labels the message one model wrote, so it is
			// the one replayed thing a model stamp still decides.
			name: "a phase another model produced is not replayed",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "the answer", Meta: phaseMeta("gpt-5.1", "commentary")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
		},
		{
			name:  "a gateway spelling of the same model still labels the message",
			model: "openai/GPT-5.6-sol",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "the answer", Meta: phaseMeta("gpt-5.6-sol", "commentary")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"the answer"}],"phase":"commentary"}]`,
		},
		{
			// The label is on the block, so it travels with it wherever the block
			// sits, and a message ends where its phase stops applying.
			name: "phase labels the message the block it is on becomes",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me look", Meta: phaseMeta("gpt-5.6-sol", "commentary")},
					api.ContentToolUse{ID: "call_a", Name: "t", Input: json.RawMessage(`{}`)},
					api.ContentText{Text: "the answer", Meta: phaseMeta("gpt-5.6-sol", "final_answer")},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"let me look"}],"phase":"commentary"},
				{"type":"function_call","call_id":"call_a","name":"t","arguments":"{}"},
				{"role":"assistant","content":[{"type":"input_text","text":"the answer"}],"phase":"final_answer"}]`,
		},
		{
			// A run is one message only as far as the blocks agree, because a
			// phase is a claim about the message and two of them cannot be one.
			name: "a change of phase inside a run ends the message",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me look", Meta: phaseMeta("gpt-5.6-sol", "commentary")},
					api.ContentText{Text: "still looking", Meta: phaseMeta("gpt-5.6-sol", "commentary")},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[
					{"type":"input_text","text":"let me look"},
					{"type":"input_text","text":"still looking"}],"phase":"commentary"},
				{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
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
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
		},
		{
			// An opaque payload is an output item the input array takes back, so
			// it goes out as the bytes that arrived, as a top level item that
			// ends the run of text around it.
			name: "an opaque payload goes back as the item it arrived as",
			req: api.Request{Messages: []api.Message{
				assistant(
					api.ContentText{Text: "let me search"},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
					api.ContentText{Text: "the answer"},
				),
			}},
			want: `[{"role":"assistant","content":[{"type":"input_text","text":"let me search"}]},
				{"id":"ws_1","type":"web_search_call","status":"completed"},
				{"role":"assistant","content":[{"type":"input_text","text":"the answer"}]}]`,
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
						Kind:   "code_interpreter_call",
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
					api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[]}`)},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
				),
			}},
			want: `[{"id":"rs_1","type":"reasoning","summary":[]},
				{"id":"ws_1","type":"web_search_call","status":"completed"}]`,
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
				{"type":"input_text","text":"[image omitted]"},
				{"type":"input_text","text":"the answer"}]}]`,
		},
		{
			name: "reasoning and a payload in a user turn belong to the assistant turn that produced them",
			req: api.Request{Messages: []api.Message{
				user(
					api.ContentReasoning{Model: "gpt-5.6-sol", Thoughts: map[int]string{0: "theirs"}},
					api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`)},
					api.ContentText{Text: "carry on"},
				),
			}},
			want: `[{"role":"user","content":[
				{"type":"input_text","text":"[reasoning omitted]"},
				{"type":"input_text","text":"[content omitted]"},
				{"type":"input_text","text":"carry on"}]}]`,
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
			body := encodeBody(t, model, set, tc.req)
			sameJSON(t, body["input"], tc.want)
		})
	}
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

	t.Run("sampling is dropped on a reasoning model and kept otherwise", func(t *testing.T) {
		set := defaultSettings()
		temp := 0.2
		set.temperature = &temp
		set.effort = "high"

		reasoning := encodeBody(t, testModel, set, base)
		if _, ok := reasoning["temperature"]; ok {
			t.Fatal("temperature reached a reasoning model")
		}
		sameJSON(t, reasoning["reasoning"], `{"effort":"high"}`)

		body := encodeBody(t, "gpt-4o", set, base)
		if body["temperature"] != 0.2 {
			t.Fatalf("temperature %v, want 0.2", body["temperature"])
		}
		if _, ok := body["reasoning"]; ok {
			t.Fatal("a reasoning block reached a non reasoning model")
		}
		if _, ok := body["include"]; ok {
			t.Fatal("encrypted reasoning was requested from a model that has none")
		}
	})

	t.Run("forced sampling overrides the model gate", func(t *testing.T) {
		set := defaultSettings()
		temp := 0.2
		set.temperature = &temp
		set.forceSampling = true
		if got := encodeBody(t, testModel, set, base)["temperature"]; got != 0.2 {
			t.Fatalf("temperature %v, want 0.2", got)
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
			name:    "no model",
			req:     api.Request{Messages: []api.Message{user(api.ContentText{Text: "hi"})}},
			wantSub: "no model",
		},
		{
			name:  "an image type the vision models do not read",
			model: testModel,
			req: api.Request{Messages: []api.Message{
				user(api.ContentImage{Data: []byte("x"), Mime: "image/svg+xml"}),
			}},
			wantSub: "image/svg+xml",
		},
		{
			name:  "a document with no media type",
			model: testModel,
			req: api.Request{Messages: []api.Message{
				user(api.ContentDocument{Data: []byte("x")}),
			}},
			wantSub: "media type",
		},
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
			_, err := encodeRequest(tc.model, defaultSettings(), tc.req)
			if err == nil {
				t.Fatal("want an error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSub)
			}
		})
	}
}

func TestModelClassification(t *testing.T) {
	reasoning := []string{
		"gpt-5.6-sol", "gpt-5.6", "gpt-5", "gpt-5-mini", "gpt-5.1-codex-max",
		"gpt-5.4-2026-01-01", "o1", "o1-pro", "o3-mini", "o4-mini",
		"codex-mini-latest", "openai/gpt-5.6-terra", "GPT-5.6-LUNA",
		"gpt-6-astra", "gpt-6-sol", "gpt-6-luna", "openai/gpt-6-sol",
	}
	for _, m := range reasoning {
		if !isReasoningModel(m) {
			t.Errorf("%q classified as non reasoning", m)
		}
	}
	plain := []string{
		"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4-turbo", "gpt-3.5-turbo",
		"chatgpt-4o-latest", "gpt-5-chat-latest", "gpt-5.3-chat-latest", "gpt-60", "",
	}
	for _, m := range plain {
		if isReasoningModel(m) {
			t.Errorf("%q classified as reasoning", m)
		}
	}
}

func TestProviderOptions(t *testing.T) {
	t.Run("the key comes from the environment", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "env-key")
		p := New(Conf{Model: api.Model{Name: testModel}})
		if p.apiKey != "env-key" {
			t.Fatalf("apiKey %q", p.apiKey)
		}
	})

	t.Run("a missing key still builds, with an empty key", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "")
		p := New(Conf{Model: api.Model{Name: testModel}})
		if p.apiKey != "" {
			t.Fatalf("apiKey %q, want empty", p.apiKey)
		}
	})

	t.Run("an explicitly empty key means no header", func(t *testing.T) {
		t.Setenv(apiKeyEnv, "env-key")
		p := New(Conf{Model: api.Model{Name: testModel}}, WithAPIKey(""))
		if p.apiKey != "" {
			t.Fatalf("apiKey %q, want empty", p.apiKey)
		}
	})

	t.Run("options land on the settings the turn is built from", func(t *testing.T) {
		p := New(Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithTemperature(0.7), WithMaxTokens(2048))
		if p.set.temperature == nil || *p.set.temperature != 0.7 {
			t.Fatalf("temperature %v", p.set.temperature)
		}
		if p.set.maxTokens != 2048 {
			t.Fatalf("maxTokens %d", p.set.maxTokens)
		}
	})

	// A cap the service answers 400 to is the configuration being wrong, and
	// whoever wrote it is there to be told, where raising it silently sends a
	// turn nobody asked for.
	t.Run("a cap under the schema minimum is refused rather than raised", func(t *testing.T) {
		p := New(Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithMaxTokens(4))
		if p.set.maxTokens != 0 {
			t.Fatalf("maxTokens %d, want the refused option to have changed nothing", p.set.maxTokens)
		}
		if err := WithMaxTokens(-1)(p); err == nil {
			t.Fatal("a negative cap must be refused")
		}
		if err := WithMaxTokens(maxOutputTokensFloor)(p); err != nil {
			t.Fatalf("the schema minimum must be accepted: %v", err)
		}
	})

	t.Run("the model reads back as configured, window included", func(t *testing.T) {
		p := New(Conf{Model: GPT56Sol}, WithAPIKey("k"))
		if got := p.Model(); got != GPT56Sol || got.Context != 1_050_000 {
			t.Fatalf("Model %+v", got)
		}
		own := api.Model{Name: testModel}
		if got := New(Conf{Model: own}, WithAPIKey("k")).Model(); got != own {
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

	p := testProvider(t, srv.URL, WithOrganization("org_1"), WithReasoningEffort("low"), WithReasoningSummary("auto"))
	drain(p.Stream(context.Background(), textRequest()))

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

// The declared image caps have to be this service's own. ImagePixels is the area
// a caller downscales an image to before the request is built, so a threshold
// borrowed from a sibling API throws away resolution this one would have read,
// and nothing downstream can see that happening. What the docs publish is 30,000
// patches of 32x32 pixels per image, exceeded by rejection rather than by
// resizing, and no per-image byte cap at all: zero says unknown, where a number
// invented for it would shed attachments the service accepts.
func TestModelImageCaps(t *testing.T) {
	const patches = 30_000 * 32 * 32
	for _, m := range []api.Model{GPT56Sol, GPT56Luna, GPT6Astra, GPT6Sol, GPT6Luna} {
		if m.ImagePixels != patches {
			t.Errorf("%s declares %d pixels in an image, want the %d of the documented patch budget", m.Name, m.ImagePixels, patches)
		}
		if m.ImageBytes != 0 {
			t.Errorf("%s declares a %d byte image cap that this service does not publish", m.Name, m.ImageBytes)
		}
		if m.ImagesPerRequest != 1500 {
			t.Errorf("%s declares %d images in a request, want the documented 1500", m.Name, m.ImagesPerRequest)
		}
	}
}
