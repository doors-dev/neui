package api

// RecordID identifies a record within its thread; -1 stands for no record.
type RecordID int

// Record is a thread-level entry produced by code: Origin is the round
// whose action created it — a record with no distinct cause carries the
// same round as its Pin; Pin is the round whose user message renders it
// on replay (NoRoundID while unpinned).
//
// The model reads a record as an element named Kind with Attrs as attributes
// and Data, escaped or JSON-encoded, as its body.
type Record struct {
	ID     RecordID       `json:"id"`     // identifies the record within its thread
	Origin RoundID        `json:"origin"` // round whose action created it
	Pin    RoundID        `json:"pin"`    // round that renders it; NoRoundID while unpinned
	Kind   string         `json:"kind"`   // element name, e.g. "task_notify"
	Attrs  map[string]any `json:"attrs"`  // element attributes; may be nil
	Data   any            `json:"data"`   // body: a string, json.RawMessage or any JSON-encodable value
}
