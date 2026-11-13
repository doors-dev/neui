// Package neisland is the ready-made widget set for neui islands: layouts,
// markdown, forms, tables and buttons. Register them on Island.Library:
//
//	Library: neisland.RegisterAll
package neisland

import (
	"context"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/neisland/button"
	"neui.dev/neisland/form"
	"neui.dev/neisland/layout"
	"neui.dev/neisland/markdown"
	"neui.dev/neisland/table"
)

// NewMarkdown builds a markdown view, for Library.View.
func NewMarkdown(ctx context.Context, p markdown.Params) (api.Widget[markdown.Patch], error) {
	return markdown.New(ctx, p)
}

// NewForm builds a form control, for Library.Control.
func NewForm(ctx context.Context, p form.Params, submit func(ctx context.Context, s map[string]any) bool) (api.Widget[struct{}], error) {
	return form.New(ctx, p, submit)
}

// NewTable builds a table control, for Library.Control.
func NewTable(ctx context.Context, p table.Params, submit func(ctx context.Context, s table.Submit) bool) (api.Widget[table.Patch], error) {
	return table.New(ctx, p, submit)
}

// NewButton builds a button control, for Library.Control.
func NewButton(ctx context.Context, p button.Params, submit func(ctx context.Context, value string) bool) (api.Widget[button.Patch], error) {
	return button.New(ctx, p, submit)
}

// Column stacks children top to bottom, for Library.Layout.
func Column(p layout.ColumnParams, children []gox.Comp) gox.Comp {
	return layout.Column(p, children)
}

// Row places children side by side, for Library.Layout.
func Row(p layout.RowParams, children []gox.Comp) gox.Comp {
	return layout.Row(p, children)
}

// Grid places children in equal-width cells, for Library.Layout.
func Grid(p layout.GridParams, children []gox.Comp) gox.Comp {
	return layout.Grid(p, children)
}

// ColumnDesc, RowDesc and GridDesc describe Column, Row and Grid to the
// agent, for Library.Layout.
const (
	ColumnDesc = layout.ColumnDesc
	RowDesc    = layout.RowDesc
	GridDesc   = layout.GridDesc
)

// MarkdownDesc, FormDesc and TableDesc describe the markdown view and the form
// and table controls to the agent, for Library.View and Library.Control.
// Button needs no description beyond its schema.
const (
	MarkdownDesc = markdown.Desc
	FormDesc     = form.Desc
	TableDesc    = table.Desc
)

// RegisterAll registers every layout and widget and adds Style to the head.
func RegisterAll(l neui.Library) error {
	l.Head(Style())
	if err := l.Layout("column", ColumnDesc, layout.Column); err != nil {
		return err
	}
	if err := l.Layout("row", RowDesc, layout.Row); err != nil {
		return err
	}
	if err := l.Layout("grid", GridDesc, layout.Grid); err != nil {
		return err
	}
	if err := l.View("markdown", MarkdownDesc, markdown.New); err != nil {
		return err
	}
	if err := l.Control("form", FormDesc, form.New); err != nil {
		return err
	}
	if err := l.Control("table", TableDesc, table.New); err != nil {
		return err
	}
	return l.Control("button", "", button.New)
}

// RegisterLayout registers the layouts column, row and grid and adds Style to
// the head.
func RegisterLayout(l neui.Library) error {
	l.Head(Style())
	if err := l.Layout("column", ColumnDesc, layout.Column); err != nil {
		return err
	}
	if err := l.Layout("row", RowDesc, layout.Row); err != nil {
		return err
	}
	return l.Layout("grid", GridDesc, layout.Grid)
}

// RegisterWidgets registers the widgets markdown, form, table and button and adds
// Style to the head.
func RegisterWidgets(l neui.Library) error {
	l.Head(Style())
	if err := l.View("markdown", MarkdownDesc, markdown.New); err != nil {
		return err
	}
	if err := l.Control("form", FormDesc, form.New); err != nil {
		return err
	}
	if err := l.Control("table", TableDesc, table.New); err != nil {
		return err
	}
	return l.Control("button", "", button.New)
}

// RegisterMarkdown registers the view markdown and adds Style to the head.
func RegisterMarkdown(l neui.Library) error {
	l.Head(Style())
	return l.View("markdown", MarkdownDesc, markdown.New)
}

// RegisterForm registers the control form and adds Style to the head.
func RegisterForm(l neui.Library) error {
	l.Head(Style())
	return l.Control("form", FormDesc, form.New)
}

// RegisterTable registers the control table and adds Style to the head.
func RegisterTable(l neui.Library) error {
	l.Head(Style())
	return l.Control("table", TableDesc, table.New)
}

// RegisterButton registers the control button and adds Style to the head.
func RegisterButton(l neui.Library) error {
	l.Head(Style())
	return l.Control("button", "", button.New)
}
