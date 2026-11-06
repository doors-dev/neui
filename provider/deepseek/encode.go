package deepseek

import (
	"bytes"
	"encoding/base64"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// encode turns an api.Request into the request body.
//
// The shape it targets is the plain OpenAI one: content is a bare string on
// every role unless a user or tool message carries an image to deepseek-flash,
// so a message holding several blocks collapses into one string and the blocks
// that are not text become separate fields or separate messages. A block this wire has no field for leaves a note
// in its place and the turn goes out, stated at the arm it happens in; what is
// refused instead is a block this wire does carry, filled with something it does
// not take.
//
// Two things go out in neither form, and both are parts of a reasoning block
// rather than blocks: a stored summary, because reasoning_content is the
// thinking and this wire has no second field a paraphrase could go in, and all
// earlier thinking in a request without tools, because the server ignores it
// there. Neither leaves a note. A note stands in for a block that is gone, and
// the block is still there carrying everything this wire takes of it. Both are
// stated at encodeAssistant.
//
// thinking always goes out, since it is the field that decides when
// reasoning_effort is present too, and a disabled thinking never carries an
// effort. Temperature and top_p go out as set: New refused them where the
// model ignores them.
func (p *Provider) encode(req api.Request) (*chatRequest, error) {
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
// stays off the wire. A forced choice is refused while thinking is on, which is
// what the server answers it with a 400 for.
func (p *Provider) encodeTools(out *chatRequest, req api.Request) error {
	seen := make(map[string]bool, len(req.Tools))
	for _, t := range req.Tools {
		if err := checkToolName(t.Name); err != nil {
			return err
		}
		if seen[t.Name] {
			return reqErrf("tool name %q is declared twice, DeepSeek requires unique names", t.Name)
		}
		seen[t.Name] = true
		out.Tools = append(out.Tools, tool{
			Type: "function",
			Function: toolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  t.Schema,
			},
		})
	}
	name, named := req.ToolChoice.Tool()
	if p.effort != EffortNone && (named || req.ToolChoice == api.ToolChoiceAny) {
		return reqErrf("tool choice %q forces a tool call, which DeepSeek refuses in thinking mode; pass WithEffort(EffortNone) to force one", req.ToolChoice)
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

// checkToolName enforces the documented name rules. The server enforces them
// too, with a 400 that does not say which tool was wrong.
func checkToolName(name string) error {
	if name == "" {
		return &RequestError{Reason: "tool has no name"}
	}
	if len(name) > maxToolNameLen {
		return reqErrf("tool name %q is %d characters, DeepSeek allows %d", name, len(name), maxToolNameLen)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return reqErrf("tool name %q contains %q, DeepSeek allows only letters, digits, underscore and dash", name, r)
		}
	}
	return nil
}

// encodeMessages lowers the whole conversation. The system prompt is an ordinary
// leading message here; there is no top level field for it. Several blocks
// become one string on every role, joined with the blank line api.Content
// requires of every adapter that flattens a block list: a single newline runs
// the end of one block into the start of the next, and a list, a heading or a
// code fence loses its framing at exactly that join.
//
// discarded is read before any call is encoded, because api.ContentToolUse says
// the result answering a call is what says whether its arguments ever parsed.
// The whole thread is scanned for it first: a result sits in a message after the
// one carrying the call, so the call cannot be encoded on a single pass that has
// not seen it.
func (p *Provider) encodeMessages(req api.Request) ([]message, error) {
	discarded := make(map[string]bool)
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if r, ok := c.(api.ContentToolResult); ok && r.Error == api.ToolErrJSON {
				discarded[r.ID] = true
			}
		}
	}
	msgs := make([]message, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, message{Role: "system", Content: req.System})
	}
	for i, m := range req.Messages {
		var err error
		switch m.Role {
		case api.RoleUser:
			msgs, err = p.encodeUser(msgs, m, i)
		case api.RoleAssistant:
			msgs, err = p.encodeAssistant(msgs, m, i, discarded, len(req.Tools) > 0)
		default:
			err = reqErrf("message %d has role %q, which is neither user nor assistant", i, m.Role)
		}
		if err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

// encodeUser lowers one user message. Tool results are their own role here, so
// a message carrying several of them explodes into several messages, and they
// are emitted before the user text regardless of where they sat in the block
// list: results have to follow the assistant turn that asked for them with
// nothing in between, and a user message in the gap is rejected.
//
// A block this wire has no field for becomes a note among the text, which is
// where the note belongs.
func (p *Provider) encodeUser(msgs []message, m api.Message, at int) ([]message, error) {
	var parts []any
	var results []message
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			parts = append(parts, textPart{Type: "text", Text: c.Text})
		case api.ContentToolResult:
			results = append(results, p.toolResult(c))
		case api.ContentImage:
			parts = append(parts, p.image(c))
		case api.ContentDocument:
			parts = append(parts, note(c, "this adapter has no encoding for a document"))
		case api.ContentToolUse:
			parts = append(parts, note(c, "a tool call is a field of the assistant message that made it"))
		case api.ContentReasoning:
			parts = append(parts, note(c, "reasoning_content is a field of an assistant message"))
		case api.ContentOpaque:
			parts = append(parts, note(c, "DeepSeek runs no tools of its own, so this wire has no field that could carry a payload back"))
		default:
			parts = append(parts, textPart{Type: "text", Text: lib.Unknown(Name, at, c)})
		}
	}
	msgs = append(msgs, results...)
	if len(parts) > 0 || len(results) == 0 {
		msgs = append(msgs, message{Role: "user", Content: content(parts)})
	}
	return msgs, nil
}

