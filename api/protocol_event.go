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

import "encoding/json"

// Event is one step of a streamed reply, as Provider.Stream sends it. Folding
// the whole sequence reconstructs the Message the turn produced.
//
//	stream    := EventErr | EventStart body (EventOk | EventErr)?
//	body      := any interleaving of block* and EventUsage*
//	block     := text | tool | reasoning | opaque
//	text      := (EventText | EventMeta)+ EventClose?
//	tool      := (EventTool | EventMeta)+ EventClose?
//	reasoning := (EventThinking | EventSummary | EventMeta)+ EventClose?
//	opaque    := EventOpaque+ EventClose?
//
// A block is the subsequence of events sharing one Index, its position in the
// reconstructed Message.Content, counted by the adapter. Blocks interleave and
// several may be open at once, so a consumer keeps one block per Index. The
// first event at an Index names the block's kind and is never an EventMeta; an
// adapter with nothing readable yet sends the zero value of that kind's event.
// Repeated events at an Index merge field by field: content appends, identity
// and adapter data replace when not empty. Adapters emit as the wire delivers
// and hold nothing back; keepalives are never forwarded.
//
// A block is complete at its EventClose, or at the terminal event. A stream
// that closes with neither, including one ended by cancelling ctx, is a cut-off
// reply and the consumer discards the turn.
type Event interface {
	eventType() string
}

// IndexedEvent is implemented by the events that belong to a block. Route a
// stream by block through it instead of switching over the seven types that
// carry an Index.
type IndexedEvent interface {
	Event
	// At returns the block Index.
	At() int
	// Offset returns a copy with Index shifted by the given amount, for
	// appending a stream after content that already holds that many blocks.
	Offset(int) IndexedEvent
}

// EventStart opens the turn and comes before any block, usage or EventOk event.
type EventStart struct {
	// ID is the provider's id for the response; empty where it sends none.
	ID string
	// Model is the model the provider is configured with and sent, not the
	// name the service reports serving. The turn's reasoning and opaque
	// blocks carry it; see ContentReasoning.Model.
	Model string
}

func (EventStart) eventType() string {
	return "start"
}

// EventText fills the text block at Index.
type EventText struct {
	Index int    // block position in Message.Content
	Text  string // appends
}

func (EventText) eventType() string {
	return "text"
}

// At returns Index.
func (e EventText) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventText) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventTool fills the tool call at Index. Name and Args may arrive fragmented
// and in either order. Args are parseable JSON only once the block closes or the
// turn ends, and may still be invalid if a token limit cut them.
//
// ID pairs the call with its later ContentToolResult. Where the provider sends
// none (or, on some adapters, a duplicate), the adapter mints one from the
// provider's response id and the block position; without a response id the
// minted id is unique only within the turn. A later EventTool with a non-empty
// ID replaces it.
type EventTool struct {
	Index int    // block position in Message.Content
	ID    string // replaces, when not empty
	Name  string // appends
	Args  string // appends
}

func (EventTool) eventType() string {
	return "tool"
}

// At returns Index.
func (e EventTool) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventTool) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventThinking fills the thought at Part of the reasoning block at Index.
type EventThinking struct {
	Index int // block position in Message.Content
	// Part is the provider's own counter for the thought run this fragment
	// belongs to, 0 where it has none. Runs may interleave: fold by sorting
	// on Part, read nothing into gaps, and never join two parts with a
	// separator the model did not generate.
	Part int
	Text string // appends to the thought at Part
}

func (EventThinking) eventType() string {
	return "thinking"
}

// At returns Index.
func (e EventThinking) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventThinking) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventSummary fills the summary at Part of the reasoning block at Index, by the
// rule EventThinking states for Part. A summary is a paraphrase a service wrote
// of thinking it did not show, so a consumer must not render it as thinking.
type EventSummary struct {
	Index int    // block position in Message.Content
	Part  int    // run counter, as in EventThinking.Part
	Text  string // appends to the summary at Part
}

func (EventSummary) eventType() string {
	return "summary"
}

// At returns Index.
func (e EventSummary) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventSummary) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventMeta attaches adapter data to the block at Index, for any kind but
// opaque. It never opens a block. Data arrives whole and nothing but the adapter
// that emitted it reads it, handed back on a later turn; for signed or sealed
// reasoning it is the replay unit (see ContentReasoning). A reasoning block
// holding only a meta, opened by an empty EventThinking, is sealed thinking
// with nothing shown.
type EventMeta struct {
	Index int             // block position in Message.Content
	Data  json.RawMessage // replaces, when not empty
}

