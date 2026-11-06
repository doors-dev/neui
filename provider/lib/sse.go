// Package lib holds the parts of a Provider that do not depend on which API
// answered: reading a server sent event stream, keeping a deadline on one, and
// handing a caller events while the wire keeps moving.
//
// Hosts use Options, passed to a provider's WithTransport to tune the idle
// deadline and the frame size cap; ErrIdle can reach them inside an
// api.EventErr.
package lib

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
)

// Frame is one dispatched server sent event: the value of its event: line and
// its data: lines joined with newlines. Data is valid only until
// Converter.Convert returns: Stream then hands the buffer back to the reader,
// which fills it with a later frame. A converter keeps nothing of Data past
// Convert but copies, which is all json.Unmarshal ever stores in a string,
// []byte or json.RawMessage.
type Frame struct {
	Event string // value of the event: line; "" when absent
	Data  []byte // data: lines joined with "\n"; never empty
}

// isKeepAlive says the frame is the wire proving it is alive rather than an
// event. The zero Frame is what no dispatched event can be: the format
// dispatches nothing while the data buffer is empty, so a frame that reaches a
// consumer always carries at least one byte.
func (f Frame) isKeepAlive() bool {
	return f.Event == "" && f.Data == nil
}

var errFrameTooLarge = errors.New("neui: server sent event exceeds the frame limit")

// frameReader parses the wire format rather than assuming it. Every provider
// puts a whole JSON payload on one data: line today, but the format allows it
// to be split across several, and one tool argument or reasoning delta runs
// well past any fixed line buffer, so lines are accumulated instead of scanned.
type frameReader struct {
	ch    chan readFrame
	free  <-chan []byte
	br    *bufio.Reader
	ctx   context.Context
	max   int
	line  []byte
	data  []byte
	name  []byte
	event string
}

type readFrame struct {
	frame Frame
	err   error
}

func read(ctx context.Context, r io.Reader, max int, free <-chan []byte) <-chan readFrame {
	reader := frameReader{
		ch:   make(chan readFrame),
		free: free,
		br:   bufio.NewReaderSize(r, 64<<10),
		max:  max,
		ctx:  ctx,
	}
	go reader.run()
	return reader.ch
}

func (r *frameReader) run() {
	for {
		f, err := r.next()
		select {
		case <-r.ctx.Done():
			return
		case r.ch <- readFrame{f, err}:
		}
		if err != nil {
			return
		}
	}
}

// next returns the next event, or the zero Frame for a stretch of wire that
// carried no event. A comment and a blank line that dispatches nothing are both
// the peer proving the connection is alive while a request waits in a queue or
// a server side tool runs, and both come back rather than being skipped, so the
// deadline above is reset by what the wire did and not by what it produced.
//
// The format dispatches nothing while the data buffer is empty, so a blank line
// after an event: with nothing attached is a dispatch point that produced no
// event. A comment arriving mid event is skipped rather than returned, because
// returning there would drop the event: line and the data: lines already read.
//
// A body that ends in the middle of an event took its last frame with it: the
// format says an event without its blank line was never dispatched, so what
// arrived is a cut connection and not a shorter reply.
//
// A dispatched frame takes the data buffer with it. The next one starts in a
// buffer a consumer handed back on free once it was done with it, or in a new
// one when none has come back yet.
func (r *frameReader) next() (Frame, error) {
	r.data = r.data[:0]
	r.name = r.name[:0]
	for {
		line, err := r.readLine()
		if err != nil {
			if errors.Is(err, io.EOF) && (len(r.data) > 0 || len(r.name) > 0) {
				return Frame{}, io.ErrUnexpectedEOF
			}
			return Frame{}, err
		}
		if len(line) == 0 {
			if len(r.data) == 0 {
				return Frame{}, nil
			}
			if string(r.name) != r.event {
				r.event = string(r.name)
			}
			data := r.data
			r.data = nil
			return Frame{Event: r.event, Data: data}, nil
		}
		if line[0] == ':' {
			if len(r.data) > 0 || len(r.name) > 0 {
				continue
			}
			return Frame{}, nil
		}
		field, value := cutField(line)
		switch string(field) {
		case "event":
			r.name = append(r.name[:0], value...)
		case "data":
			if r.data == nil {
				select {
				case buf := <-r.free:
					r.data = buf[:0]
				default:
				}
			}
			if len(r.data) > 0 {
				r.data = append(r.data, '\n')
			}
			r.data = append(r.data, value...)
			if r.max > 0 && len(r.data) > r.max {
				return Frame{}, errFrameTooLarge
			}
		}
	}
}

// readLine returns one line without its terminator. bufio.Reader rather than
// bufio.Scanner because Scanner refuses a line longer than its buffer and a
// completed response object exceeds the default on ordinary long turns.
func (r *frameReader) readLine() ([]byte, error) {
	r.line = r.line[:0]
	for {
		chunk, err := r.br.ReadSlice('\n')
		if r.max > 0 && len(r.line)+len(chunk) > r.max {
			return nil, errFrameTooLarge
		}
		r.line = append(r.line, chunk...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) && len(r.line) > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		line := bytes.TrimSuffix(r.line, []byte("\n"))
		return bytes.TrimSuffix(line, []byte("\r")), nil
	}
}

// cutField splits a field line. A single space after the colon is part of the
// syntax, not the value.
func cutField(line []byte) (field, value []byte) {
	i := bytes.IndexByte(line, ':')
	if i < 0 {
		return line, nil
	}
	field, value = line[:i], line[i+1:]
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return field, value
}
