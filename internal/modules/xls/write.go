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
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/common"
)

const writeDesc = `Write a block of values or formulas. One row repeated fills a column: cell E2, rows [["=B{row}*C{row}"]], repeat 19 fills E2:E20.`

var futureFn = map[string]bool{
	"ACOT": true, "ACOTH": true, "AGGREGATE": true, "ARABIC": true, "ARRAYTOTEXT": true, "BASE": true, "BETA.DIST": true, "BETA.INV": true,
	"BINOM.DIST": true, "BINOM.DIST.RANGE": true, "BINOM.INV": true, "BITAND": true, "BITLSHIFT": true, "BITOR": true, "BITRSHIFT": true, "BITXOR": true,
	"CEILING.MATH": true, "CEILING.PRECISE": true, "CHISQ.DIST": true, "CHISQ.DIST.RT": true, "CHISQ.INV": true, "CHISQ.INV.RT": true, "CHISQ.TEST": true, "COMBINA": true,
	"CONCAT": true, "CONFIDENCE.NORM": true, "CONFIDENCE.T": true, "COT": true, "COTH": true, "COVARIANCE.P": true, "COVARIANCE.S": true, "CSC": true,
	"CSCH": true, "DAYS": true, "DECIMAL": true, "ECMA.CEILING": true, "ERF.PRECISE": true, "ERFC.PRECISE": true, "EXPON.DIST": true, "F.DIST": true, "F.DIST.RT": true,
	"F.INV": true, "F.INV.RT": true, "F.TEST": true, "FLOOR.MATH": true, "FLOOR.PRECISE": true, "FORECAST.ETS": true, "FORECAST.ETS.CONFINT": true, "FORECAST.ETS.SEASONALITY": true,
	"FORECAST.ETS.STAT": true, "FORECAST.LINEAR": true, "FORMULATEXT": true, "GAMMA": true, "GAMMA.DIST": true, "GAMMA.INV": true, "GAMMALN.PRECISE": true, "GAUSS": true,
	"HYPGEOM.DIST": true, "IFNA": true, "IFS": true, "IMCOSH": true, "IMCOT": true, "IMCSC": true, "IMCSCH": true, "IMSEC": true,
	"IMSECH": true, "IMSINH": true, "IMTAN": true, "ISFORMULA": true, "ISO.CEILING": true, "ISOWEEKNUM": true, "LOGNORM.DIST": true, "LOGNORM.INV": true, "MAXIFS": true,
	"MINIFS": true, "MODE.MULT": true, "MODE.SNGL": true, "MUNIT": true, "NEGBINOM.DIST": true, "NETWORKDAYS.INTL": true, "NORM.DIST": true, "NORM.INV": true, "NORM.S.DIST": true,
	"NORM.S.INV": true, "NUMBERVALUE": true, "PDURATION": true, "PERCENTILE.EXC": true, "PERCENTILE.INC": true, "PERCENTRANK.EXC": true, "PERCENTRANK.INC": true, "PERMUTATIONA": true,
	"PHI": true, "POISSON.DIST": true, "QUARTILE.EXC": true, "QUARTILE.INC": true, "RANK.AVG": true, "RANK.EQ": true, "RRI": true, "SEC": true,
	"SECH": true, "SHEET": true, "SHEETS": true, "SKEW.P": true, "STDEV.P": true, "STDEV.S": true, "SWITCH": true, "T.DIST": true,
	"T.DIST.2T": true, "T.DIST.RT": true, "T.INV": true, "T.INV.2T": true, "T.TEST": true, "TEXTJOIN": true, "UNICHAR": true, "UNICODE": true,
	"VALUETOTEXT": true, "VAR.P": true, "VAR.S": true, "WEIBULL.DIST": true, "WORKDAY.INTL": true, "XOR": true, "Z.TEST": true,
}

var newerFn = map[string]bool{
	"FILTER": true, "SORT": true, "SORTBY": true, "UNIQUE": true, "SEQUENCE": true, "RANDARRAY": true, "XLOOKUP": true, "XMATCH": true,
	"LET": true, "LAMBDA": true, "ISOMITTED": true, "MAP": true, "REDUCE": true, "SCAN": true, "MAKEARRAY": true, "BYROW": true,
	"BYCOL": true, "GROUPBY": true, "PIVOTBY": true, "PERCENTOF": true, "REGEXTEST": true, "REGEXEXTRACT": true, "REGEXREPLACE": true, "TEXTSPLIT": true,
	"TEXTBEFORE": true, "TEXTAFTER": true, "VSTACK": true, "HSTACK": true, "TOCOL": true, "TOROW": true, "TAKE": true, "DROP": true,
	"CHOOSEROWS": true, "CHOOSECOLS": true, "EXPAND": true, "WRAPROWS": true, "WRAPCOLS": true, "ANCHORARRAY": true,
}

