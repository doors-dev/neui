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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const (
	goldenRequestsHash = "36a07bdba7c7ebc13cc361cf55eb76eb5d387ee8bc796d68f8f166736c46c841"
	goldenStreamsHash  = "97c72348a203994455645e8a10fb44d202daf233b2edd2ff615fe166ade07000"
)

func goldenBytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

func goldenPick[T any](r *rand.Rand, xs ...T) T {
	return xs[r.IntN(len(xs))]
}

func goldenAttachment(r *rand.Rand) api.UserContent {
	switch r.IntN(6) {
	case 0:
		return api.ContentImage{Data: goldenBytes(r, r.IntN(3000)), Mime: string(goldenPick(r, api.ImagePNG, api.ImageJPEG, api.ImageWEBP, api.ImageGIF))}
	case 1:
		return api.ContentImage{Data: goldenBytes(r, 16), Mime: goldenPick(r, "image/svg+xml", "")}
	case 2:
		return api.ContentDocument{Data: goldenBytes(r, r.IntN(3000)), Mime: string(goldenPick(r, api.DocumentPDF, api.DocumentText, api.DocumentJSON, api.DocumentMarkdown, api.DocumentCSV, api.DocumentHTML, api.DocumentXML, api.DocumentYAML, api.DocumentRTF, api.DocumentDOC, api.DocumentXLS, api.DocumentDOCX, api.DocumentXLSX, api.DocumentPPTX)), Name: goldenPick(r, "", "report", "a b.pdf")}
	case 3:
		return api.ContentDocument{Data: goldenBytes(r, 8), Mime: goldenPick(r, "application/zip", "")}
	default:
		return api.ContentText{Text: goldenPick(r, "hi", "", "line\nbreak <tag> & \"quote\"   é")}
	}
}

func goldenRequest(r *rand.Rand) api.Request {
	var req api.Request
	if r.IntN(2) == 0 {
		req.System = goldenPick(r, "be brief", "you are a helper\nwith two lines")
	}
	if r.IntN(2) == 0 {
		req.Tools = []api.Tool{
			{Name: "get_weather", Desc: "weather by city", Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}}},
			{Name: "now", Desc: "the time"},
		}
		req.ToolChoice = goldenPick(r, api.ToolChoice(""), api.ToolChoiceAuto, api.ToolChoiceNone, api.ToolChoiceAny, api.ToolChoice("get_weather"))
	}
	turns := 1 + r.IntN(4)
	call := 0
	for range turns {
		var u api.Contents
		for range 1 + r.IntN(3) {
			u = append(u, goldenAttachment(r))
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleUser, Content: u})
		if r.IntN(4) == 0 {
			continue
		}
		var a api.Contents
		var ids []string
		for range 1 + r.IntN(4) {
			switch r.IntN(7) {
			case 0:
				a = append(a, api.ContentText{Text: "the answer"})
			case 1:
				own := goldenPick(r, true, false)
				phase := goldenPick(r, "commentary", "final_answer")
				meta := json.RawMessage(`{"phase":"` + phase + `"}`)
				if own {
					meta = phaseMeta(phase)
				}
				a = append(a, api.ContentText{Text: "preamble", Meta: meta})
			case 2:
				a = append(a, api.ContentReasoning{Model: goldenPick(r, testModel, "other"), Summary: map[int]string{0: "thinking", 1: "more"}, Meta: goldenPick(r, json.RawMessage(nil), ownMeta(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"SEALED"}`), ownMeta(`{"id":"rs_2","type":"reasoning"}`))})
			case 3:
				a = append(a, api.ContentReasoning{Model: testModel, Thoughts: map[int]string{0: "raw thought"}})
			case 4:
				a = append(a, api.ContentOpaque{Model: goldenPick(r, testModel, "other"), Kind: goldenPick(r, "gpt.web_search_call", "gpt.code_interpreter_call", "gpt.custom_tool_call"), Data: goldenPick(r, json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed"}`), json.RawMessage(`{"type":"code_interpreter_call","code":""}`)), Deltas: goldenPick(r, []string(nil), []string{"print(", "1)"})})
			default:
				call++
				id := fmt.Sprintf("call_%d", call)
				ids = append(ids, id)
				a = append(a, api.ContentToolUse{ID: id, Name: goldenPick(r, "get_weather", "now"), Input: goldenPick(r, json.RawMessage(nil), json.RawMessage(`{}`), json.RawMessage(`{"city":"Paris"}`), json.RawMessage(`{"city":`))})
			}
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleAssistant, Content: a})
		if len(ids) == 0 {
			continue
		}
		var results api.Contents
		for _, id := range ids {
			var c api.UserContents
			for range r.IntN(3) {
				c = append(c, goldenAttachment(r))
			}
			results = append(results, api.ContentToolResult{ID: id, Content: c, Error: goldenPick(r, api.ToolOk, api.ToolOk, api.ToolErrRuntime, api.ToolErrJSON)})
		}
		if r.IntN(3) == 0 {
			results = append(results, api.ContentText{Text: "and also"})
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleUser, Content: results})
	}
	return req
}

