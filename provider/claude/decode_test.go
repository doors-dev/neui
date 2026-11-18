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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/provider/lib"
)

func testProvider(t *testing.T, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return mustNew(t, Conf{Model: api.Model{Name: testModel}}, append([]Option{WithAPIKey("test-key"), WithBaseURL(srv.URL)}, opts...)...)
}

func sseServer(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, body)
	}
}

// testModel is what every test provider is configured with, so a fixture that
// replays stored reasoning stamps its blocks with this to have them go back.
const testModel = "claude-opus-5"

func hello() api.Request {
	return api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "hi"}}}},
	}
}

func collect(t *testing.T, p *Provider, req api.Request) []api.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var got []api.Event
	for e := range p.Stream(ctx, req) {
		got = append(got, e)
	}
	return got
}

// raw renders a payload that may be absent, because an empty one is the common
// case and a trailing space is a bad thing to have to see in a fixture.
func raw(r json.RawMessage) string {
	if len(r) == 0 {
		return "-"
	}
	return string(r)
}

func render(e api.Event) string {
	switch e := e.(type) {
	case api.EventStart:
		return fmt.Sprintf("start %s %s", e.ID, e.Model)
	case api.EventText:
		return fmt.Sprintf("text %d %q", e.Index, e.Text)
	case api.EventTool:
		return fmt.Sprintf("tool %d %q %q %q", e.Index, e.ID, e.Name, e.Args)
	case api.EventThinking:
		return fmt.Sprintf("thinking %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventSummary:
		return fmt.Sprintf("summary %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventMeta:
		return fmt.Sprintf("meta %d %s", e.Index, raw(e.Data))
	case api.EventOpaque:
		return fmt.Sprintf("opaque %d %q %s %q", e.Index, e.Kind, raw(e.Data), e.Delta)
	case api.EventClose:
		return fmt.Sprintf("close %d", e.Index)
	case api.EventUsage:
		return fmt.Sprintf("usage in=%d out=%d rsn=%d cr=%d cw=%d", e.Input, e.Output, e.Reasoning, e.CacheRead, e.CacheWrite)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		return fmt.Sprintf("err %s", e.Code)
	}
	return fmt.Sprintf("unknown %T", e)
}

func renderAll(events []api.Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, render(e))
	}
	return out
}

func equalSeq(t *testing.T, got []api.Event, want []string) {
	t.Helper()
	have := renderAll(got)
	if len(have) != len(want) {
		t.Fatalf("event count %d, want %d\ngot:  %s\nwant: %s",
			len(have), len(want), strings.Join(have, "\n      "), strings.Join(want, "\n      "))
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("event %d = %s, want %s\ngot:  %s\nwant: %s",
				i, have[i], want[i], strings.Join(have, "\n      "), strings.Join(want, "\n      "))
		}
	}
}

// sse assembles a fixture in the wire format: a named event line, one data
// line, and a blank line.
func sse(pairs ...string) string {
	var b strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		b.WriteString("event: " + pairs[i] + "\n")
		b.WriteString("data: " + pairs[i+1] + "\n\n")
	}
	return b.String()
}

const (
	fixStart     = `{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-opus-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":25,"output_tokens":1}}}`
	fixPing      = `{"type": "ping"}`
	fixStopFrame = `{"type":"message_stop"}`
)

