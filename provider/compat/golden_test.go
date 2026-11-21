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

package compat

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
	goldenRequestsHash = "17c4d04d4c316f70efe68156f0fb17b985fe6665d58e7ef479759d211380d43f"
	goldenStreamsHash  = "f89d5b0f03e4886a043b5cfe4a4971698b77e85c38396393a1278748f857c9e6"
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
		data := goldenPick(r, goldenBytes(r, r.IntN(300)), []byte("a,b\n1,2\n"), nil)
		return api.ContentDocument{Data: data, Mime: string(goldenPick(r, api.DocumentPDF, api.DocumentText, api.DocumentCSV, api.DocumentMarkdown)), Name: goldenPick(r, "", "report")}
	default:
		return api.ContentText{Text: goldenPick(r, "hi", "", "line\nbreak <tag> & \"quote\"   é")}
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
			switch r.IntN(7) {
			case 0:
				a = append(a, api.ContentText{Text: goldenPick(r, "the answer", "")})
			case 1:
				meta := goldenPick(r, `{"provider":"compat","data":"reasoning"}`, `{"provider":"compat","data":"reasoning_content"}`, `{"provider":"deepseek"}`, `"c2ln"`, ``)
				a = append(a, api.ContentReasoning{Model: "m", Thoughts: map[int]string{0: "first", 1: " then"}, Meta: json.RawMessage(meta)})
			case 2:
				a = append(a, api.ContentOpaque{Model: "x", Kind: "web_search_call", Data: json.RawMessage(`{"x":1}`)})
			case 3:
				a = append(a, api.ContentDocument{Data: []byte("# notes"), Mime: goldenPick(r, "text/markdown", "application/pdf")})
			default:
				call++
				id := goldenPick(r, fmt.Sprintf("call_%d", call), "")
				ids = append(ids, id)
				a = append(a, api.ContentToolUse{ID: id, Name: goldenPick(r, "get_weather", "now", ""), Input: goldenPick(r, json.RawMessage(nil), json.RawMessage(`{}`), json.RawMessage(`{"city":"Paris"}`), json.RawMessage(`{"city":`))})
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
			results = append(results, api.ContentToolResult{ID: goldenPick(r, id, id, id, "call_other"), Content: c, Error: goldenPick(r, api.ToolOk, api.ToolOk, api.ToolErrRuntime, api.ToolErrJSON)})
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
		opts = append(opts, WithEffort(goldenPick(r, EffortNone, EffortLow, EffortHigh, EffortMax, "custom")))
	}
	if r.IntN(6) == 0 {
		opts = append(opts, WithTemperature(0.6))
	}
	if r.IntN(6) == 0 {
		opts = append(opts, WithTopP(0.95))
	}
	if r.IntN(3) == 0 {
		opts = append(opts, WithStopSequences("END", "\n\n"))
	}
	if r.IntN(3) == 0 {
		opts = append(opts, WithMaxTokens(4096))
	}
	return opts
}

func goldenRequestsText(t testing.TB) string {
	r := rand.New(rand.NewPCG(1, 2))
	var out strings.Builder
	for i := range 1500 {
		model := goldenPick(r, api.Model{Name: "qwen/qwen3.8-27b"}, api.Model{Name: "local"}, api.Model{Name: "gpt-oss-20b", RequestBytes: 4000})
		opts := append([]Option{WithAPIKey(goldenPick(r, "k", ""))}, goldenOptions(r)...)
		if i%4 == 0 {
			opts = append(opts, WithExtraBody(map[string]any{
				"provider":             map[string]any{"order": []string{"wafer"}, "allow_fallbacks": false},
				"cache_prompt":         true,
				"chat_template_kwargs": map[string]any{"enable_thinking": false},
			}))
		}
		req := goldenRequest(r)
		fmt.Fprintf(&out, "== %d %s\n", i, model.Name)
		p, err := New(Conf{Model: model, BaseURL: "http://golden.invalid/v1"}, opts...)
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
		fmt.Fprintf(&out, "%d %q %s\n", sent, httpReq.Header.Get("Authorization"), body)
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
	case api.EventMeta:
		return fmt.Sprintf("meta %d %s", e.Index, e.Data)
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
		return fmt.Sprintf("? %T %+v", e, e)
	}
}

func goldenQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func goldenText(r *rand.Rand) string {
	return goldenPick(r, `null`, `""`, goldenQuote(goldenPick(r, "a", "Hello", " wörld", `"q"`, `{"k":`, "1}", "\n\n")))
}

func goldenToolCall(r *rand.Rand) string {
	fn := goldenPick(r, ``, `,"function":null`, `,"function":{"name":"get_weather","arguments":""}`, `,"function":{"arguments":"{\"ci"}`, `,"function":{"name":"","arguments":"ty\":1}"}`, `,"function":{"name":"now"}`)
	id := goldenPick(r, ``, ``, ``, `,"id":""`, `,"id":"call_a"`, `,"id":"call_b"`, `,"id":"chatcmpl-tool-9bc5"`)
	index := goldenPick(r, `"index":0,`, `"index":1,`, `"index":2,`, `"index":0,`, `"index":1,`, `"index":2,`, ``)
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
	reasoning := goldenText(r)
	details := goldenPick(r, ``, `,"reasoning_details":[{"type":"reasoning.text","text":`+reasoning+`,"format":"unknown","index":0}]`)
	return fmt.Sprintf(`{"role":"assistant","content":%s,%s:%s%s,"tool_calls":%s}`, goldenText(r), goldenPick(r, `"reasoning"`, `"reasoning_content"`), reasoning, details, tc)
}

func goldenFinish(r *rand.Rand) string {
	if r.IntN(4) != 0 {
		return "null"
	}
	return goldenQuote(goldenPick(r, "stop", "tool_calls", "length", "content_filter", "function_call", "error", "abort", ""))
}

func goldenUsage(r *rand.Rand) string {
	return goldenPick(r,
		`{"prompt_tokens":440,"completion_tokens":60,"total_tokens":500,"cost":0.0001,"is_byok":false,"prompt_tokens_details":{"cached_tokens":64,"cache_write_tokens":0,"audio_tokens":0,"video_tokens":0},"completion_tokens_details":{"reasoning_tokens":15,"image_tokens":0,"audio_tokens":0}}`,
		`{"prompt_tokens":100,"completion_tokens":5}`,
		`{"prompt_tokens":60,"completion_tokens":24,"completion_tokens_details":{"reasoning_tokens":26}}`,
		`{"prompt_tokens":1000,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":600,"cache_write_tokens":300}}`,
		`null`,
	)
}

func goldenFrame(r *rand.Rand, i int) string {
	switch r.IntN(30) {
	case 0:
		return fmt.Sprintf(`{"id":"gen-%d","object":"chat.completion.chunk","model":"m","choices":[],"usage":%s}`, i, goldenUsage(r))
	case 1:
		return fmt.Sprintf(`{"id":"gen-1","choices":[{"index":0,"delta":%s,"finish_reason":%s},{"index":1,"delta":%s,"finish_reason":%s}]}`, goldenDelta(r), goldenFinish(r), goldenDelta(r), goldenFinish(r))
	case 2:
		return fmt.Sprintf(`{"id":"","choices":[{"index":0,"delta":%s,"finish_reason":%s,"native_finish_reason":%s}],"usage":%s}`, goldenDelta(r), goldenFinish(r), goldenFinish(r), goldenUsage(r))
	case 3:
		return goldenPick(r,
			`{"id":"gen-1","error":{"code":502,"message":"Provider disconnected","metadata":{"error_type":"provider_unavailable"}},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`,
			`{"id":"gen-1","error":{"code":403,"message":"declined","metadata":{"error_type":"refusal"}},"choices":[]}`,
			`{"error":{"code":500,"message":"decode failed","type":"server_error"}}`,
			`{"error":{"message":"too long","type":"invalid_request_error","code":"context_length_exceeded"}}`,
			`{"error":"plain string"}`,
			`{"error":null,"choices":[]}`,
			`{"choices":[{"delta":{"content":"partial"},"finish_reason":null}],"error":{"message":"cut","code":429}}`,
		)
	case 4:
		return goldenPick(r, `{"object":"something.else"}`, `{}`, `{"choices":null}`)
	default:
		return fmt.Sprintf(`{"id":"gen-1","object":"chat.completion.chunk","created":1795183087,"model":"qwen/qwen3.8-27b","provider":"Wafer","choices":[{"index":0,"delta":%s,"finish_reason":%s,"native_finish_reason":null}]}`, goldenDelta(r), goldenFinish(r))
	}
}

