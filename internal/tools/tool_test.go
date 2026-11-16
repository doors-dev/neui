package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
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
		"empty":   blocksError{},
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
		return inner.Agent(ctx, in)
	}); err != nil {
		t.Fatal(err)
	}
	failed := api.ContentText{Text: "render failed"}
	for name, want := range map[string]api.UserContents{
		"image":   {failed, image.Content()},
		"blocks":  {failed, blocks[0], blocks[1]},
		"empty":   {failed},
		"plain":   {api.ContentText{Text: "boom"}},
		"wrapped": {api.ContentText{Text: "x: render failed"}, image.Content()},
		"nested":  {failed, image.Content()},
	} {
		tool, _ := m.Get(name)
		content, err := tool.Agent(context.Background(), json.RawMessage(`{}`))
		got := api.NewContentToolResult("id", content, err)
		if !reflect.DeepEqual(got, api.ContentToolResult{ID: "id", Content: want, Error: api.ToolErrRuntime}) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

type label string

func (l label) String() string { return string(l) }

type parts struct{}

func (parts) Contents() []api.UserContent { return nil }

func TestToolNotesHowResultsArrive(t *testing.T) {
	m := NewModule("m", "")
	_, err1 := m.Add("image", "", func(context.Context, struct{}) (api.Image, error) { return api.Image{}, nil })
	_, err2 := m.Add("document", "", func(context.Context, struct{}) (api.Document, error) { return api.Document{}, nil })
	_, err3 := m.Add("label", "", func(context.Context, struct{}) (label, error) { return "", nil })
	_, err4 := m.Add("parts", "", func(context.Context, struct{}) (parts, error) { return parts{}, nil })
	_, err5 := m.Add("string", "", func(context.Context, struct{}) (string, error) { return "", nil })
	if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"image":    "\nOn a direct tool call the result arrives as an image block",
		"document": "\nOn a direct tool call the result arrives as a document block",
		"label":    "\nOn a direct tool call the result arrives as a text block",
		"parts":    "\nOn a direct tool call the result arrives as content blocks",
		"string":   "",
	} {
		tool, _ := m.Get(name)
		_, note, _ := strings.Cut(tool.Summary(), "\nOn a direct")
		if note != strings.TrimPrefix(want, "\nOn a direct") {
			t.Errorf("%s: summary %q, want it to end with %q", name, tool.Summary(), want)
		}
	}
}

func TestRawToolNotesBlocks(t *testing.T) {
	m := NewModule("m", "")
	for name, output := range map[string]schema.Schema[any]{"blocks": schema.BlocksRawSchema, "any": schema.AnyRawSchema} {
		if _, err := m.AddRaw(name, "", schema.AnyRawSchema, output, func(context.Context, json.RawMessage) (any, error) {
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	const note = "\nOn a direct tool call the result arrives as content blocks"
	for name, want := range map[string]bool{"blocks": true, "any": false} {
		tool, _ := m.Get(name)
		if got := strings.HasSuffix(tool.Summary(), note); got != want {
			t.Errorf("%s: summary %q carries the blocks note: %v, want %v", name, tool.Summary(), got, want)
		}
	}
}
