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

package nekit

import (
	"context"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/nekit/internal/island"
	"neui.dev/nekit/internal/island/button"
	"neui.dev/nekit/internal/island/chart"
	"neui.dev/nekit/internal/island/form"
	"neui.dev/nekit/internal/island/layout"
	"neui.dev/nekit/internal/island/markdown"
	"neui.dev/nekit/internal/island/stat"
	"neui.dev/nekit/internal/island/table"
)

// IslandMarkdownParams is what a script passes to ui.markdown.
type IslandMarkdownParams = markdown.Params

// IslandMarkdownPatch is what a script passes to update a markdown view.
type IslandMarkdownPatch = markdown.Patch

// IslandChartParams is what a script passes to ui.chart.
type IslandChartParams = chart.Params

// IslandChartPatch is what a script passes to update a chart.
type IslandChartPatch = chart.Patch

// IslandStatParams is what a script passes to ui.stat.
type IslandStatParams = stat.Params

// IslandStatPatch is what a script passes to update a stat.
type IslandStatPatch = stat.Patch

// IslandFormParams is what a script passes to ui.form.
type IslandFormParams = form.Params

// IslandFormPatch is the form's update: it takes none.
type IslandFormPatch = form.Patch

// IslandFormSubmit is what a form hands the script waiting in ui.draw().
type IslandFormSubmit = form.Submit

// IslandTableParams is what a script passes to ui.table.
type IslandTableParams = table.Params

// IslandTablePatch is what a script passes to update a table.
type IslandTablePatch = table.Patch

// IslandTableSubmit is what a table hands the script waiting in ui.draw().
type IslandTableSubmit = table.Submit

// IslandButtonParams is what a script passes to ui.button.
type IslandButtonParams = button.Params

// IslandButtonPatch is what a script passes to update a button.
type IslandButtonPatch = button.Patch

// IslandColumnParams is what a script passes to ui.column.
type IslandColumnParams = layout.ColumnParams

// IslandRowParams is what a script passes to ui.row.
type IslandRowParams = layout.RowParams

// IslandGridParams is what a script passes to ui.grid.
type IslandGridParams = layout.GridParams

// NewIslandMarkdown builds a markdown view, for Library.View.
func NewIslandMarkdown(ctx context.Context, p IslandMarkdownParams) (api.Widget[IslandMarkdownPatch], error) {
	return markdown.New(ctx, p)
}

// NewIslandChart builds a chart view, for Library.View. It draws one measure
// of records, the y field, along the x field, split into series by the
// optional splitBy field, as a line, area, bar, pie, funnel or scatter chart.
// The records are fixed (the rows param) or come from the query param, run
// while the chart renders and, with refresh, again on that period, never below
// 3 seconds and backing off up to 5 minutes while runs fail. Invalid params or
// rows fail it. A patch replaces the rows or the query, whichever the chart
// was made with; a patch of the other kind is reported with neui.IslandReport
// and changes nothing. No records show as No data. Records that do not fit are
// reported with neui.IslandReport, a failed query by neui.IslandCall itself,
// and the chart keeps its last data, marked out of date. Values show in the
// browser's locale in the format param. Series colours read --neisland-chart-1
// to --neisland-chart-8 and the plot height --neisland-chart-height.
func NewIslandChart(ctx context.Context, p IslandChartParams) (api.Widget[IslandChartPatch], error) {
	return chart.New(ctx, p)
}

// NewIslandStat builds a stat view, for Library.View: a card with the label,
// the number large, and its change against the reference as a signed
// percentage, or as the difference when the reference is 0, formatted in the
// browser's language. The number comes from the value param or from the query
// param, which runs each time the card renders: when it first shows, right
// after a patch with a new query, when the browser reports another language,
// and every refresh seconds, never below 3 and backing off up to 5 minutes
// while runs fail. It fails when neither or both are set, when refresh or
// reference is set without the one it belongs to, when the currency format has
// no ISO 4217 code in unit, and when unit is set for another format than
// number and currency. A failed query shows No data. A patch in the other mode
// is reported with neui.IslandReport and changes nothing.
func NewIslandStat(ctx context.Context, p IslandStatParams) (api.Widget[IslandStatPatch], error) {
	return stat.New(ctx, p)
}

// NewIslandForm builds a form control, for Library.Control.
func NewIslandForm(ctx context.Context, p IslandFormParams, submit func(ctx context.Context, s IslandFormSubmit) bool) (api.Widget[IslandFormPatch], error) {
	return form.New(ctx, p, submit)
}

// NewIslandTable builds a table control, for Library.Control.
func NewIslandTable(ctx context.Context, p IslandTableParams, submit func(ctx context.Context, s IslandTableSubmit) bool) (api.Widget[IslandTablePatch], error) {
	return table.New(ctx, p, submit)
}

// NewIslandButton builds a button control, for Library.Control.
func NewIslandButton(ctx context.Context, p IslandButtonParams, submit func(ctx context.Context, value string) bool) (api.Widget[IslandButtonPatch], error) {
	return button.New(ctx, p, submit)
}

// IslandColumn stacks children top to bottom, for Library.Layout.
func IslandColumn(p IslandColumnParams, children []gox.Comp) gox.Comp {
	return layout.Column(p, children)
}

