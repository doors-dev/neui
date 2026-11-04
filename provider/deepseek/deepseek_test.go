package deepseek

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
	"neui.dev/internal/testkit/conform"
	"neui.dev/provider/lib"
)

// DeepSeek publishes no raw SSE transcript anywhere, only SDK snippets, so
// every fixture below is written from the documented field names rather than
// recorded off the wire. They are shaped strictly to what the API reference
// lists and carry the fields it says are always present, including the ones
// this adapter drops.

// frame wraps a payload in the SSE framing DeepSeek uses: one data line, blank
// line separator, no event name.
func frame(payload string) string {
	return "data: " + payload + "\n\n"
}

const keepAlive = ": keep-alive\n\n"

func chunkFrame(delta string, finish string) string {
	reason := "null"
	if finish != "" {
		reason = `"` + finish + `"`
	}
	return frame(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,`+
		`"model":"deepseek-v4-pro","system_fingerprint":"fp_a1b2c3",`+
		`"choices":[{"index":0,"delta":%s,"logprobs":null,"finish_reason":%s}],"usage":null}`, delta, reason))
}

const (
	openingFrame = `{"role":"assistant","content":"","reasoning_content":null}`

	usageFrame = `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1785000000,` +
		`"model":"deepseek-v4-pro","choices":[],"usage":{"prompt_tokens":12000,"completion_tokens":950,` +
		`"total_tokens":12950,"prompt_cache_hit_tokens":9000,"prompt_cache_miss_tokens":3000,` +
		`"completion_tokens_details":{"reasoning_tokens":600}}}` + "\n\n"

	doneFrame = "data: [DONE]\n\n"
)

// serve starts a server running h and returns a Provider aimed at it.
func serve(t *testing.T, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	return serveModel(t, "deepseek-v4-pro", h, opts...)
}

// serveModel is serve for the tests that turn on which model was configured,
// since the model is no longer something a request can vary.
func serveModel(t *testing.T, model string, h http.HandlerFunc, opts ...Option) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	base := []Option{WithAPIKey("test-key"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client())}
	return New(Conf{Model: api.Model{Name: model}}, append(base, opts...)...)
}

// stringHandler writes the frames in order, flushing each so the client sees
// them as they are written rather than in one block at the end.
func stringHandler(frames ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flush(w)
		for _, f := range frames {
			io.WriteString(w, f)
			flush(w)
		}
	}
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// drop closes the socket without finishing the response, which is what a
// mid stream disconnect looks like to the client.
func drop(w http.ResponseWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	conn.Close()
}

func collect(t *testing.T, p *Provider, req api.Request) []api.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return drain(t, p.Stream(ctx, req))
}

func drain(t *testing.T, ch <-chan api.Event) []api.Event {
	t.Helper()
	var events []api.Event
	deadline := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-deadline:
			t.Fatal("stream did not close")
		}
	}
}

func turn(text string) api.Request {
	return api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: text}}}},
	}
}

// fold rebuilds the content blocks a stream produced, which is what
// protocol/event.go says the events are edits to. Nothing in this repo folds
// events yet, so a test that checks what replay sends has to do it here.
//
// Text and reasoning only, since those are the two streams this adapter assigns
// an index to; a tool call is assembled by hand where one is needed, because
// discarding an accumulation that never parsed is a rule of its own.
func fold(t *testing.T, events []api.Event) api.Contents {
	t.Helper()
	var model string
	var content api.Contents
	for _, e := range events {
		switch e := e.(type) {
		case api.EventStart:
			// The model the provider is configured with is what a stored block is
			// stamped with, and this event is the only thing that carries it.
			model = e.Model
		case api.EventText:
			if e.Index == len(content) {
				content = append(content, api.ContentText{})
			}
			b := content[e.Index].(api.ContentText)
			b.Text += e.Text
			content[e.Index] = b
		case api.EventThinking:
			if e.Index == len(content) {
				content = append(content, api.ContentReasoning{Model: model, Thoughts: map[int]string{}})
			}
			b := content[e.Index].(api.ContentReasoning)
			b.Thoughts[e.Part] += e.Text
			content[e.Index] = b
		}
	}
	return content
}

// trace renders an event as one comparable line, so a test asserts the whole
// sequence rather than that some event of some kind turned up.
func trace(e api.Event) string {
	switch e := e.(type) {
	case api.EventStart:
		return fmt.Sprintf("start %s %s", e.ID, e.Model)
	case api.EventText:
		return fmt.Sprintf("text %d %q", e.Index, e.Text)
	case api.EventThinking:
		return fmt.Sprintf("thinking %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventSummary:
		// DeepSeek returns the thinking itself and summarizes nothing, so this
		// only shows up here if that ever changes.
		return fmt.Sprintf("summary %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventMeta:
		// DeepSeek stores nothing in Meta, so this is the same kind of witness.
		return fmt.Sprintf("meta %d %s", e.Index, e.Data)
	case api.EventTool:
		return fmt.Sprintf("tool %d id=%q name=%q args=%q", e.Index, e.ID, e.Name, e.Args)
	case api.EventOpaque:
		return fmt.Sprintf("opaque %d %s", e.Index, e.Kind)
	case api.EventClose:
		return fmt.Sprintf("close %d", e.Index)
	case api.EventUsage:
		return fmt.Sprintf("usage in=%d out=%d reasoning=%d read=%d write=%d",
			e.Input, e.Output, e.Reasoning, e.CacheRead, e.CacheWrite)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		return fmt.Sprintf("err %s", e.Code)
	default:
		return fmt.Sprintf("unknown %T", e)
	}
}

func traces(events []api.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = trace(e)
	}
	return out
}

func wantTrace(t *testing.T, events []api.Event, want ...string) {
	t.Helper()
	got := traces(events)
	if len(got) != len(want) {
		t.Fatalf("event sequence:\n got %v\nwant %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("event %d: got %q, want %q\n got %v\nwant %v", i, got[i], want[i], got, want)
		}
	}
}

// Model answers from configuration alone, so it holds before any turn has run:
// a listed model carries its window, a caller's own literal reads back as
// written, and an unstated window reads as unknown rather than as no window.
func TestModel(t *testing.T) {
	if got := New(Conf{Model: V4Pro}).Model(); got != V4Pro || got.Context == 0 {
		t.Fatalf("Model %+v, want %+v", got, V4Pro)
	}
	own := api.Model{Name: "deepseek-v4-pro"}
	if got := New(Conf{Model: own}).Model(); got != own {
		t.Fatalf("Model %+v, want the caller's literal with an unknown window", got)
	}
}

func TestProviderRefusesBytesMeasuredAsFitting(t *testing.T) {
	const declared = 1 << 20
	var sent int
	p := New(
		Conf{Model: api.Model{Name: "deepseek-v4-pro", RequestBytes: declared}},
		WithAPIKey("test-key"),
		WithBaseURL(serverURL(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sent = len(body)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			io.WriteString(w, "payload too large")
		})),
	)
	events := collect(t, p, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "hi"}}}}})
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: a 413 is the request refused by its size (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if !errors.Is(e.Err, api.ErrMeasuredTooLarge) {
		t.Fatalf("error %v does not carry api.ErrMeasuredTooLarge: a caller cannot tell it from a refusal this adapter made itself, and will retry it", e.Err)
	}
	if msg := e.Err.Error(); sent == 0 || !strings.Contains(msg, fmt.Sprint(sent)) || !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q names neither the %d bytes sent nor the %d declared: nothing points at the misconfiguration", msg, sent, declared)
	}
	var carried *APIError
	if !errors.As(e.Err, &carried) || carried.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error %v lost what the API answered", e.Err)
	}
}

func serverURL(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

// A refusal by size against a model that declares no limit is not marked as
// measured. api.ErrMeasuredTooLarge states that this adapter measured the body
// against Model.RequestBytes and sent it only because it fit, which is why a
// caller reading it stops instead of sending less; at a declared zero the check
// in request never ran, so nothing was measured and sending less is the remedy
// that works. The code still says the request was refused by its byte size.
func TestUnmeasuredRequestTooLargeIsNotMarked(t *testing.T) {
	p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, "payload too large")
	})
	events := collect(t, p, turn("hi"))
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: the refusal is still one about bytes (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if errors.Is(e.Err, api.ErrMeasuredTooLarge) {
		t.Fatalf("error %v carries api.ErrMeasuredTooLarge with nothing measured: the caller ends the turn where sending less would have worked", e.Err)
	}
	if msg := e.Err.Error(); strings.Contains(msg, "the 0 declared") {
		t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
	}
}

// An over-length conversation is a 400 here with nothing machine-readable in it,
// so the message is the only discriminator there is, and leaving it unread costs
// the compaction the harness exists to run: the body below is the one the owner
// was answered with, and it was classified CodeInvalid and compacted nothing.
//
// The rows that must stay CodeInvalid are the point of the matcher's shape. Both
// halves of DeepSeek's sentence are required, so a refusal that merely says the
// word context, or merely says what was requested, is not mistaken for one.
func TestContextOverflowIsTheOne400ThatSaysSo(t *testing.T) {
	const overflow = "This model's maximum context length is 1048576 tokens. However, you requested " +
		"1210970 tokens (1210970 in the messages, 0 in the completion). Please reduce the length of the messages or completion."
	tests := []struct {
		name string
		body string
		want api.ErrCode
	}{
		{"the overflow, as the server wrote it", `{"error":{"message":"` + overflow + `","type":"invalid_request_error","code":"invalid_request_error"}}`, api.CodeContextLength},
		{"the same sentence in a body that is not json", overflow, api.CodeContextLength},
		{"an ordinary bad request", `{"error":{"message":"Invalid request body format.","type":"invalid_request_error","code":"invalid_request_error"}}`, api.CodeInvalid},
		{"a refusal that only names the context", `{"error":{"message":"Context caching is not supported for this model.","type":"invalid_request_error"}}`, api.CodeInvalid},
		{"a refusal that only says what was requested", `{"error":{"message":"You requested a tool that does not exist.","type":"invalid_request_error"}}`, api.CodeInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, tc.body)
			})
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantTrace(t, events, "err "+string(tc.want))
		})
	}
}

// A turn whose finish reason arrived and whose socket then died is a turn that
// finished. [DONE] is what this adapter ends a turn on, so Convert never saw the
// ending and End has to deliver it: swallowing the EOF discards a complete reply
// and reads to the caller as a reply that was cut off, which is a retry of a
// turn that already succeeded and was already paid for.
//
// A reason that ends the turn in failure travels the same way. Reporting nothing
// there turns a capped or filtered turn into silence, which the caller reads as
// cut off and retries unchanged.
func TestStreamDisconnectCarriesTheFinishReason(t *testing.T) {
	for _, tc := range []struct {
		finish string
		want   string
	}{
		{"stop", "ok turn"},
		{"tool_calls", "ok turn"},
		{"length", "err capped"},
		{"content_filter", "err refused"},
	} {
		t.Run(tc.finish, func(t *testing.T) {
			p := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, chunkFrame(openingFrame, ""))
				io.WriteString(w, chunkFrame(`{"content":"done"}`, ""))
				io.WriteString(w, chunkFrame(`{}`, tc.finish))
				flush(w)
				drop(w)
			})
			events := collect(t, p, turn("hi"))
			conform.CheckUnframed(t, events)
			wantTrace(t, events,
				"start chatcmpl-1 deepseek-v4-pro",
				`text 0 "done"`,
				tc.want,
			)
		})
	}
}

// The declared model values and the encoder have to agree, because nothing
// downstream can see the disagreement: the image caps drive shaping and the
// replay budget, and an adapter that drops every image while declaring them is
// a harness spending work and tokens shaping attachments that never go out.
func TestModelDeclarationsMatchTheWire(t *testing.T) {
	for _, m := range []api.Model{Flash, V4Pro} {
		if m.Context != 1_048_576 {
			t.Errorf("%s declares a %d token window; the server's own overflow names 1048576", m.Name, m.Context)
		}
		if m.ImagePixels != 0 || m.ImageBytes != 0 || m.ImagesPerRequest != 0 {
			t.Errorf("%s declares image capacity (%d px, %d bytes, %d per request) that encode.go has no path for",
				m.Name, m.ImagePixels, m.ImageBytes, m.ImagesPerRequest)
		}
	}
	p := New(Conf{Model: Flash}, WithAPIKey("k"))
	body, err := p.encode(api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
		api.ContentImage{Data: []byte("\x89PNG"), Mime: "image/png"},
	}}}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	raw, err := lib.MarshalRaw(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), "[image omitted]") {
		t.Fatalf("an image went out as something other than a note: %s", raw)
	}
}

func TestNilHTTPClientIsRefused(t *testing.T) {
	p := &Provider{client: http.DefaultClient}
	if err := WithHTTPClient(nil)(p); err == nil {
		t.Fatalf("WithHTTPClient(nil) was accepted")
	}
	if p.client != http.DefaultClient {
		t.Fatalf("a refused nil client replaced the one already set")
	}
}
