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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/provider/lib"
)

type corpusRequest struct {
	name string
	conf Conf
	opts []Option
	req  api.Request
}

type corpusStream struct {
	name    string
	summary bool
	body    string
}

const corpusBackslash = `\`

var corpusSpecial = []string{
	"<", ">", "&", `"`, corpusBackslash, "/", "\x00", "\x01", "\b", "\f", "\n", "\r", "\t", "\x1f", "\x7f",
	string(rune(0x2028)), string(rune(0x2029)), "é", "日本", "😀", string(rune(0xfffd)), "\xff", "\xc3", "\xed\xa0\x80",
	"\xf4\x90\x80\x80", "<script>", "a&&b", "x<y>z", " ", "  ", " ",
}

func corpusString(r *rand.Rand, n int) string {
	var b strings.Builder
	for range n {
		if r.IntN(3) == 0 {
			b.WriteString(corpusSpecial[r.IntN(len(corpusSpecial))])
			continue
		}
		b.WriteByte(byte('a' + r.IntN(26)))
	}
	return b.String()
}

func corpusBytes(r *rand.Rand, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(r.Uint32())
	}
	return out
}

func corpusText(s string) api.ContentText {
	return api.ContentText{Text: s}
}

func corpusRequests() []corpusRequest {
	r := rand.New(rand.NewPCG(1, 2))
	model := Conf{Model: api.Model{Name: testModel}}
	esc := corpusBackslash + "u003c"
	sig := json.RawMessage(`"EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxDYOaHNj0Xz6Y<&>"`)
	redacted := json.RawMessage(`{"type":"redacted_thinking","data":"EmwKAhgBEgy<&>` + esc + `"}`)
	redactedSpaced := json.RawMessage("{ \"type\" : \"redacted_thinking\",\n \"data\" : \"abc" + esc + "\" }")
	png := corpusBytes(r, 4099)
	jpeg := corpusBytes(r, 1)
	gif := corpusBytes(r, 2)
	webp := corpusBytes(r, 3)
	pdf := append([]byte("%PDF-1.4\n"), corpusBytes(r, 777)...)
	tools := []api.Tool{
		{Name: "read<&>", Desc: "Reads <a> & </a> a file", Schema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "a <path> & more"}}, "required": []string{"path"}}},
		{Name: "now", Desc: ""},
		{Name: "raw", Desc: "raw schema", Schema: json.RawMessage(`{"type":"object", "properties": {"q": {"type":"string","pattern":"^<[a-z]+>$"}}}`)},
		{Name: "typed", Desc: "struct schema", Schema: struct {
			Type string `json:"type"`
			Desc string `json:"description"`
		}{"object", "x > y && y < z"}},
	}
	assistantFull := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{1: "second <b>", 0: "first & "}, Meta: sig},
		api.ContentReasoning{Model: testModel, Summary: api.Indexed[string]{0: "sum<m>ary"}, Meta: json.RawMessage(`"sig2"`)},
		api.ContentReasoning{Model: testModel, Meta: json.RawMessage(`"sig3"`)},
		api.ContentReasoning{Model: testModel, Meta: redacted},
		api.ContentReasoning{Model: testModel, Meta: redactedSpaced},
		corpusText("Let me look <here> & there."),
		api.ContentToolUse{ID: "toolu_01<&>", Name: "read<&>", Input: json.RawMessage(`{"path": "a<b>&c.go", "n": 1.50, "e": "` + esc + `"}`)},
		api.ContentToolUse{ID: "toolu_02", Name: "now"},
		api.ContentToolUse{ID: "toolu_03", Name: "read<&>", Input: api.DiscardedArguments},
		api.ContentOpaque{Model: testModel, Kind: "server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}`), Deltas: []string{`{"query":`, ` "<x> & y"}`}},
		api.ContentOpaque{Model: testModel, Kind: "web_search_tool_result", Data: json.RawMessage(`{"type":"web_search_tool_result", "tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","title":"<T&>"}]}`)},
		api.ContentOpaque{Model: testModel, Kind: "compaction", Data: json.RawMessage(`{"type":"compaction","content":"short"}`)},
	}}
	results := api.Message{Role: api.RoleUser, Content: api.Contents{
		api.ContentToolResult{ID: "toolu_01<&>", Content: api.UserContents{
			corpusText("package main\n\nfunc main() { if a < b && c > d { fmt.Println(\"<ok>\") } }\n"),
			api.ContentImage{Data: png, Mime: "image/png"},
			corpusText("   "),
			api.ContentImage{Mime: "image/png"},
			api.ContentImage{Data: jpeg, Mime: "image/bmp"},
			api.ContentDocument{Data: pdf, Mime: "application/pdf", Name: "a.pdf"},
		}},
		api.ContentToolResult{ID: "toolu_02", Error: api.ToolErrRuntime, Content: api.UserContents{corpusText("boom <err> & more")}},
		api.ContentToolResult{ID: "toolu_03", Error: api.ToolErrJSON, Content: api.UserContents{corpusText("bad json")}},
		corpusText("thanks"),
	}}
	user := api.Message{Role: api.RoleUser, Content: api.Contents{
		corpusText("Hi <there> & \"quoted\" \\ back/slash " + string(rune(0x2028)) + " tab\t nl\n ctl\x01 bad\xff end"),
		api.ContentImage{Data: png, Mime: "image/png"},
		api.ContentImage{Data: jpeg, Mime: "image/jpeg"},
		api.ContentImage{Data: gif, Mime: "image/gif"},
		api.ContentImage{Data: webp, Mime: "image/webp"},
		api.ContentImage{Data: png, Mime: "image/tiff"},
		api.ContentImage{Mime: "image/png"},
		api.ContentDocument{Data: pdf, Mime: "application/pdf", Name: "report <final> & v2.pdf"},
		api.ContentDocument{Data: pdf, Mime: "application/pdf"},
		api.ContentDocument{Data: []byte("# Title\n\n<b>bold</b> & " + string(rune(0x2029)) + " done"), Mime: "text/markdown", Name: "notes.md"},
		api.ContentDocument{Data: []byte("a,b\n1,2\n"), Mime: "text/csv"},
		api.ContentDocument{Data: []byte("<html>&amp;</html>"), Mime: "text/html", Name: "<p>"},
		api.ContentDocument{Data: []byte("{\"a\":\"<\"}"), Mime: "application/json"},
		api.ContentDocument{Data: []byte("k: v"), Mime: "application/yaml"},
		api.ContentDocument{Data: []byte("{\\rtf1}"), Mime: "application/rtf"},
		api.ContentDocument{Data: []byte("<x/>"), Mime: "text/xml"},
		api.ContentDocument{Data: []byte("plain"), Mime: "text/plain"},
		api.ContentDocument{Data: []byte("bad \xff utf8"), Mime: "text/plain"},
		api.ContentDocument{Data: []byte("PK"), Mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Name: "a.docx"},
		api.ContentDocument{Mime: "application/pdf"},
		corpusText(""),
		corpusText(" \n\t"),
		nil,
	}}
	assistantNotes := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		api.ContentDocument{Data: pdf, Mime: "application/pdf"},
		api.ContentToolResult{ID: "toolu_x", Content: api.UserContents{corpusText("x")}},
		api.ContentReasoning{Model: "gpt-5", Meta: json.RawMessage(`{"type":"reasoning","id":"rs_1"}`)},
		corpusText("after"),
	}}
	foreign := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		api.ContentReasoning{Model: testModel, Meta: json.RawMessage(`""`)},
		api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{0: "kept?"}, Meta: sig},
		api.ContentOpaque{Model: testModel, Kind: "reasoning", Data: json.RawMessage(`{"type":"reasoning","id":"rs_1"}`)},
		api.ContentOpaque{Model: testModel, Kind: "other", Data: json.RawMessage(`{"type":"mismatch"}`)},
		api.ContentOpaque{Model: testModel, Kind: "x", Data: json.RawMessage(`not json`)},
		api.ContentOpaque{Model: testModel, Kind: "server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"s","input":{}}`), Deltas: []string{`{"q":`}},
		api.ContentOpaque{Model: testModel, Kind: "server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"s"}`), Deltas: []string{`{"q":1}`}},
		corpusText("end"),
	}}
	unsigned := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{0: "t"}},
		api.ContentReasoning{Model: testModel, Meta: sig},
		corpusText("x"),
	}}
	endsReplayed := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		corpusText("markable"),
		api.ContentReasoning{Model: testModel, Meta: sig},
		api.ContentOpaque{Model: testModel, Kind: "compaction", Data: json.RawMessage(`{"type":"compaction"}`)},
	}}
	onlyReplayed := api.Message{Role: api.RoleAssistant, Content: api.Contents{
		api.ContentReasoning{Model: testModel, Meta: sig},
	}}
	full := []api.Message{user, assistantFull, results, assistantNotes, foreign, unsigned, userText("last <one> & done")}
	var out []corpusRequest
	add := func(name string, conf Conf, req api.Request, opts ...Option) {
		out = append(out, corpusRequest{name: name, conf: conf, opts: append([]Option{WithAPIKey("k")}, opts...), req: req})
	}
	add("minimal", model, api.Request{Messages: []api.Message{userText("hi")}})
	add("system", model, api.Request{System: "Be <brief> & kind.", Messages: []api.Message{userText("hi")}})
	add("full no cache", model, api.Request{System: "sys <x>", Tools: tools, Messages: full})
	add("full all breakpoints", model, api.Request{System: "sys <x>", Tools: tools, Messages: full}, WithCacheBreakpoints(CacheTools, CacheSystem, CacheMessages))
	add("full breakpoints ttl", model, api.Request{System: "sys", Tools: tools, Messages: full}, WithCacheBreakpoints(CacheMessages, CacheTools), WithCacheTTL(CacheTTL1H))
	add("system breakpoint", model, api.Request{System: "sys <&>", Messages: []api.Message{userText("hi")}}, WithCacheBreakpoints(CacheSystem), WithCacheTTL(CacheTTL5M))
	add("tools breakpoint", model, api.Request{Tools: tools[:1], Messages: []api.Message{userText("hi")}}, WithCacheBreakpoints(CacheTools))
	add("breakpoints without targets", model, api.Request{Messages: []api.Message{userText("hi")}}, WithCacheBreakpoints(CacheTools, CacheSystem))
	add("mark avoids replayed", model, api.Request{Messages: []api.Message{userText("go"), endsReplayed}}, WithCacheBreakpoints(CacheMessages))
	add("mark nothing markable", model, api.Request{Messages: []api.Message{userText("go"), onlyReplayed}}, WithCacheBreakpoints(CacheMessages))
	add("mark tool result", model, api.Request{Messages: []api.Message{userText("go"), assistantFull, results}}, WithCacheBreakpoints(CacheMessages))
	add("mark image", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentImage{Data: png, Mime: "image/png"}}}}}, WithCacheBreakpoints(CacheMessages))
	add("mark document", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentDocument{Data: pdf, Mime: "application/pdf", Name: "t"}}}}}, WithCacheBreakpoints(CacheMessages))
	add("mark text document", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentDocument{Data: []byte("x<y"), Mime: "text/plain"}}}}}, WithCacheBreakpoints(CacheMessages))
	add("mark tool use", model, api.Request{Messages: []api.Message{userText("go"), {Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "t", Name: "n", Input: json.RawMessage(`{}`)}}}}}, WithCacheBreakpoints(CacheMessages))
	add("mark note", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{corpusText("")}}}}, WithCacheBreakpoints(CacheMessages))
	add("mark unknown", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{nil}}}}, WithCacheBreakpoints(CacheMessages))
	add("empty tool result", model, api.Request{Messages: []api.Message{userText("go"), {Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "t", Name: "n"}}}, {Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: "t"}}}}}, WithCacheBreakpoints(CacheMessages))
	add("options", Conf{Model: Haiku45}, api.Request{Messages: []api.Message{userText("hi")}}, WithMaxTokens(32000), WithStopSequences("<stop>", "&&"), WithThinking(Thinking{Type: ThinkingEnabled, Budget: 2048, Display: DisplaySummarized}))
	add("effort", Conf{Model: Opus55}, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoiceAuto, Tools: tools[:2]}, WithEffort(EffortXHigh), WithThinking(Thinking{Type: ThinkingAdaptive, Display: DisplayOmitted}))
	add("disabled thinking", Conf{Model: Haiku55}, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoiceNone, Tools: tools[:1]}, WithThinking(Thinking{Type: ThinkingDisabled}), WithEffort(EffortLow))
	add("tool choice any", model, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoiceAny, Tools: tools})
	add("tool choice forced", model, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoice("read<&>"), Tools: tools})
	add("err no messages", model, api.Request{})
	add("note empty message", model, api.Request{Messages: []api.Message{{Role: api.RoleUser}}})
	add("err role", model, api.Request{Messages: []api.Message{{Role: "system", Content: api.Contents{corpusText("x")}}}})
	add("note result id", model, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{Content: api.UserContents{corpusText("x")}}}}}})
	add("err discarded", model, api.Request{Messages: []api.Message{userText("go"), {Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "t", Name: "n", Input: api.DiscardedArguments}}}}})
	add("sent schema", model, api.Request{Tools: []api.Tool{{Name: "bad", Schema: []int{1}}}, Messages: []api.Message{userText("hi")}})
	add("err schema marshal", model, api.Request{Tools: []api.Tool{{Name: "bad", Schema: map[string]any{"f": func() {}}}}, Messages: []api.Message{userText("hi")}})
	add("err tool name", model, api.Request{Tools: []api.Tool{{Desc: "x"}}, Messages: []api.Message{userText("hi")}})
	add("note tool input", model, api.Request{Messages: []api.Message{userText("go"), {Role: api.RoleAssistant, Content: api.Contents{api.ContentToolUse{ID: "t", Name: "n", Input: json.RawMessage(`{bad`)}}}}})
	add("err forced refused", Conf{Model: Opus55}, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoiceAny, Tools: tools})
	add("err forced extended", Conf{Model: Haiku45}, api.Request{Messages: []api.Message{userText("hi")}, ToolChoice: api.ToolChoiceAny, Tools: tools}, WithThinking(Thinking{Type: ThinkingEnabled, Budget: 2048}))
	for i := range 40 {
		var msgs []api.Message
		for j := range 1 + r.IntN(6) {
			if j%2 == 0 {
				var c api.Contents
				for range 1 + r.IntN(4) {
					switch r.IntN(5) {
					case 0:
						c = append(c, api.ContentImage{Data: corpusBytes(r, r.IntN(300)), Mime: "image/png"})
					case 1:
						c = append(c, api.ContentDocument{Data: []byte(corpusString(r, r.IntN(200))), Mime: "text/plain", Name: corpusString(r, r.IntN(8))})
					case 2:
						c = append(c, api.ContentToolResult{ID: corpusString(r, 1+r.IntN(6)), Error: api.ToolError([]string{"", "err_runtime", "err_json"}[r.IntN(3)]), Content: api.UserContents{corpusText(corpusString(r, r.IntN(300))), api.ContentImage{Data: corpusBytes(r, r.IntN(50)), Mime: "image/gif"}}})
					default:
						c = append(c, corpusText(corpusString(r, r.IntN(400))))
					}
				}
				msgs = append(msgs, api.Message{Role: api.RoleUser, Content: c})
				continue
			}
			thoughts := api.Indexed[string]{0: corpusString(r, r.IntN(100)), 1: corpusString(r, r.IntN(30))}
			msgs = append(msgs, api.Message{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: testModel, Thoughts: thoughts, Meta: json.RawMessage(fmt.Sprintf("%q", "s"+corpusString(r, 4)))},
				corpusText(corpusString(r, r.IntN(500))),
				api.ContentToolUse{ID: corpusString(r, 1+r.IntN(6)), Name: corpusString(r, 1+r.IntN(6)), Input: json.RawMessage(fmt.Sprintf(`{"a": %q}`, corpusString(r, r.IntN(60))))},
			}})
		}
		var opts []Option
		if r.IntN(2) == 0 {
			opts = append(opts, WithCacheBreakpoints(CacheMessages, CacheSystem))
		}
		add(fmt.Sprintf("random %d", i), model, api.Request{System: corpusString(r, r.IntN(80)), Messages: msgs}, opts...)
	}
	return out
}

