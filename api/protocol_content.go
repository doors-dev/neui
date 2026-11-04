package api

import (
	"bytes"
	"encoding/json"
	"errors"
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

// Content is one block of a message. The set is limited to what every provider
// can express, plus ContentOpaque.
//
// Meta is a block's own adapter data, in whatever shape the issuing adapter
// reads back; nothing between here and that adapter interprets it. Folding a
// stream, a later Meta replaces the one before it whenever it is not empty. Only
// ContentText, ContentToolUse and ContentReasoning carry it, being the blocks a
// stream fills. On the user-authored blocks — ContentText, ContentImage,
// ContentDocument — Meta is the caller's data instead; no adapter reads or
// sends it.
//
// Where a wire takes one string and the message holds several blocks, the
// adapter joins them with a blank line. A single newline runs the end of one
// block into the start of the next, and a list, a heading or a code fence loses
// its framing at exactly that join, so the blank line is the requirement and not
// the cautious choice. Putting the fragments of one block back together is a
// different operation and adds nothing between them.
type Content interface {
	contentType() string
	json.Marshaler
}

// ContentText is a plain text block.
type ContentText struct {
	Text string          `json:"text"`
	Meta json.RawMessage `json:"meta,omitempty"`
}

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
	Data []byte          `json:"data"` // base64 on wire
	Mime string          `json:"mimeType"`
	Meta json.RawMessage `json:"meta,omitempty"` // caller data; adapters ignore it
}

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
	Data []byte          `json:"data"` // base64 on wire
	Mime string          `json:"mimeType"`
	Name string          `json:"name"`
	Meta json.RawMessage `json:"meta,omitempty"` // caller data; adapters ignore it
}

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

// ContentToolUse is a tool call the model asked for.
//
// Input is the arguments as valid JSON. Nil means one thing and nothing else:
// the provider sent no arguments, so the call takes none. It is stored as nil
// and nothing on the way to storage rewrites it to an empty object; an adapter
// encoding the call writes {} where its wire insists on an object.
//
// Arguments that arrived and never formed valid JSON are discarded rather than
// stored — a fragment of JSON is not JSON and cannot be sent back — and the fold
// that built the block puts DiscardedArguments in Input in their place. The
// discarded text survives only in the log that recorded it, because the result
// goes back with every later request of the conversation and an excerpt buys
// nothing a reissued call does not.
//
// A call carrying DiscardedArguments, like a call with nil Input, is always
// answered by a ContentToolResult, and the result answering a discarded call
// carries ToolErrJSON. That is an invariant of whoever dispatches the calls
// rather than an intention, so an adapter may assert it, and every adapter reads
// the paired result before the call: where the result carries ToolErrJSON the
// call is encoded however the wire requires and Input is not read at all, and a
// call carrying DiscardedArguments with no such result is refused with
// CodeInvalid rather than sent as {}, which would replay as a legitimate call
// that takes no arguments.
//
// Only a turn that ended EventOk can reach here with discarded arguments; a turn
// cut short ends in EventErr and is discarded whole.
//
// Input needs omitempty because json.RawMessage reads a null back as the four
// bytes of the word, so leaving the field out is how nil survives a round trip.
type ContentToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

func (c ContentToolUse) LostResult() ContentToolResult {
	return ContentToolResult{
		ID: c.ID,
		Content: UserContents{ContentText{
			Text: "<noresult>result was lost or call never ran</noresult>",
		}},
		Error: ToolErrRuntime,
	}
}

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

// DiscardedArgumentsError reports a tool call whose arguments never formed valid
// JSON, so the call was not made and the arguments were not stored. Answer the
// call with it as a failed tool result, or the model waits on a call that never
// ran.
//
// Raw is the whole accumulation, where the caller still holds one. A caller that
// kept it only in a log leaves it empty and the model is told to reissue the call
// with nothing quoted back. Error repeats only its start, because the result is
// resent with every later request of the conversation.
type DiscardedArgumentsError struct {
	Raw string
}

const discardedArgumentsLimit = 256

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
// in a tool result, and one takes a document in a message but not in a result.
// An adapter leaves a note in place of what it cannot carry and reports the
// drop, so a result always answers the call it is matched to.
type UserContent interface {
	Content
	userContent()
}

func (ContentText) userContent()     {}
func (ContentImage) userContent()    {}
func (ContentDocument) userContent() {}

// ContentToolResult reports the outcome of a ContentToolUse, matched by
// ToolUseID.
//
// Its position among the blocks of its message is the caller's arrangement, kept
// only as far as the wire allows: one provider takes a result as a block in a
// user turn, where any order survives, another as a message that must directly
// follow the assistant turn, which cannot hold a result written after text.
// Error classifies the outcome and goes on no provider wire: only one provider
// has a field for a failed result at all, and what the model reads about the
// failure is in Content, exactly as it is for a call that succeeded.
//
// Content is the blocks the call produced, in order; most tools produce one.
type ContentToolResult struct {
	ID      string       `json:"toolUseId"`
	Content UserContents `json:"content"`
	Error   ToolError    `json:"error"`
}

