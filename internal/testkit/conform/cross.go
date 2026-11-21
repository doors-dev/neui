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

package conform

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/thread/response"
	"neui.dev/provider/lib"
)

var providers = []string{"claude", "gpt", "deepseek", "gemini", "compat"}

type turn struct {
	Name    string      `json:"name"`
	Model   string      `json:"model"`
	Message api.Message `json:"message"`
}

func crossDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "cross")
}

func crossScripts(name string) []Script {
	thought := func(n int) Block {
		return Block{Kind: KindThinking, Text: name + " weighs it " + strconv.Itoa(n) + ".", Signature: base64.StdEncoding.EncodeToString([]byte("signature " + strconv.Itoa(n) + " of " + name + " for the cross corpus"))}
	}
	id := func(n int) string {
		return "call_" + name + "_" + strconv.Itoa(n)
	}
	return []Script{
		{Name: "reasoning-text", Blocks: []Block{thought(1), {Kind: KindText, Text: "Hello from " + name + "."}}},
		{Name: "opaque-text", Blocks: []Block{{Kind: KindOpaque}, {Kind: KindText, Text: "Found it."}}},
		{Name: "text-call", Blocks: []Block{{Kind: KindText, Text: "Checking."}, {Kind: KindTool, ID: id(1), Name: "lookup", Args: `{"q":"dog"}`}}},
		{Name: "reasoning-calls", Blocks: []Block{thought(2), {Kind: KindTool, ID: id(2), Name: "lookup", Args: `{"q":"cat"}`}, {Kind: KindTool, ID: id(3), Name: "clock", Args: `{}`}}},
	}
}

func issue(t *testing.T, a Adapter, srv *server, m api.Model) []turn {
	t.Helper()
	var turns []turn
	for _, s := range crossScripts(a.Name) {
		sse, ok := a.Render(m, s)
		if !ok {
			s.Blocks = slices.Clone(s.Blocks)
			for i := range s.Blocks {
				if s.Blocks[i].Kind == KindThinking {
					s.Blocks[i].Kind = KindSummary
				}
			}
			if sse, ok = a.Render(m, s); !ok {
				continue
			}
		}
		srv.reset(reply{body: sse})
		events := collect(t, a.build(t, srv.srv.URL, m, Params{}), entry(t, m.Name, "text"))
		b := response.NewBulder()
		for _, e := range events {
			if _, ok := b.On(e); !ok {
				t.Fatalf("%s: builder refused %s", s.Name, Describe(e))
			}
		}
		turns = append(turns, turn{Name: s.Name, Model: m.Name, Message: assistant(b.Flush().Content.Ordered()...)})
	}
	return turns
}

func loadTurns(t testing.TB, name string) []turn {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(crossDir(), name+".json"))
	if err != nil {
		t.Fatalf("%v: run the %s conform test with -update to write it", err, name)
	}
	var turns []turn
	if err := json.Unmarshal(data, &turns); err != nil {
		t.Fatalf("%s.json: %v", name, err)
	}
	return turns
}

func answer(m api.Message) api.Message {
	var results []api.Content
	for _, c := range m.Content {
		if u, ok := c.(api.ContentToolUse); ok {
			results = append(results, result(u.ID, api.ToolOk, text("result of "+u.ID)))
		}
	}
	if len(results) == 0 {
		results = append(results, text("thanks"))
	}
	return user(results...)
}

func thread(turns ...turn) []api.Message {
	msgs := []api.Message{user(text("look up cat"))}
	for _, tr := range turns {
		msgs = append(msgs, tr.Message, answer(tr.Message))
	}
	return msgs
}

