package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

type blockKind int

const (
	kindText blockKind = iota
	kindTool
	kindReasoning
	kindOpaque
)

// block is everything the converter has to remember about one wire block: which
// kind it is, which adapter index it claimed, and, for a tool call, whether any
// argument fragment was ever emitted. None of it grows with the content, which
// is the point: what arrives goes out immediately and is not kept.
type block struct {
	kind    blockKind
	index   int
	open    bool
	fragged bool
}

// converter turns one frame of this API's wire format into protocol events. Its
// whole job is translating a wire index into an adapter index, holding the stop
// reason until the stream really ends, and emitting every piece the moment it
// arrives.
//
// blocks is keyed by the provider's index. That index is real and contiguous,
// but it counts every block type Anthropic emits, including the ten or so this
// protocol has no place for, so passing it through would leave holes. next is
// what the protocol actually sees. An entry is dropped at content_block_stop, so
// blocks is a translation table and never a set of open blocks to sweep when the
// turn ends.
//
// model is the name this provider is configured with, which is what went out on
// the wire and what every block of the turn is stamped with. The name this API
// answers with is a different string on ordinary traffic — a pre-4.6 id resolves
// to a dated snapshot — and it is the one no later request writes, so stamping
// with it would compare a stored block against a vocabulary the caller does not
// use.
//
// summarized is the thinking display the request asked for. A summary and the
// thinking itself both arrive as thinking_delta and nothing on the wire tells
// them apart, so the request is the only thing that knows which one this turn is
// streaming, and a consumer rendering it has to be told.
type converter struct {
	blocks     map[int]*block
	next       int
	model      string
	msgID      string
	usage      usage
	stop       string
	sent       int
	declared   int
	summarized bool
}

func newConverter(model string, summarized bool, sent int, declared int) *converter {
	return &converter{model: model, blocks: map[int]*block{}, summarized: summarized, sent: sent, declared: declared}
}

// Convert applies one frame. message_stop and an in band error each end the
// turn, and nothing follows either of them on this wire. Blocks still open when
// the error arrives are incomplete, so nothing is closed first.
//
// ping is the only keepalive this API has, and reading it already held the idle
// deadline open, which is the whole of its meaning. New event names are promised
// by the versioning policy. Neither is an edit to the content block list, so
// neither produces anything.
func (c *converter) Convert(f lib.Frame) ([]api.Event, bool) {
	var ev sseMessage
	if err := json.Unmarshal(f.Data, &ev); err != nil {
		return []api.Event{api.EventErr{
			Code: api.CodeNetwork,
			Err:  fmt.Errorf("claude: malformed %q frame: %w", f.Event, err),
		}}, false
	}
	kind := ev.Type
	if kind == "" {
		kind = f.Event
	}
	switch kind {
	case "message_start":
		return c.onStart(&ev), true
	case "content_block_start":
		return c.onBlockStart(&ev)
	case "content_block_delta":
		return c.onBlockDelta(&ev)
	case "content_block_stop":
		return c.onBlockStop(&ev), true
	case "message_delta":
		return c.onMessageDelta(&ev), true
	case "message_stop":
		return []api.Event{stopEvent(c.stop)}, false
	case "error":
		var kind, msg, errorCode string
		if ev.Error != nil {
			kind, msg = ev.Error.Type, ev.Error.Message
			if ev.Error.Details != nil {
				errorCode = ev.Error.Details.ErrorCode
			}
		}
		code := inbandCode(kind)
		if kind == "rate_limit_error" && errorCode == "enforced_spend_limit_reached" {
			code = api.CodeAccess
		}
		var err error = &APIError{Type: kind, Message: msg, ErrorCode: errorCode}
		if code == api.CodeTooLarge {
			err = measuredTooLarge(c.model, c.sent, c.declared, err)
		}
		return []api.Event{api.EventErr{Code: code, Err: err}}, false
	}
	return nil, true
}

// End is every ending message_stop did not announce.
//
// A body that simply ran out produces nothing. Whether that end was expected is
// not something this adapter can tell — the reply carries no length and the
// socket carries no reason — so the turn closes with no terminal event, which is
// what a consumer reads as an incomplete reply. A caller who cancelled produces
// nothing for the same reason it hears nothing else: ctx.Err() already says so.
//
// lib.ErrIdle is reported, because it is this adapter's own deadline, invisible
// in the caller's ctx and not something they can work out for themselves. It is
// CodeNetwork like any other deadline: the request stands as it was sent and
// resending it is the whole of the fix.
func (c *converter) End(err error) []api.Event {
	if e, ok := err.(*lib.HTTPError); ok {
		failure := readAPIError(e)
		code := statusCode(e.Status)
		if code == api.CodeTooLarge {
			return []api.Event{api.EventErr{Code: code, Err: measuredTooLarge(c.model, c.sent, c.declared, failure)}}
		}
		if spendLimited(failure) {
			code = api.CodeAccess
		}
		if code == api.CodeInvalid && contextOverflow(failure.Message) {
			code = api.CodeContextLength
		}
		return []api.Event{api.EventErr{Code: code, Err: failure}}
	}
	switch {
	case errors.Is(err, lib.ErrIdle):
		return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil
	}
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}