func TestStreamSequences(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "plain text turn",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"ping", fixPing,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "Hello"`,
				`text 0 "!"`,
				"close 0",
				"usage in=25 out=15 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "one tool call",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Checking."}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01","name":"get_weather","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" \"SF\"}"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1 }`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":89}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "Checking."`,
				"close 0",
				// Identity first here because that is the order this wire
				// produces it in, and each piece goes out as it arrives.
				`tool 1 "toolu_01" "get_weather" ""`,
				`tool 1 "" "" "{\"location\":"`,
				`tool 1 "" "" " \"SF\"}"`,
				"close 1",
				"usage in=25 out=89 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "several tool calls",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_a","name":"read","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"p\":\"a\"}"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_b","name":"read","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"p\":\"b\"}"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_c","name":"now","input":{}}}`,
				"content_block_stop", `{"type":"content_block_stop","index":2}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":40}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`tool 0 "toolu_a" "read" ""`,
				`tool 0 "" "" "{\"p\":\"a\"}"`,
				"close 0",
				`tool 1 "toolu_b" "read" ""`,
				`tool 1 "" "" "{\"p\":\"b\"}"`,
				"close 1",
				`tool 2 "toolu_c" "now" ""`,
				// The block closed having carried no fragment at all, which is
				// the provider saying the call takes no arguments.
				`tool 2 "" "" "{}"`,
				"close 2",
				"usage in=25 out=40 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "reasoning turn",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"1071 = 2 x 462 + 147"}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"\nGCD is 21."}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EqQBCgIYAhIM"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"21."}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":348,"output_tokens_details":{"thinking_tokens":312}}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				// Thinking rather than a summary, because this provider was
				// configured with no thinking display. There is one part on this
				// wire and it is part zero.
				`thinking 0 part=0 "1071 = 2 x 462 + 147"`,
				`thinking 0 part=0 "\nGCD is 21."`,
				// The signature arrives alone and is stored alone: it is what
				// the block is replayed by, and the envelope around it is built
				// on the way out, so no readable half is held back here.
				`meta 0 "EqQBCgIYAhIM"`,
				"close 0",
				`text 1 "21."`,
				"close 1",
				"usage in=25 out=36 rsn=312 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "reasoning with the thinking omitted",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EosnCkYICxIMMb3"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"done"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				// A whole reasoning block carrying only the token that proves
				// the thinking happened is the signature on its own, behind the
				// empty event that says the block is reasoning at all.
				`thinking 0 part=0 ""`,
				`meta 0 "EosnCkYICxIMMb3"`,
				"close 0",
				`text 1 "done"`,
				"close 1",
				"usage in=25 out=9 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "sealed thinking is reasoning with nothing readable",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"EvwBCkYIA"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"ok"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`thinking 0 part=0 ""`,
				`meta 0 {"type":"redacted_thinking","data":"EvwBCkYIA"}`,
				"close 0",
				`text 1 "ok"`,
				"close 1",
				"usage in=25 out=4 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "tool arguments that never parse",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_cut","name":"write","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"body\": \"the quick"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":1024}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`tool 0 "toolu_cut" "write" ""`,
				// The fragment goes out as it arrived; nothing here fabricates
				// closing braces or an empty object over the top of it, and
				// discarding it belongs to whoever folds this into Content.
				`tool 0 "" "" "{\"body\": \"the quick"`,
				"close 0",
				"usage in=25 out=1024 rsn=0 cr=0 cw=0",
				// Cut at the cap, so the turn is not kept and the half written
				// call never has to be made sense of. Raising max_tokens or
				// continuing is what the code says to do about it.
				"err capped",
			},
		},
		{
			name: "whitespace arguments are not an empty object",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_ws","name":"now","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"  "}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":12}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`tool 0 "toolu_ws" "now" ""`,
				// A fragment was emitted, so the call carried something. It does
				// not parse, which makes it nothing, and nothing is not an empty
				// object: emptiness is whether a fragment went out, not what
				// trimming an accumulation would say.
				`tool 0 "" "" "  "`,
				"close 0",
				"usage in=25 out=12 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "disjoint usage counters",
			body: sse(
				"message_start", `{"type":"message_start","message":{"id":"msg_02","type":"message","role":"assistant","content":[],"model":"claude-opus-5","usage":{"input_tokens":2679,"cache_creation_input_tokens":1200,"cache_read_input_tokens":8000,"output_tokens":3}}}`,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				// Reports only output_tokens: the prompt side keeps the values
				// it already had rather than dropping to zero.
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":null},"usage":{"output_tokens":500}}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":510,"output_tokens_details":{"thinking_tokens":410}}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_02 claude-opus-5",
				"usage in=2679 out=3 rsn=0 cr=8000 cw=1200",
				`text 0 "x"`,
				"close 0",
				"usage in=2679 out=500 rsn=0 cr=8000 cw=1200",
				"usage in=2679 out=100 rsn=410 cr=8000 cw=1200",
				"ok turn",
			},
		},
		{
			name: "keepalives and unknown frames produce nothing",
			body: sse(
				"ping", fixPing,
				"message_start", fixStart,
				"ping", fixPing,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"ping", fixPing,
				"something_new", `{"type":"something_new","payload":{"a":1}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a"}}`,
				"ping", fixPing,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"citations_delta","citation":{"type":"char_location","cited_text":"q","document_index":0}}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"ping", fixPing,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
				"ping", fixPing,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "a"`,
				"close 0",
				"usage in=25 out=2 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "blocks the protocol drops leave no hole",
			body: sse(
				"message_start", fixStart,
				// Opens and closes carrying only a citation, which annotates a
				// text block rather than filling one. It produces nothing, so
				// it takes no index with it.
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"citations_delta","citation":{"type":"char_location"}}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"searching"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}}`,
				"content_block_delta", `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"x\"}"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":2}`,
				"content_block_start", `{"type":"content_block_start","index":3,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","encrypted_content":"Ev0DCio"}]}}`,
				"content_block_stop", `{"type":"content_block_stop","index":3}`,
				"content_block_start", `{"type":"content_block_start","index":4,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":4,"delta":{"type":"text_delta","text":"done"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":4}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":11}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "searching"`,
				"close 0",
				// The payload goes out as it opened, placeholder input and all,
				// and the arguments that stream into it follow as fragments.
				// Splicing them in here would mean decoding the one thing that
				// has to go back byte for byte.
				`opaque 1 "server_tool_use" {"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}} ""`,
				`opaque 1 "" - "{\"query\":\"x\"}"`,
				"close 1",
				`opaque 2 "web_search_tool_result" {"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","encrypted_content":"Ev0DCio"}]} ""`,
				"close 2",
				`text 3 "done"`,
				"close 3",
				"usage in=25 out=11 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			// A content_block_start with no content_block is a broken frame,
			// and only a reply that arrived whole is kept.
			name: "a block that names no type fails the turn",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"?"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"done"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":1}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				"err network",
			},
		},
		{
			name: "refusal mid stream leaves its block open",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"refusal","stop_sequence":null},"usage":{"output_tokens":7}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "partial"`,
				"usage in=25 out=7 rsn=0 cr=0 cw=0",
				// No content_block_stop ever arrived, so the block was cut short
				// and never closes. The consumer is holding it and can see that.
				// The text still streamed and the turn is still discarded: what
				// the terminal event decides is what enters the conversation, not
				// what arrived.
				"err refused",
			},
		},
		{
			// The same condition another provider reports as a 400, carrying
			// the same code here. It is not the cap: raising max_tokens makes
			// it worse, which is why it no longer shares a code with one.
			name: "context window exceeded is not the cap",
			body: sse(
				"message_start", fixStart,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"model_context_window_exceeded"},"usage":{"output_tokens":0}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				"usage in=25 out=0 rsn=0 cr=0 cw=0",
				"err context_length",
			},
		},
		{
			// A stop value this adapter has never seen, on an otherwise ordinary
			// turn. The text streamed and is still discarded: nothing rounds an
			// unrecognized value to a completion, because a turn that reads as
			// complete is one the caller stores.
			name: "an unrecognized stop value ends the turn unclassified",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"most of it"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"quota_exhausted_mid_turn"},"usage":{"output_tokens":6}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "most of it"`,
				"close 0",
				"usage in=25 out=6 rsn=0 cr=0 cw=0",
				"err unknown",
			},
		},
		{
			// message_stop with no message_delta before it, so no stop value was
			// ever named. The flow requires one, so the turn fails.
			name: "a turn that ends without naming a reason fails",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"all of it"}}`,
				"content_block_stop", `{"type":"content_block_stop","index":0}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "all of it"`,
				"close 0",
				"err network",
			},
		},
		{
			name: "paused turn",
			body: sse(
				"message_start", fixStart,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"pause_turn"},"usage":{"output_tokens":3}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				"usage in=25 out=3 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
		{
			name: "in band error after a 200",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"half"}}`,
				"error", `{"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "half"`,
				// The open block stays open: it is incomplete, and the error
				// says so more clearly than a close would.
				"err busy",
			},
		},
		{
			name: "connection dies with no message_stop",
			body: sse(
				"message_start", fixStart,
				"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"cut off"}}`,
			),
			// No terminal event, and no EventClose on the block that was still
			// filling. Both absences are the report: the reply is incomplete and
			// block 0 is the incomplete part of it.
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				`text 0 "cut off"`,
			},
		},
		{
			name: "empty turn still stops",
			body: sse(
				"message_start", fixStart,
				"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
				"message_stop", fixStopFrame,
			),
			want: []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				"usage in=25 out=0 rsn=0 cr=0 cw=0",
				"ok turn",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testProvider(t, sseServer(tc.body))
			got := collect(t, p, hello())
			conform.CheckGrammar(t, got)
			equalSeq(t, got, tc.want)
		})
	}
}

