package layout

// ColumnDesc, RowDesc and GridDesc describe Column, Row and Grid to the
// agent, for Library.Layout.
const (
	ColumnDesc = "Stacks children top to bottom; children take the full width."
	RowDesc    = "Places children side by side and wraps them onto the next line when there is no room. To set how children share a line, wrap them in columns with fill."
	GridDesc   = "Equal-width cells, as many per line as fit; for repeated items such as cards, options and results."
)
