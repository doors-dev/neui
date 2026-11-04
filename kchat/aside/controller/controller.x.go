// Managed by GoX v0.3.3

//line controller.gox:1
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

func New() Controller {
	s := doors.NewSource(state{})
	input := doors.DeriveBeam(s, func(s state) int {
		return s.searchInputReset
	})
	expanded := doors.DeriveBeam(s, func(s state) bool {
		return s.expanded
	})
	return Controller{
		state: s,
		inputReset: input,
		expanded: expanded,
	}
}

type Controller struct {
	state doors.Source[state]
	inputReset doors.Beam[int]
	expanded doors.Beam[bool]
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
//line controller.gox:65
			state, ok := s.state.Read(ctx)
		if !ok {
			return nil
		}

//line controller.gox:70
			__e = __c.Any(el(v, state.window, func(ctx context.Context, w Window) {
			s.saveWindow(ctx, kind, w)
		})); if __e != nil { return }
		return })
//line controller.gox:73
	})))
}

func (s Controller) Show(el gox.Elem) gox.Elem {
	return s.expanded.Bind(func(show bool) gox.Elem {
		if show {
			return el
		}
		return nil
	})
}
//line controller.gox:84
func (s Controller) Input(render func(v string) gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line controller.gox:85
		__e = __c.Any(s.inputReset.Bind(func(int) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line controller.gox:87
			s, ok := s.state.Read(ctx)
		if !ok {
			return
		}

//line controller.gox:92
			__e = __c.Any(render(s.searchInput)); if __e != nil { return }
		return })
//line controller.gox:93
	})); if __e != nil { return }
	return })
//line controller.gox:94
}

func (s Controller) Collapse(ctx context.Context) {
	s.state.Mutate(ctx, func(s state) state {
		s.expanded = false
		return s
	})
}

func (s Controller) Toggle(ctx context.Context) {
	s.state.Mutate(ctx, func(s state) state {
		switch true {
		case !s.expanded:
			s.expanded = true
			return s
		case s.searchKind == searchNone:
			s.expanded = false
			return s
		case s.searchKind == searchAll:
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
	s.state.Mutate(ctx, func(s state) state {
		if s.searchInput == input {
			return s
		}
		s.searchInput = input
		s.window = Window{}
		if s.searchInput != "" && s.searchKind == searchNone {
			s.searchKind = searchAll
		}
		if s.searchInput == "" && s.searchKind == searchAll {
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
//line controller.gox:168
				state, ok := s.state.Read(ctx)

//line controller.gox:170
				if ok {
//line controller.gox:171
					__e = __c.Any(days(state.window, func(ctx context.Context, w Window) {
					s.saveWindow(ctx, searchNone, w)
				})); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line controller.gox:175
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
