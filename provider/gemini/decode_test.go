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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/internal/thread/response"
	"neui.dev/provider/lib"
)

const (
	created      = `{"interaction":{"object":"interaction","model":"gemini-4-hydrogen"},"event_type":"interaction.created"}`
	statusUpdate = `{"status":"in_progress","event_type":"interaction.status_update"}`
	usageJSON    = `{"total_tokens":256,"total_input_tokens":43,"total_cached_tokens":0,"total_output_tokens":9,"total_tool_use_tokens":0,"total_thought_tokens":204}`
	usageLine    = "usage in=43 out=9 rsn=204 cr=0 cw=0"
	done         = "event: done\ndata: [DONE]\n\n"
)

func ev(data string) string {
	var head frameHead
	if err := json.Unmarshal([]byte(data), &head); err != nil {
		panic(err)
	}
	return "event: " + head.EventType + "\ndata: " + data + "\n\n"
}

func startStep(i int, step string) string {
	return ev(fmt.Sprintf(`{"index":%d,"step":%s,"event_type":"step.start"}`, i, step))
}

func deltaStep(i int, delta string) string {
	return ev(fmt.Sprintf(`{"index":%d,"delta":%s,"event_type":"step.delta"}`, i, delta))
}

func stopStep(i int) string {
	return ev(fmt.Sprintf(`{"index":%d,"event_type":"step.stop"}`, i))
}

func completed(status string) string {
	return ev(fmt.Sprintf(`{"interaction":{"status":%q,"usage":%s,"created":"2026-11-19T19:30:00Z","updated":"2026-11-19T19:30:00Z","service_tier":"standard","object":"interaction","model":"gemini-4-hydrogen"},"event_type":"interaction.completed"}`, status, usageJSON))
}

func signed(i int, sig string) string {
	return startStep(i, `{"type":"thought"}`) + deltaStep(i, `{"signature":"`+sig+`","type":"thought_signature"}`) + stopStep(i)
}

func opened() string {
	return ev(created) + ev(statusUpdate)
}

func serve(t *testing.T, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return mustNew(t, Conf{Model: api.Model{Name: testModel}}, append([]Option{WithAPIKey("test-key"), WithBaseURL(srv.URL)}, opts...)...)
}

func sseServer(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, f := range strings.SplitAfter(body, "\n\n") {
			io.WriteString(w, f)
			w.(http.Flusher).Flush()
		}
	}
}

func collect(t *testing.T, p *Provider, req api.Request) []api.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var events []api.Event
	for e := range p.Stream(ctx, req) {
		events = append(events, e)
	}
	if ctx.Err() != nil {
		t.Fatalf("stream did not end within 10s: %v", conform.DescribeAll(events))
	}
	return events
}

