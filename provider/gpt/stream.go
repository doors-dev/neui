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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/maphash"
	"io"
	"net/http"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

var errTooLarge = errors.New("gpt: request too large")

// block is what the adapter keeps for one item of the response's output array:
// the protocol Index it was given, and the little that decides what a later
// frame at the same output_index means. None of it grows with the content
// passing through, because nothing here is held back to be emitted later.
//
// idx is the protocol Index, or -1 until the block opens. A message has no index
// of its own; its content parts are in parts, which maps a message's
// content_index to a protocol Index for the parts that are open, and an entry is
// dropped when its part closes. phase is the meta of a message that announced
// one, which every content part of that message opens with.
//
// kind is the item type output_item.added announced, which every later frame at
// the same output_index has to agree with.
//
// sawArgs says a fragment of a function call's arguments was emitted. It is what
// tells a call that takes no arguments from one whose arguments never arrived,
// which protocol keeps apart. args hashes the fragments, so the arguments the
// done event repeats can be checked against what streamed without keeping them.
type block struct {
	idx     int
	kind    string
	parts   map[int]int
	phase   json.RawMessage
	sawArgs bool
	args    maphash.Hash
}

// converter turns one frame of the Responses wire format into protocol events.
//
// events is what the frame being converted has produced so far. It is reset at
// the top of every Convert, so nothing survives the frame that made it.
type converter struct {
	model      string
	seed       maphash.Seed
	out        map[int]*block
	next       int
	sent       int
	declared   int
	started    bool
	sawRefusal bool
	usage      api.EventUsage
	hasUsage   bool
	events     []api.Event
	head       frameHead
	ev         streamEvent
}

func newConverter(model string, sent int, declared int) *converter {
	return &converter{model: model, seed: maphash.MakeSeed(), out: map[int]*block{}, sent: sent, declared: declared}
}

func (c *converter) emit(e api.Event) {
	c.events = append(c.events, e)
}

// take allocates the next protocol Index. Every block the protocol keeps gets
// one and nothing else does, which is why no provider index is usable as is
// here: one message item becomes as many blocks as it has content parts while a
// reasoning item that shows nothing and seals nothing becomes none, so
// output_index is neither one per block nor dense, and content_index counts
// within an item, so two items each have a part zero.
func (c *converter) take() int {
	i := c.next
	c.next++
	return i
}