// TestThinkingDisplayDecidesTheEvent pins the one thing this wire does not say.
// A summary and the thinking itself arrive as the same thinking_delta, so the
// request is the only thing that knows which of the two the turn is streaming,
// and which event carries it is how a consumer learns what it is holding. The
// meta is the same either way, because the signature is what the block replays
// by whichever of the two the readable half turns out to be.
func TestThinkingDisplayDecidesTheEvent(t *testing.T) {
	body := sse(
		"message_start", fixStart,
		"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"1071 = 2 x 462 + 147"}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EqQBCgIYAhIM"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":0}`,
		"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`,
		"message_stop", fixStopFrame,
	)
	tests := []struct {
		name    string
		display ThinkingDisplay
		want    string
	}{
		{"summarized", "summarized", `summary 0 part=0 "1071 = 2 x 462 + 147"`},
		{"left to the model", "", `thinking 0 part=0 "1071 = 2 x 462 + 147"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testProvider(t, sseServer(body), WithThinking(Thinking{Type: "enabled", Budget: 1024, Display: tc.display}))
			got := collect(t, p, hello())
			conform.CheckGrammar(t, got)
			equalSeq(t, got, []string{
				"start msg_01 claude-opus-5",
				"usage in=25 out=1 rsn=0 cr=0 cw=0",
				tc.want,
				`meta 0 "EqQBCgIYAhIM"`,
				"close 0",
				"usage in=25 out=9 rsn=0 cr=0 cw=0",
				"ok turn",
			})
		})
	}
}

// TestLargeFrame covers a delta past bufio.Scanner's default 64KB line limit,
// which is the shape of a long thinking summary or an eagerly streamed tool
// argument and would silently truncate a scanner based reader.
func TestLargeFrame(t *testing.T) {
	big := strings.Repeat("abcdefghij", 30000) // 300KB in one delta
	delta, err := json.Marshal(map[string]any{
		"type":  "content_block_delta",
		"index": 0,
		"delta": map[string]string{"type": "text_delta", "text": big},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := sse(
		"message_start", fixStart,
		"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"content_block_delta", string(delta),
		"content_block_stop", `{"type":"content_block_stop","index":0}`,
		"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":75000}}`,
		"message_stop", fixStopFrame,
	)

	p := testProvider(t, sseServer(body))
	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)

	text, ok := got[2].(api.EventText)
	if !ok {
		t.Fatalf("event 2 is %s, want text", render(got[2]))
	}
	if text.Text != big {
		t.Fatalf("text is %d bytes, want %d", len(text.Text), len(big))
	}
	if last := render(got[len(got)-1]); last != "ok turn" {
		t.Fatalf("last event %s", last)
	}
}

