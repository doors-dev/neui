package rower

import (
	"context"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/nekit/internal/chat/chat/follow"
)

type streamer[C any] interface {
	gox.Comp
	Append(C)
	Close()
}

func newInitialRow[C any]() *row[C] {
	return &row[C]{
		closed: true,
	}
}

func newRow[C any](content streamer[C]) *row[C] {
	return &row[C]{
		content: content,
	}
}

type row[C any] struct {
	mu      sync.Mutex
	content streamer[C]
	closed  bool
	frozen  bool
	after   *doors.Door
	seed    *doors.Door
}

func (r *row[C]) Append(c C) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.content.Append(c)
}

func (r *row[C]) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.content.Close()
}

func (r *row[C]) Freeze(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return
	}
	r.frozen = true
	if r.after != nil {
		r.after.Static(ctx, nil)
	}
}

func (r *row[C]) After(ctx context.Context, c *row[C], mark bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return
	}
	oldDoor := r.after
	if oldDoor == nil {
		r.seed = new(doors.Door)
		oldDoor = r.seed
	}
	newDoor := new(doors.Door)
	r.after = newDoor
	oldDoor.Static(ctx, gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Comp(newDoor); err != nil {
			return err
		}
		if mark {
			if err := cur.Any(follow.Mark()); err != nil {
				return err
			}
		}
		return cur.Comp(c)
	}))
}

func (r *row[C]) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if err := cur.Comp(r.content); err != nil {
			return err
		}
		if r.seed != nil {
			if err := cur.Comp(r.seed); err != nil {
				return err
			}
			r.seed = nil
			return nil
		}
		if r.frozen {
			return nil
		}
		r.after = new(doors.Door)
		return cur.Comp(r.after)
	})
}
