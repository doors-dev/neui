package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const doneSentinel = "[DONE]"

// converter turns frames into events and owns the block indices while it does.
//
// None of DeepSeek's own indices can be used as the protocol's. choices[].index
// is a position among completions, tool_calls[].index counts only tool calls,
// and text and reasoning have no index at all, so the position of a block in
// the reconstructed message is counted here and nowhere else.
//
// An assistant message on this wire holds exactly three kinds of stream:
// content is one, reasoning_content is one, and each tool_calls[i] is one.
// Every fragment says which one it belongs to, so an index is allocated the
// first time a stream is seen and never reassigned, and a stream that goes
// quiet while another produces resumes at the index it already holds. That is
// what makes it one block: v4 interleaves reasoning and text, and a train of
// thought the model returned to is one train of thought.
//
// Text and reasoning take whichever index they are first seen at, so a turn
// that thinks before it speaks gives reasoning 0 and text 1 while one that
// speaks first gives the reverse. Index is position in the reconstructed
// message, and that is where they sit in it.
//
// Nothing is held back. Every fragment leaves as the frame carrying it is read,
// so what is kept is the indices already handed out, the ids already handed
// out, and the finish reason, which the grammar says waits for the end of the
// stream.
//
// ids guards against a tool call id arriving twice in one turn, which is
// reported for v4-pro. Correlation never uses the id, only the array index, so
// it is there purely so no two blocks claim the same handle.
type converter struct {
	model     string
	id        string
	started   bool
	next      int         // the index the next stream seen takes
	text      int         // -1 until content is seen
	reasoning int         // -1 until reasoning_content is seen
	tools     map[int]int // tool_calls[].index to block index
	ids       map[string]bool
	real      map[int]string
	stopped   bool
	sent      int
	declared  int
	finish    string // the wire value, verbatim, once a choice named one
	k         chunk
	events    []api.Event
}

func newConverter(model string, sent int, declared int) *converter {
	return &converter{
		model:     model,
		text:      -1,
		reasoning: -1,
		tools:     make(map[int]int),
		ids:       make(map[string]bool),
		real:      make(map[int]string),
		sent:      sent,
		declared:  declared,
	}
}

// Convert applies one frame. The sentinel and an in band error each end the
// turn, and nothing this adapter can use follows either of them.
//
// A frame carrying both a delta and an error carries both. What it delivered was
// generated and paid for, so it goes out ahead of the failure that ends the
// turn, the way the counters on a failed turn do.
func (c *converter) Convert(f lib.Frame) ([]api.Event, bool) {
	if string(f.Data) == doneSentinel {
		return []api.Event{c.done()}, false
	}
	c.k = chunk{}
	c.events = c.events[:0]
	if err := json.Unmarshal(f.Data, &c.k); err != nil {
		return []api.Event{api.EventErr{
			Code: api.CodeNetwork,
			Err:  fmt.Errorf("deepseek: stream frame did not parse: %w", err),
		}}, false
	}
	if err := c.chunk(&c.k); err != nil {
		return append(c.events, api.EventErr{Code: api.CodeNetwork, Err: err}), false
	}
	if e, ok := inbandError(&c.k); ok {
		return append(c.events, e), false
	}
	return c.events, true
}

// End is every ending the sentinel did not announce. Convert says the turn is
// over at [DONE] and nothing is read after that, so a turn that ran to the
// sentinel never reaches here at all.
//
// A body that ran out after a finish_reason but before the sentinel fails the
// turn with api.CodeNetwork: the usage frame and the sentinel never arrived, and
// only a reply that arrived whole is kept. With no reason yet, the reply really
// was cut, and the absence of a terminal event is the whole of the report:
// neither io.EOF nor io.ErrUnexpectedEOF says anything about whether the server
// meant to stop, and the unexpected one is manufactured by lib's frame reader
// for a body that stopped mid event rather than reported by the transport. A
// cancelled ctx is silent for the other reason: the caller decided it, so there
// is nobody left to tell.
//
// Everything else is api.CodeNetwork, lib.ErrIdle and a read deadline
// included. They are one outcome to a caller, which is that the same request
// may be sent again, and the idle deadline in particular has to be reported
// rather than dropped because it is this adapter's own, invisible in the
// caller's ctx and not something they can work out for themselves. The finish
// reason is not read on that path: a deadline or a broken read is a failure of
// its own, and a caller told the turn completed would never learn of it.
func (c *converter) End(err error) []api.Event {
	if e, ok := err.(*lib.HTTPError); ok {
		failure := apiError(e.Status, e.Header, e.Body)
		if contextOverflow(failure) {
			return []api.Event{api.EventErr{Code: api.CodeContextLength, Err: failure}}
		}
		code := statusCode(e.Status)
		if code == api.CodeTooLarge {
			return []api.Event{api.EventErr{
				Code: code,
				Err:  measuredTooLarge(c.model, c.sent, c.declared, failure),
			}}
		}
		return []api.Event{api.EventErr{Code: code, Err: failure}}
	}
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		if c.stopped {
			return []api.Event{api.EventErr{
				Code: api.CodeNetwork,
				Err:  fmt.Errorf("deepseek: the stream ended after finish_reason %q but before [DONE]: %w", c.finish, err),
			}}
		}
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil
	}
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}

