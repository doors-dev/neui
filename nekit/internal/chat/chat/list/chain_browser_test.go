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

//go:build browser

package list

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/go-rod/rod"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit/internal/chat/params"
)

func openLive(t *testing.T) (List, *rod.Page, context.Context) {
	t.Helper()
	l := New(params.Params{}, nil, roundsThread{}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
	rendered := make(chan context.Context, 1)
	c := stack.Browser(t, stack.Host{Provider: testkit.NewProvider(api.Model{}), Page: func(cur gox.Cursor) error {
		select {
		case rendered <- doors.InstanceContext(cur.Context()):
		default:
		}
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "nechat-log"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Any(l.Main()); err != nil {
			return err
		}
		return cur.Close()
	}})
	ctx := <-rendered
	page := c.Page.Timeout(time.Minute)
	page.MustWaitLoad()
	return l, page, ctx
}

func waitTurn(t *testing.T, page *rod.Page) func(api.RoundID) {
	return func(id api.RoundID) {
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if page.MustEval(`() => document.querySelectorAll(".nechat-turn").length`).Int() > int(id) {
				return
			}
		}
		t.Fatalf("round %d never showed", id)
	}
}

func settle(page *rod.Page, want []string) ([]string, bool) {
	var shown []string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		shown = blocks(page.MustEval(`() => document.querySelector(".nechat-log").innerHTML`).Str())
		if slices.Equal(shown, want) {
			return shown, true
		}
	}
	return shown, false
}

func TestLiveRoundsFoldAsTheyReload(t *testing.T) {
	input := api.Input{Text: "raise the lines"}
	for _, c := range []struct {
		name    string
		section []api.Round
	}{
		{"a chain without text", chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: summary("step 1"), 1: toolUse("page__read")}}, 6, summary, text("done"))},
		{"a chain still running", chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: thought("step 1"), 1: toolUse("page__read")}}, 6, thought, text("done"))[:5]},
		{"a chain headed by text, then reasoning, then calls", chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: text("I'll raise them one by one."), 1: summary("step 1"), 2: toolUse("page__read")}}, 4, summary, text("done"))},
		{"a chain headed by reasoning between two texts", chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: text("The order has 4 lines."), 1: summary("Planning the raise"), 2: text("I'll raise them one by one."), 3: toolUse("page__read")}}, 4, thought, text("done"))},
		{"an answer with reasoning between two texts", []api.Round{{ID: 0, State: api.RoundOk, Input: input, PinnedRecords: []api.Record{{Kind: "line_updated"}}, Response: api.Indexed[api.Content]{0: summary("Counting the order"), 1: text("The order has 12 lines."), 2: thought("The draft is shorter."), 3: text("The draft has 9 lines.")}}}},
	} {
		want := renderStored(t, c.section)
		l, page, ctx := openLive(t)
		stream(ctx, l, c.section, waitTurn(t, page))
		if shown, ok := settle(page, want); !ok {
			t.Fatalf("%s: live shows %q, a reload shows %q", c.name, shown, want)
		}
	}
}

func TestLiveSummaryRowSwapsInPlace(t *testing.T) {
	l, page, ctx := openLive(t)
	stream(ctx, l, []api.Round{{ID: 0, State: api.RoundOk, Input: api.Input{Text: "raise the lines"}, Response: api.Indexed[api.Content]{0: summary("Reading the island"), 1: toolUse("page__read")}}}, waitTurn(t, page))
	first := []string{"Input: raise the lines", "Reasoning: reasoning", "Tool calls: page", "Summary: Reading the island"}
	if shown, ok := settle(page, first); !ok {
		t.Fatalf("the first round shows %q, want %q", shown, first)
	}
	page.MustEval(`() => { window.summaryRow = document.querySelector(".nechat-summary").closest(".nechat-blk") }`)
	l.RoundStart(ctx, api.Round{ID: 1}, api.NoRoundID)
	waitTurn(t, page)(1)
	l.IndexedEvent(ctx, api.EventSummary{Index: 0, Text: "Raising line 3\n\n"})
	streaming := []string{"Input: raise the lines", "Reasoning: reasoning ×2", "Tool calls: page", "Summary: Reading the island"}
	if shown, ok := settle(page, streaming); !ok {
		t.Fatalf("while the next block streams the chain shows %q, want %q", shown, streaming)
	}
	l.IndexedEvent(ctx, api.EventClose{Index: 0})
	closed := []string{"Input: raise the lines", "Reasoning: reasoning ×2", "Tool calls: page", "Summary: Raising line 3"}
	if shown, ok := settle(page, closed); !ok {
		t.Fatalf("once the next block closes the chain shows %q, want %q", shown, closed)
	}
	if !page.MustEval(`() => document.querySelector(".nechat-summary").closest(".nechat-blk") === window.summaryRow`).Bool() {
		t.Fatal("the summary row was rebuilt instead of taking the new summary in place")
	}
}
