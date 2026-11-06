package claude

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

func (p *Provider) encode(req api.Request) (*lib.Body, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("claude: request has no messages")
	}
	w := request{
		Model:         p.model.Name,
		MaxTokens:     p.maxToken,
		Stream:        true,
		StopSequences: p.stopSeqs,
	}
	if p.thinking.Type != "" {
		w.Thinking = &thinkingWire{
			Type:    string(p.thinking.Type),
			Budget:  p.thinking.Budget,
			Display: string(p.thinking.Display),
		}
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
	return lib.MarshalBody(w)
}

var forcedToolRefused = []string{"opus-5-5", "sonnet-5-5", "fable-5-1", "mythos-5-1"}

// encodeToolChoice maps the choice onto this API's, and refuses the two cases
// the API refuses forcing a tool in: on the forcedToolRefused models on every
// request, and alongside manual extended thinking on any other, because adaptive
// thinking accepts it there, including on models that think by default.
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
	if tc.Type != "any" && tc.Type != "tool" {
		return tc, nil
	}
	for _, m := range forcedToolRefused {
		if strings.Contains(p.model.Name, m) {
			return nil, fmt.Errorf("claude: model %q refuses tool choice %q; use auto with strict tools, or structured outputs", p.model.Name, choice)
		}
	}
	if p.thinking.Type == ThinkingEnabled {
		return nil, fmt.Errorf("claude: tool choice %q is not accepted with extended thinking; use adaptive thinking or leave the choice to the model", choice)
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
			return nil, fmt.Errorf("claude: tool has no name")
		}
		schema, err := encodeSchema(t.Schema)
		if err != nil {
			return nil, fmt.Errorf("claude: tool %q schema: %w", t.Name, err)
		}
		out = append(out, tool{Name: t.Name, Description: t.Desc, InputSchema: schema})
	}
	if p.marks(CacheTools) {
		out[len(out)-1].CacheControl = p.cacheMark()
	}
	return out, nil
}

// encodeSchema fills in an empty object schema, because input_schema is
// required and a tool that takes no arguments still has to declare that.
func encodeSchema(schema any) (json.RawMessage, error) {
	if schema == nil {
		return json.RawMessage(`{"type":"object","properties":{}}`), nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("schema is not a JSON object")
	}
	return raw, nil
}

func (p *Provider) encodeSystem(system string) ([]json.RawMessage, error) {
	if system == "" {
		return nil, nil
	}
	raw, err := json.Marshal(textBlock{Type: "text", Text: system})
	if err != nil {
		return nil, err
	}
	if p.marks(CacheSystem) {
		if raw, err = injectCacheControl(raw, *p.cacheMark()); err != nil {
			return nil, err
		}
	}
	return []json.RawMessage{raw}, nil
}

// encodeMessages keeps the caller's arrangement. A tool result is a block inside
// a user message here, so wherever it was written within that message is where
// it goes, and this API has an is_error field for one that failed rather than a
// convention to invent. Both of api.ToolError's failures set it: the field says
// a call did not produce what was asked of it, and the classification telling
// the two apart is for whoever routes the result rather than for the model,
// which reads the text in the result either way.
//
// Which results carry api.ToolErrJSON is collected before the first block is
// encoded, because a tool call is encoded against the result that answers it and
// that result is in a later message. A call whose arguments were discarded is
// the one case where the call's own Input holds nothing sendable, so the pairing
// is what licenses the empty object. See encodeContent.
//
// Which message it was written in is the one thing the arrangement cannot carry:
// tool_result is a block of the user turn on this API and an assistant message
// has no room for one, so a result written into an assistant message leaves the
// note omitted returns. Moving it would be inventing a turn the caller did not
// write. The role is passed down for that decision.
//
// A cache marker never goes on a block that has to be replayed verbatim, so the
// last block that can carry one is tracked separately from the last one overall.
func (p *Provider) encodeMessages(msgs []api.Message) ([]message, error) {
	discarded := map[string]bool{}
	for _, m := range msgs {
		for _, c := range m.Content {
			if r, is := c.(api.ContentToolResult); is && r.Error == api.ToolErrJSON {
				discarded[r.ID] = true
			}
		}
	}
	out := make([]message, 0, len(msgs))
	for i, m := range msgs {
		role := string(m.Role)
		if role != string(api.RoleUser) && role != string(api.RoleAssistant) {
			return nil, fmt.Errorf("claude: message %d has role %q", i, m.Role)
		}
		blocks := make([]json.RawMessage, 0, len(m.Content))
		markable := -1
		replay := replayThinking(m)
		for j, c := range m.Content {
			raw, ok, err := p.encodeContent(c, role, replay, discarded, i)
			if err != nil {
				return nil, fmt.Errorf("claude: message %d block %d: %w", i, j, err)
			}
			if ok {
				markable = len(blocks)
			}
			blocks = append(blocks, raw)
		}
		if len(blocks) == 0 {
			return nil, fmt.Errorf("claude: message %d has no content, and an empty content array is not a message", i)
		}
		if i == len(msgs)-1 && markable >= 0 && p.marks(CacheMessages) {
			marked, err := injectCacheControl(blocks[markable], *p.cacheMark())
			if err != nil {
				return nil, err
			}
			blocks[markable] = marked
		}
		out = append(out, message{Role: role, Content: blocks})
	}
	return out, nil
}

