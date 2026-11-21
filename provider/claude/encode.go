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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func (p *Provider) encode(req api.Request) (*request, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("claude: request has no messages")
	}
	w := &request{
		Model:         p.model.Name,
		MaxTokens:     p.maxToken,
		Stream:        true,
		StopSequences: p.stopSeqs,
	}
	w.Thinking = &thinkingWire{
		Type:         "adaptive",
		Display:      string(p.display),
		BlockBinding: &blockBinding{PrefixMismatchBehavior: "drop_block"},
	}
	if p.effort != "" {
		w.OutputConfig = &outputConfig{Effort: p.effort}
	}
	var err error
	if w.Tools, err = p.encodeTools(req.Tools); err != nil {
		return nil, err
	}
	if w.ToolChoice, err = p.encodeToolChoice(req.ToolChoice); err != nil {
		return nil, err
	}
	if w.System, err = p.encodeSystem(req.System); err != nil {
		return nil, err
	}
	if w.Messages, err = p.encodeMessages(req.Messages); err != nil {
		return nil, err
	}
	switch {
	case p.noCache:
	case len(w.System) > 0:
		w.System[len(w.System)-1].CacheControl = p.cacheMark()
	case len(w.Tools) > 0:
		w.Tools[len(w.Tools)-1].CacheControl = p.cacheMark()
	}
	return w, nil
}

// encodeToolChoice maps the choice onto this API's, and on a declared model
// whose rules say so refuses forcing a tool, which the API refuses on every
// request there. A model not declared here is sent the choice as it is.
func (p *Provider) encodeToolChoice(choice api.ToolChoice) (*toolChoice, error) {
	var tc *toolChoice
	if name, forced := choice.Tool(); forced {
		tc = &toolChoice{Type: "tool", Name: name}
	} else {
		switch choice {
		case "":
			return nil, nil
		case api.ToolChoiceAuto:
			tc = &toolChoice{Type: "auto"}
		case api.ToolChoiceNone:
			tc = &toolChoice{Type: "none"}
		case api.ToolChoiceAny:
			tc = &toolChoice{Type: "any"}
		}
	}
	if m, ok := declared[p.model.Name]; ok && m.refusesForcedTool && (tc.Type == "any" || tc.Type == "tool") {
		return nil, fmt.Errorf("claude: model %q refuses tool choice %q; use auto with strict tools, or structured outputs", p.model.Name, choice)
	}
	return tc, nil
}

func (p *Provider) encodeTools(tools []api.Tool) ([]tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]tool, 0, len(tools))
	for _, t := range tools {
		if t.Name == "" {
			return nil, errors.New("claude: tool has no name")
		}
		schema, err := encodeSchema(t.Schema)
		if err != nil {
			return nil, fmt.Errorf("claude: tool %q schema: %w", t.Name, err)
		}
		out = append(out, tool{Name: t.Name, Description: t.Desc, InputSchema: schema})
	}
	return out, nil
}

// encodeSchema fills in an empty object schema, because input_schema is
// required and a tool that takes no arguments still has to declare that.
func encodeSchema(schema any) (json.RawMessage, error) {
	if schema == nil {
		return json.RawMessage(`{"type":"object","properties":{}}`), nil
	}
	return json.Marshal(schema)
}

func (p *Provider) encodeSystem(system string) ([]textBlock, error) {
	if system == "" {
		return nil, nil
	}
	return []textBlock{{Type: "text", Text: system}}, nil
}

const emptyMessage = "[empty]"

