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

package gpt

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

// messageMeta is what this adapter stores on the first text block of an
// assistant message. Phase labels a preamble before a tool call as distinct from
// a final answer and the codex models want it preserved across replay; protocol
// has no field for it. Model is which model made that claim, because a phase is
// one model's label and no model is stamped on a text block.
type messageMeta struct {
	Model string `json:"model,omitempty"`
	Phase string `json:"phase"`
}

type itemKind int

const (
	kindMessage itemKind = iota
	kindFunctionCall
	kindFunctionOutput
	kindReasoning
	kindOpaque
)

// follows says an item of this kind is one the service pairs with the reasoning
// item in front of it. A call the caller answers and a tool the service ran
// itself are both items the thinking produced, and the refusal names the pair by
// position rather than by kind, so a reasoning item in front of either is one
// ReplayToolTurns sends.
func (k itemKind) follows() bool {
	return k == kindFunctionCall || k == kindOpaque
}

type inputItem struct {
	kind itemKind
	v    any
}

// encode builds the body of one turn.
//
// A known reasoning model is asked for reasoning.encrypted_content unless the caller
// replaced the include list. Stateless mode populates it on its own now, and the
// include value is still accepted, still documented and still what older models
// need. Sending it costs a string.
func (p *Provider) encode(req api.Request) (*requestBody, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("gpt: request has no messages")
	}
	model, set := p.model.Name, p.set
	b := &requestBody{
		Model:         model,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeObfuscation: set.obfuscation},
		Store:         set.store,
		ServiceTier:   set.serviceTier,
		Metadata:      set.metadata,
	}
	switch {
	case set.include != nil:
		b.Include = set.include
	default:
		if _, known := declared[model]; known {
			b.Include = []string{"reasoning.encrypted_content"}
		}
	}
	b.MaxOutputTokens = set.maxTokens
	if req.System != "" {
		b.Instructions = req.System
	}
	tools, err := encodeTools(set, req.Tools)
	if err != nil {
		return nil, err
	}
	b.Tools = tools
	b.ToolChoice = encodeToolChoice(req.ToolChoice)
	b.ParallelToolCalls = set.parallelToolCalls
	if r := (reasoningConfig{
		Effort:  set.effort,
		Summary: set.summary,
		Mode:    set.reasoningMode,
		Context: set.reasoningContext,
	}); r != (reasoningConfig{}) {
		b.Reasoning = &r
	}
	if set.verbosity != "" {
		b.Text = &textConfig{Verbosity: set.verbosity}
	}
	b.Temperature = set.temperature
	b.TopP = set.topP
	b.PromptCacheKey = set.promptCacheKey
	if set.cacheMode != "" || set.cacheTTL != "" {
		b.PromptCacheOpts = &promptCacheConfig{Mode: set.cacheMode, TTL: set.cacheTTL}
	}
	in := &inputBuilder{set: set, model: model, results: map[string]api.ToolError{}, nameless: map[string]bool{}}
	for _, m := range req.Messages {
		for _, c := range m.Content {
			switch c := c.(type) {
			case api.ContentToolResult:
				in.results[c.ID] = c.Error
			case api.ContentToolUse:
				if c.Name == "" {
					in.nameless[c.ID] = true
				}
			}
		}
	}
	for i, m := range req.Messages {
		var err error
		switch m.Role {
		case api.RoleUser:
			err = in.user(m, i)
		case api.RoleAssistant:
			err = in.assistant(m, i)
		default:
			err = fmt.Errorf("gpt: message %d has role %q", i, m.Role)
		}
		if err != nil {
			return nil, err
		}
	}
	b.Input = in.finish()
	return b, nil
}

