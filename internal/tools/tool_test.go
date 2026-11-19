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
		in := json.RawMessage(`{}`)
		if name == "nested" {
			in = json.RawMessage(`{"input":{}}`)
		}
		content, err := tool.Agent(context.Background(), in)
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

func TestObjectModuleTakesInput(t *testing.T) {
	m := NewModule("m", "")
	numbers, err := schema.NewRawSchema(map[string]any{"type": "array", "items": map[string]any{"type": "integer"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err1 := m.Add("echo", "", func(_ context.Context, s string) (string, error) { return "got:" + s, nil })
	_, err2 := m.AddRaw("raw", "", numbers, schema.AnyRawSchema, func(_ context.Context, in json.RawMessage) (any, error) { return string(in), nil })
	_, err3 := m.AddRaw("any", "", schema.AnyRawSchema, schema.AnyRawSchema, func(_ context.Context, in json.RawMessage) (any, error) { return string(in), nil })
	if err := errors.Join(err1, err2, err3); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"echo": `{"properties":{"input":{"type":"string"}},"required":["input"],"type":"object"}`,
		"raw":  `{"properties":{"input":{"items":{"type":"integer"},"type":"array"}},"required":["input"],"type":"object"}`,
		"any":  `{"properties":{"input":{"description":"any JSON value"}},"required":["input"],"type":"object"}`,
	} {
		tool, _ := m.Get(name)
		if string(tool.Params()) != want {
			t.Errorf("%s: declares %s, want %s", name, tool.Params(), want)
		}
	}
	for _, c := range []struct {
		tool, in string
		want     any
		err      error
	}{
		{"echo", `{"input":"x"}`, "got:x", nil},
		{"echo", `"x"`, nil, schema.ErrNoInput},
		{"echo", `{}`, nil, schema.ErrNoInput},
		{"echo", `{"INPUT":"x"}`, nil, schema.ErrNoInput},
		{"echo", `{"input":1}`, nil, schema.ErrInvalidArguments},
		{"echo", `{"input":`, nil, schema.ErrInvalidInput},
		{"raw", `{"input":[1,2]}`, `[1,2]`, nil},
		{"raw", `[1,2]`, nil, schema.ErrNoInput},
		{"raw", `{"input":["a"]}`, nil, schema.ErrInvalidArguments},
		{"any", `{"input":{"a":1}}`, `{"a":1}`, nil},
		{"any", `{"input":"x"}`, `"x"`, nil},
		{"any", `{"a":1}`, nil, schema.ErrNoInput},
		{"any", `"x"`, nil, schema.ErrNoInput},
	} {
		tool, _ := m.Get(c.tool)
		fromString, err1 := tool.JS(context.Background())([]any{c.in})
		fromRaw, err2 := tool.JS(context.Background())([]any{json.RawMessage(c.in)})
		_, err3 := tool.Agent(context.Background(), json.RawMessage(c.in))
		_, err4 := tool.Decode(json.RawMessage(c.in))
		for path, err := range map[string]error{"script string": err1, "script raw": err2, "agent": err3, "decode": err4} {
			if !errors.Is(err, c.err) || c.err == nil && err != nil {
				t.Errorf("%s %s by %s: answered %v, want %v", c.tool, c.in, path, err, c.err)
			}
		}
		if c.err == nil && (fromString != c.want || fromRaw != c.want) {
			t.Errorf("%s %s: got %v and %v, want %v", c.tool, c.in, fromString, fromRaw, c.want)
		}
	}
}

func TestScriptModuleKeepsSchemas(t *testing.T) {
	m := NewScriptModule("m", "")
	if _, err := m.Add("echo", "", func(_ context.Context, s string) (string, error) { return "got:" + s, nil }); err != nil {
		t.Fatal(err)
	}
	tool, _ := m.Get("echo")
	if string(tool.Params()) != `{"type":"string"}` {
		t.Fatalf("a script module tool declares %s, want the schema as it is", tool.Params())
	}
	got, err := tool.JS(context.Background())([]any{`"x"`})
	if err != nil || got != "got:x" {
		t.Fatalf("a bare value gave %v, %v", got, err)
	}
}
