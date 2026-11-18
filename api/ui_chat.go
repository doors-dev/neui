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
	"context"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

// InstanceStatus is where the instance stands: InstanceDisabled while neui is
// not enabled for the session, InstanceLoading once it is but the tab's
// thread id has yet to sync, InstanceEnabled with a thread to render.
type InstanceStatus int

const (
	InstanceDisabled InstanceStatus = iota // session not enabled
	InstanceLoading                        // tab state not in yet
	InstanceEnabled                        // linked to a thread
)

// InstanceThread is the thread the instance has open.
type InstanceThread struct {
	// ID is the thread's id, valid for ThreadOpen, ThreadGet and the other
	// Thread* calls once the thread exists in storage.
	ID ThreadID
	// Name is the chat title set by the agent; empty until named.
	Name string
	// New is true until the thread's first round or first app event
	// (Instance.Emit) creates it in storage; until then Instance.ThreadList
	// does not list it.
	New bool
	// Updated is when this instance last saw a round start on the thread, by
	// its own clock; the zero time until one starts after the thread was
	// opened. It is not the stored Thread.Updated that Instance.ThreadList
	// orders by.
	Updated time.Time
}

// InstanceState is the instance's view of itself. Thread is the zero value
// unless Status is InstanceEnabled.
type InstanceState struct {
	// Status is whether the session is enabled and the instance has a thread
	// yet.
	Status InstanceStatus
	// Thread is the thread the instance has open.
	Thread InstanceThread
}

// InstanceChat is one open chat in the session: InstanceID is the doors
// instance id, the tab the chat belongs to.
type InstanceChat struct {
	// InstanceID is the owning instance's Instance.ID.
	InstanceID string
	// State is that instance's current InstanceState.
	State InstanceState
}

// Chat is one rendered chat view, built by a ChatFactory for one thread link.
// The instance renders it as a component and then feeds it every change to the
// thread through Update. Implement it only to replace nekit.Chat with a chat UI of
// your own.
type Chat interface {
	// Update applies one event to the view. Events arrive one at a time, in
	// order, inside a Doors batch on ctx, so the view updates the page
	// directly from it.
	Update(ctx context.Context, e ChatEvent)
	gox.Comp
}

// ChatFactory builds the views Instance.Render shows. nekit.Chat is the
// ready-made implementation.
type ChatFactory interface {
	// Chat builds the view for the thread the instance has linked. It is
	// called each time the element from Instance.Render renders while a
	// thread is linked: on first render, whenever the instance switches
	// threads (ThreadOpen, ThreadNew, a ThreadDelete of its own thread,
	// Enable) and on ChatThread.Reload. A critical error alone builds no new
	// Chat. state is the full snapshot to render from; every later change
	// arrives through Chat.Update. thread is valid until the link drops, after
	// which its methods return ErrUnlinked.
	Chat(thread ChatThread, state ChatState) Chat
	// Placeholder stands in whenever there is no thread to render. A true
	// enabled means the session is enabled and a thread is on its way, so
	// the placeholder carries the loading state too; false means it is not
	// enabled.
	Placeholder(enabled doors.Beam[bool]) gox.Elem
}