func (c *converter) onStart(ev *sseMessage) []api.Event {
	if ev.Message == nil {
		return nil
	}
	c.msgID = ev.Message.ID
	return append(
		[]api.Event{api.EventStart{ID: ev.Message.ID, Model: c.model}},
		c.onUsage(ev.Message.Usage)...,
	)
}

// onBlockStart opens a wire block and decides which kind of protocol block it
// becomes.
//
// A text block opens with nothing: the first fragment opens the protocol block,
// so a text block that never gets one costs nothing. A thinking block opens
// empty and fills by delta, and its signature arrives last and alone, which is
// what the block is replayed by.
//
// redacted_thinking is thinking the provider sealed rather than showed: no
// readable form, never fragmented, required back unchanged. It is still
// thinking, so it is reasoning carrying only the provider's own encoding of it.
//
// A block that names no type is nothing: the field is absent, or null, which
// reads the same. No block is kept for it, so it claims no index, its deltas
// find nothing to fill and its stop finds nothing to close. Claiming one would
// put a payload with no kind in the message.
//
// Everything else is opaque — server_tool_use, the several tool result kinds,
// container_upload, connector_text, fallback, and whatever ships next. The
// protocol has no type for any of them, but the API wants them back where they
// were to resume a paused turn or to validate the thinking around a model
// boundary, so the payload goes out as it opened and the fragments of it that
// stream follow separately.
func (c *converter) onBlockStart(ev *sseMessage) ([]api.Event, bool) {
	var bs blockStart
	if len(ev.ContentBlock) > 0 {
		if err := json.Unmarshal(ev.ContentBlock, &bs); err != nil {
			return []api.Event{api.EventErr{
				Code: api.CodeNetwork,
				Err:  fmt.Errorf("claude: malformed content_block: %w", err),
			}}, false
		}
	}
	if bs.Type == "" {
		return nil, true
	}
	b := &block{}
	c.blocks[ev.Index] = b
	switch bs.Type {
	case "text":
		b.kind = kindText
	case "thinking":
		b.kind = kindReasoning
	case "redacted_thinking":
		b.kind = kindReasoning
		idx := c.claim(b)
		return []api.Event{
			api.EventThinking{Index: idx},
			api.EventMeta{Index: idx, Data: ev.ContentBlock},
		}, true
	case "tool_use":
		b.kind = kindTool
		return []api.Event{api.EventTool{Index: c.claim(b), ID: c.toolID(bs.ID, ev.Index), Name: bs.Name}}, true
	default:
		b.kind = kindOpaque
		return []api.Event{api.EventOpaque{Index: c.claim(b), Kind: bs.Type, Data: ev.ContentBlock}}, true
	}
	return nil, true
}

// toolID falls back to a minted id, because an empty one is worse than a
// synthetic one: it is what a tool result is matched by later.
func (c *converter) toolID(id string, wire int) string {
	if id != "" {
		return id
	}
	if c.msgID != "" {
		return c.msgID + "_" + strconv.Itoa(wire)
	}
	return "claude_tool_" + strconv.Itoa(wire)
}