// ToolError says how a call turned out. It is a classification for whoever
// routes the result and never text for the model, which reads
// ContentToolResult.Content and nothing else.
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

func (c ContentToolResult) Refused() ContentToolResult {
	return ContentToolResult{
		ID: c.ID,
		Content: UserContents{ContentText{
			Text: "<noresult>output was omitted, because LLM provider refused to answer</noresult>",
		}},
		Error: c.Error,
	}
}

// NewContentToolResult builds a result from returned blocks and error. A
// DiscardedArgumentsError classifies as ToolErrJSON, which is what the pairing
// invariant ContentToolUse states requires of the result answering a discarded
// call; every other error is the call's own.
func NewContentToolResult(id string, content UserContents, err error) ContentToolResult {
	if err != nil {
		failure := ToolErrRuntime
		var discarded DiscardedArgumentsError
		if errors.As(err, &discarded) {
			failure = ToolErrJSON
		}
		return ContentToolResult{
			ID:      id,
			Content: UserContents{ContentText{Text: err.Error()}},
			Error:   failure,
		}
	}
	return ContentToolResult{
		ID:      id,
		Content: content,
	}
}

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
// hands over, often more than one at once.
//
// Summary and Thoughts are separate fields because which of them arrives differs
// by provider and a consumer rendering one has to know which it has. Both are
// maps of parts keyed by the provider's Part, the counter EventThinking and
// EventSummary carry: one API indexes them and promises no order between the
// parts it streams, and rejoining parts it sent apart adds separators the model
// never generated. The key is kept so a consumer rendering mid-stream can match
// a later event to the entry it fills; SummaryParts and ThoughtParts give the
// parts in Part order. Providers that index neither fill one entry of the field
// they use, at 0. Either field, and Meta, is legitimately empty.
//
// Wherever a provider signs or seals reasoning, Meta is the replay unit and the
// text never is. Its bytes go back exactly as they arrived: such a provider
// requires the block echoed on every later request and rejects the request
// otherwise, and a signature covers bytes nobody can re-derive. Which bytes to
// keep is the issuing adapter's choice. Rebuilding a block from Thoughts is only
// for providers that sign nothing; where one signs or seals, a block with no
// Meta is not replayed at all, since it cannot be made valid and what is
// readable may be a Summary rather than the thinking. Dropping one costs that
// turn's reasoning and nothing else; no block is bound to another.
//
// Whether replay is required, ignored or forbidden differs by provider, and for
// one depends on whether the turn called a tool, so whether a stored block is
// replayable is the adapter's decision and not a comparison this layer
// specifies.
//
// A sealed payload is bound to its issuer and fails at any other provider.
// Nothing here records which provider issued a block: whoever stores the
// conversation hands an adapter a thread its own provider issued, or strips what
// it did not.
//
// Model is the model the producing provider was configured with, the string
// EventStart.Model reports, not the name the service said it served: a provider
// configured the same way writes the same string, so a stored block compares
// equal with == to the model being sent now.
type ContentReasoning struct {
	Model    string          `json:"model"`
	Summary  Indexed[string] `json:"summary"`  // paraphrase written by the service
	Thoughts Indexed[string] `json:"thoughts"` // thinking as the model generated it
	Meta     json.RawMessage `json:"meta,omitempty"`
}

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

// ContentOpaque carries a payload this protocol has no type for, such as a tool
// the provider ran itself. Sealed thinking is not one of these: it is
// ContentReasoning with nothing readable in it.
//
// Kind is the issuing adapter's own name for what the payload is, not for who
// issued it, and Data is whatever that adapter needs handed back, stored as the
// bytes its API sent because the API signed them. Nothing outside that adapter
// reads either, and an adapter handed a Kind it never issued does not try to
// make sense of the Data.
//
// This layer specifies only how the pieces combine when a harness folds a
// stream: Data replaces, and Deltas append in arrival order. Deltas are
// fragments of a field inside Data, for payloads that stream one, so a payload
// is not whole when it opens. They stay beside Data because only the issuing
// adapter knows which field they belong to and it is the one that puts them
// back.
//
// Model is the model the producing provider was configured with, as on
// ContentReasoning.
type ContentOpaque struct {
	Model  string          `json:"model"`
	Kind   string          `json:"kind"`
	Data   json.RawMessage `json:"data"`
	Deltas []string        `json:"deltas"`
}

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