func checkCross(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	m := a.Models[0]
	own := issue(t, a, srv, m)
	stored := own
	if other := a.Models[len(a.Models)-1]; other.Name != m.Name {
		stored = append(slices.Clone(own), issue(t, a, srv, other)...)
	}
	t.Run("Issued", func(t *testing.T) {
		for _, tr := range stored {
			for _, c := range tr.Message.Content {
				switch c := c.(type) {
				case api.ContentReasoning:
					var owners []string
					for _, p := range providers {
						if _, mine := lib.OwnMeta(c.Meta, p); mine {
							owners = append(owners, p)
						}
					}
					if len(owners) != 1 || owners[0] != a.Name {
						t.Errorf("%s: reasoning with Meta %s is owned by %v, want %s alone", tr.Name, c.Meta, owners, a.Name)
					}
				case api.ContentOpaque:
					if !strings.HasPrefix(c.Kind, a.Name+".") {
						t.Errorf("%s: opaque kind %q does not name %s", tr.Name, c.Kind, a.Name)
					}
				}
			}
		}
		lines := make([]string, len(stored))
		for i, tr := range stored {
			data, err := lib.MarshalRaw(tr)
			if err != nil {
				t.Fatal(err)
			}
			lines[i] = string(data)
		}
		testkit.GoldenFile(t, filepath.Join(crossDir(), a.Name+".json"), []byte("[\n"+strings.Join(lines, ",\n")+"\n]\n"))
	})
	issuers := map[string][]turn{}
	for _, p := range providers {
		if p == a.Name {
			issuers[p] = own
			continue
		}
		issuers[p] = loadTurns(t, p)
	}
	if len(stored) > len(own) {
		issuers[a.Name+"-other-model"] = stored[len(own):]
	}
	mine := func(name string) turn {
		for _, tr := range own {
			if tr.Name == name {
				return tr
			}
		}
		t.Fatalf("%s issued no %s turn", a.Name, name)
		return turn{}
	}
	for name, turns := range issuers {
		var firstModel []turn
		for _, tr := range turns {
			if tr.Model == turns[0].Model {
				firstModel = append(firstModel, tr)
			}
		}
		history := append(thread(firstModel...), assistant(text("ok")), user(text("next")))
		current := thread(firstModel...)
		switched := append(thread(append([]turn{mine("reasoning-calls")}, firstModel...)...), mine("reasoning-text").Message, user(text("again")))
		for _, c := range []struct {
			name string
			msgs []api.Message
		}{{"history", history}, {"current", current}, {"switch", switched}} {
			t.Run(name+"/"+c.name, func(t *testing.T) {
				body := crossSend(t, a, sch, srv, c.msgs)
				crossAssert(t, a, body, c.msgs)
				crossStable(t, a, sch, srv, c.msgs)
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, body, "", "  "); err != nil {
					t.Fatal(err)
				}
				pretty.WriteByte('\n')
				testkit.Golden(t, pretty.Bytes())
			})
		}
	}
	t.Run("Edges", func(t *testing.T) {
		var all bytes.Buffer
		for _, e := range crossEdges() {
			t.Run(e.name, func(t *testing.T) {
				srv.reset(reply{body: mustRender(t, a, m, scriptText)})
				events := collect(t, a.build(t, srv.srv.URL, m, Params{}), api.Request{Tools: e.tools, Messages: e.msgs})
				if e.refused {
					if srv.hits() != 0 || len(events) != 1 {
						t.Fatalf("got %v, want the request refused before sending", DescribeAll(events))
					}
					if ev, ok := events[0].(api.EventErr); !ok || ev.Code != api.CodeInvalid {
						t.Fatalf("got %s, want EventErr %s", Describe(events[0]), api.CodeInvalid)
					}
					all.WriteString("== " + e.name + "\nrefused\n")
					return
				}
				body, _ := srv.last()
				if srv.hits() != 1 {
					t.Fatalf("the request was not sent: %v", DescribeAll(events))
				}
				if err := sch.validate(a.Spec.Request, body, a.Spec.Private, a.Spec.Defaults); err != nil {
					t.Fatalf("body does not match %s: %v\nbody: %s", a.Spec.Request, err, clip(body))
				}
				crossAssert(t, a, body, e.msgs)
				all.WriteString("== " + e.name + "\n")
				all.Write(body)
				all.WriteByte('\n')
			})
		}
		testkit.Golden(t, all.Bytes())
	})
}

func crossSend(t *testing.T, a Adapter, sch *schemas, srv *server, msgs []api.Message) []byte {
	t.Helper()
	m := a.Models[0]
	srv.reset(reply{body: mustRender(t, a, m, scriptText)})
	events := collect(t, a.build(t, srv.srv.URL, m, Params{}), api.Request{Tools: tools(), Messages: msgs})
	body, _ := srv.last()
	if srv.hits() != 1 {
		t.Fatalf("the request was not sent: %v", DescribeAll(events))
	}
	if err := sch.validate(a.Spec.Request, body, a.Spec.Private, a.Spec.Defaults); err != nil {
		t.Fatalf("body does not match %s: %v\nbody: %s", a.Spec.Request, err, clip(body))
	}
	return body
}