// Convert applies one frame. A returned error is a frame that did not parse,
// which is a stream that cannot be trusted to continue.
//
// The type is read first and the rest of the frame only for a type handled
// here, so a new event type produces nothing whatever shape its fields take. A
// handled frame that breaks the flow fails the turn: a stream that does not open
// with a response, an index the event needs that is absent, an item that does
// not parse or names no type, a function call with no call_id, a text,
// reasoning or hosted tool fragment for an item that was announced as another
// kind or not at all, an item done that was never announced, arguments done
// that differ from the fragments that streamed, and a completed frame with no
// response.
//
// The terminal frames read the usage before they emit their own event, because
// the final usage rides the same frame and the terminal event is the last thing
// that goes out. A turn that failed still spent the tokens it generated before
// it did.
//
// The service declares a content part finished twice, once for the text and once
// for the part, and the block closes on whichever arrives first.
//
// An empty fragment appends nothing, so it opens nothing. The service sends a
// leading empty delta for the arguments of a call and for a text part, and an
// index handed out for one would put an empty block in the reconstructed
// message.
//
// response.function_call_arguments.done is the service saying the arguments are
// complete, and it sends an empty string for a call that takes none. That is the
// one case an empty object is right: a block that closes having carried nothing
// otherwise means the arguments were lost, and protocol keeps the two apart. It
// is the emitted fragments that decide which of the two happened, which is why
// the leading empty delta must not count as one.
//
// Everything else is read for its bytes and nothing else. response.created,
// response.in_progress and response.queued exist to prove a silence is a queue
// wait rather than a dead socket, and they changed nothing about the block list.
// response.content_part.added and response.reasoning_summary_part.added are
// structure the deltas already carry indices for, and the *.done text, refusal,
// summary and argument events repeat an accumulation that has already been
// emitted piece by piece. output_text.annotation.added is citations, which
// protocol has no Content for.
//
// The hosted tool families — file_search, web_search, code_interpreter,
// image_generation, mcp_call, mcp_list_tools — and the custom tool call are
// never requested, because api.Tool describes a client side function and
// nothing else, and a gateway requests them anyway. Each is an output item and
// becomes an opaque block. Three of them stream one field of the item as it is
// produced, the interpreter's code, an MCP call's arguments and a custom call's
// input, and those fragments fill the block's parts. The in_progress, searching,
// interpreting, generating, completed and failed events say where an item has
// got to and edit nothing, and they are also why a stream goes quiet for
// minutes. image_generation_call.partial_image is a whole preview image rather
// than a fragment of one, so there is nothing for it to append to. The audio
// families carry no output item and protocol has no content for sound.
//
// Usage comes after whatever the frame edited. Any frame carrying a response
// object may carry it, so it is read wherever it shows up rather than only off
// response.completed.
//
// Response.status has a cancelled value with no event type of its own, so it
// rides an ordinary frame, terminal frames included. It is tested before the
// frame type is, because response.completed carries a status like every other
// frame and a cancellation announced on one would otherwise be read as the turn
// the model meant to give.
//
// A cancellation is somebody who is not this caller stopping the turn, which is
// not the silent ending a cancelled ctx gets: that caller already knows, and
// this one was told nothing. No code describes it, so it ends as CodeUnknown
// with the fact in Err rather than as an ending it does not mean.
func (c *converter) Convert(f lib.Frame) ([]api.Event, bool) {
	c.events = c.events[:0]
	c.head = frameHead{}
	if err := json.Unmarshal(f.Data, &c.head); err != nil {
		return []api.Event{api.EventErr{
			Code: api.CodeNetwork,
			Err:  fmt.Errorf("gpt: stream frame did not parse: %w", err),
		}}, false
	}
	c.ev = streamEvent{Type: c.head.Type}
	ev := &c.ev
	var err error
	switch ev.Type {
	case "error", "response.created", "response.in_progress", "response.queued",
		"response.failed", "response.completed", "response.incomplete":
		err = json.Unmarshal(f.Data, ev)
	case "response.output_item.added", "response.output_item.done",
		"response.function_call_arguments.delta", "response.function_call_arguments.done",
		"response.code_interpreter_call_code.delta", "response.mcp_call_arguments.delta", "response.custom_tool_call_input.delta":
		if err = json.Unmarshal(f.Data, ev); err == nil && ev.OutputIndex == nil {
			err = fmt.Errorf("gpt: %s without output_index", ev.Type)
		}
	case "response.output_text.delta", "response.refusal.delta",
		"response.output_text.done", "response.refusal.done", "response.content_part.done",
		"response.reasoning_text.delta":
		if err = json.Unmarshal(f.Data, ev); err == nil && (ev.OutputIndex == nil || ev.ContentIndex == nil) {
			err = fmt.Errorf("gpt: %s without output_index or content_index", ev.Type)
		}
	case "response.reasoning_summary_text.delta":
		if err = json.Unmarshal(f.Data, ev); err == nil && (ev.OutputIndex == nil || ev.SummaryIndex == nil) {
			err = fmt.Errorf("gpt: %s without output_index or summary_index", ev.Type)
		}
	}
	if err != nil {
		return []api.Event{api.EventErr{
			Code: api.CodeNetwork,
			Err:  fmt.Errorf("gpt: stream frame did not parse: %w", err),
		}}, false
	}
	if ev.Type == "error" {
		return []api.Event{c.inBandError(ev)}, false
	}
	if !c.started {
		if ev.Response == nil || ev.Type != "response.created" && ev.Type != "response.in_progress" && ev.Type != "response.queued" {
			return []api.Event{api.EventErr{
				Code: api.CodeNetwork,
				Err:  fmt.Errorf("gpt: the stream opened with %q, not a response", ev.Type),
			}}, false
		}
		c.started = true
		c.emit(api.EventStart{ID: ev.Response.ID, Model: c.model})
	}
	if ev.Response != nil && ev.Response.Status == "cancelled" {
		c.readUsage(ev.Response)
		c.emit(api.EventErr{
			Code: api.CodeUnknown,
			Err:  &APIError{Message: "the response was cancelled"},
		})
		return c.events, false
	}
	switch ev.Type {
	case "response.failed":
		c.readUsage(ev.Response)
		c.emit(c.responseFailed(ev))
		return c.events, false
	case "response.completed", "response.incomplete":
		if ev.Response == nil && ev.Type == "response.completed" {
			c.emit(api.EventErr{Code: api.CodeNetwork, Err: errors.New("gpt: response.completed without a response")})
			return c.events, false
		}
		c.readUsage(ev.Response)
		c.emit(c.endFor(ev))
		return c.events, false
	case "response.output_item.added":
		err = c.itemAdded(ev)
	case "response.output_item.done":
		err = c.itemDone(ev)
	case "response.output_text.delta", "response.refusal.delta":
		if ev.Type == "response.refusal.delta" {
			c.sawRefusal = true
		}
		if b := c.out[*ev.OutputIndex]; b == nil || b.kind != "message" {
			err = fmt.Errorf("gpt: %s for output %d, which was not announced as a message", ev.Type, *ev.OutputIndex)
		} else if ev.Delta != "" {
			c.emit(api.EventText{Index: c.textIndex(*ev.OutputIndex, *ev.ContentIndex), Text: ev.Delta})
		}
	case "response.output_text.done", "response.refusal.done", "response.content_part.done":
		if ev.Type == "response.refusal.done" {
			c.sawRefusal = true
		}
		c.closeText(*ev.OutputIndex, *ev.ContentIndex)
	case "response.function_call_arguments.delta":
		if ev.Delta != "" {
			b := c.tool(*ev.OutputIndex)
			if !b.sawArgs {
				b.args.SetSeed(c.seed)
			}
			b.sawArgs = true
			b.args.WriteString(ev.Delta)
			c.emit(api.EventTool{Index: b.idx, Args: ev.Delta})
		}
	case "response.function_call_arguments.done":
		b := c.tool(*ev.OutputIndex)
		switch {
		case !b.sawArgs:
			args := ev.Arguments
			if args == "" {
				args = "{}"
			}
			b.sawArgs = true
			c.emit(api.EventTool{Index: b.idx, Args: args})
		case b.args.Sum64() != maphash.String(c.seed, ev.Arguments):
			err = fmt.Errorf("gpt: the arguments done for output %d differ from the ones that streamed", *ev.OutputIndex)
		}
		if err == nil {
			c.emit(api.EventClose{Index: b.idx})
		}
	case "response.code_interpreter_call_code.delta", "response.mcp_call_arguments.delta", "response.custom_tool_call_input.delta":
		want := "code_interpreter_call"
		switch ev.Type {
		case "response.mcp_call_arguments.delta":
			want = "mcp_call"
		case "response.custom_tool_call_input.delta":
			want = "custom_tool_call"
		}
		if b := c.out[*ev.OutputIndex]; b == nil || b.kind != want {
			err = fmt.Errorf("gpt: %s for output %d, which was not announced as a %s", ev.Type, *ev.OutputIndex, want)
		} else if ev.Delta != "" {
			c.emit(api.EventOpaque{Index: c.itemIndex(*ev.OutputIndex), Delta: ev.Delta})
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		if b := c.out[*ev.OutputIndex]; b == nil || b.kind != "reasoning" {
			err = fmt.Errorf("gpt: %s for output %d, which was not announced as reasoning", ev.Type, *ev.OutputIndex)
		} else if ev.Delta != "" && ev.Type == "response.reasoning_summary_text.delta" {
			c.emit(api.EventSummary{Index: c.itemIndex(*ev.OutputIndex), Part: *ev.SummaryIndex, Text: ev.Delta})
		} else if ev.Delta != "" {
			c.emit(api.EventThinking{Index: c.itemIndex(*ev.OutputIndex), Part: *ev.ContentIndex, Text: ev.Delta})
		}
	default:
	}
	if err != nil {
		c.emit(api.EventErr{Code: api.CodeNetwork, Err: err})
		return c.events, false
	}
	c.readUsage(ev.Response)
	return c.events, true
}

// End is every ending response.completed did not announce.
//
// Reaching here at all means no terminal frame arrived, so the reply is
// incomplete however the socket behaved, and the missing terminal event is what
// says so. A body that ran out is therefore not classified: this API has no
// [DONE] sentinel, the stream simply stops, and an adapter that called that a
// failure would report a completed turn as retryable.
//
// A caller's own ctx ending the turn is reported as nothing at all, because
// ctx.Err() already says so. lib.ErrIdle is not that and falls through to the
// last line: it is this adapter's own deadline, invisible in the caller's ctx
// and not something they can work out for themselves, and a stream that stopped
// producing is a connection to retry like any other, which is the whole of what
// CodeNetwork says.
func (c *converter) End(err error) []api.Event {
	if h, ok := err.(*lib.HTTPError); ok {
		return []api.Event{httpError(h, c.model, c.sent, c.declared)}
	}
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil
	}
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}

