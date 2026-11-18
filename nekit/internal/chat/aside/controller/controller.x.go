// Managed by GoX v0.4.0

//line controller.gox:1
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

package controller

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

type SearchKind int

const (
	searchNone SearchKind = iota
	searchAll
	SearchNames
	SearchMessagesCurrent
	SearchMessagesAll
)

type state struct {
	searchInput string
	expanded bool
	searchKind SearchKind
	searchInputReset int
	window Window
}

func New(namesOnly bool, open bool) Controller {
	s := doors.NewSource(state{expanded: open})
	input := doors.DeriveBeam(s, func(s state) int {
		return s.searchInputReset
	})
	expanded := doors.DeriveBeam(s, func(s state) bool {
		return s.expanded
	})
	top := searchAll
	if namesOnly {
		top = SearchNames
	}
	return Controller{
		state: s,
		inputReset: input,
		expanded: expanded,
		top: top,
	}
}

type Controller struct {
	state doors.Source[state]
	inputReset doors.Beam[int]
	expanded doors.Beam[bool]
	top SearchKind
}

type Window struct {
	First string
	Last string
}

type searchElem = func(input string, w Window, set func(ctx context.Context, w Window)) gox.Elem

func (s Controller) searchRoute(kind SearchKind, el searchElem) doors.BeamRoute[state] {
	return doors.RouteBeamMatch(func(v state) bool {
		return v.searchKind == kind
	}, doors.RouteBeamDerive(func(b doors.Beam[state]) doors.Beam[string] {
		return doors.DeriveBeam(b, func(v state) string {
			return v.searchInput
		})
	}, doors.RouteBind(func(v string) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line controller.gox:85
			state, ok := s.state.Read(ctx)
		if !ok {
			return nil
		}

//line controller.gox:90
			__e = __c.Any(el(v, state.window, func(ctx context.Context, w Window) {
			s.saveWindow(ctx, kind, w)
		})); if __e != nil { return }
		return })
//line controller.gox:93
	})))
}

func (s Controller) Expanded() doors.Beam[bool] {
	return s.expanded
}

func (s Controller) Show(el gox.Elem) gox.Elem {
	return s.expanded.Bind(func(show bool) gox.Elem {
		if show {
			return el
		}
		return nil
	})
}
//line controller.gox:108
func (s Controller) Input(render func(v string) gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line controller.gox:109
		__e = __c.Any(s.inputReset.Bind(func(int) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line controller.gox:111
			s, ok := s.state.Read(ctx)
		if !ok {
			return
		}

//line controller.gox:116
			__e = __c.Any(render(s.searchInput)); if __e != nil { return }
		return })
//line controller.gox:117
	})); if __e != nil { return }
	return })
//line controller.gox:118
}

func (s Controller) Collapse(ctx context.Context) {
	s.state.Mutate(ctx, func(s state) state {
		s.expanded = false
		return s
	})
}

func (s Controller) Toggle(ctx context.Context) {
	top := s.top
	s.state.Mutate(ctx, func(s state) state {
		switch true {
		case !s.expanded:
			s.expanded = true
			return s
		case s.searchKind == searchNone:
			s.expanded = false
			return s
		case s.searchKind == top:
			s.searchKind = searchNone
			s.window = Window{}
			s.searchInput = ""
			s.searchInputReset += 1
			return s
		default:
			s.searchKind = searchAll
			return s
		}
	})
}

func (s Controller) SearchInput(ctx context.Context, input string) {
	top := s.top
	s.state.Mutate(ctx, func(s state) state {
		if s.searchInput == input {
			return s
		}
		s.searchInput = input
		s.window = Window{}
		if s.searchInput != "" && s.searchKind == searchNone {
			s.searchKind = top
		}
		if s.searchInput == "" && s.searchKind == top {
			s.searchKind = searchNone
		}
		return s
	})
}

func (s Controller) RouteToggle(open gox.Elem, close gox.Elem, back gox.Elem) gox.Elem {
	return s.state.Route(
		doors.RouteMatch(func(s state) bool {
			return !s.expanded
		}, doors.RouteComp[state](open)),
		doors.RouteMatch(func(s state) bool {
			return s.expanded && s.searchKind == searchNone
		}, doors.RouteComp[state](close)),
		doors.RouteMatch(func(s state) bool {
			return s.expanded && s.searchKind != searchNone
		}, doors.RouteComp[state](back)),
	)
}

func (s Controller) RouteList(
	days func(w Window, set func(ctx context.Context, w Window)) gox.Elem,
	all func(input string, open func(ctx context.Context, kind SearchKind)) gox.Elem,
	names searchElem,
	messages searchElem,
	messagesAll searchElem,
) gox.Elem {
	return s.state.Route(
		doors.RouteMatch(func(s state) bool {
			return s.searchKind == searchNone
		}, doors.RouteComp[state](gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line controller.gox:194
				state, ok := s.state.Read(ctx)

//line controller.gox:196
				if ok {
//line controller.gox:197
					__e = __c.Any(days(state.window, func(ctx context.Context, w Window) {
					s.saveWindow(ctx, searchNone, w)
				})); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line controller.gox:201
		return }))),
		s.searchRoute(searchAll, func(input string, _ Window, _ func(context.Context, Window)) gox.Elem {
			return all(input, func(ctx context.Context, kind SearchKind) {
				if ctx.Err() != nil {
					return
				}
				s.state.Mutate(ctx, func(s state) state {
					if s.searchKind != searchAll {
						return s
					}
					s.searchKind = kind
					s.window = Window{}
					return s
				})
			})
		}),
		s.searchRoute(SearchNames, names),
		s.searchRoute(SearchMessagesCurrent, messages),
		s.searchRoute(SearchMessagesAll, messagesAll),
	)
}
func (s Controller) saveWindow(ctx context.Context, kind SearchKind, w Window) {
	if ctx.Err() != nil {
		return
	}
	s.state.Mutate(ctx, func(s state) state {
		if s.searchKind != kind {
			return s
		}
		s.window = w
		return s
	})
}
