package parser

import (
	"regexp"
	"strings"
)

// offset is the absolute column of the last prefix character of the enclosing container (-1 at root); the result is relative to offset+1.
func contentColumn(line string, offset int) (column int) {
	col := 0
	i := 0
	for i < len(line) && col < offset {
		if line[i] == '\t' {
			col += 4 - col%4
		} else {
			col++
		}
		i++
	}
	if i < len(line) && col == offset {
		if line[i] == '\t' {
			col += 4 - col%4
		} else {
			col++
		}
		i++
	}
	for ; i < len(line); i++ {
		switch line[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col - (offset + 1)
		}
	}
	return col - (offset + 1)
}

func tail(line string, offset int, column int) string {
	column = max(offset+1+column, 0)
	col := 0
	for i := 0; i < len(line); i++ {
		if col >= column {
			return line[i:]
		}
		if line[i] == '\t' {
			next := col + 4 - col%4
			if next > column {
				return strings.Repeat(" ", next-column) + line[i+1:]
			}
			col = next
		} else {
			col++
		}
	}
	return ""
}

func childOffset(line string, markerColumn int) int {
	next := tail(line, markerColumn, 0)
	if strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t") {
		return markerColumn + 1
	}
	return markerColumn
}

type listKind int

const (
	listNone  listKind = iota
	listDash           // -
	listPlus           // +
	listStar           // *
	listDot            // 1.
	listParen          // 1)
)

func isList(content string) bool {
	kind, _ := listMarker(content, -1)
	return kind != listNone
}

func listMarker(line string, offset int) (kind listKind, contentCol int) {
	column := contentColumn(line, offset)
	rest := tail(line, offset, column)
	if rest == "" {
		return listNone, 0
	}
	switch rest[0] {
	case '-':
		kind = listDash
	case '+':
		kind = listPlus
	case '*':
		kind = listStar
	}
	i := 0
	if kind == listNone {
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 0 || i > 9 || i >= len(rest) {
			return listNone, 0
		}
		switch rest[i] {
		case '.':
			kind = listDot
		case ')':
			kind = listParen
		default:
			return listNone, 0
		}
	}
	if i+1 < len(rest) && rest[i+1] != ' ' && rest[i+1] != '\t' {
		return listNone, 0
	}
	markerColumn := offset + 1 + column + i
	itemColumn := contentColumn(line, markerColumn)
	if itemColumn > 4 || tail(line, markerColumn, itemColumn) == "" {
		itemColumn = 1
	}
	return kind, (markerColumn + 1 + itemColumn) - (offset + 1)
}

func isThematicBreak(content string) bool {
	var ch byte
	count := 0
	for i := 0; i < len(content); i++ {
		c := content[i]
		switch {
		case c == ' ' || c == '\t':
		case ch == 0 && (c == '-' || c == '_' || c == '*'):
			ch = c
			count = 1
		case c == ch:
			count++
		default:
			return false
		}
	}
	return count >= 3
}

func isSetextUnderline(content string) bool {
	if len(content) == 0 {
		return false
	}
	ch := content[0]
	if ch != '=' && ch != '-' {
		return false
	}
	i := 0
	for i < len(content) && content[i] == ch {
		i++
	}
	for i < len(content) {
		if content[i] != ' ' && content[i] != '\t' {
			return false
		}
		i++
	}
	return true
}

func isATXHeading(content string) bool {
	n := 0
	for n < len(content) && content[n] == '#' {
		n++
	}
	if n < 1 || n > 6 {
		return false
	}
	return n == len(content) || content[n] == ' ' || content[n] == '\t'
}

var fence = regexp.MustCompile("^(`{3,}|~{3,})")

func isFence(line string) (string, bool) {
	m := fence.FindString(line)
	if len(m) == 0 || (m[0] == '`' && strings.Contains(line[len(m):], "`")) {
		return "", false
	}
	return m, true
}

func isQuote(content string) bool {
	return strings.HasPrefix(content, ">")

}

func isBlank(line string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}
	return true
}

// A list item may interrupt a paragraph only if it is not empty and,
// when ordered, its start number is 1.
func listInterruptsParagraph(content string) bool {
	kind, contentColumn := listMarker(content, -1)
	if kind == listNone || isBlank(tail(content, -1, contentColumn)) {
		return false
	}
	if kind == listDot || kind == listParen {
		digits := content[:strings.IndexAny(content, ".)")]
		return strings.TrimLeft(digits, "0") == "1"
	}
	return true
}

// Line cannot start a new block, so it may lazily continue an open paragraph.
// column is relative to the last matched container's content base.
func isParagraphContinuation(column int, content string) bool {
	if column >= 4 {
		return true
	}
	if isATXHeading(content) || isThematicBreak(content) || isQuote(content) {
		return false
	}
	if _, ok := isFence(content); ok {
		return false
	}
	if kind := htmlBlockStart(content); kind != 0 && kind != 7 {
		return false
	}
	return !listInterruptsParagraph(content)
}

