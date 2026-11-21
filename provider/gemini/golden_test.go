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
	goldenRequestsHash = "f3982d3000d9964c67654f93e68b6ceabc6990de9c512c34253f465bdbcfdfe3"
	goldenStreamsHash  = "0d2f8e517b5844fb2865c80cb5fd085695a989171939c8d9c05fcb5e6ce38786"
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
	switch r.IntN(7) {
	case 0:
		return api.ContentImage{Data: goldenBytes(r, r.IntN(3000)), Mime: goldenPick(r, string(api.ImagePNG), string(api.ImageJPEG), string(api.ImageWEBP), string(api.ImageGIF), "image/jpg", " Image/PNG ")}
	case 1:
		return api.ContentImage{Data: goldenBytes(r, 16), Mime: goldenPick(r, "image/svg+xml", "image/bmp", "")}
	case 2:
		return api.ContentDocument{Data: goldenBytes(r, r.IntN(3000)), Mime: string(goldenPick(r, api.DocumentPDF, api.DocumentCSV, api.DocumentDOC, api.DocumentXLS, api.DocumentDOCX, api.DocumentXLSX, api.DocumentPPTX)), Name: goldenPick(r, "", "report", "a b.pdf")}
	case 3:
		return api.ContentDocument{Data: []byte(goldenPick(r, "plain <words> & more", "# heading\n\ntext", `{"k":[1,2]}`, "a: b\n", "\xff\xfe")), Mime: string(goldenPick(r, api.DocumentText, api.DocumentJSON, api.DocumentMarkdown, api.DocumentHTML, api.DocumentXML, api.DocumentYAML, api.DocumentRTF)), Name: "notes"}
	case 4:
		return api.ContentDocument{Data: goldenPick(r, []byte(nil), goldenBytes(r, 8)), Mime: goldenPick(r, "application/zip", "", "application/pdf")}
	default:
		return api.ContentText{Text: goldenPick(r, "hi", "", "  ", "line\nbreak <tag> & \"quote\"   é")}
	}
}

