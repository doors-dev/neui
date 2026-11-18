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

package controller

import (
	"context"
	"testing"
)

func TestNamesOnlySearchSkipsOverview(t *testing.T) {
	ctx := context.Background()
	c := New(true, false)
	c.Toggle(ctx)
	c.SearchInput(ctx, "plan")
	if got := c.state.Get().searchKind; got != SearchNames {
		t.Fatalf("typing went to %v, want the names list", got)
	}
	c.Toggle(ctx)
	s := c.state.Get()
	if s.searchKind != searchNone || s.searchInput != "" || s.searchInputReset != 1 || !s.expanded {
		t.Fatalf("back did not clear the search to the thread list: %+v", s)
	}
	c.SearchInput(ctx, "plan")
	c.SearchInput(ctx, "")
	if got := c.state.Get().searchKind; got != searchNone {
		t.Fatalf("clearing the input left %v, want the thread list", got)
	}
}

func TestFullSearchKeepsOverview(t *testing.T) {
	ctx := context.Background()
	c := New(false, false)
	c.Toggle(ctx)
	c.SearchInput(ctx, "plan")
	if got := c.state.Get().searchKind; got != searchAll {
		t.Fatalf("typing went to %v, want the overview", got)
	}
	c.state.Mutate(ctx, func(s state) state {
		s.searchKind = SearchNames
		return s
	})
	c.Toggle(ctx)
	if got := c.state.Get().searchKind; got != searchAll {
		t.Fatalf("back from names went to %v, want the overview", got)
	}
}
