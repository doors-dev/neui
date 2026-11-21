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
	"maps"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/params"
)

var (
	hiddenMarkup = regexp.MustCompile(`(?s)<script.*?</script>|<svg.*?</svg>`)
	blockStart   = regexp.MustCompile(`<div class="nechat-gut" title="([^"]+)">|<div class="nechat-bubble-text">`)
	markup       = regexp.MustCompile(`<[^>]*>`)
)

func blocks(page string) []string {
	page = hiddenMarkup.ReplaceAllString(html.UnescapeString(page), "")
	page = blockStart.ReplaceAllStringFunc(page, func(start string) string {
		title := blockStart.FindStringSubmatch(start)[1]
		if title == "" {
			title = "Input"
		}
		return "\x00" + title + ": "
	})
	var lines []string
	for _, block := range strings.Split(markup.ReplaceAllString(page, " "), "\x00")[1:] {
		lines = append(lines, strings.Join(strings.Fields(block), " "))
	}
	return lines
}

func renderBlocks(t *testing.T, page func(ctx context.Context) gox.Elem) []string {
	t.Helper()
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return page(ctx)
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return blocks(string(raw))
}

func renderStored(t *testing.T, section []api.Round) []string {
	t.Helper()
	return renderBlocks(t, func(ctx context.Context) gox.Elem {
		return New(params.Params{}, nil, roundsThread{rounds: section}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {}).Main()
	})
}

func stream(ctx context.Context, l List, section []api.Round, waitShown func(api.RoundID)) {
	for _, r := range section {
		l.RoundStart(ctx, api.Round{ID: r.ID, Input: r.Input, PinnedRecords: r.PinnedRecords}, api.NoRoundID)
		waitShown(r.ID)
		for _, index := range slices.Sorted(maps.Keys(r.Response)) {
			switch c := r.Response[index].(type) {
			case api.ContentReasoning:
				if part, ok := c.Summary[0]; ok {
					l.IndexedEvent(ctx, api.EventSummary{Index: index, Text: part})
				}
				if part, ok := c.Thoughts[0]; ok {
					l.IndexedEvent(ctx, api.EventThinking{Index: index, Text: part})
				}
			case api.ContentText:
				l.IndexedEvent(ctx, api.EventText{Index: index, Text: c.Text})
			case api.ContentToolUse:
				l.IndexedEvent(ctx, api.EventTool{Index: index, ID: c.ID, Name: c.Name})
			}
			l.IndexedEvent(ctx, api.EventClose{Index: index})
		}
		l.RoundDone(ctx, r.State, r.Error)
		l.ToolsDone(ctx)
	}
}

func thought(paragraph string) api.ContentReasoning {
	return api.ContentReasoning{Thoughts: api.Indexed[string]{0: paragraph}}
}

func summary(paragraph string) api.ContentReasoning {
	return api.ContentReasoning{Summary: api.Indexed[string]{0: paragraph + "\n\n"}}
}

func toolUse(name string) api.ContentToolUse {
	return api.ContentToolUse{ID: name, Name: name}
}

func text(paragraph string) api.ContentText {
	return api.ContentText{Text: paragraph + "\n\n"}
}

func chainOf(first api.Round, steps int, reason func(string) api.ContentReasoning, last api.Content) []api.Round {
	section := []api.Round{first}
	for step := 2; step <= steps; step++ {
		section = append(section, api.Round{ID: api.RoundID(step - 1), State: api.RoundOk, PinnedRecords: []api.Record{{Kind: "line_updated"}}, Response: api.Indexed[api.Content]{0: reason("step " + strconv.Itoa(step)), 1: toolUse("page__island_append")}})
	}
	return append(section, api.Round{ID: api.RoundID(steps), State: api.RoundOk, PinnedRecords: []api.Record{{Kind: "line_updated"}}, Response: api.Indexed[api.Content]{0: reason("step " + strconv.Itoa(steps+1)), 1: last}})
}

