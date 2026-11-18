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

package window

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type failingDriver struct{}

func (failingDriver) Compare(a string, b string) int {
	return strings.Compare(a, b)
}

func (failingDriver) Sections(context.Context, string, bool, bool, int) ([]Section, bool, error) {
	return nil, false, errors.New("disk")
}

func TestSyncLoadErrorClearsRangeForRetry(t *testing.T) {
	for _, older := range []bool{true, false} {
		w := &List{Driver: failingDriver{}, Margin: 2, mu: new(sync.Mutex)}
		w.history.PushBack(&slot{key: "5"})
		w.scrollState.first, w.scrollState.last = "5", "5"
		w.scrollState.oldestReached = !older
		w.scrollState.spawned = true
		w.mu.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		more := w.sync(ctx)
		w.mu.Unlock()
		if more {
			t.Fatalf("older=%v: sync asked for another pass after a load error", older)
		}
		if w.scrollState.first != "" || w.scrollState.last != "" || !w.scrollState.synced {
			t.Fatalf("older=%v: range kept after a load error: %+v", older, w.scrollState)
		}
		w.updateRange(context.Background(), "5", "5")
		if w.scrollState.synced {
			t.Fatalf("older=%v: the same range was not retried", older)
		}
	}
}