// IslandRow places children side by side, for Library.Layout.
func IslandRow(p IslandRowParams, children []gox.Comp) gox.Comp {
	return layout.Row(p, children)
}

// IslandGrid places children in equal-width cells, for Library.Layout.
func IslandGrid(p IslandGridParams, children []gox.Comp) gox.Comp {
	return layout.Grid(p, children)
}

// IslandColumnDesc, IslandRowDesc and IslandGridDesc describe IslandColumn,
// IslandRow and IslandGrid to the agent, for Library.Layout.
const (
	IslandColumnDesc = layout.ColumnDesc
	IslandRowDesc    = layout.RowDesc
	IslandGridDesc   = layout.GridDesc
)

// IslandMarkdownDesc, IslandChartDesc, IslandStatDesc, IslandFormDesc and
// IslandTableDesc describe the markdown, chart and stat views and the form and
// table controls to the agent, for Library.View and Library.Control. Button
// needs no description beyond its schema.
const (
	IslandMarkdownDesc = markdown.Desc
	IslandChartDesc    = chart.Desc
	IslandStatDesc     = stat.Desc
	IslandFormDesc     = form.Desc
	IslandTableDesc    = table.Desc
)

// IslandStyle is the stylesheet of the layouts and widgets. Every IslandRegister
// function adds it to Library.Head, and it renders once per island app.
// Spacing and grid cell widths read --neisland-gap-sm, -md, -lg and
// --neisland-item-sm, -md, -lg, so a host can override them in CSS; a fill
// column wraps onto the next line rather than get narrower than
// --neisland-item-md.
func IslandStyle() gox.Elem {
	return island.Style()
}

// IslandRegisterAll registers every layout and widget and adds IslandStyle to the
// head.
func IslandRegisterAll(l neui.Library) error {
	l.Head(IslandStyle())
	if err := l.Layout("column", IslandColumnDesc, layout.Column); err != nil {
		return err
	}
	if err := l.Layout("row", IslandRowDesc, layout.Row); err != nil {
		return err
	}
	if err := l.Layout("grid", IslandGridDesc, layout.Grid); err != nil {
		return err
	}
	if err := l.View("markdown", IslandMarkdownDesc, markdown.New); err != nil {
		return err
	}
	if err := l.View("chart", IslandChartDesc, chart.New); err != nil {
		return err
	}
	if err := l.View("stat", IslandStatDesc, stat.New); err != nil {
		return err
	}
	if err := l.Control("form", IslandFormDesc, form.New); err != nil {
		return err
	}
	if err := l.Control("table", IslandTableDesc, table.New); err != nil {
		return err
	}
	return l.Control("button", "", button.New)
}

// IslandRegisterLayout registers the layouts column, row and grid and adds
// IslandStyle to the head.
func IslandRegisterLayout(l neui.Library) error {
	l.Head(IslandStyle())
	if err := l.Layout("column", IslandColumnDesc, layout.Column); err != nil {
		return err
	}
	if err := l.Layout("row", IslandRowDesc, layout.Row); err != nil {
		return err
	}
	return l.Layout("grid", IslandGridDesc, layout.Grid)
}

// IslandRegisterWidgets registers the widgets markdown, chart, stat, form,
// table and button and adds IslandStyle to the head.
func IslandRegisterWidgets(l neui.Library) error {
	l.Head(IslandStyle())
	if err := l.View("markdown", IslandMarkdownDesc, markdown.New); err != nil {
		return err
	}
	if err := l.View("chart", IslandChartDesc, chart.New); err != nil {
		return err
	}
	if err := l.View("stat", IslandStatDesc, stat.New); err != nil {
		return err
	}
	if err := l.Control("form", IslandFormDesc, form.New); err != nil {
		return err
	}
	if err := l.Control("table", IslandTableDesc, table.New); err != nil {
		return err
	}
	return l.Control("button", "", button.New)
}

// IslandRegisterMarkdown registers the view markdown and adds IslandStyle to
// the head.
func IslandRegisterMarkdown(l neui.Library) error {
	l.Head(IslandStyle())
	return l.View("markdown", IslandMarkdownDesc, markdown.New)
}

// IslandRegisterChart registers the view chart and adds IslandStyle to the
// head.
func IslandRegisterChart(l neui.Library) error {
	l.Head(IslandStyle())
	return l.View("chart", IslandChartDesc, chart.New)
}

// IslandRegisterStat registers the view stat and adds IslandStyle to the
// head.
func IslandRegisterStat(l neui.Library) error {
	l.Head(IslandStyle())
	return l.View("stat", IslandStatDesc, stat.New)
}

// IslandRegisterForm registers the control form and adds IslandStyle to the
// head.
func IslandRegisterForm(l neui.Library) error {
	l.Head(IslandStyle())
	return l.Control("form", IslandFormDesc, form.New)
}

// IslandRegisterTable registers the control table and adds IslandStyle to the
// head.
func IslandRegisterTable(l neui.Library) error {
	l.Head(IslandStyle())
	return l.Control("table", IslandTableDesc, table.New)
}

// IslandRegisterButton registers the control button and adds IslandStyle to
// the head.
func IslandRegisterButton(l neui.Library) error {
	l.Head(IslandStyle())
	return l.Control("button", "", button.New)
}
