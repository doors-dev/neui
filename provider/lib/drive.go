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

package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gammazero/deque"
	"neui.dev/api"
)

// Converter is the whole of an adapter that Stream needs: the wire format, and
// what a failure means in it. Stream itself constructs no event.
//
// Both methods run on the driving goroutine and are never called concurrently,
// so a converter may keep whatever state its wire format forces on it — an
// index remapping, a tool call being assembled — without a lock.
type Converter interface {
	// Convert turns one frame into the events it carries. The frame's Data is
	// reused once Convert returns, so neither the converter nor the events
	// may keep any of it but copies. Returning false says the turn is over
	// and nothing further will be read, which is what bounds a peer that
	// keeps writing after saying how the reply ended. End is not called after
	// it: the converter has already said how the turn ended.
	Convert(Frame) ([]api.Event, bool)

	// End is every other ending, never called with nil: io.EOF for a body that
	// ended between events, io.ErrUnexpectedEOF for one cut mid event, an
	// *HTTPError for a non-200 response, ErrIdle (or the read error it caused)
	// for the idle deadline, the cancellation error when parentCtx is
	// cancelled, or whatever reading failed with, including an oversized
	// frame. Only the adapter can classify any of them, so only the adapter
	// builds the event; returning nothing ends the turn with no terminal event,
	// which says the reply is incomplete.
	End(err error) []api.Event
}

// Options are the transport limits Stream keeps on a turn. Hosts pass them to a
// provider's WithTransport option, which hands them here unread; the zero value
// takes both defaults.
type Options struct {
	// Idle ends a turn when no complete frame or keepalive has arrived for
	// this long, counting from the request until response headers and then
	// from each frame; a frame that takes longer than Idle to arrive in full
	// trips it even while bytes flow. A slow consumer does not count. Zero or
	// less is DefaultIdle.
	Idle time.Duration
	// Max caps the bytes of one frame's data and of any single line; going
	// over ends the reading and End receives an error. Zero or less is
	// DefaultMaxFrame.
	Max int
}

// DefaultIdle is the idle deadline when Options.Idle is unset. There is no
// "never" value; pass a long duration instead.
const DefaultIdle = 2 * time.Minute

// DefaultMaxFrame is the frame cap when Options.Max is unset. It is generous
// because the frame that ends a turn can carry the whole response, and exists
// only to keep a broken peer from exhausting memory. There is no "no cap"
// value; pass a larger number instead.
const DefaultMaxFrame = 64 << 20

// ErrIdle is the cancellation cause Stream sets when Options.Idle elapses,
// and what End is usually handed then; a read that fails from the same
// cancellation can reach End first with its own error.
var ErrIdle = errors.New("neui: stream went idle")

// maxErrorBody bounds what is read off a response that was not 200. At a
// gateway that body is frequently HTML or nothing at all, so it is read for the
// adapter to make what it can of and not trusted to be small.
const maxErrorBody = 1 << 20

// HTTPError is a response the API refused, handed to Converter.End as raw
// material rather than as a verdict: the status classifies, the headers carry
// retry-after and whatever request id the API uses, and the body is the API's
// own account of it. Every one of those is spelled differently per API, so the
// adapter decodes it and Stream does not look inside.
type HTTPError struct {
	Status int         // HTTP status code, never 200
	Header http.Header // response headers, for retry-after and request ids
	Body   []byte      // response body, at most 1 MiB
}

// Error returns the status only; the body is left to the adapter to decode.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("neui: http %d", e.Status)
}

// Drain is a turn that ended before it began, for a request that could not be
// built out of what the caller asked for. The events are already in the channel
// and it is already closed, so nothing blocks on it and a caller that never
// reads it simply drops it.
func Drain(events ...api.Event) <-chan api.Event {
	ch := make(chan api.Event, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

// Stream sends req with client and returns the turn's events as conv produces
// them from the response, applying the opt defaults. The channel closes after
// Convert returns false or End has run, once their events are delivered.
// Cancelling parentCtx ends the turn, and the channel closes without waiting
// for queued events to be read; otherwise the caller must read the channel
// until it closes, or the streaming goroutine leaks.
func Stream(parentCtx context.Context, client *http.Client, req *http.Request, conv Converter, opt Options) <-chan api.Event {
	ctx, cancel := context.WithCancelCause(parentCtx)
	if opt.Idle <= 0 {
		opt.Idle = DefaultIdle
	}
	if opt.Max <= 0 {
		opt.Max = DefaultMaxFrame
	}
	s := &streamer{
		conv:      conv,
		ch:        make(chan api.Event),
		client:    client,
		parentCtx: parentCtx,
		ctx:       ctx,
		cancel:    cancel,
		opt:       opt,
	}
	go s.run(req)
	return s.ch
}

type streamer struct {
	queue     deque.Deque[api.Event]
	conv      Converter
	ch        chan api.Event
	client    *http.Client
	parentCtx context.Context
	ctx       context.Context
	cancel    context.CancelCauseFunc
	timer     *time.Timer
	opt       Options
}

func (s *streamer) resetTimer() {
	if s.timer == nil {
		s.timer = time.AfterFunc(s.opt.Idle, func() {
			s.cancel(ErrIdle)
		})
		return
	}
	if s.ctx.Err() != nil {
		return
	}
	if !s.timer.Stop() {
		return
	}
	s.timer.Reset(s.opt.Idle)
}

func (s *streamer) run(req *http.Request) {
	s.resetTimer()
	defer s.drain()
	defer s.timer.Stop()
	defer s.cancel(nil)
	if body, ok := req.Body.(*bodyReader); ok {
		defer body.body.release()
	}
	resp, err := s.client.Do(req.WithContext(s.ctx))
	if err != nil {
		if context.Cause(s.ctx) == ErrIdle {
			err = ErrIdle
		}
		s.append(s.conv.End(err))
		return
	}
	s.resetTimer()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		endErr := &HTTPError{Status: resp.StatusCode, Header: resp.Header, Body: body}
		s.append(s.conv.End(endErr))
		return
	}
	s.read(resp.Body)
}

func (s *streamer) read(body io.Reader) {
	free := make(chan []byte, 2)
	read := read(s.ctx, body, s.opt.Max, free)
	for {
		var ch chan api.Event
		var event api.Event
		if s.queue.Len() > 0 {
			ch = s.ch
			event = s.queue.Front()
		}
		select {
		case <-s.ctx.Done():
			err := s.ctx.Err()
			if context.Cause(s.ctx) == ErrIdle {
				err = ErrIdle
			}
			s.append(s.conv.End(err))
			return
		case fr := <-read:
			if fr.err != nil {
				s.append(s.conv.End(fr.err))
				return
			}
			s.resetTimer()
			if fr.frame.isKeepAlive() {
				continue
			}
			frames, ok := s.conv.Convert(fr.frame)
			select {
			case free <- fr.frame.Data:
			default:
			}
			s.append(frames)
			if !ok {
				return
			}
		case ch <- event:
			s.queue.PopFront()
		}
	}
}

func (s *streamer) drain() {
	defer close(s.ch)
	for e := range s.queue.Iter() {
		select {
		case <-s.parentCtx.Done():
			return
		case s.ch <- e:
		}
	}
}

func (s *streamer) append(events []api.Event) {
	for _, e := range events {
		s.queue.PushBack(e)
	}
}