func TestFrameOverLimit(t *testing.T) {
	body := sse(
		"message_start", fixStart,
		"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"`+strings.Repeat("x", 4096)+`"}}`,
	)
	p := testProvider(t, sseServer(body), WithTransport(lib.Options{Max: 1024}))
	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	if last := render(got[len(got)-1]); last != "err network" {
		t.Fatalf("last event %s, want error transport", last)
	}
}

func TestMultilineDataFrame(t *testing.T) {
	// A spec compliant producer may split the payload across data: lines; the
	// pieces join with newlines, which JSON tolerates between tokens.
	body := "event: message_start\ndata: {\"type\":\"message_start\",\n" +
		"data: \"message\":{\"id\":\"msg_ml\",\"model\":\"claude-opus-5\"}}\n\n" +
		": a comment nobody has to read\n\n" +
		sse("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
			"message_stop", fixStopFrame)

	p := testProvider(t, sseServer(body))
	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	equalSeq(t, got, []string{"start msg_ml claude-opus-5", "ok turn"})
}

func TestHTTPErrorBeforeStream(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   api.ErrCode
	}{
		{"rate limited", 429, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"},"request_id":"req_1"}`, api.CodeBusy},
		{"bad key", 401, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`, api.CodeAccess},
		{"billing", 402, `{"type":"error","error":{"type":"billing_error","message":"unpaid"}}`, api.CodeAccess},
		{"bad request", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens"}}`, api.CodeInvalid},
		// A conversation that does not fit is a 400 like every other rejected
		// body, and this API says so in prose alone. Classified as one of those,
		// the caller is told to rewrite a request whose only fault is its length
		// and the compaction that is the fix never runs, so the one sentence that
		// names the condition is matched. The row under it is why the match is the
		// whole phrase: a 400 that merely mentions the prompt is not this.
		{"prompt over the window", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 1210970 tokens > 1048576 maximum"}}`, api.CodeContextLength},
		{"a 400 that only mentions the prompt", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt caching is not supported for this model"}}`, api.CodeInvalid},
		{"tier spend cap", 429, `{"type":"error","error":{"type":"rate_limit_error","message":"You have reached your API usage limits: your organization has crossed its monthly API usage threshold, set based on your organization's API tier. You will regain access on 2026-09-01 at 00:00 UTC.","details":{"error_code":"enforced_spend_limit_reached"}}}`, api.CodeAccess},
		{"organization spend limit", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"You have reached your specified API usage limits. You will regain access on 2026-12-01 at 00:00 UTC."}}`, api.CodeAccess},
		{"workspace spend limit", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"You have reached your specified workspace API usage limits. You will regain access on 2026-12-01 at 00:00 UTC."}}`, api.CodeAccess},
		{"missing model", 404, `{"type":"error","error":{"type":"not_found_error","message":"no such model"}}`, api.CodeInvalid},
		{"overloaded", 529, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, api.CodeBusy},
		{"unavailable", 503, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, api.CodeBusy},
		// Out of capacity is CodeBusy and back off; a 500 is one attempt that
		// failed and is retried as sent, which is CodeNetwork.
		{"server error", 500, `{"type":"error","error":{"type":"api_error","message":"boom"}}`, api.CodeNetwork},
		{"gateway timeout", 504, `{"type":"error","error":{"type":"timeout_error","message":"timed out"}}`, api.CodeNetwork},
		// Returned by the edge before the request reaches the API, so it need
		// not use the documented envelope at all. It is the request's byte size
		// and not the context window, so compacting the conversation is not the
		// fix and the code is not CodeContextLength; it answers a body this
		// adapter already measured as fitting, so it is not CodeInvalid either.
		// See TestProviderRefusesBytesMeasuredAsFitting.
		{"too large, not JSON", 413, "<html>413 Request Entity Too Large</html>", api.CodeTooLarge},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("retry-after", "7")
				w.Header().Set("request-id", "req_hdr")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			got := collect(t, p, hello())
			conform.CheckGrammar(t, got)
			if len(got) != 1 {
				t.Fatalf("got %v, want one error", renderAll(got))
			}
			ev, ok := got[0].(api.EventErr)
			if !ok {
				t.Fatalf("got %s, want an error", render(got[0]))
			}
			if ev.Code != tc.want {
				t.Fatalf("code %s, want %s", ev.Code, tc.want)
			}
			var api *APIError
			if !errors.As(ev.Err, &api) {
				t.Fatalf("error %v does not carry an APIError", ev.Err)
			}
			if api.Status != tc.status {
				t.Fatalf("status %d, want %d", api.Status, tc.status)
			}
			if api.RetryAfter != 7*time.Second {
				t.Fatalf("retry after %v, want 7s", api.RetryAfter)
			}
			if api.RequestID == "" {
				t.Fatalf("request id was not kept")
			}
		})
	}
}

