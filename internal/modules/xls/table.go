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
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const maxFormatCells = 100000

const tableDesc = `Create a styled table with a header row and per-column number formats; creates the workbook and the sheet when they do not exist.`

const formatDesc = `Restyle a range; only the options you pass change.`

var numText = regexp.MustCompile(`^[+-]?((\d{1,3}(,\d{3})+|\d+)(\.\d+)?|\.\d+)$`)

type columnSpec struct {
	Header string `json:"header" required:"true"`
	Format string `json:"format" enum:"text,number,currency,percent,date,integer" description:"default text. number, currency, percent and integer cells take JSON numbers or numeric strings like 1,234.5 ($1,234.5 for currency); percent takes a fraction, 0.15 shows as 15%, and reads a string 15% as 0.15. date cells take ISO 8601 strings, YYYY-MM-DD or YYYY-MM-DDThh:mm:ss with any offset dropped and the wall clock kept, or Excel serial numbers"`
}

type tableArg struct {
	Path    string       `json:"path" required:"true"`
	Sheet   string       `json:"sheet" required:"true"`
	Cell    string       `json:"cell" description:"where the header row starts; default A1"`
	Columns []columnSpec `json:"columns" required:"true"`
	Rows    [][]any      `json:"rows" required:"true" description:"one array per data row, in column order"`
	Preset  string       `json:"preset" enum:"plain,report,financial" description:"default plain"`
	Name    string       `json:"name" description:"absent generates one"`
}

type tableResult struct {
	Range   string `json:"range" description:"the table, header row included"`
	Name    string `json:"name" description:"for formulas like =SUM(Sales[Amount])"`
	Created bool   `json:"created,omitempty" description:"the workbook did not exist and this call made it"`
}

type formatArg struct {
	Path         string `json:"path" required:"true"`
	Sheet        string `json:"sheet" required:"true"`
	Range        string `json:"range" required:"true" description:"the exact cells, like B2:D20"`
	Bold         bool   `json:"bold" description:"false leaves bold as it is"`
	Fill         string `json:"fill" enum:"gray,blue,green,yellow,red"`
	NumberFormat string `json:"numberFormat" enum:"text,number,currency,percent,date,integer" description:"changes display only, text cells stay text"`
	Align        string `json:"align" enum:"left,center,right"`
}

type styleCache map[string]int

func (c styleCache) id(f *excelize.File, key string, build func() (*excelize.Style, error)) (int, error) {
	if id, ok := c[key]; ok {
		return id, nil
	}
	st, err := build()
	if err != nil {
		return 0, err
	}
	id, err := f.NewStyle(st)
	if err != nil {
		return 0, err
	}
	c[key] = id
	return id, nil
}

func applyNumFmt(st *excelize.Style, format string) {
	switch format {
	case "text":
		st.NumFmt, st.CustomNumFmt = 49, nil
	case "number":
		st.NumFmt, st.CustomNumFmt = 4, nil
	case "currency":
		fmtStr := "$#,##0.00"
		st.NumFmt, st.CustomNumFmt = 0, &fmtStr
	case "percent":
		st.NumFmt, st.CustomNumFmt = 10, nil
	case "date":
		st.NumFmt, st.CustomNumFmt = 14, nil
	case "integer":
		st.NumFmt, st.CustomNumFmt = 3, nil
	}
}

var fillPalette = map[string]string{
	"gray":   "D9D9D9",
	"blue":   "DDEBF7",
	"green":  "E2EFDA",
	"yellow": "FFF2CC",
	"red":    "FFC7CE",
}

var presetStyles = map[string]string{
	"plain":     "TableStyleMedium1",
	"report":    "TableStyleMedium2",
	"financial": "TableStyleMedium4",
}

