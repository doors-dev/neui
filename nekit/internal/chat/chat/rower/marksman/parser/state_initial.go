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

type initalState struct {
	paragraph bool
	cells     int
}

func (s initalState) line(ctrl ctrl) {
	if isBlank(ctrl.line()) {
		ctrl.flush()
		ctrl.update(initalState{})
		return
	}
	column := ctrl.contentColumn(-1)
	content := ctrl.tail(-1, column)
	if column >= 4 {
		if s.paragraph {
			cells := tableCells(content)
			ctrl.write(cells != 0)
			ctrl.update(initalState{paragraph: true, cells: cells})
			return
		}
		ctrl.flush()
		ctrl.update(&indentedState{offset: -1})
		ctrl.dispatch()
		return
	}
	if isATXHeading(content) {
		ctrl.flush()
		ctrl.write(false)
		ctrl.flush()
		ctrl.update(initalState{})
		return
	}
	if m, ok := isFence(content); ok {
		ctrl.flush()
		ctrl.write(false)
		ctrl.update(newFencedState(m, -1))
		return
	}
	if kind := htmlBlockStart(content); kind != 0 && (kind != 7 || !s.paragraph) {
		ctrl.flush()
		ctrl.write(false)
		if kind <= 5 && htmlBlockEnd(kind, content) {
			ctrl.flush()
			ctrl.update(initalState{})
			return
		}
		ctrl.update(&htmlState{offset: -1, kind: kind})
		return
	}
	if isQuote(content) {
		ctrl.flush()
		ctrl.update(&quoteState{offset: -1})
		ctrl.dispatch()
		return
	}
	if rest, ok := referenceLabel(content); ok && !s.paragraph {
		reference := &referenceState{offset: -1}
		switch reference.consume(rest) {
		case referenceContinue:
			ctrl.flush()
			ctrl.write(false)
			ctrl.update(reference)
			return
		case referenceDone:
			ctrl.flush()
			ctrl.write(false)
			ctrl.flush()
			ctrl.update(initalState{})
			return
		}
	}
	if cells, ok := isTableDelimiter(content); ok && s.paragraph && cells == s.cells {
		ctrl.flushWritten()
		ctrl.write(false)
		ctrl.update(&tableState{offset: -1})
		return
	}
	if isSetextUnderline(content) && s.paragraph {
		ctrl.write(false)
		ctrl.flush()
		ctrl.update(initalState{})
		return
	}
	if isThematicBreak(content) {
		ctrl.flush()
		ctrl.write(false)
		ctrl.flush()
		ctrl.update(initalState{})
		return
	}
	if isList(content) {
		if s.paragraph && !listInterruptsParagraph(content) {
			cells := tableCells(content)
			ctrl.write(cells != 0)
			ctrl.update(initalState{paragraph: true, cells: cells})
			return
		}
		ctrl.flush()
		ctrl.update(&listState{offset: -1})
		ctrl.dispatch()
		return
	}
	cells := tableCells(content)
	ctrl.write(cells != 0)
	ctrl.update(initalState{paragraph: true, cells: cells})
}
