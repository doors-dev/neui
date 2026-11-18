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

package list

import (
	"context"
	"testing"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

type roundsThread struct {
	api.ChatThread
	rounds []api.Round
}

func (t roundsThread) RoundBefore(ctx context.Context, start api.RoundID, limit int) ([]api.Round, error) {
	end := len(t.rounds)
	if start != api.NoRoundID {
		end = min(int(start), len(t.rounds))
	}
	return t.rounds[max(end-limit, 0):end], nil
}

func (t roundsThread) RoundAfter(ctx context.Context, start api.RoundID, limit int) ([]api.Round, error) {
	from := int(start) + 1
	if start == api.NoRoundID {
		from = 0
	}
	from = min(from, len(t.rounds))
	return t.rounds[from:min(from+limit, len(t.rounds))], nil
}

type stillRewinder struct{}

func (stillRewinder) Rewind() doors.Beam[api.RoundID] {
	return doors.NewSource(api.NoRoundID)
}

func (stillRewinder) InitRewind(context.Context, api.RoundID, api.Input) {}

func TestAnchorInLeadingNoInputRounds(t *testing.T) {
	var rounds []api.Round
	for i := range 3 * margin {
		r := api.Round{ID: api.RoundID(i), State: api.RoundOk}
		if i >= 3 {
			r.Input.Text = "hi"
		}
		rounds = append(rounds, r)
	}
	ctx := context.Background()
	thread := roundsThread{rounds: rounds}
	l := New(params.Params{}, nil, thread, stillRewinder{}, "t", nil, 1, func(context.Context, error) {})
	if _, err := l.seed(ctx, nil, 1); err != nil {
		t.Fatal(err)
	}
	if l.history.Len() == 0 || l.history.Front().id != 0 {
		t.Fatalf("history does not open at the leading section")
	}
	if !l.scrollState.topReached || l.scrollState.bottomReached {
		t.Fatalf("scroll state: want top reached and bottom open, got %+v", l.scrollState)
	}
}
