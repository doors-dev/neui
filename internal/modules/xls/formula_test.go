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

package xls

import (
	"context"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/testkit"
)

func reopen(t *testing.T, fs api.FS, name string) *excelize.File {
	t.Helper()
	path, err := fs.LocalPath(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestTotal(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	res, err := totalTool(c.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Aggs: []aggSpec{{Column: "B", Fn: "sum"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Row != 5 || res.Range != "A5:B5" {
		t.Fatalf("result = %+v", res)
	}
	f := reopen(t, fs, "book.xlsx")
	fl, err := f.GetCellFormula("Data", "B5")
	if err != nil {
		t.Fatal(err)
	}
	if fl != "SUBTOTAL(109,B2:B4)" {
		t.Fatalf("formula = %q", fl)
	}
	styleID, err := f.GetCellStyle("Data", "A5")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Font == nil || !st.Font.Bold {
		t.Fatalf("summary row not bold: %+v", st)
	}
	if v, _ := f.GetCellValue("Data", "B5", excelize.Options{RawCellValue: true}); v != "" {
		t.Fatalf("cached value not cleared: %q", v)
	}
}

func TestTotalBadFn(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := totalTool(c.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Aggs: []aggSpec{{Column: "B", Fn: "median"}}})
	if err == nil || !strings.Contains(err.Error(), "median") || !strings.Contains(err.Error(), "count_nums") {
		t.Fatalf("err = %v", err)
	}
}

func writeBook(t *testing.T, fs api.FS, name string, f *excelize.File) {
	t.Helper()
	w, err := fs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Write(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestTotalFns(t *testing.T) {
	for _, c := range []struct{ fn, want string }{
		{"sum", "SUBTOTAL(109,B2:B4)"},
		{"count", "SUBTOTAL(103,B2:B4)"},
		{"count_nums", "SUBTOTAL(102,B2:B4)"},
	} {
		ctl := testkit.NewController(t, testkit.Setup{})
		fs := ctl.FS()
		buildBook(t, fs, "book.xlsx")
		if _, err := totalTool(ctl.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Aggs: []aggSpec{{Column: "B", Fn: c.fn}}}); err != nil {
			t.Fatal(err)
		}
		fl, err := reopen(t, fs, "book.xlsx").GetCellFormula("Data", "B5")
		if err != nil {
			t.Fatal(err)
		}
		if fl != c.want {
			t.Fatalf("%s formula = %q, want %q", c.fn, fl, c.want)
		}
	}
}

func TestTotalCountEval(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "All", "B1": "Nums", "A2": 1, "A3": "x", "A4": 2, "B2": 1, "B3": "x", "B4": 2} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	writeBook(t, fs, "book.xlsx", f)
	if _, err := totalTool(ctl.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:B5", Aggs: []aggSpec{{Column: "A", Fn: "count"}, {Column: "B", Fn: "count_nums"}}}); err != nil {
		t.Fatal(err)
	}
	g := reopen(t, fs, "book.xlsx")
	for _, c := range []struct{ cell, want string }{{"A6", "3"}, {"B6", "2"}} {
		v, err := g.CalcCellValue("Sheet1", c.cell)
		if err != nil {
			t.Fatal(err)
		}
		if v != c.want {
			t.Fatalf("%s = %q, want %q", c.cell, v, c.want)
		}
	}
}

func TestTotalRefusesNonEmpty(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := totalTool(c.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B3", Aggs: []aggSpec{{Column: "B", Fn: "sum"}}})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("err = %v", err)
	}
	if v, _ := reopen(t, fs, "book.xlsx").GetCellValue("Data", "B4"); v != "30" {
		t.Fatalf("B4 = %q", v)
	}
}

func salesTable(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := tableTool(ctx, tableArg{
		Path:    "book.xlsx",
		Sheet:   "Data",
		Columns: []columnSpec{{Header: "Item"}, {Header: "Price", Format: "number"}, {Header: "Qty", Format: "integer"}},
		Rows:    [][]any{{"a", 2.0, 3.0}, {"b", 4.0, 5.0}, {"c", 6.0, 7.0}},
		Preset:  "report",
		Name:    "T",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTotalByHeader(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := ctl.Ctx()
	res, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:D4", Aggs: []aggSpec{{Column: "amount", Fn: "max"}, {Column: "A", Fn: "count"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != "A5:D5" {
		t.Fatalf("result = %+v", res)
	}
	f := reopen(t, fs, "book.xlsx")
	for _, c := range []struct{ cell, want string }{{"B5", "SUBTOTAL(104,B2:B4)"}, {"A5", "SUBTOTAL(103,A2:A4)"}} {
		if fl, _ := f.GetCellFormula("Data", c.cell); fl != c.want {
			t.Fatalf("%s formula = %q, want %q", c.cell, fl, c.want)
		}
	}
	for _, col := range []string{"Price", "F", "B"} {
		_, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Data", Range: "C1:D4", Aggs: []aggSpec{{Column: col, Fn: "sum"}}})
		if err == nil || !strings.Contains(err.Error(), "neither a header") {
			t.Fatalf("column %q: err = %v", col, err)
		}
	}
}

func TestTotalCopiesNumberFormat(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "Item", "B1": "Price", "A2": "a", "B2": 2.5, "A3": "b", "B3": 4} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	money := "$#,##0.00"
	style, err := f.NewStyle(&excelize.Style{CustomNumFmt: &money})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle("Sheet1", "B2", "B3", style); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	ctx := ctl.Ctx()
	if _, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:B3", Aggs: []aggSpec{{Column: "Price", Fn: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:B4", Aggs: []aggSpec{{Column: "Price", Fn: "count_nums"}}}); err != nil {
		t.Fatal(err)
	}
	g := reopen(t, fs, "book.xlsx")
	for _, c := range []struct {
		cell  string
		money bool
	}{{"B4", true}, {"B5", false}} {
		sid, err := g.GetCellStyle("Sheet1", c.cell)
		if err != nil {
			t.Fatal(err)
		}
		st, err := g.GetStyle(sid)
		if err != nil {
			t.Fatal(err)
		}
		if st.Font == nil || !st.Font.Bold {
			t.Fatalf("%s not bold: %+v", c.cell, st)
		}
		if got := st.CustomNumFmt != nil && *st.CustomNumFmt == money; got != c.money {
			t.Fatalf("%s currency format = %v, want %v (%+v)", c.cell, got, c.money, st)
		}
	}
}

func TestTotalRefusesTable(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs, ctx := ctl.FS(), ctl.Ctx()
	salesTable(t, ctx)
	for _, c := range []struct{ rng, want string }{{"A1:C4", "is table"}, {"B1:C4", "overlaps table"}, {"A1:C3", "overlaps table"}} {
		_, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Data", Range: c.rng, Aggs: []aggSpec{{Column: "Qty", Fn: "sum"}}})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v", c.rng, err)
		}
	}
	f := reopen(t, fs, "book.xlsx")
	tables, err := f.GetTables("Data")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Range != "A1:C4" {
		t.Fatalf("tables = %+v", tables)
	}
	if v, _ := f.GetCellValue("Data", "C5"); v != "" {
		t.Fatalf("C5 = %q", v)
	}
}

func TestTotalRefusesInsideAutoFilter(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "N", "B1": "P", "A2": "a", "B2": 1, "A3": "b", "B3": 3} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.AutoFilter("Sheet1", "A1:B6", nil); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	_, err := totalTool(c.Ctx(), totalArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:B3", Aggs: []aggSpec{{Column: "P", Fn: "sum"}}})
	if err == nil || !strings.Contains(err.Error(), "autofilter") {
		t.Fatalf("err = %v", err)
	}
}
