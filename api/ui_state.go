package api

import (
	"context"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

// ChatInput is the thread's staged input as a chat shows it.
type ChatInput struct {
	Input
	// Pending reports that the input has been submitted and waits for a round
	// to consume it; Unsend clears it again.
	Pending bool
}

// ChatAsk is a question a tool puts to the user, returned by an Ask handler
// (neui.Module.Ask) and shown by the chat in place of its composer until the
// handler answers. nechat.AskOptions is a ready-made one.
type ChatAsk struct {
	// Form is the question and its inputs, shown where the message box
	// normally is.
	Form gox.Elem
	// Button is the submit control, shown where the send button normally is;
	// its click handler calls Answer.Submit.
	Button gox.Elem
	// Cancel dismisses the ask and fails the tool call. The harness sets it,
	// overwriting whatever the handler returned, so leave it nil; a chat calls
	// it from its own dismiss control.
	Cancel func(context.Context)
	// Error is what the ask has to say about the answer as it stands, shown
	// next to the question; nil or an empty string shows nothing. It is a
	// readout, not a gate: whether the answer can be submitted stays the
	// ask's own business.
	Error doors.Beam[string]
}

// ChatState is the snapshot ChatFactory.Chat renders a thread from. Every later
// change arrives as a ChatEvent.
type ChatState struct {
	// ID is the thread's id.
	ID ThreadID
	// Name is the chat title set by the agent; empty until named.
	Name string
	// Input is the staged draft and whether it is pending.
	Input ChatInput
	// Round is the round in progress, with its response or tool results so
	// far; nil when the thread is idle. Earlier rounds are read with
	// ChatThread.RoundBefore.
	Round *Round
	// Ask is the question showing to the user; nil when there is none.
	Ask *ChatAsk
	// Tasks is the background tasks still running.
	Tasks []Task
	// Usage is the token usage of the thread's context as it stands, what
	// compaction thresholds are measured against.
	Usage EventUsage
	// Compaction reports a compaction of the history running in the background.
	Compaction bool
	// CanCompact reports whether ChatThread.Compact would be accepted now: no
	// compaction is running, at least two rounds have completed since the last
	// compaction, and Usage has reached Config().CompactionAllowed times
	// Model().Context (not checked when Model().Context is 0). Later changes
	// arrive on ChatEventRoundStart, ChatEventRoundEnd and ChatEventCompacting.
	CanCompact bool
	// Meta is the meta that the Instance.ThreadOpen which switched to this
	// thread passed, nil for every other way a chat comes up. It is never
	// interpreted here: it reaches only the first chat initialised for that
	// open, so a later reload of the same thread sees nil and it does not
	// survive a page reload.
	Meta any
}

// Task is a background task a tool started with neui.ToolTask, as a chat
// lists it.
type Task struct {
	// ID is stable for the task's lifetime and unique within the thread link.
	ID uint
	// Module is the name of the module whose tool started the task.
	Module string
	// Name is the name of that tool.
	Name string
	// Comment is the short description the tool gave when it started the
	// task; empty when it gave none.
	Comment string
	// Meta is the attrs the tool passed to ToolTask, never nil.
	Meta map[string]any
	// Cancel stops the task; the cause it is given is what the task reports.
	Cancel context.CancelCauseFunc
}
