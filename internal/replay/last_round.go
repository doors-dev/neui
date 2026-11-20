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
	err     string
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
	return l.state != api.RoundOk && !refuses(l.state, l.err)
}

func (l lastRound) isValid() bool {
	return l.id != api.NoRoundID
}