func wantTrace(t *testing.T, events []api.Event, want ...string) {
	t.Helper()
	got := conform.DescribeAll(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
	for i := range got {
		if got[i] != want[i] && !(strings.HasPrefix(want[i], "err ") && strings.HasPrefix(got[i], want[i]+" ")) {
			t.Fatalf("event %d: got %q, want %q\nfull:\n%s", i, got[i], want[i], strings.Join(got, "\n"))
		}
	}
}

func convertAll(frames ...string) []api.Event {
	c := newConverter(testModel, "gemini_test", 0, 0)
	var all []api.Event
	for _, f := range frames {
		var head frameHead
		json.Unmarshal([]byte(f), &head)
		events, more := c.Convert(lib.Frame{Event: head.EventType, Data: []byte(f)})
		all = append(all, events...)
		if !more {
			return all
		}
	}
	return append(all, c.End(io.EOF)...)
}

func TestStreamSequences(t *testing.T) {
	const start = "start  gemini-4-hydrogen"
	for _, c := range []struct {
		name string
		sse  string
		want []string
	}{
		{
			name: "text over several deltas stays on one index",
			sse: ": keep-alive\n\n" + opened() + signed(0, "c2ln") +
				startStep(1, `{"type":"model_output"}`) +
				deltaStep(1, `{"text":"Hel","type":"text"}`) + deltaStep(1, `{"text":"","type":"text"}`) + deltaStep(1, `{"text":"lo","type":"text"}`) +
				stopStep(1) + completed("completed") + done,
			want: []string{start, `thinking 0 part=0 ""`, `meta 0 {"provider":"gemini","data":{"type":"thought","signature":"c2ln"}}`, "close 0", `text 1 "Hel"`, `text 1 "lo"`, "close 1", usageLine, "ok turn"},
		},
		{
			name: "summary then signature",
			sse: opened() + startStep(0, `{"type":"thought"}`) +
				deltaStep(0, `{"content":{"text":"**Plan**\n\nFirst.\n\n\n","type":"text"},"type":"thought_summary"}`) +
				deltaStep(0, `{"content":{"text":"**More**\n\nThen.\n\n\n","type":"text"},"type":"thought_summary"}`) +
				deltaStep(0, `{"content":{"type":"image","data":"AA=="},"type":"thought_summary"}`) +
				deltaStep(0, `{"signature":"c2ln","type":"thought_signature"}`) + stopStep(0) +
				startStep(1, `{"type":"model_output"}`) + deltaStep(1, `{"text":"208","type":"text"}`) + stopStep(1) + completed("completed") + done,
			want: []string{start, `summary 0 part=0 "**Plan**\n\nFirst.\n\n\n"`, `summary 0 part=0 "**More**\n\nThen.\n\n\n"`, `meta 0 {"provider":"gemini","data":{"type":"thought","signature":"c2ln"}}`, "close 0", `text 1 "208"`, "close 1", usageLine, "ok turn"},
		},
		{
			name: "an empty thought opens nothing",
			sse: opened() + startStep(0, `{"type":"thought"}`) + stopStep(0) +
				startStep(1, `{"type":"model_output"}`) + deltaStep(1, `{"text":"hi","type":"text"}`) + stopStep(1) + completed("completed") + done,
			want: []string{start, `text 0 "hi"`, "close 0", usageLine, "ok turn"},
		},
		{
			name: "prefilled starts",
			sse: opened() + startStep(0, `{"type":"thought","signature":"","summary":[{"type":"text","text":"Plan."},{"type":"text","text":""}]}`) +
				deltaStep(0, `{"signature":"c2ln","type":"thought_signature"}`) + stopStep(0) +
				startStep(1, `{"type":"model_output","content":[{"type":"text","text":"Hi"}]}`) + deltaStep(1, `{"text":" there","type":"text"}`) + stopStep(1) +
				startStep(2, `{"type":"thought","signature":"c2lnMg=="}`) + stopStep(2) +
				completed("completed") + done,
			want: []string{start, `summary 0 part=0 "Plan."`, `meta 0 {"provider":"gemini","data":{"type":"thought","signature":"c2ln"}}`, "close 0", `text 1 "Hi"`, `text 1 " there"`, "close 1",
				`thinking 2 part=0 ""`, `meta 2 {"provider":"gemini","data":{"type":"thought","signature":"c2lnMg=="}}`, "close 2", usageLine, "ok turn"},
		},
		{
			name: "a call with an arguments delta",
			sse: opened() + signed(0, "c2ln") +
				startStep(1, `{"id":"call_1","type":"function_call","name":"get_weather","arguments":{}}`) +
				deltaStep(1, `{"arguments":"{\"city\":","type":"arguments_delta"}`) + deltaStep(1, `{"arguments":"\"Paris\"}","type":"arguments_delta"}`) +
				stopStep(1) + completed("requires_action") + done,
			want: []string{start, `thinking 0 part=0 ""`, `meta 0 {"provider":"gemini","data":{"type":"thought","signature":"c2ln"}}`, "close 0",
				`tool 1 "call_1" "get_weather" ""`, `tool 1 "" "" "{\"city\":"`, `tool 1 "" "" "\"Paris\"}"`, "close 1", usageLine, "ok turn"},
		},
		{
			name: "a call with no delta and empty arguments",
			sse:  opened() + startStep(0, `{"id":"call_1","type":"function_call","name":"clock","arguments":{}}`) + stopStep(0) + completed("requires_action") + done,
			want: []string{start, `tool 0 "call_1" "clock" ""`, `tool 0 "" "" "{}"`, "close 0", usageLine, "ok turn"},
		},
		{
			name: "a call whose start carries the arguments",
			sse: opened() + startStep(0, `{"id":"call_1","type":"function_call","name":"lookup","arguments":{"q":"cat"}}`) + stopStep(0) +
				startStep(1, `{"id":"call_2","type":"function_call","name":"lookup"}`) + stopStep(1) +
				startStep(2, `{"id":"call_3","type":"function_call","name":"lookup","arguments":null}`) + stopStep(2) + completed("requires_action") + done,
			want: []string{start, `tool 0 "call_1" "lookup" ""`, `tool 0 "" "" "{\"q\":\"cat\"}"`, "close 0", `tool 1 "call_2" "lookup" ""`, `tool 1 "" "" "{}"`, "close 1",
				`tool 2 "call_3" "lookup" ""`, `tool 2 "" "" "{}"`, "close 2", usageLine, "ok turn"},
		},
		{
			name: "parallel calls after text",
			sse: opened() + signed(0, "c2ln") +
				startStep(1, `{"type":"model_output"}`) + deltaStep(1, `{"text":"Checking.","type":"text"}`) + stopStep(1) +
				startStep(2, `{"id":"call_a","type":"function_call","name":"w","arguments":{}}`) + deltaStep(2, `{"arguments":"{\"c\":\"Paris\"}","type":"arguments_delta"}`) + stopStep(2) +
				startStep(3, `{"id":"call_b","type":"function_call","name":"w","arguments":{}}`) + deltaStep(3, `{"arguments":"{\"c\":\"Tokyo\"}","type":"arguments_delta"}`) + stopStep(3) +
				completed("requires_action") + done,
			want: []string{start, `thinking 0 part=0 ""`, `meta 0 {"provider":"gemini","data":{"type":"thought","signature":"c2ln"}}`, "close 0", `text 1 "Checking."`, "close 1",
				`tool 2 "call_a" "w" ""`, `tool 2 "" "" "{\"c\":\"Paris\"}"`, "close 2", `tool 3 "call_b" "w" ""`, `tool 3 "" "" "{\"c\":\"Tokyo\"}"`, "close 3", usageLine, "ok turn"},
		},
		{
			name: "an opaque step whose deltas merge",
			sse: opened() + startStep(0, `{"id":"gs_1","signature":"","type":"google_search_call","arguments":{}}`) +
				deltaStep(0, `{"type":"google_search_call","signature":"c2ln","arguments":{"queries":["cats <&>"]}}`) +
				stopStep(0) + startStep(1, `{"type":"google_search_result","call_id":"gs_1","result":[]}`) + stopStep(1) + completed("completed") + done,
			want: []string{start, `opaque 0 "gemini.google_search_call" {"id":"gs_1","signature":"","type":"google_search_call","arguments":{}} ""`,
				`opaque 0 "" {"arguments":{"queries":["cats <&>"]},"id":"gs_1","signature":"c2ln","type":"google_search_call"} ""`, "close 0",
				`opaque 1 "gemini.google_search_result" {"type":"google_search_result","call_id":"gs_1","result":[]} ""`, "close 1", usageLine, "ok turn"},
		},
		{
			name: "ignored frames",
			sse: opened() + startStep(0, `{"type":"user_input","content":[{"type":"text","text":"echo"}]}`) + deltaStep(0, `{"text":"x","type":"text"}`) + stopStep(0) +
				startStep(1, `{"type":"model_output"}`) + deltaStep(1, `{"text":"a","type":"text"}`) +
				deltaStep(1, `{"type":"text_annotation_delta","annotations":[{"type":"url_citation"}]}`) +
				deltaStep(1, `{"type":"image","data":"AA==","mime_type":"image/png"}`) +
				ev(`{"event_type":"interaction.future","whatever":[1]}`) + stopStep(1) +
				startStep(2, `{"id":"c","type":"function_call","name":"n","arguments":{}}`) +
				deltaStep(2, `{"type":"google_search_call","arguments":{"queries":["x"]}}`) + stopStep(2) +
				completed("requires_action") + done,
			want: []string{start, `text 0 "a"`, "close 0", `tool 1 "c" "n" ""`, `tool 1 "" "" "{}"`, "close 1", usageLine, "ok turn"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			events := collect(t, serve(t, sseServer(c.sse)), turn("hi"))
			conform.CheckGrammar(t, events)
			wantTrace(t, events, c.want...)
		})
	}
}

func TestFramesDoNotInheritFields(t *testing.T) {
	events := convertAll(created,
		`{"index":0,"step":{"type":"thought","summary":[{"type":"text","text":"first"}]},"event_type":"step.start"}`,
		`{"index":0,"event_type":"step.stop"}`,
		`{"index":1,"step":{"type":"thought"},"event_type":"step.start"}`,
		`{"index":1,"event_type":"step.stop"}`,
		`{"index":2,"step":{"type":"model_output","content":[{"type":"text","text":"hi"}]},"event_type":"step.start"}`,
		`{"index":2,"event_type":"step.stop"}`,
		`{"index":3,"step":{"type":"model_output"},"event_type":"step.start"}`,
		`{"index":3,"event_type":"step.stop"}`,
		`{"index":4,"step":{"id":"a","type":"function_call","name":"n","arguments":{"k":1}},"event_type":"step.start"}`,
		`{"index":4,"event_type":"step.stop"}`,
		`{"index":5,"step":{"id":"b","type":"function_call","name":"n"},"event_type":"step.start"}`,
		`{"index":5,"event_type":"step.stop"}`,
		`{"event_type":"step.stop"}`,
	)
	wantTrace(t, events, "start  gemini-4-hydrogen", `summary 0 part=0 "first"`, "close 0", `text 1 "hi"`, "close 1",
		`tool 2 "a" "n" ""`, `tool 2 "" "" "{\"k\":1}"`, "close 2", `tool 3 "b" "n" ""`, `tool 3 "" "" "{}"`, "close 3",
		"err network gemini: step.stop without an index")
}

func TestOpaqueDeltaGoesOutAtOnce(t *testing.T) {
	c := newConverter(testModel, "gemini_test", 0, 0)
	c.Convert(lib.Frame{Data: []byte(created)})
	c.Convert(lib.Frame{Data: []byte(`{"index":0,"step":{"id":"ce_1","type":"code_execution_call","arguments":{}},"event_type":"step.start"}`)})
	for _, f := range []struct{ delta, want string }{
		{`{"type":"code_execution_call","arguments":{"code":"print(1)"}}`, `opaque 0 "" {"arguments":{"code":"print(1)"},"id":"ce_1","type":"code_execution_call"} ""`},
		{`{"type":"code_execution_call","signature":"c2ln"}`, `opaque 0 "" {"arguments":{"code":"print(1)"},"id":"ce_1","signature":"c2ln","type":"code_execution_call"} ""`},
	} {
		events, more := c.Convert(lib.Frame{Data: []byte(`{"index":0,"delta":` + f.delta + `,"event_type":"step.delta"}`)})
		if !more {
			t.Fatalf("delta %s ended the stream", f.delta)
		}
		wantTrace(t, events, f.want)
	}
}

func TestUsageArithmetic(t *testing.T) {
	for _, c := range []struct {
		usage string
		want  string
	}{
		{`{"total_input_tokens":100,"total_cached_tokens":30,"total_tool_use_tokens":5,"total_output_tokens":7,"total_thought_tokens":9}`, "usage in=75 out=7 rsn=9 cr=30 cw=0"},
		{`{"total_input_tokens":10,"total_cached_tokens":30,"total_output_tokens":-1,"total_thought_tokens":-2}`, "usage in=0 out=0 rsn=0 cr=30 cw=0"},
		{`{}`, "usage in=0 out=0 rsn=0 cr=0 cw=0"},
	} {
		events := convertAll(created, `{"interaction":{"status":"completed","usage":`+c.usage+`},"event_type":"interaction.completed"}`)
		wantTrace(t, events, "start  gemini-4-hydrogen", c.want, "ok turn")
	}
	events := convertAll(
		`{"interaction":{"model":"m","usage":`+usageJSON+`},"event_type":"interaction.created"}`,
		`{"status":"in_progress","usage":`+usageJSON+`,"event_type":"interaction.status_update"}`,
		`{"status":"in_progress","usage":{"total_input_tokens":50},"event_type":"interaction.status_update"}`,
		`{"interaction":{"status":"completed","usage":{"total_input_tokens":50}},"event_type":"interaction.completed"}`,
	)
	wantTrace(t, events, "start  gemini-4-hydrogen", usageLine, "usage in=50 out=0 rsn=0 cr=0 cw=0", "ok turn")
}

func TestStartCarriesTheConfiguredModel(t *testing.T) {
	events := convertAll(`{"interaction":{"object":"interaction","model":"served-name"},"event_type":"interaction.created"}`,
		`{"interaction":{"status":"completed"},"event_type":"interaction.completed"}`)
	if s, ok := events[0].(api.EventStart); !ok || s.ID != "" || s.Model != testModel {
		t.Fatalf("%s, want start with no id and the configured model", conform.Describe(events[0]))
	}
	events = convertAll(`{"interaction":{"id":"v1_abc","model":"served-name"},"event_type":"interaction.created"}`)
	if s, ok := events[0].(api.EventStart); !ok || s.ID != "v1_abc" {
		t.Fatalf("%s, want the id the API sent", conform.Describe(events[0]))
	}
}

func TestStreamMustOpenWithCreated(t *testing.T) {
	wantTrace(t, convertAll(statusUpdate, created), "err network gemini: the stream opened with \"interaction.status_update\", not interaction.created")
	wantTrace(t, convertAll(`{"event_type":"interaction.future"}`, created), "err network gemini: the stream opened with \"interaction.future\", not interaction.created")
	wantTrace(t, convertAll(`{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`), "err network gemini: the stream opened with \"step.start\", not interaction.created")
	events := convertAll(`{"error":{"message":"slow down","code":"rate_limit_exceeded"},"event_type":"error"}`)
	wantTrace(t, events, "err busy")
	var a *APIError
	if e := events[0].(api.EventErr); !errors.As(e.Err, &a) || a.Code != "rate_limit_exceeded" || a.Message != "slow down" || a.Status != 0 {
		t.Fatalf("err %v, want the in-band APIError", e.Err)
	}
}

func TestMintedCallIDs(t *testing.T) {
	frames := []string{created,
		`{"index":0,"step":{"id":"","type":"function_call","name":"n","arguments":{}},"event_type":"step.start"}`,
		`{"index":0,"event_type":"step.stop"}`,
		`{"index":1,"step":{"id":"call_7","type":"function_call","name":"n","arguments":{}},"event_type":"step.start"}`,
		`{"index":1,"event_type":"step.stop"}`,
		`{"index":2,"step":{"id":"call_7","type":"function_call","name":"n","arguments":{}},"event_type":"step.start"}`,
		`{"index":2,"event_type":"step.stop"}`,
	}
	ids := func(base string) []string {
		c := newConverter(testModel, base, 0, 0)
		var out []string
		for _, f := range frames {
			events, _ := c.Convert(lib.Frame{Data: []byte(f)})
			for _, e := range events {
				if tl, ok := e.(api.EventTool); ok && tl.ID != "" {
					out = append(out, tl.ID)
				}
			}
		}
		return out
	}
	if got := ids("gemini_aaa"); strings.Join(got, ",") != "gemini_aaa_call_0,call_7,gemini_aaa_call_2" {
		t.Fatalf("ids %v", got)
	}
	if a, b := ids("gemini_aaa"), ids("gemini_bbb"); a[0] == b[0] || a[2] == b[2] {
		t.Fatalf("two bases minted the same ids: %v %v", a, b)
	}
	body := opened() + startStep(0, `{"id":"","type":"function_call","name":"n","arguments":{}}`) + stopStep(0) + completed("requires_action") + done
	p := serve(t, sseServer(body))
	first := collect(t, p, turn("hi"))[1].(api.EventTool).ID
	second := collect(t, p, turn("hi"))[1].(api.EventTool).ID
	if !strings.HasPrefix(first, "gemini_") || first == second {
		t.Fatalf("two turns minted %q and %q", first, second)
	}
}

func TestBrokenKnownFramesFail(t *testing.T) {
	for _, c := range []struct {
		name   string
		frames []string
		want   string
	}{
		{"delta before its start", []string{created, `{"index":0,"delta":{"text":"x","type":"text"},"event_type":"step.delta"}`}, "gemini: step.delta for step 0, which never started"},
		{"stop for an unknown step", []string{created, `{"index":3,"event_type":"step.stop"}`}, "gemini: step.stop for step 3, which never started"},
		{"double start", []string{created, `{"index":0,"step":{"type":"thought"},"event_type":"step.start"}`, `{"index":0,"step":{"type":"thought"},"event_type":"step.start"}`}, "gemini: step.start for open step 0"},
		{"missing index", []string{created, `{"step":{"type":"thought"},"event_type":"step.start"}`}, "gemini: step.start without an index"},
		{"start without a type", []string{created, `{"index":0,"step":{},"event_type":"step.start"}`}, "gemini: step.start 0 without a step type"},
		{"delta without a type", []string{created, `{"index":0,"step":{"type":"thought"},"event_type":"step.start"}`, `{"index":0,"delta":{},"event_type":"step.delta"}`}, "gemini: step.delta 0 without a delta type"},
		{"text delta on a call", []string{created, `{"index":0,"step":{"id":"a","type":"function_call","name":"n"},"event_type":"step.start"}`, `{"index":0,"delta":{"text":"x","type":"text"},"event_type":"step.delta"}`}, "gemini: text delta on a function_call step at 0"},
		{"signature on model output", []string{created, `{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`, `{"index":0,"delta":{"signature":"x","type":"thought_signature"},"event_type":"step.delta"}`}, "gemini: thought_signature delta on a model_output step at 0"},
		{"start arguments as a string", []string{created, `{"index":0,"step":{"id":"a","type":"function_call","name":"n","arguments":"{\"x\":1}"},"event_type":"step.start"}`}, "gemini: step.start 0 with arguments that are not an object"},
		{"start arguments as an array", []string{created, `{"index":0,"step":{"id":"a","type":"function_call","name":"n","arguments":[1]},"event_type":"step.start"}`}, "gemini: step.start 0 with arguments that are not an object"},
		{"arguments as an object", []string{created, `{"index":0,"step":{"id":"a","type":"function_call","name":"n"},"event_type":"step.start"}`, `{"index":0,"delta":{"arguments":{"a":1},"type":"arguments_delta"},"event_type":"step.delta"}`}, "gemini: stream frame did not parse"},
		{"missing event_type", []string{created, `{"index":0}`}, `gemini: "" frame without an event_type`},
		{"unparseable", []string{created, `{"event_type":`}, "gemini: stream frame did not parse"},
		{"not an object", []string{created, `[1]`}, "gemini: stream frame did not parse"},
		{"second created", []string{created, created}, "gemini: a second interaction.created"},
		{"created without an interaction", []string{`{"event_type":"interaction.created"}`}, "gemini: interaction.created without an interaction"},
		{"completed without an interaction", []string{created, `{"event_type":"interaction.completed"}`}, "gemini: interaction.completed without an interaction"},
		{"completed with open steps", []string{created, `{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`, `{"interaction":{"status":"completed"},"event_type":"interaction.completed"}`}, "gemini: interaction.completed with 1 steps still open"},
		{"done before completed", []string{created, `[DONE]`}, "gemini: the stream ended without interaction.completed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			events := convertAll(c.frames...)
			e, ok := events[len(events)-1].(api.EventErr)
			if !ok || e.Code != api.CodeNetwork || !strings.HasPrefix(e.Err.Error(), c.want) {
				t.Fatalf("ended %s, want err network %q", conform.Describe(events[len(events)-1]), c.want)
			}
			conform.CheckGrammar(t, events)
		})
	}
}

