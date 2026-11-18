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

package conform

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"neui.dev/api"
)

type reply struct {
	status int
	header http.Header
	body   string
}

type server struct {
	srv     *httptest.Server
	mu      sync.Mutex
	bodies  [][]byte
	headers []http.Header
	next    reply
}

func newServer(t *testing.T) *server {
	s := &server{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, body)
		s.headers = append(s.headers, r.Header.Clone())
		rep := s.next
		s.mu.Unlock()
		for k, vs := range rep.header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		if rep.status == 0 || rep.status == http.StatusOK {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rep.status)
		}
		io.WriteString(w, rep.body)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *server) reset(rep reply) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bodies = nil
	s.headers = nil
	s.next = rep
}

func (s *server) hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

func (s *server) last() ([]byte, http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bodies) == 0 {
		return nil, nil
	}
	return s.bodies[len(s.bodies)-1], s.headers[len(s.headers)-1]
}

func collect(t *testing.T, p api.Provider, req api.Request) []api.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var events []api.Event
	for e := range p.Stream(ctx, req) {
		events = append(events, e)
	}
	if ctx.Err() != nil {
		t.Fatalf("stream did not end within 10s: %v", DescribeAll(events))
	}
	return events
}

func frames(sse string) [][]byte {
	var out [][]byte
	var data []string
	flush := func() {
		if len(data) > 0 {
			out = append(out, []byte(strings.Join(data, "\n")))
			data = nil
		}
	}
	sc := bufio.NewScanner(strings.NewReader(sse))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	flush()
	return out
}
