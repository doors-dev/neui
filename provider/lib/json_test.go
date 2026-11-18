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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"neui.dev/api"
)

// TestMarshalRawKeepsHTMLBytes is the whole reason MarshalRaw exists. A payload
// a provider signed goes back inside the body as the bytes it arrived as, and
// encoding/json rewrites <, > and & into \u00XX escapes inside a RawMessage
// unless it is told not to, which preserves the value and not the bytes.
func TestMarshalRawKeepsHTMLBytes(t *testing.T) {
	payload := json.RawMessage(`{"code":"if a<b && c>d {}"}`)
	body := struct {
		Item json.RawMessage `json:"item"`
	}{payload}
	raw, err := MarshalRaw(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), string(payload)) {
		t.Fatalf("the payload was rewritten:\n got: %s\nwant to contain: %s", raw, payload)
	}
	plain, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), "\\u003c") {
		t.Fatalf("encoding/json stopped escaping the three bytes, so MarshalRaw has nothing left to do: %s", plain)
	}
}

// TestMarshalRawCompactsWhitespace pins the one difference that survives. The
// encoder runs every RawMessage through compact, which drops the whitespace
// between tokens, so the bytes are the payload's own for everything a signature
// covers and not for how it was laid out.
func TestMarshalRawCompactsWhitespace(t *testing.T) {
	raw, err := MarshalRaw(json.RawMessage("{\"a\": 1,\n \"b\": 2}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"a":1,"b":2}` {
		t.Fatalf("got %s, want %s", raw, `{"a":1,"b":2}`)
	}
}

// TestMarshalBodyMatchesMarshalRaw holds the pooled encoding to the bytes
// MarshalRaw produces, including after the pool hands back a buffer a larger
// body left behind.
func TestMarshalBodyMatchesMarshalRaw(t *testing.T) {
	values := []any{
		struct {
			Item json.RawMessage `json:"item"`
			Text string          `json:"text"`
		}{json.RawMessage(`{"code": "if a<b && c>d {}"}`), "<b>  ünï & \"q\"\n"},
		map[string]any{"data": bytes.Repeat([]byte{0, 1, 2, 250}, 1<<18), "n": 3},
		[]int{1, 2, 3},
		"short",
		map[string]any{"data": bytes.Repeat([]byte("x"), 1<<20)},
		nil,
	}
	for range 3 {
		for i, v := range values {
			want, err := MarshalRaw(v)
			if err != nil {
				t.Fatal(err)
			}
			body, err := MarshalBody(v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(body.Bytes(), want) {
				t.Fatalf("value %d: the pooled body differs from MarshalRaw:\n got: %.200s\nwant: %.200s", i, body.Bytes(), want)
			}
			body.release()
		}
	}
	if _, err := MarshalBody(make(chan int)); err == nil {
		t.Fatalf("a value encoding/json refuses was encoded")
	}
}

// TestPooledBodiesUnderConcurrentStreams sends many bodies at once through
// every path that reads a request body: a server that reads it, one that
// answers without reading it, and a 307 redirect that resends it through
// GetBody. Each server must see exactly the bytes encoded, and every body that
// was read in full must be back in the pool when its turn ends. Run it with
// -race: a buffer reused while a transport still reads it shows up there.
func TestPooledBodiesUnderConcurrentStreams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/read", http.StatusTemporaryRedirect)
		case "/early":
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, "slow down")
		default:
			got, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			sum := sha256.Sum256(got)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\ndata:stop\n\n", hex.EncodeToString(sum[:]))
		}
	}))
	defer srv.Close()
	client := srv.Client()
	paths := []string{"/read", "/redirect", "/early"}
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for i := range 12 {
				path := paths[(g+i)%len(paths)]
				payload := map[string]any{"g": g, "i": i, "data": bytes.Repeat([]byte{byte(g), byte(i)}, (g*i*997)%(1<<17)+1)}
				want, err := MarshalRaw(payload)
				if err != nil {
					t.Error(err)
					return
				}
				body, err := MarshalBody(payload)
				if err != nil {
					t.Error(err)
					return
				}
				if !bytes.Equal(body.Bytes(), want) {
					t.Errorf("%s %d/%d: the pooled body differs from MarshalRaw", path, g, i)
					return
				}
				req, err := body.Request(context.Background(), srv.URL+path)
				if err != nil {
					t.Error(err)
					return
				}
				var got []api.Event
				for e := range Stream(context.Background(), client, req, echo(errEvent), Options{}) {
					got = append(got, e)
				}
				events := texts(got)
				if path == "/early" {
					continue
				}
				sum := sha256.Sum256(want)
				if len(events) != 1 || events[0] != "|"+hex.EncodeToString(sum[:]) {
					t.Errorf("%s %d/%d: the server did not receive the encoded bytes: %q", path, g, i, events)
					return
				}
				deadline := time.Now().Add(time.Second)
				for {
					body.mu.Lock()
					released := body.buf == nil
					body.mu.Unlock()
					if released {
						break
					}
					if time.Now().After(deadline) {
						t.Errorf("%s %d/%d: the body never went back to the pool", path, g, i)
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
		})
	}
	wg.Wait()
}

// TestReleasedBodyCannotBeReadOrReopened is what makes the pool safe whatever
// a transport does late: a reader closed before it finished reads nothing
// more, and once the last reference is gone GetBody refuses to open another.
func TestReleasedBodyCannotBeReadOrReopened(t *testing.T) {
	body, err := MarshalBody(map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := body.Request(context.Background(), "http://example.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	if req.ContentLength != int64(len(`{"a":"b"}`)) {
		t.Fatalf("ContentLength = %d, want %d", req.ContentLength, len(`{"a":"b"}`))
	}
	first := make([]byte, 3)
	if n, err := req.Body.Read(first); n != 3 || err != nil {
		t.Fatalf("read %d bytes, %v", n, err)
	}
	again, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := io.ReadAll(again); err != nil || string(got) != `{"a":"b"}` {
		t.Fatalf("GetBody read %q, %v", got, err)
	}
	again.Close()
	req.Body.Close()
	if n, err := req.Body.Read(first); n != 0 || err == nil {
		t.Fatalf("a closed reader read %d bytes, %v", n, err)
	}
	body.release()
	if _, err := req.GetBody(); err == nil {
		t.Fatalf("GetBody opened a body that went back to the pool")
	}
}

func BenchmarkMarshalBody(b *testing.B) {
	payload := map[string]any{"text": strings.Repeat("hello world ", 1<<16), "data": make([]byte, 4<<20)}
	b.Run("raw", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := MarshalRaw(payload); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			body, err := MarshalBody(payload)
			if err != nil {
				b.Fatal(err)
			}
			body.release()
		}
	})
}

func TestLargeBodyIsNotPooled(t *testing.T) {
	body, err := MarshalBody(strings.Repeat("x", maxPooledBody+1))
	if err != nil {
		t.Fatalf("MarshalBody: %v", err)
	}
	buf := body.buf
	body.unref()
	if body.buf != nil {
		t.Fatalf("a released body kept its buffer")
	}
	for range 4 {
		if got := bodies.Get(); got == buf {
			t.Fatalf("a buffer over the cap went back to the pool")
		}
	}
}