// encodeTools declares the caller's tools. A missing schema is a tool that takes
// nothing, not a tool whose arguments are unconstrained, and null reads as the
// second, so an empty object schema goes out in its place.
func encodeTools(set settings, tools []api.Tool) ([]toolDef, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]toolDef, 0, len(tools))
	for _, t := range tools {
		if t.Name == "" {
			return nil, errors.New("gpt: tool has no name")
		}
		params := t.Schema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
		} else {
			raw, err := json.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("gpt: tool %q schema: %w", t.Name, err)
			}
			var schema any
			if err := json.Unmarshal(raw, &schema); err != nil {
				return nil, fmt.Errorf("gpt: tool %q schema: %w", t.Name, err)
			}
			dropFormats(schema)
			params = schema
		}
		out = append(out, toolDef{
			Type:        "function",
			Name:        t.Name,
			Description: t.Desc,
			Parameters:  params,
			Strict:      set.strictTools,
		})
	}
	return out, nil
}

var strictFormats = map[string]bool{
	"date-time": true, "time": true, "date": true, "duration": true, "email": true,
	"hostname": true, "ipv4": true, "ipv6": true, "uuid": true,
}

func dropFormats(v any) {
	switch v := v.(type) {
	case map[string]any:
		if f, ok := v["format"].(string); ok && !strictFormats[f] {
			delete(v, "format")
		}
		for _, child := range v {
			dropFormats(child)
		}
	case []any:
		for _, child := range v {
			dropFormats(child)
		}
	}
}

func encodeToolChoice(c api.ToolChoice) any {
	if name, ok := c.Tool(); ok {
		return toolChoiceFunction{Type: "function", Name: name}
	}
	switch c {
	case api.ToolChoiceAuto:
		return "auto"
	case api.ToolChoiceNone:
		return "none"
	case api.ToolChoiceAny:
		return "required"
	}
	return nil
}

type inputBuilder struct {
	set      settings
	model    string
	results  map[string]api.ToolError
	nameless map[string]bool
	items    []inputItem
}

func (b *inputBuilder) finish() []any {
	out := make([]any, len(b.items))
	for i, it := range b.items {
		out[i] = it.v
	}
	return out
}

