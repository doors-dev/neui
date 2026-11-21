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

	"neui.dev/internal/tools"
)

func searchDesc(light bool) string {
	desc := `Search the web and get back links with short extracts, not whole pages — fetch a result with web__fetch to read it. Results come from one backend the host configured, so it has one index and one ranking, and asking twice with different words is often better than asking for more results.`
	if !light {
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
	Title     string `json:"title" description:"the backend's title for the page"`
	URL       string `json:"url" description:"fetch it with web__fetch to read the page"`
	Snippet   string `json:"snippet,omitempty" description:"the backend's own one- or two-sentence extract; empty when it returns content instead"`
	Content   string `json:"content,omitempty" description:"passages from the page when the backend returns them, capped per result; ends with (content truncated) when cut. web__fetch the url for the rest."`
	Published string `json:"published,omitempty" description:"publication date the backend reports; empty when it reports none"`
}

type SearchArg struct {
	Query   string   `json:"query" required:"true" description:"what to search for, as words rather than a URL; include the year when you want recent material"`
	Num     uint     `json:"num" description:"results to return, default 8, at most 20"`
	Domains []string `json:"domains" description:"hostnames to restrict results to, e.g. go.dev; some backends cannot filter on this and ignore it, so do not read an empty result as no such page existing"`
	Recent  string   `json:"recent" enum:"day,week,month,year" description:"only pages published within this window; some backends cannot filter on this and ignore it"`
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
