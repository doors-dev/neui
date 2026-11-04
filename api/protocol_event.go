package api

import "encoding/json"

// Event is one step of a streamed reply. Applying a whole sequence must
// reconstruct exactly the Message the turn produced. Events stay in the process
// and are never serialized.
//
// The grammar is:
//
//	stream    := EventStart? body (EventOk | EventErr)?
//	body      := any interleaving of block* and EventUsage*
//	block     := text | tool | reasoning | opaque
//	text      := (EventText | EventMeta)+ EventClose?
//	tool      := (EventTool | EventMeta)+ EventClose?
//	reasoning := (EventThinking | EventSummary | EventMeta)+ EventClose?
//	opaque    := EventOpaque+ EventClose?
//
// A block production describes the events at one Index, not a contiguous run of
// the stream: keep the events carrying a given Index and that subsequence
// matches one block. Blocks say nothing about each other's order, several may be
// open at once, and a block that pauses and resumes is one block, except that an
// adapter unable to tell a resume from a new block may open a second. A consumer
// keeps a block per Index and must not assume the one it is filling is the one
// the next event belongs to.
//
// Index is the position of the block in the reconstructed Message.Content. The
// adapter owns and counts it; no provider index is usable as is.
//
// The first event at an Index opens that block and says which kind it is, so it
// is never an EventMeta. An adapter with nothing readable to send yet opens with
// the zero value of the event that kind streams and sends the meta after; an
// event carrying no content adds none.
//
// Repeated events at an Index combine field by field: a field carrying content
// appends, a field carrying identity or adapter data replaces whenever it is not
// empty, and an event carrying a Part appends within that part. An adapter emits
// what it has the moment it has it, in whatever order its wire produces it, and
// keeps nothing back.
//
// A block is whole at its EventClose where one arrives, and at the terminal
// event otherwise. A stream that ends with neither is a reply that was cut off:
// the consumer has an incomplete Message and discards the turn whole. Which
// block was cut is never reported. Cancelling ctx ends a turn the same silent
// way. See Provider.Stream.
//
// Keepalives are not forwarded. An adapter consumes them to hold its own read
// deadline open and emits nothing.
type Event interface {
	eventType() string
}

// IndexedEvent is implemented by the events that name a block. Route a stream by
// block through this rather than by listing the seven types that carry an Index.
type IndexedEvent interface {
	Event
	At() int
	Offset(int) IndexedEvent
}

// EventStart opens the turn. Model is the model the provider is configured with
// and just sent, not the name the service reports serving: the stamp is only
// worth something if a later turn through the same configuration writes a string
// that compares equal. See ContentReasoning.Model.
type EventStart struct {
	ID    string
	Model string
}

func (EventStart) eventType() string {
	return "start"
}

// EventText fills the text block at Index.
type EventText struct {
	Index int
	Text  string // appends
}

func (EventText) eventType() string {
	return "text"
}

func (e EventText) At() int {
	return e.Index
}

func (e EventText) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventTool fills the tool call at Index. Nothing says which field comes first:
// one provider fragments the name, and one delivers arguments before the call
// they belong to is named.
//
// Args are not valid JSON until the block ends and are not guaranteed to be
// valid even then, since a token limit can cut them mid parameter and no
// provider validates them. The call is known at the block's EventClose where the
// wire frames one, and otherwise when the turn stops, never earlier.
//
// ID is what a tool result is later matched by. Where a provider sends none or
// sends a placeholder, the adapter mints a stable ID rather than passing an
// empty string on, and replaces it if the real one turns up later.
//
// A minted ID must be unique across the thread and not merely across the turn,
// because the whole thread goes back in one request body: two calls sharing an
// ID there is a pairing nothing can resolve, and no later turn can repair it. An
// adapter reaches thread uniqueness by folding the provider's own ID for the
// turn into what it mints. That also bounds it: a reply that names neither the
// call nor the turn cannot meet the rule, since a mint built on a constant
// prefix repeats itself from one turn to the next.
type EventTool struct {
	Index int
	ID    string // replaces, when not empty
	Name  string // appends
	Args  string // appends
}