// encodeAssistant lowers one assistant message into exactly one message.
// content, reasoning_content and tool_calls are sibling fields of the same
// object rather than blocks, so a turn that thought, spoke and called two tools
// is one message with three fields set.
//
// This adapter writes no Meta, because DeepSeek's reasoning is plain readable
// text in a field of its own, sealed behind nothing, so there is no encoding of
// it to replay verbatim and Thoughts is the whole of it. A block carrying Meta
// or a Summary is therefore one another provider issued, and its Thoughts are
// dropped with it rather than replayed as this model's own: reasoning_content
// is the thinking, this adapter never produces a summary to put in it, and
// there is no second field on this wire a paraphrase could go in.
//
// A block this wire has no field for leaves a note among the text, an opaque
// payload for the reason encodeUser gives.
//
// A call with no name is not a call. The field is required and naming nothing
// dispatches nothing, so a stored turn that lost the name is refused here rather
// than replayed as "name":"" for the model to make sense of.
//
// Arguments follow api.ContentToolUse: a call its paired result marks as
// api.ToolErrJSON had its arguments thrown away before they were ever JSON, so
// Input is not read at all and the empty object goes out, which is what the
// required field takes and what the paired tool message already explains. Input
// that is nothing goes out the same way, being a call that takes no arguments.
// api.DiscardedArguments with no such result is the pairing invariant broken: it
// is refused rather than flattened to {}, which would replay as a legitimate
// call that takes none.
//
// Reasoning goes back on every assistant turn exactly when the request declares
// tools, which is the case the server requires it in: with tools, every earlier
// turn's reasoning_content has to come back, including turns that called none,
// or the request is a 400. Without tools the server ignores the field and keeps
// none of it in the context, so sending it there would buy nothing. No model
// decides this, because both models follow the same rule. Several blocks
// join into one field: one turn produces one reasoning block, however often v4
// breaks off to speak, so several mean a caller merged turns, and the wire has
// one reasoning_content per message either way.
//
// Text joins with the blank line every other role here joins with. One turn of
// this adapter produces exactly one text block, so several mean a caller merged
// turns, and a note left where an unsendable block was is among them: joining
// with nothing glued the note to the model's own words.
//
// content is documented as string or null, and null is what a turn that only
// called tools has to send.
func (p *Provider) encodeAssistant(msgs []message, m api.Message, at int, discarded map[string]bool, tools bool) ([]message, error) {
	var text, reasoning []string
	var calls []toolCall
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			text = append(text, c.Text)
		case api.ContentReasoning:
			if len(c.Meta) == 0 && len(c.Summary) == 0 {
				reasoning = append(reasoning, c.Thoughts.Ordered()...)
			}
		case api.ContentToolUse:
			if c.Name == "" {
				return nil, reqErrf("message %d replays tool call %q with no name, which names no tool", at, c.ID)
			}
			args := "{}"
			switch {
			case discarded[c.ID]:
			case bytes.Equal(c.Input, api.DiscardedArguments):
				return nil, reqErrf("message %d replays tool call %q whose arguments were discarded, with no failed result answering it", at, c.ID)
			case len(c.Input) > 0:
				args = string(c.Input)
			}
			calls = append(calls, toolCall{
				ID:   c.ID,
				Type: "function",
				Function: toolCallFunction{
					Name:      c.Name,
					Arguments: args,
				},
			})
		case api.ContentImage:
			text = append(text, lib.Omitted(c, "DeepSeek refuses an image in an assistant message"))
		case api.ContentDocument:
			text = append(text, lib.Omitted(c, "this adapter has no encoding for a document"))
		case api.ContentToolResult:
			text = append(text, lib.Omitted(c, "a tool result is a message of the tool role and belongs to the user turn that answers the call"))
		case api.ContentOpaque:
			text = append(text, lib.Omitted(c, "DeepSeek runs no tools of its own, so this wire has no field that could carry a payload back"))
		default:
			text = append(text, lib.Unknown(Name, at, c))
		}
	}
	msg := message{Role: "assistant", ToolCalls: calls}
	if tools {
		msg.ReasoningContent = strings.Join(reasoning, "")
	}
	switch {
	case len(text) > 0:
		msg.Content = strings.Join(text, "\n\n")
	case len(calls) > 0:
	default:
		msg.Content = ""
	}
	return append(msgs, msg), nil
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
func (p *Provider) toolResult(c api.ContentToolResult) message {
	parts := make([]any, 0, len(c.Content))
	for _, rc := range c.Content {
		switch rc := rc.(type) {
		case api.ContentText:
			parts = append(parts, textPart{Type: "text", Text: rc.Text})
		case api.ContentImage:
			parts = append(parts, p.image(rc))
		default:
			parts = append(parts, note(rc, "a DeepSeek tool result carries text and images only"))
		}
	}
	body := content(parts)
	if c.Error != api.ToolOk {
		if s, ok := body.(string); ok {
			body = "Error: " + s
		} else {
			body = append([]any{textPart{Type: "text", Text: "Error:"}}, parts...)
		}
	}
	return message{Role: "tool", ToolCallID: c.ID, Content: body}
}

func (p *Provider) image(c api.ContentImage) any {
	if p.family != familyFlash {
		return note(c, "only deepseek-flash reads images")
	}
	if len(c.Data) == 0 {
		return note(c, "an image with no bytes in it is refused by the server")
	}
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	if !api.ImageMime(mime).IsValid() {
		return note(c, "the server takes JPEG, PNG, GIF and WebP only")
	}
	return imagePart{Type: "image_url", ImageURL: imageURL{URL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(c.Data)}}
}

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
