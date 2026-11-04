package deepseek

import (
	"bytes"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// encode turns an api.Request into the request body.
//
// The shape it targets is the plain OpenAI one: content is a bare string on
// every role, not a typed block array, so a message holding several blocks
// collapses into one string and the blocks that are not text become separate
// fields or separate messages. A block this wire has no field for leaves a note
// in its place and the turn goes out, stated at the arm it happens in; what is
// refused instead is a block this wire does carry, filled with something it does
// not take.
//
// Two things go out in neither form, and both are parts of a reasoning block
// rather than blocks: a stored summary, because reasoning_content is the
// thinking and this wire has no second field a paraphrase could go in, and the
// thinking of a turn that called no tool, because the server ignores it there
// and bills for it. Neither leaves a note. A note stands in for a block that is
// gone, and the block is still there carrying everything this wire takes of it.
// Both are stated at encodeAssistant.
//
// Sampling only reaches the model with thinking off, and thinking is on unless
// the caller said otherwise, so it is sent only then. Sending it anyway is
// documented as harmless, but an inert field in the body is a field a reader has
// to work out, and it perturbs the cached prefix for nothing.
func (p *Provider) encode(req api.Request) (*chatRequest, error) {
	if p.model.Name == "" {
		return nil, &RequestError{Reason: "provider has no model, set Conf.Model"}
	}
	out := &chatRequest{
		Model:         p.model.Name,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
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
	th := &thinking{Type: "disabled"}
	if p.thinking {
		th.Type = "enabled"
	}
	out.Thinking = th
	if p.effort != "" {
		if p.effortTopLevel {
			out.ReasoningEffort = string(p.effort)
		} else {
			th.Effort = string(p.effort)
		}
	}
	if !p.thinking {
		out.Temperature = p.temperature
		out.TopP = p.topP
	}
	return out, nil
}

// encodeTools declares the tools and the choice. An unset choice means let the
// server decide, which is not the same as "auto" on every host, so the field
// stays off the wire.
func (p *Provider) encodeTools(out *chatRequest, req api.Request) error {
	if len(req.Tools) > maxTools {
		return reqErrf("%d tools, DeepSeek takes at most %d", len(req.Tools), maxTools)
	}
	for _, t := range req.Tools {
		if err := checkToolName(t.Name); err != nil {
			return err
		}
		out.Tools = append(out.Tools, tool{
			Type: "function",
			Function: toolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  t.Schema,
			},
		})
	}
	if name, ok := req.ToolChoice.Tool(); ok {
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
		msgs = append(msgs, message{Role: "system", Content: str(req.System)})
	}
	for i, m := range req.Messages {
		var err error
		switch m.Role {
		case api.RoleUser:
			msgs, err = p.encodeUser(msgs, m, i)
		case api.RoleAssistant:
			msgs, err = p.encodeAssistant(msgs, m, i, discarded)
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
// where the note belongs: it is the only thing a user message on this wire
// carries.
func (p *Provider) encodeUser(msgs []message, m api.Message, at int) ([]message, error) {
	var text []string
	var results []message
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			text = append(text, c.Text)
		case api.ContentToolResult:
			results = append(results, toolResult(c, at))
		case api.ContentImage, api.ContentDocument:
			text = append(text, lib.Omitted(Name, at, c, "content is a bare string on this wire and this adapter has no encoding for an attachment"))
		case api.ContentToolUse:
			text = append(text, lib.Omitted(Name, at, c, "a tool call is a field of the assistant message that made it"))
		case api.ContentReasoning:
			text = append(text, lib.Omitted(Name, at, c, "reasoning_content is a field of an assistant message"))
		case api.ContentOpaque:
			text = append(text, lib.Omitted(Name, at, c, "DeepSeek runs no tools of its own, so this wire has no field that could carry a payload back"))
		default:
			text = append(text, lib.Omitted(Name, at, c, "this version of the adapter has no field for it"))
		}
	}
	msgs = append(msgs, results...)
	if len(text) > 0 || len(results) == 0 {
		msgs = append(msgs, message{Role: "user", Content: str(strings.Join(text, "\n\n"))})
	}
	return msgs, nil
}

// encodeAssistant lowers one assistant message into exactly one message.
// content, reasoning_content and tool_calls are sibling fields of the same
// object rather than blocks, so a turn that thought, spoke and called two tools
// is one message with three fields set.
//
// Reasoning is collected rather than decided on where it is read: whether it
// goes back turns on whether this same message called a tool, and that is not
// known until every block of it has been read. Meta is not read, and this
// adapter writes none, because DeepSeek's reasoning is plain readable text in a
// field of its own, sealed behind nothing, so there is no encoding of it to
// replay verbatim and Thoughts is the whole of it. A stored summary is dropped:
// reasoning_content is the thinking, this adapter never produces a summary to
// put in it, and there is no second field on this wire a paraphrase could go in.
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
// Reasoning goes back exactly when the turn called a tool, which is the case the
// server requires it in: a replayed tool call whose reasoning_content is missing
// is a 400 saying so by name. On every other turn the server ignores the field,
// so sending it there would buy nothing and be billed for. No model decides
// this, because there are two models, no snapshots and no aliases, so a
// comparison against the model being asked for separates nothing. Several blocks
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
func (p *Provider) encodeAssistant(msgs []message, m api.Message, at int, discarded map[string]bool) ([]message, error) {
	var text, reasoning []string
	var calls []toolCall
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			text = append(text, c.Text)
		case api.ContentReasoning:
			reasoning = append(reasoning, c.Thoughts.Ordered()...)
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
		case api.ContentImage, api.ContentDocument:
			text = append(text, lib.Omitted(Name, at, c, "content is a bare string on this wire and this adapter has no encoding for an attachment"))
		case api.ContentToolResult:
			text = append(text, lib.Omitted(Name, at, c, "a tool result is a message of the tool role and belongs to the user turn that answers the call"))
		case api.ContentOpaque:
			text = append(text, lib.Omitted(Name, at, c, "DeepSeek runs no tools of its own, so this wire has no field that could carry a payload back"))
		default:
			text = append(text, lib.Omitted(Name, at, c, "this version of the adapter has no field for it"))
		}
	}
	msg := message{Role: "assistant", ToolCalls: calls}
	if len(calls) > 0 {
		msg.ReasoningContent = strings.Join(reasoning, "")
	}
	switch {
	case len(text) > 0:
		msg.Content = str(strings.Join(text, "\n\n"))
	case len(calls) > 0:
	default:
		msg.Content = str("")
	}
	return append(msgs, msg), nil
}

// toolResult lowers one result into a tool role message.
//
// There is no error flag anywhere in that message: role, tool_call_id and a
// string is the whole of it. So api.ToolError reaches the model as a prefix on
// the text or not at all, and the three values collapse to two here, since what
// the model does about a failure is in the text either way. api.ToolErrJSON is
// how api.DiscardedArgumentsError gets the model to reissue a call whose
// arguments were thrown away.
//
// What that string has no room for leaves the note among the text, the way a
// block of a message does: refusing here costs the whole request and delivers
// the image no more than the note does, and the result is the answer a call is
// waiting on, so failing it fails the conversation too.
func toolResult(c api.ContentToolResult, at int) message {
	parts := make([]string, 0, len(c.Content))
	for _, rc := range c.Content {
		switch rc := rc.(type) {
		case api.ContentText:
			parts = append(parts, rc.Text)
		default:
			parts = append(parts, lib.Omitted(Name, at, rc, "a DeepSeek tool result is a bare string and carries nothing but text"))
		}
	}
	body := strings.Join(parts, "\n\n")
	if c.Error != api.ToolOk {
		body = "Error: " + body
	}
	return message{Role: "tool", ToolCallID: c.ID, Content: str(body)}
}

func str(s string) *string {
	return &s
}
