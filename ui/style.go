package ui

import (
	_ "embed"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

//go:embed css/output.css
var style []byte

// Style is the stylesheet of the layouts and widgets, for Library.Head.
// Spacing and grid cell widths read --kisland-gap-sm, -md, -lg and
// --kisland-item-sm, -md, -lg, so a host can override them in CSS; a fill
// column wraps onto the next line rather than get narrower than
// --kisland-item-md.
func Style() gox.Elem {
	return func(cur gox.Cursor) error {
		if err := cur.InitVoid("link"); err != nil {
			return err
		}
		if err := cur.Set("rel", "stylesheet"); err != nil {
			return err
		}
		if err := cur.Modify(doors.ResourceBytes(style)); err != nil {
			return err
		}
		return cur.Submit()
	}
}
