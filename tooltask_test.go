package neui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolTaskRefusedFromScript(t *testing.T) {
	m := NewModule("jobs", "")
	if _, err := m.Tool("start", "", func(ctx context.Context, _ struct{}) (string, error) {
		_, err := ToolTask(ctx, "work", nil, func(error) {})
		return "", err
	}); err != nil {
		t.Fatal(err)
	}
	tool, ok := m.Get("start")
	if !ok {
		t.Fatal("no start tool")
	}
	_, err := tool.JS(context.Background())([]any{json.RawMessage(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "a background task cannot be started from a script") {
		t.Fatalf("ToolTask from a script: %v", err)
	}
}
