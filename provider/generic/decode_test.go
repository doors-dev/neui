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

package generic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/internal/thread/response"
	"neui.dev/provider/lib"
)

func TestStreamText(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"","role":"assistant","reasoning":null}`, ""),
		chunkFrame(`{"content":"9.11","role":"assistant"}`, ""),
		chunkFrame(`{"content":" is larger.","role":"assistant"}`, ""),
		chunkFrame(`{"content":"","role":"assistant"}`, "stop"),
		usageFrame,
		doneFrame,
	))
	events := collect(t, p, turn("9.11 or 9.8?"))
	conform.CheckUnframed(t, events)
	wantEvents(t, events,
		"start gen-1 qwen/qwen3.8-27b",
		`text 0 "9.11"`,
		`text 0 " is larger."`,
		"usage in=376 out=45 rsn=15 cr=64 cw=0",
		"ok turn",
	)
}

func TestStreamReasoningFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		delta  string
		meta   string
		fields string
	}{
		{"reasoning", `{"content":"","reasoning":"%s","reasoning_details":[{"type":"reasoning.text","text":"%s","format":"unknown","index":0}]}`, `{"provider":"generic","data":"reasoning"}`,
			`opaque 2 "generic.fields" {"fields":{"reasoning_details":[{"format":"unknown","index":0,"text":"Let me think. More.","type":"reasoning.text"}]}} ""`},
		{"reasoning_content", `{"content":null,"reasoning_content":"%s"}`, `{"provider":"generic","data":"reasoning_content"}`, ""},
		{"both, the same text", `{"reasoning":"%s","reasoning_content":"%s"}`, `{"provider":"generic","data":"reasoning"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, stringHandler(
				chunkFrame(openingFrame, ""),
				chunkFrame(strings.ReplaceAll(tc.delta, "%s", "Let me"), ""),
				chunkFrame(strings.ReplaceAll(tc.delta, "%s", " think."), ""),
				chunkFrame(`{"content":"\n\nHi"}`, ""),
				chunkFrame(strings.ReplaceAll(tc.delta, "%s", " More."), ""),
				chunkFrame(`{}`, "stop"),
				doneFrame,
			))
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			want := []string{
				"start gen-1 qwen/qwen3.8-27b",
				`thinking 0 part=0 "Let me"`,
				"meta 0 " + tc.meta,
				`thinking 0 part=0 " think."`,
				`text 1 "\n\nHi"`,
				`thinking 0 part=0 " More."`,
			}
			if tc.fields != "" {
				want = append(want, tc.fields)
			}
			wantEvents(t, events, append(want, "ok turn")...)
		})
	}
}

func TestStreamToolCalls(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"reasoning":"Two calls."}`, ""),
		chunkFrame(`{"content":"\n\n\n"}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":""}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\": "}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":1,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{\"city\": \"Rome\"}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris\"}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":2,"type":"function","function":{"name":"now","arguments":"{}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":2,"id":"call_c"}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":2}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("weather"))
	conform.CheckUnframed(t, events)
	wantEvents(t, events,
		"start gen-1 qwen/qwen3.8-27b",
		`thinking 0 part=0 "Two calls."`,
		`meta 0 {"provider":"generic","data":"reasoning"}`,
		`text 1 "\n\n\n"`,
		`tool 2 "call_a" "get_weather" ""`,
		`tool 2 "" "" "{\"city\": "`,
		`tool 3 "gen-1_call_3" "get_weather" "{\"city\": \"Rome\"}"`,
		`tool 2 "" "" "\"Paris\"}"`,
		`tool 4 "gen-1_call_4" "now" "{}"`,
		`tool 4 "call_c" "" ""`,
		"ok turn",
	)
}

func TestStreamToolCallPieces(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		want   []string
	}{
		{"a fragment repeating its own id", []string{
			`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"ping","arguments":"{"}}]}`,
			`{"tool_calls":[{"index":0,"id":"call_a","function":{"arguments":"}"}}]}`,
		}, []string{`tool 0 "call_a" "ping" "{"`, `tool 0 "" "" "}"`}},
		{"no argument fragment at all", []string{
			`{"tool_calls":[{"index":0,"id":"call_a","type":"function"}]}`,
			`{"tool_calls":[{"index":0,"function":{"name":"ping"}}]}`,
		}, []string{`tool 0 "call_a" "" ""`, `tool 0 "" "ping" ""`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames []string
			for _, f := range tc.frames {
				frames = append(frames, chunkFrame(f, ""))
			}
			events := collect(t, serve(t, stringHandler(append(frames, chunkFrame(`{}`, "tool_calls"), doneFrame)...)), turn("ping"))
			conform.CheckUnframed(t, events)
			wantEvents(t, events, append(append([]string{"start gen-1 qwen/qwen3.8-27b"}, tc.want...), "ok turn")...)
		})
	}
}

func TestStreamWholeToolCalls(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(`{"role":"assistant","tool_calls":[{"extra_content":{"google":{"thought_signature":"c2ln"}},"function":{"arguments":"{\"item\":\"alpha\"}","name":"lookup"},"id":"call_1","type":"function"}]}`, ""),
		chunkFrame(`{"role":"assistant","tool_calls":[{"function":{"arguments":"{\"item\":\"beta\"}","name":"lookup"},"id":"call_2","type":"function"},{"function":{"arguments":"{}","name":"now"},"type":"function"}]}`, ""),
		chunkFrame(`{"role":"assistant"}`, "stop"),
		doneFrame,
	))
	events := collect(t, p, turn("look up"))
	conform.CheckUnframed(t, events)
	wantEvents(t, events,
		"start gen-1 qwen/qwen3.8-27b",
		`tool 0 "call_1" "lookup" "{\"item\":\"alpha\"}"`,
		`tool 1 "call_2" "lookup" "{\"item\":\"beta\"}"`,
		`tool 2 "gen-1_call_2" "now" "{}"`,
		`opaque 3 "generic.fields" {"calls":{"call_1":{"extra_content":{"google":{"thought_signature":"c2ln"}}}}} ""`,
		"ok turn",
	)
}

func TestStreamToolCallFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		want   []string
	}{
		{"a call that changes its id", []string{
			`{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"now","arguments":""}}]}`,
			`{"tool_calls":[{"index":0,"id":"call_b"}]}`,
		}, []string{`tool 0 "call_a" "now" ""`}},
		{"a late id that changes", []string{
			`{"tool_calls":[{"index":0,"function":{"arguments":"{"}}]}`,
			`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"ping"}}]}`,
			`{"tool_calls":[{"index":0,"id":"call_b","function":{"arguments":"}"}}]}`,
		}, []string{`tool 0 "gen-1_call_0" "" "{"`, `tool 0 "call_a" "ping" ""`}},
		{"a duplicate id that changes", []string{
			`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"ping","arguments":"{}"}}]}`,
			`{"tool_calls":[{"index":1,"id":"call_a","type":"function","function":{"name":"ping","arguments":"{"}}]}`,
			`{"tool_calls":[{"index":1,"id":"call_b","function":{"arguments":"}"}}]}`,
		}, []string{`tool 0 "call_a" "ping" "{}"`, `tool 1 "gen-1_call_1" "ping" "{"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames []string
			for _, f := range tc.frames {
				frames = append(frames, chunkFrame(f, ""))
			}
			events := collect(t, serve(t, stringHandler(append(frames, chunkFrame(`{}`, "tool_calls"), doneFrame)...)), turn("ping"))
			conform.CheckUnframed(t, events)
			wantEvents(t, events, append(append([]string{"start gen-1 qwen/qwen3.8-27b"}, tc.want...), "err network")...)
		})
	}
}

func TestStreamOpaqueFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		finish string
		want   string
	}{
		{"reasoning_details merge by index", []string{
			`{"reasoning_details":[{"type":"reasoning.text","text":"\n","format":"f","index":0}]}`,
			`{"reasoning_details":[{"type":"reasoning.text","text":"\n","format":"f","index":0},{"type":"reasoning.encrypted","data":"ab","index":1}]}`,
			`{"reasoning_details":[{"type":"reasoning.encrypted","data":"cd","index":1,"id":"rs_1"}]}`,
			`{"reasoning_details":[{"type":"reasoning.text","signature":"c2","format":"f","index":0}]}`,
			`{"reasoning_details":[{"type":"reasoning.text","signature":"ln","format":"g","index":0,"id":null}]}`,
			`{"reasoning_details":[{"type":"reasoning.summary","summary":"**Plan**\n","format":"o","index":2},{"type":"reasoning.summary","summary":"one","format":"o","index":2,"extra":1}]}`,
			`{"reasoning_details":[{"type":"reasoning.summary","summary":" step","format":"o","index":2,"extra":2}]}`,
		}, "stop", `{"fields":{"reasoning_details":[{"format":"g","index":0,"signature":"c2ln","text":"\n\n","type":"reasoning.text"},{"data":"abcd","id":"rs_1","index":1,"type":"reasoning.encrypted"},{"extra":2,"format":"o","index":2,"summary":"**Plan**\none step","type":"reasoning.summary"}]}}`},
		{"reasoning_details entries without an index stay apart", []string{
			`{"reasoning_details":[{"type":"reasoning.text","text":"a"}]}`,
			`{"reasoning_details":[{"type":"reasoning.text","text":"b"}]}`,
		}, "stop", `{"fields":{"reasoning_details":[{"text":"a","type":"reasoning.text"},{"text":"b","type":"reasoning.text"}]}}`},
		{"reasoning_details that is no list stays whole", []string{
			`{"reasoning_details":{"type":"reasoning.text","text":"a"}}`,
		}, "stop", `{"fields":{"reasoning_details":{"type":"reasoning.text","text":"a"}}}`},
		{"a string that arrives again replaces", []string{
			`{"note":"same"}`,
			`{"note":"same"}`,
			`{"tag":"a"}`,
			`{"tag":"b"}`,
		}, "stop", `{"fields":{"note":"same","tag":"b"}}`},
		{"an object that arrives again replaces whole", []string{
			`{"meta":{"text":"a","n":1,"deep":{"s":"x"}}}`,
			`{"meta":{"text":"b","n":2,"flag":true,"deep":{"s":"y"}}}`,
		}, "stop", `{"fields":{"meta":{"text":"b","n":2,"flag":true,"deep":{"s":"y"}}}}`},
		{"other values keep the last", []string{
			`{"annotations":[{"type":"url_citation","url":"a","index":0}],"score":1,"label":"x"}`,
			`{"annotations":[{"type":"url_citation","url":"b","index":0}],"score":2.5,"label":{"k":1}}`,
		}, "stop", `{"fields":{"annotations":[{"type":"url_citation","url":"b","index":0}],"label":{"k":1},"score":2.5}}`},
		{"empty values are absent", []string{
			`{"refusal":null,"annotations":[],"audio":{},"note":""}`,
			`{"content":"hi","role":"assistant"}`,
		}, "stop", ""},
		{"per-call fields", []string{
			`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"now","arguments":""},"extra_content":{"google":{"thought_signature":"c2"}}}]}`,
			`{"tool_calls":[{"index":0,"function":{"arguments":"{}"},"extra_content":{"google":{"thought_signature":"ln"}}},{"index":1,"type":"function","function":{"name":"now","arguments":"{}"},"sig":"x"}]}`,
			`{"tool_calls":[{"index":1,"id":"call_b","sig":"y"}]}`,
		}, "tool_calls", `{"calls":{"call_a":{"extra_content":{"google":{"thought_signature":"ln"}}},"call_b":{"sig":"y"}}}`},
		{"per-call fields of a minted id", []string{
			`{"tool_calls":[{"index":0,"type":"function","function":{"name":"now","arguments":"{}"},"sig":"x"}]}`,
		}, "tool_calls", `{"calls":{"gen-1_call_0":{"sig":"x"}}}`},
		{"an unfinished turn keeps none", []string{
			`{"reasoning_details":[{"type":"reasoning.text","text":"a","index":0}]}`,
		}, "length", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames []string
			for _, f := range tc.frames {
				frames = append(frames, chunkFrame(f, ""))
			}
			events := collect(t, serve(t, stringHandler(append(frames, chunkFrame(`{}`, tc.finish), doneFrame)...)), turn("hi"))
			conform.CheckUnframed(t, events)
			var got string
			next := 0
			for i, e := range events {
				switch e := e.(type) {
				case api.EventOpaque:
					if e.Kind != "generic.fields" || i != len(events)-2 || e.Index != next {
						t.Fatalf("opaque %s is not the last block before the end: %q", conform.Describe(e), conform.DescribeAll(events))
					}
					got = string(e.Data)
				case api.IndexedEvent:
					next = max(next, e.At()+1)
				}
			}
			if got != tc.want {
				t.Fatalf("fields\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestStreamReadsOnlyChoiceZero(t *testing.T) {
	p := serve(t, stringHandler(
		frame(`{"id":"gen-1","choices":[{"index":1,"delta":{"content":"other"},"finish_reason":"length"},{"index":0,"delta":{"content":"mine"},"finish_reason":null}]}`),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	wantEvents(t, events, "start gen-1 qwen/qwen3.8-27b", `text 0 "mine"`, "ok turn")
}

func TestUsageArithmetic(t *testing.T) {
	for _, tc := range []struct {
		usage string
		want  string
	}{
		{`{"prompt_tokens":440,"completion_tokens":60,"prompt_tokens_details":{"cached_tokens":64,"cache_write_tokens":0},"completion_tokens_details":{"reasoning_tokens":15}}`, "usage in=376 out=45 rsn=15 cr=64 cw=0"},
		{`{"prompt_tokens":1000,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":600,"cache_write_tokens":300}}`, "usage in=100 out=10 rsn=0 cr=600 cw=300"},
		{`{"prompt_tokens":60,"completion_tokens":24,"completion_tokens_details":{"reasoning_tokens":26}}`, "usage in=60 out=0 rsn=24 cr=0 cw=0"},
		{`{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25}`, "usage in=20 out=5 rsn=0 cr=0 cw=0"},
		{`{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":30},"completion_tokens_details":{"reasoning_tokens":-2}}`, "usage in=0 out=5 rsn=0 cr=30 cw=0"},
	} {
		c := newConverter(testModel, 0, 0)
		events, more := c.Convert(lib.Frame{Data: []byte(`{"id":"x","choices":[],"usage":` + tc.usage + `}`)})
		if !more || len(events) != 2 || conform.Describe(events[1]) != tc.want {
			t.Errorf("%s: %q, want %q", tc.usage, conform.DescribeAll(events), tc.want)
		}
	}
}

