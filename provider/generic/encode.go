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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
	"unsafe"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const emptyMessage = "[empty]"

func (p *Provider) encode(req api.Request) (*chatRequest, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("generic: request has no messages")
	}
	out := &chatRequest{
		Model:           p.model.Name,
		Stream:          true,
		StreamOptions:   &streamOptions{IncludeUsage: true},
		MaxTokens:       p.maxTokens,
		ReasoningEffort: string(p.effort),
		Temperature:     p.temperature,
		TopP:            p.topP,
		Stop:            p.stop,
	}
	for _, t := range req.Tools {
		if t.Name == "" {
			return nil, errors.New("generic: tool has no name")
		}
		params := t.Schema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out.Tools = append(out.Tools, tool{
			Type:     "function",
			Function: toolFunction{Name: t.Name, Description: t.Desc, Parameters: params},
		})
	}
	if len(p.marker) > 0 && req.System == "" && len(out.Tools) > 0 {
		out.Tools[len(out.Tools)-1] = fielded{out.Tools[len(out.Tools)-1], p.marker}
	}
	name, named := req.ToolChoice.Tool()
	switch {
	case len(out.Tools) == 0:
	case named:
		out.ToolChoice = namedToolChoice{Type: "function", Function: namedToolFunction{Name: name}}
	case req.ToolChoice == api.ToolChoiceAuto:
		out.ToolChoice = "auto"
	case req.ToolChoice == api.ToolChoiceNone:
		out.ToolChoice = "none"
	case req.ToolChoice == api.ToolChoiceAny:
		out.ToolChoice = "required"
	}
	out.Messages = make([]any, 0, len(req.Messages)+1)
	if req.System != "" {
		system := any(req.System)
		if len(p.marker) > 0 {
			system = p.mark(req.System)
		}
		out.Messages = append(out.Messages, &message{Role: "system", Content: system})
	}
	var last *message
	var called []string
	for i, m := range req.Messages {
		switch m.Role {
		case api.RoleUser:
			var parts []any
			results := 0
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					parts = append(parts, textPart{Type: "text", Text: c.Text})
				case api.ContentToolResult:
					answers := false
					for _, call := range called {
						answers = answers || call == c.ID
					}
					if !answers {
						parts = append(parts, textPart{Type: "text", Text: lib.Omitted(c, "a tool message has to answer a call of the assistant message right before it, and the server refuses one that does not")})
						continue
					}
					body := make([]any, 0, len(c.Content)+1)
					for _, rc := range c.Content {
						switch rc := rc.(type) {
						case api.ContentText:
							body = append(body, textPart{Type: "text", Text: rc.Text})
						case api.ContentImage:
							body = append(body, image(rc))
						case api.ContentDocument:
							body = append(body, document(rc))
						default:
							body = append(body, textPart{Type: "text", Text: lib.Unknown(providerName, i, rc)})
						}
					}
					if len(body) == 0 {
						body = append(body, textPart{Type: "text", Text: lib.EmptyResult})
					}
					result := content(body)
					if c.Error != api.ToolOk {
						if s, ok := result.(string); ok {
							result = "Error: " + s
						} else {
							result = append([]any{textPart{Type: "text", Text: "Error:"}}, body...)
						}
					}
					last = &message{Role: "tool", ToolCallID: lib.CallID(c.ID), Content: result}
					out.Messages = append(out.Messages, last)
					results++
				case api.ContentImage:
					parts = append(parts, image(c))
				case api.ContentDocument:
					parts = append(parts, document(c))
				case api.ContentReasoning, api.ContentOpaque:
				case api.ContentToolUse:
					parts = append(parts, textPart{Type: "text", Text: lib.Omitted(c, "a tool call is a field of the assistant message that made it")})
				default:
					parts = append(parts, textPart{Type: "text", Text: lib.Unknown(providerName, i, c)})
				}
			}
			switch {
			case len(parts) > 0:
				last = &message{Role: "user", Content: content(parts)}
				out.Messages = append(out.Messages, last)
			case results == 0:
				last = &message{Role: "user", Content: emptyMessage}
				out.Messages = append(out.Messages, last)
			}
			called = nil
		case api.RoleAssistant:
			var text, thoughts []string
			var field, opaque []byte
			var calls []toolCall
			called = nil
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					text = append(text, c.Text)
				case api.ContentReasoning:
					data, own := lib.OwnMeta(c.Meta, providerName)
					if !own || c.Model != p.model.Name {
						continue
					}
					field = data
					thoughts = append(thoughts, c.Thoughts.Ordered()...)
				case api.ContentToolUse:
					discarded := false
					if i+1 < len(req.Messages) && req.Messages[i+1].Role == api.RoleUser {
						for _, next := range req.Messages[i+1].Content {
							r, ok := next.(api.ContentToolResult)
							discarded = discarded || (ok && r.ID == c.ID && r.Error == api.ToolErrJSON)
						}
					}
					args := "{}"
					switch {
					case discarded:
					case bytes.Equal(c.Input, api.DiscardedArguments):
						return nil, fmt.Errorf("generic: message %d replays tool call %q whose arguments were discarded, with no failed result answering it", i, c.ID)
					case len(c.Input) > 0:
						args = string(c.Input)
					}
					called = append(called, c.ID)
					calls = append(calls, toolCall{ID: lib.CallID(c.ID), Type: "function", Function: toolCallFunction{Name: lib.CallName(c.Name), Arguments: args}})
				case api.ContentDocument:
					text = append(text, document(c).Text)
				case api.ContentImage:
					text = append(text, lib.Omitted(c, "an assistant message carries no image parts"))
				case api.ContentToolResult:
					text = append(text, lib.Omitted(c, "a tool result is a message of the tool role and follows the assistant message that called it"))
				case api.ContentOpaque:
					if c.Kind == opaqueKind && c.Model == p.model.Name {
						opaque = c.Data
					}
				default:
					text = append(text, lib.Unknown(providerName, i, c))
				}
			}
			var fields struct {
				Fields json.RawMessage            `json:"fields"`
				Calls  map[string]json.RawMessage `json:"calls"`
			}
			if len(opaque) > 0 {
				_ = json.Unmarshal(opaque, &fields)
			}
			msg := &message{Role: "assistant"}
			for j, call := range calls {
				if f := fields.Calls[called[j]]; len(f) > 2 && f[0] == '{' && len(bytes.TrimSpace(f[1:len(f)-1])) > 0 {
					msg.ToolCalls = append(msg.ToolCalls, fielded{call, f[1 : len(f)-1]})
				} else {
					msg.ToolCalls = append(msg.ToolCalls, call)
				}
			}
			if string(field) == `"reasoning"` {
				msg.Reasoning = strings.Join(thoughts, "")
			} else {
				msg.ReasoningContent = strings.Join(thoughts, "")
			}
			switch {
			case len(text) > 0:
				msg.Content = strings.Join(text, "\n\n")
			case len(calls) == 0:
				msg.Content = emptyMessage
			}
			last = msg
			if f := fields.Fields; len(f) > 2 && f[0] == '{' && len(bytes.TrimSpace(f[1:len(f)-1])) > 0 {
				out.Messages = append(out.Messages, fielded{msg, f[1 : len(f)-1]})
			} else {
				out.Messages = append(out.Messages, msg)
			}
		default:
			return nil, fmt.Errorf("generic: message %d has role %q, which is neither user nor assistant", i, m.Role)
		}
	}
	if len(p.marker) > 0 && last != nil {
		last.Content = p.mark(last.Content)
	}
	return out, nil
}

