package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"neui.dev/api"
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

type imageError struct{ api.Image }

func (imageError) Error() string { return "render failed" }

type blocksError []api.UserContent

func (blocksError) Error() string { return "render failed" }

func (b blocksError) Contents() []api.UserContent { return b }

func TestAgentContentError(t *testing.T) {
	image := imageError{api.Image{Data: []byte{1, 2}, Mime: api.ImagePNG}}
	blocks := blocksError{api.ContentText{Text: "a"}, api.ContentText{Text: "b"}}
	m := NewModule("m", "")
	for name, fail := range map[string]error{
		"image":   image,
		"blocks":  blocks,
		"plain":   errors.New("boom"),
		"wrapped": fmt.Errorf("x: %w", image),
	} {
		if _, err := m.Add(name, "", func(ctx context.Context, _ struct{}) (struct{}, error) {
			return struct{}{}, fail
		}); err != nil {
			t.Fatal(err)
		}
	}
	inner, _ := m.Get("image")
	if _, err := m.AddRaw("nested", "", schema.AnyRawSchema, schema.AnyRawSchema, func(ctx context.Context, in json.RawMessage) (any, error) {
		return inner.Dispatched(ctx, in)
	}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]api.UserContents{
		"image":   {image.Content()},
		"blocks":  api.UserContents(blocks),
		"plain":   {api.ContentText{Text: "boom"}},
		"wrapped": {image.Content()},
		"nested":  {image.Content()},
	} {
		tool, _ := m.Get(name)
		content, err := tool.Agent(context.Background(), json.RawMessage(`{}`))
		got := api.NewContentToolResult("id", content, err)
		if !reflect.DeepEqual(got, api.ContentToolResult{ID: "id", Content: want, Error: api.ToolErrRuntime}) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}