func tableTool(ctx context.Context, arg tableArg) (tableResult, error) {
	if len(arg.Columns) == 0 {
		return tableResult{}, fmt.Errorf("columns must not be empty")
	}
	for i, col := range arg.Columns {
		if strings.TrimSpace(col.Header) == "" {
			return tableResult{}, fmt.Errorf("column %d: header must not be empty", i)
		}
		for _, prev := range arg.Columns[:i] {
			if strings.EqualFold(prev.Header, col.Header) {
				return tableResult{}, fmt.Errorf("column %d: header %q repeats %q, table headers must be unique ignoring case", i, col.Header, prev.Header)
			}
		}
	}
	anchor := arg.Cell
	if anchor == "" {
		anchor = "A1"
	}
	x1, y1, err := excelize.CellNameToCoordinates(anchor)
	if err != nil {
		return tableResult{}, fmt.Errorf("cell %q: %w", anchor, err)
	}
	anchor, err = excelize.CoordinatesToCellName(x1, y1)
	if err != nil {
		return tableResult{}, err
	}
	preset := arg.Preset
	if preset == "" {
		preset = "plain"
	}
	styleName := presetStyles[preset]
	for i, row := range arg.Rows {
		if len(row) > len(arg.Columns) {
			return tableResult{}, fmt.Errorf("row %d has %d cells, table has %d columns", i, len(row), len(arg.Columns))
		}
		for j, v := range row {
			switch v := v.(type) {
			case nil, bool, float64:
			case string:
				if !strings.HasPrefix(v, "=") {
					continue
				}
				fl, err := formula(v)
				if err != nil {
					return tableResult{}, fmt.Errorf("row %d cell %d: %w", i, j, err)
				}
				row[j] = "=" + fl
			default:
				return tableResult{}, fmt.Errorf("row %d cell %d: %T is not a cell value, pass a number, boolean, string or null", i, j, v)
			}
		}
	}
	end, err := excelize.CoordinatesToCellName(x1+len(arg.Columns)-1, y1+max(len(arg.Rows), 1))
	if err != nil {
		return tableResult{}, err
	}
	ref := anchor + ":" + end
	x2, y2, err := excelize.CellNameToCoordinates(end)
	if err != nil {
		return tableResult{}, err
	}
	fs := common.GetController(ctx).FS()
	f, created, release, err := openWrite(ctx, fs, arg.Path, true)
	if err != nil {
		return tableResult{}, err
	}
	defer release()
	newSheet := created
	if created && arg.Sheet != "Sheet1" {
		if err := f.SetSheetName("Sheet1", arg.Sheet); err != nil {
			return tableResult{}, err
		}
	}
	if !created {
		idx, err := f.GetSheetIndex(arg.Sheet)
		if err != nil {
			return tableResult{}, err
		}
		if idx < 0 {
			if _, err := f.NewSheet(arg.Sheet); err != nil {
				return tableResult{}, err
			}
			newSheet = true
		}
	}
	used := make([]bool, len(arg.Columns))
	if !newSheet {
		if err := checkTarget(ctx, fs, f, arg.Path, arg.Sheet, bounds{x1, y1, x2, y2}, false); err != nil {
			return tableResult{}, err
		}
		props, err := f.GetSheetProps(arg.Sheet)
		if err != nil {
			return tableResult{}, err
		}
		def := 9.140625
		if props.DefaultColWidth != nil && *props.DefaultColWidth > 0 {
			def = *props.DefaultColWidth
		} else if props.BaseColWidth != nil && *props.BaseColWidth > 0 {
			def = float64(*props.BaseColWidth)
		}
		for j := range arg.Columns {
			colName, err := excelize.ColumnNumberToName(x1 + j)
			if err != nil {
				return tableResult{}, err
			}
			w, err := f.GetColWidth(arg.Sheet, colName)
			if err != nil {
				return tableResult{}, err
			}
			used[j] = w != def
		}
		rows, err := f.Rows(arg.Sheet)
		if err != nil {
			return tableResult{}, err
		}
		for rows.Next() {
			v, err := rows.Columns(excelize.Options{RawCellValue: true})
			if err != nil {
				rows.Close()
				return tableResult{}, err
			}
			for j := range used {
				used[j] = used[j] || x1-1+j < len(v) && v[x1-1+j] != ""
			}
		}
		if err := rows.Close(); err != nil {
			return tableResult{}, err
		}
	}
	headers := make([]any, len(arg.Columns))
	for i, col := range arg.Columns {
		headers[i] = col.Header
	}
	if err := f.SetSheetRow(arg.Sheet, anchor, &headers); err != nil {
		return tableResult{}, err
	}
	wb, err := f.GetWorkbookProps()
	if err != nil {
		return tableResult{}, err
	}
	epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	if wb.Date1904 != nil && *wb.Date1904 {
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	timed := make([]bool, len(arg.Columns))
	formulas := false
	for i, row := range arg.Rows {
		vals := slices.Clone(row)
		var cells []int
		for j, col := range arg.Columns {
			if j >= len(row) {
				continue
			}
			s, isStr := row[j].(string)
			if isStr && strings.HasPrefix(s, "=") {
				vals[j] = nil
				cells = append(cells, j)
				continue
			}
			switch col.Format {
			case "date":
				switch v := row[j].(type) {
				case string:
					if v == "" {
						vals[j] = nil
						continue
					}
					var t time.Time
					for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05", "2006-01-02 15:04:05", time.RFC3339} {
						if t, err = time.Parse(layout, v); err == nil {
							break
						}
					}
					if err != nil {
						return tableResult{}, fmt.Errorf("row %d column %q: %q is not an ISO 8601 date, use YYYY-MM-DD or YYYY-MM-DDThh:mm:ss", i, col.Header, v)
					}
					if t.Before(epoch) {
						return tableResult{}, fmt.Errorf("row %d column %q: %q is before %s, the first date this workbook can store — pass it as text in a text column", i, col.Header, v, epoch.Format("2006-01-02"))
					}
					h, m, sec := t.Clock()
					timed[j] = timed[j] || h != 0 || m != 0 || sec != 0 || t.Nanosecond() != 0
					vals[j] = t
				case float64:
					timed[j] = timed[j] || v != math.Trunc(v)
				}
			case "number", "currency", "percent", "integer":
				if !isStr {
					continue
				}
				t := strings.TrimSpace(s)
				if t == "" {
					vals[j] = nil
					continue
				}
				pct := col.Format == "percent" && strings.HasSuffix(t, "%")
				if pct {
					t = strings.TrimSuffix(t, "%")
				}
				if col.Format == "currency" {
					t = strings.Replace(t, "$", "", 1)
				}
				n, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", ""), 64)
				if !numText.MatchString(t) || err != nil {
					return tableResult{}, fmt.Errorf("row %d column %q: %q is not a number, pass a JSON number or a string like 1234.5 or 1,234.5", i, col.Header, s)
				}
				if pct {
					n /= 100
				}
				vals[j] = n
			}
		}
		start, err := excelize.CoordinatesToCellName(x1, y1+1+i)
		if err != nil {
			return tableResult{}, err
		}
		if err := f.SetSheetRow(arg.Sheet, start, &vals); err != nil {
			return tableResult{}, err
		}
		for _, j := range cells {
			cell, err := excelize.CoordinatesToCellName(x1+j, y1+1+i)
			if err != nil {
				return tableResult{}, err
			}
			if err := f.SetCellFormula(arg.Sheet, cell, strings.ReplaceAll(row[j].(string)[1:], "{row}", strconv.Itoa(y1+1+i))); err != nil {
				return tableResult{}, err
			}
			formulas = true
		}
	}
	if formulas {
		recalc := true
		if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
			return tableResult{}, err
		}
	}
	cache := styleCache{}
	for j, col := range arg.Columns {
		format := col.Format
		if format == "" || format == "text" || len(arg.Rows) == 0 {
			continue
		}
		key := format
		if timed[j] {
			key = "datetime"
		}
		id, err := cache.id(f, key, func() (*excelize.Style, error) {
			st := &excelize.Style{}
			applyNumFmt(st, format)
			if timed[j] {
				st.NumFmt = 22
			}
			return st, nil
		})
		if err != nil {
			return tableResult{}, err
		}
		top, err := excelize.CoordinatesToCellName(x1+j, y1+1)
		if err != nil {
			return tableResult{}, err
		}
		bottom, err := excelize.CoordinatesToCellName(x1+j, y1+len(arg.Rows))
		if err != nil {
			return tableResult{}, err
		}
		if err := f.SetCellStyle(arg.Sheet, top, bottom, id); err != nil {
			return tableResult{}, err
		}
	}
	for j, col := range arg.Columns {
		if used[j] {
			continue
		}
		w := utf8.RuneCountInString(col.Header)
		for _, row := range arg.Rows {
			if j >= len(row) {
				continue
			}
			if s, ok := row[j].(string); !ok || !strings.HasPrefix(s, "=") {
				w = max(w, utf8.RuneCountInString(fmt.Sprint(row[j])))
			}
		}
		w = min(max(w, 6), 60)
		colName, err := excelize.ColumnNumberToName(x1 + j)
		if err != nil {
			return tableResult{}, err
		}
		if err := f.SetColWidth(arg.Sheet, colName, colName, float64(w)); err != nil {
			return tableResult{}, err
		}
	}
	if err := f.AddTable(arg.Sheet, &excelize.Table{Range: ref, Name: arg.Name, StyleName: styleName}); err != nil {
		return tableResult{}, err
	}
	if newSheet {
		below, err := excelize.CoordinatesToCellName(x1, y1+1)
		if err != nil {
			return tableResult{}, err
		}
		left, err := excelize.CoordinatesToCellName(1, y1+1)
		if err != nil {
			return tableResult{}, err
		}
		freeze := &excelize.Panes{
			Freeze:      true,
			YSplit:      y1,
			TopLeftCell: left,
			ActivePane:  "bottomLeft",
			Selection:   []excelize.Selection{{SQRef: below, ActiveCell: below, Pane: "bottomLeft"}},
		}
		if err := f.SetPanes(arg.Sheet, freeze); err != nil {
			return tableResult{}, err
		}
	}
	name := arg.Name
	if name == "" {
		tables, err := f.GetTables(arg.Sheet)
		if err != nil {
			return tableResult{}, err
		}
		for _, t := range tables {
			if t.Range == ref {
				name = t.Name
			}
		}
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return tableResult{}, err
	}
	return tableResult{Range: ref, Name: name, Created: created}, nil
}

