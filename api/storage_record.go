package api

// RecordID identifies a record within its thread; -1 stands for no record.
type RecordID int

// Record is a thread-level entry produced by code: Origin is the round
// whose action created it — a record with no distinct cause carries the
// same round as its Pin; Pin is the round whose user message renders it
// on replay (NoRoundID while unpinned).
type Record struct {
	ID     RecordID       `json:"id"`
	Origin RoundID        `json:"origin"`
	Pin    RoundID        `json:"pin"`
	Kind   string         `json:"kind"`
	Attrs  map[string]any `json:"attrs"`
	Data   any            `json:"data"`
}
