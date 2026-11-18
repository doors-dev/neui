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

package lib_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/provider/claude"
	"neui.dev/provider/deepseek"
	"neui.dev/provider/gpt"
)

type corpusCase struct {
	name string
	body string
	want string
}

func sseFrame(event, data string) string {
	if event == "" {
		return "data: " + data + "\n\n"
	}
	return "event: " + event + "\ndata: " + data + "\n\n"
}

func splitFrame(event string, parts ...string) string {
	return "event: " + event + "\ndata: " + strings.Join(parts, "\ndata: ") + "\n\n"
}

func quoted(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func filler(seed string, n int) string {
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		fmt.Fprintf(&b, "%s %d <&> \"q\" \\ ", seed, i)
	}
	return b.String()[:n]
}

func claudeCorpus() []corpusCase {
	start := `{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-opus-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":2500,"cache_creation_input_tokens":300,"cache_read_input_tokens":1200,"output_tokens":1}}}`
	results := make([]string, 0, 40)
	for i := range 40 {
		results = append(results, fmt.Sprintf(`{"type":"web_search_result","url":"https://example.com/%d","title":%s,"encrypted_content":%s,"page_age":"2 days"}`, i, quoted(filler("title", 40)), quoted(filler("enc", 4000))))
	}
	rich := ": connected\n\n" +
		sseFrame("message_start", start) +
		sseFrame("ping", `{"type": "ping"}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think"}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":`+quoted(filler("think", 90000))+`}}`) +
		": keepalive\n\n" +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":" done."}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":`+quoted(filler("sig", 3000))+`}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":0}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"redacted_thinking","data":`+quoted(filler("redacted", 5000))+`}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":1}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"query\":"}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"weather <SF> & co\"}"}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":2}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":3,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[`+strings.Join(results, ",")+`]}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":3}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":4,"content_block":{"type":"text","text":""}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":4,"delta":{"type":"text_delta","text":"It is "}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":4,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","cited_text":"sunny","url":"https://example.com/1","title":"Weather","encrypted_index":"Eo8BCioIAhgB"}}}`) +
		splitFrame("content_block_delta", `{"type":"content_block_delta",`, `"index":4,"delta":{"type":"text_delta",`, `"text":`+quoted(filler("text", 120000))+`}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":4,"delta":{"type":"text_delta","text":"é 😀 <b>&amp;</b>"}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":4}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":5,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":""}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":`+quoted(quoted(filler("loc", 70000)))+`}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":"}"}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":5}`) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":6,"content_block":{"type":"tool_use","id":"toolu_2","name":"noop","input":{}}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":6}`) +
		sseFrame("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":4321,"server_tool_use":{"web_search_requests":1}}}`) +
		sseFrame("message_stop", `{"type":"message_stop"}`)
	capped := sseFrame("message_start", start) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"partial"}}`) +
		sseFrame("content_block_stop", `{"type":"content_block_stop","index":0}`) +
		sseFrame("message_delta", `{"type":"message_delta","delta":{"stop_reason":"max_tokens","stop_sequence":null},"usage":{"output_tokens":64000}}`) +
		sseFrame("message_stop", `{"type":"message_stop"}`)
	overloaded := sseFrame("message_start", start) +
		sseFrame("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`) +
		sseFrame("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"half"}}`) +
		sseFrame("error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	return []corpusCase{
		{name: "claude/rich", body: rich, want: "7556f02463a7de45775c6ee4929759215ea85df202797b2e05a6e06416a24fa9"},
		{name: "claude/capped", body: capped, want: "c7b0c9e2a86392a1b8e34087bc891fbb42a566d62f0b6f02d1bd023dc9794521"},
		{name: "claude/overloaded", body: overloaded, want: "b40830a64c39306adb05c6c353ec1779e5602a646df1c11e1f76b882cab7f435"},
	}
}