func goldenOptions(r *rand.Rand) []Option {
	all := [][]Option{
		{WithMaxTokens(1000)},
		{WithEffort(goldenPick(r, EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax))},
		{WithReasoningSummary(goldenPick(r, SummaryAuto, SummaryConcise, SummaryDetailed))},
		{WithReasoningMode(goldenPick(r, ReasoningStandard, ReasoningPro))},
		{WithReasoningContext(goldenPick(r, ContextAuto, ContextCurrentTurn, ContextAllTurns))},
		{WithReasoningReplay(goldenPick(r, ReplayAll, ReplayAll, ReplayNever))},
		{WithVerbosity(goldenPick(r, VerbosityLow, VerbosityMedium, VerbosityHigh))},
		{WithParallelToolCalls(r.IntN(2) == 0), WithStrictTools(r.IntN(2) == 0)},
		{WithStore(r.IntN(2) == 0), WithInclude("reasoning.encrypted_content")},
		{WithPromptCacheKey("thread-1"), WithPromptCacheOptions(goldenPick(r, CacheImplicit, CacheExplicit), goldenPick(r, CacheTTL30M, CacheTTL12H))},
		{WithServiceTier(goldenPick(r, TierAuto, TierDefault, TierFlex, TierScale, TierPriority, TierFast))},
		{WithMetadata(map[string]string{"b": "2", "a": "1"}), WithObfuscation(r.IntN(2) == 0)},
	}
	var opts []Option
	if r.IntN(10) == 0 {
		opts = append(opts, WithTemperature(0.5), WithTopP(0.9))
	}
	for _, o := range all {
		if r.IntN(3) == 0 {
			opts = append(opts, o...)
		}
	}
	return opts
}

func goldenRequestsText(t testing.TB) string {
	r := rand.New(rand.NewPCG(1, 2))
	var out strings.Builder
	for i := range 1500 {
		model := goldenPick(r, GPT56Sol, GPT6Astra, GPT61Terra, api.Model{Name: "gpt-4.1"}, api.Model{Name: "o3"})
		opts := append([]Option{WithAPIKey("k"), WithBaseURL("http://golden.invalid")}, goldenOptions(r)...)
		req := goldenRequest(r)
		fmt.Fprintf(&out, "== %d %s\n", i, model.Name)
		p, err := New(Conf{Model: model}, opts...)
		if err != nil {
			fmt.Fprintf(&out, "new: %v\n", err)
			continue
		}
		httpReq, sent, err := p.request(context.Background(), req)
		if err != nil {
			fmt.Fprintf(&out, "request: %d %v\n", sent, err)
			continue
		}
		body, err := io.ReadAll(httpReq.Body)
		if err != nil {
			t.Fatal(err)
		}
		_ = httpReq.Body.Close()
		fmt.Fprintf(&out, "%d %s\n", sent, body)
	}
	return out.String()
}

