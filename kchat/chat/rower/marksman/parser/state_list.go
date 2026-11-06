package parser

type listItem int

const (
	itemEmpty listItem = iota
	itemClosed
	itemBlock
	itemParagraph
)

type listState struct {
	offset        int
	kind          listKind
	contentColumn int
	inner         offsetState
	item          listItem
	cells         int
}

func (s *listState) getOffset() int {
	return s.offset
}

func (s *listState) updateOffset(offset int) {
	s.offset = offset
}

func (s *listState) paragraphOpen() bool {
	if s.inner != nil {
		return s.inner.paragraphOpen()
	}
	return s.item == itemParagraph
}

func (s *listState) line(c ctrl) {
	kind, itemContentColumn := listMarker(c.line(), s.offset)
	startColumn := c.contentColumn(s.offset)
	content := c.tail(s.offset, startColumn)
	if startColumn >= 4 || isThematicBreak(content) {
		kind = listNone
	}
	first := s.kind == listNone
	if first {
		if kind == listNone {
			panic("can't be initalized with list none")
		}
		s.kind = kind
	}
	blank := isBlank(c.line())
	if kind != listNone && kind != s.kind && startColumn < s.contentColumn {
		c.flush()
		c.update(initalState{})
		c.dispatch()
		return
	}
	if s.inner != nil && (blank || startColumn >= s.contentColumn) {
		s.inner.line(listCtrl{c, s})
		return
	}
	if kind != listNone && (first || startColumn < s.contentColumn) {
		s.inner = nil
		s.item = itemBlock
		s.contentColumn = itemContentColumn
	} else if blank {
		c.write(false)
		switch s.item {
		case itemEmpty:
			s.item = itemClosed
		case itemParagraph:
			s.item = itemBlock
		}
		return
	} else if startColumn < s.contentColumn {
		if s.paragraphOpen() && isParagraphContinuation(startColumn, content) {
			s.cells = tableCells(content)
			c.write(s.cells != 0)
			return
		}
		c.flush()
		c.update(initalState{})
		c.dispatch()
		return
	}
	if s.item == itemClosed {
		c.flush()
		c.update(initalState{})
		c.dispatch()
		return
	}
	itemOffset := s.offset + s.contentColumn
	column := c.contentColumn(itemOffset)
	content = c.tail(itemOffset, column)
	if isBlank(content) {
		c.write(false)
		s.item = itemEmpty
		return
	}
	if column >= 4 {
		if s.item == itemParagraph {
			s.cells = tableCells(content)
			c.write(s.cells != 0)
			return
		}
		s.item = itemBlock
		s.inner = &indentedState{offset: itemOffset}
		s.inner.line(listCtrl{c, s})
		return
	}
	if isATXHeading(content) {
		c.write(false)
		s.item = itemBlock
		return
	}
	if m, ok := isFence(content); ok {
		c.write(false)
		s.item = itemBlock
		s.inner = newFencedState(m, itemOffset)
		return
	}
	if kind := htmlBlockStart(content); kind != 0 && (kind != 7 || s.item != itemParagraph) {
		c.write(false)
		s.item = itemBlock
		if kind <= 5 && htmlBlockEnd(kind, content) {
			return
		}
		s.inner = &htmlState{offset: itemOffset, kind: kind}
		return
	}
	if rest, ok := referenceLabel(content); ok && s.item != itemParagraph {
		reference := &referenceState{offset: itemOffset}
		switch reference.consume(rest) {
		case referenceContinue:
			c.write(false)
			s.item = itemBlock
			s.inner = reference
			return
		case referenceDone:
			c.write(false)
			s.item = itemBlock
			return
		}
	}
	if cells, ok := isTableDelimiter(content); ok && s.item == itemParagraph && cells == s.cells {
		c.write(false)
		s.item = itemBlock
		s.inner = &tableState{offset: itemOffset}
		return
	}
	if isSetextUnderline(content) && s.item == itemParagraph {
		c.write(false)
		s.item = itemBlock
		return
	}
	if isThematicBreak(content) {
		c.write(false)
		s.item = itemBlock
		return
	}
	if isQuote(content) {
		s.item = itemBlock
		s.inner = &quoteState{offset: itemOffset}
		s.inner.line(listCtrl{c, s})
		return
	}
	if isList(content) {
		if s.item == itemParagraph && !listInterruptsParagraph(content) {
			s.cells = tableCells(content)
			c.write(s.cells != 0)
			return
		}
		s.item = itemBlock
		s.inner = &listState{offset: itemOffset}
		s.inner.line(listCtrl{c, s})
		return
	}
	s.cells = tableCells(content)
	c.write(s.cells != 0)
	s.item = itemParagraph
}

type listCtrl struct {
	ctrl ctrl
	list *listState
}

func (l listCtrl) childOffset(markerColumn int) int {
	return l.ctrl.childOffset(markerColumn)
}

func (l listCtrl) contentColumn(column int) int {
	return l.ctrl.contentColumn(column)
}

func (l listCtrl) dispatch() {
	l.list.line(l.ctrl)
}

func (l listCtrl) flush() {
}

func (l listCtrl) flushWritten() {
	panic("unimplemented")
}

func (l listCtrl) line() string {
	return l.ctrl.line()
}

func (l listCtrl) tail(offset int, column int) string {
	return l.ctrl.tail(offset, column)
}

func (l listCtrl) update(s state) {
	if _, ok := s.(initalState); !ok {
		panic("list ctrl can only be returned to inital state")
	}
	l.list.inner = nil
	l.list.item = itemBlock
}

func (l listCtrl) write(hold bool) {
	l.ctrl.write(hold)
}

var _ ctrl = listCtrl{}
