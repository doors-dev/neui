package api

import (
	"context"
	"fmt"
	"iter"
	"net/url"
	"path"
	"time"
)

// Seq is a per-thread write serial that acts as an optimistic concurrency
// guard for MessageStorage writes.
type Seq int

// ThreadID names a thread. It is chosen by the caller of ThreadCreate and is
// also the key for the thread's files and workspace.
type ThreadID string

// RoundID is a thread-scoped, monotonically increasing round identifier;
// NoRoundID (-1) is the none/open sentinel.
type RoundID int

// Thread is one conversation as MessageStorage lists it.
type Thread struct {
	ID      ThreadID  `json:"id"`      // stable key for every MessageStorage call on it
	Name    string    `json:"name"`    // display name; "" while unnamed
	Seq     Seq       `json:"seq"`     // current write serial, passed to the next write
	Created time.Time `json:"created"` // when ThreadCreate or ThreadClone made it
	Updated time.Time `json:"updated"` // last content write; ThreadList orders by it
}

// Hit is one round matched by RoundSearch. Snippet is a bounded run of
// the matched text taken verbatim from the round, cut at rune boundaries
// and containing the match.
type Hit struct {
	Thread  ThreadID  `json:"thread"`  // thread holding the round
	Round   RoundID   `json:"round"`   // the matched round
	Created time.Time `json:"created"` // the round's Created
	Snippet string    `json:"snippet"` // matched text with some context
}

// Cursor positions a RoundSearch page. It names one round by the whole sort
// key RoundSearch orders by, so a page resumes exactly where the last ended
// even when several rounds share a creation time. Build it from the last Hit of
// a page.
type Cursor struct {
	Created time.Time `json:"created"` // Hit.Created
	Thread  ThreadID  `json:"thread"`  // Hit.Thread
	Round   RoundID   `json:"round"`   // Hit.Round
}

// RoundState is a round's lifecycle and replay tag; the zero value is
// RoundInput.
type RoundState string

const (
	// RoundInput marks a round that has only its input: its response is
	// still running, or it was interrupted before one was stored. It is
	// skipped on replay and can be retried.
	RoundInput = RoundState("")
	// RoundOk marks a round whose response completed (EventOk), with or
	// without tool calls.
	RoundOk = RoundState("ok")
	// RoundRefuse marks a round the model refused to answer. The model does
	// not see it again, and the tool results it answered reach the model only
	// as a note that they were omitted. It cannot be retried; Rewind it
	// instead.
	RoundRefuse = RoundState("refuse")
	// RoundAbort marks a round the user aborted during response
	// streaming; not replayed, has a retry option.
	RoundAbort = RoundState("abort")
	// RoundError marks a round that hit a network or other error; skipped
	// on replay, has retry.
	RoundError = RoundState("error")
)

// Round is one full exchange — the user input plus the model's response,
// tool results, and usage — and the unit the UI renders as a turn.
type Round struct {
	ID     RoundID    `json:"id"`     // identifies the round within Thread
	Thread ThreadID   `json:"thread"` // thread the round belongs to
	State  RoundState `json:"state"`  // lifecycle; decides replay and retry
	// Created is when the round was opened. It is the zero time on rounds
	// written before storage recorded it.
	Created  time.Time                    `json:"created"`
	Input    Input                        `json:"input"`    // what the user sent
	Error    string                       `json:"error"`    // error text when State is RoundError
	Usage    EventUsage                   `json:"usage"`    // last usage the provider reported
	Response Indexed[Content]             `json:"response"` // the model's blocks, by stream Index
	Results  map[string]ContentToolResult `json:"results"`  // tool results by tool call id
	// PinnedRecords holds the records pinned to this round, filled on reads
	// by RoundRange, RoundBefore and RoundAfter; only RoundRange includes
	// their Data.
	PinnedRecords []Record `json:"records"`
}

// Input is the user-typed text plus attachment filenames given as
// thread-relative paths.
type Input struct {
	Attachments []string `json:"attachments"` // e.g. "user_uploads/report.pdf"
	Text        string   `json:"text"`        // "" for attachments only
}

// Len returns the number of content parts the input may produce: one for
// the text plus one per attachment.
func (in Input) Len() int {
	l := len(in.Attachments)
	if in.Text != "" {
		l += 1
	}
	return l
}