func formatTool(ctx context.Context, arg formatArg) (struct{}, error) {
	if arg.Range == "" {
		return struct{}{}, fmt.Errorf("range must look like B2:D20")
	}
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return struct{}{}, err
	}
	if cells := (b.x2 - b.x1 + 1) * (b.y2 - b.y1 + 1); cells > maxFormatCells {
		return struct{}{}, fmt.Errorf("range %s covers %d cells, limit %d — narrow the range to the cells that hold data", arg.Range, cells, maxFormatCells)
	}
	fs := common.GetController(ctx).FS()
	f, _, release, err := openWrite(ctx, fs, arg.Path, false)
	if err != nil {
		return struct{}{}, err
	}
	defer release()
	cache := styleCache{}
	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			cellName, err := excelize.CoordinatesToCellName(x, y)
			if err != nil {
				return struct{}{}, err
			}
			base, err := f.GetCellStyle(arg.Sheet, cellName)
			if err != nil {
				return struct{}{}, err
			}
			id, err := cache.id(f, strconv.Itoa(base), func() (*excelize.Style, error) {
				st, err := f.GetStyle(base)
				if err != nil {
					return nil, err
				}
				if st == nil {
					st = &excelize.Style{}
				}
				if arg.Bold {
					if st.Font == nil {
						st.Font = &excelize.Font{}
					}
					st.Font.Bold = true
				}
				if arg.Fill != "" {
					st.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{fillPalette[arg.Fill]}}
				}
				if arg.NumberFormat != "" {
					applyNumFmt(st, arg.NumberFormat)
				}
				if arg.Align != "" {
					if st.Alignment == nil {
						st.Alignment = &excelize.Alignment{}
					}
					st.Alignment.Horizontal = arg.Align
				}
				return st, nil
			})
			if err != nil {
				return struct{}{}, err
			}
			if err := f.SetCellStyle(arg.Sheet, cellName, cellName, id); err != nil {
				return struct{}{}, err
			}
		}
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
}
