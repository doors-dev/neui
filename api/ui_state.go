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

type ChatAsk struct {
	Form   gox.Elem
	Button gox.Elem
	Cancel func(context.Context)
	// Error is what the ask has to say about the answer as it stands, shown
	// next to the question; nil or an empty string shows nothing. It is a
	// readout, not a gate: whether the answer can be submitted stays the
	// ask's own business.
	Error doors.Beam[string]
}

type ChatState struct {
	ID ThreadID
	// Name is the chat title set by the agent; empty until named.
	Name  string
	Input ChatInput
	Round *Round
	Ask   *ChatAsk
	Tasks []Task
	Usage EventUsage
	// Compaction reports a compaction of the history running in the background.
	Compaction bool
	// Meta is the value the Open that switched to this thread passed along,
	// nil for every other way a chat comes up. It is never interpreted here:
	// it travels from the caller of Open to the first chat initialised for
	// that open and is consumed there, so a later reload of the same thread
	// sees nil and it does not survive a page reload.
	Meta any
}

type Task struct {
	// ID is stable for the task's lifetime and unique within the thread link.
	ID     uint
	Module string
	Name   string
	// Comment is the short description the tool gave when it started the
	// task; empty when it gave none.
	Comment string
	Meta    map[string]any
	// Cancel stops the task; the cause it is given is what the task reports.
	Cancel context.CancelCauseFunc
}