func TestStatusValues(t *testing.T) {
	for _, c := range []struct {
		status string
		code   api.ErrCode
	}{
		{"incomplete", api.CodeCapped},
		{"requires_continuation", api.CodeCapped},
		{"failed", api.CodeUnknown},
		{"cancelled", api.CodeUnknown},
		{"in_progress", api.CodeUnknown},
		{"brand_new", api.CodeUnknown},
	} {
		events := convertAll(created, `{"interaction":{"status":"`+c.status+`"},"event_type":"interaction.completed"}`)
		e, ok := events[len(events)-1].(api.EventErr)
		var a *APIError
		if !ok || e.Code != c.code || !errors.As(e.Err, &a) || a.Code != c.status || a.Status != 0 || a.Provider != "gemini" {
			t.Fatalf("%s ended %s, want %s with the status as the APIError code", c.status, conform.Describe(events[len(events)-1]), c.code)
		}
	}
	for _, status := range []string{"completed", "requires_action"} {
		wantTrace(t, convertAll(created, `{"interaction":{"status":"`+status+`"},"event_type":"interaction.completed"}`), "start  gemini-4-hydrogen", "ok turn")
	}
	wantTrace(t, convertAll(created, `{"interaction":{},"event_type":"interaction.completed"}`), "start  gemini-4-hydrogen", "err network gemini: interaction.completed without a status")
	wantTrace(t, convertAll(created, `{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`, `{"interaction":{"status":"incomplete"},"event_type":"interaction.completed"}`),
		"start  gemini-4-hydrogen", "err capped")
}

