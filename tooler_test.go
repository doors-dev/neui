package neui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/island"
)

func TestTooler(t *testing.T) {
	search := Tool[string, int]{Name: "search", Desc: "find", Handler: func(ctx context.Context, q string) (int, error) { return len(q), nil }}
	m := NewModule("shop", "")
	if id, err := m.Add(search); err != nil || id != 1 {
		t.Fatalf("add: %v %v", id, err)
	}
	if id, err := m.Tool("other", "", search.Handler); err != nil || id != 2 {
		t.Fatalf("shorthand: %v %v", id, err)
	}
	ask := Ask[string, string]{Name: "pick", Desc: "choose", Handler: func(string, Answer[string]) api.ChatAsk { return api.ChatAsk{} }}
	if _, err := m.Add(ask); err != nil {
		t.Fatal(err)
	}
	tool, ok := m.Get("pick")
	if !ok || !strings.HasSuffix(tool.Desc(), "do not restate it in your reply.") {
		t.Fatalf("ask desc: %v", ok)
	}
	l := Library{inner: island.NewLibrary()}
	if err := l.Add(search); err != nil {
		t.Fatal(err)
	}
	if err := l.Tool("search", "", search.Handler); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := l.Add(ask); !errors.Is(err, api.ErrAskInIsland) {
		t.Fatalf("ask in island: %v", err)
	}
}