func TestRequestShapeOnTheWire(t *testing.T) {
	var got struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
	}
	var headers http.Header
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, sse("message_start", fixStart, "message_delta",
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`, "message_stop", fixStopFrame))
	}, WithBeta("fine-grained-tool-streaming-2025-05-14"))

	collect(t, p, hello())

	if !got.Stream {
		t.Fatalf("stream was not requested")
	}
	if got.Model != testModel {
		t.Fatalf("model %q, want the configured %q", got.Model, testModel)
	}
	if got.MaxTokens != defaultMaxTokens {
		t.Fatalf("max_tokens %d, want the configured default %d", got.MaxTokens, defaultMaxTokens)
	}
	if headers.Get("anthropic-version") != defaultVersion {
		t.Fatalf("anthropic-version %q", headers.Get("anthropic-version"))
	}
	if headers.Get("x-api-key") != "test-key" {
		t.Fatalf("x-api-key %q", headers.Get("x-api-key"))
	}
	if headers.Get("anthropic-beta") != "fine-grained-tool-streaming-2025-05-14" {
		t.Fatalf("anthropic-beta %q", headers.Get("anthropic-beta"))
	}
}

// heldStream serves a turn that starts and then stays open, the way a queued
// request or a running server side tool does, so what ends the turn is the
// caller's context and never the server running out of frames.
func heldStream(t *testing.T) *Provider {
	t.Helper()
	released := make(chan struct{})
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, sse(
			"message_start", fixStart,
			"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"one"}}`,
		))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-released:
		case <-time.After(10 * time.Second):
		}
	})
	// Registered after the server's own cleanup so it runs before it: the
	// handler has to be let go before httptest waits on it.
	t.Cleanup(func() { close(released) })
	return p
}

