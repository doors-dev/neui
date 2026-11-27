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

// ChatEvent is one update to a chat's view. It is one of the ChatEvent* types
// in this package and nothing else.
type ChatEvent interface {
	chatEvent()
}

// Final reports whether the event ends the current activity: true on a
// ChatEventRoundEnd, ChatEventToolResults or ChatEventCompacting with Final
// set, and on every ChatEventCriticalError. A chat uses it to switch its
// controls back to idle.
func Final(e ChatEvent) bool {
	switch v := e.(type) {
	case ChatEventRoundEnd:
		return v.Final
	case ChatEventToolResults:
		return v.Final
	case ChatEventCompacting:
		return v.Final
	case ChatEventCriticalError:
		return true
	}
	return false
}

// ChatEventIndexed carries one block-naming event of a streaming round.
type ChatEventIndexed struct {
	// Event is the provider event for one block of the streaming round's
	// response; apply it to Round.Response at Event.At().
	Event IndexedEvent
}

func (ChatEventIndexed) chatEvent() {}

// ChatEventAsk reports a question for the user. Show it until
// ChatEventAskDone; the next waiting question comes in a new ChatEventAsk
// after it.
type ChatEventAsk struct {
	// Ask is what to show: its Form, Button and Error.
	Ask ChatAskView
}

func (ChatEventAsk) chatEvent() {}

// ChatEventAskDone reports the pending ask resolved.
type ChatEventAskDone struct{}

func (ChatEventAskDone) chatEvent() {}

// ChatEventRoundStart reports a round entering the streaming state.
type ChatEventRoundStart struct {
	// Round is the new round with its input and no response yet.
	Round Round
	// Truncate is NoRoundID for an ordinary round. After Rewind or Retry, or
	// when the thread sends a round again on its own (after a compaction or
	// an empty response), it names the round the new one replaces: drop it
	// and every round after it from the view.
	Truncate RoundID
	// InputTaken is true when the round took the user's input: the input sent
	// or queued, or, with Truncate set, the input a Rewind carried. A view
	// clears what was taken from its composer. It is false after Retry, after
	// a Rewind with empty input, and when the thread sends a round again on
	// its own.
	InputTaken bool
	// Usage is ChatState.Usage after this event: after a Rewind, the usage of
	// the history that remains; otherwise the value already reported.
	Usage EventUsage
	// CanCompact is ChatState.CanCompact after this event.
	CanCompact bool
}

func (ChatEventRoundStart) chatEvent() {}

// ChatEventRoundEnd reports a round leaving the streaming state. Final is true
// when no agent-driven continuation follows; a round started by pending user
// input is a new activity, so it does not clear final.
type ChatEventRoundEnd struct {
	// Round is the round that ended.
	Round RoundID
	// State is how it ended: RoundOk, RoundAbort, RoundRefuse or RoundError.
	State RoundState
	// Error is the ErrCode as a string when State is RoundError, empty
	// otherwise.
	Error string
	// Usage is what the round's last request spent, the usage stored with
	// the round.
	Usage EventUsage
	// Final is true when the activity ends here; see Final.
	Final bool
	// CanCompact is ChatState.CanCompact after this event.
	CanCompact bool
}

func (ChatEventRoundEnd) chatEvent() {}

// ChatEventRoundUsage reports the usage a streaming round has accumulated so
// far, including the requests the thread sent again after empty responses;
// every event carries the running total, not an increment.
type ChatEventRoundUsage struct {
	// Round is the streaming round.
	Round RoundID
	// Usage is the round's running total so far.
	Usage EventUsage
}

func (ChatEventRoundUsage) chatEvent() {}

// ChatEventToolResults delivers the results of a tools round once every tool
// call in it has finished.
type ChatEventToolResults struct {
	// Round is the round whose tool calls these answer.
	Round RoundID
	// Results holds one result per tool call, keyed by tool call id.
	Results map[string]ContentToolResult
	// Final is true when no continuation round follows, as after Stop.
	Final bool
}

func (ChatEventToolResults) chatEvent() {}

// ChatEventInputPending reports whether input is queued for the running
// activity.
type ChatEventInputPending struct {
	// Pending is true once Send queued input behind the running activity
	// and false once it was consumed, unsent or dropped.
	Pending bool
}

func (ChatEventInputPending) chatEvent() {}

// ChatEventCompacting reports a background compaction starting or ending. On
// the end of an applied one usage estimates the compacted context — the
// summary plus the rounds kept verbatim — and is zero otherwise.
type ChatEventCompacting struct {
	// Running is true while the compaction runs.
	Running bool
	// Usage is the estimated usage of the compacted context on the end of an
	// applied compaction, zero otherwise.
	Usage EventUsage
	// Final is true when the activity ends here: Stop canceled a compaction
	// the turn waited on after its round end or tool results were already
	// reported, so no other Final event follows; see Final.
	Final bool
	// CanCompact is ChatState.CanCompact after this event; always false while
	// Running.
	CanCompact bool
}

func (ChatEventCompacting) chatEvent() {}

// ChatEventThreadCreated reports that the thread's first round or first app
// event created it in storage. It fires once per thread, before the start
// event of the round that follows.
type ChatEventThreadCreated struct{}

func (ChatEventThreadCreated) chatEvent() {}

// ChatEventName reports the thread's name changing.
type ChatEventName struct {
	// Name is the new name; empty leaves the thread unnamed.
	Name string
}

func (ChatEventName) chatEvent() {}

// ChatEventTasks reports the thread's task list changing.
type ChatEventTasks struct {
	// Tasks is the full current list, replacing the previous one.
	Tasks []Task
}

func (ChatEventTasks) chatEvent() {}

// ChatEventCriticalError reports the thread failing beyond recovery. The link
// is dropped with it, so this Chat receives no further events and every
// ChatThread call returns ErrUnlinked afterwards. The instance relinks to the
// thread, or to a fresh thread when that fails; a new Chat is built the next
// time the element renders, for example after ChatThread.Reload.
type ChatEventCriticalError struct {
	// Err is the cause: a storage failure, ErrNotFound when the thread was
	// deleted through another instance's ThreadDelete, or ErrCompactionFailed
	// when compaction cannot make the conversation fit the model's context
	// window.
	Err error
}

func (ChatEventCriticalError) chatEvent() {}
