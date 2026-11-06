package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func TestStreamText(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"9.11"}`, ""),
		chunkFrame(`{"content":" is larger."}`, ""),
		chunkFrame(`{}`, "stop"),
		usageFrame,
		doneFrame,
	))
	events := collect(t, p, turn("9.11 or 9.8?"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "9.11"`,
		`text 0 " is larger."`,
		"usage in=3000 out=350 reasoning=600 read=9000 write=0",
		"ok turn",
	)
}

// The opening delta carries content as an empty string. Opening a block on it
// would put an empty text block in front of every reply.
func TestStreamEmptyOpeningDeltaOpensNoBlock(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"","reasoning_content":null}`, ""),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events, "start chatcmpl-1 deepseek-v4-pro", "ok turn")
}

func TestStreamToolCall(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":null,"tool_calls":[{"index":0,"id":"call_0_9c1f","type":"function","function":{"name":"get_weather","arguments":""}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris\"}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		usageFrame,
		doneFrame,
	))
	events := collect(t, p, turn("weather in Paris?"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`tool 0 id="call_0_9c1f" name="get_weather" args=""`,
		fmt.Sprintf(`tool 0 id="" name="" args=%q`, `{"location":`),
		fmt.Sprintf(`tool 0 id="" name="" args=%q`, `"Paris"}`),
		"usage in=3000 out=350 reasoning=600 read=9000 write=0",
		"ok turn",
	)
}

// Each tool_calls index is a stream of its own and takes a block index the
// first time it is seen. Nothing on this wire says one call ended before the
// message did, so none of the three is closed and the terminal event is what
// completes all of them. The second id repeats the first, which is reported for
// v4-pro, and the third has none: both are minted rather than passed on, since
// two blocks may not claim one handle.
func TestStreamSeveralToolCalls(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_dup","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":1,"id":"call_dup","type":"function","function":{"name":"get_time","arguments":"{}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":2,"id":"","type":"function","function":{"name":"get_news","arguments":"{}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("three things"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		fmt.Sprintf(`tool 0 id="call_dup" name="get_weather" args=%q`, `{"city":"Paris"}`),
		`tool 1 id="chatcmpl-1_call_1" name="get_time" args="{}"`,
		`tool 2 id="chatcmpl-1_call_2" name="get_news" args="{}"`,
		"ok turn",
	)
}

// Fragments of two calls arriving alternately. tool_calls[].index is what links
// a fragment to its call, so each one fills the block its index was given and
// neither accumulation is split by the other landing between its pieces.
func TestStreamToolCallFragmentsInterleave(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"get_time","arguments":"{\"zone\":"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris\"}"}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":1,"function":{"arguments":"\"CET\"}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("weather and time"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		fmt.Sprintf(`tool 0 id="call_a" name="get_weather" args=%q`, `{"city":`),
		fmt.Sprintf(`tool 1 id="call_b" name="get_time" args=%q`, `{"zone":`),
		fmt.Sprintf(`tool 0 id="" name="" args=%q`, `"Paris"}`),
		fmt.Sprintf(`tool 1 id="" name="" args=%q`, `"CET"}`),
		"ok turn",
	)
}

// A whole call in one fragment, with the index left off. Absent reads as the
// first call, which is what it is.
func TestStreamToolCallWithoutIndex(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"tool_calls":[{"id":"call_x","type":"function","function":{"name":"ping","arguments":"{}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("ping"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`tool 0 id="call_x" name="ping" args="{}"`,
		"ok turn",
	)
}

// Reasoning is plain text with no signature and nothing sealed, so it is
// ContentReasoning and never ContentOpaque. v4 resumes it after text has
// started, and can carry both fields on one delta, so the two streams
// interleave: content and reasoning_content each keep the index they were first
// seen at, and one train of thought is one block however often text lands in
// the middle of it.
func TestStreamReasoning(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":null,"reasoning_content":"Let me compare"}`, ""),
		chunkFrame(`{"reasoning_content":" the decimals"}`, ""),
		chunkFrame(`{"content":"9.11"}`, ""),
		chunkFrame(`{"content":" wins","reasoning_content":" actually, hold on"}`, ""),
		chunkFrame(`{}`, "stop"),
		usageFrame,
		doneFrame,
	))
	events := collect(t, p, turn("9.11 or 9.8?"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`thinking 0 part=0 "Let me compare"`,
		`thinking 0 part=0 " the decimals"`,
		`text 1 "9.11"`,
		`thinking 0 part=0 " actually, hold on"`,
		`text 1 " wins"`,
		"usage in=3000 out=350 reasoning=600 read=9000 write=0",
		"ok turn",
	)
}