// TestCancelMidStream pins the cancellation contract. The turn ends where it
// was, the channel closes, and nothing is said about how it ended: the caller
// cancelled, so there is nobody left to tell and ctx.Err() already says it.
// Nothing is held open for a caller that might come back either, which is what
// the sleep is for: it is where a caller doing any work per event spends most
// of its time.
func TestCancelMidStream(t *testing.T) {
	p := heldStream(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got []api.Event
	for e := range p.Stream(ctx, hello()) {
		got = append(got, e)
		if len(got) == 1 {
			cancel()
			time.Sleep(40 * time.Millisecond)
		}
	}
	conform.CheckGrammar(t, got)
	for _, e := range got {
		switch e.(type) {
		case api.EventOk, api.EventErr:
			t.Fatalf("cancelling produced %s; a cancelled turn ends with no terminal event", render(e))
		}
	}
	if ctx.Err() == nil {
		t.Fatalf("the channel closed without the cancel landing")
	}
}

// TestCallerDeadlineIsSilent is the same ending reached without anyone
// cancelling anything: a deadline the caller put on ctx is the caller's own, so
// it ends the turn the same silent way. An idle deadline this adapter keeps is
// the one that is reported, because nobody outside can see it.
func TestCallerDeadlineIsSilent(t *testing.T) {
	p := heldStream(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var got []api.Event
	for e := range p.Stream(ctx, hello()) {
		got = append(got, e)
		if len(got) == 1 {
			time.Sleep(60 * time.Millisecond)
		}
	}
	conform.CheckGrammar(t, got)
	for _, e := range got {
		switch e.(type) {
		case api.EventOk, api.EventErr:
			t.Fatalf("a caller's own deadline produced %s", render(e))
		}
	}
}

func TestIdleTimeout(t *testing.T) {
	released := make(chan struct{})
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, sse("message_start", fixStart))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-released:
		case <-time.After(10 * time.Second):
		}
	}, WithTransport(lib.Options{Idle: 150 * time.Millisecond}))
	defer close(released)

	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	last, ok := got[len(got)-1].(api.EventErr)
	if !ok {
		t.Fatalf("last event %s, want an error", render(got[len(got)-1]))
	}
	if last.Code != api.CodeNetwork {
		t.Fatalf("code %s, want network", last.Code)
	}
	if !errors.Is(last.Err, lib.ErrIdle) {
		t.Fatalf("error %v, want the idle deadline", last.Err)
	}
}

