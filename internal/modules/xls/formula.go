package xls

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const maxCheckCells = 10000

const totalDesc = `Add a summary row in the empty row right under a range: a SUBTOTAL per chosen column, bold with a top border, each cell keeping its column's number format (counts excepted). Plain ranges only: a range that is or overlaps a table is refused, since total rows inside tables are not supported — total a table with write instead, e.g. =SUBTOTAL(109,Sales[Amount]) in a cell clear of it. The summary row must also be clear of tables, the autofilter and merged cells.`
const checkDesc = `Spot-check a workbook's formulas with excelize's evaluator, which is not Excel: errors it reports are real, but a clean pass is encouraging rather than proof. Functions and references it cannot compute come back in uncalculated and their cells are skipped, never counted as passing — check those in Excel. SUBTOTAL 101-111 evaluate like 1-11, ignoring hidden rows.`

var unsupportedFn = regexp.MustCompile(`not support (\S+) function`)

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

type checkArg struct {
	Path string `json:"path" required:"true"`
}

type cellError struct {
	Cell  string `json:"cell" description:"like 'Sheet1'!D4"`
	Error string `json:"error"`
}

type checkResult struct {
	Formulas     int         `json:"formulas" description:"formula cells evaluated"`
	Errors       []cellError `json:"errors" description:"cells that evaluated to an error, like #DIV/0!"`
	Uncalculated []string    `json:"uncalculated" description:"functions and references the evaluator cannot compute"`
	Skipped      int         `json:"skipped" description:"formula cells not evaluated because they use, or depend on a cell that uses, something in uncalculated"`
	Truncated    bool        `json:"truncated,omitempty" description:"stopped early, at the formula cell cap or with the error list full; the rest of the workbook was not checked"`
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

func checkTool(ctx context.Context, arg checkArg) (checkResult, error) {
	c := cfg(ctx)
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return checkResult{}, err
	}
	defer f.Close()
	res := checkResult{Errors: []cellError{}}
	uncalculated := map[string]bool{}
	for _, sheet := range f.GetSheetList() {
		rows, err := f.Rows(sheet)
		if err != nil {
			return checkResult{}, err
		}
		num := 0
		for rows.Next() && !res.Truncated {
			num++
			if err := ctx.Err(); err != nil {
				rows.Close()
				return checkResult{}, err
			}
			v, err := rows.Columns()
			if err != nil {
				rows.Close()
				return checkResult{}, err
			}
			for i := range v {
				cell, err := excelize.CoordinatesToCellName(i+1, num)
				if err != nil {
					rows.Close()
					return checkResult{}, err
				}
				fl, _ := f.GetCellFormula(sheet, cell)
				if fl == "" {
					continue
				}
				if res.Formulas+res.Skipped == maxCheckCells || uint(len(res.Errors)) == c.MaxReadRows {
					res.Truncated = true
					break
				}
				val, err := f.CalcCellValue(sheet, cell)
				if err != nil {
					if m := unsupportedFn.FindStringSubmatch(err.Error()); m != nil {
						uncalculated[m[1]] = true
						res.Skipped++
						continue
					}
					if strings.Contains(err.Error(), "invalid reference") {
						uncalculated["references it cannot resolve, such as Table[Column]"] = true
						res.Skipped++
						continue
					}
				}
				res.Formulas++
				ref := sheetRef(sheet) + "!" + cell
				if err != nil {
					res.Errors = append(res.Errors, cellError{Cell: ref, Error: err.Error()})
				} else if strings.HasPrefix(val, "#") {
					res.Errors = append(res.Errors, cellError{Cell: ref, Error: val})
				}
			}
		}
		if err := rows.Close(); err != nil {
			return checkResult{}, err
		}
		if res.Truncated {
			break
		}
	}
	res.Uncalculated = slices.Sorted(maps.Keys(uncalculated))
	if res.Uncalculated == nil {
		res.Uncalculated = []string{}
	}
	return res, nil
}
