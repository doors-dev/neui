package rower

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/doors-dev/gox"
)

type Indexed interface {
	At() int
}

func newRower[C Indexed](content map[int]streamer[C], factory func(C) streamer[C]) *rower[C] {
	r := &rower[C]{factory: factory}
	if len(content) == 0 {
		r.rows = map[int]*row[C]{
			-1: newInitialRow[C](),
		}
	} else {
		r.rows = make(map[int]*row[C], len(content))
		for index, c := range content {
			r.rows[index] = newRow(c)
		}
	}
	return r
}

type rower[C Indexed] struct {
	mu       sync.Mutex
	closed   bool
	rendered bool
	factory  func(C) streamer[C]
	rows     map[int]*row[C]
}

func (r *rower[C]) Append(c C) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		println("LOG ALREADY CLOSED")
		return
	}
	r.on(context.Background(), c)
}

func (rw *rower[C]) IsEmpty() bool {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	for num := range rw.rows {
		if num >= 0 {
			return false
		}
	}
	return true
}

func (rw *rower[C]) Close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.closed {
		return
	}
	rw.closed = true
	for _, r := range rw.rows {
		r.Close()
		r.Freeze(context.Background())
	}
}

func (rw *rower[C]) on(ctx context.Context, c C) {
	if r, ok := rw.rows[c.At()]; ok {
		r.Append(c)
		return
	}
	nums := rw.nums()
	content := rw.factory(c)
	if content == nil {
		return
	}
	row := newRow(content)
	rw.rows[c.At()] = row
	after := nums[len(nums)-1]
	for _, num := range nums {
		if c.At() < num {
			break
		}
		after = num
	}
	rw.rows[after].After(ctx, row, rw.rendered)
}

func (rw *rower[C]) nums() []int {
	return slices.Sorted(maps.Keys(rw.rows))
}

func (rw *rower[C]) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		rw.mu.Lock()
		defer rw.mu.Unlock()
		if rw.rendered {
			return errors.New("rower can be rendered only once")
		}
		rw.rendered = true
		for _, num := range rw.nums() {
			row := rw.rows[num]
			if err := cur.Comp(row); err != nil {
				return err
			}
		}
		return nil
	}
}

/*
func newRower[C Indexed](content map[int]streamer[C], factory func(C) streamer[C]) streamer[C] {
	r := &rower[C]{factory: factory}
	if len(content) == 0 {
		r.rows = map[int]*row[C]{
			-1: newInitialRow[C](),
		}
	} else {
		r.rows = make(map[int]*row[C], len(content))
		for index, c := range content {
			r.rows[index] = newRow(c)
		}
	}
	return r
}

type rower[C Indexed] struct {
	mu       sync.Mutex
	closed   bool
	factory  func(C) streamer[C]
	rendered bool
	queue    deque.Deque[*C]
	rows     map[int]*row[C]
	trigger  *chan struct{}
}

func (rw *rower[C]) Append(c C) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.send(&c)
}
func (rw *rower[C]) Close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.closed {
		return
	}
	rw.closed = true
	if rw.rendered {
		rw.send(nil)
		return
	}
	rw.close()
}

func (rw *rower[C]) send(c *C) {
	rw.queue.PushBack(c)
	if rw.trigger != nil {
		close(*rw.trigger)
		rw.trigger = nil
	}
}

func (rw *rower[C]) loop(ctx context.Context) {
	defer rw.close()
	for ctx.Err() == nil {
		rw.mu.Lock()
		if rw.queue.Len() == 0 {
			ch := make(chan struct{})
			rw.trigger = &ch
			rw.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ch:
				continue
			}
		}
		e := rw.queue.PopFront()
		rw.mu.Unlock()
		if e == nil {
			return
		}
		rw.on(ctx, *e)
	}
}

func (rw *rower[C]) on(ctx context.Context, c C) {
	if r, ok := rw.rows[c.At()]; ok {
		r.Append(c)
		return
	}
	nums := rw.nums()
	if len(nums) == 0 {
		println("LOG: added after close")
		return
	}
	content := rw.factory(c)
	if content == nil {
		return
	}
	row := newRow(content)
	rw.rows[c.At()] = row
	after := nums[len(nums)-1]
	for _, num := range nums {
		if c.At() < num {
			break
		}
		after = num
	}
	rw.rows[after].After(ctx, row)
}

func (rw *rower[C]) close() {
	for _, r := range rw.rows {
		r.Close()
		r.Freeze(context.Background())
	}
}

func (rw *rower[C]) nums() []int {
	return slices.Sorted(maps.Keys(rw.rows))
}

func (rw *rower[C]) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		rw.mu.Lock()
		defer rw.mu.Unlock()
		if rw.rendered {
			return errors.New("rower can be rendered only once")
		}
		rw.rendered = true
		for _, num := range rw.nums() {
			row := rw.rows[num]
			if err := cur.Comp(row); err != nil {
				return err
			}
		}
		if rw.closed {
			return nil
		}
		doors.OnReady(cur.Context(), func(ctx context.Context) {
			go rw.loop(ctx)
		})
		return nil
	})
} */