func corpusFrames(pairs ...string) string {
	return sse(pairs...)
}

func corpusStreams() []corpusStream {
	r := rand.New(rand.NewPCG(3, 4))
	start := `{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-opus-5-20260101","stop_reason":null,"usage":{"input_tokens":25,"output_tokens":1,"cache_read_input_tokens":7,"cache_creation_input_tokens":3}}}`
	stop := `{"type":"message_stop"}`
	q := func(s string) string {
		b, _ := json.Marshal(s)
		return string(b)
	}
	md := func(reason string) string {
		return `{"type":"message_delta","delta":{"stop_reason":` + reason + `,"stop_sequence":null},"usage":{"output_tokens":90,"output_tokens_details":{"thinking_tokens":40}}}`
	}
	full := corpusFrames(
		"message_start", start,
		"ping", `{"type":"ping"}`,
		"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me <think> & "}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EqQBCgIYAh<&>"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":0}`,
		"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"signature_delta","signature":"sigonly"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":1}`,
		"content_block_start", `{"type":"content_block_start","index":2,"content_block":{"type":"redacted_thinking","data":"EmwKAhgB<&>"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":2}`,
		"content_block_start", `{"type":"content_block_start","index":3,"content_block":{"type":"text","text":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"Hello <b> & "}}`,
		"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"citations_delta","citation":{"type":"char_location","cited_text":"x"}}}`,
		"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"future_delta","text":"ignored"}}`,
		"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"world "}}`,
		"content_block_stop", `{"type":"content_block_stop","index":3}`,
		"content_block_start", `{"type":"content_block_start","index":4,"content_block":{"type":"text","text":""}}`,
		"content_block_stop", `{"type":"content_block_stop","index":4}`,
		"content_block_start", `{"type":"content_block_start","index":5,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}}`,
		"content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":""}}`,
		"content_block_delta", `{"type":"content_block_delta","index":5,"delta":{"type":"input_json_delta","partial_json":"{\"query\": \"<x>\"}"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":5}`,
		"content_block_start", `{"type":"content_block_start","index":6,"content_block":{"type":"web_search_tool_result", "tool_use_id":"srvtoolu_1","content":[]}}`,
		"content_block_stop", `{"type":"content_block_stop","index":6}`,
		"content_block_start", `{"type":"content_block_start","index":7,"content_block":{"type":"tool_use","id":"toolu_01","name":"read","input":{}}}`,
		"content_block_delta", `{"type":"content_block_delta","index":7,"delta":{"type":"input_json_delta","partial_json":"{\"p\":"}}`,
		"content_block_delta", `{"type":"content_block_delta","index":7,"delta":{"type":"input_json_delta","partial_json":"\"a\"}"}}`,
		"content_block_stop", `{"type":"content_block_stop","index":7}`,
		"content_block_start", `{"type":"content_block_start","index":8,"content_block":{"type":"tool_use","id":"toolu_02","name":"now","input":{}}}`,
		"content_block_stop", `{"type":"content_block_stop","index":8}`,
		"future_event", `{"type":"future_event","payload":{"a":1}}`,
		"message_delta", `{"type":"message_delta","delta":{"stop_reason":null},"usage":{"output_tokens":50}}`,
		"message_delta", `{"type":"message_delta","delta":{},"usage":null}`,
		"message_delta", `{"type":"message_delta"}`,
		"message_delta", md(`"tool_use"`),
		"message_stop", stop,
	)
	var out []corpusStream
	add := func(name string, body string) {
		out = append(out, corpusStream{name: name, body: body})
	}
	add("full", full)
	out = append(out, corpusStream{name: "full summarized", summary: true, body: full})
	for _, reason := range []string{`"end_turn"`, `"stop_sequence"`, `"pause_turn"`, `"max_tokens"`, `"model_context_window_exceeded"`, `"refusal"`, `"brand_new"`, `""`, `null`} {
		add("stop "+reason, corpusFrames("message_start", start, "message_delta", md(reason), "message_stop", stop))
	}
	add("stop missing", corpusFrames("message_start", start, "message_stop", stop))
	add("type from event name", corpusFrames("message_start", `{"message":{"id":"m"}}`, "content_block_start", `{"index":0,"content_block":{"type":"text"}}`, "content_block_delta", `{"index":0,"delta":{"type":"text_delta","text":"x"}}`, "message_stop", `{}`))
	add("start without message", corpusFrames("message_start", `{"type":"message_start"}`, "message_stop", stop))
	add("start no usage", corpusFrames("message_start", `{"type":"message_start","message":{"id":"m","usage":{}}}`, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":9}}`, "message_stop", stop))
	add("cut short", corpusFrames("message_start", start, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}`))
	add("malformed json", corpusFrames("message_start", start, "content_block_start", `{"type":"content_block_start","index":0,`))
	add("malformed index", corpusFrames("message_start", start, "content_block_stop", `{"type":"content_block_stop","index":"0"}`))
	add("malformed content block", corpusFrames("message_start", start, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":5}}`))
	add("error overloaded", corpusFrames("message_start", start, "error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded <&>"}}`))
	add("error spend", corpusFrames("error", `{"type":"error","error":{"type":"rate_limit_error","message":"spend","details":{"error_code":"enforced_spend_limit_reached"}}}`))
	add("error too large", corpusFrames("error", `{"type":"error","error":{"type":"request_too_large","message":"big"}}`))
	add("error empty", corpusFrames("error", `{"type":"error"}`))
	add("error unknown", corpusFrames("error", `{"type":"error","error":{"type":"teapot_error","message":"x"}}`))
	for i := range 30 {
		frames := []string{"message_start", start}
		reason := []string{`"end_turn"`, `"tool_use"`, `"max_tokens"`}[r.IntN(3)]
		for idx := range 1 + r.IntN(8) {
			kind := r.IntN(4)
			switch kind {
			case 0:
				frames = append(frames, "content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}`, idx))
				for range r.IntN(20) {
					frames = append(frames, "content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":%s}}`, idx, q(corpusString(r, r.IntN(30)))))
				}
			case 1:
				frames = append(frames, "content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"thinking","thinking":""}}`, idx))
				for range r.IntN(20) {
					frames = append(frames, "content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"thinking_delta","thinking":%s}}`, idx, q(corpusString(r, r.IntN(30)))))
				}
				frames = append(frames, "content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"signature_delta","signature":%s}}`, idx, q(corpusString(r, 10))))
			case 2:
				frames = append(frames, "content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":%s,"name":%s,"input":{}}}`, idx, q("toolu_"+corpusString(r, r.IntN(5))), q(corpusString(r, 4))))
				for range r.IntN(20) {
					frames = append(frames, "content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%s}}`, idx, q(corpusString(r, r.IntN(30)))))
				}
			default:
				frames = append(frames, "content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"code_execution_tool_result","tool_use_id":%s, "content":{"stdout":%s}}}`, idx, q(corpusString(r, 3)), q(corpusString(r, 20))))
			}
			if r.IntN(5) > 0 || reason != `"max_tokens"` {
				frames = append(frames, "content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, idx))
			}
			if r.IntN(4) == 0 {
				frames = append(frames, "message_delta", fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":null},"usage":{"output_tokens":%d}}`, r.IntN(100)))
			}
		}
		frames = append(frames, "message_delta", md(reason), "message_stop", stop)
		add(fmt.Sprintf("random %d", i), corpusFrames(frames...))
	}
	return out
}