func TestChainFoldsRecordsAndReasoning(t *testing.T) {
	input := api.Input{Text: "raise the lines"}
	for _, c := range []struct {
		name    string
		section []api.Round
		head    []string
		page    []string
	}{
		{
			"a chain without text",
			chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: thought("step 1"), 1: toolUse("page__read")}}, 6, thought, text("done")),
			[]string{"Events: event ×5", "Reasoning: reasoning ×6 step 6", "Tool calls: page ×6"},
			[]string{"Input: raise the lines", "Events: event ×6", "Reasoning: reasoning ×7 step 7", "Tool calls: page ×6", "Text: done"},
		},
		{
			"a chain headed by text, then reasoning, then calls",
			chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: text("I'll raise them one by one."), 1: thought("step 1"), 2: toolUse("page__read")}}, 3, thought, text("done")),
			[]string{"Events: event ×2", "Reasoning: reasoning ×3 step 3", "Tool calls: page ×3"},
			[]string{"Input: raise the lines", "Text: I'll raise them one by one.", "Events: event ×3", "Reasoning: reasoning ×4 step 4", "Tool calls: page ×3", "Text: done"},
		},
		{
			"a chain headed by reasoning between two texts, then calls",
			chainOf(api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: text("The order has 4 lines."), 1: summary("Planning the raise"), 2: text("I'll raise them one by one."), 3: toolUse("page__read")}}, 3, thought, text("done")),
			[]string{"Events: event ×2", "Reasoning: reasoning ×2 step 3", "Tool calls: page ×3"},
			[]string{"Input: raise the lines", "Text: The order has 4 lines.", "Reasoning: reasoning", "Summary: Planning the raise", "Text: I'll raise them one by one.", "Events: event ×3", "Reasoning: reasoning ×3 step 4", "Tool calls: page ×3", "Text: done"},
		},
	} {
		running := c.section[:len(c.section)-1]
		live := renderBlocks(t, func(ctx context.Context) gox.Elem {
			l := New(params.Params{}, nil, roundsThread{}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
			if _, err := l.seed(ctx, nil, api.NoRoundID); err != nil {
				t.Error(err)
				return nil
			}
			stream(ctx, l, running, func(api.RoundID) {})
			return l.lastSection.chips.Main()
		})
		stored := renderBlocks(t, func(ctx context.Context) gox.Elem {
			l := New(params.Params{}, nil, roundsThread{}, stillRewinder{}, "t", nil, api.NoRoundID, func(context.Context, error) {})
			l.ctx = ctx
			_, head := l.intoRounds(running, false)
			return head.Main()
		})
		if !slices.Equal(live, c.head) || !slices.Equal(stored, c.head) {
			t.Fatalf("%s: a running chain heads with %q live and %q from history, want %q", c.name, live, stored, c.head)
		}
		if page := renderStored(t, c.section); !slices.Equal(page, c.page) {
			t.Fatalf("%s: a chain closed by text shows %q, want %q", c.name, page, c.page)
		}
	}
}

func TestSingleRoundAnswerRendersAsBefore(t *testing.T) {
	input := api.Input{Text: "How many lines does the order have?"}
	for _, c := range []struct {
		name  string
		round api.Round
		want  []string
	}{
		{
			"an answer with a summary",
			api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: summary("Counting the lines"), 1: text("The order has 12 lines.")}},
			[]string{"Input: How many lines does the order have?", "Reasoning: reasoning", "Summary: Counting the lines", "Text: The order has 12 lines."},
		},
		{
			"an answer with thoughts and an event",
			api.Round{ID: 0, State: api.RoundOk, Input: input, PinnedRecords: []api.Record{{Kind: "line_updated"}}, Response: api.Indexed[api.Content]{0: thought("The island lists them."), 1: text("The order has 12 lines.")}},
			[]string{"Input: How many lines does the order have?", "Events: event", "Reasoning: reasoning The island lists them.", "Text: The order has 12 lines."},
		},
		{
			"an answer that ends in calls",
			api.Round{ID: 0, State: api.RoundOk, Input: input, Response: api.Indexed[api.Content]{0: text("Let me read the order."), 1: toolUse("page__read"), 2: toolUse("files__read")}},
			[]string{"Input: How many lines does the order have?", "Text: Let me read the order.", "Tool calls: page files"},
		},
	} {
		if page := renderStored(t, []api.Round{c.round}); !slices.Equal(page, c.want) {
			t.Fatalf("%s shows %q, want %q", c.name, page, c.want)
		}
	}
}

func TestReasoningBetweenTextsStaysBetweenThem(t *testing.T) {
	round := api.Round{ID: 0, State: api.RoundOk, Input: api.Input{Text: "Count the lines on both orders."}, Response: api.Indexed[api.Content]{
		0: summary("Counting the order"),
		1: text("The order has 12 lines."),
		2: summary("Counting the draft"),
		3: text("The draft has 9 lines."),
	}}
	want := []string{"Input: Count the lines on both orders.", "Reasoning: reasoning", "Summary: Counting the order", "Text: The order has 12 lines.", "Reasoning: reasoning", "Summary: Counting the draft", "Text: The draft has 9 lines."}
	if page := renderStored(t, []api.Round{round}); !slices.Equal(page, want) {
		t.Fatalf("reasoning between two texts shows %q, want %q", page, want)
	}
}
