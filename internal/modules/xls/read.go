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
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const profileSampleRows = 5

const sheetsDesc = `List a workbook's sheets with row and column counts, tables, merged ranges and the autofilter.`
const profileDesc = `Profile a sheet or range: its header row, per-column types, data row count and a sample.`
const readDesc = `Read cell values from a sheet, page by page. A merged cell reports its value at its top-left cell only.`
const findDesc = `Find cells whose value contains a substring.`

type sheetsArg struct {
	Path string `json:"path" required:"true"`
}

type sheetInfo struct {
	Name   string      `json:"name"`
	Rows   int         `json:"rows"`
	Cols   int         `json:"cols"`
	Tables []tableInfo `json:"tables"`
	Merges []string    `json:"merges" description:"merged ranges like A1:B2"`
	Filter string      `json:"autoFilter,omitempty" description:"the sheet's autofilter range, like A1:F200; tables carry their own"`
}

type tableInfo struct {
	Name  string `json:"name"`
	Range string `json:"range"`
}

type profileArg struct {
	Path  string `json:"path" required:"true"`
	Sheet string `json:"sheet" required:"true"`
	Range string `json:"range" description:"like A1:D20, header row included; default the whole sheet"`
}

type profileResult struct {
	Headers  []string   `json:"headers"`
	Types    []string   `json:"types" description:"per column: number, text, date, bool, formula, or mixed when the non-empty cells disagree; guessed from the first data rows, up to the read cap"`
	RowCount int        `json:"rowCount" description:"non-empty data rows below the header"`
	Sample   [][]string `json:"sample" description:"first 5 data rows, formatted"`
}

type readArg struct {
	Path   string `json:"path" required:"true"`
	Sheet  string `json:"sheet" required:"true"`
	Range  string `json:"range" description:"like A1:D20, header row included; default the whole sheet"`
	Offset uint   `json:"offset" description:"0-based row within the range to start at; 0 is the range's first row, header included"`
	Limit  uint   `json:"limit" description:"max rows to return; the server caps large values — rely on truncated and nextOffset to page"`
	Raw    bool   `json:"raw" description:"underlying values instead of formatted ones: date serials, unformatted numbers, formulas"`
}

type readResult struct {
	Rows       [][]string `json:"rows"`
	NextOffset uint       `json:"nextOffset" description:"pass as offset to continue"`
	Truncated  bool       `json:"truncated,omitempty" description:"more rows remain"`
}

type findArg struct {
	Path  string `json:"path" required:"true"`
	Query string `json:"query" required:"true" description:"substring to look for in formatted cell values"`
	Sheet string `json:"sheet" description:"restrict to one sheet; default all"`
}

type findHit struct {
	Sheet string `json:"sheet"`
	Cell  string `json:"cell"`
	Value string `json:"value"`
}

type findResult struct {
	Hits      []findHit `json:"hits"`
	Truncated bool      `json:"truncated,omitempty" description:"hit cap reached, more matches may exist; narrow the query or restrict to one sheet"`
}

type bounds struct {
	x1, y1, x2, y2 int
}

func rangeBounds(ref string) (bounds, error) {
	if ref == "" {
		return bounds{1, 1, excelize.MaxColumns, excelize.TotalRows}, nil
	}
	from, to, ok := strings.Cut(ref, ":")
	if !ok {
		return bounds{}, fmt.Errorf("range %q must look like A1:D20", ref)
	}
	x1, y1, err := excelize.CellNameToCoordinates(from)
	if err != nil {
		return bounds{}, fmt.Errorf("range %q: %w", ref, err)
	}
	x2, y2, err := excelize.CellNameToCoordinates(to)
	if err != nil {
		return bounds{}, fmt.Errorf("range %q: %w", ref, err)
	}
	if x2 < x1 || y2 < y1 {
		return bounds{}, fmt.Errorf("range %q ends before it starts", ref)
	}
	return bounds{x1, y1, x2, y2}, nil
}

func (b bounds) ref() string {
	from, _ := excelize.CoordinatesToCellName(b.x1, b.y1)
	to, _ := excelize.CoordinatesToCellName(b.x2, b.y2)
	return from + ":" + to
}

