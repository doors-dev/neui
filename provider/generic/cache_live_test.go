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

//go:build live

package generic

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

type streamTee struct {
	n int
}

func (s *streamTee) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	s.n++
	if err != nil || resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return resp, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, os.WriteFile(filepath.Join("testdata", "streams", fmt.Sprintf("openrouter-qwen-cache-%d.sse", s.n)), data, 0o644)
}

func TestLiveCacheRead(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}
	client := &http.Client{}
	if f := flag.Lookup("record"); f != nil && f.Value.String() == "true" {
		client.Transport = &streamTee{}
	}
	p, err := New(Conf{Model: api.Model{Name: "qwen/qwen3.8-27b"}, BaseURL: "https://openrouter.ai/api/v1"},
		WithAPIKey(key), WithHTTPClient(client), WithEffort(EffortNone), WithMaxTokens(16),
		WithExtraBody(map[string]any{"provider": map[string]any{"order": []string{"wafer"}, "allow_fallbacks": false}}))
	if err != nil {
		t.Fatal(err)
	}
	var system strings.Builder
	fmt.Fprintf(&system, "Ledger run %d.\n", time.Now().UnixNano())
	for i := range 360 {
		fmt.Fprintf(&system, "Record %04d: the ledger keeps entry %d of %d in its original order.\n", i, i*7, 360*7)
	}
	req := api.Request{System: system.String(), Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "Reply with the single word ok."}}}}}
	var usage [2]api.EventUsage
	for i := range usage {
		if i > 0 {
			time.Sleep(5 * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		var events []api.Event
		for e := range p.Stream(ctx, req) {
			events = append(events, e)
			if u, ok := e.(api.EventUsage); ok {
				usage[i] = u
			}
		}
		cancel()
		if len(events) == 0 {
			t.Fatalf("request %d: the stream closed with no event", i+1)
		}
		if _, ok := events[len(events)-1].(api.EventOk); !ok {
			t.Fatalf("request %d: %v", i+1, conform.DescribeAll(events))
		}
		t.Logf("request %d: %s", i+1, conform.Describe(usage[i]))
	}
	if prompt := usage[0].Input + usage[0].CacheRead + usage[0].CacheWrite; prompt < 8_000 || prompt > 14_000 {
		t.Fatalf("the first request took %d prompt tokens, want about 10k", prompt)
	}
	if usage[1].CacheRead == 0 {
		t.Fatalf("the second identical request read nothing from the cache: %s", conform.Describe(usage[1]))
	}
}