func goldenEvent(e api.Event) string {
	switch e := e.(type) {
	case api.EventStart:
		return fmt.Sprintf("start %q %q", e.ID, e.Model)
	case api.EventText:
		return fmt.Sprintf("text %d %q", e.Index, e.Text)
	case api.EventTool:
		return fmt.Sprintf("tool %d %q %q %q", e.Index, e.ID, e.Name, e.Args)
	case api.EventThinking:
		return fmt.Sprintf("thinking %d %d %q", e.Index, e.Part, e.Text)
	case api.EventSummary:
		return fmt.Sprintf("summary %d %d %q", e.Index, e.Part, e.Text)
	case api.EventMeta:
		return fmt.Sprintf("meta %d %q", e.Index, e.Data)
	case api.EventOpaque:
		return fmt.Sprintf("opaque %d %q %q %q", e.Index, e.Kind, e.Data, e.Delta)
	case api.EventClose:
		return fmt.Sprintf("close %d", e.Index)
	case api.EventUsage:
		return fmt.Sprintf("usage %+v", e)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		var a *APIError
		if errors.As(e.Err, &a) {
			return fmt.Sprintf("err %s %T %+v %q", e.Code, e.Err, *a, e.Err.Error())
		}
		return fmt.Sprintf("err %s %T %q", e.Code, e.Err, e.Err.Error())
	default:
		return fmt.Sprintf("? %T", e)
	}
}

func goldenQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func goldenItem(r *rand.Rand, i int) string {
	status := goldenPick(r, "completed", "in_progress", "incomplete")
	switch r.IntN(7) {
	case 0, 1:
		phase := goldenPick(r, "", `,"phase":"commentary"`, `,"phase":"final_answer"`)
		return fmt.Sprintf(`{"id":"msg_%d","type":"message","status":%q,"role":"assistant","content":[]%s}`, i, status, phase)
	case 2:
		return fmt.Sprintf(`{"id":"fc_%d","type":"function_call","status":%q,"call_id":%s,"name":%s,"arguments":""}`, i, status, goldenQuote(goldenPick(r, "", "call_a", "call_b")), goldenQuote(goldenPick(r, "get_weather", "")))
	case 3:
		enc := goldenPick(r, "", `,"encrypted_content":"SEALED"`)
		return fmt.Sprintf(`{"id":"rs_%d","type":"reasoning","summary":[],"status":%q%s}`, i, status, enc)
	case 4:
		return fmt.Sprintf(`{"id":"ws_%d","type":"web_search_call","status":%q}`, i, status)
	case 5:
		return fmt.Sprintf(`{"id":"ct_%d","type":"custom_tool_call","call_id":"call_c","name":"n","input":"x"}`, i)
	default:
		return goldenPick(r, `5`, `null`, `{"type":7}`)
	}
}