// onBlockDelta fills a block from one delta.
//
// A leading empty input_json_delta fragment is common. It appends nothing, so it
// is not a fragment: it opens no block and proves no arguments were sent.
//
// The same delta on an opaque block carries the arguments of a tool the provider
// runs itself, streamed the way it streams any others. They are not spliced into
// the payload here: that means decoding it and encoding it again, which reorders
// its keys and costs the byte fidelity it is kept for.
//
// citations_delta annotates a text block rather than adding to it and there is
// nowhere for a citation to go. Unknown delta types are features this adapter
// has no type for. Both are dropped without opening a block.
func (c *converter) onBlockDelta(ev *sseMessage) ([]api.Event, bool) {
	b := c.blocks[ev.Index]
	if b == nil {
		return nil, true
	}
	var d blockDelta
	if len(ev.Delta) > 0 {
		if err := json.Unmarshal(ev.Delta, &d); err != nil {
			return []api.Event{api.EventErr{
				Code: api.CodeNetwork,
				Err:  fmt.Errorf("claude: malformed content_block_delta: %w", err),
			}}, false
		}
	}
	switch d.Type {
	case "text_delta":
		if b.kind == kindText && d.Text != "" {
			return []api.Event{api.EventText{Index: c.claim(b), Text: d.Text}}, true
		}
	case "input_json_delta":
		if d.PartialJSON == "" {
			return nil, true
		}
		switch b.kind {
		case kindTool:
			b.fragged = true
			return []api.Event{api.EventTool{Index: c.claim(b), Args: d.PartialJSON}}, true
		case kindOpaque:
			return []api.Event{api.EventOpaque{Index: c.claim(b), Delta: d.PartialJSON}}, true
		}
	case "thinking_delta":
		if b.kind == kindReasoning && d.Thinking != "" {
			if c.summarized {
				return []api.Event{api.EventSummary{Index: c.claim(b), Text: d.Thinking}}, true
			}
			return []api.Event{api.EventThinking{Index: c.claim(b), Text: d.Thinking}}, true
		}
	case "signature_delta":
		if b.kind == kindReasoning && d.Signature != "" {
			opened := b.open
			idx := c.claim(b)
			meta := api.EventMeta{Index: idx, Data: sealSignature(d.Signature)}
			if opened {
				return []api.Event{meta}, true
			}
			if c.summarized {
				return []api.Event{api.EventSummary{Index: idx}, meta}, true
			}
			return []api.Event{api.EventThinking{Index: idx}, meta}, true
		}
	}
	return nil, true
}

// sealSignature stores the signature of a thinking block as the string it
// arrived as. It is the whole of what a signed block needs handed back: the
// envelope the API replays one by is built around it on the way out, out of the
// readable half that has already gone out by the time the signature arrives, so
// fabricating an envelope here would mean holding a whole block's thinking back
// to say what was already said. The bytes go through lib.MarshalRaw because the
// signature is checked as bytes.
func sealSignature(signature string) json.RawMessage {
	raw, _ := lib.MarshalRaw(signature)
	return raw
}

// onBlockStop ends one block. content_block_stop is an explicit, unambiguous
// signal, which makes it the earliest point a block can be proven finished and
// the only place a close comes from: a block the turn cut short never gets one,
// and nothing here sweeps what was left open.
//
// A tool block that closes having carried no arguments at all is this API saying
// the call takes none. The input:{} on content_block_start is a documented
// placeholder and never goes out, so an empty object here is the only thing that
// tells a zero argument call apart from one whose arguments were unusable.
//
// A block that produced nothing claimed no index and has nothing to close.
// Claiming one anyway would put an empty block in the message.
func (c *converter) onBlockStop(ev *sseMessage) []api.Event {
	b := c.blocks[ev.Index]
	if b == nil {
		return nil
	}
	delete(c.blocks, ev.Index)
	var events []api.Event
	if b.kind == kindTool && !b.fragged {
		events = append(events, api.EventTool{Index: b.index, Args: "{}"})
	}
	if !b.open {
		return events
	}
	return append(events, api.EventClose{Index: b.index})
}

// onMessageDelta takes the stop reason and the usage the frame carries. The stop
// is held rather than emitted: the flow allows more than one of these and the
// final usage rides on the last, so it only goes out once nothing else can
// follow it.
//
// The unmarshal error is discarded on purpose. The frame was parsed before this
// ran, so delta is already known to be valid JSON, and the only way reading two
// documented fields out of it fails is this API changing the type of one of them
// inside a version. Branching on that is a guard against something nothing has
// been shown to do, and the usage riding on the same frame is worth having
// either way.
func (c *converter) onMessageDelta(ev *sseMessage) []api.Event {
	if len(ev.Delta) > 0 {
		var d messageDelta
		if err := json.Unmarshal(ev.Delta, &d); err == nil && d.StopReason != nil {
			c.stop = *d.StopReason
		}
	}
	return c.onUsage(ev.Usage)
}

// claim hands out this adapter's index. It is deferred to the first event the
// block produces, so a block that turns out to be empty consumes no index and
// leaves no hole behind it.
func (c *converter) claim(b *block) int {
	if !b.open {
		b.index = c.next
		c.next++
		b.open = true
	}
	return b.index
}

func (c *converter) onUsage(u *wireUsage) []api.Event {
	if u == nil {
		return nil
	}
	c.usage.merge(u)
	return []api.Event{c.usage.event()}
}

// usage keeps the last value seen for every counter. message_delta reports only
// the fields that moved and message_start sometimes reports none at all, so a
// field missing from one frame keeps the value it had rather than dropping to
// zero.
type usage struct {
	input, output, cacheRead, cacheWrite, thinking int
}