func TestToolArgumentsThatNeverParse(t *testing.T) {
	sse := opened() + startStep(0, `{"id":"call_t","type":"function_call","name":"search","arguments":{}}`) +
		deltaStep(0, `{"arguments":"{\"q\": \"San Fr","type":"arguments_delta"}`) + completed("incomplete") + done
	events := collect(t, serve(t, sseServer(sse)), turn("search"))
	conform.CheckGrammar(t, events)
	wantTrace(t, events, "start  gemini-4-hydrogen", `tool 0 "call_t" "search" ""`, `tool 0 "" "" "{\"q\": \"San Fr"`, usageLine, "err capped")
	logs := countLogs(t)
	var raw strings.Builder
	b := response.NewBulder()
	for _, e := range events {
		if tl, ok := e.(api.EventTool); ok {
			raw.WriteString(tl.Args)
		}
		if _, ok := b.On(e); !ok {
			t.Fatalf("builder refused %s", conform.Describe(e))
		}
	}
	if raw.String() != `{"q": "San Fr` {
		t.Fatalf("arguments passed through as %q", raw.String())
	}
	resp := b.Flush()
	if code, ok := resp.Err(); !ok || code != api.CodeCapped {
		t.Fatalf("builder ended the turn %v, want capped", resp.Cause)
	}
	use, ok := resp.Content[0].(api.ContentToolUse)
	if !ok || use.ID != "call_t" || !bytes.Equal(use.Input, api.DiscardedArguments) || logs[slog.LevelWarn] != 1 {
		t.Fatalf("builder stored %#v with %d warnings, want the call with its arguments discarded and one warning", resp.Content[0], logs[slog.LevelWarn])
	}
	got := steps(t, api.Request{Messages: []api.Message{
		user(text("search")),
		assistant(use),
		user(api.NewContentToolResult(use.ID, nil, api.DiscardedArgumentsError{Raw: raw.String()})),
	}})
	if !strings.Contains(got, `"name":"search","arguments":{}`) || !strings.Contains(got, `"is_error":true`) || !strings.Contains(got, `San Fr`) {
		t.Fatalf("the discarded call went back as %s", got)
	}
}