func gptCorpus() []corpusCase {
	usage := `"usage":{"input_tokens":2000,"input_tokens_details":{"cached_tokens":1500,"cache_write_tokens":200},"output_tokens":1186,"output_tokens_details":{"reasoning_tokens":1024},"total_tokens":3186}`
	enc := quoted(filler("encrypted", 60000))
	long := filler("answer", 110000)
	args := quoted(filler("arg", 50000))
	reasoning := `{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking the map."}],"encrypted_content":` + enc + `}`
	message := `{"id":"msg_1","type":"message","status":"completed","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":` + quoted("Hello "+long) + `,"annotations":[]}]}`
	call := `{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_1","name":"get_weather","arguments":` + quoted(`{"city":`+args+`}`) + `}`
	search := `{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"weather <SF>"}}`
	rich := sseFrame("response.created", `{"type":"response.created","sequence_number":0,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`) +
		sseFrame("response.in_progress", `{"type":"response.in_progress","sequence_number":1,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`) +
		": ping\n\n" +
		sseFrame("response.output_item.added", `{"type":"response.output_item.added","sequence_number":2,"output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
		sseFrame("response.reasoning_summary_part.added", `{"type":"response.reasoning_summary_part.added","sequence_number":3,"item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""}}`) +
		sseFrame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","sequence_number":4,"item_id":"rs_1","output_index":0,"summary_index":0,"delta":"Checking"}`) +
		sseFrame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","sequence_number":5,"item_id":"rs_1","output_index":0,"summary_index":0,"delta":" the map."}`) +
		sseFrame("response.reasoning_summary_text.done", `{"type":"response.reasoning_summary_text.done","sequence_number":6,"item_id":"rs_1","output_index":0,"summary_index":0,"text":"Checking the map."}`) +
		sseFrame("response.reasoning_summary_part.done", `{"type":"response.reasoning_summary_part.done","sequence_number":7,"item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":"Checking the map."}}`) +
		sseFrame("response.output_item.done", `{"type":"response.output_item.done","sequence_number":8,"output_index":0,"item":`+reasoning+`}`) +
		sseFrame("response.output_item.added", `{"type":"response.output_item.added","sequence_number":9,"output_index":1,"item":{"id":"ws_1","type":"web_search_call","status":"in_progress"}}`) +
		sseFrame("response.web_search_call.in_progress", `{"type":"response.web_search_call.in_progress","sequence_number":10,"output_index":1,"item_id":"ws_1"}`) +
		sseFrame("response.web_search_call.completed", `{"type":"response.web_search_call.completed","sequence_number":11,"output_index":1,"item_id":"ws_1"}`) +
		sseFrame("response.output_item.done", `{"type":"response.output_item.done","sequence_number":12,"output_index":1,"item":`+search+`}`) +
		sseFrame("response.output_item.added", `{"type":"response.output_item.added","sequence_number":13,"output_index":2,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant","phase":"final_answer","content":[]}}`) +
		sseFrame("response.content_part.added", `{"type":"response.content_part.added","sequence_number":14,"item_id":"msg_1","output_index":2,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`) +
		sseFrame("response.output_text.delta", `{"type":"response.output_text.delta","sequence_number":15,"item_id":"msg_1","output_index":2,"content_index":0,"delta":"Hello ","obfuscation":"xxxx"}`) +
		splitFrame("response.output_text.delta", `{"type":"response.output_text.delta","sequence_number":16,`, `"item_id":"msg_1","output_index":2,"content_index":0,`, `"delta":`+quoted(long)+`}`) +
		sseFrame("response.output_text.annotation.added", `{"type":"response.output_text.annotation.added","sequence_number":17,"item_id":"msg_1","output_index":2,"content_index":0,"annotation_index":0,"annotation":{"type":"url_citation","url":"https://example.com","title":"Ex","start_index":0,"end_index":5}}`) +
		sseFrame("response.output_text.done", `{"type":"response.output_text.done","sequence_number":18,"item_id":"msg_1","output_index":2,"content_index":0,"text":`+quoted("Hello "+long)+`}`) +
		sseFrame("response.content_part.done", `{"type":"response.content_part.done","sequence_number":19,"item_id":"msg_1","output_index":2,"content_index":0,"part":{"type":"output_text","text":`+quoted("Hello "+long)+`,"annotations":[]}}`) +
		sseFrame("response.output_item.done", `{"type":"response.output_item.done","sequence_number":20,"output_index":2,"item":`+message+`}`) +
		sseFrame("response.output_item.added", `{"type":"response.output_item.added","sequence_number":21,"output_index":3,"item":{"id":"fc_1","type":"function_call","status":"in_progress","call_id":"call_1","name":"get_weather","arguments":""}}`) +
		sseFrame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","sequence_number":22,"item_id":"fc_1","output_index":3,"delta":"{\"city\":"}`) +
		sseFrame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","sequence_number":23,"item_id":"fc_1","output_index":3,"delta":`+quoted(args+"}")+`}`) +
		sseFrame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","sequence_number":24,"item_id":"fc_1","output_index":3,"arguments":`+quoted(`{"city":`+args+`}`)+`}`) +
		sseFrame("response.output_item.done", `{"type":"response.output_item.done","sequence_number":25,"output_index":3,"item":`+call+`}`) +
		sseFrame("response.completed", `{"type":"response.completed","sequence_number":26,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","output":[`+reasoning+`,`+search+`,`+message+`,`+call+`],`+usage+`}}`)
	incomplete := sseFrame("response.created", `{"type":"response.created","sequence_number":0,"response":{"id":"resp_2","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`) +
		sseFrame("response.output_item.added", `{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"msg_2","type":"message","status":"in_progress","role":"assistant","content":[]}}`) +
		sseFrame("response.output_text.delta", `{"type":"response.output_text.delta","sequence_number":2,"item_id":"msg_2","output_index":0,"content_index":0,"delta":"cut"}`) +
		sseFrame("response.incomplete", `{"type":"response.incomplete","sequence_number":3,"response":{"id":"resp_2","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},`+usage+`}}`)
	failed := sseFrame("response.created", `{"type":"response.created","sequence_number":0,"response":{"id":"resp_3","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`) +
		sseFrame("error", `{"type":"error","sequence_number":1,"code":"server_error","message":"The server had an error <retry>","param":null}`)
	return []corpusCase{
		{name: "gpt/rich", body: rich, want: "2b41fd6f6f50e565746b39a1138be96829e72b6fd77743d055617a58ea90245b"},
		{name: "gpt/incomplete", body: incomplete, want: "9ee4c59167f0e951ccae9750370c83df524c2e97a39f38b272a90519dac2f150"},
		{name: "gpt/error", body: failed, want: "141300441a84d7513e6b73a81dc2fbbdddba21450625dbd75a3483667a12b9b3"},
	}
}

func deepseekCorpus() []corpusCase {
	chunk := func(delta, finish string) string {
		return sseFrame("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","system_fingerprint":"fp_1","choices":[{"index":0,"delta":`+delta+`,"logprobs":null,"finish_reason":`+finish+`}]}`)
	}
	usage := `{"prompt_tokens":3000,"completion_tokens":900,"total_tokens":3900,"prompt_tokens_details":{"cached_tokens":2048},"completion_tokens_details":{"reasoning_tokens":400},"prompt_cache_hit_tokens":2048,"prompt_cache_miss_tokens":952}`
	rich := ": keep-alive\n\n" +
		chunk(`{"role":"assistant","content":null,"reasoning_content":""}`, "null") +
		chunk(`{"reasoning_content":"Let me"}`, "null") +
		chunk(`{"reasoning_content":`+quoted(filler("reason", 80000))+`}`, "null") +
		": keep-alive\n\n" +
		chunk(`{"content":"Hello <b> & "}`, "null") +
		chunk(`{"content":`+quoted(filler("content", 100000))+`}`, "null") +
		chunk(`{"tool_calls":[{"index":0,"id":"call_0","type":"function","function":{"name":"get_weather","arguments":""}}]}`, "null") +
		chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}`, "null") +
		chunk(`{"tool_calls":[{"index":0,"function":{"arguments":`+quoted(quoted(filler("city", 60000))+"}")+`}}]}`, "null") +
		chunk(`{"tool_calls":[{"index":1,"id":"call_1","type":"function","function":{"name":"noop","arguments":"{}"}}]}`, "null") +
		chunk(`{"content":""}`, `"tool_calls"`) +
		sseFrame("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[],"usage":`+usage+`}`) +
		"data: [DONE]\n\n"
	capped := chunk(`{"role":"assistant","content":"cut"}`, "null") +
		sseFrame("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"content":" off"},"finish_reason":"length"}],"usage":`+usage+`}`) +
		"data: [DONE]\n\n"
	return []corpusCase{
		{name: "deepseek/rich", body: rich, want: "99a0bd33cfe610214c77fd703c37424ccf9cf395bc304457cf1f676b17ce0553"},
		{name: "deepseek/capped", body: capped, want: "1ba26a71013fa45e616741f0d8038f05709c073617d9b7c1f0be77c0b819e4d7"},
	}
}

func chunked(body string, done chan<- struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i, size := 0, 1; i < len(body); size = size*7%4093 + 1 {
			end := min(i+size, len(body))
			if _, err := w.Write([]byte(body[i:end])); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			i = end
		}
	}
}

func dump(events []api.Event) string {
	var b strings.Builder
	for _, e := range events {
		if e, ok := e.(api.EventErr); ok {
			fmt.Fprintf(&b, "%T %s %s\n", e, e.Code, e.Err)
			continue
		}
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Fprintf(&b, "%T marshal: %v\n", e, err)
			continue
		}
		fmt.Fprintf(&b, "%T %s\n", e, data)
	}
	return b.String()
}

type streamer interface {
	Stream(context.Context, api.Request) <-chan api.Event
}

func corpusProvider(t *testing.T, name, url string) streamer {
	t.Helper()
	var p streamer
	var err error
	switch strings.Split(name, "/")[0] {
	case "claude":
		p, err = claude.New(claude.Conf{Model: api.Model{Name: "claude-opus-5"}}, claude.WithAPIKey("k"), claude.WithBaseURL(url))
	case "gpt":
		p, err = gpt.New(gpt.Conf{Model: api.Model{Name: "gpt-5.6-sol"}}, gpt.WithAPIKey("k"), gpt.WithBaseURL(url))
	case "deepseek":
		p, err = deepseek.New(deepseek.Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, deepseek.WithAPIKey("k"), deepseek.WithBaseURL(url))
	}
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func corpus() []corpusCase {
	return append(append(claudeCorpus(), gptCorpus()...), deepseekCorpus()...)
}

func runCorpus(t *testing.T, c corpusCase, late bool) []api.Event {
	t.Helper()
	done := make(chan struct{})
	srv := httptest.NewServer(chunked(c.body, done))
	defer srv.Close()
	p := corpusProvider(t, c.name, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req := api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "hi"}}}}}
	ch := p.Stream(ctx, req)
	if late {
		<-done
	}
	var got []api.Event
	for e := range ch {
		got = append(got, e)
	}
	return got
}

// Every converter keeps only copies of a frame's data, so reusing frame
// buffers changes nothing a consumer sees: these hashes are the event streams
// the corpus produced before frame data was pooled, read both as the events
// arrive and only after the whole body was written, which leaves later frames
// to overwrite any buffer an earlier event still pointed into.
func TestCorpusEventStreamsUnchanged(t *testing.T) {
	for _, c := range corpus() {
		for _, late := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/late=%v", c.name, late), func(t *testing.T) {
				got := dump(runCorpus(t, c, late))
				sum := sha256.Sum256([]byte(got))
				if hex.EncodeToString(sum[:]) != c.want {
					t.Fatalf("event stream hash %x, want %s; events:\n%.4000s", sum, c.want, got)
				}
			})
		}
	}
}
