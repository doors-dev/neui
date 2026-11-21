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
	"neui.dev/nekit/internal/island/form"
	"neui.dev/nekit/internal/island/layout"
	"neui.dev/nekit/internal/island/markdown"
	"neui.dev/nekit/internal/island/table"
)

// IslandMarkdownParams is what a script passes to ui.markdown.
type IslandMarkdownParams = markdown.Params

// IslandMarkdownPatch is what a script passes to update a markdown view.
type IslandMarkdownPatch = markdown.Patch

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

// IslandMarkdownDesc, IslandFormDesc and IslandTableDesc describe the markdown
// view and the form and table controls to the agent, for Library.View and
// Library.Control. Button needs no description beyond its schema.
const (
	IslandMarkdownDesc = markdown.Desc
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

// IslandRegisterWidgets registers the widgets markdown, form, table and button
// and adds IslandStyle to the head.
func IslandRegisterWidgets(l neui.Library) error {
	l.Head(IslandStyle())
	if err := l.View("markdown", IslandMarkdownDesc, markdown.New); err != nil {
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