func goldenRequest(r *rand.Rand) api.Request {
	var req api.Request
	if r.IntN(2) == 0 {
		req.System = goldenPick(r, "be brief", "you are a helper\nwith <two> lines")
	}
	if r.IntN(2) == 0 {
		req.Tools = []api.Tool{
			{Name: "get_weather", Desc: "weather by <city>", Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}}},
			{Name: "now", Desc: goldenPick(r, "", "the time")},
		}
		req.ToolChoice = goldenPick(r, api.ToolChoice(""), api.ToolChoiceAuto, api.ToolChoiceNone, api.ToolChoiceAny, api.ToolChoice("get_weather"))
	}
	turns := 1 + r.IntN(4)
	call := 0
	for range turns {
		var u api.Contents
		for range r.IntN(4) {
			u = append(u, goldenAttachment(r))
		}
		if r.IntN(10) == 0 {
			u = append(u, goldenPick[api.Content](r, nil, api.ContentReasoning{Meta: json.RawMessage(ownMeta)}, api.ContentToolUse{ID: "x", Name: "now"}))
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleUser, Content: u})
		if r.IntN(4) == 0 {
			continue
		}
		var a api.Contents
		var ids []string
		for range r.IntN(5) {
			switch r.IntN(9) {
			case 0:
				a = append(a, api.ContentText{Text: goldenPick(r, "the answer", "", "<b>bold</b> & co")})
			case 1:
				a = append(a, api.ContentReasoning{Model: goldenPick(r, testModel, "other"), Summary: map[int]string{0: "thinking"}, Meta: goldenPick(r,
					json.RawMessage(nil), json.RawMessage(ownMeta), json.RawMessage(`"claude-signature"`),
					json.RawMessage(`{"type":"redacted_thinking","data":"x"}`), json.RawMessage(`{"id":"rs_1","type":"reasoning","encrypted_content":"SEALED"}`),
					json.RawMessage(`{"type":"thought","signature":""}`))})
			case 2:
				a = append(a, api.ContentOpaque{Model: testModel, Kind: goldenPick(r, "gemini.google_search_call", "web_search_tool_result", "file_search_call", "gemini.code_execution_call"), Data: goldenPick(r,
					json.RawMessage(`{"arguments":{"queries":["cats"]},"id":"gs_1","signature":"c2ln","type":"google_search_call"}`),
					json.RawMessage(`{"id":"fs_1","type":"file_search_call"}`),
					json.RawMessage(`{"type":"code_execution_call","id":"ce_1","arguments":{"code":"print(1)"}}`))})
			case 3:
				a = append(a, goldenPick[api.Content](r, api.ContentImage{Data: goldenBytes(r, 8), Mime: "image/png"}, api.ContentDocument{Data: []byte("text"), Mime: "text/plain"}, nil, api.ContentToolResult{ID: "call_1"}))
			default:
				call++
				id := goldenPick(r, fmt.Sprintf("call_%d", call), "", fmt.Sprintf("toolu_%d", call))
				ids = append(ids, id)
				a = append(a, api.ContentToolUse{ID: id, Name: goldenPick(r, "get_weather", "now", ""), Input: goldenPick(r, json.RawMessage(nil), json.RawMessage(`{}`), json.RawMessage(`{"city":"Paris"}`), json.RawMessage(` {"city": "Rome"} `), json.RawMessage(`[1]`), json.RawMessage(`{"city":`), api.DiscardedArguments)})
			}
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleAssistant, Content: a})
		if len(ids) == 0 {
			continue
		}
		var results api.Contents
		if r.IntN(3) == 0 {
			results = append(results, api.ContentText{Text: "first"})
		}
		for _, id := range ids {
			var c api.UserContents
			for range r.IntN(3) {
				c = append(c, goldenAttachment(r))
			}
			results = append(results, api.ContentToolResult{ID: goldenPick(r, id, id, id, "stray"), Content: c, Error: goldenPick(r, api.ToolOk, api.ToolOk, api.ToolErrRuntime, api.ToolErrJSON)})
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
		{WithMaxTokens(goldenPick(r, 0, 1, 1000, 65_536, 65_537, -1))},
		{WithEffort(goldenPick(r, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, Effort("max"), Effort("")))},
		{WithThoughtSummaries(r.IntN(2) == 0)},
		{WithStopSequences(goldenPick(r, []string{"END"}, []string{"a", "\n\n"}, nil)...)},
		{WithRevision(goldenPick(r, "2026-05-20", "2026-07-01", ""))},
	}
	var opts []Option
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
		model := goldenPick(r, Hydrogen4, Neon4, Carbon4, Argon4, Flash38, FlashLite35, api.Model{Name: "gemini-9-unknown"})
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
		httpReq.Body.Close()
		fmt.Fprintf(&out, "%d %s %s\n", sent, httpReq.Header.Get("Api-Revision"), body)
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

func goldenStep(r *rand.Rand) (string, string) {
	switch n := r.IntN(40); {
	case n < 10:
		return goldenPick(r, `{"type":"thought"}`, `{"type":"thought","signature":"","summary":[{"type":"text","text":"Plan."}]}`, `{"type":"thought","signature":"c2ln"}`), "thought"
	case n < 20:
		return goldenPick(r, `{"type":"model_output"}`, `{"type":"model_output","content":[{"type":"text","text":"Hi <b>"}]}`), "model_output"
	case n < 30:
		return fmt.Sprintf(`{"id":%s,"type":"function_call","name":%s,"arguments":%s}`, goldenQuote(goldenPick(r, "call_1", "", "call_2")), goldenQuote(goldenPick(r, "get_weather", "")), goldenPick(r, `{}`, `{"city":"Paris"}`, `[]`, `null`)), "function_call"
	case n < 35:
		return goldenPick(r, `{"id":"gs_1","signature":"","type":"google_search_call","arguments":{}}`, `{"type":"google_search_result","call_id":"gs_1","result":[]}`, `{"type":"code_execution_call","id":"ce_1","arguments":{"code":"x"}}`), "opaque"
	case n < 38:
		return goldenPick(r, `{"type":"user_input","content":[]}`, `{"type":"function_result","call_id":"c","result":"x"}`), "ignored"
	default:
		return goldenPick(r, `{}`, `{"type":""}`, `[1]`, `"x"`, `{"type":7}`), ""
	}
}