func (c *converter) item(oi int) *block {
	b, ok := c.out[oi]
	if !ok {
		b = &block{idx: -1}
		c.out[oi] = b
	}
	return b
}

// itemAdded reads an output item the moment it is announced, which is where a
// function call is named and where an item of a kind protocol has no type for
// opens the opaque block that carries it.
//
// A message carries a phase, which separates a preamble before a tool call from
// a final answer, and the codex models want it back on replay. It goes on every
// content part of that message, each of which is a block of its own, so replay
// keeps the parts in one message under the one phase; part zero opens with it
// here and a later part as it opens. A message that announces no phase leaves
// nothing behind, because a value the adapter cannot replay anyway would be
// noise.
//
// The meta stores the phase and the model that claimed it, because those two are
// what replay reads: a phase is one model's label and protocol stamps no model
// on a text block.
//
// A reasoning item announces nothing here. It is named so that it does not reach
// the opaque case, and it opens its block on whichever of its own events comes
// first.
//
// Everything else is an item this protocol has no type for — a tool the service
// ran itself, a call whose input is freeform text, whatever ships next — which
// is what an opaque block is for. Kind is the item's type as the service spells
// it, because that is the name the input array reads it back by, and Data is the
// item as it stands, which the finished item replaces.
//
// The item's type is what every later frame at its output_index is checked
// against. A call_id is what a tool result is matched by, so a function call
// without one fails the turn.
func (c *converter) itemAdded(ev *streamEvent) error {
	var item outputItem
	if err := json.Unmarshal(ev.Item, &item); err != nil {
		return fmt.Errorf("gpt: %s item did not parse: %w", ev.Type, err)
	}
	if item.Type == "" {
		return fmt.Errorf("gpt: %s item without a type", ev.Type)
	}
	oi := *ev.OutputIndex
	c.item(oi).kind = item.Type
	switch item.Type {
	case "message":
		if item.Phase == "" {
			return nil
		}
		data, _ := json.Marshal(messageMeta{Model: c.model, Phase: item.Phase})
		c.item(oi).phase = data
		c.textIndex(oi, 0)
	case "function_call":
		if item.CallID == "" {
			return fmt.Errorf("gpt: function_call at output %d without a call_id", oi)
		}
		b := c.tool(oi)
		c.emit(api.EventTool{Index: b.idx, ID: item.CallID, Name: item.Name})
	case "reasoning":
	default:
		c.emit(api.EventOpaque{Index: c.itemIndex(oi), Kind: item.Type, Data: ev.Item})
	}
	return nil
}

