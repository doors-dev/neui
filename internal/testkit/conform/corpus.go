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
	"encoding/json"

	"neui.dev/api"
)

type Entry struct {
	Name    string
	Request api.Request
}

const ProbeEntry = "tools"

var (
	pngBytes  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpegBytes = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	gifBytes  = []byte("GIF89a\x01\x00\x01\x00")
	webpBytes = []byte("RIFF\x1a\x00\x00\x00WEBPVP8 ")
	pdfBytes  = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
)

func tools() []api.Tool {
	return []api.Tool{
		{Name: "lookup", Desc: "Look a word up.", Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"q": map[string]any{"type": "string"}},
			"required":   []string{"q"},
		}},
		{Name: "clock", Desc: "Read the time."},
	}
}

func user(c ...api.Content) api.Message {
	return api.Message{Role: api.RoleUser, Content: c}
}

func assistant(c ...api.Content) api.Message {
	return api.Message{Role: api.RoleAssistant, Content: c}
}

func text(s string) api.ContentText {
	return api.ContentText{Text: s}
}

func call(id, name, input string) api.ContentToolUse {
	c := api.ContentToolUse{ID: id, Name: name}
	if input != "" {
		c.Input = json.RawMessage(input)
	}
	return c
}

func result(id string, err api.ToolError, c ...api.UserContent) api.ContentToolResult {
	return api.ContentToolResult{ID: id, Content: c, Error: err}
}

func Corpus(model string) []Entry {
	return []Entry{
		{"text", api.Request{Messages: []api.Message{user(text("hi"))}}},
		{"system", api.Request{System: "Answer in one word.", Messages: []api.Message{user(text("hi"))}}},
		{"multiturn", api.Request{Messages: []api.Message{
			user(text("hi")), assistant(text("hello")), user(text("how are you?")),
		}}},
		{"whitespace-text", api.Request{Messages: []api.Message{user(text("hi"), text("  "))}}},
		{"images", api.Request{Messages: []api.Message{user(
			text("describe these"),
			api.ContentImage{Data: jpegBytes, Mime: "image/jpeg"},
			api.ContentImage{Data: pngBytes, Mime: "image/png"},
			api.ContentImage{Data: gifBytes, Mime: "image/gif"},
			api.ContentImage{Data: webpBytes, Mime: "image/webp"},
		)}}},
		{"image-no-mime", api.Request{Messages: []api.Message{user(text("describe"), api.ContentImage{Data: pngBytes})}}},
		{"image-no-bytes", api.Request{Messages: []api.Message{user(text("describe"), api.ContentImage{Mime: "image/png"})}}},
		{"documents", api.Request{Messages: []api.Message{user(
			text("summarize these"),
			api.ContentDocument{Data: pdfBytes, Mime: "application/pdf", Name: "a.pdf"},
			api.ContentDocument{Data: []byte("plain words"), Mime: "text/plain", Name: "b.txt"},
		)}}},
		{"document-csv", api.Request{Messages: []api.Message{user(text("summarize"), api.ContentDocument{Data: []byte("a,b\n1,2\n"), Mime: "text/csv"})}}},
		{"document-no-bytes", api.Request{Messages: []api.Message{user(text("summarize"), api.ContentDocument{Mime: "application/pdf"})}}},
		{ProbeEntry, api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(text("Looking."), call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolOk, text("a small feline"))),
		}}},
		{"tool-no-input", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("what time is it")),
			assistant(call("call_1", "clock", "")),
			user(result("call_1", api.ToolOk, text("noon"))),
		}}},
		{"tool-result-image", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolOk, text("a picture"), api.ContentImage{Data: pngBytes, Mime: "image/png"})),
		}}},
		{"tool-result-image-bad-mime", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolOk, api.ContentImage{Data: pngBytes, Mime: "image/bmp"})),
		}}},
		{"tool-result-document", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolOk, api.ContentDocument{Data: pdfBytes, Mime: "application/pdf", Name: "cat.pdf"})),
		}}},
		{"tool-result-empty", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolOk)),
		}}},
		{"tool-error", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(call("call_1", "lookup", `{"q":"cat"}`)),
			user(result("call_1", api.ToolErrRuntime, text("lookup is down"))),
		}}},
		{"tool-discarded-arguments", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("look up cat")),
			assistant(api.ContentToolUse{ID: "call_1", Name: "lookup", Input: api.DiscardedArguments}),
			user(api.NewContentToolResult("call_1", nil, api.DiscardedArgumentsError{Raw: `{"q":`})),
		}}},
		{"tools-text-turns", api.Request{Tools: tools(), Messages: []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Model: model, Thoughts: api.Indexed[string]{0: "greet back"}}, text("hello")),
			user(text("look up cat")),
		}}},
		{"reasoning-unsigned", api.Request{Messages: []api.Message{
			user(text("hi")),
			assistant(api.ContentReasoning{Model: model, Thoughts: api.Indexed[string]{0: "greet back"}}, text("hello")),
			user(text("again")),
		}}},
		{"tool-choice-none", api.Request{Tools: tools(), ToolChoice: api.ToolChoiceNone, Messages: []api.Message{user(text("hi"))}}},
	}
}