// All three kinds of stream in one turn. Each takes a block the first time it
// is seen, so the call lands on index 2 rather than on a count of its own, and
// the handle minted for it is minted from that index.
func TestStreamReasoningTextAndToolCallEachTakeABlock(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"reasoning_content":"which city"}`, ""),
		chunkFrame(`{"content":"looking it up"}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"type":"function","function":{"name":"get_weather","arguments":"{}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("weather?"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`thinking 0 part=0 "which city"`,
		`text 1 "looking it up"`,
		`tool 2 id="chatcmpl-1_call_2" name="get_weather" args="{}"`,
		"ok turn",
	)
}

// A fragment carrying only an id has no function object at all, which is why
// toolDelta.Function is a pointer, so a call's name can start arriving after
// its id and nothing says it arrives whole — and the pieces can land the other
// way round too. Every piece goes out as it lands: api.EventTool appends
// Name and Args and replaces ID, so a name in two halves is two events, an id
// that turns up late replaces the minted one, and the call is whole at the
// terminal event and not before.
func TestStreamToolCallIdentityArrivesInPieces(t *testing.T) {
	tests := []struct {
		name   string
		frames []string
		want   []string
	}{
		{
			name: "name on the fragment after the id",
			frames: []string{
				`{"tool_calls":[{"index":0,"id":"call_a","type":"function"}]}`,
				`{"tool_calls":[{"index":0,"function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]}`,
			},
			want: []string{
				`tool 0 id="call_a" name="" args=""`,
				fmt.Sprintf(`tool 0 id="" name="get_weather" args=%q`, `{"city":"Paris"}`),
			},
		},
		{
			name: "name split across two fragments",
			frames: []string{
				`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_"}}]}`,
				`{"tool_calls":[{"index":0,"function":{"name":"weather","arguments":"{\"city\":\"Paris\"}"}}]}`,
			},
			want: []string{
				`tool 0 id="call_a" name="get_" args=""`,
				fmt.Sprintf(`tool 0 id="" name="weather" args=%q`, `{"city":"Paris"}`),
			},
		},
		{
			// Arguments before the call has any identity. The block opens on
			// the fragment that mentions it first and carries a minted handle
			// until the real one turns up; EventTool.ID replaces, so the real
			// id goes out rather than being dropped for a handle DeepSeek
			// never issued.
			name: "the id arrives after the arguments",
			frames: []string{
				`{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}`,
				`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"\"Paris\"}"}}]}`,
			},
			want: []string{
				fmt.Sprintf(`tool 0 id="chatcmpl-1_call_0" name="" args=%q`, `{"city":`),
				fmt.Sprintf(`tool 0 id="call_a" name="get_weather" args=%q`, `"Paris"}`),
			},
		},
		{
			// No arguments ever arrive. The call is still a block, and the
			// terminal event is what completes it.
			name: "no argument fragment at all",
			frames: []string{
				`{"tool_calls":[{"index":0,"id":"call_a","type":"function"}]}`,
				`{"tool_calls":[{"index":0,"function":{"name":"ping"}}]}`,
			},
			want: []string{
				`tool 0 id="call_a" name="" args=""`,
				`tool 0 id="" name="ping" args=""`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frames := []string{chunkFrame(openingFrame, "")}
			for _, f := range tc.frames {
				frames = append(frames, chunkFrame(f, ""))
			}
			frames = append(frames, chunkFrame(`{}`, "tool_calls"), doneFrame)

			events := collect(t, serve(t, stringHandler(frames...)), turn("weather in Paris?"))
			checkGrammar(t, events)
			want := append([]string{"start chatcmpl-1 deepseek-v4-pro"}, tc.want...)
			wantTrace(t, events, append(want, "ok turn")...)
		})
	}
}

