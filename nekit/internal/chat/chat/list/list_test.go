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
	"errors"
	"testing"

	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

type failingThread struct {
	api.ChatThread
}

func (failingThread) RoundBefore(context.Context, api.RoundID, int) ([]api.Round, error) {
	return nil, errors.New("disk")
}

func (failingThread) RoundAfter(context.Context, api.RoundID, int) ([]api.Round, error) {
	return nil, errors.New("disk")
}

func TestFailedSeedIgnoresRoundEvents(t *testing.T) {
	ctx := context.Background()
	l := New(params.Params{}, nil, failingThread{}, nil, "t", nil, api.NoRoundID, func(context.Context, error) {})
	if _, err := l.seed(ctx, nil, api.NoRoundID); err == nil {
		t.Fatal("seed succeeded on a failing thread")
	}
	l.RoundStart(ctx, api.Round{ID: 1}, api.NoRoundID)
	l.RoundStart(ctx, api.Round{ID: 2}, 1)
	l.IndexedEvent(nil)
	l.ToolsDone(ctx)
	l.RoundDone(ctx, api.RoundOk)
	if l.lastSection.live != nil {
		t.Fatal("an unseeded list took a live round")
	}
}

func TestSyncLoadErrorClearsRangeForRetry(t *testing.T) {
	for _, older := range []bool{true, false} {
		l := &list{thread: failingThread{}}
		l.lastSection.entry.id = 30
		l.history.PushBack(entry{id: 10})
		l.scrollState.first, l.scrollState.last = 10, 10
		l.scrollState.topReached = !older
		l.scrollState.spawned = true
		l.mu.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		more := l.sync(ctx)
		l.mu.Unlock()
		if more {
			t.Fatalf("older=%v: sync asked for another pass after a load error", older)
		}
		if l.scrollState.first != api.NoRoundID || l.scrollState.last != api.NoRoundID || !l.scrollState.synced {
			t.Fatalf("older=%v: range kept after a load error: %+v", older, l.scrollState)
		}
		l.updateRange(context.Background(), 10, 10)
		if l.scrollState.synced {
			t.Fatalf("older=%v: the same range was not retried", older)
		}
	}
}