func TestFinishReasons(t *testing.T) {
	for finish, want := range map[string]string{
		"stop":           "ok turn",
		"tool_calls":     "ok turn",
		"function_call":  "ok turn",
		"length":         "err capped generic: length: the turn ended on this finish_reason (request gen-1)",
		"content_filter": "err refused generic: content_filter: the turn ended on this finish_reason (request gen-1)",
		"abort":          "err network generic: abort: the turn ended on this finish_reason (request gen-1)",
		"error":          "err network generic: error: the turn ended on this finish_reason (request gen-1)",
		"repetition":     "err unknown generic: repetition: the turn ended on this finish_reason (request gen-1)",
	} {
		t.Run(finish, func(t *testing.T) {
			p := serve(t, stringHandler(chunkFrame(`{"content":"x"}`, ""), chunkFrame(`{}`, finish), chunkFrame(`{}`, finish), doneFrame))
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			if got := conform.Describe(events[len(events)-1]); got != want {
				t.Fatalf("ended %q, want %q", got, want)
			}
		})
	}
}

func TestStreamDoneWithoutFinishReason(t *testing.T) {
	p := serve(t, stringHandler(chunkFrame(`{"content":"x"}`, ""), doneFrame))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", `text 0 "x"`, "err network")
	p = serve(t, stringHandler(doneFrame))
	wantEvents(t, collect(t, p, turn("hi")), "err network")
}

func TestStreamKeepAliveAndUnusedFrames(t *testing.T) {
	p := serve(t, stringHandler(
		": OPENROUTER PROCESSING\n\n",
		": OPENROUTER PROCESSING\n\n",
		chunkFrame(openingFrame, ""),
		frame(`{"id":"gen-1","object":"something.else"}`),
		frame(`{}`),
		frame(`{"id":"gen-1","choices":null,"usage":null,"error":null}`),
		chunkFrame(`{"content":"hi","refusal":null,"annotations":[]}`, ""),
		": OPENROUTER PROCESSING\n\n",
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", `text 0 "hi"`, "ok turn")
}

func TestStreamKeepAliveHoldsTheIdleDeadline(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for range 6 {
			io.WriteString(w, ": OPENROUTER PROCESSING\n\n")
			flush(w)
			time.Sleep(40 * time.Millisecond)
		}
		io.WriteString(w, chunkFrame(`{"content":"late"}`, "stop")+doneFrame)
	}, WithTransport(lib.Options{Idle: 100 * time.Millisecond}))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", `text 0 "late"`, "ok turn")
}

func TestStreamIdleTimeout(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, chunkFrame(openingFrame, ""))
		flush(w)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}, WithTransport(lib.Options{Idle: 100 * time.Millisecond}))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", "err network")
}

func TestStreamIdleTimeoutBeforeHeaders(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}, WithTransport(lib.Options{Idle: 100 * time.Millisecond}))
	wantEvents(t, collect(t, p, turn("hi")), "err network")
}

