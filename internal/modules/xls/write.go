package xls

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/common"
)

const writeDesc = `Write values or formulas into empty cells, row by row from a top-left cell. A string starting with = is a formula; {row} in a formula becomes the row number of the cell it lands in, so one row repeated fills a column: cell E2, rows [["=B{row}*C{row}"]], repeat 19 fills E2:E20. Every target cell must be empty, and the block must sit wholly inside or wholly outside each table and the sheet's autofilter; merged cells and array formulas are refused. Cells written next to a table stay outside it — tables are never resized.`

var dynamicFn = regexp.MustCompile(`(?i)(?:^|[^\w.])(?:_xlfn\.)?(?:_xlws\.)?(FILTER|SORT|SORTBY|UNIQUE|SEQUENCE|RANDARRAY|XLOOKUP|XMATCH|LET|LAMBDA|TEXTSPLIT|TEXTBEFORE|TEXTAFTER|VSTACK|HSTACK|TOCOL|TOROW|TAKE|DROP|CHOOSEROWS|CHOOSECOLS|EXPAND|WRAPROWS|WRAPCOLS)\s*\(`)

var externalRef = regexp.MustCompile(`(?i)\[[^\[\]]*\.(xlsx|xlsm|xlsb|xltx|xls|csv)\]|\[\d+\][^\[\]()]*!`)

type writeArg struct {
	Path   string  `json:"path" required:"true"`
	Sheet  string  `json:"sheet" required:"true"`
	Cell   string  `json:"cell" required:"true" description:"top-left cell of the block, like E1"`
	Rows   [][]any `json:"rows" required:"true" description:"one array per row: numbers, booleans and text are stored as given, a string starting with = is a formula, null leaves the cell empty"`
	Repeat uint    `json:"repeat" description:"write rows this many times, stacked downward; default 1"`
}

type writeResult struct {
	Range string `json:"range" description:"the block written, like E2:E20"`
}

func writeTool(ctx context.Context, arg writeArg) (writeResult, error) {
	x1, y1, err := excelize.CellNameToCoordinates(arg.Cell)
	if err != nil {
		return writeResult{}, fmt.Errorf("cell %q: %w", arg.Cell, err)
	}
	width := 0
	for i, row := range arg.Rows {
		width = max(width, len(row))
		for j, v := range row {
			switch v := v.(type) {
			case nil, bool, float64:
			case string:
				if !strings.HasPrefix(v, "=") {
					continue
				}
				if strings.TrimSpace(v[1:]) == "" {
					return writeResult{}, fmt.Errorf("row %d cell %d: formula %q is empty", i, j, v)
				}
				if m := dynamicFn.FindStringSubmatch(v[1:]); m != nil {
					return writeResult{}, fmt.Errorf("row %d cell %d: %s is a dynamic-array or Excel 365 function, which is not supported — use classic functions such as INDEX/MATCH, SUMIFS or COUNTIFS", i, j, strings.ToUpper(m[1]))
				}
				if externalRef.MatchString(v[1:]) {
					return writeResult{}, fmt.Errorf("row %d cell %d: formula %q links another workbook, and external links are not supported — copy the data into this workbook first", i, j, v)
				}
			default:
				return writeResult{}, fmt.Errorf("row %d cell %d: %T is not a cell value, pass a number, boolean, string or null", i, j, v)
			}
		}
	}
	if width == 0 {
		return writeResult{}, fmt.Errorf("rows hold no cells")
	}
	height := len(arg.Rows) * int(max(arg.Repeat, 1))
	if cells := width * height; cells > maxFormatCells {
		return writeResult{}, fmt.Errorf("block covers %d cells, limit %d — split it into several writes, such as from a js script", cells, maxFormatCells)
	}
	start, err := excelize.CoordinatesToCellName(x1, y1)
	if err != nil {
		return writeResult{}, err
	}
	end, err := excelize.CoordinatesToCellName(x1+width-1, y1+height-1)
	if err != nil {
		return writeResult{}, fmt.Errorf("block from %s does not fit on a sheet: %w", start, err)
	}
	fs := common.GetController(ctx).FS()
	f, _, release, err := openWrite(ctx, fs, arg.Path, false)
	if err != nil {
		return writeResult{}, err
	}
	defer release()
	if err := checkTarget(ctx, fs, f, arg.Path, arg.Sheet, bounds{x1, y1, x1 + width - 1, y1 + height - 1}, true); err != nil {
		return writeResult{}, err
	}
	formulas := false
	for i := range height {
		for j, v := range arg.Rows[i%len(arg.Rows)] {
			if v == nil {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(x1+j, y1+i)
			if err != nil {
				return writeResult{}, err
			}
			if s, ok := v.(string); ok && strings.HasPrefix(s, "=") {
				formulas = true
				if err := f.SetCellFormula(arg.Sheet, cell, strings.ReplaceAll(s[1:], "{row}", strconv.Itoa(y1+i))); err != nil {
					return writeResult{}, err
				}
				continue
			}
			if err := f.SetCellValue(arg.Sheet, cell, v); err != nil {
				return writeResult{}, err
			}
		}
	}
	if formulas {
		recalc := true
		if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
			return writeResult{}, err
		}
	}
	if err := save(ctx, fs, f, arg.Path); err != nil {
		return writeResult{}, err
	}
	return writeResult{Range: start + ":" + end}, nil
}

