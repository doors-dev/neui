package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"neui.dev/internal/schema"
)

func rawTool(t *testing.T, seen *json.RawMessage) tool {
	t.Helper()
	input, err := schema.NewRawSchema(json.RawMessage(`{"type":"object","properties":{"n":{"type":"number"}},"required":["n"]}`))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModule("raw", "raw")
	if _, err := m.AddRaw(
		"pass",
		"pass",
		input,
		schema.AnyRawSchema,
		func(ctx context.Context, in json.RawMessage) (any, error) {
			*seen = bytes.Clone(in)
			return "ok", nil
		},
	); err != nil {
		t.Fatal(err)
	}
	found, ok := m.Get("pass")
	if !ok {
		t.Fatal("tool not registered")
	}
	return found.(tool)
}

func TestRawToolPassesBytes(t *testing.T) {
	var seen json.RawMessage
	tool := rawTool(t, &seen)
	arg := `{"n":1234567890123456789}`
	if _, err := tool.Agent(context.Background(), json.RawMessage(arg)); err != nil {
		t.Fatalf("raw call: %v", err)
	}
	if string(seen) != arg {
		t.Fatalf("raw call saw %s, sent %s", seen, arg)
	}
	seen = nil
	if _, err := tool.JS(context.Background())([]any{arg}); err != nil {
		t.Fatalf("string call: %v", err)
	}
	if string(seen) != arg {
		t.Fatalf("string call saw %s, sent %s", seen, arg)
	}
}

func TestRawToolValidates(t *testing.T) {
	var seen json.RawMessage
	tool := rawTool(t, &seen)
	if _, err := tool.Agent(context.Background(), json.RawMessage(`{"n":"text"}`)); err == nil {
		t.Fatal("invalid input accepted")
	}
	if _, err := tool.JS(context.Background())([]any{`{}`}); err == nil {
		t.Fatal("input missing a required property accepted")
	}
	if seen != nil {
		t.Fatalf("callback ran with %s", seen)
	}
}
