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

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
)

// Indexed holds values the stream fills by index, so entries arrive in any
// order and the set can be sparse. Ordered reads them back in index order.
type Indexed[T any] map[int]T

// Iter yields the values in index order.
func (i Indexed[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, key := range slices.Sorted(maps.Keys(i)) {
			if !yield(i[key]) {
				return
			}
		}
	}
}

// Ordered returns the values in index order; nil when empty.
func (i Indexed[T]) Ordered() []T {
	if len(i) == 0 {
		return nil
	}
	return slices.AppendSeq(make([]T, 0, len(i)), i.Iter())
}

// Content is one block of a message: ContentText, ContentImage,
// ContentDocument, ContentToolUse, ContentToolResult, ContentReasoning, or
// ContentOpaque for what only one provider understands. Not every adapter can
// carry every block; see UserContent.
//
// Meta on ContentText, ContentToolUse and ContentReasoning is the issuing
// adapter's own data, handed back to it unchanged on replay; folding a stream,
// a non-empty Meta replaces the one before it. An adapter writes it as
// {"provider":"<its Name>","data":<its payload>} and reads only Meta naming
// itself. On user-authored blocks
// (ContentText, ContentImage, ContentDocument) Meta is the caller's data, which
// no adapter reads or sends.
//
// An adapter whose wire takes one string for several blocks joins them with a
// blank line, so lists, headings and code fences keep their framing.
type Content interface {
	contentType() string
	json.Marshaler
}

// ContentText is a plain text block.
type ContentText struct {
	// Text is the block's text.
	Text string `json:"text"`
	// Meta is adapter data on model output, caller data on user input.
	Meta json.RawMessage `json:"meta,omitempty"`
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentText) MarshalJSON() ([]byte, error) {
	type alias ContentText
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentText) contentType() string {
	return "text"
}

// ContentImage is an image attachment.
type ContentImage struct {
	Data []byte          `json:"data"`           // raw bytes; base64 in JSON
	Mime string          `json:"mimeType"`       // an ImageMime value
	Meta json.RawMessage `json:"meta,omitempty"` // caller data; adapters ignore it
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentImage) MarshalJSON() ([]byte, error) {
	type alias ContentImage
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentImage) contentType() string {
	return "image"
}

// ContentDocument is a file attachment, such as a PDF.
type ContentDocument struct {
	Data []byte          `json:"data"`           // raw bytes; base64 in JSON
	Mime string          `json:"mimeType"`       // a DocumentMime value
	Name string          `json:"name"`           // file name or title, sent where the wire has one; may be empty
	Meta json.RawMessage `json:"meta,omitempty"` // caller data; adapters ignore it
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentDocument) MarshalJSON() ([]byte, error) {
	type alias ContentDocument
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentDocument) contentType() string {
	return "document"
}

// ContentToolUse is a tool call the model asked for. Every call replayed to a
// provider must be answered by a ContentToolResult with the same ID; LostResult
// stands in for one that is missing.
//
// Input is the arguments as valid JSON; nil means the provider sent none, and
// an adapter writes {} where its wire requires an object. Arguments that never
// formed valid JSON are not stored: Input holds DiscardedArguments instead, and
// the result answering the call must carry ToolErrJSON. An adapter encodes such
// a call without reading Input, and refuses with CodeInvalid a discarded call
// whose result lacks ToolErrJSON.
type ContentToolUse struct {
	ID    string          `json:"id"`              // provider's call id, matched by ContentToolResult.ID
	Name  string          `json:"name"`            // name of the Tool called
	Input json.RawMessage `json:"input,omitempty"` // omitempty keeps nil through a round trip
	Meta  json.RawMessage `json:"meta,omitempty"`  // issuing adapter's data
}

// LostResult returns a ToolErrRuntime result answering c, telling the model the
// result was lost or the call never ran. Use it to keep a call answered when
// its real result is missing.
func (c ContentToolUse) LostResult() ContentToolResult {
	return ContentToolResult{
		ID: c.ID,
		Content: UserContents{ContentText{
			Text: "<noresult>result was lost or call never ran</noresult>",
		}},
		Error: ToolErrRuntime,
	}
}

// MarshalJSON encodes c with its "type" tag, writing DiscardedArguments as no
// input.
func (c ContentToolUse) MarshalJSON() ([]byte, error) {
	if bytes.Equal(c.Input, DiscardedArguments) {
		c.Input = nil
	}
	type alias ContentToolUse
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentToolUse) contentType() string {
	return "tool_use"
}

