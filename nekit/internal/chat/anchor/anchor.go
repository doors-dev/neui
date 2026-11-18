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

package anchor

import (
	"sync/atomic"

	"neui.dev/api"
)

type Anchor struct {
	round *atomic.Pointer[api.RoundID]
}

func New(round api.RoundID) Anchor {
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