func goldenFrame(r *rand.Rand, i int) string {
	oi, ci, si := r.IntN(4), r.IntN(3), r.IntN(2)
	delta := goldenQuote(goldenPick(r, "", "a", "Hello", " wörld", `"q"`, `{"k":`, "1}", "\n"))
	usage := `{"input_tokens":2000,"input_tokens_details":{"cached_tokens":1500,"cache_write_tokens":200},"output_tokens":1186,"output_tokens_details":{"reasoning_tokens":1024}}`
	switch r.IntN(22) {
	case 0:
		return fmt.Sprintf(`{"type":"response.created","response":{"id":"resp_%d","model":"served","status":"in_progress","usage":null}}`, i)
	case 1:
		return fmt.Sprintf(`{"type":"response.in_progress","response":{"id":"resp_x","status":"in_progress","usage":%s}}`, usage)
	case 2, 3:
		return fmt.Sprintf(`{"type":"response.output_item.added","output_index":%d,"item":%s}`, oi, goldenItem(r, i))
	case 4, 5:
		return fmt.Sprintf(`{"type":"response.output_item.done","output_index":%d,"item":%s}`, oi, goldenItem(r, i))
	case 6:
		return fmt.Sprintf(`{"type":"response.content_part.added","output_index":%d,"content_index":%d,"part":{"type":"output_text","text":""}}`, oi, ci)
	case 7, 8:
		return fmt.Sprintf(`{"type":"response.output_text.delta","output_index":%d,"content_index":%d,"delta":%s}`, oi, ci, delta)
	case 9:
		return fmt.Sprintf(`{"type":"response.refusal.delta","output_index":%d,"content_index":%d,"delta":%s}`, oi, ci, delta)
	case 10:
		return fmt.Sprintf(`{"type":%q,"output_index":%d,"content_index":%d,"text":"x"}`, goldenPick(r, "response.output_text.done", "response.refusal.done", "response.content_part.done"), oi, ci)
	case 11, 12:
		return fmt.Sprintf(`{"type":"response.function_call_arguments.delta","output_index":%d,"delta":%s}`, oi, delta)
	case 13:
		return fmt.Sprintf(`{"type":"response.function_call_arguments.done","output_index":%d,"arguments":%s}`, oi, goldenQuote(goldenPick(r, "", `{"city":"Paris"}`)))
	case 14:
		return fmt.Sprintf(`{"type":%q,"output_index":%d,"delta":%s}`, goldenPick(r, "response.code_interpreter_call_code.delta", "response.mcp_call_arguments.delta", "response.custom_tool_call_input.delta"), oi, delta)
	case 15:
		return fmt.Sprintf(`{"type":"response.reasoning_summary_text.delta","output_index":%d,"summary_index":%d,"delta":%s}`, oi, si, delta)
	case 16:
		return fmt.Sprintf(`{"type":"response.reasoning_text.delta","output_index":%d,"content_index":%d,"delta":%s}`, oi, ci, delta)
	case 17:
		return fmt.Sprintf(`{"type":"response.future_thing","output_index":%d,"item":{"type":"x"},"response":null}`, oi)
	case 18:
		return `{"type":"response.output_text.annotation.added","output_index":0,"content_index":0,"annotation":{"type":"url_citation"}}`
	case 19:
		return fmt.Sprintf(`{"type":"response.web_search_call.searching","output_index":%d,"sequence_number":%d}`, oi, i)
	case 20:
		return fmt.Sprintf(`{"type":"response.output_text.delta","output_index":%d,"content_index":%d,"delta":%s,"response":{"id":"resp_y","status":"in_progress","usage":%s}}`, oi, ci, delta, usage)
	default:
		return `{"type":"response.queued","response":{"id":"resp_q","status":"queued"}}`
	}
}

func goldenTerminal(r *rand.Rand) string {
	usage := goldenPick(r, `null`, `{"input_tokens":20,"input_tokens_details":{"cached_tokens":5},"output_tokens":7,"output_tokens_details":{"reasoning_tokens":3}}`, `{"input_tokens":1,"output_tokens":-3}`)
	switch r.IntN(9) {
	case 0, 1:
		return fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":%s}}`, usage)
	case 2:
		return fmt.Sprintf(`{"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":%q},"usage":%s}}`, goldenPick(r, "max_output_tokens", "content_filter", "other", ""), usage)
	case 3:
		return fmt.Sprintf(`{"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"code":%q,"message":"it broke"},"usage":%s}}`, goldenPick(r, "server_error", "rate_limit_exceeded", "context_length_exceeded", "invalid_prompt", "vector_store_timeout", ""), usage)
	case 4:
		return fmt.Sprintf(`{"type":"error","code":%q,"message":%s,"param":"input"}`, goldenPick(r, "server_error", "rate_limit_exceeded", "context_length_exceeded", "insufficient_quota", "invalid_request_error", ""), goldenQuote(goldenPick(r, "bad", "Your input exceeds the context window of this model.")))
	case 5:
		return `{"type":"error","error":{"type":"invalid_request_error","code":"string_above_max_length","message":"too long","param":"input[0]"}}`
	case 6:
		return fmt.Sprintf(`{"type":"response.in_progress","response":{"id":"resp_1","status":"cancelled","usage":%s}}`, usage)
	case 7:
		return `{"type":"response.output_text.delta","delta":`
	default:
		return `[1,2]`
	}
}

