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

package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/tools"
)

type specRow struct {
	Name string `json:"name"`
}

func TestBuildWritesSharedDefinitionOnce(t *testing.T) {
	l := NewLibrary()
	if err := l.View("list", "", func(context.Context, struct{ Rows []specRow }) (api.Widget[struct{}], error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if err := l.View("card", "", func(context.Context, struct{ Row specRow }) (api.Widget[struct{}], error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	find, err := tools.NewTool("find", "", func(context.Context, string) (struct{ Row specRow }, error) { return struct{ Row specRow }{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Fetch(find); err != nil {
		t.Fatal(err)
	}
	if err := l.Build(); err != nil {
		t.Fatal(err)
	}
	spec, ends, items := l.Spec()
	if spec != "\nViews: v = ui.<name>(params); v.update(patch)\nui.card\nui.list\n\nApp functions: app.<name>(input) -> result\napp.find\n" || len(ends) != 3 || ends[2] != len(spec) || !slices.Equal(items, []string{"ui.card", "ui.list", "app.find"}) {
		t.Fatalf("catalog ends %v of %d bytes, items %v:\n%s", ends, len(spec), items, spec)
	}
	schema, err := Schema(l, []string{"ui.card", "ui.list", "app.find"})
	if err != nil {
		t.Fatal(err)
	}
	head, defs, ok := strings.Cut(schema, "\nDefinitions\n")
	if !ok || !strings.HasPrefix(head, "ui.card\n") || strings.Count(head, `"$ref":"#/definitions/CoreSpecRow"`) != 3 || strings.Contains(head, `"definitions"`) || defs != "  CoreSpecRow "+`{"properties":{"name":{"type":"string"}},"type":"object"}`+"\n" {
		t.Fatalf("schema:\n%s", schema)
	}
	if one, _ := Schema(l, []string{"ui.card"}); strings.Contains(one, "ui.list") || !strings.Contains(one, "  CoreSpecRow ") {
		t.Fatalf("one item's schema:\n%s", one)
	}
	if _, err := Schema(l, []string{"ui.nope"}); !errors.Is(err, errUnknownItem) {
		t.Fatalf("unknown item: %v", err)
	}
}

func TestBuildRenamesDefinitionClash(t *testing.T) {
	l := NewLibrary()
	{
		type specClash struct{ A int }
		if err := l.View("a", "", func(context.Context, struct{ Item specClash }) (api.Widget[struct{}], error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
	}
	{
		type specClash struct{ B string }
		if err := l.View("b", "", func(context.Context, struct{ Item specClash }) (api.Widget[struct{}], error) { return nil, nil }); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Build(); err != nil {
		t.Fatal(err)
	}
	schema, err := Schema(l, []string{"ui.a", "ui.b"})
	if err != nil {
		t.Fatal(err)
	}
	head, defs, _ := strings.Cut(schema, "\nDefinitions\n")
	a, b, _ := strings.Cut(head, "ui.b\n")
	renamed := definitionRef.FindStringSubmatch(b)
	if renamed == nil || !strings.HasPrefix(renamed[1], "CoreSpecClashType_") || !strings.Contains(a, `"#/definitions/CoreSpecClash"`) || !strings.Contains(defs, "  CoreSpecClash ") || !strings.Contains(defs, "  "+renamed[1]+" ") {
		t.Fatalf("schema:\n%s", schema)
	}
}
