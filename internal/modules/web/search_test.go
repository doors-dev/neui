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

func TestSearchBudgetAndEmptiness(t *testing.T) {
	backend := &fakeBackend{results: []Result{
		{Title: "first", URL: "https://example.com/1", Content: strings.Repeat("a", 90)},
		{Title: "second", URL: "https://example.com/2", Content: strings.Repeat("b", 40)},
		{Title: "third", URL: "https://example.com/3", Content: strings.Repeat("c", 40), Snippet: "kept"},
	}}
	search, _ := searchTool(t, Config{Search: backend, SearchChars: 100})
	results, err := search(`{"query":"widgets"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Content != strings.Repeat("a", 90) {
		t.Errorf("first result should be admitted whole: %q", results[0].Content)
	}
	if results[1].Content != strings.Repeat("b", 10)+"… (content truncated)" {
		t.Errorf("straddling result wrong: %q", results[1].Content)
	}
	if results[2].Content != "" || results[2].Snippet != "kept" {
		t.Errorf("third result should keep only its snippet: %+v", results[2])
	}
	backend.results = nil
	if _, err := search(`{"query":"widgets"}`); err == nil || err.Error() != "No search results found. Please try a different query." {
		t.Errorf("empty backend: %v", err)
	}
	backend.err = errors.New("429 sign in")
	if _, err := search(`{"query":"widgets"}`); err == nil || !strings.HasPrefix(err.Error(), "the search backend is unavailable: 429 sign in. Use web__fetch") {
		t.Errorf("backend failure: %v", err)
	}
}

func TestSearchRefusedFromScriptButFetchIsNot(t *testing.T) {
	backend := &fakeBackend{results: []Result{{Title: "a", URL: "https://example.com/a"}}}
	_, script := searchTool(t, Config{Search: backend})
	if err := script(`{"query":"widgets"}`); err == nil || err.Error() != "search: the web module's search is not accessible from a script" {
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