func goldenDelta(r *rand.Rand, kind string) string {
	if r.IntN(10) == 0 {
		kind = goldenPick(r, "thought", "model_output", "function_call", "opaque", "other", "broken")
	}
	switch kind {
	case "thought":
		switch r.IntN(4) {
		case 0, 1:
			return fmt.Sprintf(`{"content":{"text":%s,"type":"text"},"type":"thought_summary"}`, goldenQuote(goldenPick(r, "", "**Plan**\n\nThink.\n\n\n", "more <&>")))
		case 2:
			return goldenPick(r, `{"content":{"type":"image","data":"AA=="},"type":"thought_summary"}`, `{"type":"thought_summary"}`)
		default:
			return fmt.Sprintf(`{"signature":%s,"type":"thought_signature"}`, goldenQuote(goldenPick(r, "", "EoMNCoANAWkU", "c2ln")))
		}
	case "model_output":
		return fmt.Sprintf(`{"text":%s,"type":"text"}`, goldenQuote(goldenPick(r, "", "a", "Hello", " wörld", "<b>&amp;</b>", "\n")))
	case "function_call":
		return fmt.Sprintf(`{"arguments":%s,"type":"arguments_delta"}`, goldenPick(r, `""`, `"{\"city\":"`, `"\"Paris\"}"`, `"{}"`, `"{\"x\":1}"`, `"[1]"`, `"{\"y\":"`, `{"a":1}`))
	case "opaque":
		return goldenPick(r, `{"type":"google_search_call","signature":"c2ln","arguments":{"queries":["x"]}}`, `{"type":"google_search_result","result":[{"url":"u"}]}`, `{"type":"code_execution_call","arguments":{"code":"print(1)"}}`)
	case "other":
		return goldenPick(r, `{"type":"text_annotation_delta","annotations":[]}`, `{"type":"image","data":"AA=="}`, `{"type":"audio"}`)
	}
	return goldenPick(r, `{}`, `{"type":""}`, `[1]`, `null`)
}

func goldenFrame(r *rand.Rand, open map[int]string, next *int) string {
	var indices []int
	for i := range *next {
		if _, ok := open[i]; ok {
			indices = append(indices, i)
		}
	}
	usage := goldenPick(r, ``, `,"usage":{"total_input_tokens":2000,"total_cached_tokens":1500,"total_output_tokens":180,"total_thought_tokens":1024,"total_tool_use_tokens":12}`)
	switch n := r.IntN(100); {
	case n < 1:
		return goldenPick(r, `{}`, `[1]`, `{"index":0,"delta":`, `[DONE]`, `{"delta":{"text":"x","type":"text"},"event_type":"step.delta"}`, `{"index":0}`, `{"event_type":""}`,
			`{"interaction":{"object":"interaction","model":"served"},"event_type":"interaction.created"}`)
	case n < 2:
		return fmt.Sprintf(`{"index":%d,%s"event_type":%q}`, r.IntN(6)+*next, goldenPick(r, `"delta":{"text":"x","type":"text"},`, ``), goldenPick(r, "step.delta", "step.stop"))
	case n < 6:
		return goldenPick(r, `{"event_type":"interaction.future","x":1}`, `{"status":"in_progress","event_type":"interaction.status_update"`+usage+`}`)
	case len(indices) == 0 || n < 30:
		idx := *next
		if len(indices) > 0 && r.IntN(30) == 0 {
			idx = indices[0]
		} else {
			*next = idx + 1
		}
		step, kind := goldenStep(r)
		open[idx] = kind
		return fmt.Sprintf(`{"index":%d,"step":%s,"event_type":"step.start"}`, idx, step)
	case n < 80:
		at := indices[r.IntN(len(indices))]
		return fmt.Sprintf(`{"index":%d,"delta":%s,"event_type":"step.delta"}`, at, goldenDelta(r, open[at]))
	default:
		at := indices[r.IntN(len(indices))]
		delete(open, at)
		return fmt.Sprintf(`{"index":%d,"event_type":"step.stop"%s}`, at, usage)
	}
}