func TestStreamEndsAtTerminalFrame(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened()+completed("completed"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	wantTrace(t, collect(t, p, turn("hi")), "start  gemini-4-hydrogen", usageLine, "ok turn")
}

func TestStreamKeepalivesDoNotHoldTheTurnOpen(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened()+completed("completed"))
		w.(http.Flusher).Flush()
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				io.WriteString(w, ": keep-alive\n\n"+startStep(9, `{"type":"model_output"}`))
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	wantTrace(t, collect(t, p, turn("hi")), "start  gemini-4-hydrogen", usageLine, "ok turn")
}

func TestStreamDisconnectBeforeCompletedIsSilent(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened()+startStep(0, `{"type":"model_output"}`)+deltaStep(0, `{"text":"cut","type":"text"}`)+"event: step.delta\ndata: {\"ind")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	wantTrace(t, collect(t, p, turn("hi")), "start  gemini-4-hydrogen", `text 0 "cut"`)
}

func TestInBandErrorKeepsTheContent(t *testing.T) {
	sse := opened() + startStep(0, `{"type":"model_output"}`) + deltaStep(0, `{"text":"half","type":"text"}`) +
		ev(`{"error":{"message":"Deadline expired before operation could complete.","code":"gateway_timeout"},"event_type":"error"}`) + done
	events := collect(t, serve(t, sseServer(sse)), turn("hi"))
	conform.CheckGrammar(t, events)
	wantTrace(t, events, "start  gemini-4-hydrogen", `text 0 "half"`, "err network gemini: gateway_timeout: Deadline expired before operation could complete.")
}

func TestIdleTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	held := func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened())
		w.(http.Flusher).Flush()
		held(w, r)
	}, WithTransport(lib.Options{Idle: 50 * time.Millisecond}))
	events := collect(t, p, turn("hi"))
	wantTrace(t, events, "start  gemini-4-hydrogen", "err network")
	if !errors.Is(events[1].(api.EventErr).Err, lib.ErrIdle) {
		t.Fatalf("err %v, want lib.ErrIdle", events[1].(api.EventErr).Err)
	}
	p = serve(t, held, WithTransport(lib.Options{Idle: 50 * time.Millisecond}))
	wantTrace(t, collect(t, p, turn("hi")), "err network")
}

func TestKeepaliveHoldsTheDeadlineOpen(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for range 12 {
			io.WriteString(w, ": keep-alive\n\n")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
		io.WriteString(w, opened()+startStep(0, `{"type":"model_output"}`)+deltaStep(0, `{"text":"late","type":"text"}`)+stopStep(0)+completed("completed")+done)
	}, WithTransport(lib.Options{Idle: 200 * time.Millisecond}))
	wantTrace(t, collect(t, p, turn("hi")), "start  gemini-4-hydrogen", `text 0 "late"`, "close 0", usageLine, "ok turn")
}

func TestCancelMidStream(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened()+startStep(0, `{"type":"model_output"}`)+deltaStep(0, `{"text":"one","type":"text"}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []api.Event
	for e := range p.Stream(ctx, turn("hi")) {
		events = append(events, e)
		if _, ok := e.(api.EventText); ok {
			cancel()
		}
	}
	wantTrace(t, events, "start  gemini-4-hydrogen", `text 0 "one"`)
}

func TestCancelWhileParkedOnASend(t *testing.T) {
	sse := opened() + startStep(0, `{"type":"model_output"}`) + deltaStep(0, `{"text":"one","type":"text"}`) + deltaStep(0, `{"text":"two","type":"text"}`) + stopStep(0) + completed("completed") + done
	p := serve(t, sseServer(sse))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := p.Stream(ctx, turn("hi"))
	events := []api.Event{<-ch}
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	for e := range ch {
		events = append(events, e)
	}
	conform.CheckGrammar(t, events)
	for _, e := range events {
		if e, ok := e.(api.EventErr); ok {
			t.Fatalf("a cancelled turn reported %s", e.Code)
		}
	}
}

func TestCancelWithoutReading(t *testing.T) {
	sse := opened() + startStep(0, `{"type":"model_output"}`) + deltaStep(0, `{"text":"one","type":"text"}`) + stopStep(0) + completed("completed") + done
	p := serve(t, sseServer(sse))
	ctx, cancel := context.WithCancel(context.Background())
	ch := p.Stream(ctx, turn("hi"))
	time.Sleep(50 * time.Millisecond)
	cancel()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for range ch {
		}
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream goroutine did not exit after cancellation")
	}
}

func TestCallerDeadlineIsSilent(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened())
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var events []api.Event
	for e := range p.Stream(ctx, turn("hi")) {
		events = append(events, e)
	}
	wantTrace(t, events, "start  gemini-4-hydrogen")
}

func TestLargeAndMultilineFrames(t *testing.T) {
	big := strings.Repeat("lorem ipsum <&> ", 8000)
	delta, _ := json.Marshal(map[string]any{"index": 0, "delta": map[string]any{"type": "text", "text": big}, "event_type": "step.delta"})
	sse := opened() + startStep(0, `{"type":"model_output"}`) + "event: step.delta\ndata: " + string(delta) + "\n\n" +
		"event: step.delta\ndata: {\"index\":0,\ndata: \"delta\":{\"text\":\"!\",\"type\":\"text\"},\ndata: \"event_type\":\"step.delta\"}\n\n" +
		stopStep(0) + completed("completed") + done
	events := collect(t, serve(t, sseServer(sse)), turn("hi"))
	conform.CheckGrammar(t, events)
	if len(events) != 6 || events[1].(api.EventText).Text != big || events[2].(api.EventText).Text != "!" {
		t.Fatalf("got %d events: %.300v", len(events), conform.DescribeAll(events))
	}
}

func TestFrameOverLimit(t *testing.T) {
	big, _ := json.Marshal(map[string]any{"index": 0, "delta": map[string]any{"type": "text", "text": strings.Repeat("x", 4096)}, "event_type": "step.delta"})
	sse := opened() + startStep(0, `{"type":"model_output"}`) + "event: step.delta\ndata: " + string(big) + "\n\n" + stopStep(0) + completed("completed") + done
	events := collect(t, serve(t, sseServer(sse), WithTransport(lib.Options{Max: 1024})), turn("hi"))
	conform.CheckGrammar(t, events)
	wantTrace(t, events, "start  gemini-4-hydrogen", "err network")
}

func TestTransportTimeoutsAreNetworkErrors(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	held := func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}
	dial := &http.Client{Transport: &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded}
	}}}
	wantTrace(t, collect(t, serve(t, held, WithHTTPClient(dial)), turn("hi")), "err network")
	headers := &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond}}
	wantTrace(t, collect(t, serve(t, held, WithHTTPClient(headers)), turn("hi")), "err network")
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, opened())
		w.(http.Flusher).Flush()
		held(w, r)
	}, WithHTTPClient(&http.Client{Timeout: 200 * time.Millisecond}))
	wantTrace(t, collect(t, p, turn("hi")), "start  gemini-4-hydrogen", "err network")
}

func TestUnreachableHost(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	p := mustNew(t, Conf{Model: Hydrogen4}, WithAPIKey("k"), WithBaseURL(url))
	wantTrace(t, collect(t, p, turn("hi")), "err network")
}

func TestEncodeFailureIsATerminalEvent(t *testing.T) {
	events := collect(t, encoder(t), api.Request{})
	wantTrace(t, events, "err invalid gemini: request has no messages")
}

func TestHTTPErrors(t *testing.T) {
	rpc := `[{
  "error": {
    "code": 400,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {"@type": "type.googleapis.com/google.rpc.LocalizedMessage", "locale": "en-US", "message": "x"},
      {"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "API_KEY_INVALID", "domain": "googleapis.com"}
    ]
  }
}
]`
	for _, c := range []struct {
		name    string
		status  int
		header  http.Header
		body    string
		code    api.ErrCode
		typ     string
		errCode string
		message string
	}{
		{"interactions object", 404, nil, `{"error":{"message":"Model 'x' not found.","code":"not_found"}}`, api.CodeInvalid, "", "not_found", "Model 'x' not found."},
		{"sse framed", 400, nil, "event: error\ndata: {\"error\":{\"message\":\"Request contains an invalid argument.\",\"code\":\"invalid_request\"},\"event_type\":\"error\"}\n\n", api.CodeInvalid, "", "invalid_request", "Request contains an invalid argument."},
		{"rpc array bad key", 400, nil, rpc, api.CodeAccess, "INVALID_ARGUMENT", "API_KEY_INVALID", "API key not valid. Please pass a valid API key."},
		{"rpc array no key", 403, nil, `[{"error":{"code":403,"message":"Permission denied on the service.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"ACCESS_DENIED"}]}}]`, api.CodeAccess, "PERMISSION_DENIED", "ACCESS_DENIED", "Permission denied on the service."},
		{"rpc object", 400, nil, `{"error":{"code":400,"message":"bad","status":"INVALID_ARGUMENT"}}`, api.CodeInvalid, "INVALID_ARGUMENT", "", "bad"},
		{"html", 502, nil, "<html>bad gateway</html>", api.CodeNetwork, "", "", "<html>bad gateway</html>"},
		{"empty", 500, nil, "", api.CodeNetwork, "", "", ""},
		{"json without an error", 400, nil, `{"interaction":{}}`, api.CodeInvalid, "", "", `{"interaction":{}}`},
		{"failed precondition", 400, nil, `{"error":{"message":"m","code":"failed_precondition"}}`, api.CodeAccess, "", "failed_precondition", "m"},
		{"safety", 400, nil, `{"error":{"message":"m","code":"prohibited_content"}}`, api.CodeRefused, "", "prohibited_content", "m"},
		{"context overflow", 400, nil, `{"error":{"message":"The input token count (1210970) exceeds the maximum number of tokens allowed (1048576).","code":"invalid_request"}}`, api.CodeContextLength, "", "invalid_request", "The input token count (1210970) exceeds the maximum number of tokens allowed (1048576)."},
		{"context overflow, other phrasing", 400, nil, `{"error":{"message":"the input token count is 1200000 but model only supports up to 1048576","code":"invalid_request"}}`, api.CodeContextLength, "", "invalid_request", "the input token count is 1200000 but model only supports up to 1048576"},
		{"tokens without overflow", 400, nil, `{"error":{"message":"max_output_tokens must be positive; token count unknown","code":"invalid_request"}}`, api.CodeInvalid, "", "invalid_request", "max_output_tokens must be positive; token count unknown"},
		{"aborted", 409, nil, `{"error":{"message":"m","code":"aborted"}}`, api.CodeNetwork, "", "aborted", "m"},
		{"already exists", 409, nil, `{"error":{"message":"m","code":"already_exists"}}`, api.CodeInvalid, "", "already_exists", "m"},
		{"timeout 408", 408, nil, `x`, api.CodeNetwork, "", "", "x"},
		{"unimplemented", 501, nil, `{"error":{"message":"m","code":"unimplemented"}}`, api.CodeInvalid, "", "unimplemented", "m"},
		{"rate limit", 429, http.Header{"Retry-After": {"7"}}, `{"error":{"message":"slow","code":"rate_limit_exceeded"}}`, api.CodeBusy, "", "rate_limit_exceeded", "slow"},
		{"per minute", 429, nil, `{"error":{"message":"Rate limit exceeded for model gemini-4-hydrogen (limit: 5 requests per minute).","code":"too_many_requests"}}`, api.CodeBusy, "", "too_many_requests", "Rate limit exceeded for model gemini-4-hydrogen (limit: 5 requests per minute)."},
		{"quota exceeded", 429, nil, `{"error":{"message":"You have exceeded your daily quota.","code":"quota_exceeded"}}`, api.CodeAccess, "", "quota_exceeded", "You have exceeded your daily quota."},
		{"daily quota", 429, http.Header{"Retry-After": {"49314"}}, `{"error":{"message":"Rate limit exceeded for model gemini-4-hydrogen (limit: 20 requests per day on Free Tier). Please retry in 13h41m54s or upgrade your tier at https://ai.dev/rate-limit.","code":"too_many_requests"}}`, api.CodeAccess, "", "too_many_requests", "Rate limit exceeded for model gemini-4-hydrogen (limit: 20 requests per day on Free Tier). Please retry in 13h41m54s or upgrade your tier at https://ai.dev/rate-limit."},
		{"free tier locked", 429, http.Header{"Retry-After": {"46247"}}, `{"error":{"message":"Rate limit exceeded for model gemini-4-neon (limit: 0 requests per day on Free Tier). Please upgrade your tier at https://ai.dev/rate-limit.","code":"too_many_requests"}}`, api.CodeAccess, "", "too_many_requests", "Rate limit exceeded for model gemini-4-neon (limit: 0 requests per day on Free Tier). Please upgrade your tier at https://ai.dev/rate-limit."},
		{"overloaded", 503, http.Header{"Retry-After": {"30"}}, `{"error":{"message":"high demand","code":"service_unavailable"}}`, api.CodeBusy, "", "service_unavailable", "high demand"},
		{"teapot", 418, nil, `{"error":{"message":"m","code":"x"}}`, api.CodeUnknown, "", "x", "m"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				for k, v := range c.header {
					w.Header()[k] = v
				}
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(c.status)
				io.WriteString(w, c.body)
			})
			events := collect(t, p, turn("hi"))
			if len(events) != 1 {
				t.Fatalf("%v, want one EventErr", conform.DescribeAll(events))
			}
			e, ok := events[0].(api.EventErr)
			var a *APIError
			if !ok || e.Code != c.code || !errors.As(e.Err, &a) {
				t.Fatalf("%s, want EventErr %s carrying an APIError", conform.Describe(events[0]), c.code)
			}
			if a.Provider != "gemini" || a.Status != c.status || a.Type != c.typ || a.Code != c.errCode || a.Message != c.message {
				t.Fatalf("APIError %+v, want status %d type %q code %q message %q", *a, c.status, c.typ, c.errCode, c.message)
			}
			if want := lib.RetryAfter(c.header); a.RetryAfter != want {
				t.Fatalf("RetryAfter %v, want %v", a.RetryAfter, want)
			}
		})
	}
}

