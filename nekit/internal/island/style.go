package island

import (
	_ "embed"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//go:embed css/output.css
var style []byte

type styleKey struct{}

// Style is the stylesheet of the layouts and widgets. Every Register function
// adds it to Library.Head, and it renders once per island app.
// Spacing and grid cell widths read --neisland-gap-sm, -md, -lg and
// --neisland-item-sm, -md, -lg, so a host can override them in CSS; a fill
// column wraps onto the next line rather than get narrower than
// --neisland-item-md.
func Style() gox.Elem {
	return func(cur gox.Cursor) error {
		if store := neui.IslandStore(cur.Context()); store != nil {
			first := false
			store.Init(styleKey{}, func() any {
				first = true
				return struct{}{}
			})
			if !first {
				return nil
			}
		}
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
