package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
)

// MarshalRaw is json.Marshal without HTML escaping: <, > and & stay as they
// are, including inside json.RawMessage values, which json.Marshal would
// rewrite as \u escapes. Raw messages are still compacted. There is no
// trailing newline.
func MarshalRaw(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

var bodies = sync.Pool{New: func() any { return new(bytes.Buffer) }}

var errBodyReleased = errors.New("neui: request body already released")

// Body is a request body encoded the way MarshalRaw encodes, into a buffer
// taken from a pool. Once the request built by Request has gone through Stream,
// the buffer goes back to the pool only after the response body is closed and
// the transport has closed every reader it opened on it, so no redirect, retry
// or late body write can read it after. A reader the transport never closes
// leaves the buffer to the garbage collector instead, as does a Body that never
// reaches Stream. A buffer that grew past 4 MiB is never pooled.
type Body struct {
	mu   sync.Mutex
	buf  *bytes.Buffer
	refs int
}

// MarshalBody encodes v as MarshalRaw does, into a pooled buffer.
func MarshalBody(v any) (*Body, error) {
	buf := bodies.Get().(*bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		buf.Reset()
		bodies.Put(buf)
		return nil, err
	}
	buf.Truncate(len(bytes.TrimRight(buf.Bytes(), "\n")))
	return &Body{buf: buf, refs: 1}, nil
}

// Bytes returns the encoded body. It is valid until the request is handed to
// Stream.
func (b *Body) Bytes() []byte {
	return b.buf.Bytes()
}

// Request builds a POST of the body to url. Its GetBody opens the same bytes
// again, as http.NewRequest does for a bytes.Reader, so redirects and transport
// retries resend it unchanged.
func (b *Body) Request(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	body, err := b.open()
	if err != nil {
		return nil, err
	}
	req.Body = body
	req.GetBody = b.open
	req.ContentLength = int64(b.buf.Len())
	return req, nil
}

func (b *Body) open() (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf == nil {
		return nil, errBodyReleased
	}
	b.refs++
	return &bodyReader{body: b, r: bytes.NewReader(b.buf.Bytes())}, nil
}

func (b *Body) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unref()
}

const maxPooledBody = 4 << 20

func (b *Body) unref() {
	b.refs--
	if b.refs > 0 {
		return
	}
	if b.buf.Cap() <= maxPooledBody {
		b.buf.Reset()
		bodies.Put(b.buf)
	}
	b.buf = nil
}

type bodyReader struct {
	body   *Body
	r      *bytes.Reader
	closed bool
}

func (r *bodyReader) Read(p []byte) (int, error) {
	r.body.mu.Lock()
	defer r.body.mu.Unlock()
	if r.closed {
		return 0, http.ErrBodyReadAfterClose
	}
	return r.r.Read(p)
}

func (r *bodyReader) Close() error {
	r.body.mu.Lock()
	defer r.body.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	r.body.unref()
	return nil
}