func TestUnparseableErrorBodyIsCut(t *testing.T) {
	e := readAPIError(&lib.HTTPError{Status: 502, Body: []byte(strings.Repeat("é", 2000))})
	if n := len([]rune(e.Message)); n != 512+1 || !strings.HasSuffix(e.Message, "…") {
		t.Fatalf("message has %d runes, want 512 and the cut mark", n)
	}
}

func TestSizeRefusal(t *testing.T) {
	const body = `{"error":{"message":"Request payload size exceeds the limit: 104857600 bytes.","code":"invalid_request"}}`
	for _, c := range []struct {
		declared int
		measured bool
	}{{100_000_000, true}, {0, false}} {
		conv := newConverter(testModel, "gemini_test", 5000, c.declared)
		e := conv.End(&lib.HTTPError{Status: 400, Body: []byte(body)})[0].(api.EventErr)
		if e.Code != api.CodeTooLarge || strings.Contains(e.Err.Error(), "sent 5000 bytes") != c.measured {
			t.Fatalf("declared %d: %s, want too_large naming the bytes sent %v", c.declared, conform.Describe(e), c.measured)
		}
		conv = newConverter(testModel, "gemini_test", 5000, c.declared)
		conv.Convert(lib.Frame{Data: []byte(created)})
		events, more := conv.Convert(lib.Frame{Data: []byte(`{"error":{"message":"Request payload size exceeds the limit: 104857600 bytes.","code":"invalid_request"},"event_type":"error"}`)})
		e = events[0].(api.EventErr)
		if more || e.Code != api.CodeTooLarge || strings.Contains(e.Err.Error(), "sent 5000 bytes") != c.measured {
			t.Fatalf("in-band, declared %d: %s, want too_large naming the bytes sent %v", c.declared, conform.Describe(e), c.measured)
		}
	}
}