// chunk folds one decoded frame into events. The finish reason itself is held:
// the usage frame and the sentinel still follow it.
//
// The turn is stamped with the model this Provider is configured with, not the
// one the frame echoes back: every block of this turn is stamped from here, and
// a stamp is only worth having if the next request writes the same string.
//
// The frame carrying usage may have an empty choices array, so nothing here may
// assume a first choice exists. Only the choice at index 0 is this turn; any
// other is ignored.
func (c *converter) chunk(k *chunk) error {
	if !c.started {
		c.started = true
		c.id = k.ID
		c.events = append(c.events, api.EventStart{ID: k.ID, Model: c.model})
	}
	for i := range k.Choices {
		ch := &k.Choices[i]
		if ch.Index != 0 {
			continue
		}
		if err := c.delta(&ch.Delta); err != nil {
			return err
		}
		if ch.FinishReason == nil || *ch.FinishReason == "" {
			continue
		}
		c.stopped = true
		c.finish = *ch.FinishReason
	}
	if k.Usage != nil {
		c.events = append(c.events, k.Usage.event())
	}
	return nil
}

// delta applies one incremental message.
//
// Reasoning is handled before text and both are handled unconditionally,
// because v4 is reported to interleave the two rather than finishing its
// thinking first the way the older reasoner model did. A branch that took one
// or the other would silently lose whichever it did not pick, and each field
// fills the block its own stream was given, so thinking that resumes after text
// carries on where it left off.
//
// Emptiness rather than presence is what allocates an index. Both fields arrive
// as null on frames that do not carry them, and the opening frame of a turn
// carries content as an empty string, so an index handed out on presence would
// put an empty text block in front of every reply.
//
// A tool call fragment without an index cannot be correlated and fails the
// turn, and so does a later fragment naming a different real id than the call
// already has.
func (c *converter) delta(d *delta) error {
	if d.ReasoningContent != "" {
		if c.reasoning < 0 {
			c.reasoning = c.alloc()
		}
		c.events = append(c.events, api.EventThinking{Index: c.reasoning, Text: d.ReasoningContent})
	}
	if d.Content != "" {
		if c.text < 0 {
			c.text = c.alloc()
		}
		c.events = append(c.events, api.EventText{Index: c.text, Text: d.Content})
	}
	for i := range d.ToolCalls {
		if d.ToolCalls[i].Index == nil {
			return errors.New("deepseek: tool call fragment without an index")
		}
		e, ok, err := c.toolCall(&d.ToolCalls[i])
		if err != nil {
			return err
		}
		if ok {
			c.events = append(c.events, e)
		}
	}
	return nil
}

// toolCall applies one tool call fragment. Fragments are correlated by the
// provider's array index, not by id: fragments after the first carry no id to
// match on, so the index is the only thing that links them, and the block index
// a call took on first sight is the one every later fragment of it fills.
//
// Whatever the fragment carries goes out as one event, in the state the wire
// delivered it: api.EventTool appends both the name and the arguments, so
// a name arriving in halves is two events and nothing waits for a piece to
// become final. Argument fragments go through split wherever DeepSeek split
// them, including mid escape, and validity is neither checked nor checkable
// here: a fragment that parses is not a fragment that finished, and an
// accumulation that never parses is discarded by whoever accumulates it. A
// fragment carrying nothing but an index for a call already seen is nothing,
// and produces no event.
//
// An id is taken wherever it turns up. A call first seen without one is
// carrying a minted handle, and api.EventTool.ID replaces, so the real one
// arriving late replaces it rather than being dropped: the id is what a tool
// result is matched by. A handle another block already claimed is never taken,
// on either path. Once a call has a real id, a fragment naming another one is a
// broken frame.
func (c *converter) toolCall(t *toolDelta) (api.EventTool, bool, error) {
	e := api.EventTool{}
	index, seen := c.tools[*t.Index]
	switch {
	case !seen:
		index = c.alloc()
		c.tools[*t.Index] = index
		c.real[*t.Index] = t.ID
		id := t.ID
		if id == "" || c.ids[id] {
			id = c.mintID(index)
		}
		c.ids[id] = true
		e.ID = id
	case t.ID == "" || t.ID == c.real[*t.Index]:
	case c.real[*t.Index] != "":
		return e, false, fmt.Errorf("deepseek: tool call %d named %q after %q", *t.Index, t.ID, c.real[*t.Index])
	default:
		c.real[*t.Index] = t.ID
		if !c.ids[t.ID] {
			c.ids[t.ID] = true
			e.ID = t.ID
		}
	}
	e.Index = index
	if t.Function != nil {
		e.Name, e.Args = t.Function.Name, t.Function.Arguments
	}
	if e.ID == "" && e.Name == "" && e.Args == "" {
		return e, false, nil
	}
	return e, true, nil
}

