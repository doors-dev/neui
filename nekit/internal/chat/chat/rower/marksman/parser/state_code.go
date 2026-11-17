package parser

import "strings"

type indentedState struct {
	offset int
}

func (s *indentedState) getOffset() int {
	return s.offset
}

func (s *indentedState) updateOffset(offset int) {
	s.offset = offset
}

func (s *indentedState) paragraphOpen() bool { return false }

func (s indentedState) line(ctrl ctrl) {
	if isBlank(ctrl.line()) {
		ctrl.write(false)
		return
	}
	column := ctrl.contentColumn(s.offset)
	if column < 4 {
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	ctrl.write(false)
}

func newFencedState(marker string, offset int) *fencedState {
	return &fencedState{
		char:   string(marker[0]),
		count:  len(marker),
		offset: offset,
	}
}

type fencedState struct {
	char   string
	count  int
	offset int
}

func (s *fencedState) getOffset() int {
	return s.offset
}

func (s *fencedState) updateOffset(offset int) {
	s.offset = offset
}

func (s *fencedState) paragraphOpen() bool { return false }

func (s *fencedState) line(ctrl ctrl) {
	ctrl.write(false)
	if isBlank(ctrl.line()) {
		return
	}
	column := ctrl.contentColumn(s.offset)
	if column >= 4 {
		return
	}
	content := ctrl.tail(s.offset, column)
	content = strings.TrimRight(content, " \t")
	if len(strings.Trim(content, s.char)) != 0 || len(content) < s.count {
		return
	}
	ctrl.flush()
	ctrl.update(initalState{})
}

type tableState struct {
	offset int
}

func (s *tableState) getOffset() int {
	return s.offset
}

func (s *tableState) updateOffset(offset int) {
	s.offset = offset
}

func (s *tableState) paragraphOpen() bool {
	return false
}

func (s *tableState) line(ctrl ctrl) {
	if isBlank(ctrl.line()) {
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	column := ctrl.contentColumn(s.offset)
	content := ctrl.tail(s.offset, column)
	_, fence := isFence(content)
	if column >= 4 || isATXHeading(content) || fence || isThematicBreak(content) || isQuote(content) || isList(content) || htmlBlockStart(content) != 0 {
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	ctrl.write(false)
}

type htmlState struct {
	offset int
	kind   int
}

func (s *htmlState) getOffset() int {
	return s.offset
}

func (s *htmlState) updateOffset(offset int) {
	s.offset = offset
}

func (s *htmlState) paragraphOpen() bool {
	return false
}

func (s *htmlState) line(ctrl ctrl) {
	if s.kind >= 6 {
		if isBlank(ctrl.line()) {
			ctrl.flush()
			ctrl.update(initalState{})
			ctrl.dispatch()
			return
		}
		ctrl.write(false)
		return
	}
	ctrl.write(false)
	column := ctrl.contentColumn(s.offset)
	if htmlBlockEnd(s.kind, ctrl.tail(s.offset, column)) {
		ctrl.flush()
		ctrl.update(initalState{})
	}
}

type referenceStatus int

const (
	referenceContinue referenceStatus = iota
	referenceDone
	referenceEnded
	referenceInvalid
)

type referenceStage int

const (
	referenceDestinationStage referenceStage = iota
	referenceTitleStage
	referenceInTitleStage
)

type referenceState struct {
	offset  int
	stage   referenceStage
	closing byte
}

func (s *referenceState) getOffset() int {
	return s.offset
}

func (s *referenceState) updateOffset(offset int) {
	s.offset = offset
}

func (s *referenceState) paragraphOpen() bool {
	return false
}

func (s *referenceState) consume(text string) referenceStatus {
	switch s.stage {
	case referenceDestinationStage:
		rest, ok := referenceDestination(text)
		if !ok {
			if isBlank(text) {
				return referenceContinue
			}
			return referenceInvalid
		}
		if isBlank(rest) {
			s.stage = referenceTitleStage
			return referenceContinue
		}
		if rest[0] != ' ' && rest[0] != '\t' {
			return referenceInvalid
		}
		closing, rest, ok := referenceTitleStart(rest)
		if !ok {
			return referenceInvalid
		}
		s.closing = closing
		rest, ok = referenceTitleEnd(rest, closing)
		if !ok {
			s.stage = referenceInTitleStage
			return referenceContinue
		}
		if isBlank(rest) {
			return referenceDone
		}
		return referenceInvalid
	case referenceTitleStage:
		closing, rest, ok := referenceTitleStart(text)
		if !ok {
			return referenceEnded
		}
		s.closing = closing
		rest, ok = referenceTitleEnd(rest, closing)
		if !ok {
			s.stage = referenceInTitleStage
			return referenceContinue
		}
		if isBlank(rest) {
			return referenceDone
		}
		return referenceEnded
	}
	rest, ok := referenceTitleEnd(text, s.closing)
	if !ok {
		return referenceContinue
	}
	if isBlank(rest) {
		return referenceDone
	}
	return referenceInvalid
}

func (s *referenceState) line(ctrl ctrl) {
	if isBlank(ctrl.line()) {
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
		return
	}
	column := ctrl.contentColumn(s.offset)
	switch s.consume(ctrl.tail(s.offset, column)) {
	case referenceContinue:
		ctrl.write(false)
	case referenceDone:
		ctrl.write(false)
		ctrl.flush()
		ctrl.update(initalState{})
	case referenceEnded:
		ctrl.flush()
		ctrl.update(initalState{})
		ctrl.dispatch()
	case referenceInvalid:
		ctrl.update(initalState{})
		ctrl.dispatch()
	}
}