func (p *Provider) encodeMessages(msgs []api.Message) ([]message, error) {
	out := make([]message, 0, len(msgs))
	for i, m := range msgs {
		role := string(m.Role)
		if role != string(api.RoleUser) && role != string(api.RoleAssistant) {
			return nil, fmt.Errorf("claude: message %d has role %q", i, m.Role)
		}
		replay := true
		for _, c := range m.Content {
			if r, is := c.(api.ContentReasoning); is {
				data, own := lib.OwnMeta(r.Meta, providerName)
				var signature string
				var block struct {
					Type string `json:"type"`
				}
				switch {
				case !own:
					replay = false
				case json.Unmarshal(data, &signature) == nil:
					replay = replay && signature != ""
				default:
					replay = replay && json.Unmarshal(data, &block) == nil && block.Type == "redacted_thinking"
				}
			}
		}
		blocks := make([]any, 0, len(m.Content))
		var results []any
		var markable, resultMark *cached
		for j, c := range m.Content {
			block, mark, err := p.encodeContent(msgs, i, c, replay)
			if err != nil {
				return nil, fmt.Errorf("claude: message %d block %d: %w", i, j, err)
			}
			if block == nil {
				continue
			}
			if _, is := block.(*toolResultBlock); is {
				results = append(results, block)
				resultMark = mark
				continue
			}
			if mark != nil {
				markable = mark
			}
			if _, is := block.(*textBlock); is && m.Role == api.RoleUser && len(blocks) > 0 {
				if prev, is := blocks[len(blocks)-1].(*textBlock); is && !strings.HasSuffix(prev.Text, "\n") {
					prev.Text += "\n"
				}
			}
			blocks = append(blocks, block)
		}
		if len(results) > 0 {
			blocks = append(results, blocks...)
		}
		if markable == nil {
			markable = resultMark
		}
		if len(blocks) == 0 {
			b := &textBlock{Type: "text", Text: emptyMessage}
			blocks, markable = append(blocks, b), &b.cached
		}
		if i == len(msgs)-1 && markable != nil && !p.noCache {
			markable.CacheControl = p.cacheMark()
		}
		out = append(out, message{Role: role, Content: blocks})
	}
	return out, nil
}

