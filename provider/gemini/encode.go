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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

var emptyObject = json.RawMessage(`{}`)

var nativeDocuments = map[string]bool{
	"application/pdf":               true,
	"text/csv":                      true,
	"application/msword":            true,
	"application/vnd.ms-excel":      true,
	"application/vnd.ms-powerpoint": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.template":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/vnd.oasis.opendocument.text":                                   true,
	"application/vnd.oasis.opendocument.spreadsheet":                            true,
	"application/vnd.oasis.opendocument.presentation":                           true,
}

func (p *Provider) encode(req api.Request) (*request, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("gemini: request has no messages")
	}
	w := &request{
		Model:             p.model.Name,
		Stream:            true,
		Store:             false,
		SystemInstruction: req.System,
	}
	for _, t := range req.Tools {
		if t.Name == "" {
			return nil, errors.New("gemini: tool has no name")
		}
		params := json.RawMessage(`{"type":"object","properties":{}}`)
		if t.Schema != nil {
			raw, err := lib.MarshalRaw(t.Schema)
			if err != nil {
				return nil, fmt.Errorf("gemini: tool %q schema: %w", t.Name, err)
			}
			params = raw
		}
		w.Tools = append(w.Tools, tool{Type: "function", Name: t.Name, Description: t.Desc, Parameters: params})
	}
	gc := &generationConfig{
		MaxOutputTokens:   p.maxTokens,
		StopSequences:     p.stopSeqs,
		ThinkingLevel:     p.effort,
		ThinkingSummaries: "none",
	}
	if p.summaries {
		gc.ThinkingSummaries = "auto"
	}
	if name, ok := req.ToolChoice.Tool(); ok {
		gc.ToolChoice = toolChoiceConfig{AllowedTools: allowedTools{Mode: "any", Tools: []string{name}}}
	} else if req.ToolChoice != "" {
		gc.ToolChoice = string(req.ToolChoice)
	}
	w.GenerationConfig = gc

	for i, m := range req.Messages {
		switch m.Role {
		case api.RoleUser:
			var blocks []any
			results := 0
			noted := map[int]bool{}
			for j, c := range m.Content {
				r, ok := c.(api.ContentToolResult)
				if !ok {
					continue
				}
				var called *api.ContentToolUse
				var by api.Role
				for k := i; k >= 0 && called == nil; k-- {
					content := req.Messages[k].Content
					if k == i {
						content = content[:j]
					}
					for l := len(content) - 1; l >= 0 && called == nil; l-- {
						if u, is := content[l].(api.ContentToolUse); is && u.ID == r.ID {
							called, by = &u, req.Messages[k].Role
						}
					}
				}
				switch {
				case called == nil:
					w.Input = append(w.Input, resultStep(r, lib.CallName(""), i))
				case by == api.RoleAssistant:
					w.Input = append(w.Input, resultStep(r, lib.CallName(called.Name), i))
				default:
					noted[j] = true
					continue
				}
				results++
			}
			for j, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					blocks = append(blocks, textBlock(c))
				case api.ContentImage:
					blocks = append(blocks, imageBlock(c))
				case api.ContentDocument:
					blocks = append(blocks, documentBlock(c))
				case api.ContentToolResult:
					if noted[j] {
						blocks = append(blocks, note(c, "the result answers a call written into a user message"))
					}
				case api.ContentToolUse:
					blocks = append(blocks, note(c, "a call belongs to the model's turn"))
				case api.ContentReasoning, api.ContentOpaque:
				default:
					blocks = append(blocks, textContent{Type: "text", Text: lib.Unknown(providerName, i, c)})
				}
			}
			if len(blocks) > 0 {
				w.Input = append(w.Input, userInput{Type: "user_input", Content: blocks})
			} else if results == 0 {
				w.Input = append(w.Input, userInput{Type: "user_input", Content: []any{textContent{Type: "text", Text: "[empty]"}}})
			}
		case api.RoleAssistant:
			start := len(w.Input)
			var run []any
			opened, called := false, false
			flush := func() {
				if len(run) > 0 && !called {
					w.Input = append(w.Input, modelOutput{Type: "model_output", Content: run})
					run = nil
				}
			}
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					run = append(run, textBlock(c))
				case api.ContentImage:
					run = append(run, imageBlock(c))
				case api.ContentDocument:
					run = append(run, documentBlock(c))
				case api.ContentReasoning:
					data, own := lib.OwnMeta(c.Meta, providerName)
					var th thought
					if !own || json.Unmarshal(data, &th) != nil || th.Type != "thought" || th.Signature == "" {
						continue
					}
					flush()
					opened = opened || len(w.Input) == start
					w.Input = append(w.Input, data)
				case api.ContentToolUse:
					discarded := false
					if i+1 < len(req.Messages) && req.Messages[i+1].Role == api.RoleUser {
						for _, next := range req.Messages[i+1].Content {
							r, is := next.(api.ContentToolResult)
							discarded = discarded || is && r.ID == c.ID && r.Error == api.ToolErrJSON
						}
					}
					args := emptyObject
					switch {
					case discarded:
					case bytes.Equal(c.Input, api.DiscardedArguments):
						return nil, fmt.Errorf("gemini: tool call %q carries discarded arguments and no result answering it reports err_json", c.ID)
					default:
						args = lib.ObjectInput(c.Input)
					}
					flush()
					if !opened {
						w.Input = slices.Insert(w.Input, start, any(thought{Type: "thought", Signature: skipSignature}))
						opened = true
					}
					w.Input = append(w.Input, functionCall{Type: "function_call", ID: lib.CallID(c.ID), Name: lib.CallName(c.Name), Arguments: args})
					called = true
				case api.ContentOpaque:
					kind, own := strings.CutPrefix(c.Kind, opaquePrefix)
					var head typeHead
					if !own || kind == "" || json.Unmarshal(c.Data, &head) != nil || head.Type != kind {
						continue
					}
					flush()
					w.Input = append(w.Input, json.RawMessage(c.Data))
				case api.ContentToolResult:
					run = append(run, note(c, "a result belongs to the user's turn"))
				default:
					run = append(run, textContent{Type: "text", Text: lib.Unknown(providerName, i, c)})
				}
			}
			if len(run) > 0 {
				w.Input = append(w.Input, modelOutput{Type: "model_output", Content: run})
			}
			if len(w.Input) == start {
				w.Input = append(w.Input, modelOutput{Type: "model_output", Content: []any{textContent{Type: "text", Text: "[empty]"}}})
			}
		default:
			return nil, fmt.Errorf("gemini: message %d has role %q", i, m.Role)
		}
	}
	return w, nil
}