// The same turn, sent back on the next request. Interleaved as it arrived, it
// folds to one text block and one reasoning block, which is the shape of a
// DeepSeek assistant message: one content field and one reasoning_content
// field. So replay hands back exactly what the model emitted, character for
// character, with nothing inserted where the two streams took turns.
//
// The request declares a tool, because that is the request whose reasoning
// goes back at all.
func TestReplayReproducesTheTurnByteForByte(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":null,"reasoning_content":"Let me compare"}`, ""),
		chunkFrame(`{"reasoning_content":" the decimals"}`, ""),
		chunkFrame(`{"content":"9.11"}`, ""),
		chunkFrame(`{"content":" wins","reasoning_content":" actually, hold on"}`, ""),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	events := collect(t, p, turn("9.11 or 9.8?"))
	checkGrammar(t, events)

	body := encodeBody(t, mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")), api.Request{
		Tools: []api.Tool{{Name: "note"}},
		Messages: []api.Message{
			{Role: api.RoleUser, Content: text("9.11 or 9.8?")},
			{Role: api.RoleAssistant, Content: fold(t, events)},
		},
	})
	m := body.Messages[1]
	if m.Content.(string) != "9.11 wins" {
		t.Fatalf("the model said %q and is replayed %q", "9.11 wins", m.Content.(string))
	}
	if want := "Let me compare the decimals actually, hold on"; m.ReasoningContent != want {
		t.Fatalf("reasoning_content %q, want %q", m.ReasoningContent, want)
	}
}

// Schemas go through unconstrained on this host, so a truncated argument
// accumulation is an ordinary outcome. The adapter passes the fragments through
// as they arrive and the cap that cut them ends the turn with CodeCapped, which
// is the caller's cue to raise the cap or continue rather than to store this.
//
// The second half is the other route the same accumulation takes: wherever a
// caller does end up holding one, the rule in protocol/content.go applies to it
// — it never parsed, so it is discarded rather than stored, and the call is
// answered with DiscardedArgumentsError so the model reissues it.
func TestStreamToolArgumentsThatNeverParse(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_trunc","type":"function","function":{"name":"search","arguments":""}}]}`, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\": \"San Franc"}}]}`, ""),
		chunkFrame(`{}`, "length"),
		doneFrame,
	))
	events := collect(t, p, turn("search"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`tool 0 id="call_trunc" name="search" args=""`,
		fmt.Sprintf(`tool 0 id="" name="" args=%q`, `{"query": "San Franc`),
		"err capped",
	)

	var raw strings.Builder
	var call api.ContentToolUse
	for _, e := range events {
		if e, ok := e.(api.EventTool); ok {
			if e.ID != "" {
				call.ID = e.ID
			}
			call.Name += e.Name
			raw.WriteString(e.Args)
		}
	}
	if json.Valid([]byte(raw.String())) {
		t.Fatal("fixture is supposed to be unparseable")
	}
	if call.Input != nil {
		t.Fatal("an accumulation that never parsed must not be stored")
	}

	// The discarded call goes back to the model on the next turn: the arguments
	// field is not optional, so it becomes the empty object, and the paired
	// tool message carries the error, since there is no error flag on the wire.
	body, err := mustNew(t, Conf{Model: api.Model{Name: "deepseek-v4-pro"}}, WithAPIKey("k")).encode(api.Request{
		Messages: []api.Message{
			{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "search"}}},
			{Role: api.RoleAssistant, Content: api.Contents{call}},
			{Role: api.RoleUser, Content: api.Contents{
				api.NewContentToolResult(call.ID, nil, api.DiscardedArgumentsError{Raw: raw.String()}),
			}},
		},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	assistant := body.Messages[1]
	if got := assistant.ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("discarded arguments replayed as %q, want %q", got, "{}")
	}
	result := body.Messages[2]
	if result.Role != "tool" {
		t.Fatalf("result replayed with role %q", result.Role)
	}
	if !strings.HasPrefix(result.Content.(string), "Error: tool call arguments were not valid JSON") {
		t.Fatalf("result content %q does not report the discard", result.Content.(string))
	}
	if !strings.Contains(result.Content.(string), `{"query": "San Franc`) {
		t.Fatalf("result content %q does not show the model what it botched", result.Content.(string))
	}
}