func (p *Provider) mark(content any) any {
	switch c := content.(type) {
	case string:
		return []any{fielded{textPart{Type: "text", Text: c}, p.marker}}
	case []any:
		for i := len(c) - 1; i >= 0; i-- {
			if part, ok := c[i].(textPart); ok {
				c[i] = fielded{part, p.marker}
				break
			}
		}
	}
	return content
}

func image(c api.ContentImage) any {
	if len(c.Data) == 0 {
		return textPart{Type: "text", Text: lib.Omitted(c, "an image with no bytes has nothing to send")}
	}
	mime, ok := lib.ImageMime(c.Mime)
	if !ok {
		return textPart{Type: "text", Text: lib.Omitted(c, "the image types OpenAI-compatible servers take are JPEG, PNG, GIF and WebP")}
	}
	url := make([]byte, len("data:")+len(mime)+len(";base64,")+base64.StdEncoding.EncodedLen(len(c.Data)))
	n := copy(url, "data:")
	n += copy(url[n:], mime)
	n += copy(url[n:], ";base64,")
	base64.StdEncoding.Encode(url[n:], c.Data)
	return imagePart{Type: "image_url", ImageURL: imageURL{URL: unsafe.String(&url[0], len(url))}}
}

func document(c api.ContentDocument) textPart {
	switch {
	case len(c.Data) == 0:
		return textPart{Type: "text", Text: lib.Omitted(c, "a document with no bytes has nothing to send")}
	case !api.DocumentMime(strings.ToLower(strings.TrimSpace(c.Mime))).IsText():
		return textPart{Type: "text", Text: lib.Omitted(c, "this wire has no document form and carries only the text types, as text")}
	case !utf8.Valid(c.Data):
		return textPart{Type: "text", Text: lib.Omitted(c, "a text document goes as a JSON string, which cannot carry bytes that are not UTF-8")}
	}
	return textPart{Type: "text", Text: string(c.Data)}
}

func content(parts []any) any {
	text := make([]string, 0, len(parts))
	for _, part := range parts {
		t, ok := part.(textPart)
		if !ok {
			return parts
		}
		text = append(text, t.Text)
	}
	return strings.Join(text, "\n\n")
}
