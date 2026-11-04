package xls

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"neui.dev/api"
	"neui.dev/internal/common"
)

const maxFormatCells = 100000

const tableDesc = `Write rows as a styled table: header row, per-column number formats, banded rows, a frozen header and computed column widths. Creates the workbook and sheet when missing. The table is the unit of report building — prefer it over ad-hoc cell writes.`
const formatDesc = `Restyle a range: bold, a named fill, a number format, alignment. Only the flags you pass change.`

type columnSpec struct {
	Header string `json:"header" required:"true"`
	Format string `json:"format" description:"text, number, currency, percent, date or integer; default text"`
}

type tableArg struct {
	Path    string       `json:"path" required:"true" description:"workspace path of the .xlsx workbook, created when missing"`
	Sheet   string       `json:"sheet" required:"true" description:"created when missing"`
	Cell    string       `json:"cell" description:"top-left anchor; default A1"`
	Columns []columnSpec `json:"columns" required:"true"`
	Rows    [][]any      `json:"rows" required:"true" description:"one array per data row, in column order"`
	Preset  string       `json:"preset" description:"plain, report or financial; default plain"`
	Name    string       `json:"name" description:"table name, for sheets output and structured references"`
}

type tableResult struct {
	Range string `json:"range"`
	Name  string `json:"name"`
}

type formatArg struct {
	Path         string `json:"path" required:"true"`
	Sheet        string `json:"sheet" required:"true"`
	Range        string `json:"range" required:"true" description:"like B2:D20"`
	Bold         bool   `json:"bold" description:"set bold; false is a no-op — this tool cannot remove bold"`
	Fill         string `json:"fill" description:"gray, blue, green, yellow or red"`
	NumberFormat string `json:"numberFormat" description:"text, number, currency, percent, date or integer"`
	Align        string `json:"align" description:"left, center or right"`
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

func applyNumFmt(st *excelize.Style, format string) error {
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
	default:
		return fmt.Errorf("format %q must be text, number, currency, percent, date or integer", format)
	}
	return nil
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
	styleName, ok := presetStyles[preset]
	if !ok {
		return tableResult{}, fmt.Errorf("preset %q must be plain, report or financial", arg.Preset)
	}
	for i, row := range arg.Rows {
		if len(row) > len(arg.Columns) {
			return tableResult{}, fmt.Errorf("row %d has %d cells, table has %d columns", i, len(row), len(arg.Columns))
		}
	}
	fs := common.GetController(ctx).FS()
	var f *excelize.File
	created := false
	if _, err := fs.Stat(ctx, arg.Path); errors.Is(err, api.ErrNotFound) {
		f = excelize.NewFile()
		created = true
	} else if err != nil {
		return tableResult{}, err
	} else if f, err = openRead(ctx, fs, arg.Path); err != nil {
		return tableResult{}, err
	}
	defer f.Close()
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
		}
	}
	headers := make([]any, len(arg.Columns))
	for i, col := range arg.Columns {
		headers[i] = col.Header
	}
	if err := f.SetSheetRow(arg.Sheet, anchor, &headers); err != nil {
		return tableResult{}, err
	}
	for i, row := range arg.Rows {
		start, err := excelize.CoordinatesToCellName(x1, y1+1+i)
		if err != nil {
			return tableResult{}, err
		}
		if err := f.SetSheetRow(arg.Sheet, start, &row); err != nil {
			return tableResult{}, err
		}
	}
	cache := styleCache{}
	for j, col := range arg.Columns {
		format := col.Format
		if format == "" || format == "text" || len(arg.Rows) == 0 {
			continue
		}
		id, err := cache.id(f, format, func() (*excelize.Style, error) {
			st := &excelize.Style{}
			if err := applyNumFmt(st, format); err != nil {
				return nil, err
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
		w := utf8.RuneCountInString(col.Header)
		for _, row := range arg.Rows {
			if j < len(row) {
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
	end, err := excelize.CoordinatesToCellName(x1+len(arg.Columns)-1, y1+max(len(arg.Rows), 1))
	if err != nil {
		return tableResult{}, err
	}
	ref := anchor + ":" + end
	x2, y2, err := excelize.CellNameToCoordinates(end)
	if err != nil {
		return tableResult{}, err
	}
	existing, err := f.GetTables(arg.Sheet)
	if err != nil {
		return tableResult{}, err
	}
	for _, t := range existing {
		tb, err := rangeBounds(t.Range)
		if err != nil {
			continue
		}
		if tb.x1 <= x2 && x1 <= tb.x2 && tb.y1 <= y2 && y1 <= tb.y2 {
			return tableResult{}, fmt.Errorf("range %s overlaps existing table %q at %s — pick another anchor or sheet", ref, t.Name, t.Range)
		}
	}
	if err := f.AddTable(arg.Sheet, &excelize.Table{Range: ref, Name: arg.Name, StyleName: styleName}); err != nil {
		return tableResult{}, err
	}
	below, err := excelize.CoordinatesToCellName(x1, y1+1)
	if err != nil {
		return tableResult{}, err
	}
	panes := &excelize.Panes{
		Freeze:      true,
		YSplit:      y1,
		TopLeftCell: below,
		ActivePane:  "bottomLeft",
		Selection:   []excelize.Selection{{SQRef: below, ActiveCell: below, Pane: "bottomLeft"}},
	}
	if err := f.SetPanes(arg.Sheet, panes); err != nil {
		return tableResult{}, err
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
	return tableResult{Range: ref, Name: name}, nil
}

func formatTool(ctx context.Context, arg formatArg) (struct{}, error) {
	if arg.Fill != "" {
		if _, ok := fillPalette[arg.Fill]; !ok {
			return struct{}{}, fmt.Errorf("fill %q must be gray, blue, green, yellow or red", arg.Fill)
		}
	}
	switch arg.Align {
	case "", "left", "center", "right":
	default:
		return struct{}{}, fmt.Errorf("align %q must be left, center or right", arg.Align)
	}
	if arg.NumberFormat != "" {
		if err := applyNumFmt(&excelize.Style{}, arg.NumberFormat); err != nil {
			return struct{}{}, err
		}
	}
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
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return struct{}{}, err
	}
	defer f.Close()
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
					if err := applyNumFmt(st, arg.NumberFormat); err != nil {
						return nil, err
					}
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
