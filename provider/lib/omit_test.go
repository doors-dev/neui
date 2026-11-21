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

func TestTextMime(t *testing.T) {
	for mime, want := range map[string]bool{
		string(api.DocumentText):     true,
		string(api.DocumentMarkdown): true,
		string(api.DocumentCSV):      true,
		string(api.DocumentHTML):     true,
		string(api.DocumentXML):      true,
		string(api.DocumentJSON):     true,
		string(api.DocumentYAML):     true,
		string(api.DocumentRTF):      true,
		string(api.DocumentPDF):      false,
		string(api.DocumentDOC):      false,
		string(api.DocumentXLS):      false,
		string(api.DocumentDOCX):     false,
		string(api.DocumentXLSX):     false,
		string(api.DocumentPPTX):     false,
		"":                           false,
		"application/xml":            false,
		"text/x-go":                  false,
		" text/plain":                false,
		"Text/Plain":                 false,
	} {
		if got := TextMime(mime); got != want {
			t.Errorf("TextMime(%q) = %v, want %v", mime, got, want)
		}
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