// DiscardedArguments stands in ContentToolUse.Input for arguments that arrived
// and never formed valid JSON. It is deliberately not JSON at all: arguments a
// provider sent are always valid JSON, so nothing can collide with it and
// bytes.Equal is the whole test.
//
// It is a marker held in memory and never reaches a wire of any kind.
// ContentToolUse.MarshalJSON writes nil in its place, so storage and every
// reload read a call that was sent no arguments, and what survives the round
// trip is the ToolErrJSON on the result answering it.
var DiscardedArguments = json.RawMessage("!discarded-arguments")

// DiscardedArgumentsError reports a tool call whose arguments never formed
// valid JSON, so the call was not made and the arguments were not stored.
// Answer the call with it through NewContentToolResult, which classifies it as
// ToolErrJSON, the classification ContentToolUse requires of the result
// answering a discarded call.
type DiscardedArgumentsError struct {
	// Raw is the invalid argument text, if the caller still holds it; empty
	// asks the model to reissue the call with nothing quoted back.
	Raw string
}

const discardedArgumentsLimit = 256

// Error tells the model to reissue the call, quoting at most the first 256
// runes of Raw, since the result is resent with every later request.
func (e DiscardedArgumentsError) Error() string {
	const msg = "tool call arguments were not valid JSON, the call was not made, reissue it with complete arguments"
	if e.Raw == "" {
		return msg
	}
	if r := []rune(e.Raw); len(r) > discardedArgumentsLimit {
		return msg + ": " + string(r[:discardedArgumentsLimit]) + "…"
	}
	return msg + ": " + e.Raw
}

// UserContent is the subset of Content a user may author: text and
// attachments. A tool result carries the same subset, so it is also the
// content of ContentToolResult, and nesting a result in a result stays
// inexpressible.
//
// Not every provider accepts the whole subset everywhere: some take only text
// in a tool result. An adapter leaves a note in place of a block it cannot
// carry, including one whose media type its API does not accept, and logs
// nothing, so a result always answers the call it is matched to and the request
// still goes out. The one refusal left is a discarded call whose result lacks
// ToolErrJSON; see ContentToolUse.
type UserContent interface {
	Content
	userContent()
}

func (ContentText) userContent()     {}
func (ContentImage) userContent()    {}
func (ContentDocument) userContent() {}

// ContentToolResult reports the outcome of the ContentToolUse with the same
// ID. NewContentToolResult builds one from a tool's output and error.
//
// Its position among the blocks of a message is kept only as far as the wire
// allows. What the model reads is Content, plus whatever failure marker the
// adapter sends for an Error other than ToolOk.
type ContentToolResult struct {
	ID      string       `json:"toolUseId"` // ContentToolUse.ID this answers
	Content UserContents `json:"content"`   // blocks the call produced, in order
	Error   ToolError    `json:"error"`     // ToolOk on success
}

// ToolError says how a call turned out. Any value but ToolOk is sent as the
// adapter's failure marker (an is_error flag or an "Error:" prefix); the
// failure itself is described in ContentToolResult.Content.
//
// The three values are the whole set. A stored value that is none of them
// decodes as ToolErrRuntime, so a vocabulary that grew reads as a call that
// failed rather than as one that succeeded.
type ToolError string

const (
	ToolOk         ToolError = ""            // the call ran and its output is in Content
	ToolErrRuntime ToolError = "err_runtime" // the call failed, or never ran for a reason of its own
	ToolErrJSON    ToolError = "err_json"    // the arguments never formed valid JSON, so the call was never made; see DiscardedArguments
)

// UnmarshalJSON decodes a stored value, reading any unknown one as
// ToolErrRuntime.
func (e *ToolError) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch ToolError(s) {
	case ToolOk, ToolErrRuntime, ToolErrJSON:
		*e = ToolError(s)
	default:
		*e = ToolErrRuntime
	}
	return nil
}

// Refused returns c with its content replaced by a note that the output was
// omitted because the provider refused it, keeping ID and Error so the call
// stays answered.
func (c ContentToolResult) Refused() ContentToolResult {
	return ContentToolResult{
		ID: c.ID,
		Content: UserContents{ContentText{
			Text: "<noresult>output was omitted, because LLM provider refused to answer</noresult>",
		}},
		Error: c.Error,
	}
}

