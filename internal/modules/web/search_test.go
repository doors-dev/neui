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

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
)

type fakeBackend struct {
	seen    Query
	results []Result
	err     error
}

func (f *fakeBackend) Search(_ context.Context, q Query) ([]Result, error) {
	f.seen = q
	return f.results, f.err
}

func searchTool(t *testing.T, c Config) (func(arg string) ([]Result, error), func(arg string) error) {
	t.Helper()
	m := Module(c)(context.Background())
	tool, ok := m.Get("search")
	if !ok {
		t.Fatal("web module registered no search tool")
	}
	agent := func(arg string) ([]Result, error) {
		out, err := tool.Agent(context.Background(), json.RawMessage(arg))
		if err != nil {
			return nil, err
		}
		var results []Result
		if err := json.Unmarshal([]byte(out[0].(api.ContentText).Text), &results); err != nil {
			t.Fatalf("result is not a JSON list: %v", err)
		}
		return results, nil
	}
	script := func(arg string) error {
		_, err := tool.JS(context.Background())([]any{json.RawMessage(arg)})
		return err
	}
	return agent, script
}

func TestSearchDefaultsAndFilters(t *testing.T) {
	backend := &fakeBackend{results: []Result{{Title: "a", URL: "https://example.com/a"}}}
	search, _ := searchTool(t, Config{Search: backend})
	if _, err := search(`{"query":"widgets"}`); err != nil {
		t.Fatal(err)
	}
	if backend.seen.Num != 8 || !backend.seen.Since.IsZero() || backend.seen.Domains != nil {
		t.Errorf("defaults wrong: %+v", backend.seen)
	}
	if _, err := search(`{"query":"widgets","num":99,"domains":["go.dev"],"recent":"week"}`); err != nil {
		t.Fatal(err)
	}
	if backend.seen.Num != 20 {
		t.Errorf("num must clamp to 20, got %d", backend.seen.Num)
	}
	if len(backend.seen.Domains) != 1 || backend.seen.Domains[0] != "go.dev" {
		t.Errorf("domains not passed: %+v", backend.seen.Domains)
	}
	if age := time.Since(backend.seen.Since); age < 167*time.Hour || age > 169*time.Hour {
		t.Errorf("recent=week resolved to %s ago", age)
	}
}

func TestSearchCapsEachResultAndTrimsToNum(t *testing.T) {
	backend := &fakeBackend{results: []Result{
		{Title: "first", URL: "https://example.com/1", Content: strings.Repeat("a", 90)},
		{Title: "second", URL: "https://example.com/2", Content: strings.Repeat("b", 50)},
		{Title: "third", URL: "https://example.com/3", Content: strings.Repeat("c", 40), Snippet: "kept"},
		{Title: "fourth", URL: "https://example.com/4", Snippet: "only a snippet"},
		{Title: "fifth", URL: "https://example.com/5", Content: strings.Repeat("漢", 80)},
	}}
	search, _ := searchTool(t, Config{Search: backend, SearchChars: 50})
	results, err := search(`{"query":"widgets"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Content != strings.Repeat("a", 50)+"… (content truncated)" {
		t.Errorf("a long result must be cut at the per-result cap: %q", results[0].Content)
	}
	if results[1].Content != strings.Repeat("b", 50) {
		t.Errorf("a result exactly at the cap must stay whole and unmarked: %q", results[1].Content)
	}
	if results[2].Content != strings.Repeat("c", 40) || results[2].Snippet != "kept" {
		t.Errorf("a later result must keep its content, earlier ones spend no shared budget: %+v", results[2])
	}
	if results[3].Content != "" || results[3].Snippet != "only a snippet" {
		t.Errorf("a snippet-only result changed: %+v", results[3])
	}
	if results[4].Content != strings.Repeat("漢", 50)+"… (content truncated)" {
		t.Errorf("the cap must count characters, not bytes: %q", results[4].Content)
	}
	trimmed, err := search(`{"query":"widgets","num":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(trimmed) != 2 || trimmed[1].Title != "second" {
		t.Errorf("a backend returning more than num must be trimmed to num, best first: %+v", trimmed)
	}
	backend.results = nil
	empty, err := search(`{"query":"widgets"}`)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("an empty backend result must be [] with no error: %v %v", empty, err)
	}
	backend.err = errors.New("429 sign in")
	if _, err := search(`{"query":"widgets"}`); err == nil || !strings.HasPrefix(err.Error(), "the search backend is unavailable: 429 sign in. Use web__fetch") {
		t.Errorf("backend failure: %v", err)
	}
}

func TestSearchCharsDefault(t *testing.T) {
	c := Config{}
	c.initDefaults()
	if c.SearchChars != 2000 {
		t.Errorf("SearchChars defaults to %d, want 2000 per result", c.SearchChars)
	}
}

func TestSearchRefusedFromScriptButFetchIsNot(t *testing.T) {
	backend := &fakeBackend{results: []Result{{Title: "a", URL: "https://example.com/a"}}}
	_, script := searchTool(t, Config{Search: backend})
	if err := script(`{"query":"widgets"}`); err == nil || err.Error() != "the web module's search is not accessible from a script" {
		t.Errorf("search from a script: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "reachable from a script")
	}))
	defer srv.Close()
	m := Module(Config{HTTPClient: srv.Client(), Search: backend})(context.Background())
	fetch, ok := m.Get("fetch")
	if !ok {
		t.Fatal("no fetch tool")
	}
	out, err := fetch.JS(context.Background())([]any{json.RawMessage(fmt.Sprintf(`{"url":%q}`, srv.URL))})
	if err != nil {
		t.Fatalf("fetch must stay available to a script: %v", err)
	}
	if !strings.Contains(fmt.Sprint(out), "reachable from a script") {
		t.Errorf("script fetch returned %v", out)
	}
}

func TestSearchForeignCancelIsUnavailable(t *testing.T) {
	search, _ := searchTool(t, Config{Search: &fakeBackend{err: context.Canceled}})
	if _, err := search(`{"query":"widgets"}`); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("a backend's own cancellation must be reported as a failure: %v", err)
	}
}

func TestSearchDescNamesScriptsOutsideLightMode(t *testing.T) {
	factory := Module(Config{Search: &fakeBackend{}})
	for _, light := range []bool{false, true} {
		search, ok := factory(context.WithValue(context.Background(), common.LightModeKey{}, light)).Get("search")
		if !ok {
			t.Fatal("no search tool")
		}
		if got := strings.Contains(search.Desc(), "script"); got == light {
			t.Fatalf("light %v: the search description reads %q", light, search.Desc())
		}
	}
}
