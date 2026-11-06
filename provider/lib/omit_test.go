package lib

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"neui.dev/api"
)

type recorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *recorder) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	return nil
}

func (r *recorder) WithAttrs([]slog.Attr) slog.Handler { return r }

func (r *recorder) WithGroup(string) slog.Handler { return r }

func (r *recorder) levels() []slog.Level {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]slog.Level, len(r.records))
	for i, rec := range r.records {
		out[i] = rec.Level
	}
	return out
}

func record(t *testing.T) *recorder {
	t.Helper()
	r := &recorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(r))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return r
}

func TestOmittedLogsNothing(t *testing.T) {
	r := record(t)
	tests := []struct {
		c    api.Content
		want string
	}{
		{api.ContentImage{}, "[image omitted]"},
		{api.ContentDocument{}, "[document omitted]"},
		{api.ContentReasoning{}, "[reasoning omitted]"},
		{api.ContentOpaque{}, "[content omitted]"},
		{api.ContentToolResult{}, "[content omitted]"},
	}
	for _, tc := range tests {
		if got := Omitted(tc.c, "why"); got != tc.want {
			t.Fatalf("Omitted(%T) = %q, want %q", tc.c, got, tc.want)
		}
	}
	if got := r.levels(); len(got) != 0 {
		t.Fatalf("Omitted logged %v, want nothing", got)
	}
}

func TestUnknownLogsOncePerProviderAndType(t *testing.T) {
	unknownSeen.Clear()
	r := record(t)
	if got := Unknown("test-unknown", 0, api.ContentText{}); got != "[content omitted]" {
		t.Fatalf("Unknown = %q, want [content omitted]", got)
	}
	Unknown("test-unknown", 3, api.ContentText{})
	if got := r.levels(); len(got) != 1 || got[0] != slog.LevelError {
		t.Fatalf("two calls with one type logged %v, want one Error", got)
	}
	Unknown("test-unknown", 0, api.ContentImage{})
	Unknown("test-unknown-other", 0, api.ContentText{})
	if got := r.levels(); len(got) != 3 || got[1] != slog.LevelError || got[2] != slog.LevelError {
		t.Fatalf("a new type and a new provider logged %v, want one more Error each", got)
	}
}
