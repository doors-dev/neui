package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
)

// conv converts with two functions, so a test says what a frame and an ending
// mean without declaring a type for each case.
type conv struct {
	convert func(Frame) ([]api.Event, bool)
	end     func(error) []api.Event
}

func (c conv) Convert(f Frame) ([]api.Event, bool) {
	return c.convert(f)
}

func (c conv) End(err error) []api.Event {
	if c.end == nil {
		return nil
	}
	return c.end(err)
}

// echo turns every frame into one EventText carrying "<event>|<data>", and the
// data "stop" into an EventOk that ends the turn, so a test reads the framing
// off the events.
func echo(end func(error) []api.Event) conv {
	return conv{
		convert: func(f Frame) ([]api.Event, bool) {
			if string(f.Data) == "stop" {
				return []api.Event{api.EventOk{Code: api.CodeTurn}}, false
			}
			return []api.Event{api.EventText{Text: f.Event + "|" + string(f.Data)}}, true
		},
		end: end,
	}
}

// errEvent is what most of these tests want out of End: the error, verbatim, as
// one event, so the test asserts what was handed over rather than a policy.
func errEvent(err error) []api.Event {
	return []api.Event{api.EventErr{Code: api.CodeNetwork, Err: err}}
}

// serve runs one turn against a handler and collects everything it produced.
func serve(t *testing.T, ctx context.Context, c Converter, opt Options, h http.HandlerFunc) []api.Event {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var got []api.Event
	for e := range Stream(ctx, srv.Client(), req, c, opt) {
		got = append(got, e)
	}
	return got
}

// stall holds a handler open until the client gives up, bounded so that
// httptest.Server.Close is never the thing that blocks a failing test.
func stall(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(2 * time.Second):
	}
}

func sse(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, body)
	}
}

func texts(events []api.Event) []string {
	var out []string
	for _, e := range events {
		if t, ok := e.(api.EventText); ok {
			out = append(out, t.Text)
		}
	}
	return out
}

func TestFraming(t *testing.T) {
	body := "event: one\ndata: {\"a\":1}\n\n" +
		"data: line1\ndata: line2\n\n" +
		"data:stop\n\n"

	got := texts(serve(t, context.Background(), echo(nil), Options{}, sse(body)))
	want := []string{"one|{\"a\":1}", "|line1\nline2"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A comment, a blank line that dispatches nothing, and an event: line with
// nothing attached are all the wire staying alive without producing an event.
// None of them reaches a converter.
func TestKeepalivesNeverReachTheConverter(t *testing.T) {
	seen := 0
	c := conv{convert: func(f Frame) ([]api.Event, bool) {
		seen++
		if string(f.Data) == "stop" {
			return []api.Event{api.EventOk{Code: api.CodeTurn}}, false
		}
		return nil, true
	}}
	got := serve(t, context.Background(), c, Options{}, sse(
		":keepalive\n\n"+"\n"+"event: bare\n\n"+"data:stop\n\n"))
	if seen != 1 {
		t.Fatalf("Convert was called %d times, want 1: only the stop frame is an event", seen)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(got), got)
	}
}

// A comment inside a half read event is skipped rather than dispatched, because
// dispatching would drop the event: and data: lines already read.
func TestCommentInsideAnEventKeepsIt(t *testing.T) {
	got := texts(serve(t, context.Background(), echo(nil), Options{},
		sse("event: one\n:keepalive\ndata: payload\n\ndata:stop\n\n")))
	if len(got) != 1 || got[0] != "one|payload" {
		t.Fatalf("got %q, want [one|payload]", got)
	}
}

// A converter that says the turn is over is not asked how it ended, and the
// rest of the body is never read.
func TestConverterEndsTheTurn(t *testing.T) {
	ended := make(chan struct{}, 1)
	c := echo(func(error) []api.Event {
		ended <- struct{}{}
		return errEvent(errors.New("End was called"))
	})
	got := serve(t, context.Background(), c, Options{}, sse("data:stop\n\ndata:after\n\n"))
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(got), got)
	}
	ok0, ok := got[0].(api.EventOk)
	if !ok {
		t.Fatalf("got %T, want EventOk", got[0])
	}
	if ok0.Code != api.CodeTurn {
		t.Fatalf("got %q, want %q", ok0.Code, api.CodeTurn)
	}
	if len(ended) != 0 {
		t.Fatal("End was called after the converter had already ended the turn")
	}
}

