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

package pops

import (
	"context"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func (p Popups) Popup[T comparable]() Popup[T] {
	return Popup[T]{
		equal: func(t1, t2 T) bool {
			return t1 == t2
		},
		state: p.state,
	}
}

type Popup[T any] struct {
	equal func(T, T) bool
	state doors.Source[popState]
}

func (p Popup[T]) Show(ctx context.Context, id T) {
	doors.OnBatch(ctx, nil, func(ctx context.Context) {
		p.state.Mutate(ctx, func(ps popState) popState {
			v, ok := ps.id.(T)
			if ok && p.equal(v, id) {
				return ps
			}
			ps.id = id
			ps.seq += 1
			return ps
		})
	})
}

func (p Popup[T]) Hide(ctx context.Context, id T) {
	doors.OnBatch(ctx, nil, func(ctx context.Context) {
		p.state.Mutate(ctx, func(ps popState) popState {
			v, ok := ps.id.(T)
			if !ok || !p.equal(v, id) {
				return ps
			}
			ps.id = nil
			ps.seq += 1
			return ps
		})
	})
}

func (p Popup[T]) match(id T) func(popState) bool {
	return func(ps popState) bool {
		v, ok := ps.id.(T)
		if !ok {
			return false
		}
		return p.equal(v, id)
	}
}

func (p Popup[T]) BindPop(id T, el func(hide func(context.Context)) gox.Elem) gox.Elem {
	return p.state.Route(
		doors.RouteMatch(p.match(id), doors.RouteComp[popState](gox.Elem(func(cur gox.Cursor) error {
			return doors.ProxyMod(gox.ModifyFunc(func(ctx context.Context, tag string, attrs gox.Attrs) error {
				attrs.Get("data-nechat-pop").Set("")
				return nil
			})).Proxy(cur, el(func(ctx context.Context) {
				p.Hide(ctx, id)
			}))
		}))),
	)
}

func (p Popup[T]) BindControls(
	id T,
	hidden func(show func(ctx context.Context)) gox.Elem,
	shown func(hide func(ctx context.Context)) gox.Elem,
) gox.Elem {
	return p.state.Route(
		doors.RouteMatch(p.match(id), doors.RouteComp[popState](shown(func(ctx context.Context) {
			p.Hide(ctx, id)
		}))),
		doors.RouteComp[popState](hidden(func(ctx context.Context) {
			p.Show(ctx, id)
		})),
	)
}
