package timemod

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeTask struct {
	ok      chan json.RawMessage
	err     chan error
	release chan struct{}
}

func newFakeTask() *fakeTask {
	return &fakeTask{ok: make(chan json.RawMessage, 1), err: make(chan error, 1), release: make(chan struct{})}
}

func (t *fakeTask) ID() uint { return 1 }

func (t *fakeTask) Ok(out json.RawMessage) {
	t.ok <- out
	<-t.release
}

func (t *fakeTask) Err(err error) { t.err <- err }

func (t *fakeTask) Notify(json.RawMessage, map[string]any) {}

func arm(m *module) (*pending, context.Context) {
	ctx, cancel := context.WithCancelCause(context.Background())
	p := &pending{comment: "c", cancel: cancel, done: make(chan struct{})}
	m.pending.Store(p)
	return p, ctx
}

func TestCancelDuringFiringFindsNoTimer(t *testing.T) {
	m := &module{}
	p, ctx := arm(m)
	task := newFakeTask()
	go m.wait(ctx, p, task, time.Now())
	select {
	case <-task.ok:
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
	if m.pending.Load() != nil {
		t.Fatal("pending not cleared before task.Ok")
	}
	_, err := m.cancel(context.Background(), struct{}{})
	if err == nil || !strings.Contains(err.Error(), "no timer is pending") {
		t.Fatalf("cancel during firing: %v", err)
	}
	close(task.release)
	<-p.done
}

func TestCancelClaimedBeforeFiringReportsCancellation(t *testing.T) {
	m := &module{}
	p, ctx := arm(m)
	task := newFakeTask()
	p.claimed.Store(true)
	m.pending.Store(nil)
	go m.wait(ctx, p, task, time.Now())
	time.Sleep(20 * time.Millisecond)
	p.cancel(nil)
	select {
	case err := <-task.err:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected cause %v", err)
		}
	case <-task.ok:
		t.Fatal("cancelled timer reported a firing")
	case <-time.After(time.Second):
		t.Fatal("no result")
	}
	<-p.done
}

func TestCancelPendingTimer(t *testing.T) {
	m := &module{}
	p, ctx := arm(m)
	task := newFakeTask()
	go m.wait(ctx, p, task, time.Now().Add(time.Hour))
	if _, err := m.cancel(context.Background(), struct{}{}); err != nil {
		t.Fatal(err)
	}
	if err := <-task.err; !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cause %v", err)
	}
	if m.pending.Load() != nil {
		t.Fatal("pending not cleared")
	}
}

func TestTimerHugeAfterRejected(t *testing.T) {
	m := &module{}
	for _, after := range []uint{uint(maxDelay.Milliseconds()) + 1, 1 << 62, ^uint(0)} {
		_, err := m.timer(context.Background(), TimerArg{After: after, Comment: "c"})
		if err == nil || !strings.Contains(err.Error(), "at most") {
			t.Fatalf("afterMs=%d: %v", after, err)
		}
	}
}
