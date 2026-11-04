package api

import (
	"context"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

// InstanceStatus is where the instance stands: InstanceDisabled with no
// backend enabled, InstanceLoading once there is one but the tab's thread id
// has yet to sync, InstanceEnabled with a thread to render.
type InstanceStatus int

const (
	InstanceDisabled InstanceStatus = iota // no backend
	InstanceLoading                        // tab state not in yet
	InstanceEnabled                        // linked to a thread
)

// InstanceThread is the thread the instance has open.
type InstanceThread struct {
	ID ThreadID
	// Name is the chat title set by the agent; empty until named.
	Name string
	// New is true until the thread's first message creates it in storage, so
	// it is not in Instance.ThreadList yet.
	New bool
	// Updated is when this tab last started a round on the thread, which is
	// also when Instance.ThreadList last reordered it to the front. It is the
	// zero time until the tab starts one, so a thread opened and left alone
	// carries nothing newer than what ThreadList already reports.
	Updated time.Time
}

// InstanceState is the instance's view of itself. Thread is the zero value
// unless Status is InstanceEnabled.
type InstanceState struct {
	Status InstanceStatus
	Thread InstanceThread
}

// InstanceChat is one open chat in the session: InstanceID is the doors
// instance id, the tab the chat belongs to.
type InstanceChat struct {
	InstanceID string
	State      InstanceState
}

type Chat interface {
	Update(ctx context.Context, e ChatEvent)
	gox.Comp
}

type ChatFactory interface {
	Chat(thread ChatThread, state ChatState) Chat
	// Placeholder stands in whenever there is no thread to render. A true
	// enabled means a backend is up and a thread is on its way, so the
	// placeholder carries the loading state too; false means no backend.
	Placeholder(enabled doors.Beam[bool]) gox.Elem
}

type Instance interface {
	// ID is the instance's immutable id, the one its own entry in Chats
	// carries.
	ID() string
	// State is this instance's own chat.
	State() doors.Beam[InstanceState]
	// Chats is every chat open in the session, keyed by thread. A tab with no
	// thread yet has no key, so it is absent; two tabs on one thread collapse
	// into a single entry, and which of them it carries is unspecified.
	Chats() doors.Beam[map[ThreadID]InstanceChat]
	// Location is the session's time zone, a fixed UTC offset read once from
	// the client clock while the first instance syncs. It is the server's own
	// zone until that reading lands, which is before Status leaves
	// InstanceLoading.
	Location() *time.Location
	// ThreadOpen switches the instance to the thread, replacing whatever it
	// had open. meta reaches the chat that initialises for this open as
	// ChatState.Meta and is consumed there; pass nil when there is nothing
	// to hand over.
	ThreadOpen(ctx context.Context, id ThreadID, meta any) error
	ThreadNew(ctx context.Context) error
	// ThreadRename sets the thread's name, replacing whatever it was named
	// before; an empty name leaves it unnamed again. The instance stays on the
	// thread it has open, whichever thread was renamed, and a chat that has
	// the renamed thread open sees the new name at once.
	ThreadRename(ctx context.Context, id ThreadID, name string) error
	// ThreadClone copies the thread with all of its history into a new one and
	// returns the new chat's id; the copy is listed and ready to ThreadOpen
	// right away, and the instance stays on what it had open.
	ThreadClone(ctx context.Context, id ThreadID) (ThreadID, error)
	// ThreadDelete removes the thread and everything in it; a thread that is
	// not there is not an error. An instance holding the deleted thread loses
	// it: this one moves to a fresh chat the way ThreadNew does, while any
	// other is dropped with a critical error and lands on a fresh chat of its
	// own.
	ThreadDelete(ctx context.Context, id ThreadID) error
	ThreadGet(ctx context.Context, id ThreadID) (Thread, error)
	// ThreadList returns threads updated in [from, to), at most limit of them.
	// A zero from or to drops that bound; limit <= 0 means no cap. A non-empty
	// name keeps only threads whose name contains it, case-insensitively, and
	// limit counts what is left. They come most recently updated first, ties
	// broken by id descending, unless asc reverses both — so limit always cuts
	// the far end of the range from the end the caller is reading toward.
	ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]Thread, error)
	// RoundSearch returns rounds whose input text or response text contains
	// query, case-insensitively, at most limit of them. Reasoning, tool calls,
	// tool results, attachments and errors are not searched. A zero thread
	// searches every chat; a named thread restricts the search to it, or with
	// exclude set, to every chat but it. An empty query matches nothing. Hits
	// are ordered by (created, thread, round), ascending when asc and
	// descending when not; a nil from starts at the end the direction runs
	// from and inclusive says nothing there, otherwise the walk starts at
	// from, taking the round it names when inclusive and starting past it when
	// not. limit caps the result and 0 means no cap.
	RoundSearch(ctx context.Context, thread ThreadID, exclude bool, query string, from *Cursor, inclusive bool, asc bool, limit int) ([]Hit, error)
	// Emit sends an app event to the thread this instance has open. The agent
	// reads it as an <app_event name="…"> element carrying attrs, with data as
	// its body: a string is escaped text, json.RawMessage is passed as is,
	// anything else is encoded as JSON. An idle agent starts a turn at once; a
	// busy one reads it in its next turn. Returns ErrDisabled when neui is off
	// for the session, and ErrUnlinked when the instance has no thread open or
	// switched threads before the event landed.
	Emit(ctx context.Context, name string, attrs map[string]string, data any) error
	Render(factory ChatFactory) gox.Elem
}

// ChatThread is the live handle a Chat drives the thread through: the commands
// a user can issue and the reads a chat renders from. Once the thread has
// dropped this instance's link, every method that returns an error returns
// ErrUnlinked.
type ChatThread interface {
	Input(ctx context.Context, input Input) error
	Send(ctx context.Context, input Input) error
	Stop(ctx context.Context) error
	Unsend(ctx context.Context) error
	Retry(ctx context.Context) error
	// Compact starts a compaction of the history in the background;
	// ErrWrongState when one is already running, the context is below
	// Config().CompactionAllowed or there is nothing to compact yet.
	Compact(ctx context.Context) error
	Rewind(ctx context.Context, round RoundID, input Input) error
	Reload(ctx context.Context)
	RoundBefore(ctx context.Context, start RoundID, limit int) ([]Round, error)
	RoundAfter(ctx context.Context, start RoundID, limit int) ([]Round, error)
	// Model is the model the linked backend streams with; the zero value when
	// unlinked.
	Model() Model
	// Config is the linked backend's tunables; the zero value when unlinked.
	Config() ThreadConfig
	FS() FS
}
