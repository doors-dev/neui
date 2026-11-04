package layouts

import (
	_ "embed"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

//go:embed output.css
var style []byte

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
