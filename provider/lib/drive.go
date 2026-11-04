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

// Converter is the whole of an adapter that Drive needs: the wire format, and
// what a failure means in it. Drive itself constructs no event.
//
// Both methods run on the driving goroutine and are never called concurrently,
// so a converter may keep whatever state its wire format forces on it — an
// index remapping, a tool call being assembled — without a lock.
type Converter interface {
	// Convert turns one frame into the events it carries. Returning false says
	// the turn is over and nothing further will be read, which is what bounds a
	// peer that keeps writing after saying how the reply ended. End is not
	// called after it: the converter has already said how the turn ended.
	Convert(Frame) ([]api.Event, bool)

	// End is every other ending. err is nil for a body that simply ran out, and
	// otherwise is what stopped the turn: an *HTTPError for a request the API
	// refused, ErrIdle for the deadline, or whatever the connection failed
	// with. Only the adapter can classify any of them, so only the adapter
	// builds the event; returning nothing ends the turn with no terminal event,
	// which says the reply is incomplete.
	End(err error) []api.Event
}

// Options are the limits Drive keeps on a turn, and the whole of what an
// adapter takes from its caller about transport: an adapter exposes one option
// carrying this value and hands it here unread, so the fields, their defaults
// and the reasoning for them live in one place rather than three.
//
// Idle ends a turn whose wire has been silent that long. The clock covers the
// wait for the response headers too, because a gateway that accepts a request
// and never answers is silence like any other. It does not run while the caller
// is slow to take what has already been produced. Zero or less is DefaultIdle.
//
// Max caps the bytes one frame may accumulate. A completed response object
// carries every text part, every tool call's arguments and every reasoning
// item's sealed payload, so frames run to megabytes and this is the only thing
// between a broken peer and the heap. Zero or less is DefaultMaxFrame.
type Options struct {
	Idle time.Duration
	Max  int
}

// DefaultIdle is the deadline a turn keeps when Options.Idle asks for none, and
// it is the only idle default there is: no adapter carries one of its own.
//
// There is no value meaning never. A caller who wants a turn to run effectively
// unbounded passes an hour, which says so out loud and needs no rule to
// remember, where a disable switch is a magic value whose failure mode is the
// one thing nothing recovers from: a half open socket parks the producer
// forever on a channel that never closes.
//
// Two minutes, because this deadline measures zero bytes and not slowness.
// Every API these adapters speak sends keepalives through a queue wait
// precisely so that a long wait is not silent, and Drive resets the clock on
// them without a Converter ever seeing one, so two minutes of nothing is a dead
// socket in every case anyone has observed. A larger number is defensible only
// for an API that says it may go that long without sending a byte, and none
// says so.
const DefaultIdle = 2 * time.Minute

// DefaultMaxFrame is the cap a turn keeps when Options.Max asks for none, and
// as with DefaultIdle it is the only frame default there is: no adapter carries
// one of its own.
//
// 64 MiB, because the frame that ends a turn carries the whole completed
// response object on the wire that sends one — every text part, every tool
// call's arguments and every sealed reasoning payload — so frames reach
// megabytes on an ordinary long turn and a cap tight enough to read as a budget
// would cut turns for being long. This one is here to keep a broken peer from
// taking the heap with it and nothing else.
//
// There is no value meaning no cap, for the reason DefaultIdle gives: a caller
// who wants one passes a number large enough to say so.
const DefaultMaxFrame = 64 << 20

// ErrIdle is what End is handed when Options.Idle elapsed.
var ErrIdle = errors.New("neui: stream went idle")

// maxErrorBody bounds what is read off a response that was not 200. At a
// gateway that body is frequently HTML or nothing at all, so it is read for the
// adapter to make what it can of and not trusted to be small.
const maxErrorBody = 1 << 20

// HTTPError is a response the API refused, handed to Converter.End as raw
// material rather than as a verdict: the status classifies, the headers carry
// retry-after and whatever request id the API uses, and the body is the API's
// own account of it. Every one of those is spelled differently per API, so the
// adapter decodes it and Drive does not look inside.
type HTTPError struct {
	Status int
	Header http.Header
	Body   []byte
}

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
	if s.ctx.Err() != nil {
		return
	}
	if s.timer == nil {
		s.timer = time.AfterFunc(s.opt.Idle, func() {
			s.cancel(ErrIdle)
		})
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
	read := read(s.ctx, body, s.opt.Max)
	for {
		//<-time.After(time.Second / 10)

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
