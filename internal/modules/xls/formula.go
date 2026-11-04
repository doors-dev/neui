package xls

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const columnDesc = `Append a computed column to the right of a range: a header plus per-row formulas from a template, {row} replaced with each row number, e.g. =B{row}*C{row}.`
const totalDesc = `Add a summary row below a range with an aggregate per chosen column, bold with a top border.`
const checkDesc = `Spot-check a workbook's formulas: evaluate each one and collect the failures. The evaluator is not Excel — every error it reports is real, but a clean pass is encouraging rather than proof, especially for uncommon functions.`

type columnArg struct {
	Path    string `json:"path" required:"true"`
	Sheet   string `json:"sheet" required:"true"`
	Range   string `json:"range" required:"true" description:"data range the column extends, like A1:C20, header row included"`
	Header  string `json:"header" required:"true"`
	Formula string `json:"formula" required:"true" description:"template with {row}, e.g. =B{row}*C{row}"`
}

type aggSpec struct {
	Column string `json:"column" required:"true" description:"column letter, like B"`
	Fn     string `json:"fn" required:"true" description:"sum, avg, count, min or max"`
}

type totalArg struct {
	Path  string    `json:"path" required:"true"`
	Sheet string    `json:"sheet" required:"true"`
	Range string    `json:"range" required:"true" description:"data range to summarize, like A1:C20, header row included"`
	Aggs  []aggSpec `json:"aggs" required:"true"`
}

type columnResult struct {
	Column string `json:"column" description:"letter of the new column"`
	Range  string `json:"range" description:"the new column, header included, like D1:D20"`
}

type totalResult struct {
	Row   int    `json:"row" description:"row number of the summary row"`
	Range string `json:"range" description:"the summary row, like A21:C21"`
}

type checkArg struct {
	Path string `json:"path" required:"true"`
}

type cellError struct {
	Cell  string `json:"cell" description:"like Sheet1!D4"`
	Error string `json:"error"`
}

type checkResult struct {
	Formulas int         `json:"formulas" description:"formula cells checked"`
	Errors   []cellError `json:"errors"`
}

var aggFns = map[string]int{
	"sum":   109,
	"avg":   101,
	"count": 103,
	"min":   105,
	"max":   104,
}

func columnTool(ctx context.Context, arg columnArg) (columnResult, error) {
	if !strings.Contains(arg.Formula, "{row}") {
		return columnResult{}, fmt.Errorf("formula template %q has no {row} placeholder", arg.Formula)
	}
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return columnResult{}, err
	}
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return columnResult{}, err
	}
	defer f.Close()
	col := b.x2 + 1
	head, err := excelize.CoordinatesToCellName(col, b.y1)
	if err != nil {
		return columnResult{}, err
	}
	if err := f.SetCellValue(arg.Sheet, head, arg.Header); err != nil {
		return columnResult{}, err
	}
	for r := b.y1 + 1; r <= b.y2; r++ {
		cell, err := excelize.CoordinatesToCellName(col, r)
		if err != nil {
			return columnResult{}, err
		}
		formula := strings.TrimPrefix(strings.ReplaceAll(arg.Formula, "{row}", strconv.Itoa(r)), "=")
		if err := f.SetCellFormula(arg.Sheet, cell, formula); err != nil {
			return columnResult{}, err
		}
	}
	if err := f.UpdateLinkedValue(); err != nil {
		return columnResult{}, err
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return columnResult{}, err
	}
	colName, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return columnResult{}, err
	}
	bottom, err := excelize.CoordinatesToCellName(col, b.y2)
	if err != nil {
		return columnResult{}, err
	}
	return columnResult{Column: colName, Range: head + ":" + bottom}, nil
}

func totalTool(ctx context.Context, arg totalArg) (totalResult, error) {
	if len(arg.Aggs) == 0 {
		return totalResult{}, fmt.Errorf("aggs must name at least one column")
	}
	b, err := rangeBounds(arg.Range)
	if err != nil {
		return totalResult{}, err
	}
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return totalResult{}, err
	}
	defer f.Close()
	row := b.y2 + 1
	for _, a := range arg.Aggs {
		fn, ok := aggFns[strings.ToLower(a.Fn)]
		if !ok {
			return totalResult{}, fmt.Errorf("agg fn %q: pick sum, avg, count, min or max", a.Fn)
		}
		x, err := excelize.ColumnNameToNumber(a.Column)
		if err != nil {
			return totalResult{}, fmt.Errorf("agg column %q: %w", a.Column, err)
		}
		col, err := excelize.ColumnNumberToName(x)
		if err != nil {
			return totalResult{}, err
		}
		cell, err := excelize.CoordinatesToCellName(x, row)
		if err != nil {
			return totalResult{}, err
		}
		formula := fmt.Sprintf("SUBTOTAL(%d,%s%d:%s%d)", fn, col, b.y1+1, col, b.y2)
		if err := f.SetCellFormula(arg.Sheet, cell, formula); err != nil {
			return totalResult{}, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Border: []excelize.Border{{Type: "top", Color: "000000", Style: 1}},
	})
	if err != nil {
		return totalResult{}, err
	}
	from, err := excelize.CoordinatesToCellName(b.x1, row)
	if err != nil {
		return totalResult{}, err
	}
	to, err := excelize.CoordinatesToCellName(b.x2, row)
	if err != nil {
		return totalResult{}, err
	}
	if err := f.SetCellStyle(arg.Sheet, from, to, style); err != nil {
		return totalResult{}, err
	}
	if err := f.UpdateLinkedValue(); err != nil {
		return totalResult{}, err
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return totalResult{}, err
	}
	return totalResult{Row: row, Range: from + ":" + to}, nil
}

func checkTool(ctx context.Context, arg checkArg) (checkResult, error) {
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return checkResult{}, err
	}
	defer f.Close()
	res := checkResult{Errors: []cellError{}}
	for _, sheet := range f.GetSheetList() {
		rows, err := f.Rows(sheet)
		if err != nil {
			return checkResult{}, err
		}
		num := 0
		for rows.Next() {
			num++
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
				res.Formulas++
				val, err := f.CalcCellValue(sheet, cell)
				if err != nil {
					res.Errors = append(res.Errors, cellError{Cell: sheet + "!" + cell, Error: err.Error()})
					continue
				}
				if strings.HasPrefix(val, "#") {
					res.Errors = append(res.Errors, cellError{Cell: sheet + "!" + cell, Error: val})
				}
			}
		}
		if err := rows.Close(); err != nil {
			return checkResult{}, err
		}
	}
	return res, nil
}
