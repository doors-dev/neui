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

package rower

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

func renderChips(t *testing.T, c Chips) string {
	t.Helper()
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return c.Main()
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

func TestEventChipText(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		records []int
		want    string
	}{
		{[]int{1}, `<div class="nechat-col nechat-events">event </div>`},
		{[]int{1, 2}, `<div class="nechat-col nechat-events">event <span class="nechat-tool-count">×3</span></div>`},
	} {
		chips := NewChips(params.Params{}, nil, false)
		for _, count := range c.records {
			chips.Records(ctx, count)
		}
		page := renderChips(t, chips)
		if !strings.Contains(page, `<div class="nechat-gut" title="Events">`) || !strings.Contains(page, c.want) {
			t.Fatalf("records %v do not show the Events chip %q: %s", c.records, c.want, page)
		}
	}
}

func TestSummaryRowKeepsTheLatestSummary(t *testing.T) {
	ctx := context.Background()
	first, second := "Reading the island", "Raising line 3"
	stream := []api.IndexedEvent{
		api.EventSummary{Index: 0, Text: first + "\n\n"},
		api.EventClose{Index: 0},
		api.EventSummary{Index: 2, Text: second + "\n\n"},
		api.EventClose{Index: 2},
	}
	for _, c := range []struct {
		name   string
		events int
		shown  string
		gone   string
	}{
		{"the first summary while it streams", 1, first, second},
		{"a later summary while its block streams", 3, first, second},
		{"a later summary once its block closes", 4, second, first},
	} {
		chips := NewChips(params.Params{}, nil, false)
		for _, e := range stream[:c.events] {
			chips.Reason(ctx, e)
		}
		page := renderChips(t, chips)
		if rows := strings.Count(page, `title="Summary"`); rows != 1 {
			t.Fatalf("%s: %d summary rows, want one: %s", c.name, rows, page)
		}
		if !strings.Contains(page, "<p>"+c.shown+"</p>") || strings.Contains(page, c.gone) {
			t.Fatalf("%s: the summary row does not show %q alone: %s", c.name, c.shown, page)
		}
	}
}
