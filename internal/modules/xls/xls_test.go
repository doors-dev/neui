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
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/common"
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

func TestWrittenFormulasHaveNoSavedValue(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	ctx := c.Ctx()
	if _, err := writeTool(ctx, writeArg{Path: "book.xlsx", Sheet: "Data", Cell: "E2", Rows: [][]any{{"=B2*3", "=A2&\"-x\""}}}); err != nil {
		t.Fatal(err)
	}
	res, err := readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Range: "E2:F2"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"", ""}; !reflect.DeepEqual(res.Rows[0], want) {
		t.Fatalf("rows = %q, want %q", res.Rows[0], want)
	}
	raw, err := readTool(ctx, readArg{Path: "book.xlsx", Sheet: "Data", Range: "E2:F2", Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"=B2*3", "=A2&\"-x\""}; !reflect.DeepEqual(raw.Rows[0], want) {
		t.Fatalf("raw = %q, want %q", raw.Rows[0], want)
	}
	hits, err := findTool(ctx, findArg{Path: "book.xlsx", Query: "Alice-"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits.Hits) != 0 {
		t.Fatalf("hits = %+v", hits.Hits)
	}
}

func TestReadRawMergedFormula(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	if err := f.SetCellFormula("Sheet1", "A1", "1+1"); err != nil {
		t.Fatal(err)
	}
	if err := f.MergeCell("Sheet1", "A1", "C1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "D1", "x"); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "m.xlsx", f)
	res, err := readTool(c.Ctx(), readArg{Path: "m.xlsx", Sheet: "Sheet1", Range: "A1:D1", Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"=1+1", "", "", "x"}; !reflect.DeepEqual(res.Rows[0], want) {
		t.Fatalf("raw = %q, want %q", res.Rows[0], want)
	}
}

func TestReadRefusesHugeOffset(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	_, err := readTool(c.Ctx(), readArg{Path: "book.xlsx", Sheet: "Data", Range: "A1:B4", Offset: math.MaxUint64})
	if err == nil || !strings.Contains(err.Error(), "offset") {
		t.Fatalf("err = %v", err)
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

func TestOpenRepetitiveWorkbook(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	row := []any{"same text", 12345.678, 42}
	for r := 1; r <= 20000; r++ {
		cell, _ := excelize.CoordinatesToCellName(1, r)
		if err := sw.SetRow(cell, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "big.xlsx", f)
	path, err := fs.LocalPath(context.Background(), "big.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw uint64
	for _, zf := range z.File {
		raw += zf.UncompressedSize64
	}
	z.Close()
	e, err := fs.Stat(context.Background(), "big.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if raw <= 4*uint64(e.Size) {
		t.Fatalf("fixture inflates only %d -> %d", e.Size, raw)
	}
	tight := context.WithValue(c.Ctx(), configKey{}, Config{MaxFileSize: e.Size})
	if _, err := sheetsTool(tight, sheetsArg{Path: "big.xlsx"}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenZipBomb(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	w, err := fs.Create("bomb.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	ct, err := zw.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	ct.Write([]byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`))
	sheet, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.Write(make([]byte, 16<<20)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = sheetsTool(c.Ctx(), sheetsArg{Path: "bomb.xlsx"})
	if err == nil || !strings.Contains(err.Error(), "100x") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenMaxUnzipSize(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	buildBook(t, fs, "book.xlsx")
	low := context.WithValue(c.Ctx(), configKey{}, Config{MaxUnzipSize: 1000})
	_, err := sheetsTool(low, sheetsArg{Path: "book.xlsx"})
	if err == nil || !strings.Contains(err.Error(), "uncompressed") {
		t.Fatalf("err = %v", err)
	}
	small := context.WithValue(c.Ctx(), configKey{}, Config{MaxUnzipSize: 10 << 20})
	if _, err := sheetsTool(small, sheetsArg{Path: "book.xlsx"}); err != nil {
		t.Fatal(err)
	}
}

func TestProfileTypesPastSample(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	for cell, v := range map[string]any{"A1": "Late", "B1": "Mixed", "C1": "Num"} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	for r := 2; r <= 12; r++ {
		if err := f.SetCellValue("Sheet1", "C"+strconv.Itoa(r), r); err != nil {
			t.Fatal(err)
		}
		if err := f.SetCellValue("Sheet1", "B"+strconv.Itoa(r), r); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetCellValue("Sheet1", "A10", 3.5); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "B11", "n/a"); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	res, err := profileTool(c.Ctx(), profileArg{Path: "book.xlsx", Sheet: "Sheet1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"number", "mixed", "number"}; !slices.Equal(res.Types, want) {
		t.Fatalf("types = %v, want %v", res.Types, want)
	}
	if len(res.Sample) != profileSampleRows || res.RowCount != 11 {
		t.Fatalf("sample = %d rows, rowCount = %d", len(res.Sample), res.RowCount)
	}
}

func TestSheetsAutoFilter(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "N"); err != nil {
		t.Fatal(err)
	}
	if err := f.AutoFilter("Sheet1", "A1:C9", nil); err != nil {
		t.Fatal(err)
	}
	writeBook(t, fs, "book.xlsx", f)
	infos, err := sheetsTool(c.Ctx(), sheetsArg{Path: "book.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Filter != "A1:C9" {
		t.Fatalf("infos = %+v", infos)
	}
}

func TestTierModule(t *testing.T) {
	factory := Module(Config{})
	for _, tier := range []common.ModelTier{common.TierArchitect, common.TierAgent} {
		m := factory(context.WithValue(context.Background(), common.TierKey{}, tier))
		if n := len(m.List()); n != 10 {
			t.Fatalf("tier %d registers %d tools, want 10", tier, n)
		}
		read, _ := m.Get("read")
		if params := string(read.Params()); !strings.Contains(params, `"raw"`) || !strings.Contains(params, `"limit"`) {
			t.Fatalf("tier %d read takes %s, want raw and limit", tier, params)
		}
	}
	assistant := factory(context.WithValue(context.Background(), common.TierKey{}, common.TierAssistant))
	var names []string
	for _, tool := range assistant.List() {
		names = append(names, tool.Name())
	}
	if want := []string{"chart", "find", "read", "sheets", "table", "write"}; !slices.Equal(names, want) {
		t.Fatalf("TierAssistant registers %v, want %v", names, want)
	}
	read, _ := assistant.Get("read")
	if params := string(read.Params()); strings.Contains(params, `"raw"`) || strings.Contains(params, `"limit"`) {
		t.Fatalf("TierAssistant read takes %s, want no raw or limit", params)
	}
	c := testkit.NewController(t, testkit.Setup{})
	buildBook(t, c.FS(), "book.xlsx")
	ctx := context.WithValue(c.Ctx(), common.TierKey{}, common.TierAssistant)
	out, err := read.Agent(ctx, json.RawMessage(`{"path":"book.xlsx","sheet":"Data","offset":1,"raw":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var res readResult
	if err := json.Unmarshal([]byte(out[0].(api.ContentText).Text), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 3 || res.Rows[0][0] != "Alice" || res.Rows[0][2] == "45000" || res.Rows[0][3] != "" {
		t.Fatalf("TierAssistant read with raw set gave %v, want formatted values from Alice on", res.Rows)
	}
	over := json.RawMessage(fmt.Sprintf(`{"path":"book.xlsx","sheet":"Data","cell":"F1","rows":[[1]],"repeat":%d}`, maxFormatCells+1))
	write, _ := assistant.Get("write")
	if _, err := write.Agent(ctx, over); err == nil || !strings.HasSuffix(err.Error(), "split it into several writes") {
		t.Fatalf("TierAssistant write refused with %v, want the split hint alone", err)
	}
	write, _ = factory(context.Background()).Get("write")
	if _, err := write.Agent(c.Ctx(), over); err == nil || !strings.HasSuffix(err.Error(), "split it into several writes") {
		t.Fatalf("TierArchitect write refused with %v, want the split hint alone", err)
	}
}
