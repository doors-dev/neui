package xls

import (
	"archive/zip"
	"context"
	"strings"
	"testing"
)

func TestChart(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := chartTool(threadCtx(fs), chartArg{
		Path:       "book.xlsx",
		Sheet:      "Data",
		Type:       "column",
		DataRange:  "B2:B4",
		Categories: "A2:A4",
		Anchor:     "F2",
		Title:      "Amounts",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "book.xlsx")
	if _, err := f.GetSheetIndex("Data"); err != nil {
		t.Fatal(err)
	}
	path, err := fs.LocalPath(context.Background(), "book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	found := false
	for _, zf := range zr.File {
		if strings.HasPrefix(zf.Name, "xl/charts/chart") {
			found = true
		}
	}
	if !found {
		t.Fatal("no chart part in workbook")
	}
}

func TestChartBadType(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := chartTool(threadCtx(fs), chartArg{Path: "book.xlsx", Sheet: "Data", Type: "donut", DataRange: "B2:B4", Anchor: "F2"})
	if err == nil || !strings.Contains(err.Error(), "donut") {
		t.Fatalf("err = %v", err)
	}
}

func TestHighlight(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := highlightTool(threadCtx(fs), highlightArg{
		Path:  "book.xlsx",
		Sheet: "Data",
		Range: "B2:B4",
		Rule:  highlightRule{Type: "gt", Value: "15"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "book.xlsx")
	formats, err := f.GetConditionalFormats("Data")
	if err != nil {
		t.Fatal(err)
	}
	opts, ok := formats["B2:B4"]
	if !ok || len(opts) != 1 {
		t.Fatalf("formats = %+v", formats)
	}
	if opts[0].Type != "cell" || opts[0].Criteria != "greater than" || opts[0].Value != "15" {
		t.Fatalf("opts[0] = %+v", opts[0])
	}
}

func TestHighlightColorScale(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := highlightTool(threadCtx(fs), highlightArg{
		Path:  "book.xlsx",
		Sheet: "Data",
		Range: "B2:B4",
		Rule:  highlightRule{Type: "colorScale"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "book.xlsx")
	formats, err := f.GetConditionalFormats("Data")
	if err != nil {
		t.Fatal(err)
	}
	opts := formats["B2:B4"]
	if len(opts) != 1 || opts[0].Type != "3_color_scale" {
		t.Fatalf("formats = %+v", formats)
	}
}

func TestSheetRefQuotes(t *testing.T) {
	for in, want := range map[string]string{"Data": "'Data'", "Q1-2024": "'Q1-2024'", "2024": "'2024'", "A1": "'A1'", "Bob's": "'Bob''s'"} {
		if got := sheetRef(in); got != want {
			t.Fatalf("sheetRef(%q) = %q, want %q", in, got, want)
		}
	}
}
