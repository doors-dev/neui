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
