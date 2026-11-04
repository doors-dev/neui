package replay

import (
	"time"

	"neui.dev/api"
)

type lastRound struct {
	id      api.RoundID
	created time.Time
	input   api.Input
	state   api.RoundState
	records []api.Record
}

func (r lastRound) round() api.Round {
	return api.Round{
		ID:            r.id,
		State:         r.state,
		Created:       r.created,
		Input:         r.input,
		PinnedRecords: r.records,
	}
}

func (l lastRound) isCompleted() bool {
	if !l.isValid() {
		return false
	}
	return l.state == api.RoundOk
}

func (l lastRound) isActive() bool {
	if !l.isValid() {
		return false
	}
	return l.state == api.RoundInput
}

func (l lastRound) isRetriable() bool {
	if !l.isValid() {
		return false
	}
	return l.state != api.RoundOk && l.state != api.RoundRefuse
}

func (l lastRound) isValid() bool {
	return l.id != api.NoRoundID
}
