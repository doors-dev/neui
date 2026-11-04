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
		// 4 columns of indent: indented code, not a marker
		if depth == 0 && column >= 4 {
			break
		}
		// +1 for the optional space after a marker: indented code, not a nested marker
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
		// lazy continuation of an open paragraph, otherwise the quote ends
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
		// too few markers for the child: lazy continuation keeps it, anything else drops it
		if s.paragraphOpen() && isParagraphContinuation(column, content) {
			s.cells = tableCells(content)
			ctrl.write(s.cells != 0)
			return
		}
		s.inner = nil
		s.innerDepth = 0
		s.paragraph = false
	}
	// the child is re-based on the marker at its depth, deeper markers are its content
	if s.inner != nil {
		s.inner.updateOffset(ctrl.childOffset(offsets[s.innerDepth-1]))
		s.inner.line(quoteCtrl{s, ctrl})
		return
	}
	// markers only: the paragraph ends, the quote doesn't
	if isBlank(content) {
		ctrl.write(false)
		s.paragraph = false
		return
	}
	// +1 for the optional space after the last marker: indented code
	if column >= 5 {
		if s.paragraph {
			s.cells = tableCells(content) // indented code can't interrupt a paragraph
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
	// kind 7, a lone complete tag, can't interrupt a paragraph
	if kind := htmlBlockStart(content); kind != 0 && (kind != 7 || !s.paragraph) {
		ctrl.write(false)
		s.paragraph = false
		// kinds 1-5 may end on the line that opens them
		if kind <= 5 && htmlBlockEnd(kind, content) {
			return
		}
		s.inner = &htmlState{offset: ctrl.childOffset(offset()), kind: kind}
		s.innerDepth = depth
		return
	}
	// a definition can't interrupt a paragraph
	if rest, ok := referenceLabel(content); ok && !s.paragraph {
		reference := &referenceState{offset: ctrl.childOffset(offset())}
		// an invalid start falls through: the line is paragraph text
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
	// a delimiter row with as many cells as the line above makes that line a table header
	if cells, ok := isTableDelimiter(content); ok && s.paragraph && cells == s.cells {
		ctrl.write(false)
		s.paragraph = false
		s.inner = &tableState{offset: ctrl.childOffset(offset())}
		s.innerDepth = depth
		return
	}
	if isSetextUnderline(content) && s.paragraph { // before thematic: "---" under a paragraph is a heading
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
		// an empty item or an ordered one not starting at 1 can't interrupt a paragraph
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
	ctrl.write(s.cells != 0) // a line with a pipe is held: a delimiter row next makes it a table header
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
	// the whole quote is one block: a child's flush must not split it
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