func (EventTool) eventType() string {
	return "tool"
}

func (e EventTool) At() int {
	return e.Index
}

func (e EventTool) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventThinking fills the thought at Part of the reasoning block at Index.
//
// Part is the provider's own counter for the run the fragment came from, carried
// through as it arrived; providers that index nothing send 0. Runs may
// interleave, so whoever folds the stream sorts by Part and reads nothing into
// gaps or spacing; within one Part the fragments append in arrival order.
// Joining two runs would add a separator the model never generated.
type EventThinking struct {
	Index int
	Part  int
	Text  string // appends to the thought at Part
}

func (EventThinking) eventType() string {
	return "thinking"
}

func (e EventThinking) At() int {
	return e.Index
}

func (e EventThinking) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventSummary fills the summary at Part of the reasoning block at Index, by the
// rule EventThinking states for Part. A summary is a paraphrase a service wrote
// of thinking it did not show, so a consumer must not render it as thinking.
type EventSummary struct {
	Index int
	Part  int
	Text  string // appends to the summary at Part
}

func (EventSummary) eventType() string {
	return "summary"
}

func (e EventSummary) At() int {
	return e.Index
}

func (e EventSummary) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventMeta attaches adapter data to the block at Index. It never opens a block,
// and it is valid at every kind but opaque, where ContentOpaque.Data already
// carries anything the adapter would say.
//
// Data is whatever the adapter emitting it needs handed back on a later turn. It
// arrives whole rather than in pieces, which is why it has no part, and nothing
// between here and that adapter reads it. Where a provider signs or seals a
// block this is the replay unit. See ContentReasoning.
//
// A reasoning block carrying only this is thinking a provider sealed and showed
// nothing of, opened by an empty EventThinking ahead of the seal.
type EventMeta struct {
	Index int
	Data  json.RawMessage // replaces, when not empty
}

func (EventMeta) eventType() string {
	return "meta"
}

func (e EventMeta) At() int {
	return e.Index
}

func (e EventMeta) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventOpaque fills the opaque block at Index.
//
// Data is whatever the adapter emitting it needs handed back on a later turn,
// and nothing between here and that adapter reads it. See ContentOpaque.
//
// Delta is a fragment of a field inside Data, for the payloads that stream one.
// Each becomes an entry of ContentOpaque.Deltas and is not put back into Data
// here: the adapter that emitted them reassembles them on the way back, since it
// is the only one that knows which field they belong to.
//
// Kind is the adapter's own name for the payload and is all that identifies it.
// Sealed thinking does not come this way; it is a reasoning block carrying an
// EventMeta.
type EventOpaque struct {
	Index int
	Kind  string          // replaces, when not empty
	Data  json.RawMessage // replaces, when not empty
	Delta string          // appends as one entry of Deltas
}

func (EventOpaque) eventType() string {
	return "opaque"
}

func (e EventOpaque) At() int {
	return e.Index
}

func (e EventOpaque) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventClose reports the block at Index complete before the turn is; for a tool
// call it is the point the arguments become parseable. An adapter emits it where
// its wire frames the end of one specific block and nowhere else, so a frame
// saying the whole message is complete closes nothing here, and a wire that
// frames no block emits none all turn.
//
// Absence carries no information. A block with no close may have ended when the
// turn did, been cut short, or streamed on a wire that never says. A consumer
// asking whether a block is complete asks whether it closed or whether the turn
// has stopped.
type EventClose struct {
	Index int
}

func (EventClose) eventType() string {
	return "close"
}

func (e EventClose) At() int {
	return e.Index
}

func (e EventClose) Offset(n int) IndexedEvent {
	e.Index += n
	return e
}