// replayThinking decides, once for a whole message, whether its thinking goes
// back with it. Every thinking and redacted_thinking block of the message
// replays or none does, and that is the whole rule.
//
// Models are not compared. This API ignores or drops thinking the target model
// cannot read instead of rejecting the request, so a block another Claude model
// wrote goes back like this model's own. Partial dropping is the part that is
// refused:
// within the latest assistant message the sequence of consecutive thinking
// blocks has to match what the model generated, and anything else comes back as
// "`thinking` or `redacted_thinking` blocks in the latest assistant message
// cannot be modified". So one block that cannot go settles the message.
//
// A block that cannot go is one Anthropic did not issue: its meta is neither a
// signature, a JSON string, nor a redacted_thinking block. That covers another
// provider's sealed payload and a block that arrived unsigned, which this API
// rejects and which has nothing to be replayed by. Both are counted here rather
// than at the block, because a message that drops either one has to drop the
// rest with it.
func replayThinking(m api.Message) bool {
	for _, c := range m.Content {
		r, is := c.(api.ContentReasoning)
		if !is {
			continue
		}
		var signature string
		if json.Unmarshal(r.Meta, &signature) == nil {
			if signature == "" {
				return false
			}
			continue
		}
		var block struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(r.Meta, &block) != nil || block.Type != "redacted_thinking" {
			return false
		}
	}
	return true
}