func note(c api.Content, why string) textContent {
	return textContent{Type: "text", Text: lib.Omitted(c, why)}
}

func textBlock(c api.ContentText) any {
	if c.Text == "" {
		return note(c, "the API refuses an empty text")
	}
	return textContent{Type: "text", Text: c.Text}
}

func imageBlock(c api.ContentImage) any {
	if len(c.Data) == 0 {
		return note(c, "the image has no bytes")
	}
	mime, ok := lib.ImageMime(c.Mime)
	if !ok {
		return note(c, "the API takes PNG, JPEG, WEBP and GIF here")
	}
	if mime == "image/jpg" {
		mime = "image/jpeg"
	}
	return imageContent{Type: "image", Data: c.Data, MimeType: mime}
}

func documentBlock(c api.ContentDocument) any {
	if len(c.Data) == 0 {
		return note(c, "the document has no bytes")
	}
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	switch {
	case nativeDocuments[mime]:
		return documentContent{Type: "document", Data: c.Data, MimeType: mime}
	case api.DocumentMime(mime).IsText():
		if !utf8.Valid(c.Data) {
			return note(c, "the text document is not UTF-8")
		}
		return textContent{Type: "text", Text: string(c.Data)}
	}
	return note(c, "the API does not take this media type")
}

func resultStep(r api.ContentToolResult, name string, at int) functionResult {
	out := make([]any, 0, len(r.Content))
	for _, c := range r.Content {
		switch c := c.(type) {
		case api.ContentText:
			out = append(out, textBlock(c))
		case api.ContentImage:
			out = append(out, imageBlock(c))
		case api.ContentDocument:
			out = append(out, documentBlock(c))
		default:
			out = append(out, textContent{Type: "text", Text: lib.Unknown(providerName, at, c)})
		}
	}
	if len(out) == 0 {
		out = append(out, textContent{Type: "text", Text: lib.EmptyResult})
	}
	return functionResult{Type: "function_result", Name: name, CallID: lib.CallID(r.ID), Result: out, IsError: r.Error != api.ToolOk}
}
