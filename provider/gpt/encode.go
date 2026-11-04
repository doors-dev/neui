package gpt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// encodeRequest builds the body of one turn.
//
// A reasoning model is asked for reasoning.encrypted_content unless the caller
// replaced the include list. Stateless mode populates it on its own now, and the
// include value is still accepted, still documented and still what older models
// need. Sending it costs a string.
//
// Sampling is dropped on reasoning models rather than forwarded, unless the
// effort is none. At any other effort they answer 400 to the parameter whatever
// its value, including the default, so there is nothing to send that would
// work; at none the docs keep it.
func encodeRequest(model string, set settings, req api.Request) (*requestBody, error) {
	if model == "" {
		return nil, errors.New("gpt: provider has no model, set Conf.Model")
	}
	reasoner := isReasoningModel(model)
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
	case reasoner:
		b.Include = []string{"reasoning.encrypted_content"}
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
	if reasoner {
		if r := (reasoningConfig{
			Effort:  set.effort,
			Summary: set.summary,
			Mode:    set.reasoningMode,
			Context: set.reasoningContext,
		}); r != (reasoningConfig{}) {
			b.Reasoning = &r
		}
	}
	if set.verbosity != "" {
		b.Text = &textConfig{Verbosity: set.verbosity}
	}
	if !reasoner || set.forceSampling || set.effort == string(EffortNone) {
		b.Temperature = set.temperature
		b.TopP = set.topP
	}
	b.PromptCacheKey = set.promptCacheKey
	if set.cacheMode != "" || set.cacheTTL != "" {
		b.PromptCacheOpts = &promptCacheConfig{Mode: set.cacheMode, TTL: set.cacheTTL}
	}
	in := &inputBuilder{set: set, model: model, results: map[string]api.ToolError{}}
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if r, ok := c.(api.ContentToolResult); ok {
				in.results[r.ID] = r.Error
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
			err = fmt.Errorf("gpt: unknown role %q", m.Role)
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
	model   string
	results map[string]api.ToolError
	items   []inputItem
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
// later one.
//
// An attachment with no bytes in it goes the same way, for the thread's sake
// rather than the wire's: the service refuses a part carrying an empty data URL,
// and a user message is stored and replayed, so one sent as a part would be
// refused again on every later turn rather than on this one alone.
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
				blocks = append(blocks, omitted(c, at, "an image part with no bytes in it is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			img, err := imageBlock(c)
			if err != nil {
				return err
			}
			blocks = append(blocks, img)
		case api.ContentDocument:
			if len(c.Data) == 0 {
				blocks = append(blocks, omitted(c, at, "a file part with no bytes in it is refused by the service, and the block is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			f, err := documentBlock(c)
			if err != nil {
				return err
			}
			blocks = append(blocks, f)
		case api.ContentToolUse:
			flush()
			call, err := callItem(c, b.results[c.ID])
			if err != nil {
				return err
			}
			b.items = append(b.items, call)
		case api.ContentToolResult:
			flush()
			r, err := resultItem(c, at)
			if err != nil {
				return err
			}
			b.items = append(b.items, r)
		case api.ContentReasoning:
			blocks = append(blocks, omitted(c, at, "a reasoning item is an item of its own and belongs to the assistant turn that produced it"))
		case api.ContentOpaque:
			blocks = append(blocks, omitted(c, at, "an item the service ran itself belongs to the assistant turn that produced it"))
		default:
			blocks = append(blocks, omitted(c, at, "this version of the adapter has no input part for it"))
		}
	}
	flush()
	return nil
}

// omitted is the note that stands in for content this adapter cannot send.
// input_text is the one part both roles take, so the note lands where the block
// was whichever message it came from.
func omitted(c api.Content, at int, why string) inputText {
	return inputText{Type: "input_text", Text: lib.Omitted("gpt", at, c, why)}
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
// encoding to send, and the note omitted returns when it has not, in the place
// it held: what is readable on it is a summary the service wrote, and replaying
// that as the model's own words is the fabrication reasoningReplay refuses.
// Nothing else in the run depends on the block, so a block with nothing to send
// leaves nothing behind an item would have to stand for.
//
// Nothing compares models. A reasoning item has an id, a type, a summary,
// content, an encrypted payload and a status, and no field naming a model at
// all; the payload is sealed against the organization and against the item's own
// id, so the service either reads these bytes or refuses them for a reason no
// comparison here could predict. OpenAI's own SDKs replay the output array
// verbatim, and the one capability refusal there is — reasoning sent to a model
// that does not reason — is what WithReasoningReplay(ReplayNever) is for.
//
// A tool result belongs to a user turn, but a Responses item has no role and the
// model reads it the same either way, so it is carried rather than refused.
func (b *inputBuilder) assistant(m api.Message, at int) error {
	start := len(b.items)
	var parts []any
	var phase string
	flush := func() {
		if len(parts) > 0 {
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
			flush()
			b.items = append(b.items, inputItem{kind: kindOpaque, v: c.Data})
		case api.ContentReasoning:
			item := reasoningReplay(c)
			if item == nil {
				parts = append(parts, omitted(c, at, "the block carries no encoding of its own, and the service never signed what is readable on it"))
				continue
			}
			flush()
			b.items = append(b.items, inputItem{kind: kindReasoning, v: item})
		case api.ContentToolUse:
			flush()
			call, err := callItem(c, b.results[c.ID])
			if err != nil {
				return err
			}
			b.items = append(b.items, call)
		case api.ContentToolResult:
			flush()
			r, err := resultItem(c, at)
			if err != nil {
				return err
			}
			b.items = append(b.items, r)
		case api.ContentImage, api.ContentDocument:
			parts = append(parts, omitted(c, at, "the Responses API has no attachment part for the assistant role"))
		default:
			parts = append(parts, omitted(c, at, "this version of the adapter has no input part for it"))
		}
	}
	flush()
	b.applyReplayPolicy(start)
	return nil
}

// phaseOf reads the phase a stored text block was labelled with, and nothing
// when the label was written for another model: phase labels an assistant
// message one particular model produced, and an empty claim is no claim rather
// than a wildcard. The comparison is on the normalized name, so a gateway prefix
// or a case difference does not silently drop one.
//
// A block carrying meta this adapter did not write leaves both fields empty and
// so claims nothing, which is what a block with no meta at all claims.
//
// Reasoning does not come through here. Nothing about a model decides whether a
// reasoning item can be replayed; see the assistant loop.
func (b *inputBuilder) phaseOf(meta json.RawMessage) string {
	var m messageMeta
	_ = json.Unmarshal(meta, &m)
	if m.Model == "" || baseModel(m.Model) != baseModel(b.model) {
		return ""
	}
	return m.Phase
}

// reasoningReplay is the input item for a stored reasoning block: the service's
// own encoding of it, which is the whole reasoning item as it arrived, because
// the seal covers those bytes and re-encoding a value is not the same as keeping
// one.
//
// A block without one is not sent. What is readable on it is a summary the
// service wrote, not the thinking it summarizes, and the service never signed
// it, so an item rebuilt from it is a paraphrase presented as the model's own
// reasoning with nothing to prove it. Sending it back is at best ignored and at
// worst a rejection that costs the whole request, where dropping it costs that
// one item and nothing else in the turn.
func reasoningReplay(c api.ContentReasoning) any {
	if len(c.Meta) == 0 {
		return nil
	}
	return c.Meta
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
// than the kind, so both pair the same way.
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
			keep[i] = i+1 < len(run) && run[i+1].kind.follows()
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

func resultItem(c api.ContentToolResult, at int) (inputItem, error) {
	out, err := resultOutput(c, at)
	if err != nil {
		return inputItem{}, err
	}
	return inputItem{kind: kindFunctionOutput, v: functionCallOutput{
		Type:   "function_call_output",
		CallID: c.ID,
		Output: out,
	}}, nil
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
// A block with no part to put it in becomes the note omitted returns, in the
// place it held, the same as a block of a message does, and so does an
// attachment that arrived with no bytes in it, for the reason inputBuilder.user
// gives. The subset a result may carry is what every wire takes something of and
// not what each one takes all of, so a later version of this protocol widening it
// lands here, and a request refused over one block answers nothing the model is
// waiting on.
func resultOutput(c api.ContentToolResult, at int) (any, error) {
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
				blocks = append(blocks, omitted(rc, at, "an image part with no bytes in it is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if !imageMediaTypes[strings.ToLower(strings.TrimSpace(rc.Mime))] {
				blocks = append(blocks, omitted(rc, at, fmt.Sprintf("an image of type %q is refused by the service, which takes PNG, JPEG, WEBP and GIF, and the result is stored, so it would be refused again on every later turn of the thread", rc.Mime)))
				continue
			}
			img, err := imageBlock(rc)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, img)
		case api.ContentDocument:
			if len(rc.Data) == 0 {
				blocks = append(blocks, omitted(rc, at, "a file part with no bytes in it is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			if strings.TrimSpace(rc.Mime) == "" {
				blocks = append(blocks, omitted(rc, at, "a file part with no media type is refused by the service, and the result is stored, so it would be refused again on every later turn of the thread"))
				continue
			}
			f, err := documentBlock(rc)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, f)
		default:
			blocks = append(blocks, omitted(rc, at, "this version of the adapter has no output part for it"))
		}
	}
	return blocks, nil
}

// imageMediaTypes is what the vision models take. Anything else is refused
// here, with the media type named, rather than forwarded to be refused there
// with a message about a data URL.
var imageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/webp": true,
	"image/gif":  true,
}

func imageBlock(c api.ContentImage) (inputImage, error) {
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	if !imageMediaTypes[mime] {
		return inputImage{}, fmt.Errorf("gpt: image media type %q is not accepted: PNG, JPEG, WEBP and non animated GIF only", c.Mime)
	}
	return inputImage{Type: "input_image", ImageURL: dataURL(mime, c.Data), Detail: "auto"}, nil
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

func documentBlock(c api.ContentDocument) (inputFile, error) {
	mime := strings.ToLower(strings.TrimSpace(c.Mime))
	if mime == "" {
		return inputFile{}, errors.New("gpt: document has no media type")
	}
	name := c.Name
	if name == "" {
		name = "document" + documentExtensions[mime]
	}
	return inputFile{Type: "input_file", Filename: name, FileData: dataURL(mime, c.Data)}, nil
}

// dataURL is the form every current example uses for both image_url and
// file_data, and the only one that carries the media type alongside the bytes.
func dataURL(mime string, data []byte) string {
	var sb strings.Builder
	sb.Grow(len(mime) + base64.StdEncoding.EncodedLen(len(data)) + 16)
	sb.WriteString("data:")
	sb.WriteString(mime)
	sb.WriteString(";base64,")
	sb.WriteString(base64.StdEncoding.EncodeToString(data))
	return sb.String()
}