// A body that ran out on a frame boundary reaches End as io.EOF, and one cut
// mid event as io.ErrUnexpectedEOF. What either means is the converter's to
// decide.
func TestEndSeesHowTheBodyRanOut(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{"between frames", "data:hello\n\n", io.EOF},
		{"mid event", "data:hello\n", io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(chan error, 1)
			c := echo(func(err error) []api.Event { seen <- err; return nil })
			serve(t, context.Background(), c, Options{}, sse(tc.body))
			if len(seen) == 0 {
				t.Fatal("End was never called")
			}
			if err := <-seen; !errors.Is(err, tc.want) {
				t.Fatalf("End got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFrameTooLarge(t *testing.T) {
	seen := make(chan error, 1)
	c := echo(func(err error) []api.Event { seen <- err; return errEvent(err) })
	serve(t, context.Background(), c, Options{Max: 8}, sse("data:"+strings.Repeat("x", 64)+"\n\n"))
	if err := <-seen; !errors.Is(err, errFrameTooLarge) {
		t.Fatalf("End got %v, want %v", err, errFrameTooLarge)
	}
}

// A response that is not 200 reaches End as raw material: the status, the
// headers and the body, for the adapter to make what it can of.
func TestRefusedRequest(t *testing.T) {
	seen := make(chan error, 1)
	c := echo(func(err error) []api.Event { seen <- err; return errEvent(err) })
	serve(t, context.Background(), c, Options{}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("retry-after", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":"slow down"}`)
	})

	h, ok := (<-seen).(*HTTPError)
	if !ok {
		t.Fatal("End was not handed an *HTTPError")
	}
	if h.Status != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", h.Status)
	}
	if h.Header.Get("retry-after") != "30" {
		t.Fatalf("headers did not come through: %v", h.Header)
	}
	if string(h.Body) != `{"error":"slow down"}` {
		t.Fatalf("body %q", h.Body)
	}
}

func TestIdleFiresOnSilentWire(t *testing.T) {
	seen := make(chan error, 1)
	c := echo(func(err error) []api.Event { seen <- err; return errEvent(err) })
	got := serve(t, context.Background(), c, Options{Idle: 50 * time.Millisecond},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data:a\n\n")
			w.(http.Flusher).Flush()
			stall(r)
		})
	if err := <-seen; !errors.Is(err, ErrIdle) {
		t.Fatalf("End got %v, want %v", err, ErrIdle)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want the frame plus the error: %v", len(got), got)
	}
}

// The wait for the response headers is silence like any other: a peer that
// accepts the request and never answers is exactly what the deadline is for.
func TestIdleCoversTheWaitForHeaders(t *testing.T) {
	seen := make(chan error, 1)
	c := echo(func(err error) []api.Event { seen <- err; return errEvent(err) })
	got := serve(t, context.Background(), c, Options{Idle: 50 * time.Millisecond},
		func(w http.ResponseWriter, r *http.Request) { stall(r) })
	if err := <-seen; !errors.Is(err, ErrIdle) {
		t.Fatalf("End got %v, want %v", err, ErrIdle)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want only the error: %v", len(got), got)
	}
}

// Keepalives are bytes on the wire without being events, and holding the
// deadline open is the whole of what they are for.
func TestKeepalivesHoldTheDeadline(t *testing.T) {
	got := serve(t, context.Background(), echo(nil), Options{Idle: 50 * time.Millisecond},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			for range 6 {
				io.WriteString(w, ":ping\n\n")
				w.(http.Flusher).Flush()
				time.Sleep(20 * time.Millisecond)
			}
			io.WriteString(w, "data:stop\n\n")
		})
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(got), got)
	}
	ok0, ok := got[0].(api.EventOk)
	if !ok {
		t.Fatalf("got %T, want EventOk", got[0])
	}
	if ok0.Code != api.CodeTurn {
		t.Fatalf("got %q, want %q", ok0.Code, api.CodeTurn)
	}
}

// A cancelled turn is never given an ending: the caller walked away, and
// ctx.Err() is the one ending it can work out for itself. What was already
// queued may still arrive, and the caller ignores it.
func TestCancelDeliversNoTerminalEvent(t *testing.T) {
	srv := httptest.NewServer(sse("data:a\n\ndata:b\n\ndata:stop\n\n"))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("{}"))

	ctx, cancel := context.WithCancel(context.Background())
	c := echo(func(error) []api.Event {
		return errEvent(errors.New("a cancelled turn was told something"))
	})

	ch := Stream(ctx, srv.Client(), req, c, Options{})
	<-ch // take one event, then walk away
	cancel()
	for e := range ch {
		if _, ok := e.(api.EventErr); ok {
			t.Fatalf("a cancelled turn delivered %v", e)
		}
	}
}

// Drain is a turn that ended before it began. Nothing blocks on it.
func TestDrain(t *testing.T) {
	ch := Drain(api.EventErr{Code: api.CodeInvalid, Err: errors.New("no model")})
	var got []api.Event
	for e := range ch {
		got = append(got, e)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if len(Drain()) != 0 {
		t.Fatal("Drain with no events produced one")
	}
}
