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
