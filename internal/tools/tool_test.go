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
		"image":   {failed, image.Contents()[0]},
		"blocks":  {failed, blocks[0], blocks[1]},
		"empty":   {failed},
		"plain":   {api.ContentText{Text: "boom"}},
		"wrapped": {api.ContentText{Text: "x: render failed"}, image.Contents()[0]},
		"nested":  {failed, image.Contents()[0]},
	} {
		tool, _ := m.Get(name)
		in := json.RawMessage(`{}`)
		if name == "nested" {
			in = json.RawMessage(`{"value":{}}`)
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
		"image":    "native content blocks",
		"document": "native content blocks",
		"label":    "a text block",
		"parts":    "native content blocks",
		"string":   "a text block",
	} {
		tool, _ := m.Get(name)
		if tool.OutputHint() != want {
			t.Errorf("%s: hint %q, want %q", name, tool.OutputHint(), want)
		}
	}
	container, _ := m.Get("parts")
	own, err := schema.NewSchema[parts]()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(container.Output(), own.View()) {
		t.Errorf("a container declares %+v, want its own type's schema", container.Output())
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
	for name, want := range map[string]string{"blocks": "native content blocks", "any": ""} {
		tool, _ := m.Get(name)
		if tool.OutputHint() != want {
			t.Errorf("%s: hint %q, want %q", name, tool.OutputHint(), want)
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
		"echo": `{"properties":{"value":{"type":"string"}},"required":["value"],"type":"object"}`,
		"raw":  `{"properties":{"value":{"items":{"type":"integer"},"type":"array"}},"required":["value"],"type":"object"}`,
		"any":  `{"properties":{"value":{"description":"any JSON value"}},"required":["value"],"type":"object"}`,
	} {
		tool, _ := m.Get(name)
		if string(tool.Input().Raw) != want {
			t.Errorf("%s: declares %s, want %s", name, tool.Input().Raw, want)
		}
	}
	for _, c := range []struct {
		tool, in string
		want     any
		err      error
	}{
		{"echo", `{"value":"x"}`, "got:x", nil},
		{"echo", `"x"`, nil, schema.ErrNoInput},
		{"echo", `{}`, nil, schema.ErrNoInput},
		{"echo", `{"VALUE":"x"}`, nil, schema.ErrNoInput},
		{"echo", `{"value":1}`, nil, schema.ErrInvalidArguments},
		{"echo", `{"value":`, nil, schema.ErrInvalidInput},
		{"raw", `{"value":[1,2]}`, `[1,2]`, nil},
		{"raw", `[1,2]`, nil, schema.ErrNoInput},
		{"raw", `{"value":["a"]}`, nil, schema.ErrInvalidArguments},
		{"any", `{"value":{"a":1}}`, `{"a":1}`, nil},
		{"any", `{"value":"x"}`, `"x"`, nil},
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
	if string(tool.Input().Raw) != `{"type":"string"}` {
		t.Fatalf("a script module tool declares %s, want the schema as it is", tool.Input().Raw)
	}
	got, err := tool.JS(context.Background())([]any{`"x"`})
	if err != nil || got != "got:x" {
		t.Fatalf("a bare value gave %v, %v", got, err)
	}
}

func TestModuleNameStartsWithALetter(t *testing.T) {
	for name, want := range map[string]string{"3d": "x3d", "files": "files", "9 lives": "x9_lives"} {
		if got := NewModule(name, "").Name(); got != want {
			t.Errorf("NewModule(%q) is named %q, want %q", name, got, want)
		}
		if got := NewScriptModule(name, "").Name(); got != want {
			t.Errorf("NewScriptModule(%q) is named %q, want %q", name, got, want)
		}
	}
	long := NewModule("1"+strings.Repeat("a", 40), "").Name()
	if len(long) > 31 || long[0] != 'x' {
		t.Errorf("a long name starting with a digit became %q, want at most 31 characters starting with x", long)
	}
}

type describedPart struct {
	_    struct{} `description:"the part's own description"`
	Part int      `json:"part"`
}

type described struct {
	Rows []string `json:"rows"`
	describedPart
	_ struct{} `description:"rows of the sheet; truncated: call again"`
}

type undescribed struct {
	Rows []string `json:"rows"`
}

func TestOutputCarriesTheResultDescription(t *testing.T) {
	m := NewModule("m", "")
	_, err1 := m.Add("described", "Read rows.", func(context.Context, struct{}) (described, error) { return described{}, nil })
	_, err2 := m.Add("pointer", "Read rows.", func(context.Context, struct{}) (*described, error) { return nil, nil })
	_, err3 := m.Add("undescribed", "Read rows.", func(context.Context, struct{}) (undescribed, error) { return undescribed{}, nil })
	_, err4 := m.Add("list", "Read rows.", func(context.Context, struct{}) ([]describedPart, error) { return nil, nil })
	_, err5 := m.Add("string", "Read rows.", func(context.Context, struct{}) (string, error) { return "", nil })
	_, err6 := m.Add("parts", "Read rows.", func(context.Context, struct{}) (parts, error) { return parts{}, nil })
	_, err7 := m.AddRaw("any", "Read rows.", schema.AnyRawSchema, schema.AnyRawSchema, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	raw, err8 := schema.NewRawSchema(json.RawMessage(`{"type":"object","description":"the server's rows","properties":{"rows":{"type":"array"}}}`))
	_, err9 := m.AddRaw("raw", "Read rows.", schema.AnyRawSchema, raw, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	if err := errors.Join(err1, err2, err3, err4, err5, err6, err7, err8, err9); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"described":   "Result: rows of the sheet; truncated: call again",
		"pointer":     "Result: rows of the sheet; truncated: call again",
		"undescribed": "",
		"list":        "Result: the part's own description",
		"string":      "",
		"parts":       "",
		"any":         "",
		"raw":         "Result: the server's rows",
	} {
		tool, _ := m.Get(name)
		if got := tool.Output().Brief; got != want {
			t.Errorf("%s: result description %q, want %q", name, got, want)
		}
		if tool.Desc() != "Read rows." {
			t.Errorf("%s: description %q, want the tool's own", name, tool.Desc())
		}
	}
	tool, _ := m.Get("described")
	if !strings.Contains(string(tool.Output().Raw), `"description":"rows of the sheet; truncated: call again"`) {
		t.Errorf("the output schema %s lacks the output description", tool.Output().Raw)
	}
}