func (b bounds) window(v []string) []string {
	if b.x1 == 1 && b.x2 == excelize.MaxColumns {
		return v
	}
	w := make([]string, b.x2-b.x1+1)
	for i := range w {
		if x := b.x1 - 1 + i; x < len(v) {
			w[i] = v[x]
		}
	}
	return w
}

func empty(v []string) bool {
	for _, s := range v {
		if s != "" {
			return false
		}
	}
	return true
}

func sheetsTool(ctx context.Context, arg sheetsArg) ([]sheetInfo, error) {
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	local, err := fs.LocalPath(ctx, arg.Path)
	if err != nil {
		return nil, err
	}
	infos := []sheetInfo{}
	for _, name := range f.GetSheetList() {
		info := sheetInfo{Name: name, Tables: []tableInfo{}, Merges: []string{}}
		rows, err := f.Rows(name)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			info.Rows++
			v, err := rows.Columns()
			if err != nil {
				rows.Close()
				return nil, err
			}
			info.Cols = max(info.Cols, len(v))
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		tables, err := f.GetTables(name)
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			info.Tables = append(info.Tables, tableInfo{Name: t.Name, Range: t.Range})
		}
		merges, err := f.GetMergeCells(name)
		if err != nil {
			return nil, err
		}
		for _, mc := range merges {
			info.Merges = append(info.Merges, mc.GetStartAxis()+":"+mc.GetEndAxis())
		}
		if info.Filter, _, err = scanSheet(local, name); err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func profileTool(ctx context.Context, arg profileArg) (profileResult, error) {
	c := cfg(ctx)
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return profileResult{}, err
	}
	defer f.Close()
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return profileResult{}, err
	}
	rows, err := f.Rows(arg.Sheet)
	if err != nil {
		return profileResult{}, err
	}
	res := profileResult{Headers: []string{}, Sample: [][]string{}}
	var sampled []int
	num := 0
	for rows.Next() {
		num++
		if num < b.y1 {
			continue
		}
		if num > b.y2 {
			break
		}
		v, err := rows.Columns()
		if err != nil {
			rows.Close()
			return profileResult{}, err
		}
		w := b.window(v)
		if num == b.y1 {
			if w != nil {
				res.Headers = w
			}
			continue
		}
		if empty(w) {
			continue
		}
		res.RowCount++
		if len(res.Sample) < profileSampleRows {
			res.Sample = append(res.Sample, w)
		}
		if uint(len(sampled)) < c.MaxReadRows {
			sampled = append(sampled, num)
		}
	}
	if err := rows.Close(); err != nil {
		return profileResult{}, err
	}
	res.Types = make([]string, len(res.Headers))
	for i := range res.Types {
		res.Types[i] = columnType(f, arg.Sheet, b.x1+i, sampled)
	}
	return res, nil
}

func columnType(f *excelize.File, sheet string, col int, rowNums []int) string {
	guess := ""
	for _, r := range rowNums {
		cell, err := excelize.CoordinatesToCellName(col, r)
		if err != nil {
			return "text"
		}
		typ := "text"
		if fl, _ := f.GetCellFormula(sheet, cell); fl != "" {
			typ = "formula"
		} else if v, _ := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true}); v == "" {
			continue
		} else if ct, _ := f.GetCellType(sheet, cell); ct == excelize.CellTypeBool {
			typ = "bool"
		} else if ct == excelize.CellTypeDate {
			typ = "date"
		} else if ct == excelize.CellTypeUnset || ct == excelize.CellTypeNumber {
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				typ = "number"
			}
			if styleID, err := f.GetCellStyle(sheet, cell); err == nil {
				if st, err := f.GetStyle(styleID); err == nil && dateFmt(st) {
					typ = "date"
				}
			}
		}
		if guess != "" && guess != typ {
			return "mixed"
		}
		guess = typ
	}
	if guess == "" {
		return "text"
	}
	return guess
}

func dateFmt(st *excelize.Style) bool {
	if st == nil {
		return false
	}
	id := st.NumFmt
	if id >= 14 && id <= 22 || id >= 27 && id <= 36 || id >= 45 && id <= 47 || id >= 50 && id <= 58 {
		return true
	}
	if st.CustomNumFmt != nil {
		return strings.ContainsAny(stripFmtLiterals(strings.ToLower(*st.CustomNumFmt)), "ydh")
	}
	return false
}

