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

package list

import (
	"context"
	"html"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

func greetingParams(disable ...params.Feature) params.Params {
	p := params.Params{}
	p.InitDefault()
	p.Disable(disable)
	return p
}

func TestGreetingOnlyInAnEmptyLog(t *testing.T) {
	ctx := context.Background()
	empty := New(greetingParams(), nil, roundsThread{}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
	if _, err := empty.seed(ctx, nil, api.NoRoundID); err != nil {
		t.Fatal(err)
	}
	if empty.greeting == nil {
		t.Fatalf("an empty log has no greeting")
	}
	busy := New(greetingParams(), nil, roundsThread{rounds: []api.Round{{ID: 0, State: api.RoundOk, Input: api.Input{Text: "hi"}}}}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
	if _, err := busy.seed(ctx, nil, api.NoRoundID); err != nil {
		t.Fatal(err)
	}
	if busy.greeting != nil {
		t.Fatalf("a log with a round shows the greeting")
	}
	off := New(greetingParams(params.Greeting), nil, roundsThread{}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
	if _, err := off.seed(ctx, nil, api.NoRoundID); err != nil {
		t.Fatal(err)
	}
	if off.greeting != nil {
		t.Fatalf("the Greeting feature is off, but the log shows the greeting")
	}
}

func renderGreeting(t *testing.T, title any, body any) string {
	t.Helper()
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return greeting(title, body)
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return html.UnescapeString(string(raw))
}

func TestGreetingParts(t *testing.T) {
	p := greetingParams()
	page := renderGreeting(t, p.GreetingTitle, p.GreetingBody)
	for _, want := range []string{"How can I help?", "Ask a question or describe what you need.", "nechat-greet-body_text"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the default greeting misses %q: %s", want, page)
		}
	}
	page = renderGreeting(t, "", gox.Elem(func(cur gox.Cursor) error {
		return cur.Text("billing <help>")
	}))
	if strings.Contains(page, "nechat-greet-title") {
		t.Fatalf("an empty title still renders: %s", page)
	}
	if !strings.Contains(page, "billing <help>") || strings.Contains(page, "nechat-greet-body_text") {
		t.Fatalf("a template body is missing or capped like text: %s", page)
	}
}