func (p *Provider) encodeContent(msgs []api.Message, at int, c api.Content, replay bool) (any, *cached, error) {
	role := string(msgs[at].Role)
	switch c := c.(type) {
	case api.ContentText:
		if strings.TrimSpace(c.Text) == "" {
			return omitted(c, "a text block with no non-whitespace content in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		b := &textBlock{Type: "text", Text: c.Text}
		return b, &b.cached, nil
	case api.ContentImage:
		if len(c.Data) == 0 {
			return omitted(c, "an image source with no bytes in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		mime, ok := lib.ImageMime(c.Mime)
		if !ok {
			return omitted(c, fmt.Sprintf("an image of type %q is refused by this API, which takes image/jpeg, image/png, image/gif and image/webp, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
		}
		if mime == "image/jpg" {
			mime = string(api.ImageJPEG)
		}
		b := &imageBlock{Type: "image", Source: base64Source{Type: "base64", MediaType: mime, Data: c.Data}}
		return b, &b.cached, nil
	case api.ContentDocument:
		if len(c.Data) == 0 {
			return omitted(c, "a document source with no bytes in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		if role == string(api.RoleAssistant) {
			if !api.DocumentMime(strings.ToLower(strings.TrimSpace(c.Mime))).IsText() || !utf8.Valid(c.Data) || strings.TrimSpace(string(c.Data)) == "" {
				return omitted(c, "a document is a block of the user turn on this API, which refuses one placed directly in an assistant message, so only a text document whose bytes are UTF-8 and not blank goes there, as a text block")
			}
			b := &textBlock{Type: "text", Text: string(c.Data)}
			return b, &b.cached, nil
		}
		return encodeDocument(c)
	case api.ContentToolUse:
		if role == string(api.RoleUser) {
			return omitted(c, "a tool call is a block of the assistant turn that made it, and a user message on this API takes none")
		}
		discarded := false
		if at+1 < len(msgs) && msgs[at+1].Role == api.RoleUser {
			for _, next := range msgs[at+1].Content {
				r, is := next.(api.ContentToolResult)
				discarded = discarded || is && r.ID == c.ID && r.Error == api.ToolErrJSON
			}
		}
		input := json.RawMessage(`{}`)
		switch {
		case discarded:
		case bytes.Equal(c.Input, api.DiscardedArguments):
			return nil, nil, fmt.Errorf("tool call %q holds discarded arguments and no result answers it as %s, so there are no arguments to send and an empty object would replay as a call that took none", c.ID, api.ToolErrJSON)
		default:
			input = lib.ObjectInput(c.Input)
		}
		b := &toolUseBlock{Type: "tool_use", ID: lib.CallID(c.ID), Name: lib.CallName(c.Name), Input: input}
		return b, &b.cached, nil
	case api.ContentToolResult:
		if role == string(api.RoleAssistant) {
			return omitted(c, "a tool result is a block of the user turn that answers the call, and an assistant message on this API takes none")
		}
		answers := false
		if at > 0 && msgs[at-1].Role == api.RoleAssistant {
			for _, prev := range msgs[at-1].Content {
				u, is := prev.(api.ContentToolUse)
				answers = answers || is && u.ID == c.ID
			}
		}
		if !answers {
			return omitted(c, "a tool result has to answer a call of the assistant message right before it, and this API refuses one that does not on every later turn of the thread")
		}
		b, err := encodeToolResult(c)
		if err != nil {
			return nil, nil, err
		}
		return b, &b.cached, nil
	case api.ContentReasoning:
		if role == string(api.RoleUser) || !replay {
			return nil, nil, nil
		}
		data, _ := lib.OwnMeta(c.Meta, providerName)
		var signature string
		if json.Unmarshal(data, &signature) != nil {
			return data, nil, nil
		}
		readable := c.Thoughts.Ordered()
		if len(readable) == 0 {
			readable = c.Summary.Ordered()
		}
		return &thinkingBlock{
			Type:      "thinking",
			Thinking:  strings.Join(readable, ""),
			Signature: signature,
		}, nil, nil
	case api.ContentOpaque:
		kind, own := strings.CutPrefix(c.Kind, providerName+".")
		var block struct {
			Type string `json:"type"`
		}
		if role == string(api.RoleUser) || !own || json.Unmarshal(c.Data, &block) != nil || block.Type == "" || block.Type != kind {
			return nil, nil, nil
		}
		if len(c.Deltas) == 0 {
			return c.Data, nil, nil
		}
		raw, err := spliceField(c.Data, "input", json.RawMessage(strings.Join(c.Deltas, "")))
		if err != nil {
			return omitted(c, "the fragments this block collected never parsed as JSON, or the payload holding the field they belong in is not the object this API sent, and either way there is nothing to splice back into it")
		}
		return raw, nil, nil
	}
	b := &textBlock{Type: "text", Text: lib.Unknown(providerName, at, c)}
	return b, &b.cached, nil
}

// omitted is the note that stands in for content this API cannot send. It is a
// text block because text is the one block both roles take here, so the note
// lands where the content was whichever message it came from, and it is markable
// for the same reason it is placeable: nothing about it has to be replayed
// unchanged.
func omitted(c api.Content, why string) (any, *cached, error) {
	b := &textBlock{Type: "text", Text: lib.Omitted(c, why)}
	return b, &b.cached, nil
}

// encodeDocument sends a PDF as base64 and the text types as a text source,
// which are the two sources this API takes. The docs have plain text files such
// as markdown or csv sent as text/plain, so every text type goes out labelled
// that way. Anything else, the binary Office types and whatever is unknown,
// becomes the note omitted returns: the block is stored, so a refusal would be
// earned again on every later turn of the thread.
//
// A text source is a JSON string, and a JSON string is UTF-8 by definition, so a
// text document whose bytes are not UTF-8 would lose them to U+FFFD. There is
// nowhere on this API for those bytes to go, so it becomes the note as well.
func encodeDocument(c api.ContentDocument) (any, *cached, error) {
	b := &documentBlock{Type: "document", Title: c.Name}
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	switch {
	case mime == "application/pdf":
		b.Source = &base64Source{Type: "base64", MediaType: mime, Data: c.Data}
	case api.DocumentMime(mime).IsText():
		if !utf8.Valid(c.Data) {
			return omitted(c, fmt.Sprintf("a document of type %q is sent as text, its bytes are not UTF-8, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
		}
		b.Source = &textSource{Type: "text", MediaType: "text/plain", Data: string(c.Data)}
	default:
		return omitted(c, fmt.Sprintf("document type %q is not taken by this API, which reads PDF and text only, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
	}
	return b, &b.cached, nil
}

// encodeToolResult carries the text, images and documents this API takes in a
// result, and leaves the note omitted returns in place of anything else: a text
// block with no non whitespace content in it, an image or a document that
// arrived with no bytes in it, an image of a type this API does not take, a
// document encodeDocument cannot send, and whatever kind a later version of
// this protocol adds to the subset. The substitution is the
// one every other block gets, for the same reason. Refusing the block costs the whole request and still does not deliver
// it, where the note leaves the call answered and the drop reported.
//
// A result that carried no blocks at all takes the text lib.EmptyResult names
// instead of the note, so content is never the empty array: a tool that
// returned nothing, which api.NewContentToolResult answers with no blocks, is a
// result this API is not documented to accept, and it is stored like any other.
func encodeToolResult(c api.ContentToolResult) (*toolResultBlock, error) {
	content := make([]any, 0, len(c.Content))
	for _, rc := range c.Content {
		var block any
		switch rc := rc.(type) {
		case api.ContentText:
			if strings.TrimSpace(rc.Text) == "" {
				block, _, _ = omitted(rc, "a text block with no non-whitespace content in it is refused by this API, and the result is stored, so it would be refused again on every later turn of the thread")
				break
			}
			block = &textBlock{Type: "text", Text: rc.Text}
		case api.ContentImage:
			if len(rc.Data) == 0 {
				block, _, _ = omitted(rc, "an image source with no bytes in it is refused by this API, and the result is stored, so it would be refused again on every later turn of the thread")
				break
			}
			mime, ok := lib.ImageMime(rc.Mime)
			if !ok {
				block, _, _ = omitted(rc, fmt.Sprintf("an image of type %q is refused by this API, which takes image/jpeg, image/png, image/gif and image/webp, and the result is stored, so it would be refused again on every later turn of the thread", rc.Mime))
				break
			}
			if mime == "image/jpg" {
				mime = string(api.ImageJPEG)
			}
			block = &imageBlock{Type: "image", Source: base64Source{Type: "base64", MediaType: mime, Data: rc.Data}}
		case api.ContentDocument:
			if len(rc.Data) == 0 {
				block, _, _ = omitted(rc, "a document source with no bytes in it is refused by this API, and the result is stored, so it would be refused again on every later turn of the thread")
				break
			}
			var err error
			if block, _, err = encodeDocument(rc); err != nil {
				return nil, err
			}
		default:
			block, _, _ = omitted(rc, "a result on this API takes text, images and documents and carries nothing else")
		}
		if _, is := block.(*textBlock); is && len(content) > 0 {
			if prev, is := content[len(content)-1].(*textBlock); is && !strings.HasSuffix(prev.Text, "\n") {
				prev.Text += "\n"
			}
		}
		content = append(content, block)
	}
	if len(content) == 0 {
		content = append(content, &textBlock{Type: "text", Text: lib.EmptyResult})
	}
	return &toolResultBlock{
		Type:      "tool_result",
		ToolUseID: lib.CallID(c.ID),
		Content:   content,
		IsError:   c.Error != api.ToolOk,
	}, nil
}

// spliceField replaces the value of one field of a payload and leaves every
// other byte where it was. The payload is what the API checks for having been
// modified, so it is edited in place rather than decoded into a map and
// marshalled again, which reorders every key in it.
//
// A payload with no such field goes back as it stands. Only a block that streams
// a field collects fragments of one, and every block of this API that streams a
// field opened with it, so there is nothing to put anywhere else and nothing
// about the request to refuse.
func spliceField(raw json.RawMessage, key string, value json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(value) {
		return nil, fmt.Errorf("claude: %s of a replayed block is not valid JSON", key)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("claude: a replayed block is not a JSON object")
	}
	for dec.More() {
		name, err := dec.Token()
		if err != nil {
			return nil, err
		}
		start := dec.InputOffset()
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
		if name != key {
			continue
		}
		out := make([]byte, 0, len(raw)+len(value))
		out = append(out, raw[:start]...)
		out = append(out, ':')
		out = append(out, value...)
		return append(out, raw[dec.InputOffset():]...), nil
	}
	return raw, nil
}

func (p *Provider) cacheMark() *cacheControl {
	return &cacheControl{Type: "ephemeral", TTL: p.cacheTTL}
}