// TestKeepaliveHoldsTheDeadlineOpen is the reason ping frames are consumed
// rather than ignored: a turn that says nothing but stays alive must not be
// cut, and only the layer holding the connection can tell the two apart. The
// caller hears nothing of them.
func TestKeepaliveHoldsTheDeadlineOpen(t *testing.T) {
	p := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, sse("message_start", fixStart))
		w.(http.Flusher).Flush()
		for i := 0; i < 6; i++ {
			time.Sleep(40 * time.Millisecond)
			io.WriteString(w, sse("ping", fixPing))
			w.(http.Flusher).Flush()
		}
		io.WriteString(w, sse(
			"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"late"}}`,
			"content_block_stop", `{"type":"content_block_stop","index":0}`,
			"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
			"message_stop", fixStopFrame,
		))
	}, WithTransport(lib.Options{Idle: 150 * time.Millisecond}))

	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	equalSeq(t, got, []string{
		"start msg_01 claude-opus-5",
		"usage in=25 out=1 rsn=0 cr=0 cw=0",
		`text 0 "late"`,
		"close 0",
		"usage in=25 out=2 rsn=0 cr=0 cw=0",
		"ok turn",
	})
}

func TestMalformedFrame(t *testing.T) {
	body := sse("message_start", fixStart) + "event: content_block_start\ndata: {not json\n\n"
	p := testProvider(t, sseServer(body))
	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	if last := render(got[len(got)-1]); last != "err network" {
		t.Fatalf("last event %s, want error transport", last)
	}
}

// TestBrokenKnownFramesFail pins the strict half of decoding: a frame of a known
// type that breaks the flow fails the turn with CodeNetwork rather than being
// salvaged.
func TestBrokenKnownFramesFail(t *testing.T) {
	text0 := `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
	stop := `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`
	for name, frames := range map[string][]string{
		"no type":                      {"message_start", `{"message":{"id":"m"}}`},
		"start without message":        {"message_start", `{"type":"message_start"}`},
		"block start without index":    {"content_block_start", `{"type":"content_block_start","content_block":{"type":"text"}}`},
		"block start with null block":  {"content_block_start", `{"type":"content_block_start","index":0,"content_block":null}`},
		"second start for open index":  {"content_block_start", text0, "content_block_start", text0},
		"tool without id":              {"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"f","input":{}}}`},
		"delta without index":          {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}`},
		"delta for no block":           {"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"x"}}`},
		"text delta on a tool":         {"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t","name":"f","input":{}}}`, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}`},
		"json delta on text":           {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{"}}`},
		"thinking delta on text":       {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"x"}}`},
		"null delta":                   {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":null}`},
		"absent delta":                 {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","index":0}`},
		"untyped delta":                {"content_block_start", text0, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"","text":"x"}}`},
		"stop without index":           {"content_block_start", text0, "content_block_stop", `{"type":"content_block_stop"}`},
		"stop for no block":            {"content_block_stop", `{"type":"content_block_stop","index":2}`},
		"message stop with open block": {"content_block_start", text0, "message_delta", stop, "message_stop", fixStopFrame},
	} {
		t.Run(name, func(t *testing.T) {
			got := collect(t, testProvider(t, sseServer(sse(append([]string{"message_start", fixStart}, frames...)...))), hello())
			conform.CheckGrammar(t, got)
			if last := render(got[len(got)-1]); last != "err network" {
				t.Fatalf("last event %s, want err network\n%s", last, strings.Join(renderAll(got), "\n"))
			}
		})
	}
}

// TestUnknownFramesAreIgnored is the forward half: a frame type this adapter
// does not know is not decoded past its type, so fields that reuse a known name
// with another shape do not end the turn, and a text delta on an opaque block is
// dropped rather than failing.
func TestUnknownFramesAreIgnored(t *testing.T) {
	got := collect(t, testProvider(t, sseServer(sse(
		"message_start", fixStart,
		"future_frame", `{"type":"future_frame","index":"x","delta":"scalar","message":7}`,
		"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"connector_text","text":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":0}`,
		"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"message_stop", fixStopFrame,
	))), hello())
	conform.CheckGrammar(t, got)
	if last := render(got[len(got)-1]); last != "ok turn" {
		t.Fatalf("last event %s, want ok turn\n%s", last, strings.Join(renderAll(got), "\n"))
	}
}

func TestUnreachableHost(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithBaseURL("http://127.0.0.1:1"))
	got := collect(t, p, hello())
	conform.CheckGrammar(t, got)
	if len(got) != 1 {
		t.Fatalf("got %v, want one error", renderAll(got))
	}
	if ev := got[0].(api.EventErr); ev.Code != api.CodeNetwork {
		t.Fatalf("code %s, want network", ev.Code)
	}
}

