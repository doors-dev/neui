package anchor

import (
	"sync/atomic"

	"neui.dev/api"
)

type Anchor struct {
	round *atomic.Pointer[api.RoundID]
}

func NewAnchor(round api.RoundID) Anchor {
	a := Anchor{round: new(atomic.Pointer[api.RoundID])}
	a.round.Store(&round)
	return a
}

func Read(meta any) api.RoundID {
	a, ok := meta.(Anchor)
	if !ok || a.round == nil {
		return api.NoRoundID
	}
	round := a.round.Swap(nil)
	if round == nil {
		return api.NoRoundID
	}
	return *round
}