// A host may send usage in a frame of its own with an empty choices array, so a
// decoder that reaches for the first choice fails on the one frame that prices
// the turn.
func TestStreamUsageFrameHasNoChoices(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{}`, "stop"),
		usageFrame,
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		"usage in=3000 out=350 reasoning=600 read=9000 write=0",
		"ok turn",
	)
}

// v4-pro returns a completely empty successful turn after tool results, so a
// stream with no content at all is a completed turn, not a failure.
func TestStreamEmptyTurn(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(`{}`, "stop"),
		frame(`{"id":"chatcmpl-1","model":"deepseek-v4-pro","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10,"prompt_cache_hit_tokens":0,"prompt_cache_miss_tokens":10}}`),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		"usage in=10 out=0 reasoning=0 read=0 write=0",
		"ok turn",
	)
}

func TestUsageArithmetic(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want api.EventUsage
	}{
		{
			// The cache counters partition the prompt rather than sitting
			// beside it, and reasoning is part of the completion.
			name: "documented",
			raw:  `{"prompt_tokens":12000,"completion_tokens":950,"total_tokens":12950,"prompt_cache_hit_tokens":9000,"prompt_cache_miss_tokens":3000,"completion_tokens_details":{"reasoning_tokens":600}}`,
			want: api.EventUsage{Input: 3000, Output: 350, Reasoning: 600, CacheRead: 9000},
		},
		{
			name: "no cache counters",
			raw:  `{"prompt_tokens":500,"completion_tokens":40,"total_tokens":540}`,
			want: api.EventUsage{Input: 500, Output: 40},
		},
		{
			name: "no reasoning breakdown",
			raw:  `{"prompt_tokens":100,"completion_tokens":40,"prompt_cache_hit_tokens":64,"prompt_cache_miss_tokens":36}`,
			want: api.EventUsage{Input: 36, Output: 40, CacheRead: 64},
		},
		{
			// The identity is documented but never guaranteed, so the miss is
			// derived from the total and the invariant survives a disagreement.
			name: "counters disagree with the prompt",
			raw:  `{"prompt_tokens":100,"completion_tokens":10,"prompt_cache_hit_tokens":90,"prompt_cache_miss_tokens":5}`,
			want: api.EventUsage{Input: 10, Output: 10, CacheRead: 90},
		},
		{
			name: "cache read exceeds the prompt",
			raw:  `{"prompt_tokens":10,"completion_tokens":10,"prompt_cache_hit_tokens":90,"prompt_cache_miss_tokens":0}`,
			want: api.EventUsage{Input: 0, Output: 10, CacheRead: 90},
		},
		{
			name: "reasoning exceeds the completion",
			raw:  `{"prompt_tokens":10,"completion_tokens":5,"prompt_cache_miss_tokens":10,"completion_tokens_details":{"reasoning_tokens":9}}`,
			want: api.EventUsage{Input: 10, Output: 0, Reasoning: 9},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var u usage
			if err := json.Unmarshal([]byte(tc.raw), &u); err != nil {
				t.Fatal(err)
			}
			got := u.event()
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			if got.CacheWrite != 0 {
				t.Fatal("DeepSeek has no cache write tier, so CacheWrite must stay zero")
			}
			// The counters are disjoint, which is the whole point of doing the
			// arithmetic here: the two sides have to add back up.
			if u.Prompt > 0 && got.Input+got.CacheRead+got.CacheWrite != u.Prompt && got.CacheRead <= u.Prompt {
				t.Fatalf("prompt %d != Input+CacheRead+CacheWrite %d", u.Prompt, got.Input+got.CacheRead+got.CacheWrite)
			}
			if got.Output+got.Reasoning > u.Completion+got.Reasoning {
				t.Fatalf("generated side exceeds completion_tokens")
			}
		})
	}
}