func goldenTerminal(r *rand.Rand) string {
	usage := goldenPick(r, ``, `,"usage":{"total_input_tokens":20,"total_cached_tokens":5,"total_output_tokens":7,"total_thought_tokens":3}`, `,"usage":{"total_input_tokens":1,"total_cached_tokens":4,"total_output_tokens":-3}`)
	switch r.IntN(6) {
	case 0, 1, 2:
		return fmt.Sprintf(`{"interaction":{"status":%q%s},"event_type":"interaction.completed"}`, goldenPick(r, "completed", "requires_action", "incomplete", "requires_continuation", "failed", "cancelled", "", "new_status"), usage)
	case 3:
		return fmt.Sprintf(`{"error":{"code":%q,"message":%s},"event_type":"error"}`, goldenPick(r, "service_unavailable", "too_many_requests", "invalid_request", "safety", "gateway_timeout", "quota_exceeded", ""),
			goldenQuote(goldenPick(r, "bad", "The input token count (2) exceeds the maximum number of tokens allowed (1).", "limit: 20 requests per day", "Request payload size exceeds the limit: 1 bytes.")))
	case 4:
		return `[DONE]`
	default:
		return `{"interaction":{"status":"completed"`
	}
}

func goldenEnding(r *rand.Rand) error {
	return goldenPick[error](r,
		io.EOF, io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded, lib.ErrIdle, errors.New("boom"),
		&lib.HTTPError{Status: 429, Header: http.Header{"Retry-After": {"30"}}, Body: []byte(`{"error":{"message":"Rate limit exceeded (limit: 20 requests per day on Free Tier).","code":"too_many_requests"}}`)},
		&lib.HTTPError{Status: 400, Body: []byte("event: error\ndata: {\"error\":{\"message\":\"Request contains an invalid argument.\",\"code\":\"invalid_request\"},\"event_type\":\"error\"}\n\n")},
		&lib.HTTPError{Status: 400, Body: []byte(`[{"error":{"code":400,"message":"API key not valid.","status":"INVALID_ARGUMENT","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID"}]}}]`)},
		&lib.HTTPError{Status: 503, Header: http.Header{"Retry-After": {"Wed, 21 Oct 2015 07:28:00 GMT"}}, Body: []byte(`{"error":{"message":"high demand","code":"service_unavailable"}}`)},
		&lib.HTTPError{Status: 413, Body: []byte(`not json`)},
		&lib.HTTPError{Status: 500},
	)
}

func goldenStreamsText() string {
	r := rand.New(rand.NewPCG(3, 4))
	var out strings.Builder
	for i := range 3000 {
		c := newConverter(goldenPick(r, testModel, "gemini-3.8-flash"), "gemini_golden", 1234, goldenPick(r, 0, 999))
		var all []api.Event
		open := map[int]string{}
		next := 0
		ended := false
		frames := r.IntN(30)
		for j := range frames + 1 {
			var data string
			switch {
			case j == frames && r.IntN(5) == 0:
			case j == frames:
				data = goldenTerminal(r)
			case j == 0 && r.IntN(8) > 0:
				data = fmt.Sprintf(`{"interaction":{"object":"interaction","model":"served","id":%s},"event_type":"interaction.created"}`, goldenQuote(goldenPick(r, "", fmt.Sprintf("v1_%d", i))))
			default:
				data = goldenFrame(r, open, &next)
			}
			if data == "" {
				break
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
		[]byte(created),
		[]byte(statusUpdate),
		[]byte(`{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`),
	}
	for range 1000 {
		frames = append(frames, []byte(`{"index":0,"delta":{"text":" token","type":"text"},"event_type":"step.delta"}`))
	}
	frames = append(frames,
		[]byte(`{"index":0,"event_type":"step.stop"}`),
		[]byte(`{"interaction":{"status":"completed","usage":{"total_input_tokens":20,"total_output_tokens":7}},"event_type":"interaction.completed"}`),
	)
	b.ReportAllocs()
	for b.Loop() {
		c := newConverter(testModel, "gemini_bench", 0, 0)
		n := 0
		for _, f := range frames {
			events, _ := c.Convert(lib.Frame{Data: f})
			n += len(events)
		}
		if n != 1004 {
			b.Fatalf("%d events", n)
		}
	}
}