// Content renders the input as protocol content: a file link per
// attachment followed by the text; nil when the input is empty.
func (in Input) Content() []Content {
	if in.Len() == 0 {
		return nil
	}
	content := make([]Content, 0, 1+len(in.Attachments))
	for _, a := range in.Attachments {
		escaped := (&url.URL{Path: a}).EscapedPath()
		content = append(content, ContentText{Text: fmt.Sprintf("[%s](file:///%s)", path.Base(a), escaped)})
	}
	if in.Text != "" {
		content = append(content, ContentText{Text: in.Text})
	}
	return content
}

// NoCompaction is the none sentinel Compaction, returned when no
// compaction exists.
var NoCompaction = Compaction{
	Target: NoRoundID,
	Tail:   NoRoundID,
}

// Compaction is an additive summary row: Target is the round the summary
// attaches to on replay (the first kept round), and Tail is the last
// completed round when the compaction was made. ThreadCut at or before
// Tail removes the compaction, so a rewind there replays the originals.
//
// Until a round runs on the compacted context, its reported usage is an
// estimate, a lower bound.
type Compaction struct {
	Text   string  `json:"text"`   // the summary that replaces rounds before Target
	Target RoundID `json:"target"` // first kept round
	Tail   RoundID `json:"tail"`   // last completed round when it was made
	// Output is the output token count of the compaction response — the
	// summary's own size.
	Output int `json:"output"`
	// Base is the context total of the round replayed just before Target,
	// what the summarized part of the conversation cost.
	Base int `json:"base"`
}

// IsValid reports whether the compaction is set: both Target and Tail
// differ from NoRoundID.
func (c Compaction) IsValid() bool {
	return c.Target != NoRoundID && c.Tail != NoRoundID
}

// NoRoundID is the none/open sentinel for RoundID parameters.
const NoRoundID = RoundID(-1)