func (u *usage) merge(w *wireUsage) {
	setIf(&u.input, w.InputTokens)
	setIf(&u.output, w.OutputTokens)
	setIf(&u.cacheRead, w.CacheReadInputTokens)
	setIf(&u.cacheWrite, w.CacheCreationInputTokens)
	if w.OutputTokensDetails != nil {
		setIf(&u.thinking, w.OutputTokensDetails.ThinkingTokens)
	}
}

// event does the one subtraction this API needs. The prompt side is already
// disjoint: input_tokens counts what was neither read from nor written to a
// cache and the two cache counters are its siblings, so the three add up to the
// prompt with nothing to take away. The generated side is not: thinking is
// billed as output, output_tokens is the inclusive total, and
// output_tokens_details.thinking_tokens says how much of it was reasoning.
//
// That breakdown only arrives on the last message_delta of a stream, so earlier
// events report no reasoning and an output inflated by it, and the event that
// finally separates them lowers both. That is correct rather than a gap to
// fill: every event carries the complete total known at that moment, and the
// last one is the answer.
func (u usage) event() api.EventUsage {
	out := u.output - u.thinking
	if out < 0 {
		out = 0
	}
	return api.EventUsage{
		Input:      u.input,
		Output:     out,
		Reasoning:  u.thinking,
		CacheRead:  u.cacheRead,
		CacheWrite: u.cacheWrite,
	}
}

func setIf(dst *int, v *int) {
	if v != nil {
		*dst = *v
	}
}

// stopEvent turns the value this API ends a turn with into the event that ends
// the stream. It yields a whole event rather than a code because the value
// decides which of the two terminal events is emitted: whether the caller keeps
// the turn is the first thing said about it, and a function returning one code
// would leave that decision to be re-derived from the code at every call site.
//
// end_turn, stop_sequence and tool_use all mean the model finished. tool_use is
// not told apart, because a turn that wants results says so by carrying the
// calls: the reason repeats what the content already says, and a caller
// dispatches from the calls it has rather than from a claim about them. An
// absent or empty value is this API ending a message without naming why, which
// is an ordinary completion and not a value nobody recognized — only a named one
// can be that.
//
// pause_turn is a finished turn here, which is a decision rather than an
// equality: it says the API stopped a server-tool loop it would take the turn
// back to continue, and abandoning one is documented as legal and costs an
// incomplete answer, where EventErr would discard content that is real and
// already streamed. NOTES.md carries the restore condition and what a
// continuation needs.
//
// max_tokens and model_context_window_exceeded no longer share a code. Both cut
// the output, and telling them apart used to mean comparing this API's own
// string, which put opposite fixes behind provider specific knowledge: raising
// max_tokens answers the first and makes the second worse. They are CodeCapped
// and CodeContextLength, which is the same pair of codes an overflow reported as
// a 400 elsewhere carries.
//
// The three failures end the turn on EventErr because the caller keeps none of
// them, not because of how this API framed them: a refusal is a 200 with partial
// or no output, and a turn cut at the cap streamed most of an answer, and
// neither enters the conversation. Content that already arrived still arrived.
//
// A named value this switch does not know is CodeUnknown with the raw string in
// Err, never the nearest Ok: a vocabulary that grew has to surface as
// unclassified rather than as a turn that quietly reads as complete.
func stopEvent(native string) api.Event {
	var code api.ErrCode
	switch native {
	case "", "end_turn", "stop_sequence", "tool_use":
		return api.EventOk{Code: api.CodeTurn}
	case "pause_turn":
		return api.EventOk{Code: api.CodeTurn}
	case "max_tokens":
		code = api.CodeCapped
	case "model_context_window_exceeded":
		code = api.CodeContextLength
	case "refusal":
		code = api.CodeRefused
	default:
		code = api.CodeUnknown
	}
	return api.EventErr{Code: code, Err: fmt.Errorf("claude: stop_reason %q", native)}
}

// maxErrorMessage bounds how much of an unparseable error body is kept. At
// gateways it is frequently a whole HTML page.
const maxErrorMessage = 512

// readAPIError decodes a refused request, tolerating a body that is not JSON: a
// 413 is returned by the edge before the request reaches the API and need not
// use the documented envelope at all. The whole error is returned rather than
// the interface, because the message is what separates a conversation over the
// window from every other 400. See contextOverflow.
func readAPIError(h *lib.HTTPError) *APIError {
	var env errorEnvelope
	if err := json.Unmarshal(h.Body, &env); err == nil && env.Error != nil {
		return newAPIError(h.Status, h.Header, &env)
	}
	e := newAPIError(h.Status, h.Header, nil)
	if text := strings.TrimSpace(string(h.Body)); text != "" {
		if r := []rune(text); len(r) > maxErrorMessage {
			text = string(r[:maxErrorMessage]) + "…"
		}
		e.Message = text
	}
	return e
}