// splitTableRow splits a row on unescaped pipes; a leading and a trailing
// pipe are optional. pipes reports whether the row had any unescaped pipe.
func splitTableRow(content string) (cells []string, pipes bool) {
	content = strings.TrimRight(content, " \t")
	start := 0
	backslashes := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\\' {
			backslashes++
			continue
		}
		if content[i] == '|' && backslashes%2 == 0 {
			pipes = true
			if i != 0 {
				cells = append(cells, content[start:i])
			}
			start = i + 1
		}
		backslashes = 0
	}
	if start < len(content) || !pipes {
		cells = append(cells, content[start:])
	}
	return cells, pipes
}

// tableCells counts the cells of a possible header row; 0 when the line has
// no unescaped pipe and so cannot start a table.
func tableCells(content string) int {
	cells, pipes := splitTableRow(content)
	if !pipes {
		return 0
	}
	return max(len(cells), 1)
}

// isTableDelimiter reports the cell count of a delimiter row: every cell is
// dashes with an optional colon on either side.
func isTableDelimiter(content string) (int, bool) {
	cells, _ := splitTableRow(content)
	for _, cell := range cells {
		cell = strings.Trim(cell, " \t")
		cell = strings.TrimPrefix(cell, ":")
		cell = strings.TrimSuffix(cell, ":")
		if len(cell) == 0 || len(strings.Trim(cell, "-")) != 0 {
			return 0, false
		}
	}
	return len(cells), true
}

var htmlTagName = `[A-Za-z][A-Za-z0-9-]*`
var htmlAttribute = `\s+[A-Za-z_:][A-Za-z0-9_.:-]*(\s*=\s*([^\s"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?`
var htmlOpenTag = regexp.MustCompile(`^<` + htmlTagName + `(` + htmlAttribute + `)*\s*/?>\s*$`)
var htmlClosingTag = regexp.MustCompile(`^</` + htmlTagName + `\s*>\s*$`)
var htmlRawStart = regexp.MustCompile(`(?i)^<(script|pre|style|textarea)(\s|>|$)`)
var htmlRawEnd = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)
var htmlBlockStart6 = regexp.MustCompile(`(?i)^</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(\s|/?>|$)`)

// htmlBlockStart reports which of the seven HTML block start conditions the
// content meets, 0 if none. Kind 7 (a complete open or closing tag alone on
// the line) cannot interrupt a paragraph.
func htmlBlockStart(content string) int {
	switch {
	case htmlRawStart.MatchString(content):
		return 1
	case strings.HasPrefix(content, "<!--"):
		return 2
	case strings.HasPrefix(content, "<?"):
		return 3
	case strings.HasPrefix(content, "<![CDATA["):
		return 5
	case len(content) > 2 && content[0] == '<' && content[1] == '!' && isASCIILetter(content[2]):
		return 4
	case htmlBlockStart6.MatchString(content):
		return 6
	case htmlOpenTag.MatchString(content) || htmlClosingTag.MatchString(content):
		return 7
	}
	return 0
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// htmlBlockEnd reports whether the line ends an HTML block of kinds 1-5
// (kinds 6 and 7 end at a blank line instead).
func htmlBlockEnd(kind int, content string) bool {
	switch kind {
	case 1:
		return htmlRawEnd.MatchString(content)
	case 2:
		return strings.Contains(content, "-->")
	case 3:
		return strings.Contains(content, "?>")
	case 4:
		return strings.Contains(content, ">")
	case 5:
		return strings.Contains(content, "]]>")
	}
	return false
}

// referenceLabel returns the text after "[label]:" when content starts with a
// link reference label: one line, no unescaped brackets, at least one
// non-space character, at most 999 characters.
func referenceLabel(content string) (rest string, ok bool) {
	if len(content) == 0 || content[0] != '[' {
		return "", false
	}
	text := false
	for i := 1; i < len(content); i++ {
		switch content[i] {
		case '\\':
			i++
			text = true
		case '[':
			return "", false
		case ']':
			if !text || i > 1000 || i+1 >= len(content) || content[i+1] != ':' {
				return "", false
			}
			return content[i+2:], true
		case ' ', '\t':
		default:
			text = true
		}
	}
	return "", false
}

// referenceDestination consumes leading whitespace and a link destination
// (<...> without line breaks or unescaped angle brackets, or a run of
// non-space characters); ok is false when there is none.
func referenceDestination(text string) (rest string, ok bool) {
	text = strings.TrimLeft(text, " \t")
	if len(text) == 0 {
		return text, false
	}
	if text[0] == '<' {
		for i := 1; i < len(text); i++ {
			switch text[i] {
			case '\\':
				i++
			case '<':
				return text, false
			case '>':
				return text[i+1:], true
			}
		}
		return text, false
	}
	i := 0
	for i < len(text) && text[i] != ' ' && text[i] != '\t' {
		i++
	}
	return text[i:], true
}

// referenceTitleStart consumes leading whitespace and a title opener,
// returning the matching closer.
func referenceTitleStart(text string) (closing byte, rest string, ok bool) {
	text = strings.TrimLeft(text, " \t")
	if len(text) == 0 {
		return 0, text, false
	}
	switch text[0] {
	case '"', '\'':
		return text[0], text[1:], true
	case '(':
		return ')', text[1:], true
	}
	return 0, text, false
}

// referenceTitleEnd scans for the unescaped closer, returning what follows it.
func referenceTitleEnd(text string, closing byte) (rest string, ok bool) {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case closing:
			return text[i+1:], true
		}
	}
	return text, false
}
