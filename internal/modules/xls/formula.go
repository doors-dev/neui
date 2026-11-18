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
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const totalDesc = `Add a summary row in the empty row right under a range: a SUBTOTAL per chosen column, bold with a top border, each cell keeping its column's number format (counts excepted). Plain ranges only: a range that is or overlaps a table is refused, since total rows inside tables are not supported — total a table with write instead, e.g. =SUBTOTAL(109,Sales[Amount]) in a cell clear of it. The summary row must also be clear of tables, the autofilter and merged cells.`

type aggSpec struct {
	Column string `json:"column" required:"true" description:"header of a column in the range, like Amount; a column letter inside the range, like B, also works"`
	Fn     string `json:"fn" required:"true" description:"sum, avg, min, max, count (non-empty cells, text included — Excel Total Row 'Count', COUNTA) or count_nums (numeric cells only — 'Count Numbers', COUNT). All are SUBTOTAL 1xx, so filtered and hidden rows are skipped."`
}

type totalArg struct {
	Path  string    `json:"path" required:"true"`
	Sheet string    `json:"sheet" required:"true"`
	Range string    `json:"range" required:"true" description:"data range to summarize, header row included, like A1:C20; the summary goes in the row below"`
	Aggs  []aggSpec `json:"aggs" required:"true"`
}

type totalResult struct {
	Row   int    `json:"row" description:"row number of the summary row"`
	Range string `json:"range" description:"the summary row, like A21:C21"`
}

var aggFns = map[string]int{
	"sum":        109,
	"avg":        101,
	"count":      103,
	"count_nums": 102,
	"min":        105,
	"max":        104,
}

func totalTool(ctx context.Context, arg totalArg) (totalResult, error) {
	if len(arg.Aggs) == 0 {
		return totalResult{}, fmt.Errorf("aggs must name at least one column")
	}
	for _, a := range arg.Aggs {
		if _, ok := aggFns[strings.ToLower(a.Fn)]; !ok {
			return totalResult{}, fmt.Errorf("agg fn %q: pick sum, avg, count, count_nums, min or max", a.Fn)
		}
	}
	if arg.Range == "" {
		return totalResult{}, fmt.Errorf("range must look like A1:C20")
	}
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return totalResult{}, err
	}
	if b.y2 == b.y1 {
		return totalResult{}, fmt.Errorf("range %s has no data rows below its header", arg.Range)
	}
	fs := common.GetController(ctx).FS()
	f, _, release, err := openWrite(ctx, fs, arg.Path, false)
	if err != nil {
		return totalResult{}, err
	}
	defer release()
	tables, err := f.GetTables(arg.Sheet)
	if err != nil {
		return totalResult{}, err
	}
	for _, t := range tables {
		tb, err := rangeBounds(t.Range)
		if err != nil || tb.x1 > b.x2 || b.x1 > tb.x2 || tb.y1 > b.y2 || b.y1 > tb.y2 {
			continue
		}
		if tb == b {
			return totalResult{}, fmt.Errorf("range %s is table %q: total rows inside tables are not supported — write =SUBTOTAL(109,%s[Column]) with write in a cell clear of the table instead", arg.Range, t.Name, t.Name)
		}
		return totalResult{}, fmt.Errorf("range %s overlaps table %q at %s: total goes under plain ranges only — total a table with write, e.g. =SUBTOTAL(109,%s[Column]) in a cell clear of it", arg.Range, t.Name, t.Range, t.Name)
	}
	row := b.y2 + 1
	target := bounds{b.x1, row, b.x2, row}
	if err := checkTarget(ctx, fs, f, arg.Path, arg.Sheet, target, false); err != nil {
		return totalResult{}, err
	}
	headers := make([]string, b.x2-b.x1+1)
	for i := range headers {
		cell, err := excelize.CoordinatesToCellName(b.x1+i, b.y1)
		if err != nil {
			return totalResult{}, err
		}
		if headers[i], err = f.GetCellValue(arg.Sheet, cell); err != nil {
			return totalResult{}, err
		}
	}
	fns := map[int]int{}
	for _, a := range arg.Aggs {
		x := 0
		for i, h := range headers {
			if x == 0 && h != "" && strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(a.Column)) {
				x = b.x1 + i
			}
		}
		if n, err := excelize.ColumnNameToNumber(a.Column); x == 0 && err == nil && b.x1 <= n && n <= b.x2 {
			x = n
		}
		if x == 0 {
			return totalResult{}, fmt.Errorf("agg column %q is neither a header of %s (%s) nor a column letter inside it", a.Column, arg.Range, strings.Join(headers, ", "))
		}
		if _, ok := fns[x]; ok {
			return totalResult{}, fmt.Errorf("agg column %q is named twice", a.Column)
		}
		fns[x] = aggFns[strings.ToLower(a.Fn)]
	}
	cache := styleCache{}
	for x := b.x1; x <= b.x2; x++ {
		above, err := excelize.CoordinatesToCellName(x, b.y2)
		if err != nil {
			return totalResult{}, err
		}
		cell, err := excelize.CoordinatesToCellName(x, row)
		if err != nil {
			return totalResult{}, err
		}
		base, err := f.GetCellStyle(arg.Sheet, above)
		if err != nil {
			return totalResult{}, err
		}
		fn := fns[x]
		counts := fn == 102 || fn == 103
		id, err := cache.id(f, fmt.Sprint(base, counts), func() (*excelize.Style, error) {
			st := &excelize.Style{
				Font:   &excelize.Font{Bold: true},
				Border: []excelize.Border{{Type: "top", Color: "000000", Style: 1}},
			}
			src, err := f.GetStyle(base)
			if err != nil {
				return nil, err
			}
			if src != nil && !counts {
				st.NumFmt, st.DecimalPlaces, st.CustomNumFmt, st.NegRed = src.NumFmt, src.DecimalPlaces, src.CustomNumFmt, src.NegRed
			}
			return st, nil
		})
		if err != nil {
			return totalResult{}, err
		}
		if err := f.SetCellStyle(arg.Sheet, cell, cell, id); err != nil {
			return totalResult{}, err
		}
		if fn == 0 {
			continue
		}
		col, err := excelize.ColumnNumberToName(x)
		if err != nil {
			return totalResult{}, err
		}
		if err := f.SetCellFormula(arg.Sheet, cell, fmt.Sprintf("SUBTOTAL(%d,%s%d:%s%d)", fn, col, b.y1+1, col, b.y2)); err != nil {
			return totalResult{}, err
		}
	}
	recalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
		return totalResult{}, err
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return totalResult{}, err
	}
	return totalResult{Row: row, Range: target.ref()}, nil
}