// finish_reason decides which of the two terminal events ends the turn, split
// by whether the caller keeps what streamed rather than by how DeepSeek framed
// it. Every row emits the same partial text first: a turn that fails here is
// not a claim that nothing arrived, only that none of it is stored.
//
// A row that fails carries the wire value verbatim through Err, which for the
// unrecognized one is the only record of what the server actually said.
func TestFinishReasons(t *testing.T) {
	tests := []struct {
		native string
		want   string
	}{
		{"stop", "ok turn"},
		{"tool_calls", "ok turn"},
		{"length", "err capped"},
		{"content_filter", "err refused"},
		// A capacity failure delivered inside a 200, after the headers are
		// committed. The blocks already emitted are real, but the turn is not
		// one to store, and CodeBusy is what says it is worth retrying.
		{"insufficient_system_resource", "err busy"},
		// An interrupted generation with nothing wrong in the request: send it
		// again as sent.
		{"aborted", "err network"},
		// A value this adapter has not seen is never rounded to a completion:
		// it arrives unclassified so it cannot pass as a turn that finished.
		{"something_new", "err unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.native, func(t *testing.T) {
			p := serve(t, stringHandler(
				chunkFrame(openingFrame, ""),
				chunkFrame(`{"content":"partial"}`, ""),
				chunkFrame(`{}`, tc.native),
				doneFrame,
			))
			events := collect(t, p, turn("hi"))
			checkGrammar(t, events)
			wantTrace(t, events,
				"start chatcmpl-1 deepseek-v4-pro",
				`text 0 "partial"`,
				tc.want,
			)
			e, ok := events[2].(api.EventErr)
			if !ok {
				return
			}
			var finishErr *FinishError
			if !errors.As(e.Err, &finishErr) {
				t.Fatalf("err is %T, want *FinishError", e.Err)
			}
			if finishErr.Reason != tc.native {
				t.Fatalf("Reason %q, want the wire value %q", finishErr.Reason, tc.native)
			}
		})
	}
}

// Keep-alive comments hold the connection while a request waits for inference.
// They are not edits to the block list, so they produce nothing, and reading
// them is what keeps the idle deadline from firing on a live queued turn.
func TestStreamKeepAliveProducesNoEvents(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flush(w)
		for range 4 {
			io.WriteString(w, keepAlive)
			flush(w)
			time.Sleep(30 * time.Millisecond)
		}
		for _, f := range []string{
			chunkFrame(openingFrame, ""),
			chunkFrame(`{"content":"late"}`, ""),
			chunkFrame(`{}`, "stop"),
			doneFrame,
		} {
			io.WriteString(w, f)
			flush(w)
		}
	}, WithTransport(lib.Options{Idle: 3 * time.Second}))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "late"`,
		"ok turn",
	)
}

// The other framing shapes that carry nothing: a bare empty data line, which is
// a legal heartbeat, and an SSE field this stream has no use for. Neither is a
// frame that failed to parse.
func TestStreamIgnoresEmptyAndUnusedFrames(t *testing.T) {
	p := serve(t, stringHandler(
		"data:\n\n",
		"event: message\n",
		"id: 7\n\n",
		": waiting for a slot\n\n",
		chunkFrame(openingFrame, ""),
		"data: \n\n",
		chunkFrame(`{"content":"hi"}`, ""),
		chunkFrame(`{}`, "stop"),
		"data:[DONE]\n\n",
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "hi"`,
		"ok turn",
	)
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
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events, "start chatcmpl-1 deepseek-v4-pro", "err network")
}