func TestStreamFrameLargerThanScannerLimit(t *testing.T) {
	big := strings.Repeat("a", 100_000)
	p := serve(t, stringHandler(
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_big","type":"function","function":{"name":"write","arguments":"{\"body\":\"`+big+`\"}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("write"))
	if call, ok := events[1].(api.EventTool); !ok || call.Args != `{"body":"`+big+`"}` {
		t.Fatalf("argument fragment lost: %.200q", conform.DescribeAll(events))
	}
}

func TestStreamUnparseableFrame(t *testing.T) {
	p := serve(t, stringHandler(chunkFrame(`{"content":"x"}`, ""), frame(`{"choices":[`), doneFrame))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", `text 0 "x"`, "err network")
}

func TestStreamDisconnectIsSilent(t *testing.T) {
	for _, finish := range []string{"", "stop", "length"} {
		t.Run("finish="+finish, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, chunkFrame(`{"content":"done"}`, finish))
				flush(w)
				drop(w)
			})
			wantEvents(t, collect(t, p, turn("hi")), "start gen-1 qwen/qwen3.8-27b", `text 0 "done"`)
		})
	}
}

func TestStreamCancelIsSilent(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, chunkFrame(`{"content":"part"}`, ""))
		flush(w)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	ch := p.Stream(ctx, turn("hi"))
	var events []api.Event
	for e := range ch {
		events = append(events, e)
		if _, ok := e.(api.EventText); ok {
			cancel()
		}
	}
	wantEvents(t, events, "start gen-1 qwen/qwen3.8-27b", `text 0 "part"`)
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	events = nil
	for e := range p.Stream(ctx, turn("hi")) {
		events = append(events, e)
	}
	wantEvents(t, events, "start gen-1 qwen/qwen3.8-27b", `text 0 "part"`)
}

func TestStreamUnreachableHost(t *testing.T) {
	p := mustNew(t, Conf{Model: api.Model{Name: testModel}, BaseURL: "http://127.0.0.1:1"})
	wantEvents(t, collect(t, p, turn("hi")), "err network")
}

func TestStartCarriesTheConfiguredModel(t *testing.T) {
	p := serve(t, stringHandler(frame(`{"id":"gen-9","model":"qwen/qwen3.8-27b-20261101","provider":"Wafer","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":"stop"}]}`), doneFrame))
	wantEvents(t, collect(t, p, turn("hi")), "start gen-9 qwen/qwen3.8-27b", `text 0 "x"`, "ok turn")
}