// itemDone reads the finished output item, which is the whole of a hosted tool
// call and the only place a reasoning item's seal appears.
//
// A message and a function call say nothing here their own events did not,
// status included: each of those blocks is closed by the frame that declares its
// text or its arguments complete, and a turn cut short sends neither, so the
// item's status has nothing left to decide.
//
// A reasoning item's encrypted_content exists on the item alone and never
// streams, and the whole item is the provider's own encoding of the thinking,
// which is what goes back on a later turn. A block whose summary streamed keeps
// the item even without encrypted_content, and encode replays it only with
// encrypted_content, or by id when store is on. Nothing readable and nothing sealed
// is an item carrying only its own id, and with no stored response to reference
// it by there is nothing to put in a block.
//
// A hosted tool item arrives complete, results and all, and every field that
// streamed is filled in on it, so this is the payload a later turn sends back
// and it replaces what the item opened with. The block ends where the item does.
//
// An item output_item.added never announced is a broken frame and fails the
// turn.
func (c *converter) itemDone(ev *streamEvent) error {
	var item outputItem
	if err := json.Unmarshal(ev.Item, &item); err != nil {
		return fmt.Errorf("gpt: %s item did not parse: %w", ev.Type, err)
	}
	if b := c.out[*ev.OutputIndex]; b == nil || b.kind == "" {
		return fmt.Errorf("gpt: %s for output %d, which was never announced", ev.Type, *ev.OutputIndex)
	}
	switch item.Type {
	case "":
		return fmt.Errorf("gpt: %s item without a type", ev.Type)
	case "message", "function_call":
	case "reasoning":
		b := c.item(*ev.OutputIndex)
		if b.idx < 0 {
			if item.EncryptedContent == "" {
				return nil
			}
			b.idx = c.take()
			c.emit(api.EventThinking{Index: b.idx})
		}
		c.emit(api.EventMeta{Index: b.idx, Data: ev.Item})
		c.closeItem(b.idx, item.Status)
	default:
		idx := c.itemIndex(*ev.OutputIndex)
		c.emit(api.EventOpaque{Index: idx, Kind: item.Type, Data: ev.Item})
		c.closeItem(idx, item.Status)
	}
	return nil
}