func goldenEnding(r *rand.Rand) error {
	return goldenPick[error](r,
		io.EOF, io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded, lib.ErrIdle, errors.New("boom"),
		&lib.HTTPError{Status: 429, Header: http.Header{"Retry-After": {"3"}, "X-Generation-Id": {"gen-9"}}, Body: []byte(`{"error":{"message":"Rate limit exceeded","code":429,"metadata":{"error_type":"rate_limit_exceeded"}}}`)},
		&lib.HTTPError{Status: 400, Body: []byte(`{"error":{"message":"This endpoint's maximum context length is 131072 tokens.","code":400,"metadata":{"provider_name":null}}}`)},
		&lib.HTTPError{Status: 400, Body: []byte(`{"error":{"code":400,"message":"the request exceeds the available context size, try increasing it","type":"exceed_context_size_error"}}`)},
		&lib.HTTPError{Status: 400, Header: http.Header{"X-Request-Id": {"req_1"}}, Body: []byte(`{"object":"error","message":"bad","type":"BadRequestError","param":null,"code":400}`)},
		&lib.HTTPError{Status: 413, Body: []byte(`not json`)},
		&lib.HTTPError{Status: 401, Body: []byte(`{"error":{"message":"User not found.","code":401}}`)},
		&lib.HTTPError{Status: 503, Body: []byte(`{"error":{"code":503,"message":"Loading model","type":"unavailable_error"}}`)},
		&lib.HTTPError{Status: 501},
	)
}

func goldenStreamsText() string {
	r := rand.New(rand.NewPCG(3, 4))
	var out strings.Builder
	for i := range 3000 {
		c := newConverter(goldenPick(r, "qwen/qwen3.8-27b", "local"), 1234, goldenPick(r, 0, 999))
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

func BenchmarkEncode(b *testing.B) {
	p, err := New(Conf{Model: api.Model{Name: testModel}, BaseURL: "http://bench.invalid/v1"})
	if err != nil {
		b.Fatal(err)
	}
	req := goldenRequest(rand.New(rand.NewPCG(5, 6)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.encode(req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConvertTextTurn(b *testing.B) {
	var frames [][]byte
	for range 1000 {
		frames = append(frames, []byte(`{"id":"gen-1","object":"chat.completion.chunk","created":1795183087,"model":"qwen/qwen3.8-27b","provider":"Wafer","choices":[{"index":0,"delta":{"content":" token","role":"assistant"},"finish_reason":null,"native_finish_reason":null}]}`))
	}
	frames = append(frames,
		[]byte(`{"id":"gen-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"now","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`),
		[]byte(`{"id":"gen-1","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":5}}`),
		[]byte(doneSentinel),
	)
	b.ReportAllocs()
	for b.Loop() {
		c := newConverter(testModel, 0, 0)
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

func BenchmarkImageURL(b *testing.B) {
	img := api.ContentImage{Data: make([]byte, 1<<20), Mime: "image/png"}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := image(img).(imagePart); !ok {
			b.Fatal("not an image part")
		}
	}
}
