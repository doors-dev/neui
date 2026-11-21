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

const chartDesc = `Add a chart with one series per column of dataRange.`

type chartArg struct {
	Path       string `json:"path" required:"true"`
	Sheet      string `json:"sheet" required:"true"`
	Type       string `json:"type" required:"true" enum:"column,bar,line,pie,scatter"`
	DataRange  string `json:"dataRange" required:"true" description:"header row included, like B1:C13; each header names its series"`
	Categories string `json:"categoriesRange" description:"axis labels, header row included, like A1:A13"`
	Anchor     string `json:"anchor" required:"true" description:"top-left cell of the chart; pick one clear of the data"`
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
	return "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
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
	if b.y2 == b.y1 {
		return struct{}{}, fmt.Errorf("dataRange %s has no data rows below its header", arg.DataRange)
	}
	categories := ""
	if arg.Categories != "" {
		cb, err := rangeBounds(arg.Categories)
		if err != nil {
			return struct{}{}, err
		}
		if cb.y2 == cb.y1 {
			return struct{}{}, fmt.Errorf("categoriesRange %s has no data rows below its header", arg.Categories)
		}
		if categories, err = absRange(arg.Sheet, cb.x1, cb.y1+1, cb.x2, cb.y2); err != nil {
			return struct{}{}, err
		}
	}
	series := []excelize.ChartSeries{}
	for x := b.x1; x <= b.x2; x++ {
		values, err := absRange(arg.Sheet, x, b.y1+1, x, b.y2)
		if err != nil {
			return struct{}{}, err
		}
		name, err := excelize.CoordinatesToCellName(x, b.y1, true)
		if err != nil {
			return struct{}{}, err
		}
		series = append(series, excelize.ChartSeries{Name: sheetRef(arg.Sheet) + "!" + name, Values: values, Categories: categories})
	}
	fs := common.GetController(ctx).FS()
	f, _, release, err := openWrite(ctx, fs, arg.Path, false)
	if err != nil {
		return struct{}{}, err
	}
	defer release()
	chart := &excelize.Chart{Type: t, Series: series}
	if arg.Title != "" {
		chart.Title = excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: arg.Title}}}
	}
	if err := f.AddChart(arg.Sheet, arg.Anchor, chart); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, save(ctx, fs, f, arg.Path)
}
