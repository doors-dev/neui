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
