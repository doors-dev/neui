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

// OwnMeta reads Meta in the shared form {"provider":"<name>","data":<payload>}
// that an adapter writes on a reasoning block it can replay. It returns the
// payload, nil when there is none, and true when Meta names provider; for
// empty Meta, Meta in another form or Meta naming another provider it
// returns false, and the adapter skips the block.
func OwnMeta(meta json.RawMessage, provider string) (json.RawMessage, bool) {
	if len(meta) == 0 {
		return nil, false
	}
	var m struct {
		Provider string          `json:"provider"`
		Data     json.RawMessage `json:"data"`
	}
	if json.Unmarshal(meta, &m) != nil || m.Provider != provider {
		return nil, false
	}
	return m.Data, true
}

var bodies = sync.Pool{New: func() any { return new(bytes.Buffer) }}

var errBodyReleased = errors.New("neui: request body already released")

// Body is a request body encoded the way MarshalRaw encodes, into a pooled
// buffer. Hand the request built by Request to Stream, which recycles the
// buffer once the exchange is over; a Body that never reaches Stream needs no
// cleanup.
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

// AppendFields adds fields, members of a JSON object written without braces
// such as "a":1,"b":true, at the end of the object the body holds. A provider
// uses it for fields its host adds to every request. Call it before Request;
// empty fields leave the body as it is.
func (b *Body) AppendFields(fields []byte) {
	if len(fields) == 0 {
		return
	}
	b.buf.Truncate(b.buf.Len() - 1)
	if b.buf.Len() > 1 {
		b.buf.WriteByte(',')
	}
	b.buf.Write(fields)
	b.buf.WriteByte('}')
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
