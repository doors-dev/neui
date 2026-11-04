package api

// ChatEvent is one update to a chat's view. It is one of the ChatEvent* types
// in this package and nothing else.
type ChatEvent interface {
	chatEvent()
}

// Final reports whether the event ends the current activity: true on a
// RoundEnd or ToolResults not followed by an agent-driven continuation round,
// and on CriticalError.
func Final(e ChatEvent) bool {
	switch v := e.(type) {
	case ChatEventRoundEnd:
		return v.Final
	case ChatEventToolResults:
		return v.Final
	case ChatEventCriticalError:
		return true
	}
	return false
}

// ChatEventIndexed carries one block-naming event of a streaming round.
type ChatEventIndexed struct {
	Event IndexedEvent
}

func (ChatEventIndexed) chatEvent() {}

// ChatEventAsk reports a tool asking the user for a decision.
type ChatEventAsk struct {
	Ask ChatAsk
}

func (ChatEventAsk) chatEvent() {}

// ChatEventAskDone reports the pending ask resolved.
type ChatEventAskDone struct{}

func (ChatEventAskDone) chatEvent() {}

// ChatEventRoundStart reports a round entering the streaming state. Truncate
// names the round the new one replaces, or NoRoundID.
type ChatEventRoundStart struct {
	Round    Round
	Truncate RoundID
}

func (ChatEventRoundStart) chatEvent() {}

// ChatEventRoundEnd reports a round leaving the streaming state with the usage
// the round spent in total. Final is true when no agent-driven continuation
// follows; a round started by pending user input is a new activity, so it does
// not clear final.
type ChatEventRoundEnd struct {
	Round RoundID
	State RoundState
	Error string
	Usage EventUsage
	Final bool
}

func (ChatEventRoundEnd) chatEvent() {}

// ChatEventRoundUsage reports the usage a streaming round has accumulated so
// far; every event carries the running total, not an increment.
type ChatEventRoundUsage struct {
	Round RoundID
	Usage EventUsage
}

func (ChatEventRoundUsage) chatEvent() {}

// ChatEventToolResults delivers the results of a tools round. Final is true when
// no continuation round follows.
type ChatEventToolResults struct {
	Round   RoundID
	Results map[string]ContentToolResult
	Final   bool
}

func (ChatEventToolResults) chatEvent() {}

// ChatEventInputPending reports whether input is queued for the running
// activity.
type ChatEventInputPending struct {
	Pending bool
}

func (ChatEventInputPending) chatEvent() {}

// ChatEventCompacting reports a background compaction starting or ending. On
// the end of an applied one usage estimates the compacted context — the
// summary plus the rounds kept verbatim — and is zero otherwise.
type ChatEventCompacting struct {
	Running bool
	Usage   EventUsage
}

func (ChatEventCompacting) chatEvent() {}

// ChatEventThreadCreated reports that the thread's first round created it in
// storage; it fires once per thread, before that round's start event.
type ChatEventThreadCreated struct{}

func (ChatEventThreadCreated) chatEvent() {}

// ChatEventName reports the thread's name changing.
type ChatEventName struct {
	Name string
}

func (ChatEventName) chatEvent() {}

// ChatEventTasks reports the thread's task list changing.
type ChatEventTasks struct {
	Tasks []Task
}

func (ChatEventTasks) chatEvent() {}

// ChatEventCriticalError reports the thread failing beyond recovery.
type ChatEventCriticalError struct {
	Err error
}

func (ChatEventCriticalError) chatEvent() {}