// A frame can exceed bufio.Scanner's 64KB default, which is why the reader does
// not use one: a single argument fragment arrives whole at whatever size the
// server chose.
func TestStreamFrameLargerThanScannerLimit(t *testing.T) {
	big := strings.Repeat("a", 100_000)
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"tool_calls":[{"index":0,"id":"call_big","type":"function","function":{"name":"write","arguments":"{\"body\":\"`+big+`\"}"}}]}`, ""),
		chunkFrame(`{}`, "tool_calls"),
		doneFrame,
	))
	events := collect(t, p, turn("write"))
	checkGrammar(t, events)
	if len(events) != 3 {
		t.Fatalf("sequence %v", traces(events))
	}
	call, ok := events[1].(api.EventTool)
	if !ok {
		t.Fatalf("event 1 is %T", events[1])
	}
	if want := `{"body":"` + big + `"}`; call.Args != want {
		t.Fatalf("argument fragment truncated: got %d bytes, want %d", len(call.Args), len(want))
	}
}

func TestHTTPErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   api.ErrCode
	}{
		{"invalid format", 400, `{"error":{"message":"Invalid request body format.","type":"invalid_request_error","code":"invalid_request_error"}}`, api.CodeInvalid},
		{"auth", 401, `{"error":{"message":"Authentication fails due to the wrong API key.","code":"invalid_api_key"}}`, api.CodeAccess},
		// Insufficient balance: the credentials are fine and no edit to the
		// body helps, which is what puts it with the credentials rather than
		// with a bad request. Neither is retryable, so the practical cost is nil.
		{"insufficient balance", 402, `{"error":{"message":"You have run out of balance."}}`, api.CodeAccess},
		{"retired model", 404, `{"error":{"message":"Model Not Exist","code":"model_not_found"}}`, api.CodeInvalid},
		{"invalid parameters", 422, `{"error":{"message":"Your request contains invalid parameters."}}`, api.CodeInvalid},
		{"rate limit", 429, `{"error":{"message":"You are sending requests too quickly."}}`, api.CodeBusy},
		// 503 is the one 5xx DeepSeek names, and it names it overloaded, which
		// is a backoff. A 500 says nothing about capacity, so it goes out as
		// CodeNetwork and asks for the same request again.
		{"server error", 500, `{"error":{"message":"Our server encounters an issue."}}`, api.CodeNetwork},
		{"overloaded", 503, `{"error":{"message":"The server is overloaded due to high traffic."}}`, api.CodeBusy},
		// The body is not reliably JSON on failure, so the status decides.
		{"non json body", 400, "something went wrong", api.CodeInvalid},
		// A status nobody listed is unclassified rather than rounded to a bad
		// request: the body it would send a caller off to fix may be perfect.
		{"unlisted 4xx", 418, ``, api.CodeUnknown},
		{"unlisted 5xx", 502, ``, api.CodeNetwork},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			events := collect(t, p, turn("hi"))
			checkGrammar(t, events)
			wantTrace(t, events, "err "+string(tc.want))
			var apiErr *APIError
			if !errors.As(events[0].(api.EventErr).Err, &apiErr) {
				t.Fatalf("err is %T, want *APIError", events[0].(api.EventErr).Err)
			}
			if apiErr.Status != tc.status {
				t.Fatalf("status %d, want %d", apiErr.Status, tc.status)
			}
			if apiErr.Body != tc.body {
				t.Fatalf("body %q not kept verbatim", apiErr.Body)
			}
			if apiErr.RetryAfter != 7*time.Second {
				t.Fatalf("Retry-After %v, want 7s", apiErr.RetryAfter)
			}
		})
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"7", true},
		{"-1", false},
		{"nonsense", false},
		{time.Now().Add(time.Minute).UTC().Format(http.TimeFormat), true},
		{time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat), false},
	}
	for _, tc := range tests {
		h := http.Header{}
		if tc.header != "" {
			h.Set("Retry-After", tc.header)
		}
		if got := retryAfter(h) > 0; got != tc.want {
			t.Fatalf("Retry-After %q: got %v, want %v", tc.header, got, tc.want)
		}
	}
}

// DeepSeek documents no in band error frame at all, and the two shapes that
// circulate for OpenAI compatible hosts are unconfirmed for it. Neither is
// asserted to be what DeepSeek sends; what is asserted is that a frame carrying
// one ends the turn with a single EventErr instead of being folded into content
// or read as a completed turn.
func TestStreamInBandError(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		want  api.ErrCode
	}{
		{
			// The code is what classifies this, not the sentence beside it:
			// context_length_exceeded is a code and gets CodeContextLength,
			// while a host that only says so in prose gets whatever its code
			// says, which is the deliberate limit of this classification.
			"openai envelope",
			frame(`{"error":{"message":"This model's maximum context length is 1048576 tokens","type":"invalid_request_error","code":"context_length_exceeded"}}`),
			api.CodeContextLength,
		},
		{
			"vllm style pair",
			frame(`{"error_type":"rate_limit_exceeded","error_message":"upstream refused"}`),
			api.CodeBusy,
		},
		{
			// The vocabulary of a host in front of DeepSeek is open, so a code
			// nobody here knows is unclassified rather than assumed to be a cut
			// stream.
			"unrecognized code",
			frame(`{"error":{"message":"something gave out","code":"who_knows"}}`),
			api.CodeUnknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, stringHandler(
				chunkFrame(openingFrame, ""),
				chunkFrame(`{"content":"partial"}`, ""),
				tc.frame,
				doneFrame,
			))
			events := collect(t, p, turn("hi"))
			checkGrammar(t, events)
			wantTrace(t, events,
				"start chatcmpl-1 deepseek-v4-pro",
				`text 0 "partial"`,
				"err "+string(tc.want),
			)
			var streamErr *StreamError
			if !errors.As(events[2].(api.EventErr).Err, &streamErr) {
				t.Fatalf("err is %T, want *StreamError", events[2].(api.EventErr).Err)
			}
		})
	}
}

// A frame is allowed to carry a delta and an error at once, and the delta was
// generated before whatever went wrong. It goes out ahead of the failure that
// ends the turn rather than being dropped with the frame.
func TestStreamInBandErrorKeepsTheDeltaItRodeWith(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		frame(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,`+
			`"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"content":"half a sen"},"finish_reason":null}],`+
			`"error":{"message":"the server gave out","type":"server_error","code":"server_error"}}`),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "half a sen"`,
		"err network",
	)
}

// A frame that does not parse is a stream that stopped making sense, which is
// indistinguishable from one that was cut, so it fails as CodeNetwork and the
// same request may be sent again.
func TestStreamUnparseableFrame(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		"data: {not json\n\n",
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events, "start chatcmpl-1 deepseek-v4-pro", "err network")
}

// A socket that dies before the finish reason tells the adapter nothing about
// whether the reply finished, so it says nothing: the missing terminal event is
// the whole report. Which block was cut is not said and could not be, since no
// block on this wire is ever closed and an open one is the normal state.
func TestStreamDisconnectBeforeFinishReason(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, chunkFrame(openingFrame, ""))
		io.WriteString(w, chunkFrame(`{"content":"half a sen"}`, ""))
		flush(w)
		drop(w)
	})
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "half a sen"`,
	)
}