func TestInBandCodes(t *testing.T) {
	for _, c := range []struct {
		code, message string
		want          api.ErrCode
	}{
		{"service_unavailable", "m", api.CodeBusy},
		{"too_many_requests", "Rate limit exceeded (limit: 20 requests per day on Free Tier).", api.CodeAccess},
		{"too_many_requests", "Rate limit exceeded (limit: 0 requests per day on Free Tier).", api.CodeAccess},
		{"quota_exceeded", "You have exceeded your daily quota.", api.CodeAccess},
		{"rate_limit_exceeded", "slow down", api.CodeBusy},
		{"invalid_request", "The input token count (2000000) exceeds the maximum number of tokens allowed (1048576).", api.CodeContextLength},
		{"invalid_request", "Request contains an invalid argument.", api.CodeInvalid},
		{"gateway_timeout", "Deadline expired before operation could complete.", api.CodeNetwork},
		{"aborted", "m", api.CodeNetwork},
		{"safety", "m", api.CodeRefused},
		{"image_safety", "m", api.CodeRefused},
		{"malformed_function_call", "m", api.CodeUnknown},
		{"cancelled", "m", api.CodeUnknown},
		{"", "", api.CodeUnknown},
	} {
		events := convertAll(created, fmt.Sprintf(`{"error":{"message":%q,"code":%q},"event_type":"error"}`, c.message, c.code))
		if e, ok := events[len(events)-1].(api.EventErr); !ok || e.Code != c.want {
			t.Fatalf("%s %q ended %s, want %s", c.code, c.message, conform.Describe(events[len(events)-1]), c.want)
		}
	}
	events := convertAll(created, `{"event_type":"error"}`)
	if e := events[1].(api.EventErr); e.Code != api.CodeUnknown || e.Err.Error() != "gemini: no message" {
		t.Fatalf("an empty error event ended %s", conform.Describe(e))
	}
}

func TestEndings(t *testing.T) {
	for _, c := range []struct {
		err  error
		want []string
	}{
		{io.EOF, nil},
		{io.ErrUnexpectedEOF, nil},
		{context.Canceled, nil},
		{context.DeadlineExceeded, nil},
		{fmt.Errorf("dial tcp: %w", context.DeadlineExceeded), []string{"err network dial tcp: context deadline exceeded"}},
		{fmt.Errorf("read: %w", context.Canceled), []string{"err network read: context canceled"}},
		{lib.ErrIdle, []string{"err network neui: stream went idle"}},
		{errors.New("boom"), []string{"err network boom"}},
	} {
		got := conform.DescribeAll(newConverter(testModel, "b", 0, 0).End(c.err))
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Fatalf("End(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
