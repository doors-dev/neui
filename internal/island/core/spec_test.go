package core

import (
	"context"
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
	head, defs, ok := strings.Cut(spec, "\nDefinitions\n")
	if !ok || strings.Count(head, `"$ref":"#/definitions/CoreSpecRow"`) != 3 || strings.Contains(head, `"definitions"`) || defs != "  CoreSpecRow "+`{"properties":{"name":{"type":"string"}},"type":"object"}`+"\n" {
		t.Fatalf("spec:\n%s", spec)
	}
	if len(ends) != 5 || !strings.HasPrefix(spec, "\nViews show something: ") || !strings.HasPrefix(spec[ends[0]:], "ui.list\n") || !strings.HasPrefix(spec[ends[1]:], "\nApp functions: ") || spec[ends[2]:ends[3]] != "\nDefinitions\n" || ends[4] != len(spec) || !slices.Equal(items, []string{"ui.card", "ui.list", "app.find", "Definitions", "CoreSpecRow"}) {
		t.Fatalf("ends %v of %d bytes, items %v:\n%s", ends, len(spec), items, spec)
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
	spec, _, items := l.Spec()
	head, defs, _ := strings.Cut(spec, "\nDefinitions\n")
	a, b, _ := strings.Cut(head, "ui.b\n")
	renamed := slices.IndexFunc(items, func(item string) bool { return strings.HasPrefix(item, "CoreSpecClashType_") })
	if renamed < 0 || !strings.Contains(a, `"#/definitions/CoreSpecClash"`) || !strings.Contains(b, `"#/definitions/`+items[renamed]+`"`) || !strings.Contains(defs, "  CoreSpecClash ") || !strings.Contains(defs, "  "+items[renamed]+" ") {
		t.Fatalf("items %v:\n%s", items, spec)
	}
}