func stripFmtLiterals(s string) string {
	var b strings.Builder
	inQuote, inBracket := false, false
	for _, r := range s {
		switch {
		case inQuote:
			if r == '"' {
				inQuote = false
			}
		case inBracket:
			if r == ']' {
				inBracket = false
			}
		case r == '"':
			inQuote = true
		case r == '[':
			inBracket = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func readTool(ctx context.Context, arg readArg) (readResult, error) {
	c := cfg(ctx)
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return readResult{}, err
	}
	defer f.Close()
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return readResult{}, err
	}
	limit := arg.Limit
	if limit == 0 || limit > c.MaxReadRows {
		limit = c.MaxReadRows
	}
	if arg.Offset > excelize.TotalRows {
		return readResult{}, fmt.Errorf("offset %d is past the last sheet row %d", arg.Offset, excelize.TotalRows)
	}
	rows, err := f.Rows(arg.Sheet)
	if err != nil {
		return readResult{}, err
	}
	start := b.y1 + int(arg.Offset)
	res := readResult{Rows: [][]string{}}
	var nums []int
	num := 0
	for rows.Next() {
		num++
		if num < start {
			continue
		}
		if num > b.y2 {
			break
		}
		if uint(len(res.Rows)) == limit {
			res.Truncated = true
			break
		}
		var v []string
		if arg.Raw {
			v, err = rows.Columns(excelize.Options{RawCellValue: true})
		} else {
			v, err = rows.Columns()
		}
		if err != nil {
			rows.Close()
			return readResult{}, err
		}
		res.Rows = append(res.Rows, b.window(v))
		nums = append(nums, num)
	}
	if err := rows.Close(); err != nil {
		return readResult{}, err
	}
	if arg.Raw {
		merges, err := f.GetMergeCells(arg.Sheet, true)
		if err != nil {
			return readResult{}, err
		}
		var spans []bounds
		for _, mc := range merges {
			if m, err := rangeBounds(mc.GetStartAxis() + ":" + mc.GetEndAxis()); err == nil {
				spans = append(spans, m)
			}
		}
		for i, n := range nums {
			row := res.Rows[i]
			for j := range row {
				x := b.x1 + j
				if slices.ContainsFunc(spans, func(m bounds) bool {
					return m.x1 <= x && x <= m.x2 && m.y1 <= n && n <= m.y2 && (x != m.x1 || n != m.y1)
				}) {
					continue
				}
				cell, err := excelize.CoordinatesToCellName(x, n)
				if err != nil {
					return readResult{}, err
				}
				if fl, _ := f.GetCellFormula(arg.Sheet, cell); fl != "" {
					row[j] = "=" + fl
				}
			}
		}
	}
	res.NextOffset = arg.Offset + uint(len(res.Rows))
	return res, nil
}

func findTool(ctx context.Context, arg findArg) (findResult, error) {
	if arg.Query == "" {
		return findResult{}, fmt.Errorf("query must not be empty")
	}
	c := cfg(ctx)
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return findResult{}, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if arg.Sheet != "" {
		sheets = []string{arg.Sheet}
	}
	res := findResult{Hits: []findHit{}}
	for _, sheet := range sheets {
		rows, err := f.Rows(sheet)
		if err != nil {
			return findResult{}, err
		}
		num := 0
		for rows.Next() {
			num++
			v, err := rows.Columns()
			if err != nil {
				rows.Close()
				return findResult{}, err
			}
			for i, val := range v {
				if val == "" || !strings.Contains(val, arg.Query) {
					continue
				}
				cell, err := excelize.CoordinatesToCellName(i+1, num)
				if err != nil {
					rows.Close()
					return findResult{}, err
				}
				res.Hits = append(res.Hits, findHit{Sheet: sheet, Cell: cell, Value: val})
				if uint(len(res.Hits)) == c.MaxReadRows {
					res.Truncated = true
					rows.Close()
					return res, nil
				}
			}
		}
		if err := rows.Close(); err != nil {
			return findResult{}, err
		}
	}
	return res, nil
}