func goldenEnding(r *rand.Rand) error {
	return goldenPick[error](r,
		io.EOF, io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded, lib.ErrIdle, errors.New("boom"),
		&lib.HTTPError{Status: 429, Header: http.Header{"Retry-After": {"3"}}, Body: []byte(`{"error":{"message":"slow down","type":"requests","code":"rate_limit_exceeded"}}`)},
		&lib.HTTPError{Status: 400, Body: []byte(`{"error":{"message":"Your input exceeds the context window of this model.","type":"invalid_request_error","code":"context_length_exceeded"}}`)},
		&lib.HTTPError{Status: 413, Body: []byte(`not json`)},
		&lib.HTTPError{Status: 500},
	)
}

func goldenStreamsText() string {
	r := rand.New(rand.NewPCG(3, 4))
	var out strings.Builder
	for i := range 3000 {
		c := newConverter(goldenPick(r, testModel, "gpt-6-astra"), 1234, goldenPick(r, 0, 999))
		var all []api.Event
		ended := false
		frames := r.IntN(30)
		for j := range frames + 1 {
			data := goldenFrame(r, j)
			if j == 0 && r.IntN(8) > 0 {
				data = fmt.Sprintf(`{"type":"response.created","response":{"id":"resp_%d","model":"served","status":"in_progress","usage":null}}`, i)
			}
			if j == frames {
				if r.IntN(5) == 0 {
					break
				}
				data = goldenTerminal(r)
			}
			events, more := c.Convert(lib.Frame{Event: "x", Data: []byte(data)})
			all = append(all, events...)
			if !more {
				ended = true
				break
			}
		}
		if !ended {
			all = append(all, c.End(goldenEnding(r))...)
		}
		fmt.Fprintf(&out, "== %d\n", i)
		for _, e := range all {
			out.WriteString(goldenEvent(e))
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func goldenHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// The request corpus was rendered by the encoder before the request path was
// optimized, and the stream corpus by the converter once decoding became
// strict. The hashes pin those renderings: a request body that changes by one
// byte, or an event stream that changes in any field, fails here.
func TestGoldenRequestBodies(t *testing.T) {
	if got := goldenHash(goldenRequestsText(t)); got != goldenRequestsHash {
		t.Fatalf("request corpus hash %s, want %s", got, goldenRequestsHash)
	}
}

func TestGoldenEventStreams(t *testing.T) {
	if got := goldenHash(goldenStreamsText()); got != goldenStreamsHash {
		t.Fatalf("event stream corpus hash %s, want %s", got, goldenStreamsHash)
	}
}

func BenchmarkConvertTextTurn(b *testing.B) {
	frames := [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","status":"in_progress","usage":null}}`),
		[]byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant","content":[],"phase":"final_answer"}}`),
	}
	for range 1000 {
		frames = append(frames, []byte(`{"type":"response.output_text.delta","sequence_number":12,"item_id":"msg_1","output_index":0,"content_index":0,"delta":" token","logprobs":[],"obfuscation":"abcd"}`))
	}
	frames = append(frames,
		[]byte(`{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"x"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":20,"output_tokens":7}}}`),
	)
	b.ReportAllocs()
	for b.Loop() {
		c := newConverter(testModel, 0, 0)
		n := 0
		for _, f := range frames {
			events, _ := c.Convert(lib.Frame{Data: f})
			n += len(events)
		}
		if n != 1006 {
			b.Fatalf("%d events", n)
		}
	}
}
