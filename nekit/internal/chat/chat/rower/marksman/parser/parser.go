package parser

import (
	"bytes"
	"html"
	"strings"

	gmparser "github.com/yuin/goldmark/parser"
	"neui.dev/internal/markdown"
)

type state interface {
	line(ctrl ctrl)
}

type offsetState interface {
	getOffset() int
	updateOffset(offset int)
	line(ctrl ctrl)
	paragraphOpen() bool
}

type ctrl interface {
	line() string
	tail(offset int, column int) string
	contentColumn(offset int) int
	childOffset(markerColumn int) int
	flush()
	flushWritten()
	write(hold bool)
	dispatch()
	update(state)
}

type Parser = *parser

func New(upstream func([]byte)) Parser {
	r := newStreamer(nil)
	r.upstream = func(block []byte) { upstream(r.convert(block, true)) }
	return r
}

func newStreamer(upstream func([]byte)) *parser {
	return &parser{upstream: upstream, state: initalState{}}
}

type parser struct {
	upstream    func([]byte)
	state       state
	lineBuffer  strings.Builder
	blockBuffer bytes.Buffer
	held        string
	holding     bool
	references  []gmparser.Reference
}

func (r *parser) Write(chunk string) {
	for {
		tail, rest, found := strings.Cut(chunk, "\n")
		r.lineBuffer.WriteString(tail)
		if !found {
			break
		}
		r.state.line(streamerCtrl{r.line(), r})
		chunk = rest
	}
}

func (r *parser) Close() {
	if r.lineBuffer.Len() != 0 {
		r.state.line(streamerCtrl{r.line(), r})
	}
	r.flush()
}

func (r *parser) Partial() (partial []byte, ok bool) {
	raw, ok := r.pending()
	if !ok {
		return nil, false
	}
	return r.convert(raw, false), true
}

func (r *parser) pending() ([]byte, bool) {
	var buf bytes.Buffer
	buf.Write(r.blockBuffer.Bytes())
	if r.holding {
		buf.WriteString(r.held)
		buf.WriteRune('\n')
	}
	buf.WriteString(r.lineBuffer.String())
	if buf.Len() == 0 {
		return nil, false
	}
	return buf.Bytes(), true
}

func (r *parser) convert(block []byte, keep bool) []byte {
	context := gmparser.NewContext()
	for _, reference := range r.references {
		context.AddReference(reference)
	}
	var buf bytes.Buffer
	if err := markdown.Convert(block, &buf, gmparser.WithContext(context)); err != nil {
		buf.Reset()
		buf.WriteString("<pre>")
		buf.WriteString(html.EscapeString(string(block)))
		buf.WriteString("</pre>\n")
	} else if keep {
		r.references = context.References()
	}
	return buf.Bytes()
}

func (r *parser) line() string {
	line := strings.TrimSuffix(r.lineBuffer.String(), "\r")
	r.lineBuffer.Reset()
	return line
}

func (r *parser) release() {
	if !r.holding {
		return
	}
	r.blockBuffer.WriteString(r.held)
	r.blockBuffer.WriteRune('\n')
	r.holding = false
}

func (r *parser) flush() {
	r.release()
	if r.blockBuffer.Len() == 0 {
		return
	}
	r.upstream(bytes.Clone(r.blockBuffer.Bytes()))
	r.blockBuffer.Reset()
}

func (r *parser) flushWritten() {
	if r.blockBuffer.Len() == 0 {
		return
	}
	r.upstream(bytes.Clone(r.blockBuffer.Bytes()))
	r.blockBuffer.Reset()
}

type streamerCtrl struct {
	raw     string
	builder *parser
}

func (c streamerCtrl) contentColumn(col int) int {
	return contentColumn(c.raw, col)
}

func (c streamerCtrl) tail(offset int, column int) string {
	return tail(c.raw, offset, column)
}

func (c streamerCtrl) childOffset(markerColumn int) int {
	return childOffset(c.raw, markerColumn)
}

func (c streamerCtrl) line() string {
	return c.raw
}

func (c streamerCtrl) flush() {
	c.builder.flush()
}

func (c streamerCtrl) flushWritten() {
	c.builder.flushWritten()
}

func (c streamerCtrl) write(hold bool) {
	c.builder.release()
	if hold {
		c.builder.held = c.raw
		c.builder.holding = true
	} else {
		c.builder.blockBuffer.WriteString(c.raw)
		c.builder.blockBuffer.WriteRune('\n')
	}
}

func (c streamerCtrl) dispatch() {
	c.builder.state.line(c)
}

func (c streamerCtrl) update(s state) {
	c.builder.state = s
}