// encodeContent returns the wire block and whether a cache marker may be
// attached to it. Content this API cannot send becomes the note omitted returns,
// in the place the block held: a text block with no non whitespace content in
// it, an image or a document that arrived with no bytes in it, an image of a
// type this API does not take, a document in an assistant message or one
// encodeDocument cannot send, thinking the message does not replay, a payload
// this API did not issue, a payload whose streamed fragments never parsed back
// into it, and a tool result
// in an assistant message. A type this version
// of the adapter has no case for becomes the note lib.Unknown returns.
//
// An empty text block and an attachment this API refuses are notes for the
// thread's sake rather than the API's. This API refuses each outright, and a
// user message and a tool result are both stored and replayed, so one sent as a
// block would be refused again on every later turn of the thread rather than on
// this one alone — a 400 no caller can act on, since shedding media does not
// reach it and nothing short of a rewind drops the block. The attachment is
// checked before the mime type, because a block that cannot be sent whatever it
// claims to be is not a request worth refusing.
//
// A tool call is read against the result that answers it. Where that result
// carries api.ToolErrJSON the call's arguments were discarded before they were
// stored, so Input is not read at all: the API requires an object, and the failed
// result is what tells the model what happened, so an empty object is the honest
// stand in for arguments that no longer exist. Otherwise Input goes out as it
// stands, and nil — a call the provider sent no arguments for — becomes the same
// empty object because this API has no other shape for one.
//
// A call holding api.DiscardedArguments that no such result answers is the one
// tool call refused rather than noted. The marker says the arguments are gone
// and the missing result says nothing happened to them, and sending {} between
// those two would replay as a call that legitimately took none. It is an
// invariant of whoever dispatched the calls, so a thread that breaks it is a
// caller's defect and not content this API cannot carry.
func (p *Provider) encodeContent(c api.Content, role string, replay bool, discarded map[string]bool, at int) (json.RawMessage, bool, error) {
	switch c := c.(type) {
	case api.ContentText:
		if strings.TrimSpace(c.Text) == "" {
			return omitted(c, "a text block with no non-whitespace content in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		raw, err := json.Marshal(textBlock{Type: "text", Text: c.Text})
		return raw, true, err
	case api.ContentImage:
		if len(c.Data) == 0 {
			return omitted(c, "an image source with no bytes in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		if !api.ImageMime(c.Mime).IsValid() {
			return omitted(c, fmt.Sprintf("an image of type %q is refused by this API, which takes image/jpeg, image/png, image/gif and image/webp, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
		}
		raw, err := json.Marshal(imageBlock{
			Type: "image",
			Source: mediaSource{
				Type:      "base64",
				MediaType: c.Mime,
				Data:      base64.StdEncoding.EncodeToString(c.Data),
			},
		})
		return raw, true, err
	case api.ContentDocument:
		if role == string(api.RoleAssistant) {
			return omitted(c, "a document is a block of the user turn on this API, which refuses one placed directly in an assistant message")
		}
		if len(c.Data) == 0 {
			return omitted(c, "a document source with no bytes in it is refused by this API, and the block is stored, so it would be refused again on every later turn of the thread")
		}
		return encodeDocument(c)
	case api.ContentToolUse:
		input := c.Input
		switch {
		case discarded[c.ID]:
			input = json.RawMessage(`{}`)
		case bytes.Equal(input, api.DiscardedArguments):
			return nil, false, fmt.Errorf("tool call %q holds discarded arguments and no result answers it as %s, so there are no arguments to send and an empty object would replay as a call that took none", c.ID, api.ToolErrJSON)
		case len(input) == 0:
			input = json.RawMessage(`{}`)
		}
		raw, err := json.Marshal(toolUseBlock{Type: "tool_use", ID: c.ID, Name: c.Name, Input: input})
		return raw, true, err
	case api.ContentToolResult:
		if role == string(api.RoleAssistant) {
			return omitted(c, "a tool result is a block of the user turn that answers the call, and an assistant message on this API takes none")
		}
		raw, err := encodeToolResult(c)
		return raw, true, err
	case api.ContentReasoning:
		return encodeReasoning(c, replay)
	case api.ContentOpaque:
		return encodeOpaque(c)
	}
	raw, err := json.Marshal(textBlock{Type: "text", Text: lib.Unknown(providerName, at, c)})
	return raw, true, err
}

// omitted is the note that stands in for content this API cannot send. It is a
// text block because text is the one block both roles take here, so the note
// lands where the content was whichever message it came from, and it is markable
// for the same reason it is placeable: nothing about it has to be replayed
// unchanged.
func omitted(c api.Content, why string) (json.RawMessage, bool, error) {
	raw, err := json.Marshal(textBlock{Type: "text", Text: lib.Omitted(c, why)})
	return raw, true, err
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
func encodeDocument(c api.ContentDocument) (json.RawMessage, bool, error) {
	b := documentBlock{Type: "document", Title: c.Name}
	switch c.Mime {
	case "application/pdf":
		b.Source = mediaSource{
			Type:      "base64",
			MediaType: c.Mime,
			Data:      base64.StdEncoding.EncodeToString(c.Data),
		}
	case "text/plain", "text/markdown", "text/csv", "text/html", "text/xml",
		"application/json", "application/yaml", "application/rtf":
		if !utf8.Valid(c.Data) {
			return omitted(c, fmt.Sprintf("a document of type %q is sent as text, its bytes are not UTF-8, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
		}
		b.Source = mediaSource{Type: "text", MediaType: "text/plain", Data: string(c.Data)}
	default:
		return omitted(c, fmt.Sprintf("document type %q is not taken by this API, which reads PDF and text only, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime))
	}
	raw, err := json.Marshal(b)
	return raw, true, err
}

// encodeToolResult carries the text and images this API takes in a result, and
// leaves the note omitted returns in place of anything else: a text block with
// no non whitespace content in it, an image that arrived with no bytes in it or
// with a type this API does not take, a document today, and whatever kind a
// later version of this protocol adds to the subset. The substitution is the
// one every other block gets, for the same reason. Refusing the block costs the whole request and still does not deliver
// it, where the note leaves the call answered and the drop reported.
//
// A result that carried no blocks at all takes the text lib.EmptyResult names
// instead of the note, so content is never the empty array: a tool that
// returned nothing, which api.NewContentToolResult answers with no blocks, is a
// result this API is not documented to accept, and it is stored like any other.
//
// A result with no tool use id is refused instead, because that is not content
// this API cannot carry: it is a result matched to no call, and there is nothing
// for the model to read it against.
func encodeToolResult(c api.ContentToolResult) (json.RawMessage, error) {
	if c.ID == "" {
		return nil, fmt.Errorf("tool result has no tool use id")
	}
	content := make([]json.RawMessage, 0, len(c.Content))
	for _, rc := range c.Content {
		var raw json.RawMessage
		var err error
		switch rc := rc.(type) {
		case api.ContentText:
			if strings.TrimSpace(rc.Text) == "" {
				raw, _, err = omitted(rc, "a text block with no non-whitespace content in it is refused by this API, and the result is stored, so it would be refused again on every later turn of the thread")
				break
			}
			raw, err = json.Marshal(textBlock{Type: "text", Text: rc.Text})
		case api.ContentImage:
			if len(rc.Data) == 0 {
				raw, _, err = omitted(rc, "an image source with no bytes in it is refused by this API, and the result is stored, so it would be refused again on every later turn of the thread")
				break
			}
			if !api.ImageMime(rc.Mime).IsValid() {
				raw, _, err = omitted(rc, fmt.Sprintf("an image of type %q is refused by this API, which takes image/jpeg, image/png, image/gif and image/webp, and the result is stored, so it would be refused again on every later turn of the thread", rc.Mime))
				break
			}
			raw, err = json.Marshal(imageBlock{
				Type: "image",
				Source: mediaSource{
					Type:      "base64",
					MediaType: rc.Mime,
					Data:      base64.StdEncoding.EncodeToString(rc.Data),
				},
			})
		default:
			raw, _, err = omitted(rc, "a result on this API takes text and images and carries nothing else")
		}
		if err != nil {
			return nil, err
		}
		content = append(content, raw)
	}
	if len(content) == 0 {
		raw, err := json.Marshal(textBlock{Type: "text", Text: lib.EmptyResult})
		if err != nil {
			return nil, err
		}
		content = append(content, raw)
	}
	return json.Marshal(toolResultBlock{
		Type:      "tool_result",
		ToolUseID: c.ID,
		Content:   content,
		IsError:   c.Error != api.ToolOk,
	})
}

// encodeReasoning replays thinking. Meta holds one of two shapes here and the
// branch is on which one arrived: a JSON string is the signature of a thinking
// block, and the envelope this API replays one by is built back around it; a
// JSON object is a redacted_thinking block, which is a wire block this API sent
// and goes back byte for byte. A signature is stored alone because the envelope
// is fabricated either way — the readable half streams out of a field the
// signature frame does not carry — while a redacted block arrived whole.
//
// Which of the two readable fields holds the text is what the caller configured
// display as on the turn that produced the block, and both stream out of the
// same thinking field, so whichever is filled is what goes into it. A summary is
// the model's thinking as far as this API is concerned: the signature was issued
// over the block the API sent, and it sent the summary. An envelope with nothing
// readable in it is normal and replays as it is: that is what a model whose
// thinking display is omitted returns, and dropping one is the documented way to
// break a tool use turn.
//
// replay is the message's decision, taken before any of its blocks were
// encoded, because this API refuses a message whose thinking was partly kept.
// See replayThinking. A message that does not replay leaves a note in place of
// each of its thinking blocks, rather than the gap a model reads as a turn that
// never thought. The readable half is not sent as the assistant's text: it is
// another provider's thinking, or a summary this API wrote, and either one replayed
// as words this model spoke is a fabrication.
func encodeReasoning(c api.ContentReasoning, replay bool) (json.RawMessage, bool, error) {
	if !replay {
		return omitted(c, "the message holds thinking another provider issued or thinking that arrived unsigned, and this API refuses a message whose thinking was partly kept")
	}
	var signature string
	if json.Unmarshal(c.Meta, &signature) != nil {
		return c.Meta, false, nil
	}
	readable := c.Thoughts.Ordered()
	if len(readable) == 0 {
		readable = c.Summary.Ordered()
	}
	raw, err := lib.MarshalRaw(thinkingBlock{
		Type:      "thinking",
		Thinking:  strings.Join(readable, ""),
		Signature: signature,
	})
	return raw, false, err
}

// encodeOpaque replays a payload this protocol has no type for.
//
// What this API needs handed back is the bytes it sent. It signs the blocks it
// issues and checks them for having been modified, and a signature covers bytes
// rather than a value, so Data goes back byte for byte: decoding it and
// marshalling it again keeps what it means and reorders the keys it is checked
// by. That is this adapter's requirement and not a rule about opaque payloads
// anywhere else. Deltas are the fragments of the one field this API streams
// inside such a payload, the arguments of a tool it ran itself, and they are
// spliced back into it without the rest of it being decoded.
//
// Models are not compared, as replayThinking does not compare them: the API
// restores what it sealed in a server tool's result whichever Claude model reads
// it. A payload this API did not issue leaves a note in its place: one that is
// not a JSON object whose type is its Kind, or one that carries an id without
// being a *_use block, which is the shape of a Responses item, since every
// Responses item carries an id and the only Messages blocks that do are tool
// calls. So does one the splice cannot be done on. A turn cut inside a server
// tool's arguments stores fragments that never became JSON, and then there is
// neither anything to put back nor a half written field worth sending: the
// placeholder the payload opened with would replay as a call made with
// arguments it never had. Failing the request instead would end the thread on
// stored history the caller cannot rewrite, over one block that is already lost.
func encodeOpaque(c api.ContentOpaque) (json.RawMessage, bool, error) {
	var block struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if json.Unmarshal(c.Data, &block) != nil || block.Type == "" || block.Type != c.Kind || block.ID != "" && !strings.HasSuffix(block.Type, "_use") {
		return omitted(c, "the payload was issued by another provider")
	}
	if len(c.Deltas) == 0 {
		return c.Data, false, nil
	}
	raw, err := spliceField(c.Data, "input", json.RawMessage(strings.Join(c.Deltas, "")))
	if err != nil {
		return omitted(c, "the fragments this block collected never parsed as JSON, or the payload holding the field they belong in is not the object this API sent, and either way there is nothing to splice back into it")
	}
	return raw, false, nil
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

func (p *Provider) marks(b Breakpoint) bool {
	for _, have := range p.breakpoints {
		if have == b {
			return true
		}
	}
	return false
}

func (p *Provider) cacheMark() *cacheControl {
	return &cacheControl{Type: "ephemeral", TTL: p.cacheTTL}
}

// injectCacheControl splices the field in rather than decoding and re-encoding
// the block, so the rest of the bytes are the ones that were marshalled.
func injectCacheControl(block json.RawMessage, cc cacheControl) (json.RawMessage, error) {
	if len(block) == 0 || block[0] != '{' {
		return nil, fmt.Errorf("claude: cannot mark a block that is not a JSON object")
	}
	enc, err := json.Marshal(cc)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(block)+len(enc)+len(`"cache_control":,`))
	out = append(out, '{')
	out = append(out, `"cache_control":`...)
	out = append(out, enc...)
	rest := block[1:]
	if strings.TrimSpace(string(rest)) != "}" {
		out = append(out, ',')
	}
	out = append(out, rest...)
	return out, nil
}