func TestHTTPErrors(t *testing.T) {
	const overflow = "This model's maximum context length is 32768 tokens. However, you requested 40000 tokens. Please reduce the length of the messages."
	for _, tc := range []struct {
		name   string
		status int
		header http.Header
		body   string
		want   string
		check  func(e *APIError) bool
	}{
		{"openrouter bad model", 400, nil, `{"error":{"message":"no-such/model-conform is not a valid model ID","code":400},"user_id":"user_x"}`,
			"err invalid generic: http 400: no-such/model-conform is not a valid model ID", nil},
		{"openrouter context", 400, nil, `{"error":{"message":"This endpoint's maximum context length is 131072 tokens. However, you requested about 187521 tokens.","code":400,"metadata":{"provider_name":null}}}`,
			"err context_length", nil},
		{"openrouter rate limit with a wait", 429, http.Header{"Retry-After": {"7"}, "X-Generation-Id": {"gen-7"}}, `{"error":{"message":"Rate limit exceeded","code":429,"metadata":{"error_type":"rate_limit_exceeded","provider_code":"rate_limited"}}}`,
			"err busy generic: http 429 rate_limit_exceeded: Rate limit exceeded (request gen-7)", func(e *APIError) bool { return e.RetryAfter == 7*time.Second }},
		{"openrouter refusal on a 403", 403, nil, `{"error":{"code":403,"message":"declined","metadata":{"error_type":"refusal"}}}`, "err refused", nil},
		{"openrouter guardrail on a 403", 403, nil, `{"error":{"code":403,"message":"blocked by guardrail"}}`, "err access", nil},
		{"llama.cpp context", 400, nil, `{"error":{"code":400,"message":"request (40000 tokens) exceeds the available context size (32768 tokens), try increasing it","type":"exceed_context_size_error","n_prompt_tokens":40000,"n_ctx":32768}}`, "err context_length", nil},
		{"llama.cpp loading", 503, nil, `{"error":{"code":503,"message":"Loading model","type":"unavailable_error"}}`, "err busy generic: http 503 unavailable_error: Loading model", nil},
		{"llama.cpp not supported", 501, nil, `{"error":{"code":501,"message":"This server does not support metrics endpoint.","type":"not_supported_error"}}`, "err invalid", nil},
		{"llama.cpp image to a text model", 500, nil, `{"error":{"code":500,"message":"image input is not supported - hint: if this is unexpected, you may need to provide the mmproj","type":"server_error"}}`, "err invalid", nil},
		{"openai shape with a request id", 400, http.Header{"X-Request-Id": {"req_1"}}, `{"error":{"message":"bad","type":"invalid_request_error","param":"messages","code":"invalid_value"}}`,
			"err invalid generic: http 400 invalid_request_error invalid_value: bad (param messages) (request req_1)", nil},
		{"flat vllm shape", 400, nil, `{"object":"error","message":"This model's maximum context length is 32768 tokens.","type":"BadRequestError","param":null,"code":400}`, "err context_length", nil},
		{"error as a string", 500, nil, `{"error":"model crashed"}`, "err network generic: http 500: model crashed", nil},
		{"not json", 502, nil, "<html>bad gateway</html>", "err network generic: http 502: <html>bad gateway</html>", nil},
		{"fastapi detail", 404, nil, `{"detail":"Not Found"}`, `err invalid generic: http 404: {"detail":"Not Found"}`, nil},
		{"payment", 402, nil, `{"error":{"message":"Insufficient credits","code":402}}`, "err access", nil},
		{"a timeout", 408, nil, ``, "err network", nil},
		{"an unlisted status", 418, nil, ``, "err unknown", nil},
		{"a context word on a 404 is not an overflow", 404, nil, `{"error":{"message":"model with context length 8k not found"}}`, "err invalid", nil},
		{"a context overflow on a 413", 413, nil, `{"error":{"message":"` + overflow + `"}}`, "err context_length", nil},
		{"a 413 that names no context", 413, nil, `{"error":{"message":"payload too large"}}`, "err too_large", nil},
		{"a context overflow on a 422", 422, nil, `{"error":{"message":"` + overflow + `"}}`, "err context_length", nil},
		{"a context overflow in upper case", 400, nil, `{"error":{"message":"` + strings.ToUpper(overflow) + `"}}`, "err context_length", nil},
		{"a context overflow in a body that is not json", 400, nil, overflow, "err context_length", nil},
		{"a refusal that only names the context", 400, nil, `{"error":{"message":"Context caching is not supported for this model.","type":"invalid_request_error"}}`, "err invalid", nil},
		{"openai quota on a 429", 429, nil, `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`,
			"err access generic: http 429 insufficient_quota insufficient_quota: You exceeded your current quota, please check your plan and billing details.", nil},
		{"a spend limit on a 429", 429, nil, `{"error":{"message":"Your project has reached its spend limit.","type":"insufficient_quota","code":"project_spend_limit_exceeded"}}`, "err access", nil},
		{"openrouter upstream rejects one sample", 502, http.Header{"X-Generation-Id": {"gen-5"}}, `{"error":{"message":"` + upstreamToolValidation + `","code":502,"metadata":{"provider_name":"Groq"}}}`,
			"err network generic: http 502: " + upstreamToolValidation + " (request gen-5)", func(e *APIError) bool { return e.Message == upstreamToolValidation && e.Code == "" }},
		{"openrouter upstream with an error type", 502, nil, `{"error":{"message":"` + upstreamToolValidation + `","code":502,"metadata":{"error_type":"invalid_request","provider_name":"Groq"}}}`,
			"err network generic: http 502 invalid_request: " + upstreamToolValidation, func(e *APIError) bool { return e.Message == upstreamToolValidation }},
		{"a param that is not a string", 400, nil, `{"error":{"message":"bad","type":"invalid_request_error","param":{"x":1},"code":"invalid_value"}}`,
			"err invalid generic: http 400 invalid_request_error invalid_value: bad", func(e *APIError) bool {
				return e.Param == "" && e.Type == "invalid_request_error" && e.Code == "invalid_value"
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.header {
					w.Header()[k] = v
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			events := collect(t, p, turn("hi"))
			wantEvents(t, events, tc.want)
			var e *APIError
			if !errors.As(events[0].(api.EventErr).Err, &e) || e.Status != tc.status || (tc.check != nil && !tc.check(e)) {
				t.Fatalf("APIError %+v", e)
			}
		})
	}
}

func TestErrorBodyIsCut(t *testing.T) {
	c := newConverter(testModel, 0, 0)
	events := c.End(&lib.HTTPError{Status: 502, Body: []byte(strings.Repeat("é", 2000))})
	var e *APIError
	if !errors.As(events[0].(api.EventErr).Err, &e) || len([]rune(e.Message)) != 513 {
		t.Fatalf("message %q", e.Message)
	}
}

func TestEndings(t *testing.T) {
	const failed = `{"id":"gen-1","choices":[],"error":{"code":502,"message":"Provider disconnected","metadata":{"error_type":"provider_unavailable"}}}`
	const held = "err network generic: provider_unavailable: Provider disconnected (request gen-1)"
	for _, c := range []struct {
		err  error
		held bool
		want string
	}{
		{io.EOF, false, ""},
		{io.ErrUnexpectedEOF, false, ""},
		{context.Canceled, false, ""},
		{context.DeadlineExceeded, false, ""},
		{fmt.Errorf("dial tcp: %w", context.DeadlineExceeded), false, "err network dial tcp: context deadline exceeded"},
		{fmt.Errorf("read: %w", context.Canceled), false, "err network read: context canceled"},
		{lib.ErrIdle, false, "err network neui: stream went idle"},
		{errors.New("boom"), false, "err network boom"},
		{context.Canceled, true, ""},
		{io.EOF, true, held},
		{fmt.Errorf("read: %w", context.DeadlineExceeded), true, held},
		{lib.ErrIdle, true, held},
	} {
		conv := newConverter(testModel, 0, 0)
		if c.held {
			conv.Convert(lib.Frame{Data: []byte(failed)})
		}
		if got := strings.Join(conform.DescribeAll(conv.End(c.err)), "|"); got != c.want {
			t.Fatalf("End(%v) held %v = %q, want %q", c.err, c.held, got, c.want)
		}
	}
}

func TestStreamInBandError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame string
		want  string
	}{
		{"openrouter", `{"id":"gen-1","object":"chat.completion.chunk","created":1,"model":"m","provider":"P","error":{"code":502,"message":"Provider disconnected","metadata":{"error_type":"provider_unavailable"}},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`,
			"err network generic: provider_unavailable: Provider disconnected (request gen-1)"},
		{"openrouter without a type", `{"id":"gen-1","error":{"code":429,"message":"slow down"},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`,
			"err busy generic: 429: slow down (request gen-1)"},
		{"openrouter context", `{"id":"gen-1","error":{"code":400,"message":"too long","metadata":{"error_type":"context_length_exceeded"}},"choices":[]}`, "err context_length"},
		{"llama.cpp", `{"error":{"code":500,"message":"decode failed","type":"server_error"}}`, "err network generic: server_error 500: decode failed (request gen-1)"},
		{"typed, no number", `{"error":{"message":"Rate limit reached.","type":"rate_limit_error","code":"rate_limit_exceeded"}}`, "err busy"},
		{"ollama", `{"error":{"message":"model runner has unexpectedly stopped","type":"api_error","param":null,"code":null}}`, "err network"},
		{"a string", `{"error":"something broke"}`, "err unknown generic: something broke (request gen-1)"},
		{"no message", `{"error":{"code":503}}`, `err busy generic: 503: {"code":503} (request gen-1)`},
		{"openrouter upstream rejects one sample", `{"id":"gen-1","object":"chat.completion.chunk","created":1,"model":"openai/gpt-oss-20b","provider":"Groq","system_fingerprint":"fp_aef561cf8e","choices":[],"error":{"code":502,"message":"` + upstreamToolValidation + `","metadata":{"error_type":"provider_unavailable"}}}`,
			"err network generic: provider_unavailable: " + upstreamToolValidation + " (request gen-1)"},
		{"openrouter upstream without a type", `{"id":"gen-1","choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}],"error":{"code":502,"message":"` + upstreamToolValidation + `","metadata":{"provider_name":"Groq"}}}`,
			"err network generic: 502: " + upstreamToolValidation + " (request gen-1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, stringHandler(chunkFrame(`{"content":"partial"}`, ""), frame(tc.frame), doneFrame))
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantEvents(t, events, "start gen-1 qwen/qwen3.8-27b", `text 0 "partial"`, tc.want)
		})
	}
}

