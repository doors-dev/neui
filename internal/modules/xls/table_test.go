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
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/testkit"
)

func TestTable(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	res, err := tableTool(ctx, tableArg{
		Path:  "report.xlsx",
		Sheet: "Report",
		Columns: []columnSpec{
			{Header: "Item description column"},
			{Header: "Amount", Format: "currency"},
			{Header: "Qty", Format: "integer"},
		},
		Rows:   [][]any{{"Widget", 12.5, 3.0}, {"Gadget", 99.99, 1.0}},
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	cols := []columnSpec{{Header: "K"}, {Header: "V", Format: "number"}}
	if _, err := tableTool(ctx, tableArg{Path: "multi.xlsx", Sheet: "First", Columns: cols, Rows: [][]any{{"a", 1.0}}}); err != nil {
		t.Fatal(err)
	}
	res, err := tableTool(ctx, tableArg{Path: "multi.xlsx", Sheet: "Second", Cell: "B2", Columns: cols, Rows: [][]any{{"b", 2.0}, {"c", 3.0}}})
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
	if !panes.Freeze || panes.YSplit != 2 || panes.XSplit != 0 || panes.TopLeftCell != "A3" {
		t.Fatalf("panes = %+v", panes)
	}
}

func TestFormat(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
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

func TestTableDates(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	ctx := ctl.Ctx()
	cols := []columnSpec{{Header: "Day", Format: "date"}, {Header: "At", Format: "date"}, {Header: "Serial", Format: "date"}}
	if _, err := tableTool(ctx, tableArg{Path: "d.xlsx", Sheet: "S", Columns: cols, Rows: [][]any{{"2024-03-15", "2024-03-15T13:30:00+02:00", 45366.0}}}); err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "d.xlsx")
	for _, c := range []struct {
		cell, raw string
		numFmt    int
	}{{"A2", "45366", 14}, {"B2", "45366.5625", 22}, {"C2", "45366", 14}} {
		v, _ := f.GetCellValue("S", c.cell, excelize.Options{RawCellValue: true})
		if v != c.raw {
			t.Fatalf("%s raw = %q, want %q", c.cell, v, c.raw)
		}
		ct, _ := f.GetCellType("S", c.cell)
		if ct != excelize.CellTypeUnset && ct != excelize.CellTypeNumber {
			t.Fatalf("%s type = %v", c.cell, ct)
		}
		sid, err := f.GetCellStyle("S", c.cell)
		if err != nil {
			t.Fatal(err)
		}
		st, err := f.GetStyle(sid)
		if err != nil {
			t.Fatal(err)
		}
		if st.NumFmt != c.numFmt {
			t.Fatalf("%s numFmt = %d, want %d", c.cell, st.NumFmt, c.numFmt)
		}
	}
	res, err := profileTool(ctx, profileArg{Path: "d.xlsx", Sheet: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Types[0] != "date" {
		t.Fatalf("types = %v", res.Types)
	}
}

func TestTableDate1904(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	on := true
	if err := f.SetWorkbookProps(&excelize.WorkbookPropsOptions{Date1904: &on}); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "d.xlsx", f)
	if _, err := tableTool(c.Ctx(), tableArg{Path: "d.xlsx", Sheet: "Sheet1", Columns: []columnSpec{{Header: "Day", Format: "date"}}, Rows: [][]any{{"2024-03-15"}}}); err != nil {
		t.Fatal(err)
	}
	if v, _ := reopen(t, fs, "d.xlsx").GetCellValue("Sheet1", "A2", excelize.Options{RawCellValue: true}); v != "43904" {
		t.Fatalf("raw = %q", v)
	}
}

func TestTableBadDate(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	_, err := tableTool(c.Ctx(), tableArg{Path: "d.xlsx", Sheet: "S", Columns: []columnSpec{{Header: "Day", Format: "date"}}, Rows: [][]any{{"15/03/2024"}}})
	if err == nil || !strings.Contains(err.Error(), "15/03/2024") {
		t.Fatalf("err = %v", err)
	}
}

func TestTableRefusesEarlyDates(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	for _, v := range []string{"1899-12-31", "1899-12-31T12:00:00", "1850-06-01"} {
		_, err := tableTool(c.Ctx(), tableArg{Path: "d.xlsx", Sheet: "S", Columns: []columnSpec{{Header: "Day", Format: "date"}}, Rows: [][]any{{v}}})
		if err == nil || !strings.Contains(err.Error(), "before 1900-01-01") {
			t.Fatalf("%s: err = %v", v, err)
		}
	}
	if _, err := tableTool(c.Ctx(), tableArg{Path: "d.xlsx", Sheet: "S", Columns: []columnSpec{{Header: "Day", Format: "date"}}, Rows: [][]any{{"1900-01-01"}}}); err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	on := true
	if err := f.SetWorkbookProps(&excelize.WorkbookPropsOptions{Date1904: &on}); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "e.xlsx", f)
	_, err := tableTool(c.Ctx(), tableArg{Path: "e.xlsx", Sheet: "Sheet1", Columns: []columnSpec{{Header: "Day", Format: "date"}}, Rows: [][]any{{"1903-12-31"}}})
	if err == nil || !strings.Contains(err.Error(), "before 1904-01-01") {
		t.Fatalf("1904 err = %v", err)
	}
}

func TestTableCreatesXLSXOnly(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	for _, name := range []string{"r.csv", "r.xlsm", "report"} {
		_, err := tableTool(c.Ctx(), tableArg{Path: name, Sheet: "S", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}})
		if err == nil || !strings.Contains(err.Error(), "does not end in .xlsx") {
			t.Fatalf("%s: err = %v", name, err)
		}
		if _, err := fs.Stat(c.Ctx(), name); err == nil {
			t.Fatalf("%s was created", name)
		}
	}
	if _, err := tableTool(c.Ctx(), tableArg{Path: "R.XLSX", Sheet: "S", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestTableFormulas(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	ctx := ctl.Ctx()
	cols := []columnSpec{{Header: "Qty", Format: "integer"}, {Header: "Price", Format: "currency"}, {Header: "Total", Format: "currency"}, {Header: "Note"}}
	rows := [][]any{{2.0, 1.5, "=A{row}*B{row}", "=IFS(A{row}>1,\"many\",TRUE,\"one\")"}, {1.0, 4.0, "=A{row}*B{row}", "=\"one\""}}
	if _, err := tableTool(ctx, tableArg{Path: "t.xlsx", Sheet: "S", Cell: "A5", Columns: cols, Rows: rows}); err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "t.xlsx")
	for _, c := range []struct{ cell, formula, value string }{
		{"C6", "A6*B6", "$3.00"},
		{"C7", "A7*B7", "$4.00"},
		{"D6", `_xlfn.IFS(A6>1,"many",TRUE,"one")`, "many"},
	} {
		if fl, _ := f.GetCellFormula("S", c.cell); fl != c.formula {
			t.Fatalf("%s formula = %q, want %q", c.cell, fl, c.formula)
		}
		if v, err := f.CalcCellValue("S", c.cell); err != nil || v != c.value {
			t.Fatalf("%s = %q, %v, want %q", c.cell, v, err, c.value)
		}
	}
	for _, bad := range []struct {
		v    string
		want string
	}{{"=WEBSERVICE(\"https://x.test\")", "external links"}, {"=XLOOKUP(1,A:A,B:B)", "XLOOKUP is a dynamic-array"}, {"=", "is empty"}} {
		_, err := tableTool(ctx, tableArg{Path: "t.xlsx", Sheet: "Other", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{bad.v}}})
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("%q: err = %v, want %q", bad.v, err, bad.want)
		}
	}
}

func TestTableKeepsPanes(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	user := excelize.Panes{Freeze: true, XSplit: 1, YSplit: 3, TopLeftCell: "B4", ActivePane: "bottomRight", Selection: []excelize.Selection{{SQRef: "C5", ActiveCell: "C5", Pane: "bottomRight"}}}
	if err := f.SetPanes("Sheet1", &user); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "p.xlsx", f)
	if _, err := tableTool(c.Ctx(), tableArg{Path: "p.xlsx", Sheet: "Sheet1", Cell: "E10", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}}); err != nil {
		t.Fatal(err)
	}
	panes, err := reopen(t, fs, "p.xlsx").GetPanes("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(panes, user) {
		t.Fatalf("panes = %+v, want %+v", panes, user)
	}
}

