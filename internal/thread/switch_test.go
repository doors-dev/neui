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

package thread_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/provider/gpt"
	"neui.dev/provider/lib"
)

func TestProviderSwitchKeepsEarlierMessagesByteForByte(t *testing.T) {
	signature := lib.Meta("claude", json.RawMessage(`"c2lnbmF0dXJlLW9mLWNsYXVkZQ=="`))
	item := json.RawMessage(`{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAAB-sealed-by-gpt"}`)
	claudeCall := testkit.Events(
		api.EventStart{ID: "a1", Model: "claude-haiku-5-5"},
		api.EventThinking{Index: 0, Text: "I should call the probe."},
		api.EventMeta{Index: 0, Data: signature},
		api.EventClose{Index: 0},
		api.EventTool{Index: 1, ID: "toolu_01", Name: "probe__run", Args: `{"q":"a"}`},
		api.EventClose{Index: 1},
		api.EventUsage{Input: 1, Output: 1},
		api.EventOk{Code: api.CodeTurn},
	)
	gptTurn := testkit.Events(
		api.EventStart{ID: "b1", Model: "gpt-5.6-sol"},
		api.EventThinking{Index: 0},
		api.EventMeta{Index: 0, Data: lib.Meta("gpt", item)},
		api.EventClose{Index: 0},
		api.EventText{Index: 1, Text: "gpt answers"},
		api.EventClose{Index: 1},
		api.EventUsage{Input: 1, Output: 1},
		api.EventOk{Code: api.CodeTurn},
	)
	p := testkit.NewProvider(api.Model{}, claudeCall, testkit.Say("claude answers"), gptTurn, testkit.Say("claude again"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{
		common.OptionSystem([]string{"You are the assistant."}),
		probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
			return map[string]any{"echo": input}, nil
		}),
	}})
	c.Send("first")
	c.Wait()
	c.Send("second")
	c.Wait()
	c.Send("third")
	c.Wait()
	requests := p.Requests()
	if len(requests) != 4 {
		t.Fatalf("%d requests, want 4", len(requests))
	}
	claudeA := claudeBodies(t, []api.Request{requests[1], requests[3]})
	_, before, _ := flatten(t, claudeA[0])
	_, after, _ := flatten(t, claudeA[1])
	if len(after) < len(before) {
		t.Fatalf("the later claude request has %d blocks, fewer than the %d of the earlier one", len(after), len(before))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("block %d changed between the two claude sessions:\nwas %s\nnow %s", i, before[i], after[i])
		}
	}
	if !bytes.Contains(claudeA[1], []byte(`"signature":"c2lnbmF0dXJlLW9mLWNsYXVkZQ=="`)) || bytes.Contains(claudeA[1], []byte("sealed-by-gpt")) {
		t.Fatalf("the claude request after the switch does not carry its own thinking alone:\n%s", claudeA[1])
	}
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	b, err := gpt.New(gpt.Conf{Model: gpt.GPT56Sol}, gpt.WithAPIKey("k"), gpt.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	for range b.Stream(context.Background(), requests[3]) {
	}
	if bytes.Contains(body, []byte("c2lnbmF0dXJlLW9mLWNsYXVkZQ")) || bytes.Contains(body, []byte("I should call the probe")) || !bytes.Contains(body, []byte("sealed-by-gpt")) {
		t.Fatalf("the gpt request carries claude's thinking, or lost its own:\n%s", body)
	}
}
