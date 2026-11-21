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
	"bytes"
	"encoding/base64"
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
		return nil, errors.New("compat: request has no messages")
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
			return nil, errors.New("compat: tool has no name")
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
	out.Messages = make([]message, 0, len(req.Messages)+1)
	if req.System != "" {
		out.Messages = append(out.Messages, message{Role: "system", Content: req.System})
	}
	var called []toolCall
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
						answers = answers || (c.ID != "" && call.ID == c.ID)
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
					out.Messages = append(out.Messages, message{Role: "tool", ToolCallID: c.ID, Content: result})
					results++
				case api.ContentImage:
					parts = append(parts, image(c))
				case api.ContentDocument:
					parts = append(parts, document(c))
				case api.ContentReasoning:
				case api.ContentToolUse:
					parts = append(parts, textPart{Type: "text", Text: lib.Omitted(c, "a tool call is a field of the assistant message that made it")})
				case api.ContentOpaque:
					parts = append(parts, textPart{Type: "text", Text: lib.Omitted(c, "this wire has no field for a payload of a tool the server ran itself")})
				default:
					parts = append(parts, textPart{Type: "text", Text: lib.Unknown(providerName, i, c)})
				}
			}
			switch {
			case len(parts) > 0:
				out.Messages = append(out.Messages, message{Role: "user", Content: content(parts)})
			case results == 0:
				out.Messages = append(out.Messages, message{Role: "user", Content: emptyMessage})
			}
			called = nil
		case api.RoleAssistant:
			var text, thoughts []string
			var field []byte
			called = nil
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					text = append(text, c.Text)
				case api.ContentReasoning:
					data, own := lib.OwnMeta(c.Meta, providerName)
					if !own {
						continue
					}
					field = data
					thoughts = append(thoughts, c.Thoughts.Ordered()...)
				case api.ContentToolUse:
					if c.ID == "" || c.Name == "" {
						text = append(text, lib.Omitted(c, "a tool call with no id or no name cannot be answered, and the server refuses it"))
						continue
					}
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
						return nil, fmt.Errorf("compat: message %d replays tool call %q whose arguments were discarded, with no failed result answering it", i, c.ID)
					case len(c.Input) > 0:
						args = string(c.Input)
					}
					called = append(called, toolCall{ID: c.ID, Type: "function", Function: toolCallFunction{Name: c.Name, Arguments: args}})
				case api.ContentDocument:
					text = append(text, document(c).Text)
				case api.ContentImage:
					text = append(text, lib.Omitted(c, "an assistant message carries no image parts"))
				case api.ContentToolResult:
					text = append(text, lib.Omitted(c, "a tool result is a message of the tool role and follows the assistant message that called it"))
				case api.ContentOpaque:
					text = append(text, lib.Omitted(c, "this wire has no field for a payload of a tool the server ran itself"))
				default:
					text = append(text, lib.Unknown(providerName, i, c))
				}
			}
			msg := message{Role: "assistant", ToolCalls: called}
			if string(field) == `"reasoning"` {
				msg.Reasoning = strings.Join(thoughts, "")
			} else {
				msg.ReasoningContent = strings.Join(thoughts, "")
			}
			switch {
			case len(text) > 0:
				msg.Content = strings.Join(text, "\n\n")
			case len(called) == 0:
				msg.Content = emptyMessage
			}
			out.Messages = append(out.Messages, msg)
		default:
			return nil, fmt.Errorf("compat: message %d has role %q, which is neither user nor assistant", i, m.Role)
		}
	}
	return out, nil
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
