package xls

import (
	"context"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
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

func TestColumn(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	res, err := columnTool(threadCtx(fs), columnArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:D4", Header: "Triple", Formula: "=B{row}*3"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Column != "E" || res.Range != "E1:E4" {
		t.Fatalf("result = %+v", res)
	}
	f := reopen(t, fs, "book.xlsx")
	if v, _ := f.GetCellValue("Data", "E1"); v != "Triple" {
		t.Fatalf("header = %q", v)
	}
	for _, c := range []struct{ cell, want string }{{"E2", "B2*3"}, {"E3", "B3*3"}, {"E4", "B4*3"}} {
		fl, err := f.GetCellFormula("Data", c.cell)
		if err != nil {
			t.Fatal(err)
		}
		if fl != c.want {
			t.Fatalf("%s formula = %q, want %q", c.cell, fl, c.want)
		}
	}
	if v, _ := f.GetCellValue("Data", "E2", excelize.Options{RawCellValue: true}); v != "" {
		t.Fatalf("cached value not cleared: %q", v)
	}
}

func TestColumnNoPlaceholder(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := columnTool(threadCtx(fs), columnArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:D4", Header: "X", Formula: "=B2*3"})
	if err == nil || !strings.Contains(err.Error(), "{row}") {
		t.Fatalf("err = %v", err)
	}
}

func TestTotal(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	res, err := totalTool(threadCtx(fs), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Aggs: []aggSpec{{Column: "B", Fn: "sum"}}})
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
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	_, err := totalTool(threadCtx(fs), totalArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Aggs: []aggSpec{{Column: "B", Fn: "median"}}})
	if err == nil || !strings.Contains(err.Error(), "median") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheck(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	ctx := threadCtx(fs)
	res, err := checkTool(ctx, checkArg{Path: "book.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Formulas != 1 || len(res.Errors) != 0 {
		t.Fatalf("clean book: %+v", res)
	}
	_, err = columnTool(ctx, columnArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:D4", Header: "Bad", Formula: "=NOPEFN(B{row})"})
	if err != nil {
		t.Fatal(err)
	}
	res, err = checkTool(ctx, checkArg{Path: "book.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Formulas != 4 {
		t.Fatalf("formulas = %d", res.Formulas)
	}
	if len(res.Errors) != 3 {
		t.Fatalf("errors = %+v", res.Errors)
	}
	if res.Errors[0].Cell != "Data!E2" || res.Errors[0].Error == "" {
		t.Fatalf("errors[0] = %+v", res.Errors[0])
	}
}

func TestColumnKeepsOtherCachedValues(t *testing.T) {
	fs, _ := newFSStore(t)
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "N", "B1": "Twice", "A2": 1, "A3": 2, "B2": 2, "B3": 4, "C2": 99, "C3": 98} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellFormula("Sheet1", "B2", "A2*2"); err != nil {
		t.Fatal(err)
	}
	w, err := fs.Create("book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Write(w); err != nil {
		t.Fatal(err)
	}
	w.Close()
	f.Close()
	ctx := threadCtx(fs)
	if _, err := columnTool(ctx, columnArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:B3", Header: "Thrice", Formula: "=A{row}*3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := totalTool(ctx, totalArg{Path: "book.xlsx", Sheet: "Sheet1", Range: "A1:C3", Aggs: []aggSpec{{Column: "A", Fn: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	g := reopen(t, fs, "book.xlsx")
	if v, _ := g.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); v != "2" {
		t.Fatalf("unrelated cached value = %q, want 2", v)
	}
	for _, cell := range []string{"C2", "C3", "A4"} {
		if v, _ := g.GetCellValue("Sheet1", cell, excelize.Options{RawCellValue: true}); v != "" {
			t.Fatalf("%s stale cached value %q", cell, v)
		}
	}
	calc, err := g.GetCalcProps()
	if err != nil {
		t.Fatal(err)
	}
	if calc.FullCalcOnLoad == nil || !*calc.FullCalcOnLoad {
		t.Fatal("fullCalcOnLoad not set")
	}
}
