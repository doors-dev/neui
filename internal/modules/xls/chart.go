package xls

import (
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const chartDesc = `Add a chart over a data range, one series per data column. Anchor is the top-left cell the chart is placed at — pick one clear of the data.`

type chartArg struct {
	Path       string `json:"path" required:"true"`
	Sheet      string `json:"sheet" required:"true"`
	Type       string `json:"type" required:"true" description:"column, bar, line, pie or scatter"`
	DataRange  string `json:"dataRange" required:"true" description:"values to plot, like B2:C13, one series per column — exclude the header row; series names are taken automatically from the cells just above the range"`
	Categories string `json:"categoriesRange" description:"labels along the axis, like A2:A13"`
	Anchor     string `json:"anchor" required:"true" description:"top-left cell of the chart"`
	Title      string `json:"title"`
}

var chartTypes = map[string]excelize.ChartType{
	"column":  excelize.Col,
	"bar":     excelize.Bar,
	"line":    excelize.Line,
	"pie":     excelize.Pie,
	"scatter": excelize.Scatter,
}

func sheetRef(sheet string) string {
	if strings.ContainsAny(sheet, " '") {
		return "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
	}
	return sheet
}

func absRange(sheet string, x1, y1, x2, y2 int) (string, error) {
	from, err := excelize.CoordinatesToCellName(x1, y1, true)
	if err != nil {
		return "", err
	}
	to, err := excelize.CoordinatesToCellName(x2, y2, true)
	if err != nil {
		return "", err
	}
	return sheetRef(sheet) + "!" + from + ":" + to, nil
}

func chartTool(ctx context.Context, arg chartArg) (struct{}, error) {
	t, ok := chartTypes[arg.Type]
	if !ok {
		return struct{}{}, fmt.Errorf("chart type %q: pick column, bar, line, pie or scatter", arg.Type)
	}
	b, err := rangeBounds(arg.DataRange)
	if err != nil {
		return struct{}{}, err
	}
	categories := ""
	if arg.Categories != "" {
		cb, err := rangeBounds(arg.Categories)
		if err != nil {
			return struct{}{}, err
		}
		if categories, err = absRange(arg.Sheet, cb.x1, cb.y1, cb.x2, cb.y2); err != nil {
			return struct{}{}, err
		}
	}
	series := []excelize.ChartSeries{}
	for x := b.x1; x <= b.x2; x++ {
		values, err := absRange(arg.Sheet, x, b.y1, x, b.y2)
		if err != nil {
			return struct{}{}, err
		}
		s := excelize.ChartSeries{Values: values, Categories: categories}
		if b.y1 > 1 {
			name, err := excelize.CoordinatesToCellName(x, b.y1-1, true)
			if err != nil {
				return struct{}{}, err
			}
			s.Name = sheetRef(arg.Sheet) + "!" + name
		}
		series = append(series, s)
	}
	fs := common.GetController(ctx).FS()
	f, err := openRead(ctx, fs, arg.Path)
	if err != nil {
		return struct{}{}, err
	}
	defer f.Close()
	chart := &excelize.Chart{Type: t, Series: series}
	if arg.Title != "" {
		chart.Title = excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: arg.Title}}}
	}
	if err := f.AddChart(arg.Sheet, arg.Anchor, chart); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, save(ctx, fs, f, arg.Path)
}
