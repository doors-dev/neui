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

package round

import (
	"context"
	"testing"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

type thread struct {
	api.ChatThread
}

func (thread) FS() api.FS {
	return nil
}

type rewinder struct{}

func (rewinder) Rewind() doors.Beam[api.RoundID] {
	return doors.NewSource(api.NoRoundID)
}

func (rewinder) InitRewind(context.Context, api.RoundID, api.Input) {}

func TestRejectedRoundOffersNoRetry(t *testing.T) {
	latest := doors.NewSource(api.RoundID(1))
	for _, c := range []struct {
		state     api.RoundState
		err       string
		retriable bool
	}{
		{api.RoundError, string(api.CodeInvalid), false},
		{api.RoundError, string(api.CodeTooLarge), false},
		{api.RoundError, string(api.CodeNetwork), true},
		{api.RoundAbort, "", true},
		{api.RoundRefuse, "", false},
		{api.RoundOk, "", false},
	} {
		stored := NewRound(context.Background(), params.Params{}, thread{}, rewinder{}, latest, api.Round{ID: 1, State: c.state, Error: c.err}, nil, false)
		if stored.retriable != c.retriable {
			t.Fatalf("a stored %s round with error %q is retriable %v, want %v", c.state, c.err, stored.retriable, c.retriable)
		}
		live := NewLiveRound(context.Background(), params.Params{}, thread{}, rewinder{}, latest, api.Round{ID: 1}, false, nil)
		live.Done(context.Background(), c.state, c.err)
		if got := live.retriable.Get(); got != c.retriable {
			t.Fatalf("a live round ending %s with error %q is retriable %v, want %v", c.state, c.err, got, c.retriable)
		}
	}
}