func TestStreamInBandErrorWaitsForTheUsage(t *testing.T) {
	const failed = `{"id":"gen-1","object":"chat.completion.chunk","created":1,"model":"m","provider":"P","choices":[],"error":{"code":502,"message":"Provider disconnected","metadata":{"error_type":"provider_unavailable"}}}`
	for _, tc := range []struct {
		name   string
		ending string
		hold   bool
	}{
		{"at [DONE]", doneFrame, false},
		{"at the end of the body", "", false},
		{"at a frame cut short", `data: {"id":"gen-1"`, false},
		{"at a frame that does not parse", frame(`{"choices":[`), false},
		{"past a second error", frame(`{"error":{"code":429,"message":"slow down"}}`) + doneFrame, false},
		{"at the idle deadline", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				stringHandler(chunkFrame(`{"content":"partial"}`, ""), frame(failed), chunkFrame(`{"content":"late"}`, "error"), usageFrame, tc.ending)(w, r)
				if tc.hold {
					select {
					case <-r.Context().Done():
					case <-time.After(5 * time.Second):
					}
				}
			}, WithTransport(lib.Options{Idle: 200 * time.Millisecond}))
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantEvents(t, events,
				"start gen-1 qwen/qwen3.8-27b",
				`text 0 "partial"`,
				"usage in=376 out=45 rsn=15 cr=64 cw=0",
				"err network generic: provider_unavailable: Provider disconnected (request gen-1)",
			)
		})
	}
	c := newConverter(testModel, 0, 0)
	if _, more := c.Convert(lib.Frame{Data: []byte(failed)}); !more {
		t.Fatal("the turn ended at the error frame")
	}
	if events := c.End(context.Canceled); len(events) != 0 {
		t.Fatalf("a cancelled turn ended with %q", conform.DescribeAll(events))
	}
}