// Instance is the neui side of one Doors page instance, the browser tab:
// which thread it has open, the session's other chats, thread storage, and the
// element that renders the chat. Get it with neui.Instance(ctx) from any ctx of
// that page. Every method that returns an error returns ErrDisabled while neui
// is not enabled for the session and context.Canceled once ctx is done.
// ThreadOpen, ThreadNew, ThreadDelete and Emit return ErrUnlinked until the
// instance has synced with its tab, shortly after the page loads.
type Instance interface {
	// ID is the instance's immutable id, the one its own entry in Chats
	// carries.
	ID() string
	// State is this instance's own chat.
	State() doors.Beam[InstanceState]
	// Chats is every chat open in the session, keyed by thread. A tab with no
	// thread yet has no key, so it is absent. A thread is open in at most one
	// tab: when a tab opens a thread another tab has open, that other tab
	// moves to a fresh thread, so each entry is the one tab that has it.
	Chats() doors.Beam[map[ThreadID]InstanceChat]
	// Location is the session's time zone, a fixed UTC offset read from the
	// browser's clock and kept for the session once one reading succeeds.
	// Until then it is time.Local, the server's zone. When an instance's own
	// reading succeeds, it is stored before that instance's Status leaves
	// InstanceLoading.
	Location() *time.Location
	// ThreadOpen switches the instance to the thread, replacing whatever it
	// had open. A thread that is not in storage, a New one included, returns
	// the storage's ErrNotFound. meta reaches the chat that initialises for
	// this open as ChatState.Meta and is consumed there; pass nil when there
	// is nothing to hand over.
	ThreadOpen(ctx context.Context, id ThreadID, meta any) error
	// ThreadNew switches the instance to a fresh, empty thread, replacing
	// whatever it had open. The thread is not stored until its first round or
	// app event, so until then InstanceThread.New is true and ThreadList does
	// not list it.
	ThreadNew(ctx context.Context) error
	// ThreadRename sets the thread's name, replacing whatever it was named
	// before. The name is trimmed; one that is then empty leaves the thread
	// unnamed, and one that holds line breaks, control or format characters is
	// refused. A name over 80 runes is cut to its first 80 and trimmed again.
	// A thread that is not in storage, a New one included, returns
	// ErrNotFound. The instance stays on
	// the thread it has open, whichever thread was renamed, and a chat that
	// has the renamed thread open sees the new name at once.
	ThreadRename(ctx context.Context, id ThreadID, name string) error
	// ThreadClone copies the thread, with its name, all of its stored history
	// and its committed files of both origins, into a new one and returns the
	// new thread's id. A source that is not in storage returns the storage's
	// ErrNotFound; when copying the files fails, the clone is deleted and the
	// error returned. The copy is listed
	// and ready to ThreadOpen right away, and the instance stays on what it
	// had open.
	ThreadClone(ctx context.Context, id ThreadID) (ThreadID, error)
	// ThreadDelete removes the thread and everything in it; a thread that is
	// not there is not an error. An instance holding the deleted thread loses
	// it: this one moves to a fresh chat the way ThreadNew does, while any
	// other is dropped with a critical error and lands on a fresh chat of its
	// own.
	ThreadDelete(ctx context.Context, id ThreadID) error
	// ThreadGet reads the thread's stored metadata: name, dates and write
	// serial. A thread that is not in storage returns the storage's
	// ErrNotFound.
	ThreadGet(ctx context.Context, id ThreadID) (Thread, error)
	// ThreadList returns threads updated in [from, to), at most limit of them.
	// A zero from or to drops that bound; limit <= 0 means no cap. A non-empty
	// name keeps only threads whose name contains it, case-insensitively.
	// They come most recently updated first, ties broken by id descending, or
	// the reverse when asc is set, and limit keeps the first limit in that
	// order.
	ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]Thread, error)
	// RoundSearch returns rounds whose input text or response text contains
	// query, case-insensitively, at most limit of them; limit <= 0 means no
	// cap. Reasoning, tool calls, tool results, attachments and errors are not
	// searched, and an empty query matches nothing. A zero thread searches
	// every chat; a non-zero thread restricts the search to it, or, with
	// exclude set, to every chat but it. Hits are ordered by (created, thread,
	// round), ascending when asc and descending otherwise. A nil from starts
	// at the first hit in that order and inclusive is ignored; otherwise the
	// result starts at the round from names when inclusive, or just past it
	// when not.
	RoundSearch(ctx context.Context, thread ThreadID, exclude bool, query string, from *Cursor, inclusive bool, asc bool, limit int) ([]Hit, error)
	// Emit sends an app event to the thread this instance has open. The agent
	// reads it as an <app_event name="…"> element carrying attrs, with data as
	// its body: a string is escaped text, json.RawMessage is passed as is,
	// anything else is encoded as JSON. The event is stored in the thread,
	// creating a New thread in storage. An idle agent starts a turn at once; a
	// busy one reads it in its next turn, or a later one while the records
	// ahead of it fill a turn's ThreadConfig.ReplayItemShare budget. Returns
	// ErrUnlinked when the instance has no thread open or switched threads
	// before the event landed; a storage failure is returned and also ends the
	// link with ChatEventCriticalError.
	Emit(ctx context.Context, name string, attrs map[string]string, data any) error
	// Render returns the element that shows this instance's chat. It renders
	// factory.Placeholder while there is no thread to show and factory.Chat
	// once there is, and swaps the view in place whenever the instance
	// switches threads. Render it once per page; nekit.Chat does this for
	// you.
	Render(factory ChatFactory) gox.Elem
}