// MessageStorage stores one user's threads and rounds; round ids are
// thread-scoped and monotonically increasing. Writes are serialized per
// thread via Seq: each write takes seq as an optimistic concurrency guard,
// applies only if the thread is currently at that Seq, bumps it, and
// returns the new Seq; a stale seq fails the write with ErrDrift. A missing
// thread or round is ErrNotFound. NoRoundID (-1) is the open/none sentinel
// for RoundID parameters.
type MessageStorage interface {
	// ThreadList returns threads updated in [from, to), at most limit of
	// them. A zero from or to drops that bound; limit <= 0 means no cap. A
	// non-empty name keeps only threads whose name contains it,
	// case-insensitively, and limit counts what is left. They come most
	// recently updated first, ties broken by id descending, unless asc
	// reverses both, and limit keeps the first limit threads in that order.
	ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]Thread, error)
	// ThreadBump fetches the thread and bumps its Seq, taking the write
	// serial without writing any content. Updated stays as it was, so a
	// bump on its own does not reorder the thread in ThreadList.
	ThreadBump(ctx context.Context, thread ThreadID) (Thread, error)
	// ThreadGet returns the thread; an unknown id is ErrNotFound.
	ThreadGet(ctx context.Context, thread ThreadID) (Thread, error)
	// ThreadCreate creates an empty thread with the caller-supplied id; an id
	// already in use is ErrExists.
	ThreadCreate(ctx context.Context, id ThreadID) (Thread, error)
	// ThreadCut removes all rounds starting with round, and all compactions
	// whose Tail references a removed round (Tail >= round) — a rewind into a
	// compaction's verbatim region thereby replays originals. NoRoundID means
	// everything. Records with Origin >= round are removed; records with
	// Pin >= round are unpinned and surface again in the current request.
	ThreadCut(ctx context.Context, id ThreadID, seq Seq, round RoundID) (Seq, error)
	// ThreadRename sets the thread's name, replacing whatever it was named
	// before; an empty name leaves it unnamed again. Updated stays as it was,
	// so a rename does not reorder the thread in ThreadList.
	ThreadRename(ctx context.Context, id ThreadID, seq Seq, name string) (Seq, error)
	// ThreadClone creates the target thread as a copy of source with its name,
	// rounds, compactions and records; files and workspace are not copied. A
	// missing source is ErrNotFound and a target already in use ErrExists.
	ThreadClone(ctx context.Context, source ThreadID, target ThreadID) (Thread, error)
	// ThreadDelete removes the thread with its rounds, compactions and records;
	// its files and workspace are left to UploadStorage and WorkspaceStorage. It
	// takes no seq, and a thread that is not there is not an error.
	ThreadDelete(ctx context.Context, id ThreadID) error

	// CompactionCreate stores the compaction record: its Target is the round
	// the summary attaches to on replay (the first kept round), its Tail the
	// last round completed when the compaction was applied, and Output with
	// Base size the context it leaves behind.
	CompactionCreate(ctx context.Context, thread ThreadID, seq Seq, compaction Compaction) (Seq, error)
	// CompactionGet returns the latest compaction whose Tail < beforeTail;
	// a NoRoundID beforeTail means simply the latest. Returns NoCompaction
	// and a nil error when none exists.
	CompactionGet(ctx context.Context, thread ThreadID, beforeTail RoundID) (Compaction, error)

	// RoundRange returns rounds from start to end one by one; a NoRoundID
	// start means from the beginning, a NoRoundID end means to the head. A
	// non-nil second value ends the iteration.
	RoundRange(ctx context.Context, thread ThreadID, start RoundID, end RoundID) iter.Seq2[Round, error]
	// RoundBefore returns at most limit rounds before start (excluding it);
	// limit <= 0 returns none. They come in ascending id order; a NoRoundID
	// start means the last rounds, including the head. These rounds feed a chat, which never renders
	// tool results or record bodies, so Results comes back empty and
	// PinnedRecords come back without Data — read a round through
	// RoundRange when they are needed.
	RoundBefore(ctx context.Context, thread ThreadID, start RoundID, limit int) ([]Round, error)
	// RoundAfter returns at most limit rounds after start (excluding it);
	// limit <= 0 returns none. They come in ascending id order; a NoRoundID
	// start means the first rounds, including the oldest. Results and record Data come back empty, as in
	// RoundBefore.
	RoundAfter(ctx context.Context, thread ThreadID, start RoundID, limit int) ([]Round, error)
	// RoundSearch returns rounds whose input text or response text contains
	// query, case-insensitively — one Hit per round, at the first match,
	// input before response. Reasoning, tool calls, tool results,
	// attachments and errors are not searched. A zero thread searches every
	// thread; a named thread restricts the search to it, or with exclude
	// set, to every thread but it. An empty query matches nothing. Hits are
	// ordered by (created, thread, round), ascending when asc and descending
	// when not; a nil from starts at the end the direction runs from and
	// inclusive says nothing there, otherwise the walk starts at from, taking
	// the round it names when inclusive and starting past it when not. limit
	// caps the result; limit <= 0 means no cap. Only the chat calls this,
	// and an implementation that does not support search may return no
	// hits.
	RoundSearch(ctx context.Context, thread ThreadID, exclude bool, query string, from *Cursor, inclusive bool, asc bool, limit int) ([]Hit, error)
	// RoundInput creates a round in the input state with the user's input,
	// returns its id.
	RoundInput(ctx context.Context, thread ThreadID, seq Seq, input Input) (RoundID, Seq, error)
	// RoundOk marks the round completed with the model's usage and response
	// content.
	RoundOk(ctx context.Context, thread ThreadID, seq Seq, round RoundID, usage EventUsage, content Indexed[Content]) (Seq, error)
	// RoundRefuse marks the round refused, keeping whatever usage and
	// partial response content arrived before the refusal.
	RoundRefuse(ctx context.Context, thread ThreadID, seq Seq, round RoundID, usage EventUsage, content Indexed[Content]) (Seq, error)
	// RoundError marks the round errored and stores the error text, keeping
	// whatever usage and partial response content arrived before the error.
	RoundError(ctx context.Context, thread ThreadID, seq Seq, round RoundID, err string, usage EventUsage, content Indexed[Content]) (Seq, error)
	// RoundResult sets the round's tool results, keyed by tool call id,
	// replacing any stored before; pass every result in one call.
	RoundResult(ctx context.Context, thread ThreadID, seq Seq, round RoundID, result map[string]ContentToolResult) (Seq, error)
	// RoundAbort marks the round aborted, keeping whatever usage and partial
	// response content arrived before the abort.
	RoundAbort(ctx context.Context, thread ThreadID, seq Seq, round RoundID, usage EventUsage, content Indexed[Content]) (Seq, error)

	// RecordAdd stores a record, unpinned; origin is the round whose action
	// produced it. Returns the new record's id.
	RecordAdd(ctx context.Context, thread ThreadID, seq Seq, origin RoundID, kind string, attrs map[string]any, data any) (RecordID, Seq, error)
	// RecordPin pins the records to the round whose user message renders
	// them on replay. An unknown record id is ErrNotFound and pins nothing.
	RecordPin(ctx context.Context, thread ThreadID, seq Seq, pin RoundID, records []RecordID) (Seq, error)
	// RecordList returns the unpinned records in id order.
	RecordList(ctx context.Context, thread ThreadID) ([]Record, error)
}
