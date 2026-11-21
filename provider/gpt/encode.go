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

type messageMeta struct {
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
	in := &inputBuilder{set: set, msgs: req.Messages, carried: map[string]bool{}}
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
	set     settings
	msgs    []api.Message
	carried map[string]bool
	items   []inputItem
}

func (b *inputBuilder) finish() []any {
	out := make([]any, len(b.items))
	for i, it := range b.items {
		out[i] = it.v
	}
	return out
}

func (b *inputBuilder) answer(at, block int, id string) (api.ToolError, bool) {
	for i := at; i < len(b.msgs); i++ {
		for j, c := range b.msgs[i].Content {
			if r, is := c.(api.ContentToolResult); is && (i > at || j > block) && r.ID == id {
				return r.Error, true
			}
		}
	}
	return api.ToolOk, false
}

func (b *inputBuilder) user(m api.Message, at int) error {
	var blocks []any
	flush := func() {
		if len(blocks) == 0 {
			return
		}
		b.items = append(b.items, inputItem{kind: kindMessage, v: inputMessage{Role: "user", Content: blocks}})
		blocks = nil
	}
	for j, c := range m.Content {
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
			call, err := b.call(c, at, j)
			if err != nil {
				return err
			}
			if t, noted := call.v.(inputText); noted {
				blocks = append(blocks, t)
				continue
			}
			flush()
			b.items = append(b.items, call)
		case api.ContentToolResult:
			if !b.carried[c.ID] {
				blocks = append(blocks, omitted(c, "a function_call_output has to follow a function_call with its call_id, and the service refuses one that does not on every later turn of the thread"))
				continue
			}
			flush()
			b.items = append(b.items, resultItem(c, at))
		case api.ContentReasoning, api.ContentOpaque:
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
	for j, c := range m.Content {
		switch c := c.(type) {
		case api.ContentText:
			var meta messageMeta
			if data, own := lib.OwnMeta(c.Meta, providerName); own {
				_ = json.Unmarshal(data, &meta)
			}
			if meta.Phase != "commentary" && meta.Phase != "final_answer" {
				meta.Phase = ""
			}
			if meta.Phase != phase {
				flush()
				phase = meta.Phase
			}
			parts = append(parts, inputText{Type: "input_text", Text: c.Text})
		case api.ContentOpaque:
			kind, own := strings.CutPrefix(c.Kind, providerName+".")
			var item outputItem
			if !own || !opaqueItemTypes[kind] || json.Unmarshal(c.Data, &item) != nil || item.Type != kind {
				continue
			}
			flush()
			b.items = append(b.items, inputItem{kind: kindOpaque, v: c.Data})
		case api.ContentReasoning:
			data, own := lib.OwnMeta(c.Meta, providerName)
			var item outputItem
			if !own || b.set.replay == ReplayNever || json.Unmarshal(data, &item) != nil || item.Type != "reasoning" || item.EncryptedContent == "" && !b.set.store {
				continue
			}
			flush()
			b.items = append(b.items, inputItem{kind: kindReasoning, v: data})
		case api.ContentToolUse:
			call, err := b.call(c, at, j)
			if err != nil {
				return err
			}
			if t, noted := call.v.(inputText); noted {
				parts = append(parts, t)
				continue
			}
			flush()
			b.items = append(b.items, call)
		case api.ContentToolResult:
			if !b.carried[c.ID] {
				parts = append(parts, omitted(c, "a function_call_output has to follow a function_call with its call_id, and the service refuses one that does not on every later turn of the thread"))
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
	if !b.set.store {
		return nil
	}
	run := b.items[start:]
	out := b.items[:start]
	for i, it := range run {
		keep := it.kind != kindReasoning
		for _, next := range run[i+1:] {
			if keep || next.kind == kindReasoning {
				break
			}
			keep = next.kind == kindFunctionCall || next.kind == kindOpaque
		}
		if keep {
			out = append(out, it)
		}
	}
	b.items = out
	return nil
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

// call is the input item for the stored tool call at block j of message at, or
// the note in its place when no later result answers it. arguments is schema
// required and must be a string, so the empty object stands for both a call
// that took no arguments and a call whose arguments never parsed and were
// discarded.
//
// The result answering the call decides which: where it reports
// api.ToolErrJSON the arguments are gone and Input is not read at all, and what
// went wrong is in that result, which the model reads. Otherwise Input is sent
// as it is, and a call carrying api.DiscardedArguments with no such result is
// refused rather than sent as an empty object, which would replay as a
// legitimate call that takes no arguments.
func (b *inputBuilder) call(c api.ContentToolUse, at, j int) (inputItem, error) {
	failure, answered := b.answer(at, j, c.ID)
	if failure != api.ToolErrJSON && bytes.Equal(c.Input, api.DiscardedArguments) {
		return inputItem{}, fmt.Errorf("gpt: tool call %q carries discarded arguments and no result answering it reports err_json", c.ID)
	}
	if !answered {
		return inputItem{v: omitted(c, "a function_call has to be answered by a function_call_output later in the input, and the service refuses one that is not on every later turn of the thread")}, nil
	}
	args := "{}"
	if failure != api.ToolErrJSON && len(c.Input) > 0 {
		args = string(c.Input)
	}
	b.carried[c.ID] = true
	return inputItem{kind: kindFunctionCall, v: functionCall{
		Type:      "function_call",
		CallID:    lib.CallID(c.ID),
		Name:      lib.CallName(c.Name),
		Arguments: args,
	}}, nil
}

func resultItem(c api.ContentToolResult, at int) inputItem {
	return inputItem{kind: kindFunctionOutput, v: functionCallOutput{
		Type:   "function_call_output",
		CallID: lib.CallID(c.ID),
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
