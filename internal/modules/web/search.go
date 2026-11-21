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
	"errors"
	"fmt"
	"time"

	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

func searchDesc(tier common.ModelTier) string {
	desc := `Search the web. Results are links with short extracts, not whole pages: read one with web__fetch. When the results miss, ask again with different words rather than for more results; an empty result does not mean no such page exists. The search engine may ignore domains and recent.`
	if tier != common.TierAssistant {
		desc += ` Not accessible from a script.`
	}
	return desc
}

type Query struct {
	Text    string
	Num     int
	Domains []string
	Since   time.Time
}

type SearchBackend interface {
	Search(ctx context.Context, q Query) ([]Result, error)
}

type Result struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet,omitempty"`
	Content   string `json:"content,omitempty"`
	Published string `json:"published,omitempty" description:"as the search engine writes it; absent when unknown"`
}

type SearchArg struct {
	Query   string   `json:"query" required:"true" description:"words, not a URL; include the year when you want recent material"`
	Num     uint     `json:"num" description:"default 8, at most 20"`
	Domains []string `json:"domains" description:"hostnames to restrict results to, like go.dev"`
	Recent  string   `json:"recent" enum:"day,week,month,year" description:"only pages published within it, counted back from now"`
}

func (m *module) search(ctx context.Context, arg SearchArg) ([]Result, error) {
	if tools.FromJS(ctx) {
		return nil, errors.New("the web module's search is not accessible from a script")
	}
	num := min(arg.Num, 20)
	if num == 0 {
		num = 8
	}
	q := Query{Text: arg.Query, Num: int(num), Domains: arg.Domains}
	switch arg.Recent {
	case "day":
		q.Since = time.Now().Add(-24 * time.Hour)
	case "week":
		q.Since = time.Now().Add(-7 * 24 * time.Hour)
	case "month":
		q.Since = time.Now().Add(-30 * 24 * time.Hour)
	case "year":
		q.Since = time.Now().Add(-365 * 24 * time.Hour)
	}
	ctx, cancel := context.WithTimeout(ctx, m.conf.Timeout)
	defer cancel()
	results, err := m.backend.Search(ctx, q)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, fmt.Errorf("the search backend is unavailable: %s. Use web__fetch on a URL you already know instead of retrying.", err)
	}
	if len(results) == 0 {
		return []Result{}, nil
	}
	results = results[:min(len(results), q.Num)]
	for i := range results {
		if runes := []rune(results[i].Content); len(runes) > m.conf.SearchChars {
			results[i].Content = string(runes[:m.conf.SearchChars]) + "… (content truncated)"
		}
	}
	return results, nil
}