func TestRecordedStreams(t *testing.T) {
	for _, tc := range []struct {
		file string
		want string
	}{
		{"openrouter-qwen-text.sse", `reasoning[reasoning] "The user wants me to say \"hi\" in one word. This is simple - I just need to respond with a single word greeting.\n" | text "\n\nHi" | opaque generic.fields reasoning_details | usage in=58 out=3 rsn=30 cr=0 cw=0 | ok turn`},
		{"openrouter-qwen-tools.sse", `reasoning[reasoning] "The user is asking for the weather in Paris and Rome. Since the two are independent of each other, they can be called in parallel.\n" | text "\n\n\n" | tool call_004ac60f136a49038037f6c9 get_weather {"city": "Paris"} | tool call_7fc25788ec4f4f409db6ccfb get_weather {"city": "Rome"} | opaque generic.fields reasoning_details | usage in=335 out=54 rsn=30 cr=0 cw=0 | ok turn`},
		{"openrouter-qwen-cap.sse", `reasoning[reasoning] "We need to respond to user: \"Write a four-line poem about tea.\" Simple. Then final with poem four lines" | usage in=60 out=0 rsn=24 cr=0 cw=0 | err capped`},
		{"openrouter-gptoss-tools.sse", `reasoning[reasoning] "We need to call get_weather for Paris and Rome." | tool chatcmpl-tool-ad2a7e93260970c4 get_weather {"city":"Paris"} | opaque generic.fields reasoning_details | usage in=74 out=19 rsn=16 cr=64 cw=0 | ok turn`},
		{"openrouter-gptoss-upstream-error.sse", `reasoning[reasoning] "Need to call tool." | usage in=107 out=0 rsn=5 cr=0 cw=0 | err network`},
		{"openrouter-qwen-cache-2.sse", `text "ok" | usage in=48 out=2 rsn=0 cr=9920 cw=0 | ok turn`},
		{"openrouter-mistral-tools.sse", `reasoning[reasoning] "The user wants to know the current weather in Paris and Rome. I need to use the get_weather tool for each city. Let me make two separate calls - one for Paris and one for Rome." | text "I'll get the current weather for both Paris and Rome for you." | tool I77MZpriD get_weather {"city": "Paris"} | tool bAk2mbhvM get_weather {"city": "Rome"} | opaque generic.fields reasoning_details | usage in=94 out=39 rsn=41 cr=0 cw=0 | ok turn`},
		{"llamacpp-weather-tools.sse", `reasoning[reasoning_content] "The user is asking for the current weather in two cities: Paris and Rome. They specifically want me to use the tool for each city. I have a get_weather function that takes a city parameter. I need to call this function twice, once for Paris and once for Rome." | tool EbJNSsMDTpz1hWF7LnV5DjEKzWHX1x6p get_weather {"city":"Paris"} | tool lku4tuMDAPTHMZiTmb8fb8dF36jBw9Pi get_weather {"city":"Rome"} | usage in=286 out=112 rsn=0 cr=0 cw=0 | ok turn`},
		{"llamacpp-tools.sse", `reasoning[reasoning_content] "The user is asking me to look up the code of item \"alpha\" using the lookup tool. I need to call the lookup function with the item parameter set to \"alpha\"." | tool t9Obof2b5xOfzVwv4E4fUivinD9O4L7G lookup {"item":"alpha"} | usage in=287 out=64 rsn=0 cr=0 cw=0 | ok turn`},
		{"llamacpp-result-rc.sse", `reasoning[reasoning_content] "The lookup function returned \"alpha: 58\", which means the code for item \"alpha\" is 58. The user asked me to reply with the code only, so I should just return \"58\"." | text "58" | usage in=21 out=51 rsn=0 cr=350 cw=0 | ok turn`},
		{"llamacpp-cache-2.sse", `text "ok" | usage in=4 out=2 rsn=0 cr=3147 cw=0 | ok turn`},
		{"llamacpp-cap.sse", `reasoning[reasoning_content] "Thinking Process:\n\n1.  **Analyze the Request:**\n    *   Topic: Tea.\n    *" | usage in=4 out=24 rsn=0 cr=14 cw=0 | err capped`},
		{"llamacpp-stop-off.sse", `text "1, 2, 3, 4, 5, 6, " | usage in=27 out=19 rsn=0 cr=0 cw=0 | ok turn`},
		{"llamacpp-stop-thinking.sse", `reasoning[reasoning_content] "Thinking Process:\n\n1.  **Analyze the Request:**\n    *   Task: Count from 1 to 10.\n    *   Format: Comma-separated.\n    *   Constraint: Nothing else (no intro, no outro, no explanations).\n\n2.  **Generate the Content:**\n    *   Numbers: 1, 2, 3, 4, 5, 6, " | usage in=2 out=91 rsn=0 cr=23 cw=0 | ok turn`},
		{"llamacpp-empty.sse", `text "ok" | usage in=35 out=2 rsn=0 cr=0 cw=0 | ok turn`},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "streams", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			events := collect(t, serve(t, stringHandler(string(data))), turn("hi"))
			conform.CheckUnframed(t, events)
			b := response.NewBulder()
			var parts []string
			for _, e := range events {
				if _, ok := b.On(e); !ok {
					t.Fatalf("builder refused %s", conform.Describe(e))
				}
				switch e := e.(type) {
				case api.EventUsage, api.EventOk:
					parts = append(parts, conform.Describe(e))
				case api.EventErr:
					parts = append(parts, "err "+string(e.Code))
				}
			}
			var blocks []string
			for _, c := range b.Flush().Content.Ordered() {
				switch c := c.(type) {
				case api.ContentReasoning:
					data, own := lib.OwnMeta(c.Meta, providerName)
					if !own {
						t.Fatalf("reasoning without own Meta: %s", c.Meta)
					}
					blocks = append(blocks, fmt.Sprintf("reasoning[%s] %q", strings.Trim(string(data), `"`), strings.Join(c.Thoughts.Ordered(), "")))
				case api.ContentText:
					blocks = append(blocks, fmt.Sprintf("text %q", c.Text))
				case api.ContentToolUse:
					blocks = append(blocks, fmt.Sprintf("tool %s %s %s", c.ID, c.Name, c.Input))
				case api.ContentOpaque:
					var data struct {
						Fields map[string]any `json:"fields"`
						Calls  map[string]any `json:"calls"`
					}
					if err := json.Unmarshal(c.Data, &data); err != nil {
						t.Fatal(err)
					}
					blocks = append(blocks, strings.Join(append([]string{"opaque", c.Kind}, append(slices.Sorted(maps.Keys(data.Fields)), slices.Sorted(maps.Keys(data.Calls))...)...), " "))
				default:
					blocks = append(blocks, fmt.Sprintf("%T", c))
				}
			}
			if got := strings.Join(append(blocks, parts...), " | "); got != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestRecordedOpaqueFields(t *testing.T) {
	type entry struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		Signature string `json:"signature"`
		Data      string `json:"data"`
		Format    string `json:"format"`
		ID        string `json:"id"`
		Index     *int   `json:"index"`
	}
	for _, tc := range []struct {
		file  string
		model string
		check func(t *testing.T, details []entry, annotations []any, thoughts string, calls []api.ContentToolUse)
	}{
		{"openrouter-claude-haiku-thinking-tools", "anthropic/claude-haiku-5.5", func(t *testing.T, d []entry, _ []any, thoughts string, _ []api.ContentToolUse) {
			if len(d) != 1 || d[0].Type != "reasoning.text" || d[0].Format != "anthropic-claude-v1" || d[0].Text != thoughts || len(thoughts) != 151 ||
				!strings.HasPrefix(d[0].Signature, "CAQS7AYK") || len(d[0].Signature) != 1273 || d[0].Index == nil || *d[0].Index != 0 {
				t.Fatalf("details %+v, thoughts %q", d, thoughts)
			}
		}},
		{"openrouter-gpt5mini-encrypted-tools", "openai/gpt-5-mini", func(t *testing.T, d []entry, _ []any, thoughts string, _ []api.ContentToolUse) {
			if len(d) != 1 || d[0].Type != "reasoning.encrypted" || d[0].Format != "openai-responses-v1" || !strings.HasPrefix(d[0].ID, "rs_") ||
				!strings.HasPrefix(d[0].Data, "gAAAAA") || len(d[0].Data) != 2351 || thoughts != "" {
				t.Fatalf("details %+v, thoughts %q", d, thoughts)
			}
		}},
		{"openrouter-gemini-flashlite-encrypted-tools", "google/gemini-3.5-flash-lite", func(t *testing.T, d []entry, _ []any, thoughts string, calls []api.ContentToolUse) {
			if len(d) != 1 || d[0].Type != "reasoning.encrypted" || d[0].Format != "google-gemini-v1" || len(calls) != 1 || d[0].ID != calls[0].ID || len(d[0].Data) != 265 {
				t.Fatalf("details %+v, calls %+v", d, calls)
			}
		}},
		{"openrouter-qwen-details-tools", "qwen/qwen3.8-27b", func(t *testing.T, d []entry, _ []any, thoughts string, _ []api.ContentToolUse) {
			if len(d) != 1 || d[0].Type != "reasoning.text" || d[0].Format != "unknown" || d[0].Text != thoughts || len(thoughts) != 109 {
				t.Fatalf("details %+v, thoughts %q", d, thoughts)
			}
		}},
		{"openrouter-qwen-web-annotations", "qwen/qwen3.8-27b", func(t *testing.T, d []entry, a []any, _ string, _ []api.ContentToolUse) {
			if len(d) != 0 || len(a) != 1 || a[0].(map[string]any)["type"] != "url_citation" {
				t.Fatalf("details %+v, annotations %v", d, a)
			}
		}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "streams", tc.file+".sse"))
			if err != nil {
				t.Fatal(err)
			}
			p := serve(t, stringHandler(string(data)))
			p.model.Name = tc.model
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			b := response.NewBulder()
			for _, e := range events {
				if _, ok := b.On(e); !ok {
					t.Fatalf("builder refused %s", conform.Describe(e))
				}
			}
			content := b.Flush().Content.Ordered()
			var opaque []api.ContentOpaque
			var calls []api.ContentToolUse
			var thoughts string
			for _, c := range content {
				switch c := c.(type) {
				case api.ContentOpaque:
					opaque = append(opaque, c)
				case api.ContentToolUse:
					calls = append(calls, c)
				case api.ContentReasoning:
					thoughts = strings.Join(c.Thoughts.Ordered(), "")
				}
			}
			if len(opaque) != 1 || opaque[0].Kind != "generic.fields" || opaque[0].Model != tc.model {
				t.Fatalf("opaque blocks %+v", opaque)
			}
			var fields struct {
				Fields struct {
					Details     []entry `json:"reasoning_details"`
					Annotations []any   `json:"annotations"`
				} `json:"fields"`
				Calls map[string]any `json:"calls"`
			}
			if err := json.Unmarshal(opaque[0].Data, &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields.Calls) != 0 {
				t.Fatalf("per-call fields %v", fields.Calls)
			}
			tc.check(t, fields.Fields.Details, fields.Fields.Annotations, thoughts, calls)

			msgs := []api.Message{user(text("hi")), assistant(content...), answer(calls)}
			sent, err := os.ReadFile(filepath.Join("testdata", "streams", tc.file+".sent.json"))
			if err != nil {
				t.Fatal(err)
			}
			var got []any
			var want any
			if err := json.Unmarshal([]byte(messagesJSON(t, p, msgs...)), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(sent, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[1], want) {
				t.Fatalf("replayed\n%v\nthe probe sent and the server took\n%v", got[1], want)
			}

			other := mustNew(t, Conf{Model: api.Model{Name: tc.model + ":nitro"}, BaseURL: "http://127.0.0.1:8080/v1"})
			replay := messagesJSON(t, other, msgs...)
			if strings.Contains(replay, "reasoning") || strings.Contains(replay, "annotations") {
				t.Fatalf("another model got the reasoning or fields: %s", replay)
			}
		})
	}
}

