package xls

import (
	"archive/zip"
	"context"
	"strings"
	"testing"

	"neui.dev/internal/testkit"
)

func TestChart(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := chartTool(c.Ctx(), chartArg{
		Path:       "book.xlsx",
		Sheet:      "Data",
		Type:       "column",
		DataRange:  "B1:B4",
		Categories: "A1:A4",
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
	chart := ""
	for _, zf := range zr.File {
		if strings.HasPrefix(zf.Name, "xl/charts/chart") {
			chart = readPart(t, fs, "book.xlsx", zf.Name)
		}
	}
	for _, want := range []string{"<f>&#39;Data&#39;!$B$1</f>", "<f>&#39;Data&#39;!$B$2:$B$4</f>", "<f>&#39;Data&#39;!$A$2:$A$4</f>"} {
		if !strings.Contains(chart, want) {
			t.Fatalf("chart lacks %s:\n%s", want, chart)
		}
	}
	_, err = chartTool(c.Ctx(), chartArg{Path: "book.xlsx", Sheet: "Data", Type: "line", DataRange: "B2:B2", Anchor: "F20"})
	if err == nil || !strings.Contains(err.Error(), "no data rows") {
		t.Fatalf("header-only err = %v", err)
	}
}

func TestChartBadType(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := chartTool(c.Ctx(), chartArg{Path: "book.xlsx", Sheet: "Data", Type: "donut", DataRange: "B1:B4", Anchor: "F2"})
	if err == nil || !strings.Contains(err.Error(), "donut") {
		t.Fatalf("err = %v", err)
	}
}

func TestHighlight(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := highlightTool(c.Ctx(), highlightArg{
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := highlightTool(c.Ctx(), highlightArg{
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

func TestHighlightRules(t *testing.T) {
	for _, c := range []struct {
		rule           highlightRule
		typ, criteria  string
		value, wantErr string
	}{
		{rule: highlightRule{Type: "lt", Value: "15"}, typ: "cell", criteria: "less than", value: "15"},
		{rule: highlightRule{Type: "top", Value: "2"}, typ: "top", criteria: "=", value: "2"},
		{rule: highlightRule{Type: "bottom", Value: "1"}, typ: "bottom", criteria: "=", value: "1"},
		{rule: highlightRule{Type: "contains", Value: "li"}, typ: "text", criteria: "containing", value: "li"},
		{rule: highlightRule{Type: "dataBar"}, typ: "data_bar"},
		{rule: highlightRule{Type: "gt"}, wantErr: "needs a value"},
		{rule: highlightRule{Type: "gt", Value: " 2.50 "}, typ: "cell", criteria: "greater than", value: "2.5"},
		{rule: highlightRule{Type: "gt", Value: "1,000"}, wantErr: "plain number"},
		{rule: highlightRule{Type: "lt", Value: "500€"}, wantErr: "plain number"},
		{rule: highlightRule{Type: "gt", Value: "10%"}, wantErr: "plain number"},
		{rule: highlightRule{Type: "gt", Value: "NaN"}, wantErr: "plain number"},
		{rule: highlightRule{Type: "top", Value: "x"}, wantErr: "integer count"},
		{rule: highlightRule{Type: "top", Value: "0"}, wantErr: "integer count"},
		{rule: highlightRule{Type: "between", Value: "1"}, wantErr: "between"},
	} {
		ctl := testkit.NewController(t, testkit.Setup{})
		fs := ctl.FS()
		buildBook(t, fs, "book.xlsx")
		_, err := highlightTool(ctl.Ctx(), highlightArg{Path: "book.xlsx", Sheet: "Data", Range: "A2:B4", Rule: c.rule})
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("%+v: err = %v", c.rule, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%+v: %v", c.rule, err)
		}
		formats, err := reopen(t, fs, "book.xlsx").GetConditionalFormats("Data")
		if err != nil {
			t.Fatal(err)
		}
		opts := formats["A2:B4"]
		if len(opts) != 1 || opts[0].Type != c.typ || c.criteria != "" && opts[0].Criteria != c.criteria || opts[0].Value != c.value {
			t.Fatalf("%+v: formats = %+v", c.rule, formats)
		}
	}
}

func TestSheetRefQuotes(t *testing.T) {
	for in, want := range map[string]string{"Data": "'Data'", "Q1-2024": "'Q1-2024'", "2024": "'2024'", "A1": "'A1'", "Bob's": "'Bob''s'"} {
		if got := sheetRef(in); got != want {
			t.Fatalf("sheetRef(%q) = %q, want %q", in, got, want)
		}
	}
}