func TestTableFreezesFirstOnly(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	cols := []columnSpec{{Header: "K"}}
	if _, err := tableTool(ctx, tableArg{Path: "p.xlsx", Sheet: "S", Columns: cols, Rows: [][]any{{"a"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tableTool(ctx, tableArg{Path: "p.xlsx", Sheet: "S", Cell: "A20", Columns: cols, Rows: [][]any{{"b"}}}); err != nil {
		t.Fatal(err)
	}
	panes, err := reopen(t, fs, "p.xlsx").GetPanes("S")
	if err != nil {
		t.Fatal(err)
	}
	if !panes.Freeze || panes.YSplit != 1 || panes.TopLeftCell != "A2" {
		t.Fatalf("panes = %+v", panes)
	}
}

func TestTableRefusesNonEmpty(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
	_, err := tableTool(ctx, tableArg{Path: "book.xlsx", Sheet: "Data", Cell: "D2", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("err = %v", err)
	}
	_, err = tableTool(ctx, tableArg{Path: "book.xlsx", Sheet: "Notes", Cell: "B1", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}})
	if err == nil || !strings.Contains(err.Error(), "merged cells") {
		t.Fatalf("merge err = %v", err)
	}
	f := reopen(t, fs, "book.xlsx")
	if v, _ := f.GetCellValue("Data", "D3"); v != "" {
		t.Fatalf("D3 = %q", v)
	}
	if tables, _ := f.GetTables("Data"); len(tables) != 0 {
		t.Fatalf("tables = %+v", tables)
	}
}

func TestTableRefusesAutoFilter(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "N", "B1": "P", "A2": "a", "B2": 1} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.AutoFilter("Sheet1", "A1:B10", nil); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	_, err := tableTool(c.Ctx(), tableArg{Path: "book.xlsx", Sheet: "Sheet1", Cell: "A5", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}})
	if err == nil || !strings.Contains(err.Error(), "autofilter") {
		t.Fatalf("err = %v", err)
	}
}

func TestTableKeepsUserWidths(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "taken"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetColWidth("Sheet1", "B", "B", 30); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	cols := []columnSpec{{Header: "Long header A"}, {Header: "Long header B"}, {Header: "Long header C"}}
	if _, err := tableTool(ctl.Ctx(), tableArg{Path: "book.xlsx", Sheet: "Sheet1", Cell: "A10", Columns: cols, Rows: [][]any{{"a", "b", "c"}}}); err != nil {
		t.Fatal(err)
	}
	g := reopen(t, fs, "book.xlsx")
	for _, c := range []struct {
		col  string
		want float64
	}{{"A", 9.140625}, {"B", 30}, {"C", 13}} {
		w, err := g.GetColWidth("Sheet1", c.col)
		if err != nil {
			t.Fatal(err)
		}
		if w != c.want {
			t.Fatalf("width %s = %v, want %v", c.col, w, c.want)
		}
	}
	panes, err := g.GetPanes("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if panes.Freeze || panes.YSplit != 0 {
		t.Fatalf("panes changed on an existing sheet: %+v", panes)
	}
}

func TestTableParsesNumbers(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	cols := []columnSpec{{Header: "N", Format: "number"}, {Header: "C", Format: "currency"}, {Header: "P", Format: "percent"}, {Header: "I", Format: "integer"}}
	rows := [][]any{
		{"1,234.5", "$1,234.50", "15%", "12"},
		{" -7 ", "-$3", 0.15, 4.0},
		{"", "2", ".5", "1,000"},
	}
	if _, err := tableTool(c.Ctx(), tableArg{Path: "n.xlsx", Sheet: "S", Columns: cols, Rows: rows}); err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "n.xlsx")
	for cell, want := range map[string]string{"A2": "1234.5", "B2": "1234.5", "C2": "0.15", "D2": "12", "A3": "-7", "B3": "-3", "C3": "0.15", "D3": "4", "A4": "", "B4": "2", "C4": "0.5", "D4": "1000"} {
		v, _ := f.GetCellValue("S", cell, excelize.Options{RawCellValue: true})
		if v != want {
			t.Fatalf("%s raw = %q, want %q", cell, v, want)
		}
		if ct, _ := f.GetCellType("S", cell); want != "" && ct != excelize.CellTypeUnset && ct != excelize.CellTypeNumber {
			t.Fatalf("%s type = %v", cell, ct)
		}
	}
	for _, bad := range []struct {
		format string
		value  any
	}{{"number", "abc"}, {"number", "1,23"}, {"number", "$5"}, {"integer", "1e5"}, {"percent", "15 %%"}, {"currency", "€5"}, {"number", "-"}, {"number", "15%"}, {"integer", "15%"}, {"currency", "15%"}} {
		_, err := tableTool(c.Ctx(), tableArg{Path: "bad.xlsx", Sheet: "S", Columns: []columnSpec{{Header: "X", Format: bad.format}}, Rows: [][]any{{bad.value}}})
		if err == nil || !strings.Contains(err.Error(), "not a number") {
			t.Fatalf("%s %q: err = %v", bad.format, bad.value, err)
		}
	}
}

func TestTableRefusesBadColumnsAndCells(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	for _, c := range []struct {
		cols    []columnSpec
		rows    [][]any
		wantErr string
	}{
		{cols: []columnSpec{{Header: "Amount"}, {Header: "amount"}}, rows: [][]any{{1.0, 2.0}}, wantErr: "unique"},
		{cols: []columnSpec{{Header: "K"}, {Header: " "}}, rows: [][]any{{"a", "b"}}, wantErr: "header must not be empty"},
		{cols: []columnSpec{{Header: "Total", Format: "money"}}, rows: [][]any{}, wantErr: "format \"money\""},
		{cols: []columnSpec{{Header: "K"}, {Header: "V"}}, rows: [][]any{{"a", map[string]any{"v": 1.0}}}, wantErr: "not a cell value"},
		{cols: []columnSpec{{Header: "K"}}, rows: [][]any{{[]any{"a"}}}, wantErr: "not a cell value"},
	} {
		_, err := tableTool(ctl.Ctx(), tableArg{Path: "bad.xlsx", Sheet: "S", Columns: c.cols, Rows: c.rows})
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Fatalf("%+v: err = %v", c.cols, err)
		}
	}
	if _, err := fs.Stat(ctl.Ctx(), "bad.xlsx"); err == nil {
		t.Fatal("bad.xlsx was written")
	}
}