func TestRecordedReasoningSummary(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "streams", "openrouter-gpt5mini-summary.sse"))
	if err != nil {
		t.Fatal(err)
	}
	p := serve(t, stringHandler(string(data)))
	p.model.Name = "openai/gpt-5-mini"
	events := collect(t, p, turn("hi"))
	conform.CheckUnframed(t, events)
	b := response.NewBulder()
	for _, e := range events {
		if _, ok := b.On(e); !ok {
			t.Fatalf("builder refused %s", conform.Describe(e))
		}
	}
	var thoughts string
	var fields struct {
		Fields struct {
			Details []struct {
				Type    string `json:"type"`
				Summary string `json:"summary"`
				Data    string `json:"data"`
				Format  string `json:"format"`
				ID      string `json:"id"`
				Index   int    `json:"index"`
			} `json:"reasoning_details"`
		} `json:"fields"`
	}
	for _, c := range b.Flush().Content.Ordered() {
		switch c := c.(type) {
		case api.ContentReasoning:
			thoughts = strings.Join(c.Thoughts.Ordered(), "")
		case api.ContentOpaque:
			if err := json.Unmarshal(c.Data, &fields); err != nil {
				t.Fatal(err)
			}
		}
	}
	d := fields.Fields.Details
	if len(d) != 2 || d[0].Type != "reasoning.summary" || d[0].Format != "openai-responses-v1" || d[0].Index != 0 || d[0].Summary != thoughts || !strings.HasPrefix(thoughts, "**Calculating the sum**\n\nI'm looking") || !strings.HasSuffix(thoughts, "Keeping it simple is key here!") ||
		d[1].Type != "reasoning.encrypted" || d[1].Index != 1 || !strings.HasPrefix(d[1].ID, "rs_") || !strings.HasPrefix(d[1].Data, "gAAAAA") {
		t.Fatalf("details %+v, thoughts %q", d, thoughts)
	}
}

func answer(calls []api.ContentToolUse) api.Message {
	var results api.Contents
	for _, c := range calls {
		results = append(results, api.ContentToolResult{ID: c.ID, Content: api.UserContents{text("alpha: 58")}})
	}
	if len(results) == 0 {
		results = append(results, text("Thanks."))
	}
	return user(results...)
}