// The mirror image — a socket that dies after the finish reason — is
// TestStreamDisconnectCarriesTheFinishReason, with the rest of the End
// behaviour.

// The sentinel with no finish_reason before it: the server said the stream was
// over but never said why. Absent is not the same as unrecognized, and only a
// named value nobody knows is an unknown, so nothing was cut and this is an
// ordinary completion rather than a failure invented from a silence.
//
// The text block gets no close, as no block here ever does, and the terminal event
// is what completes it.
func TestStreamDoneWithoutFinishReason(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"hello"}`, ""),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start chatcmpl-1 deepseek-v4-pro",
		`text 0 "hello"`,
		"ok turn",
	)
}

// A cancelled turn ends with no terminal event and no further events of any
// kind: the caller walked away, and ctx.Err() is the ending it can work out for
// itself. What it has is a prefix of the grammar and a closed channel.
//
// Cancelling races with an event already on its way, so one more may still
// arrive, which is why nothing is asserted about what the last event is. What
// is asserted is that no failure was invented to describe the caller's own
// decision.
func TestStreamCancelMidTurn(t *testing.T) {
	held := make(chan struct{})
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, chunkFrame(openingFrame, ""))
		io.WriteString(w, chunkFrame(`{"content":"first"}`, ""))
		flush(w)
		select {
		case <-held:
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer close(held)

	ch := p.Stream(ctx, turn("hi"))
	var events []api.Event
	for e := range ch {
		events = append(events, e)
		if _, ok := e.(api.EventText); ok {
			cancel()
		}
	}
	checkPrefix(t, events)
	for _, e := range events {
		if e, ok := e.(api.EventErr); ok {
			t.Fatalf("a cancelled turn reported %s\nsequence %v", e.Code, traces(events))
		}
	}
}

// The same, for a caller that cancels while the producer is parked on a send
// nobody is taking. That send is what has to notice, or the goroutine outlives
// the turn.
func TestStreamCancelWhileParkedOnASend(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"one"}`, ""),
		chunkFrame(`{"content":"two"}`, ""),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := p.Stream(ctx, turn("hi"))
	events := []api.Event{<-ch}
	// One event read and no more, so the producer is parked on the next.
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	events = append(events, drain(t, ch)...)

	checkPrefix(t, events)
	for _, e := range events {
		if e, ok := e.(api.EventErr); ok {
			t.Fatalf("a cancelled turn reported %s\nsequence %v", e.Code, traces(events))
		}
	}
}