func checkTarget(ctx context.Context, fs api.FS, f *excelize.File, name, sheet string, t bounds, nested bool) error {
	ref := t.ref()
	overlaps := func(o bounds) bool {
		return o.x1 <= t.x2 && t.x1 <= o.x2 && o.y1 <= t.y2 && t.y1 <= o.y2
	}
	holds := func(o bounds) bool {
		return o.x1 <= t.x1 && t.x2 <= o.x2 && o.y1 <= t.y1 && t.y2 <= o.y2
	}
	tables, err := f.GetTables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		b, err := rangeBounds(tb.Range)
		if err != nil || !overlaps(b) || nested && holds(b) {
			continue
		}
		if nested {
			return fmt.Errorf("%s partly overlaps table %q at %s: write wholly inside the table's empty cells or wholly outside it — resizing a table is not supported", ref, tb.Name, tb.Range)
		}
		return fmt.Errorf("%s overlaps table %q at %s: pick cells clear of it — tables are never resized or nested", ref, tb.Name, tb.Range)
	}
	local, err := fs.LocalPath(ctx, name)
	if err != nil {
		return err
	}
	filter, arrays, err := scanSheet(local, sheet)
	if err != nil {
		return err
	}
	if filter != "" {
		if b, err := rangeBounds(filter); err == nil && overlaps(b) && !(nested && holds(b)) {
			if nested {
				return fmt.Errorf("%s partly overlaps the autofilter at %s: write wholly inside or wholly outside it — changing an autofilter is not supported", ref, filter)
			}
			return fmt.Errorf("%s overlaps the autofilter at %s: pick cells clear of it — changing an autofilter is not supported", ref, filter)
		}
	}
	merges, err := f.GetMergeCells(sheet, true)
	if err != nil {
		return err
	}
	for _, mc := range merges {
		m := mc.GetStartAxis() + ":" + mc.GetEndAxis()
		if b, err := rangeBounds(m); err == nil && overlaps(b) {
			return fmt.Errorf("%s overlaps merged cells %s: pick cells clear of them", ref, m)
		}
	}
	for _, a := range arrays {
		if b, err := rangeBounds(a); err == nil && overlaps(b) {
			return fmt.Errorf("%s overlaps the array formula at %s: pick cells clear of it", ref, a)
		}
	}
	for y := t.y1; y <= t.y2; y++ {
		for x := t.x1; x <= t.x2; x++ {
			cell, err := excelize.CoordinatesToCellName(x, y)
			if err != nil {
				return err
			}
			v, err := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
			if err != nil {
				return err
			}
			if v == "" {
				if v, err = f.GetCellFormula(sheet, cell); err != nil {
					return err
				}
			}
			if v != "" {
				return fmt.Errorf("%s is not empty (%s holds %q): the xls tools only write into empty cells — pick an empty spot, such as the columns right of the data or a new sheet", ref, cell, v)
			}
		}
	}
	return nil
}