// ChatThread is the live handle a Chat drives the thread through: the commands
// a user can issue and the reads a chat renders from. Once the thread has
// dropped this instance's link, every method that returns an error returns
// ErrUnlinked.
//
// Call the commands with the ctx of the Doors event that triggered them, such
// as a click handler's: each blocks until the thread accepts or refuses it,
// and the ChatEvent it causes reaches Chat.Update before that event settles. A
// refused command changes nothing, except that a Send refused for empty input
// still replaces the staged draft with it.
type ChatThread interface {
	// Input stages input as the thread's draft without sending it, so a
	// reload or another tab on the thread picks it up as ChatState.Input.
	// ErrWrongState while an input is pending; ErrInputTooLarge when its text
	// is over the ThreadConfig.ReplayItemShare budget.
	Input(ctx context.Context, input Input) error
	// Send submits input. An idle thread starts a round with it at once
	// (ChatEventRoundStart); a busy one queues it as pending
	// (ChatEventInputPending) and runs it after the current activity.
	// ErrWrongState when an input is already pending or input is empty (an
	// idle thread with unanswered tool results accepts an empty Send and
	// resumes); ErrInputTooLarge when its text is over the
	// ThreadConfig.ReplayItemShare budget.
	Send(ctx context.Context, input Input) error
	// Stop aborts the running round, cancels the running tool calls with
	// ErrCanceledByUser, or cancels a compaction the turn waits on (results
	// already stashed stay for the next round), and drops any pending input;
	// the activity ends with a Final event. ErrWrongState when nothing is
	// running.
	Stop(ctx context.Context) error
	// Unsend withdraws the pending input back into the draft.
	// ErrWrongState when nothing is pending.
	Unsend(ctx context.Context) error
	// Retry runs the last round again when the thread is idle and that round
	// ended without an answer (error, abort, or input only); the new round's
	// ChatEventRoundStart names the replaced one in Truncate. ErrWrongState
	// otherwise, including after an ok or refused round.
	Retry(ctx context.Context) error
	// Compact starts a compaction of the history in the background. It
	// returns ErrWrongState when ChatState.CanCompact, as last reported, is
	// false; any other failure to start it is returned as is.
	Compact(ctx context.Context) error
	// Rewind deletes round and every round after it, then starts a new round
	// with input in its place; it works in any state and stops whatever is
	// running first. The ChatEventRoundStart it causes carries round in
	// Truncate. ErrInputTooLarge when the text is over the
	// ThreadConfig.ReplayItemShare budget. An empty input resumes from the
	// cut, and returns ErrWrongState with nothing changed when the cut would
	// leave nothing for the model to answer: no tool calls from the last
	// successful round before it and no records. A storage failure arrives as
	// ChatEventCriticalError instead of an error.
	Rewind(ctx context.Context, round RoundID, input Input) error
	// Reload re-renders the chat from scratch: the ChatFactory builds a new
	// Chat from a fresh ChatState and the current view is dropped.
	Reload(ctx context.Context)
	// RoundBefore returns at most limit stored rounds before start, oldest
	// first; NoRoundID returns the latest ones. Use it to page history in
	// as the user scrolls up. Results and record Data come back empty, as
	// MessageStorage.RoundBefore documents.
	RoundBefore(ctx context.Context, start RoundID, limit int) ([]Round, error)
	// RoundAfter returns at most limit stored rounds after start, oldest
	// first; NoRoundID returns the first ones. Results and record Data come
	// back empty.
	RoundAfter(ctx context.Context, start RoundID, limit int) ([]Round, error)
	// Model is the provider's model of the Enable configuration this handle
	// was linked with; it keeps returning it after the link drops.
	Model() Model
	// Config is the thread configuration of the Enable configuration this
	// handle was linked with; it keeps returning it after the link drops.
	Config() ThreadConfig
	// FS is the thread's file system, the workspace plus the uploads dirs, as
	// the agent's tools see it; use it to list or attach files from the chat.
	FS() FS
}