var outsideFn = map[string]bool{
	"WEBSERVICE": true, "FILTERXML": true, "IMAGE": true, "RTD": true, "CALL": true, "REGISTER.ID": true,
}

var fnName = regexp.MustCompile(`[A-Za-z_][\w.]*`)

var externalRef = regexp.MustCompile(`(?i)\[[^\[\]]*\.(xlsx|xlsm|xlsb|xltx|xls|csv)\]|\[\d+\][^\[\]()]*!`)

type writeArg struct {
	Path   string  `json:"path" required:"true"`
	Sheet  string  `json:"sheet" required:"true"`
	Cell   string  `json:"cell" required:"true" description:"top-left cell of the block, like E1"`
	Rows   [][]any `json:"rows" required:"true" description:"one array per row: numbers, booleans and text are stored as given, null leaves the cell empty"`
	Repeat uint    `json:"repeat" description:"write rows this many times, stacked downward; default 1"`
}

type writeResult struct {
	_     struct{} `description:"range: the block written"`
	Range string   `json:"range"`
}

func writeTool(ctx context.Context, arg writeArg, light bool) (writeResult, error) {
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
				fl, err := formula(v)
				if err != nil {
					return writeResult{}, fmt.Errorf("row %d cell %d: %w", i, j, err)
				}
				row[j] = "=" + fl
			default:
				return writeResult{}, fmt.Errorf("row %d cell %d: %T is not a cell value, pass a number, boolean, string or null", i, j, v)
			}
		}
	}
	if width == 0 {
		return writeResult{}, fmt.Errorf("rows hold no cells")
	}
	split := "split it into several writes"
	if !light {
		split += ", such as from a js script"
	}
	if arg.Repeat > maxFormatCells {
		return writeResult{}, fmt.Errorf("repeat %d is over the limit %d — %s", arg.Repeat, maxFormatCells, split)
	}
	height := len(arg.Rows) * int(max(arg.Repeat, 1))
	if cells := width * height; cells > maxFormatCells {
		return writeResult{}, fmt.Errorf("block covers %d cells, limit %d — %s", cells, maxFormatCells, split)
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

func formula(s string) (string, error) {
	body := s[1:]
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("formula %q is empty", s)
	}
	lits, blank := []byte(body), []byte(body)
	var quote byte
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case quote == 0:
			if c == '"' || c == '\'' {
				quote = c
			}
		case c == quote && i+1 < len(body) && body[i+1] == quote:
			blank[i], blank[i+1] = ' ', ' '
			if quote == '"' {
				lits[i], lits[i+1] = ' ', ' '
			}
			i++
		case c == quote:
			quote = 0
		default:
			blank[i] = ' '
			if quote == '"' {
				lits[i] = ' '
			}
		}
	}
	outside := fmt.Errorf("formula %q reaches outside this workbook (another workbook, a web service, a DDE link or another program), and external links are not supported — copy the data into this workbook first", s)
	if externalRef.Match(lits) {
		return "", outside
	}
	code := string(blank)
	if strings.Contains(code, "|") {
		return "", outside
	}
	var b strings.Builder
	last := 0
	for _, m := range fnName.FindAllStringIndex(code, -1) {
		if !strings.HasPrefix(strings.TrimLeft(code[m[1]:], " \t\r\n"), "(") {
			continue
		}
		name := strings.ToUpper(code[m[0]:m[1]])
		bare := strings.TrimPrefix(strings.TrimPrefix(name, "_XLFN."), "_XLWS.")
		switch {
		case outsideFn[bare]:
			return "", outside
		case newerFn[bare]:
			return "", fmt.Errorf("%s is a dynamic-array, LAMBDA or other Excel 365 function, which is not supported — use classic functions such as INDEX/MATCH, SUMIFS or COUNTIFS", bare)
		case futureFn[name]:
			b.WriteString(body[last:m[0]])
			b.WriteString("_xlfn." + name)
			last = m[1]
		}
	}
	b.WriteString(body[last:])
	return b.String(), nil
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
