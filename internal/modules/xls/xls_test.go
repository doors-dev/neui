package xls

import (
	"context"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/testkit"
)

func buildBook(t *testing.T, fs api.FS, name string) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Data"); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		cell string
		vals []any
	}{
		{"A1", []any{"Name", "Amount", "When", "Double"}},
		{"A2", []any{"Alice", 10.5}},
		{"A3", []any{"Bob", 20}},
		{"A4", []any{"Cara", 30}},
	}
	for _, r := range rows {
		if err := f.SetSheetRow("Data", r.cell, &r.vals); err != nil {
			t.Fatal(err)
		}
	}
	style, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle("Data", "C2", "C4", style); err != nil {
		t.Fatal(err)
	}
	for cell, serial := range map[string]int{"C2": 45000, "C3": 45001, "C4": 45002} {
		if err := f.SetCellValue("Data", cell, serial); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellFormula("Data", "D2", "B2*2"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet("Notes"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Notes", "A1", "note"); err != nil {
		t.Fatal(err)
	}
	if err := f.MergeCell("Notes", "A1", "B1"); err != nil {
		t.Fatal(err)
	}
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
}

func TestSheets(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	infos, err := sheetsTool(c.Ctx(), sheetsArg{Path: "book.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[0].Name != "Data" || infos[1].Name != "Notes" {
		t.Fatalf("infos = %+v", infos)
	}
	if infos[0].Rows != 4 || infos[0].Cols != 4 {
		t.Fatalf("Data size = %dx%d", infos[0].Rows, infos[0].Cols)
	}
	if len(infos[1].Merges) != 1 || infos[1].Merges[0] != "A1:B1" {
		t.Fatalf("Notes merges = %v", infos[1].Merges)
	}
}

func TestProfile(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	res, err := profileTool(c.Ctx(), profileArg{Path: "book.xlsx", Sheet: "Data"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Headers) != 4 || res.Headers[0] != "Name" || res.Headers[3] != "Double" {
		t.Fatalf("headers = %v", res.Headers)
	}
	want := []string{"text", "number", "date", "formula"}
	for i, typ := range want {
		if res.Types[i] != typ {
			t.Fatalf("types = %v, want %v", res.Types, want)
		}
	}
	if res.RowCount != 3 {
		t.Fatalf("rowCount = %d", res.RowCount)
	}
	if len(res.Sample) != 3 || res.Sample[0][0] != "Alice" {
		t.Fatalf("sample = %v", res.Sample)
	}
}

func TestProfileRange(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	res, err := profileTool(c.Ctx(), profileArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Headers) != 2 || res.Headers[1] != "Amount" {
		t.Fatalf("headers = %v", res.Headers)
	}
	if res.RowCount != 2 || len(res.Sample) != 2 {
		t.Fatalf("rowCount = %d, sample = %v", res.RowCount, res.Sample)
	}
}

func TestReadPaging(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
	res, err := readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 || res.Rows[0][0] != "Alice" || res.Rows[1][0] != "Bob" {
		t.Fatalf("rows = %v", res.Rows)
	}
	if !res.Truncated || res.NextOffset != 3 {
		t.Fatalf("truncated = %v, nextOffset = %d", res.Truncated, res.NextOffset)
	}
	res, err = readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Offset: res.NextOffset, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "Cara" {
		t.Fatalf("rows = %v", res.Rows)
	}
	if res.Truncated || res.NextOffset != 4 {
		t.Fatalf("truncated = %v, nextOffset = %d", res.Truncated, res.NextOffset)
	}
}

func TestReadRawFormatted(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
	res, err := readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Range: "C2:D2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || len(res.Rows[0]) != 2 {
		t.Fatalf("rows = %v", res.Rows)
	}
	if res.Rows[0][0] == "45000" || res.Rows[0][0] == "" {
		t.Fatalf("formatted date = %q", res.Rows[0][0])
	}
	raw, err := readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Range: "C2:D2", Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if raw.Rows[0][0] != "45000" {
		t.Fatalf("raw date = %q", raw.Rows[0][0])
	}
	if raw.Rows[0][1] != "=B2*2" {
		t.Fatalf("raw formula = %q", raw.Rows[0][1])
	}
}

func TestFind(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	res, err := findTool(c.Ctx(), findArg{Path: "book.xlsx", Query: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].Sheet != "Data" || res.Hits[0].Cell != "A3" || res.Hits[0].Value != "Bob" {
		t.Fatalf("hits = %+v", res.Hits)
	}
	if res.Truncated {
		t.Fatal("truncated on a full result")
	}
	res, err = findTool(c.Ctx(), findArg{Path: "book.xlsx", Query: "note", Sheet: "Notes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].Cell != "A1" {
		t.Fatalf("hits = %+v", res.Hits)
	}
}

func TestOpenReadErrors(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	store := c.Store()
	if _, err := sheetsTool(c.Ctx(), sheetsArg{Path: "missing.xlsx"}); err == nil {
		t.Fatal("missing file: no error")
	}
	if _, err := store.FilePut(context.Background(), "thread", api.OriginUser, "up.xlsx", "application/octet-stream", strings.NewReader("junk"), false); err != nil {
		t.Fatal(err)
	}
	_, err := sheetsTool(c.Ctx(), sheetsArg{Path: "user_uploads/up.xlsx"})
	if err == nil || !strings.Contains(err.Error(), "virtual") {
		t.Fatalf("virtual err = %v", err)
	}
}

func TestFindEmptyQuery(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	if _, err := findTool(c.Ctx(), findArg{Path: "book.xlsx", Query: ""}); err == nil {
		t.Fatal("empty query: no error")
	}
}

func TestProfileBlankHeaderRow(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A2", "x"); err != nil {
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
	res, err := profileTool(c.Ctx(), profileArg{Path: "book.xlsx", Sheet: "Sheet1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Headers == nil {
		t.Fatal("headers = nil")
	}
}
