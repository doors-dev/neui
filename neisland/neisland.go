// Package neisland is the ready-made widget set for neui islands: layouts,
// text, forms, tables and buttons. Register them on Island.Library:
//
//	Library: neisland.RegisterAll
package neisland

import (
	"neui.dev"
	"neui.dev/neisland/button"
	"neui.dev/neisland/form"
	"neui.dev/neisland/layout"
	"neui.dev/neisland/table"
	"neui.dev/neisland/text"
)

// Widget constructors for Library.View and Library.Control, and layouts for
// Library.Layout.
var (
	NewText   = text.New
	NewForm   = form.New
	NewTable  = table.New
	NewButton = button.New
	Column    = layout.Column
	Row       = layout.Row
	Grid      = layout.Grid
)

// ColumnDesc, RowDesc and GridDesc describe Column, Row and Grid to the
// agent, for Library.Layout.
const (
	ColumnDesc = layout.ColumnDesc
	RowDesc    = layout.RowDesc
	GridDesc   = layout.GridDesc
)

// RegisterAll adds Style to the head and registers every layout and widget.
func RegisterAll(l neui.Library) error {
	l.Head(Style())
	if err := RegisterWidgets(l); err != nil {
		return err
	}
	return RegisterLayout(l)
}

// RegisterLayout registers the layouts column, row and grid.
func RegisterLayout(l neui.Library) error {
	if err := l.Layout("column", ColumnDesc, layout.Column); err != nil {
		return err
	}
	if err := l.Layout("row", RowDesc, layout.Row); err != nil {
		return err
	}
	return l.Layout("grid", GridDesc, layout.Grid)
}

// RegisterWidgets registers the widgets text, form, table and button.
func RegisterWidgets(l neui.Library) error {
	if err := RegisterText(l); err != nil {
		return err
	}
	if err := RegisterForm(l); err != nil {
		return err
	}
	if err := RegisterTable(l); err != nil {
		return err
	}
	return RegisterButton(l)
}

// RegisterText registers the view text.
func RegisterText(l neui.Library) error {
	return l.View("text", "a text", text.New)
}

// RegisterForm registers the control form.
func RegisterForm(l neui.Library) error {
	return l.Control("form", "a form", form.New)
}

// RegisterTable registers the control table.
func RegisterTable(l neui.Library) error {
	return l.Control("table", "a table", table.New)
}

// RegisterButton registers the control button.
func RegisterButton(l neui.Library) error {
	return l.Control("button", "a button", button.New)
}