// closeItem ends the block an output item became. An item the token cap cut ends
// as incomplete, and a block the turn cut short gets no close: what the item
// carried is real and goes out, and the missing close is what says the rest of
// it never arrived.
func (c *converter) closeItem(idx int, status string) {
	if status == "incomplete" {
		return
	}
	c.emit(api.EventClose{Index: idx})
}

// textIndex finds or opens the block for one content part of one message. The
// key is the pair, because content_index counts within an item and two items
// each have a part zero; keying on either alone merges two blocks into one.
func (c *converter) textIndex(oi, ci int) int {
	b := c.item(oi)
	if idx, ok := b.parts[ci]; ok {
		return idx
	}
	idx := c.take()
	if b.parts == nil {
		b.parts = map[int]int{}
	}
	b.parts[ci] = idx
	if b.phase != nil {
		c.emit(api.EventText{Index: idx})
		c.emit(api.EventMeta{Index: idx, Data: b.phase})
	}
	return idx
}

// closeText closes a content part that opened. A part the service declares
// finished without ever having sent a fragment for it never opened one.
func (c *converter) closeText(oi, ci int) {
	b, ok := c.out[oi]
	if !ok {
		return
	}
	idx, open := b.parts[ci]
	if !open {
		return
	}
	delete(b.parts, ci)
	c.emit(api.EventClose{Index: idx})
}

// tool opens the block for a function call. Arguments can arrive before the
// call is named, so the index is allocated by whichever frame mentions the item
// first and the identity goes out when the item is announced.
func (c *converter) tool(oi int) *block {
	b := c.item(oi)
	if b.idx < 0 {
		b.idx = c.take()
	}
	return b
}

// itemIndex opens the block an output item becomes whole, which is a reasoning
// item and a hosted tool call. Whichever of the item's events arrives first
// opens it: reasoning has two kinds of fragment and a finished item that carries
// a seal, a hosted call has an announcement and the fragments of the one field
// it streams.
//
// Which part a fragment belongs to is the wire's own counter and rides on the
// event, so nothing is tracked here. The service indexes summaries and reasoning
// content in separate namespaces and guarantees no order between the parts of
// either, so a single value naming the part the last fragment came from would
// read every alternation of two interleaved parts as a boundary.
func (c *converter) itemIndex(oi int) int {
	b := c.item(oi)
	if b.idx < 0 {
		b.idx = c.take()
	}
	return b.idx
}

// readUsage does the arithmetic api.EventUsage wants. The service reports
// its details relative to their parents — cached and written prompt tokens are
// both inside input_tokens, reasoning tokens are inside output_tokens — and
// each of the five is priced differently, so passing them through would misprice
// every turn.
func (c *converter) readUsage(r *responseObject) {
	if r == nil || r.Usage == nil {
		return
	}
	u := r.Usage
	read := u.InputTokensDetails.CachedTokens
	written := u.InputTokensDetails.CacheWriteTokens
	thought := u.OutputTokensDetails.ReasoningTokens
	e := api.EventUsage{
		Input:      clampZero(u.InputTokens - read - written),
		Output:     clampZero(u.OutputTokens - thought),
		Reasoning:  thought,
		CacheRead:  read,
		CacheWrite: written,
	}
	if c.hasUsage && e == c.usage {
		return
	}
	c.usage, c.hasUsage = e, true
	c.emit(e)
}

func clampZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// endFor reads a terminal frame, and the split it produces is whether the turn
// is worth keeping rather than how the service framed it. Only a completed
// response with no refusal in it is, which is why response.incomplete never
// produces an EventOk: every reason it carries is a turn the caller discards and
// then does something about, and the code says which something.
//
// A refusal arrives as a completed response with a refusal part rather than as a
// status of its own, and a filtered one arrives as an incomplete response with
// content_filter. They are one code because the caller does the same thing with
// either — change the input and ask again — and the line between the model
// declining and the filter cutting in is the service's own and moves.
//
// max_tokens is the spelling in a stale documented example; the enum says
// max_output_tokens. Both are the cap, so both are CodeCapped, and any other
// reason is CodeUnknown with the spelling in Err: a reason nobody here knows
// rounded to the nearest code is a turn the caller mishandles quietly, where an
// unclassified one is a turn it can look at.
func (c *converter) endFor(ev *streamEvent) api.Event {
	if ev.Type == "response.incomplete" {
		reason := ""
		if ev.Response != nil && ev.Response.IncompleteDetails != nil {
			reason = ev.Response.IncompleteDetails.Reason
		}
		switch reason {
		case "content_filter":
			return api.EventErr{Code: api.CodeRefused, Err: &APIError{Code: reason, Message: "the response was filtered"}}
		case "max_output_tokens", "max_tokens":
			return api.EventErr{Code: api.CodeCapped, Err: &APIError{Code: reason, Message: "the output was cut at the token cap"}}
		}
		return api.EventErr{
			Code: api.CodeUnknown,
			Err:  &APIError{Code: reason, Message: fmt.Sprintf("the response was incomplete, reason %q", reason)},
		}
	}
	if c.sawRefusal {
		return api.EventErr{Code: api.CodeRefused, Err: &APIError{Message: "the model refused to answer"}}
	}
	return api.EventOk{Code: api.CodeTurn}
}

// inBandError reads the error frame. It arrives in two incompatible shapes: the
// documented one is flat, the one seen on the wire nests everything under
// error, and both official SDKs detect only the nested one.
func (c *converter) inBandError(ev *streamEvent) api.EventErr {
	e := &APIError{}
	if ev.Error != nil {
		e.Type, e.Code, e.Message, e.Param = ev.Error.Type, ev.Error.Code, ev.Error.Message, ev.Error.Param
	} else {
		e.Code, e.Message, e.Param = ev.Code, ev.Message, ev.Param
	}
	return api.EventErr{Code: inBandCode(e.Code, e.Type), Err: e}
}

// responseFailed reads the terminal frame that says the turn failed. The nested
// error object is the only discriminator there is, so a frame that carries none
// classifies as CodeUnknown: what went wrong is unnamed, and guessing at it from
// the sentence beside it is how a caller ends up retrying something no retry
// fixes.
func (c *converter) responseFailed(ev *streamEvent) api.EventErr {
	e := &APIError{Message: "the response failed"}
	if ev.Response != nil && ev.Response.Error != nil {
		e.Type, e.Code, e.Message, e.Param = ev.Response.Error.Type, ev.Response.Error.Code, ev.Response.Error.Message, ev.Response.Error.Param
	}
	return api.EventErr{Code: inBandCode(e.Code, e.Type), Err: e}
}

// request encodes the turn. http.NewRequestWithContext cannot fail on what is
// passed to it here — the method is a constant and the URL is the configured
// base with a fixed path — so a failure is a base URL that was never valid,
// which is a mistake at construction rather than an outcome of this turn.
func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	body, err := encodeRequest(p.model.Name, p.set, req)
	if err != nil {
		return nil, 0, err
	}
	payload, err := lib.MarshalBody(body)
	if err != nil {
		return nil, 0, fmt.Errorf("gpt: encoding the request: %w", err)
	}
	sent := len(payload.Bytes())
	if p.model.RequestBytes > 0 && sent > p.model.RequestBytes {
		return nil, sent, fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, sent, p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := payload.Request(ctx, p.baseURL+"/responses")
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	if p.org != "" {
		httpReq.Header.Set("OpenAI-Organization", p.org)
	}
	if p.project != "" {
		httpReq.Header.Set("OpenAI-Project", p.project)
	}
	return httpReq, sent, nil
}