func crossAssert(t *testing.T, a Adapter, body []byte, msgs []api.Message) {
	t.Helper()
	if bytes.Contains(body, []byte("[reasoning omitted]")) {
		t.Errorf("the body carries a reasoning note:\n%s", clip(body))
	}
	quoted := func(s string) []byte {
		q, _ := lib.MarshalRaw(s)
		return q
	}
	for _, msg := range msgs {
		for _, c := range msg.Content {
			switch c := c.(type) {
			case api.ContentReasoning:
				var meta struct {
					Data json.RawMessage `json:"data"`
				}
				_ = json.Unmarshal(c.Meta, &meta)
				said := strings.Join(append(c.Thoughts.Ordered(), c.Summary.Ordered()...), "")
				data, own := lib.OwnMeta(c.Meta, a.Name)
				if !own || msg.Role == api.RoleUser {
					if len(meta.Data) >= 24 && bytes.Contains(body, meta.Data) {
						t.Errorf("foreign reasoning payload %s went out", meta.Data)
					}
					if len(said) >= 8 && bytes.Contains(body, []byte(said)) {
						t.Errorf("foreign reasoning text %q went out", said)
					}
					continue
				}
				thoughts := strings.Join(c.Thoughts.Ordered(), "")
				if !(len(data) >= 12 && bytes.Contains(body, data)) && !(thoughts != "" && bytes.Contains(body, quoted(thoughts))) {
					t.Errorf("own reasoning %s %q did not go out:\n%s", data, thoughts, clip(body))
				}
			case api.ContentOpaque:
				ownKind := strings.HasPrefix(c.Kind, a.Name+".") && msg.Role == api.RoleAssistant
				if len(c.Data) >= 8 && bytes.Contains(body, c.Data) != ownKind {
					t.Errorf("opaque %s %s: sent %v, own %v", c.Kind, c.Data, !ownKind, ownKind)
				}
			case api.ContentToolUse:
				if msg.Role != api.RoleAssistant {
					continue
				}
				if !bytes.Contains(body, quoted(lib.CallID(c.ID))) || !bytes.Contains(body, quoted(lib.CallName(c.Name))) {
					t.Errorf("call %q %q did not go out as %q %q:\n%s", c.ID, c.Name, lib.CallID(c.ID), lib.CallName(c.Name), clip(body))
				}
				if c.ID != "" && c.ID != lib.CallID(c.ID) && bytes.Contains(body, quoted(c.ID)) {
					t.Errorf("call id %q went out unmapped", c.ID)
				}
			}
		}
	}
	if a.CrossCheck != nil {
		a.CrossCheck(t, body)
	}
}

func crossStable(t *testing.T, a Adapter, sch *schemas, srv *server, msgs []api.Message) {
	t.Helper()
	var prev []json.RawMessage
	for i, msg := range msgs {
		if msg.Role != api.RoleUser {
			continue
		}
		body := crossSend(t, a, sch, srv, msgs[:i+1])
		var items []json.RawMessage
		v, ok := lookup(body, a.Messages)
		if !ok {
			t.Fatalf("the body has no %s", a.Messages)
		}
		for _, item := range v.([]any) {
			raw, _ := json.Marshal(dropKey(item, "cache_control"))
			items = append(items, raw)
		}
		if len(items) < len(prev) {
			t.Fatalf("messages[:%d] encodes to %d items, fewer than the %d of the prefix before it", i+1, len(items), len(prev))
		}
		for j := range prev {
			if !bytes.Equal(prev[j], items[j]) {
				t.Fatalf("messages[:%d] re-encodes item %d of the prefix before it:\nwas %s\nnow %s", i+1, j, prev[j], items[j])
			}
		}
		prev = items
	}
}

func dropKey(v any, key string) any {
	switch v := v.(type) {
	case map[string]any:
		delete(v, key)
		for k, child := range v {
			v[k] = dropKey(child, key)
		}
	case []any:
		for i, child := range v {
			v[i] = dropKey(child, key)
		}
	}
	return v
}

type crossEdge struct {
	name    string
	tools   []api.Tool
	msgs    []api.Message
	refused bool
}