// NewToolContents encodes a tool's output as the content of its result:
// UserContents and UserContent as they are, a Container as its Content, a
// MultiContainer as its Contents, a string or fmt.Stringer as one text block,
// and anything else as its JSON in one text block.
func NewToolContents(output any) (UserContents, error) {
	switch output := output.(type) {
	case UserContents:
		return output, nil
	case UserContent:
		return UserContents{output}, nil
	case Container:
		return UserContents{output.Content()}, nil
	case MultiContainer:
		return output.Contents(), nil
	case string:
		return UserContents{ContentText{
			Text: output,
		}}, nil
	case fmt.Stringer:
		return UserContents{ContentText{
			Text: output.String(),
		}}, nil
	}
	json, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return UserContents{ContentText{
		Text: string(json),
	}}, nil
}

// NewContentToolResult builds the result answering call id. With a nil err it
// carries content as ToolOk. Otherwise content is dropped and the result is
// err.Error() as one text block, followed by the Container, else the
// MultiContainer, that err is or wraps, as NewToolContents encodes it; content
// that fails to encode is left out. It is classified ToolErrJSON when err
// wraps a DiscardedArgumentsError and ToolErrRuntime for any other error.
func NewContentToolResult(id string, content UserContents, err error) ContentToolResult {
	if err != nil {
		failure := ToolErrRuntime
		var discarded DiscardedArgumentsError
		if errors.As(err, &discarded) {
			failure = ToolErrJSON
		}
		var more UserContents
		var container Container
		var multi MultiContainer
		if errors.As(err, &container) {
			more, _ = NewToolContents(container)
		} else if errors.As(err, &multi) {
			more, _ = NewToolContents(multi)
		}
		content = append(UserContents{ContentText{Text: err.Error()}}, more...)
		return ContentToolResult{ID: id, Content: content, Error: failure}
	}
	return ContentToolResult{
		ID:      id,
		Content: content,
	}
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentToolResult) MarshalJSON() ([]byte, error) {
	type alias ContentToolResult
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentToolResult) contentType() string {
	return "tool_result"
}

// ContentReasoning is the model's thinking, in whichever forms the provider
// hands over. Summary and Thoughts are keyed by the Part of EventSummary and
// EventThinking; read them in order with Ordered. A provider that does not
// index parts fills key 0. Any of Summary, Thoughts and Meta may be empty.
//
// Where a provider signs or seals reasoning, Meta is what is replayed, byte for
// byte, and the text never is. An adapter replays a block only when its Meta
// names that adapter's provider and, for an adapter that requires it, Model
// names the model of the request; then it may use any of the block. Any other
// block is skipped whole, with nothing sent in its place. Nothing upstream
// strips them.
type ContentReasoning struct {
	// Model is the model the provider was configured with, as EventStart.Model
	// reports. The generic adapter replays the block only to a request for
	// exactly this model name, so "x:nitro" and "x" differ; the others judge
	// by Meta alone.
	Model    string          `json:"model"`
	Summary  Indexed[string] `json:"summary"`        // paraphrase written by the service
	Thoughts Indexed[string] `json:"thoughts"`       // thinking as the model generated it
	Meta     json.RawMessage `json:"meta,omitempty"` // signature or sealed payload, replayed as is
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentReasoning) MarshalJSON() ([]byte, error) {
	type alias ContentReasoning
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentReasoning) contentType() string {
	return "reasoning"
}

// ContentOpaque carries a payload this protocol has no type for, such as the
// output of a tool the provider ran itself or fields of a reply the adapter
// does not read. It is meant for the adapter that issued it: Kind starts with
// that adapter's Name and a dot, as in "claude.compaction", and an adapter
// replays only the kinds it issues, to the model it replays its reasoning to
// (see ContentReasoning.Model), and skips every other one; nothing upstream
// strips them. Folding a stream, a non-empty Kind or Data replaces the one
// before it and each Delta appends to Deltas in arrival order.
type ContentOpaque struct {
	Model  string          `json:"model"`  // configured model, as on ContentReasoning
	Kind   string          `json:"kind"`   // issuing adapter's Name, a dot, and its name for the payload
	Data   json.RawMessage `json:"data"`   // bytes as the API sent them, which it may have signed
	Deltas []string        `json:"deltas"` // streamed fragments of a field inside Data, put back by the adapter
}

// MarshalJSON encodes c with its "type" tag.
func (c ContentOpaque) MarshalJSON() ([]byte, error) {
	type alias ContentOpaque
	return json.Marshal(struct {
		Type string `json:"type"`
		alias
	}{c.contentType(), alias(c)})
}

func (ContentOpaque) contentType() string {
	return "opaque"
}
