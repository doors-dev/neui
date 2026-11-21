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

package deepseek

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

// encode turns an api.Request into the request body.
//
// The shape it targets is the plain OpenAI one: content is a bare string on
// every role unless a user or tool message carries an image, so a message
// holding several blocks collapses into one string and the blocks
// that are not text become separate fields or separate messages. A block this wire has no field for leaves a note
// in its place and the turn goes out, stated at the arm it happens in, and so
// does a block this wire does carry, filled with something it does not take.
// The one refusal is a call whose arguments were discarded with no failed
// result answering it.
//
// thinking always goes out, since it is the field that decides when
// reasoning_effort is present too, and a disabled thinking never carries an
// effort. Temperature and top_p go out as set: New refused them where a
// declared model ignores them.
func (p *Provider) encode(req api.Request) (*chatRequest, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("deepseek: request has no messages")
	}
	out := &chatRequest{
		Model:         p.model.Name,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
		Temperature:   p.temperature,
		TopP:          p.topP,
		Stop:          p.stop,
		UserID:        p.userID,
	}
	if err := p.encodeTools(out, req); err != nil {
		return nil, err
	}
	msgs, err := p.encodeMessages(req)
	if err != nil {
		return nil, err
	}
	out.Messages = msgs
	if p.maxTokens > 0 {
		out.MaxTokens = p.maxTokens
	}
	if p.effort == EffortNone {
		out.Thinking = &thinking{Type: "disabled"}
	} else {
		out.Thinking = &thinking{Type: "enabled"}
		out.ReasoningEffort = string(p.effort)
	}
	return out, nil
}

// encodeTools declares the tools and the choice. An unset choice means let the
// server decide, which is not the same as "auto" on every host, so the field
// stays off the wire. On a declared model a forced choice is refused while
// thinking is on, which is what the server answers it with a 400 for; any other
// model is sent the choice as it is.
func (p *Provider) encodeTools(out *chatRequest, req api.Request) error {
	for _, t := range req.Tools {
		if t.Name == "" {
			return errors.New("deepseek: tool has no name")
		}
		params := t.Schema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out.Tools = append(out.Tools, tool{
			Type: "function",
			Function: toolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		})
	}
	name, named := req.ToolChoice.Tool()
	if p.family != familyUnknown && p.effort != EffortNone && (named || req.ToolChoice == api.ToolChoiceAny) {
		return fmt.Errorf("deepseek: tool choice %q forces a tool call, which DeepSeek refuses in thinking mode; pass WithEffort(EffortNone) to force one", req.ToolChoice)
	}
	if named {
		out.ToolChoice = namedToolChoice{
			Type:     "function",
			Function: namedToolFunction{Name: name},
		}
		return nil
	}
	switch req.ToolChoice {
	case api.ToolChoiceAuto:
		out.ToolChoice = "auto"
	case api.ToolChoiceNone:
		out.ToolChoice = "none"
	case api.ToolChoiceAny:
		out.ToolChoice = "required"
	}
	return nil
}

// encodeMessages lowers the whole conversation. The system prompt is an ordinary
// leading message here; there is no top level field for it. Several blocks
// become one string on every role, joined with the blank line api.Content
// requires of every adapter that flattens a block list: a single newline runs
// the end of one block into the start of the next, and a list, a heading or a
// code fence loses its framing at exactly that join.
func (p *Provider) encodeMessages(req api.Request) ([]message, error) {
	msgs := make([]message, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, message{Role: "system", Content: req.System})
	}
	for i, m := range req.Messages {
		switch m.Role {
		case api.RoleUser:
			var parts []any
			var results []message
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					parts = append(parts, textPart{Type: "text", Text: c.Text})
				case api.ContentToolResult:
					answers := false
					if i > 0 && req.Messages[i-1].Role == api.RoleAssistant {
						for _, prev := range req.Messages[i-1].Content {
							u, is := prev.(api.ContentToolUse)
							answers = answers || is && u.ID == c.ID
						}
					}
					if !answers {
						parts = append(parts, note(c, "a tool message has to answer a call of the assistant message right before it, and the server refuses one that does not"))
						continue
					}
					results = append(results, toolResult(c))
				case api.ContentImage:
					parts = append(parts, image(c))
				case api.ContentDocument:
					parts = append(parts, document(c))
				case api.ContentToolUse:
					parts = append(parts, note(c, "a tool call is a field of the assistant message that made it"))
				case api.ContentReasoning, api.ContentOpaque:
				default:
					parts = append(parts, textPart{Type: "text", Text: lib.Unknown(providerName, i, c)})
				}
			}
			msgs = append(msgs, results...)
			switch {
			case len(parts) > 0:
				msgs = append(msgs, message{Role: "user", Content: content(parts)})
			case len(results) == 0:
				msgs = append(msgs, message{Role: "user", Content: emptyMessage})
			}
		case api.RoleAssistant:
			var text, reasoning []string
			var calls []toolCall
			for _, c := range m.Content {
				switch c := c.(type) {
				case api.ContentText:
					text = append(text, c.Text)
				case api.ContentReasoning:
					if _, own := lib.OwnMeta(c.Meta, providerName); own {
						reasoning = append(reasoning, c.Thoughts.Ordered()...)
					}
				case api.ContentToolUse:
					discarded := false
					if i+1 < len(req.Messages) && req.Messages[i+1].Role == api.RoleUser {
						for _, next := range req.Messages[i+1].Content {
							r, is := next.(api.ContentToolResult)
							discarded = discarded || is && r.ID == c.ID && r.Error == api.ToolErrJSON
						}
					}
					args := "{}"
					switch {
					case discarded:
					case bytes.Equal(c.Input, api.DiscardedArguments):
						return nil, fmt.Errorf("deepseek: message %d replays tool call %q whose arguments were discarded, with no failed result answering it", i, c.ID)
					case len(c.Input) > 0:
						args = string(c.Input)
					}
					calls = append(calls, toolCall{
						ID:   lib.CallID(c.ID),
						Type: "function",
						Function: toolCallFunction{
							Name:      lib.CallName(c.Name),
							Arguments: args,
						},
					})
				case api.ContentImage:
					text = append(text, lib.Omitted(c, "DeepSeek refuses an image in an assistant message"))
				case api.ContentDocument:
					text = append(text, document(c).Text)
				case api.ContentToolResult:
					text = append(text, lib.Omitted(c, "a tool result is a message of the tool role and belongs to the user turn that answers the call"))
				case api.ContentOpaque:
				default:
					text = append(text, lib.Unknown(providerName, i, c))
				}
			}
			thoughts := strings.Join(reasoning, "")
			msg := message{Role: "assistant", ReasoningContent: &thoughts, ToolCalls: calls}
			switch {
			case len(text) > 0:
				msg.Content = strings.Join(text, "\n\n")
			case len(calls) == 0:
				msg.Content = emptyMessage
			}
			msgs = append(msgs, msg)
		default:
			return nil, fmt.Errorf("deepseek: message %d has role %q, which is neither user nor assistant", i, m.Role)
		}
	}
	return msgs, nil
}

