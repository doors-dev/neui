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
	goldenRequestsHash = "78834b18c1e17875da619d35c0790f9b2ea86bbefdfc65c8c092029e33612ea1"
	goldenStreamsHash  = "4fa0c7c9a5f8a000a9167fd12e2df978b01fb7bb4a974f832b6a7eda8c5225d9"
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
	case 0, 1:
		return api.ContentImage{Data: goldenBytes(r, r.IntN(3000)), Mime: string(goldenPick(r, api.ImagePNG, api.ImageJPEG, api.ImageWEBP, api.ImageGIF))}
	case 2:
		return api.ContentImage{Data: goldenBytes(r, goldenPick(r, 0, 16)), Mime: goldenPick(r, "image/svg+xml", "", "image/png")}
	case 3:
		return api.ContentDocument{Data: goldenBytes(r, r.IntN(300)), Mime: string(goldenPick(r, api.DocumentPDF, api.DocumentText, api.DocumentCSV)), Name: goldenPick(r, "", "report")}
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
	call := 0
	for range 1 + r.IntN(4) {
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
			switch r.IntN(6) {
			case 0:
				a = append(a, api.ContentText{Text: goldenPick(r, "the answer", "")})
			case 1:
				a = append(a, api.ContentReasoning{Model: goldenPick(r, "deepseek-v4-pro", "deepseek-flash", "other"), Thoughts: map[int]string{0: "first", 1: " then"}, Summary: goldenPick(r, api.Indexed[string](nil), map[int]string{0: "s"})})
			case 2:
				a = append(a, api.ContentOpaque{Model: "x", Kind: "web_search_call", Data: json.RawMessage(`{"x":1}`)})
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
	var opts []Option
	if r.IntN(2) == 0 {
		opts = append(opts, WithEffort(goldenPick(r, EffortNone, EffortLow, EffortHigh, EffortMax)))
	}
	if r.IntN(6) == 0 {
		opts = append(opts, WithTemperature(0.5))
	}
	if r.IntN(6) == 0 {
		opts = append(opts, WithTopP(0.9))
	}
	if r.IntN(3) == 0 {
		opts = append(opts, WithStop("END", "\n\n"))
	}
	if r.IntN(3) == 0 {
		opts = append(opts, WithMaxTokens(4096))
	}
	if r.IntN(3) == 0 {
		opts = append(opts, WithUserID("user-1"))
	}
	return opts
}

func goldenRequestsText(t testing.TB) string {
	r := rand.New(rand.NewPCG(1, 2))
	var out strings.Builder
	for i := range 1500 {
		model := goldenPick(r, Flash, Flash, V4Pro, api.Model{Name: "deepseek-chat"})
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
	case api.EventUsage:
		return fmt.Sprintf("usage %+v", e)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		var a *APIError
		var s *StreamError
		var f *FinishError
		switch {
		case errors.As(e.Err, &a):
			return fmt.Sprintf("err %s %T %+v %q", e.Code, e.Err, *a, e.Err.Error())
		case errors.As(e.Err, &s):
			return fmt.Sprintf("err %s %T %+v %q", e.Code, e.Err, *s, e.Err.Error())
		case errors.As(e.Err, &f):
			return fmt.Sprintf("err %s %T %+v %q", e.Code, e.Err, *f, e.Err.Error())
		}
		return fmt.Sprintf("err %s %T %q", e.Code, e.Err, e.Err.Error())
	default:
		return fmt.Sprintf("? %T %+v", e, e)
	}
}

func goldenQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func goldenText(r *rand.Rand) string {
	return goldenPick(r, `null`, `""`, goldenQuote(goldenPick(r, "a", "Hello", " wörld", `"q"`, `{"k":`, "1}", "\n")))
}

func goldenToolCall(r *rand.Rand) string {
	fn := goldenPick(r, ``, `,"function":null`, `,"function":{"name":"get_weather","arguments":""}`, `,"function":{"arguments":"{\"ci"}`, `,"function":{"name":"","arguments":"ty\":1}"}`, `,"function":{"name":"now"}`)
	id := goldenPick(r, ``, `,"id":""`, `,"id":"call_a"`, `,"id":"call_b"`)
	index := goldenPick(r, `"index":0,`, `"index":1,`, `"index":2,`)
	return fmt.Sprintf(`{%s"type":"function"%s%s}`, index, id, fn)
}

func goldenDelta(r *rand.Rand) string {
	var tools []string
	for range goldenPick(r, 0, 0, 0, 1, 2) {
		tools = append(tools, goldenToolCall(r))
	}
	tc := goldenPick(r, `null`, `[]`)
	if len(tools) > 0 {
		tc = "[" + strings.Join(tools, ",") + "]"
	}
	return fmt.Sprintf(`{"role":"assistant","content":%s,"reasoning_content":%s,"tool_calls":%s}`, goldenText(r), goldenText(r), tc)
}

func goldenFinish(r *rand.Rand) string {
	if r.IntN(4) != 0 {
		return "null"
	}
	return goldenQuote(goldenPick(r, "stop", "tool_calls", "length", "content_filter", "insufficient_system_resource", "aborted", "weird", ""))
}

func goldenUsage(r *rand.Rand) string {
	return goldenPick(r,
		`{"prompt_tokens":12000,"completion_tokens":950,"total_tokens":12950,"prompt_cache_hit_tokens":9000,"prompt_cache_miss_tokens":3000,"completion_tokens_details":{"reasoning_tokens":600}}`,
		`{"prompt_tokens":100,"completion_tokens":5}`,
		`{"prompt_tokens":100,"completion_tokens":5,"prompt_cache_hit_tokens":70,"prompt_cache_miss_tokens":10,"completion_tokens_details":{"reasoning_tokens":9}}`,
	)
}

func goldenFrame(r *rand.Rand, i int) string {
	switch r.IntN(30) {
	case 0:
		return fmt.Sprintf(`{"id":"chatcmpl-%d","object":"chat.completion.chunk","model":"deepseek-v4-pro","choices":[],"usage":%s}`, i, goldenUsage(r))
	case 1:
		return fmt.Sprintf(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":%s,"finish_reason":%s},{"index":1,"delta":%s,"finish_reason":%s}],"usage":null}`, goldenDelta(r), goldenFinish(r), goldenDelta(r), goldenFinish(r))
	case 2:
		return fmt.Sprintf(`{"id":"","choices":[{"index":0,"delta":%s,"finish_reason":%s}],"usage":%s}`, goldenDelta(r), goldenFinish(r), goldenUsage(r))
	case 3:
		return goldenPick(r,
			`{"error":{"message":"boom","type":"server_error","code":500}}`,
			`{"error":{"message":"too long","type":"invalid_request_error","code":"context_length_exceeded"}}`,
			`{"error":"plain string"}`,
			`{"error":null,"choices":[]}`,
			`{"error_type":"rate_limit","error_message":"slow"}`,
			`{"choices":[{"delta":{"content":"partial"},"finish_reason":null}],"error":{"message":"cut","code":"overloaded"}}`,
		)
	case 4:
		return goldenPick(r, `{"object":"something.else"}`, `{}`, `{"choices":null}`)
	default:
		return fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,"model":"deepseek-v4-pro","system_fingerprint":"fp","choices":[{"index":0,"delta":%s,"logprobs":null,"finish_reason":%s}],"usage":null}`, goldenDelta(r), goldenFinish(r))
	}
}

func goldenEnding(r *rand.Rand) error {
	return goldenPick[error](r,
		io.EOF, io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded, lib.ErrIdle, errors.New("boom"),
		&lib.HTTPError{Status: 429, Header: http.Header{"Retry-After": {"3"}}, Body: []byte(`{"error":{"message":"slow down","type":"rate_limit","code":"429"}}`)},
		&lib.HTTPError{Status: 400, Body: []byte(`{"error":{"message":"This model's maximum context length is 1048576 tokens.","type":"invalid_request_error","code":"invalid_request_error"}}`)},
		&lib.HTTPError{Status: 413, Body: []byte(`not json`)},
		&lib.HTTPError{Status: 402, Body: []byte(`{"error":{"message":"Insufficient Balance","type":"unknown_error","code":"invalid_request_error"}}`)},
		&lib.HTTPError{Status: 503},
	)
}

func goldenStreamsText() string {
	r := rand.New(rand.NewPCG(3, 4))
	var out strings.Builder
	for i := range 3000 {
		c := newConverter(goldenPick(r, "deepseek-v4-pro", "deepseek-flash"), 1234, goldenPick(r, 0, 999))
		var all []api.Event
		ended := false
		frames := r.IntN(30)
		for j := range frames + 1 {
			data := goldenFrame(r, j)
			if j == frames {
				if r.IntN(4) == 0 {
					break
				}
				data = goldenPick(r, doneSentinel, doneSentinel, `{"choices":[`, `[1]`)
			}
			events, more := c.Convert(lib.Frame{Data: []byte(data)})
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

// The hashes pin the renderings of both corpora, the stream one as of strict
// decoding and the request one as of forced choices going unchecked to models
// not declared here: a request body that changes by one byte, or an event
// stream that changes in any field, fails here.
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
	var frames [][]byte
	for range 1000 {
		frames = append(frames, []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,"model":"deepseek-v4-pro","system_fingerprint":"fp","choices":[{"index":0,"delta":{"content":" token","reasoning_content":null},"logprobs":null,"finish_reason":null}],"usage":null}`))
	}
	frames = append(frames,
		[]byte(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"now","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":null}`),
		[]byte(`{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":5}}`),
		[]byte(doneSentinel),
	)
	b.ReportAllocs()
	for b.Loop() {
		c := newConverter("deepseek-v4-pro", 0, 0)
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
