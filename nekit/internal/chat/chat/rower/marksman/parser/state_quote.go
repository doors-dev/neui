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

package parser

import (
	"strings"
)

type quoteState struct {
	offset     int
	innerDepth int
	inner      offsetState
	paragraph  bool
	cells      int
}

func (s *quoteState) getOffset() int {
	return s.offset
}

func (s *quoteState) updateOffset(offset int) {
	s.offset = offset
}

func (s *quoteState) paragraphOpen() bool {
	if s.inner != nil {
		return s.inner.paragraphOpen()
	}
	return s.paragraph
}

func (s *quoteState) line(ctrl ctrl) {
	if isBlank(ctrl.line()) {
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	depth := 0
	offsets := make([]int, 0)
	offset := func() int {
		if len(offsets) == 0 {
			return s.offset
		}
		return offsets[len(offsets)-1]
	}
	for {
		column := ctrl.contentColumn(offset())
		if depth == 0 && column >= 4 {
			break
		}
		if depth > 0 && column >= 5 {
			break
		}
		if !strings.HasPrefix(ctrl.tail(offset(), column), ">") {
			break
		}
		depth += 1
		offsets = append(offsets, offset()+1+column)
	}
	column := ctrl.contentColumn(offset())
	content := ctrl.tail(offset(), column)
	if depth == 0 {
		if s.paragraphOpen() && isParagraphContinuation(column, content) {
			s.cells = tableCells(content)
			ctrl.write(s.cells != 0)
			return
		}
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	if s.inner != nil && depth < s.innerDepth {
		if s.paragraphOpen() && isParagraphContinuation(column, content) {
			s.cells = tableCells(content)
			ctrl.write(s.cells != 0)
			return
		}
		s.inner = nil
		s.innerDepth = 0
		s.paragraph = false
	}
	if s.inner != nil {
		s.inner.updateOffset(ctrl.childOffset(offsets[s.innerDepth-1]))
		s.inner.line(quoteCtrl{s, ctrl})
		return
	}
	if isBlank(content) {
		ctrl.write(false)
		s.paragraph = false
		return
	}
	if column >= 5 {
		if s.paragraph {
			s.cells = tableCells(content)
			ctrl.write(s.cells != 0)
			return
		}
		s.inner = &indentedState{offset: ctrl.childOffset(offset())}
		s.innerDepth = depth
		s.inner.line(quoteCtrl{s, ctrl})
		return
	}
	if isATXHeading(content) {
		ctrl.write(false)
		s.paragraph = false
		return
	}
	if m, ok := isFence(content); ok {
		ctrl.write(false)
		s.paragraph = false
		s.inner = newFencedState(m, ctrl.childOffset(offset()))
		s.innerDepth = depth
		return
	}
	if kind := htmlBlockStart(content); kind != 0 && (kind != 7 || !s.paragraph) {
		ctrl.write(false)
		s.paragraph = false
		if kind <= 5 && htmlBlockEnd(kind, content) {
			return
		}
		s.inner = &htmlState{offset: ctrl.childOffset(offset()), kind: kind}
		s.innerDepth = depth
		return
	}
	if rest, ok := referenceLabel(content); ok && !s.paragraph {
		reference := &referenceState{offset: ctrl.childOffset(offset())}
		switch reference.consume(rest) {
		case referenceContinue:
			ctrl.write(false)
			s.inner = reference
			s.innerDepth = depth
			return
		case referenceDone:
			ctrl.write(false)
			return
		}
	}
	if cells, ok := isTableDelimiter(content); ok && s.paragraph && cells == s.cells {
		ctrl.write(false)
		s.paragraph = false
		s.inner = &tableState{offset: ctrl.childOffset(offset())}
		s.innerDepth = depth
		return
	}
	if isSetextUnderline(content) && s.paragraph {
		ctrl.write(false)
		s.paragraph = false
		return
	}
	if isThematicBreak(content) {
		ctrl.write(false)
		s.paragraph = false
		return
	}
	if isList(content) {
		if s.paragraph && !listInterruptsParagraph(content) {
			s.cells = tableCells(content)
			ctrl.write(s.cells != 0)
			return
		}
		s.paragraph = false
		s.inner = &listState{offset: ctrl.childOffset(offset())}
		s.innerDepth = depth
		s.inner.line(quoteCtrl{s, ctrl})
		return
	}
	s.cells = tableCells(content)
	ctrl.write(s.cells != 0)
	s.paragraph = true
}

type quoteCtrl struct {
	quote *quoteState
	ctrl  ctrl
}

func (c quoteCtrl) contentColumn(column int) int {
	return c.ctrl.contentColumn(column)
}

func (c quoteCtrl) tail(offset int, column int) string {
	return c.ctrl.tail(offset, column)
}

func (c quoteCtrl) childOffset(markerColumn int) int {
	return c.ctrl.childOffset(markerColumn)
}

func (c quoteCtrl) dispatch() {
	c.quote.line(c.ctrl)
}

func (c quoteCtrl) flush() {
}

func (c quoteCtrl) flushWritten() {
	panic("unimplemented")
}

func (c quoteCtrl) line() string {
	return c.ctrl.line()
}

func (c quoteCtrl) update(s state) {
	if _, ok := s.(initalState); !ok {
		panic("quote ctrl can only be returned to inital state")
	}
	c.quote.inner = nil
	c.quote.innerDepth = 0
	c.quote.paragraph = false
}

func (c quoteCtrl) write(hold bool) {
	c.ctrl.write(hold)
}

var _ ctrl = quoteCtrl{}