// user turns one user message into items, in the order the caller wrote them.
// A tool call and its result are top level items rather than blocks inside a
// message, but the input array is flat and takes them in any order that keeps a
// result after its call, so the arrangement survives: text written before a
// result stays before it.
//
// A block this role has nowhere to put becomes the note omitted returns, in the
// place it held. Reasoning and an opaque payload both belong to an assistant
// turn on this wire, and a type this version has no case for was written by a
// later one and becomes the note unknown returns.
//
// An attachment the service refuses goes the same way, for the thread's sake
// rather than the wire's: one with no bytes in it, an image of a type the
// service does not take and a file with no media type. A user message is
// stored and replayed, so one sent as a part would be refused again on every
// later turn rather than on this one alone.
func (b *inputBuilder) user(m api.Message, at int) error {
	var blocks []any
	flush := func() {
		if len(blocks) == 0 {
			return
		}
		b.items = append(b.items, inputItem{kind: kindMessage, v: inputMessage{Role: "user", Content: blocks}})
		blocks = nil
	}
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			blocks = append(blocks, inputText{Type: "input_text", Text: c.Text})
		case api.ContentImage:
			if len(c.Data) == 0 {
				blocks = append(blocks, omitted(c, "an image part with no bytes in it is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if _, ok := lib.ImageMime(c.Mime); !ok {
				blocks = append(blocks, omitted(c, fmt.Sprintf("an image of type %q is refused by the service, which takes PNG, JPEG, WEBP and GIF, and the block is stored, so it would be refused again on every later turn of the thread", c.Mime)))
				continue
			}
			blocks = append(blocks, imageBlock(c))
		case api.ContentDocument:
			if len(c.Data) == 0 {
				blocks = append(blocks, omitted(c, "a file part with no bytes in it is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if strings.TrimSpace(c.Mime) == "" {
				blocks = append(blocks, omitted(c, "a file part with no media type is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			blocks = append(blocks, documentBlock(c))
		case api.ContentToolUse:
			if c.Name == "" {
				blocks = append(blocks, omitted(c, "a function_call with no name names no tool and is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			flush()
			call, err := callItem(c, b.results[c.ID])
			if err != nil {
				return err
			}
			b.items = append(b.items, call)
		case api.ContentToolResult:
			if c.ID == "" || b.nameless[c.ID] {
				blocks = append(blocks, omitted(c, "a function_call_output with no call_id, or one answering a call that went out as a note, answers no call and is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			flush()
			b.items = append(b.items, resultItem(c, at))
		case api.ContentReasoning:
			blocks = append(blocks, omitted(c, "a reasoning item is an item of its own and belongs to the assistant turn that produced it"))
		case api.ContentOpaque:
			blocks = append(blocks, omitted(c, "an item the service ran itself belongs to the assistant turn that produced it"))
		default:
			blocks = append(blocks, unknown(c, at))
		}
	}
	flush()
	return nil
}

// omitted is the note that stands in for content this adapter cannot send. It is
// built as input_text and the assistant flush turns it into output_text, the
// only text part an assistant message takes, so the note lands where the block
// was whichever message it came from.
func omitted(c api.Content, why string) inputText {
	return inputText{Type: "input_text", Text: lib.Omitted(c, why)}
}

func unknown(c api.Content, at int) inputText {
	return inputText{Type: "input_text", Text: lib.Unknown(providerName, at, c)}
}

// assistant turns one assistant message into items, then applies the reasoning
// replay policy to the run it just produced.
//
// A run of text blocks becomes one message carrying one input_text part per
// block, so the boundaries the caller wrote survive and no text acquires a
// separator the model never generated. A call, a result, a reasoning block or an
// opaque payload ends the run, because each of those is a top level item and
// cannot sit inside a message.
//
// A phase is a label on the message rather than content of its own, so it is
// read off the text block that carries it and a change of phase ends the run:
// the message being built claimed the phase of the block that opened it, and a
// block claiming another one belongs to a message of its own. A phase written
// for another model leaves nothing behind, because the label is all it was.
//
// An opaque payload is an output item of a kind protocol has no type for, and
// the input array takes those back the way it takes a reasoning item, so the
// bytes that arrived go out as they arrived. The fragments its block collected
// are not spliced back in: the finished item carried the field they streamed
// already whole, and they are only what it looked like on the way there.
//
// A stored reasoning block becomes an item when it has the service's own
// encoding to send, or an id the service kept, and the note omitted returns
// when it has neither, in the place it held: what is readable on it is a
// summary the service wrote, and replaying that as the model's own words is the
// fabrication reasoningReplay refuses. Nothing else in the run depends on the
// block, so a block with nothing to send leaves nothing behind an item would
// have to stand for.
//
// Models are not compared; OpenAI omits reasoning from another family itself.
// Payloads another provider issued are dropped: ownItem accepts a reasoning
// block's meta or an opaque payload only when it is an output item of this
// API's own shape, and anything else leaves the note in its place. The one
// capability refusal left — reasoning sent to a model that does not reason — is
// what WithReasoningReplay(ReplayNever) is for.
//
// A tool result belongs to a user turn, but a Responses item has no role and the
// model reads it the same either way, so it is carried rather than refused.
func (b *inputBuilder) assistant(m api.Message, at int) error {
	start := len(b.items)
	var parts []any
	var phase string
	flush := func() {
		if len(parts) > 0 {
			for i, p := range parts {
				if t, ok := p.(inputText); ok {
					t.Type = "output_text"
					parts[i] = t
				}
			}
			b.items = append(b.items, inputItem{kind: kindMessage, v: inputMessage{
				Role:    "assistant",
				Content: parts,
				Phase:   phase,
			}})
			parts = nil
		}
		phase = ""
	}
	for _, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			if p := b.phaseOf(c.Meta); p != phase {
				flush()
				phase = p
			}
			parts = append(parts, inputText{Type: "input_text", Text: c.Text})
		case api.ContentOpaque:
			if !ownItem(c.Data, c.Kind) {
				parts = append(parts, omitted(c, "the payload was issued by another provider"))
				continue
			}
			flush()
			b.items = append(b.items, inputItem{kind: kindOpaque, v: c.Data})
		case api.ContentReasoning:
			if len(c.Meta) > 0 && !ownItem(c.Meta, "reasoning") {
				parts = append(parts, omitted(c, "the payload was issued by another provider"))
				continue
			}
			item := reasoningReplay(c, b.set.store)
			if item == nil {
				parts = append(parts, omitted(c, "the block carries neither the service's encoding nor, with store off, an id the service kept"))
				continue
			}
			flush()
			b.items = append(b.items, inputItem{kind: kindReasoning, v: item})
		case api.ContentToolUse:
			if c.Name == "" {
				parts = append(parts, omitted(c, "a function_call with no name names no tool and is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			flush()
			call, err := callItem(c, b.results[c.ID])
			if err != nil {
				return err
			}
			b.items = append(b.items, call)
		case api.ContentToolResult:
			if c.ID == "" || b.nameless[c.ID] {
				parts = append(parts, omitted(c, "a function_call_output with no call_id, or one answering a call that went out as a note, answers no call and is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			flush()
			b.items = append(b.items, resultItem(c, at))
		case api.ContentImage:
			parts = append(parts, omitted(c, "the Responses API has no attachment part for the assistant role"))
		case api.ContentDocument:
			if len(c.Data) == 0 || !api.DocumentMime(strings.ToLower(strings.TrimSpace(c.Mime))).IsText() || !utf8.Valid(c.Data) {
				parts = append(parts, omitted(c, "the Responses API has no attachment part for the assistant role, so only a text document whose bytes are UTF-8 goes there, as text"))
				continue
			}
			parts = append(parts, inputText{Type: "input_text", Text: string(c.Data)})
		default:
			parts = append(parts, unknown(c, at))
		}
	}
	flush()
	b.applyReplayPolicy(start)
	return nil
}

// phaseOf reads the phase a stored text block was labelled with, and nothing
// when the label was written for another model: phase labels an assistant
// message one particular model produced, and an empty claim is no claim rather
// than a wildcard. The comparison is on the name exactly as configured.
//
// A block carrying meta this adapter did not write leaves both fields empty and
// so claims nothing, which is what a block with no meta at all claims.
//
// Reasoning does not come through here. Nothing about a model decides whether a
// reasoning item can be replayed; see the assistant loop.
func (b *inputBuilder) phaseOf(meta json.RawMessage) string {
	var m messageMeta
	_ = json.Unmarshal(meta, &m)
	if m.Model == "" || m.Model != b.model {
		return ""
	}
	return m.Phase
}

// reasoningReplay is the input item for a stored reasoning block: the service's
// own encoding of it, which is the whole reasoning item as it arrived, because
// the seal covers those bytes and re-encoding a value is not the same as keeping
// one. An item without encrypted_content goes back only when store is on, where
// its id names a response the service kept; with store off that id names
// nothing and the service refuses it.
//
// A block without one is not sent. What is readable on it is a summary the
// service wrote, not the thinking it summarizes, and the service never signed
// it, so an item rebuilt from it is a paraphrase presented as the model's own
// reasoning with nothing to prove it. Sending it back is at best ignored and at
// worst a rejection that costs the whole request, where dropping it costs that
// one item and nothing else in the turn.
func reasoningReplay(c api.ContentReasoning, store bool) any {
	if len(c.Meta) == 0 {
		return nil
	}
	var item outputItem
	_ = json.Unmarshal(c.Meta, &item)
	if item.EncryptedContent == "" && !store {
		return nil
	}
	return c.Meta
}

func ownItem(raw json.RawMessage, kind string) bool {
	var item outputItem
	if json.Unmarshal(raw, &item) != nil || item.Type != kind || item.ID == "" {
		return false
	}
	if kind == "reasoning" {
		return strings.HasPrefix(item.ID, "rs_")
	}
	return opaqueItemTypes[kind]
}

var opaqueItemTypes = map[string]bool{
	"file_search_call":        true,
	"function_call_output":    true,
	"web_search_call":         true,
	"computer_call":           true,
	"computer_call_output":    true,
	"program":                 true,
	"program_output":          true,
	"tool_search_call":        true,
	"tool_search_output":      true,
	"additional_tools":        true,
	"compaction":              true,
	"image_generation_call":   true,
	"code_interpreter_call":   true,
	"local_shell_call":        true,
	"local_shell_call_output": true,
	"shell_call":              true,
	"shell_call_output":       true,
	"apply_patch_call":        true,
	"apply_patch_call_output": true,
	"mcp_call":                true,
	"mcp_list_tools":          true,
	"mcp_approval_request":    true,
	"mcp_approval_response":   true,
	"custom_tool_call":        true,
	"custom_tool_call_output": true,
}

// applyReplayPolicy decides which reasoning items in the run starting at start
// go back, and decides nothing else. The unnamed branch is ReplayToolTurns,
// which is ReplayMode's zero value.
//
// The pairing the service checks against us is one directional. It names a
// reasoning item by the id on the item itself — "Item 'rs_…' of type 'reasoning'
// was provided without its required following item" — so a reasoning item is
// sent only when something follows it in the run, which is what the length test
// in each arm is. The refusal in the other direction names the function_call by
// its own id, and the function_call this adapter sends is {type, call_id, name,
// arguments} and carries no id at all, so it names nothing that can be here.
//
// So nothing but a reasoning item is ever dropped. A call and the result
// answering it are separate items in consecutive messages and this pass sees one
// message, so dropping a call would strand its function_call_output in the run
// after this one — "No tool call found for function call output with call_id …"
// — which is a refusal traded for one that was never coming.
//
// The follower is a call the caller answers or a tool the service ran itself.
// Both are items the thinking produced and the check reads the position rather
// than the kind, so both pair the same way. Preamble or commentary messages
// between a reasoning item and its call do not break the pairing: under
// ReplayToolTurns a reasoning item goes back when a follower comes later in the
// run before the next reasoning item.
func (b *inputBuilder) applyReplayPolicy(start int) {
	run := b.items[start:]
	keep := make([]bool, len(run))
	for i := range run {
		keep[i] = true
	}
	for i, it := range run {
		if it.kind != kindReasoning {
			continue
		}
		switch b.set.replay {
		case ReplayNever:
			keep[i] = false
		case ReplayAlways:
			keep[i] = i+1 < len(run)
		default:
			keep[i] = false
			for _, next := range run[i+1:] {
				if next.kind == kindReasoning {
					break
				}
				if next.kind.follows() {
					keep[i] = true
					break
				}
			}
		}
	}
	out := b.items[:start]
	for i, it := range run {
		if keep[i] {
			out = append(out, it)
		}
	}
	b.items = out
}

// callItem is the input item for a stored tool call. arguments is schema
// required and must be a string, so the empty object stands for both a call that
// took no arguments and a call whose arguments never parsed and were discarded.
//
// The result answering the call decides which: where it reports
// api.ToolErrJSON the arguments are gone and Input is not read at all, and what
// went wrong is in that result, which the model reads. Otherwise Input is sent
// as it is, and a call carrying api.DiscardedArguments with no such result is
// refused rather than sent as an empty object, which would replay as a
// legitimate call that takes no arguments.
func callItem(c api.ContentToolUse, failure api.ToolError) (inputItem, error) {
	args := "{}"
	if failure != api.ToolErrJSON {
		if bytes.Equal(c.Input, api.DiscardedArguments) {
			return inputItem{}, fmt.Errorf("gpt: tool call %q carries discarded arguments and no result answering it reports err_json", c.ID)
		}
		if len(c.Input) > 0 {
			args = string(c.Input)
		}
	}
	return inputItem{kind: kindFunctionCall, v: functionCall{
		Type:      "function_call",
		CallID:    c.ID,
		Name:      c.Name,
		Arguments: args,
	}}, nil
}

func resultItem(c api.ContentToolResult, at int) inputItem {
	return inputItem{kind: kindFunctionOutput, v: functionCallOutput{
		Type:   "function_call_output",
		CallID: c.ID,
		Output: resultOutput(c, at),
	}}
}

// resultOutput builds the output of a function_call_output. The output is
// always the block array and never the plain string the item also accepts, so
// one result block is one part and two of them stay two, the same way they do
// in a message.
//
// There is no error flag anywhere on the item — the model reads the output as
// text and the documentation says the format is the caller's to pick — so a
// ContentToolResult.Error of any kind is folded into the output rather than
// dropped, as a part of its own in front of the result. It is a part rather than
// a prefix on the first one because the result blocks are what the tool returned
// and stay what the tool returned. Which kind of failure it was is not spelled
// out: the classification is for whoever routes the result, and what the model
// needs to read about it is already in the blocks.
//
// A result with no blocks gets the text lib.EmptyResult names, because the
// documentation asks for text saying a call that returns nothing succeeded.
//
// A block with no part to put it in becomes the note omitted returns, in the
// place it held, the same as a block of a message does, and so does an
// attachment the service refuses, for the reason inputBuilder.user gives. The subset a result may carry is what every wire takes something of and
// not what each one takes all of, so a later version of this protocol widening it
// lands here, and a request refused over one block answers nothing the model is
// waiting on.
func resultOutput(c api.ContentToolResult, at int) any {
	blocks := make([]any, 0, len(c.Content)+1)
	if c.Error != api.ToolOk {
		blocks = append(blocks, inputText{Type: "input_text", Text: "Error:"})
	}
	for _, rc := range c.Content {
		switch rc := rc.(type) {
		case api.ContentText:
			blocks = append(blocks, inputText{Type: "input_text", Text: rc.Text})
		case api.ContentImage:
			if len(rc.Data) == 0 {
				blocks = append(blocks, omitted(rc, "an image part with no bytes in it is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if _, ok := lib.ImageMime(rc.Mime); !ok {
				blocks = append(blocks, omitted(rc, fmt.Sprintf("an image of type %q is refused by the service, which takes PNG, JPEG, WEBP and GIF, and the result is stored, so it would be refused again on every later turn of the thread", rc.Mime)))
				continue
			}
			blocks = append(blocks, imageBlock(rc))
		case api.ContentDocument:
			if len(rc.Data) == 0 {
				blocks = append(blocks, omitted(rc, "a file part with no bytes in it is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if strings.TrimSpace(rc.Mime) == "" {
				blocks = append(blocks, omitted(rc, "a file part with no media type is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			blocks = append(blocks, documentBlock(rc))
		default:
			blocks = append(blocks, unknown(rc, at))
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, inputText{Type: "input_text", Text: lib.EmptyResult})
	}
	return blocks
}

func imageBlock(c api.ContentImage) inputImage {
	mime, _ := lib.ImageMime(c.Mime)
	return inputImage{Type: "input_image", ImageURL: dataURL(mime, c.Data), Detail: "high"}
}

// documentExtensions maps the media types worth guessing a filename for. The
// accepted list runs to dozens of text and code types and the service reads the
// extension, so a document that arrived without a name gets one that matches
// what it is.
var documentExtensions = map[string]string{
	"application/pdf":          ".pdf",
	"application/json":         ".json",
	"text/plain":               ".txt",
	"text/markdown":            ".md",
	"text/csv":                 ".csv",
	"text/html":                ".html",
	"text/xml":                 ".xml",
	"application/xml":          ".xml",
	"application/yaml":         ".yaml",
	"text/x-python":            ".py",
	"text/x-go":                ".go",
	"application/rtf":          ".rtf",
	"application/msword":       ".doc",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
}

func documentBlock(c api.ContentDocument) inputFile {
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	name := c.Name
	if name == "" {
		name = "document" + documentExtensions[mime]
	}
	return inputFile{Type: "input_file", Filename: name, FileData: dataURL(mime, c.Data)}
}

// dataURL is the form every current example uses for both image_url and
// file_data, and the only one that carries the media type alongside the bytes.
func dataURL(mime string, data []byte) string {
	buf := make([]byte, len("data:")+len(mime)+len(";base64,")+base64.StdEncoding.EncodedLen(len(data)))
	n := copy(buf, "data:")
	n += copy(buf[n:], mime)
	n += copy(buf[n:], ";base64,")
	base64.StdEncoding.Encode(buf[n:], data)
	return unsafe.String(&buf[0], len(buf))
}