func corpusBody(t testing.TB, c corpusRequest) string {
	p, err := New(c.conf, c.opts...)
	if err != nil {
		t.Fatalf("%s: New: %v", c.name, err)
	}
	body, err := encodeBody(p, c.req)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(body.Bytes())
}

func corpusEvents(t *testing.T, c corpusStream) string {
	var opts []Option
	if c.summary {
		opts = append(opts, WithThinking(Thinking{Type: ThinkingAdaptive, Display: DisplaySummarized}))
	}
	var b strings.Builder
	for _, e := range collect(t, testProvider(t, sseServer(c.body), opts...), hello()) {
		switch e := e.(type) {
		case api.EventErr:
			fmt.Fprintf(&b, "err %s %v", e.Code, e.Err)
		case api.EventStart:
			b.WriteString(conform.Describe(e))
			fmt.Fprintf(&b, " %q", e.Model)
		default:
			b.WriteString(conform.Describe(e))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

var corpusRequestHashes = map[string]string{
	"minimal":                     "82f3c65b38021182",
	"system":                      "ee66b61b8da05c0c",
	"full no cache":               "aad045f69de0bca1",
	"full all breakpoints":        "cbec313a2164ac17",
	"full breakpoints ttl":        "4f3fa71e598f56c2",
	"system breakpoint":           "fc3cbdc7ebbae5fb",
	"tools breakpoint":            "21eeb95b6a7fd043",
	"breakpoints without targets": "82f3c65b38021182",
	"mark avoids replayed":        "79de692ad69cf7e1",
	"mark nothing markable":       "22755ee3fe6775ae",
	"mark tool result":            "c7f8fa1bb97d6b01",
	"mark image":                  "54b22b77dfeb4575",
	"mark document":               "7b590fa24982e4e7",
	"mark text document":          "e8a99e2ff32d2fa0",
	"mark tool use":               "00092d3989bd5804",
	"mark note":                   "16d6d769a308f978",
	"mark unknown":                "16d6d769a308f978",
	"empty tool result":           "81cd9ff7fa8f78f0",
	"options":                     "9890a38facb897ea",
	"effort":                      "77d90fcfe5424138",
	"disabled thinking":           "3edb0654d8816a52",
	"tool choice any":             "fa99010651b04d07",
	"tool choice forced":          "3cbaf13ef3370fb6",
	"err no messages":             "d8bc720b1a784a3f",
	"note empty message":          "c142382f1d2c3b06",
	"err role":                    "9e3fb82d508ccd57",
	"note result id":              "1f0216c9b413186a",
	"err discarded":               "54ea0a6bdb4aaad8",
	"sent schema":                 "d75388b359510b85",
	"err schema marshal":          "c33f18da575c1150",
	"err tool name":               "a17f8f6ec58b78ad",
	"note tool input":             "ab01adf5f582abfe",
	"err forced refused":          "c914f78650b59f43",
	"err forced extended":         "00ebb4d277102ece",
	"random 0":                    "9a54b7a865feb851",
	"random 1":                    "63566b620c9edeff",
	"random 2":                    "14507205f7a78b55",
	"random 3":                    "08b1accbb5b73f86",
	"random 4":                    "f9fbae5fdc46476a",
	"random 5":                    "be1f97c7da3ac7f9",
	"random 6":                    "ffec54bf953ce2de",
	"random 7":                    "f68bd08ae3fc893d",
	"random 8":                    "f86c566c195cfd59",
	"random 9":                    "7284d69740b53a87",
	"random 10":                   "a3a71f32043c3da6",
	"random 11":                   "858c90bf038a8bde",
	"random 12":                   "fa8f48fa013d07da",
	"random 13":                   "f568ed6f189f32bc",
	"random 14":                   "52f0c131006d3b0c",
	"random 15":                   "f68c26451fce21c6",
	"random 16":                   "c75aae660953bbd6",
	"random 17":                   "59fc995b07180eac",
	"random 18":                   "291f9d5492fa69c5",
	"random 19":                   "bdf89cf6cf6dda97",
	"random 20":                   "2e2f32a4648f4648",
	"random 21":                   "bca3f9c6077b15a9",
	"random 22":                   "18da5cb8a0f8db0f",
	"random 23":                   "4dc1279203ade970",
	"random 24":                   "bf07fbbce243dd49",
	"random 25":                   "1a64281e5fb5454b",
	"random 26":                   "074b072940ae2fe7",
	"random 27":                   "312a4297e46875ec",
	"random 28":                   "8bd3cf45d96d0bc2",
	"random 29":                   "65630d4a5f7288d6",
	"random 30":                   "dfcaba3a032d3b51",
	"random 31":                   "670f31626c364e32",
	"random 32":                   "60864a3c941d4c45",
	"random 33":                   "ddace69560f2873c",
	"random 34":                   "2bc0ef6dd257d598",
	"random 35":                   "3492a3315e96f901",
	"random 36":                   "e13fb2c3fef4ceec",
	"random 37":                   "c835396330adaa73",
	"random 38":                   "a8bb26eada64fb69",
	"random 39":                   "f9f3870e4312e81a",
}

var corpusStreamHashes = map[string]string{
	"full":                                   "f9c7f406c34f64a5",
	"full summarized":                        "bfd1de57c2277233",
	"stop \"end_turn\"":                      "f6d1ad774e6568dd",
	"stop \"stop_sequence\"":                 "f6d1ad774e6568dd",
	"stop \"pause_turn\"":                    "f6d1ad774e6568dd",
	"stop \"max_tokens\"":                    "a712905558c0818e",
	"stop \"model_context_window_exceeded\"": "489e53d7d4a460b2",
	"stop \"refusal\"":                       "38ca2eefd9d2b6c5",
	"stop \"brand_new\"":                     "199f68fa06ab3892",
	"stop \"\"":                              "119c2f5bc49bb859",
	"stop null":                              "119c2f5bc49bb859",
	"stop missing":                           "0f719510a7a5c3cc",
	"type from event name":                   "2fc56d1e282f21f1",
	"start without message":                  "2e55eae72d7a0be8",
	"start no usage":                         "adbea65d8a53f27f",
	"cut short":                              "dded498396502e8c",
	"malformed json":                         "6e0a94501e82eb77",
	"malformed index":                        "a6808b91f58dd01b",
	"malformed content block":                "fbbbbcf8ede6bad4",
	"error overloaded":                       "eb89ac96ada4caa3",
	"error spend":                            "843471569b6174d8",
	"error too large":                        "a06327bf864770e8",
	"error empty":                            "10eb88c5909f79c4",
	"error unknown":                          "88abbdcab35f5662",
	"random 0":                               "1e4580d28492fa6e",
	"random 1":                               "cde0bd936ee56b77",
	"random 2":                               "85d181f94ec58674",
	"random 3":                               "b0a390673b626542",
	"random 4":                               "dfc1b31dad1097b8",
	"random 5":                               "66ee9b841cd7e743",
	"random 6":                               "4196af441dc4e9d6",
	"random 7":                               "d187795db7bd87fb",
	"random 8":                               "e94c37aa7f9fece6",
	"random 9":                               "87200344ade18bab",
	"random 10":                              "0ac94809e4bc9ab0",
	"random 11":                              "c18537899c48736e",
	"random 12":                              "d58bdcec96adf04b",
	"random 13":                              "21bd0bbb3711231d",
	"random 14":                              "f74a5f21c198a367",
	"random 15":                              "92d82f584875c493",
	"random 16":                              "e638148387706dd5",
	"random 17":                              "022e446ebb50baa8",
	"random 18":                              "34833937e7ed2fab",
	"random 19":                              "08f86cdbd38dc90e",
	"random 20":                              "31bd400140dae925",
	"random 21":                              "5c58bf74103c648b",
	"random 22":                              "b1330640f7a5efc4",
	"random 23":                              "0affe2c29737a1d0",
	"random 24":                              "b888dc783d0a4845",
	"random 25":                              "f7be542019bd4b1a",
	"random 26":                              "316b6146653d7008",
	"random 27":                              "69583bc2d4e0f685",
	"random 28":                              "e51485445637b649",
	"random 29":                              "757cc6839188123d",
}

func corpusHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func TestRequestBodiesMatchTheCorpus(t *testing.T) {
	for _, c := range corpusRequests() {
		want, ok := corpusRequestHashes[c.name]
		if !ok {
			t.Fatalf("%s: no recorded body", c.name)
		}
		if got := corpusHash(corpusBody(t, c)); got != want {
			t.Errorf("%s: body hash %s, want %s", c.name, got, want)
		}
	}
}

func TestEventStreamsMatchTheCorpus(t *testing.T) {
	for _, c := range corpusStreams() {
		want, ok := corpusStreamHashes[c.name]
		if !ok {
			t.Fatalf("%s: no recorded stream", c.name)
		}
		if got := corpusEvents(t, c); corpusHash(got) != want {
			t.Errorf("%s: events changed:\n%s", c.name, got)
		}
	}
}

func TestMalformedDeltaFailsTheRound(t *testing.T) {
	start := `{"type":"message_start","message":{"id":"msg_01","usage":{"input_tokens":25,"output_tokens":1}}}`
	for name, body := range map[string]string{
		"wrong field type":    sse("message_start", start, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":5}}`, "message_stop", fixStopFrame),
		"not an object":       sse("message_start", start, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":"text"}`, "message_stop", fixStopFrame),
		"unknown block":       sse("message_start", start, "content_block_delta", `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":5}}`, "message_stop", fixStopFrame),
		"stop reason type":    sse("message_start", start, "message_delta", `{"type":"message_delta","delta":{"stop_reason":7},"usage":{"output_tokens":3}}`, "message_stop", fixStopFrame),
		"message delta shape": sse("message_start", start, "message_delta", `{"type":"message_delta","delta":[1],"usage":{"output_tokens":3}}`, "message_stop", fixStopFrame),
	} {
		got := collect(t, testProvider(t, sseServer(body)), hello())
		equalSeq(t, got, []string{"start msg_01 claude-opus-5", "usage in=25 out=1 rsn=0 cr=0 cw=0", "err network"})
		if err := got[len(got)-1].(api.EventErr).Err; !strings.Contains(err.Error(), "malformed") {
			t.Errorf("%s: error %q does not say the frame was malformed", name, err)
		}
	}
}

func benchThread(images bool) api.Request {
	r := rand.New(rand.NewPCG(5, 6))
	src := strings.Repeat("func (c *converter) claim(b *block) int { if !b.open && x < y { b.index = c.next } }\n", 128)
	var msgs []api.Message
	for i := range 200 {
		id := fmt.Sprintf("toolu_%03d", i)
		msgs = append(msgs,
			userText(src[:1024]),
			api.Message{Role: api.RoleAssistant, Content: api.Contents{
				api.ContentReasoning{Model: testModel, Thoughts: api.Indexed[string]{0: src[:512]}, Meta: json.RawMessage(`"` + strings.Repeat("EqQBCgIYAhIM", 30) + `"`)},
				corpusText(src[:2048]),
				api.ContentToolUse{ID: id, Name: "read", Input: json.RawMessage(`{"path":"provider/claude/decode.go","line":120}`)},
			}},
			api.Message{Role: api.RoleUser, Content: api.Contents{api.ContentToolResult{ID: id, Content: api.UserContents{corpusText(src[:8192])}}}},
		)
	}
	if images {
		last := &msgs[len(msgs)-1]
		for range 3 {
			last.Content = append(last.Content, api.ContentImage{Data: corpusBytes(r, 2<<20), Mime: "image/png"})
		}
	}
	msgs = append(msgs, userText("next"))
	return api.Request{System: src[:20480%len(src)], Tools: []api.Tool{{Name: "read", Desc: "Reads a file", Schema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}}, Messages: msgs}
}

func benchEncode(b *testing.B, images bool) {
	p, err := New(Conf{Model: api.Model{Name: testModel}}, WithAPIKey("k"), WithCacheBreakpoints(CacheTools, CacheSystem, CacheMessages))
	if err != nil {
		b.Fatal(err)
	}
	req := benchThread(images)
	b.ReportAllocs()
	for b.Loop() {
		body, err := encodeBody(p, req)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(body.Bytes())))
	}
}

func BenchmarkEncodeText(b *testing.B) {
	benchEncode(b, false)
}

func BenchmarkEncodeImages(b *testing.B) {
	benchEncode(b, true)
}

func BenchmarkConvert(b *testing.B) {
	r := rand.New(rand.NewPCG(7, 8))
	frames := []lib.Frame{{Event: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_01","model":"claude-opus-5","usage":{"input_tokens":25,"output_tokens":1}}}`)}}
	add := func(event, data string) {
		frames = append(frames, lib.Frame{Event: event, Data: []byte(data)})
	}
	add("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	for range 5000 {
		b, _ := json.Marshal(corpusString(r, 16))
		add("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+string(b)+`}}`)
	}
	add("content_block_stop", `{"type":"content_block_stop","index":0}`)
	for i := 1; i <= 3; i++ {
		add("content_block_start", fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":"toolu_%d","name":"write","input":{}}}`, i, i))
		for range 750 {
			b, _ := json.Marshal(`{"content":"` + strings.Repeat("x", 30) + `"`)
			add("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%s}}`, i, b))
		}
		add("content_block_stop", fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, i))
	}
	add("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":20000}}`)
	add("message_stop", `{"type":"message_stop"}`)
	b.ReportAllocs()
	for b.Loop() {
		c := newConverter(testModel, false, 0, 0)
		n := 0
		for _, f := range frames {
			events, ok := c.Convert(f)
			n += len(events)
			if !ok {
				break
			}
		}
		if n == 0 {
			b.Fatal("no events")
		}
	}
}