// mintID names a call whose id was missing or already used. It is derived from
// the turn id and the block index, so it is stable for the block and unique
// within the turn without a counter.
func (c *converter) mintID(index int) string {
	base := c.id
	if base == "" {
		base = Name
	}
	return fmt.Sprintf("%s_call_%d", base, index)
}

// alloc hands out the index for a stream seen for the first time.
func (c *converter) alloc() int {
	i := c.next
	c.next++
	return i
}

// done ends the turn at the sentinel.
//
// A sentinel with no finish_reason before it fails the turn with
// api.CodeNetwork: the flow requires a reason, and only a reply that arrived
// whole is kept. A named value nobody knows fails as well. See finishEvent.
//
// No block of this turn carries an EventClose, here or anywhere earlier,
// because this wire frames no block ends at all: deltas arrive on one choice
// with no notion of a block, and finish_reason says the whole message is
// complete rather than that any particular stream within it stopped. Spreading
// that one signal over every index would draw per block boundaries the server
// never drew and say nothing the terminal event does not already say, so the
// terminal event is what completes them and the consumer holding them is what
// applies it.
func (c *converter) done() api.Event {
	if !c.stopped {
		return api.EventErr{Code: api.CodeNetwork, Err: errors.New("deepseek: [DONE] without a finish_reason")}
	}
	return finishEvent(c.finish)
}

// finishEvent turns one finish_reason into the event that ends the turn. Which
// of the two it is follows from whether the caller keeps what streamed, not
// from DeepSeek having framed all six of these the same way.
//
// tool_calls ends the turn as an ordinary completion and takes no code of its
// own. A turn that wants results says so by carrying the calls, so a code would
// repeat what the content already says and a caller dispatches from the calls
// rather than from a claim about them.
//
// insufficient_system_resource is a capacity failure delivered inside a 200,
// after the headers are committed and text has already gone out. Those blocks
// were generated and they really streamed, but the turn is not one to store, so
// it fails, and api.CodeBusy is what says it is worth sending again after
// a backoff.
//
// aborted is documented only as "the generation was interrupted". That nothing
// was wrong in the request is this adapter's inference, not DeepSeek's wording,
// so it fails as api.CodeNetwork: send it again as sent.
//
// A value nobody here recognizes fails with api.CodeUnknown and carries
// the raw string, rather than being rounded to the nearest completion: a
// vocabulary that grew has to arrive unclassified, because a turn cut short for
// a reason this adapter has never seen must not read as one the model finished.
func finishEvent(v string) api.Event {
	switch v {
	case "stop", "tool_calls":
		return api.EventOk{Code: api.CodeTurn}
	case "length":
		return api.EventErr{Code: api.CodeCapped, Err: &FinishError{Reason: v}}
	case "content_filter":
		return api.EventErr{Code: api.CodeRefused, Err: &FinishError{Reason: v}}
	case "insufficient_system_resource":
		return api.EventErr{Code: api.CodeBusy, Err: &FinishError{Reason: v}}
	case "aborted":
		return api.EventErr{Code: api.CodeNetwork, Err: &FinishError{Reason: v}}
	default:
		return api.EventErr{Code: api.CodeUnknown, Err: &FinishError{Reason: v}}
	}
}

// event splits DeepSeek's counters into the five disjoint ones.
//
// Both of DeepSeek's breakdowns are inclusive, not additional: prompt_tokens is
// documented as equal to the two cache counters added together, and
// reasoning_tokens is part of completion_tokens. So the input side is a
// partition of the prompt and the output side is a subtraction.
//
// CacheWrite is always zero, and that is a decision rather than a missing
// field. DeepSeek's cache is automatic, it never reports which missed tokens it
// went on to persist, and its price list has exactly two input tiers, hit and
// miss, with no write tier at all. Putting missed tokens in CacheWrite would
// empty Input, price ordinary input at a rate that does not exist, and make
// every turn look like it wrote a cache it may not have written.
//
// A host in front of DeepSeek that drops the pair leaves the whole prompt as
// fresh input rather than as a cache read nobody reported. And the identity is
// documented but never asserted as a guarantee, so deriving the miss from the
// total keeps Input+CacheRead+CacheWrite equal to the prompt even if the two
// counters ever disagree with it.
func (u *usage) event() api.EventUsage {
	read, in := u.CacheHit, u.CacheMiss
	switch {
	case read == 0 && in == 0:
		in = u.Prompt
	case read+in != u.Prompt && u.Prompt > 0:
		in = u.Prompt - read
	}
	if in < 0 {
		in = 0
	}
	var reasoning int
	if u.Details != nil {
		reasoning = u.Details.Reasoning
	}
	out := u.Completion - reasoning
	if out < 0 {
		out = 0
	}
	return api.EventUsage{
		Input:      in,
		Output:     out,
		Reasoning:  reasoning,
		CacheRead:  read,
		CacheWrite: 0,
	}
}