// toolResult lowers one result into a tool role message.
//
// There is no error flag anywhere in that message: role, tool_call_id and the
// content are the whole of it. So api.ToolError reaches the model as a prefix on
// the text or not at all, and the three values collapse to two here, since what
// the model does about a failure is in the text either way. api.ToolErrJSON is
// how api.DiscardedArgumentsError gets the model to reissue a call whose
// arguments were thrown away.
//
// What the content has no room for leaves the note among the text, the way a
// block of a message does: refusing here costs the whole request and delivers
// the image no more than the note does, and the result is the answer a call is
// waiting on, so failing it fails the conversation too.
func toolResult(c api.ContentToolResult) message {
	parts := make([]any, 0, len(c.Content))
	for _, rc := range c.Content {
		switch rc := rc.(type) {
		case api.ContentText:
			parts = append(parts, textPart{Type: "text", Text: rc.Text})
		case api.ContentImage:
			parts = append(parts, image(rc))
		case api.ContentDocument:
			parts = append(parts, document(rc))
		default:
			parts = append(parts, note(rc, "a DeepSeek tool result carries text, images and text documents only"))
		}
	}
	if len(parts) == 0 && c.Error == api.ToolOk {
		parts = append(parts, textPart{Type: "text", Text: lib.EmptyResult})
	}
	body := content(parts)
	if c.Error != api.ToolOk {
		if s, ok := body.(string); ok {
			body = "Error: " + s
		} else {
			body = append([]any{textPart{Type: "text", Text: "Error:"}}, parts...)
		}
	}
	return message{Role: "tool", ToolCallID: lib.CallID(c.ID), Content: body}
}

func image(c api.ContentImage) any {
	if len(c.Data) == 0 {
		return note(c, "an image with no bytes in it is refused by the server")
	}
	mime, ok := lib.ImageMime(c.Mime)
	if !ok {
		return note(c, "the server takes JPEG, PNG, GIF and WebP only")
	}
	url := make([]byte, len("data:")+len(mime)+len(";base64,")+base64.StdEncoding.EncodedLen(len(c.Data)))
	n := copy(url, "data:")
	n += copy(url[n:], mime)
	n += copy(url[n:], ";base64,")
	base64.StdEncoding.Encode(url[n:], c.Data)
	return imagePart{Type: "image_url", ImageURL: imageURL{URL: unsafe.String(&url[0], len(url))}}
}

func document(c api.ContentDocument) textPart {
	if len(c.Data) == 0 {
		return note(c, "a document with no bytes in it has nothing to send")
	}
	if !api.DocumentMime(strings.ToLower(strings.TrimSpace(c.Mime))).IsText() {
		return note(c, "this wire has no document form and carries only the text types, as text")
	}
	if !utf8.Valid(c.Data) {
		return note(c, "a text document goes as a JSON string, which cannot carry bytes that are not UTF-8")
	}
	return textPart{Type: "text", Text: string(c.Data)}
}

const emptyMessage = "[empty]"

func note(c api.Content, why string) textPart {
	return textPart{Type: "text", Text: lib.Omitted(c, why)}
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