// EventUsage reports what the turn has cost so far. It can arrive more than once
// and before the turn ends, and some providers send none. Every event carries
// the complete total known at that moment rather than an increment, so the last
// one seen is the answer and nothing downstream merges anything.
//
// A total may go down: a counter that is part of another arrives late on at
// least one provider, and the event that finally separates them lowers both.
// Keep the last event rather than the difference between two, which reads that
// correction as negative spend.
//
// The counters are disjoint, so the prompt was Input+CacheRead+CacheWrite tokens
// long and the model generated Output+Reasoning. Providers report this every
// other way and adapters do the arithmetic, because each counter is priced
// differently.
//
// A zero is a count of zero or a provider that never said, and the two are not
// told apart here. The cache counters are still how a caching strategy is
// checked, with that caveat: a prefix meant to be stable that never reports a
// read is either not being cached or cached by a provider that does not say so.
type EventUsage struct {
	Input      int // prompt tokens processed fresh
	Output     int // generated tokens the model showed
	Reasoning  int // tokens spent thinking
	CacheRead  int // prompt tokens served from cache
	CacheWrite int // prompt tokens written into cache
}

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

// ErrCode says why the turn is unusable and what the caller does about it. The
// order of the values is the grouping: change the input, adjust the request and
// send again, retry unchanged, give up, decide for yourself. This is the small
// set worth branching on, and Err keeps the failure as the provider spelled it.
//
// CodeCapped and CodeContextLength stay apart because their fixes are opposite:
// raising the cap makes an overflow worse. CodeTooLarge stays apart from both
// because it is bytes rather than tokens: the request is past what the API
// accepts before a token is counted, so what fixes it is sending less of what
// takes bytes rather than summarizing what takes context.
type ErrCode string

const (
	CodeRefused       ErrCode = "refused"        // the model or the provider would not produce this reply; change the input
	CodeCapped        ErrCode = "capped"         // output was cut at the provider's configured response cap; raise it or continue
	CodeContextLength ErrCode = "context_length" // the conversation does not fit the model; compact it and retry
	CodeTooLarge      ErrCode = "too_large"      // the request is over Model.RequestBytes; shed media and retry
	CodeNetwork       ErrCode = "network"        // connection, stream or deadline failed; retry as sent
	CodeBusy          ErrCode = "busy"           // rate limited or out of capacity; back off and retry
	CodeInvalid       ErrCode = "invalid"        // the request was refused as written; fix the request
	CodeAccess        ErrCode = "access"         // credentials, permission or balance; no request fixes it
	CodeUnknown       ErrCode = "unknown"        // nothing was recognized; Err carries it and the caller decides
)

// EventOk ends the turn with a reply worth keeping: the caller folds what it
// holds into a Message and stores it.
//
// Adapters emit it only once nothing else can follow, since several providers
// send the final usage after the frame that says how the turn ended.
//
// It closes every block still open, and a block that got no EventClose is not
// thereby unfinished. See EventClose.
type EventOk struct {
	Code OkCode
}

func (EventOk) eventType() string {
	return "ok"
}

// EventErr ends the turn without a reply worth keeping. Nothing from it enters
// the conversation: the caller stores no Message and acts on Code instead.
//
// Which of the two terminal events ends a turn follows from the outcome and
// never from which frame carried it. The same condition is a stop value on one
// wire and a status code on another, and on one provider it is both.
//
// Content that already streamed still stands. One provider reports running out
// of capacity inside a 200 after text has gone out, so this says nothing more
// arrives rather than that nothing arrived. Which block was cut is not said
// either. See EventClose.
//
// A provider value the normalization does not recognize is never rounded to the
// nearest OkCode. It ends the turn here with CodeUnknown and the raw string in
// Err, so a vocabulary that grew arrives as something unclassified rather than
// as a turn that quietly reads as complete.
//
// Cancelling ctx is not reported here: the channel closes with no code of its
// own, and CodeUnknown is not that. See Provider.Stream.
type EventErr struct {
	Code ErrCode
	Err  error // the failure as it happened, for logs and errors.Is; always set
}

func (EventErr) eventType() string {
	return "err"
}
