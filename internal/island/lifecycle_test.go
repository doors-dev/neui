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

package island

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/doors-dev/gox"
)

func newTestIsland(t *testing.T) (*island, context.Context) {
	t.Helper()
	i := NewIsland(gox.Elem(func(gox.Cursor) error { return nil }), func(slot gox.Comp) gox.Elem { return slot.Main() }, nil, nil, NewLibrary())
	t.Cleanup(i.Unmount)
	return i, context.Background()
}

func hungRun(i *island) *run {
	ctx, cancel := context.WithCancelCause(context.Background())
	r := &run{
		island:      i,
		pageContext: context.Background(),
		ctx:         ctx,
		cancel:      cancel,
		end:         make(chan struct{}),
	}
	i.mu.Lock()
	i.seq++
	r.seq = i.seq
	i.run = r
	i.mu.Unlock()
	return r
}

func waitFor(t *testing.T, i *island, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		i.mu.Lock()
		ok := cond()
		i.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitDone[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("call did not return")
	}
	var zero T
	return zero
}

func TestStartWaitingOnOldRunOvertaken(t *testing.T) {
	i, ctx := newTestIsland(t)
	old := hungRun(i)
	first := make(chan error, 1)
	go func() {
		_, _, err := i.StartCode(ctx, "app", "", "1")
		first <- err
	}()
	waitFor(t, i, func() bool { return i.seq == old.seq+1 })
	secondCtx, cancel := context.WithCancel(ctx)
	second := make(chan error, 1)
	go func() {
		_, _, err := i.StartCode(secondCtx, "app", "", "1")
		second <- err
	}()
	waitFor(t, i, func() bool { return i.seq == old.seq+2 })
	cancel()
	if err := waitDone(t, second); !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	close(old.end)
	if err := waitDone(t, first); !errors.Is(err, errOvertaken) {
		t.Fatalf("want overtaken, got %v", err)
	}
	waitFor(t, i, func() bool { return i.run == nil })
}

func TestUnmountWhileStartAndStopWaitOnHungRun(t *testing.T) {
	i, ctx := newTestIsland(t)
	old := hungRun(i)
	start := make(chan error, 1)
	go func() {
		_, _, err := i.StartCode(ctx, "app", "", "1")
		start <- err
	}()
	waitFor(t, i, func() bool { return i.seq == old.seq+1 })
	stop := make(chan error, 1)
	go func() {
		stop <- i.Done(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	unmounted := make(chan struct{}, 1)
	go func() {
		i.Unmount()
		unmounted <- struct{}{}
	}()
	waitDone(t, unmounted)
	close(old.end)
	if err := waitDone(t, start); !errors.Is(err, errUnmounted) {
		t.Fatalf("want unmounted, got %v", err)
	}
	if err := waitDone(t, stop); err != nil {
		t.Fatal(err)
	}
}

func TestStopReturnsOnCanceledCall(t *testing.T) {
	i, ctx := newTestIsland(t)
	old := hungRun(i)
	callCtx, cancel := context.WithCancel(ctx)
	stop := make(chan error, 1)
	go func() {
		stop <- i.Done(callCtx)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := waitDone(t, stop); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(old.ctx), errStopped) {
		t.Fatalf("the run was not stopped: %v", context.Cause(old.ctx))
	}
	close(old.end)
}

func TestOvertakeCanceledShowsPlaceholderOnceOldRunEnds(t *testing.T) {
	i, ctx := newTestIsland(t)
	old := hungRun(i)
	callCtx, cancel := context.WithCancel(ctx)
	start := make(chan error, 1)
	go func() {
		_, _, err := i.StartCode(callCtx, "app", "", "1")
		start <- err
	}()
	waitFor(t, i, func() bool { return i.seq == old.seq+1 })
	cancel()
	if err := waitDone(t, start); !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	i.mu.Lock()
	kept := i.run == old
	i.mu.Unlock()
	if !kept {
		t.Fatal("the old run was dropped before it ended")
	}
	close(old.end)
	waitFor(t, i, func() bool { return i.run == nil })
}

func TestOvertakeCanceledKeepsNewerStart(t *testing.T) {
	i, ctx := newTestIsland(t)
	old := hungRun(i)
	callCtx, cancel := context.WithCancel(ctx)
	start := make(chan error, 1)
	go func() {
		_, _, err := i.StartCode(callCtx, "app", "", "1")
		start <- err
	}()
	waitFor(t, i, func() bool { return i.seq == old.seq+1 })
	cancel()
	if err := waitDone(t, start); !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	newer := &run{end: make(chan struct{})}
	i.mu.Lock()
	i.seq++
	i.run = newer
	i.mu.Unlock()
	close(old.end)
	time.Sleep(50 * time.Millisecond)
	i.mu.Lock()
	kept := i.run == newer
	i.mu.Unlock()
	if !kept {
		t.Fatal("the canceled start cleared a newer start's run")
	}
	i.mu.Lock()
	i.run = nil
	i.mu.Unlock()
}
