package neui

import "neui.dev/internal/island/layouts"

// IslandLayouts registers the built-in "column", "row" and "grid" layouts
// in l, with their stylesheet in its head, and returns the first registration
// error. Their spacing and grid cell widths read --kisland-gap-sm, -md, -lg and
// --kisland-item-sm, -md, -lg, so a host can override them in CSS.
func IslandLayouts(l Library) error {
	if err := l.Layout("column", "Stacks children top to bottom; children take the full width.", layouts.Column); err != nil {
		return err
	}
	if err := l.Layout("row", "Places children side by side and wraps them onto the next line when there is no room; children take their content width unless they are a column with fill.", layouts.Row); err != nil {
		return err
	}
	if err := l.Layout("grid", "Equal-width cells, as many per line as fit; for repeated items such as cards, options and results.", layouts.Grid); err != nil {
		return err
	}
	l.Head(layouts.Style())
	return nil
}