func TestEncodeFailureIsATerminalEvent(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"))
	got := collect(t, p, api.Request{})
	conform.CheckGrammar(t, got)
	if ev := got[0].(api.EventErr); ev.Code != api.CodeInvalid {
		t.Fatalf("code %s, want invalid", ev.Code)
	}
}

// TestStopValues pins the whole vocabulary this API ends a turn with, including
// which of the two terminal events each value produces: the type is what decides
// whether the turn is stored, so a table that pinned only the code would be
// pinning half of the answer.
//
// something_new is the row that carries the rule: a named value nobody knows is
// unclassified and never rounded to the nearest completion. An absent stop never
// reaches stopEvent; message_stop fails the turn first.
func TestStopValues(t *testing.T) {
	for native, want := range map[string]string{
		"end_turn":                      "ok turn",
		"stop_sequence":                 "ok turn",
		"tool_use":                      "ok turn",
		"pause_turn":                    "ok turn",
		"max_tokens":                    "err capped",
		"model_context_window_exceeded": "err context_length",
		"refusal":                       "err refused",
		"something_new":                 "err unknown",
	} {
		if got := render(stopEvent(native)); got != want {
			t.Errorf("stopEvent(%q) = %s, want %s", native, got, want)
		}
	}
}

// TestUnknownStopCarriesTheRawValue is the other half of the unknown rule. The
// code says nothing was recognized, so the value itself has to survive on Err or
// whoever reads the log has no way to learn which vocabulary grew.
func TestUnknownStopCarriesTheRawValue(t *testing.T) {
	ev, ok := stopEvent("model_ran_out_of_ideas").(api.EventErr)
	if !ok {
		t.Fatalf("an unrecognized stop produced %s, want an error", render(stopEvent("model_ran_out_of_ideas")))
	}
	if !strings.Contains(ev.Err.Error(), "model_ran_out_of_ideas") {
		t.Fatalf("error %v does not carry the raw stop value", ev.Err)
	}
}

// TestStartCarriesTheConfiguredModel pins which of the two model names a turn is
// stamped with. A pre-4.6 id resolves to a dated snapshot and message_start
// names the snapshot, which is a string no later request writes; the stamp on
// every block of the turn comes from this event and is compared against the
// model the next turn is built with.
func TestStartCarriesTheConfiguredModel(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: "claude-opus-4-5"}}, WithAPIKey("k"), WithBaseURL(httptest.NewServer(sseServer(sse(
		"message_start", `{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-opus-4-5-20250929","usage":{"input_tokens":1,"output_tokens":1}}}`,
		"message_stop", fixStopFrame,
	))).URL))
	got := collect(t, p, hello())
	if model := got[0].(api.EventStart).Model; model != "claude-opus-4-5" {
		t.Fatalf("EventStart.Model %q, want the model the provider was configured with", model)
	}
}

func TestInbandSpendLimitIsAccess(t *testing.T) {
	p := testProvider(t, sseServer(sse(
		"message_start", fixStart,
		"error", `{"type":"error","error":{"type":"rate_limit_error","message":"spend limit","details":{"error_code":"enforced_spend_limit_reached"}}}`,
	)))
	events := collect(t, p, hello())
	last, ok := events[len(events)-1].(api.EventErr)
	if !ok {
		t.Fatalf("the turn ended with %#v, want api.EventErr", events[len(events)-1])
	}
	if last.Code != api.CodeAccess {
		t.Fatalf("code = %q, want %q: a spend limit is not lifted by retrying", last.Code, api.CodeAccess)
	}
	var apiErr *APIError
	if !errors.As(last.Err, &apiErr) || apiErr.ErrorCode != "enforced_spend_limit_reached" {
		t.Fatalf("error %#v does not carry the frame's error_code", last.Err)
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
	e := readAPIError(&lib.HTTPError{Status: 502, Body: []byte(strings.Repeat("é", 2000))})
	if n := len([]rune(e.Message)); n != maxErrorMessage+1 {
		t.Fatalf("message has %d runes, want %d", n, maxErrorMessage+1)
	}
}