// A caller's own deadline is its own cancellation and is just as silent:
// ctx.Err() says DeadlineExceeded, and a CodeNetwork event would only repeat
// it. An idle timeout the adapter keeps is the one that is reported, because
// nobody outside can see it; TestStreamIdleTimeout covers that.
func TestStreamCallerDeadlineIsSilent(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, chunkFrame(openingFrame, ""))
		flush(w)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	events := drain(t, p.Stream(ctx, turn("hi")))
	checkPrefix(t, events)
	for _, e := range events {
		if e, ok := e.(api.EventErr); ok {
			t.Fatalf("a caller deadline reported %s\nsequence %v", e.Code, traces(events))
		}
	}
}

// A caller that stops reading without cancelling would block the goroutine
// forever. Cancelling is what releases it, and the goroutine has to notice on
// the send rather than only between frames.
func TestStreamCancelWithoutReading(t *testing.T) {
	p := serve(t, stringHandler(
		chunkFrame(openingFrame, ""),
		chunkFrame(`{"content":"one"}`, ""),
		chunkFrame(`{"content":"two"}`, ""),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	ctx, cancel := context.WithCancel(context.Background())
	ch := p.Stream(ctx, turn("hi"))
	time.Sleep(50 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream goroutine did not exit after cancellation")
	}
}

// The turn id comes off the wire and the model does not. A frame that reports
// neither still opens the turn, stamped with the configured model.
func TestStreamStartFallsBackToTheConfiguredModel(t *testing.T) {
	p := serve(t, stringHandler(
		frame(`{"id":"","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":null}]}`),
		chunkFrame(`{}`, "stop"),
		doneFrame,
	))
	events := collect(t, p, turn("hi"))
	checkGrammar(t, events)
	wantTrace(t, events,
		"start  deepseek-v4-pro",
		`text 0 "x"`,
		"ok turn",
	)
}

// TestStartCarriesTheConfiguredModel pins which of the two model names a turn is
// stamped with. DeepSeek echoes what was sent, so the two agree in a live turn
// and a test that used one string for both would prove nothing about which one
// is carried; here the frames say pro and the Provider is flash.
func TestStartCarriesTheConfiguredModel(t *testing.T) {
	p := serveModel(t, "deepseek-v4-flash", stringHandler(chunkFrame(openingFrame, ""), chunkFrame(`{"content":"hi"}`, "stop"), doneFrame))
	events := collect(t, p, turn("hello"))
	if got := events[0].(api.EventStart).Model; got != "deepseek-v4-flash" {
		t.Fatalf("EventStart.Model %q, want the model the Provider is configured with", got)
	}
}
