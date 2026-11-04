package xls

import (
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	fs, _ := newFSStore(t)
	ctx := threadCtx(fs)
	res, err := tableTool(ctx, tableArg{
		Path:  "report.xlsx",
		Sheet: "Report",
		Columns: []columnSpec{
			{Header: "Item description column"},
			{Header: "Amount", Format: "currency"},
			{Header: "Qty", Format: "integer"},
		},
		Rows:   [][]any{{"Widget", 12.5, 3}, {"Gadget", 99.99, 1}},
		Preset: "report",
		Name:   "Sales",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != "A1:C3" || res.Name != "Sales" {
		t.Fatalf("result = %+v", res)
	}
	f, err := openRead(ctx, fs, "report.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tables, err := f.GetTables("Report")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Name != "Sales" || tables[0].Range != "A1:C3" {
		t.Fatalf("tables = %+v", tables)
	}
	if tables[0].StyleName != "TableStyleMedium2" {
		t.Fatalf("style = %q", tables[0].StyleName)
	}
	sid, err := f.GetCellStyle("Report", "B2")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.GetStyle(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st.CustomNumFmt == nil || *st.CustomNumFmt != "$#,##0.00" {
		t.Fatalf("B2 style = %+v", st)
	}
	sid, err = f.GetCellStyle("Report", "C3")
	if err != nil {
		t.Fatal(err)
	}
	st, err = f.GetStyle(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st.NumFmt != 3 {
		t.Fatalf("C3 numFmt = %d", st.NumFmt)
	}
	wA, err := f.GetColWidth("Report", "A")
	if err != nil {
		t.Fatal(err)
	}
	if wA != 23 {
		t.Fatalf("width A = %v", wA)
	}
	wC, err := f.GetColWidth("Report", "C")
	if err != nil {
		t.Fatal(err)
	}
	if wC != 6 {
		t.Fatalf("width C = %v", wC)
	}
	panes, err := f.GetPanes("Report")
	if err != nil {
		t.Fatal(err)
	}
	if !panes.Freeze || panes.YSplit != 1 || panes.TopLeftCell != "A2" {
		t.Fatalf("panes = %+v", panes)
	}
}

func TestTableSecondSheet(t *testing.T) {
	fs, _ := newFSStore(t)
	ctx := threadCtx(fs)
	cols := []columnSpec{{Header: "K"}, {Header: "V", Format: "number"}}
	if _, err := tableTool(ctx, tableArg{Path: "multi.xlsx", Sheet: "First", Columns: cols, Rows: [][]any{{"a", 1}}}); err != nil {
		t.Fatal(err)
	}
	res, err := tableTool(ctx, tableArg{Path: "multi.xlsx", Sheet: "Second", Cell: "B2", Columns: cols, Rows: [][]any{{"b", 2}, {"c", 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != "B2:C4" || res.Name == "" {
		t.Fatalf("result = %+v", res)
	}
	f, err := openRead(ctx, fs, "multi.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	list := f.GetSheetList()
	if len(list) != 2 || list[0] != "First" || list[1] != "Second" {
		t.Fatalf("sheets = %v", list)
	}
	tables, err := f.GetTables("Second")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Range != "B2:C4" || tables[0].Name != res.Name {
		t.Fatalf("tables = %+v", tables)
	}
	panes, err := f.GetPanes("Second")
	if err != nil {
		t.Fatal(err)
	}
	if !panes.Freeze || panes.YSplit != 2 || panes.TopLeftCell != "B3" {
		t.Fatalf("panes = %+v", panes)
	}
}

func TestFormat(t *testing.T) {
	fs, _ := newFSStore(t)
	buildBook(t, fs, "book.xlsx")
	ctx := threadCtx(fs)
	if _, err := formatTool(ctx, formatArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B1", Bold: true, Fill: "yellow"}); err != nil {
		t.Fatal(err)
	}
	if _, err := formatTool(ctx, formatArg{Path: "book.xlsx", Sheet: "Data", Range: "C2:C2", Bold: true}); err != nil {
		t.Fatal(err)
	}
	f, err := openRead(ctx, fs, "book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sid, err := f.GetCellStyle("Data", "A1")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.GetStyle(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st.Font == nil || !st.Font.Bold {
		t.Fatalf("A1 font = %+v", st.Font)
	}
	if len(st.Fill.Color) == 0 || !strings.Contains(strings.ToUpper(st.Fill.Color[0]), "FFF2CC") {
		t.Fatalf("A1 fill = %+v", st.Fill)
	}
	sid, err = f.GetCellStyle("Data", "C2")
	if err != nil {
		t.Fatal(err)
	}
	st, err = f.GetStyle(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st.Font == nil || !st.Font.Bold || st.NumFmt != 14 {
		t.Fatalf("C2 style = %+v", st)
	}
	if _, err := formatTool(ctx, formatArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B1", Fill: "pink"}); err == nil {
		t.Fatal("bad fill: no error")
	}
	if _, err := formatTool(ctx, formatArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B1", NumberFormat: "money"}); err == nil {
		t.Fatal("bad numberFormat: no error")
	}
}