func crossEdges() []crossEdge {
	long := strings.Repeat("x", 93)
	answered := func(id string) api.Message {
		return user(result(id, api.ToolOk, text("ok")))
	}
	pdf := api.ContentDocument{Data: pdfBytes, Mime: "application/pdf", Name: "a.pdf"}
	md := api.ContentDocument{Data: []byte("# Notes\n\nsome text"), Mime: "text/markdown", Name: "n.md"}
	docx := api.ContentDocument{Data: []byte("PK\x03\x04docx"), Mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Name: "a.docx"}
	zip := api.ContentDocument{Data: []byte("PK\x05\x06"), Mime: "application/zip", Name: "a.zip"}
	latin := api.ContentDocument{Data: []byte("caf\xe9"), Mime: "text/plain", Name: "l.txt"}
	png := api.ContentImage{Data: pngBytes, Mime: "image/png"}
	gif := api.ContentImage{Data: gifBytes, Mime: "image/gif"}
	foreign := api.ContentReasoning{Model: "other", Thoughts: api.Indexed[string]{0: "someone else's thinking"}, Meta: lib.Meta("elsewhere", json.RawMessage(`"c2lnbmF0dXJlLWVsc2V3aGVyZQ=="`))}
	return []crossEdge{
		{name: "id with dots and colons", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call.1:x", "lookup", `{"q":"a"}`)), answered("call.1:x")}},
		{name: "id of 93 characters", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call(long, "lookup", `{"q":"a"}`)), answered(long)}},
		{name: "empty id", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("", "lookup", `{"q":"a"}`)), answered("")}},
		{name: "id reused in a later turn", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `{"q":"a"}`)), answered("call_1"), assistant(call("call_1", "clock", `{}`)), answered("call_1")}},
		{name: "dotted name", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "functions.lookup", `{"q":"a"}`)), answered("call_1")}},
		{name: "empty name", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "", `{}`)), answered("call_1")}},
		{name: "name of 139 characters", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", strings.Repeat("n", 139), `{}`)), answered("call_1")}},
		{name: "array input", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `[1]`)), answered("call_1")}},
		{name: "string input", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `"cat"`)), answered("call_1")}},
		{name: "no input", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "clock", ``)), answered("call_1")}},
		{name: "discarded arguments answered as err_json", tools: tools(), msgs: []api.Message{user(text("go")), assistant(api.ContentToolUse{ID: "call_1", Name: "lookup", Input: api.DiscardedArguments}), user(api.NewContentToolResult("call_1", nil, api.DiscardedArgumentsError{Raw: `{"q":`}))}},
		{name: "discarded arguments answered otherwise", tools: tools(), msgs: []api.Message{user(text("go")), assistant(api.ContentToolUse{ID: "call_1", Name: "lookup", Input: api.DiscardedArguments}), answered("call_1")}, refused: true},
		{name: "orphan result", tools: tools(), msgs: []api.Message{user(text("go")), assistant(text("ok")), answered("call_9")}},
		{name: "result two messages after its call", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `{"q":"a"}`)), answered("call_1"), assistant(text("ok")), answered("call_1")}},
		{name: "result before its call", tools: tools(), msgs: []api.Message{answered("call_1"), assistant(call("call_1", "lookup", `{"q":"a"}`)), answered("call_1")}},
		{name: "result in an assistant message", tools: tools(), msgs: []api.Message{user(text("go")), assistant(text("ok"), result("call_1", api.ToolOk, text("ok"))), user(text("next"))}},
		{name: "call in a user message answered there", tools: tools(), msgs: []api.Message{user(text("go"), call("call_1", "lookup", `{}`), result("call_1", api.ToolOk, text("ok")))}},
		{name: "call in a user message answered next", tools: tools(), msgs: []api.Message{user(text("go"), call("call_1", "lookup", `{}`)), assistant(text("ok")), answered("call_1")}},
		{name: "call in a user message never answered", tools: tools(), msgs: []api.Message{user(text("go"), call("call_1", "lookup", `{}`))}},
		{name: "reasoning in a user message", tools: tools(), msgs: []api.Message{user(text("go"), foreign)}},
		{name: "message emptied by skips", tools: tools(), msgs: []api.Message{user(text("go")), assistant(foreign), user(text("next"))}},
		{name: "documents in a user message", tools: tools(), msgs: []api.Message{user(text("read"), pdf, md, docx, zip, latin)}},
		{name: "documents in an assistant message", tools: tools(), msgs: []api.Message{user(text("go")), assistant(pdf, md, docx, zip, latin, text("done")), user(text("next"))}},
		{name: "documents in a result", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `{}`)), user(result("call_1", api.ToolOk, pdf, md, docx, zip, latin))}},
		{name: "images in a result", tools: tools(), msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `{}`)), user(result("call_1", api.ToolOk, png, gif))}},
		{name: "history calls with no tools declared", msgs: []api.Message{user(text("go")), assistant(call("call_1", "lookup", `{"q":"a"}`)), answered("call_1"), assistant(text("done")), user(text("next"))}},
	}
}

func CheckChatPairs(t *testing.T, body []byte) {
	t.Helper()
	type call struct {
		ID string `json:"id"`
	}
	var req struct {
		Messages []struct {
			Role       string `json:"role"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []call `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	for i, m := range req.Messages {
		if m.Role == "tool" {
			j := i - 1
			for j >= 0 && req.Messages[j].Role == "tool" {
				j--
			}
			if j < 0 || !slices.Contains(req.Messages[j].ToolCalls, call{m.ToolCallID}) {
				t.Errorf("tool message %d answers %q, which the assistant message before it did not call", i, m.ToolCallID)
			}
		}
		for _, c := range m.ToolCalls {
			answered := false
			for _, n := range req.Messages[i+1:] {
				if n.Role != "tool" {
					break
				}
				answered = answered || n.ToolCallID == c.ID
			}
			if !answered {
				t.Errorf("call %q of message %d has no tool message right after it", c.ID, i)
			}
		}
	}
}
