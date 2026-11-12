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
		more := w.sync(context.Background())
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