func (EventMeta) eventType() string {
	return "meta"
}

// At returns Index.
func (e EventMeta) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventMeta) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventOpaque fills the opaque block at Index: a payload only the emitting
// adapter understands, handed back to it on a later turn. See ContentOpaque.
// Kind is the adapter's name for the payload. Delta is a streamed fragment of a
// field inside Data; each becomes one entry of ContentOpaque.Deltas and the
// adapter reassembles them on the way back.
type EventOpaque struct {
	Index int             // block position in Message.Content
	Kind  string          // replaces, when not empty
	Data  json.RawMessage // replaces, when not empty
	Delta string          // appends as one entry of Deltas
}

func (EventOpaque) eventType() string {
	return "opaque"
}

// At returns Index.
func (e EventOpaque) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventOpaque) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventClose reports the block at Index complete before the turn ends; for a
// tool call it is when Args become parseable. Adapters emit it only where the
// wire frames the end of that specific block, so some wires never emit it. Its
// absence means nothing: a block is complete once it closed or the turn ended.
type EventClose struct {
	Index int // block position in Message.Content
}

func (EventClose) eventType() string {
	return "close"
}

// At returns Index.
func (e EventClose) At() int {
	return e.Index
}

// Offset returns a copy with Index shifted by n.
func (e EventClose) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventUsage reports what the turn has cost so far. It may arrive several times
// or never, and each one carries the full running total rather than an
// increment: keep the last one seen, even when a total goes down as a provider
// corrects itself. The counters are disjoint, so the prompt was
// Input+CacheRead+CacheWrite tokens and the model generated Output+Reasoning. A
// zero is either zero or not reported.
type EventUsage struct {
	Input      int // prompt tokens processed fresh
	Output     int // generated tokens the model showed
	Reasoning  int // tokens spent thinking
	CacheRead  int // prompt tokens served from cache
	CacheWrite int // prompt tokens written into cache
}

// Total returns every counter summed: the context the turn occupied, prompt
// plus generated tokens.
func (u EventUsage) Total() int {
	return u.Input + u.CacheRead + u.CacheWrite + u.Output + u.Reasoning
}

func (EventUsage) eventType() string {
	return "usage"
}

// OkCode says how a turn that is worth keeping ended, for a caller that renders
// it. The decision to store was already made by the event this arrives on. A
// turn that wants tool results has no code of its own, and a caller dispatches
// from the calls it holds.
type OkCode string

const (
	CodeTurn OkCode = "turn" // the model said everything it meant to say
)

// ErrCode says why a turn is unusable and what the caller does about it. The
// constants are grouped by remedy: change the input, adjust the request and
// resend, retry unchanged, give up, decide yourself.
type ErrCode string

const (
	CodeRefused       ErrCode = "refused"        // the model or the provider would not produce this reply; change the input
	CodeCapped        ErrCode = "capped"         // output was cut at the provider's configured response cap; raise it or continue
	CodeContextLength ErrCode = "context_length" // the conversation does not fit the model; compact it and retry
	CodeTooLarge      ErrCode = "too_large"      // the request is too large: over Model.RequestBytes, or refused by the service for its size; send fewer bytes
	CodeNetwork       ErrCode = "network"        // connection, stream or deadline failed; retry as sent
	CodeBusy          ErrCode = "busy"           // rate limited or out of capacity; back off and retry
	CodeInvalid       ErrCode = "invalid"        // the request was refused as written; fix the request
	CodeAccess        ErrCode = "access"         // credentials, permission or balance; no request fixes it
	CodeUnknown       ErrCode = "unknown"        // nothing was recognized; Err carries it and the caller decides
)

// EventOk ends the turn with a reply worth keeping: fold the blocks into a
// Message and store it. Nothing follows it, and it completes every block still
// open.
type EventOk struct {
	Code OkCode // how the reply ended
}

func (EventOk) eventType() string {
	return "ok"
}

// EventErr ends the turn without a reply worth keeping: store no Message and
// act on Code. Content that already streamed was real, but nothing more
// arrives. A provider value the adapter does not recognize ends here as
// CodeUnknown with the raw value in Err, never as a quiet EventOk. Cancelling
// ctx is not reported this way; the channel just closes. See Provider.Stream.
type EventErr struct {
	Code ErrCode // what to do about it
	Err  error   // the failure as it happened, for logs and errors.Is; always set
}

func (EventErr) eventType() string {
	return "err"
}
