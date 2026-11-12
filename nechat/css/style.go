package css

import (
	"bytes"
	_ "embed"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

//go:embed output.css
var style []byte

// Style renders the nechat stylesheet link and, when dark or light set any
// colour, a rule applying them to widgets whose data-nechat-theme is their FaceKey.
func Style(dark Theme, light Theme) gox.Elem {
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
		if err := cur.Submit(); err != nil {
			return err
		}
		key := FaceKey(dark, light)
		if key == "" {
			return nil
		}
		var rule bytes.Buffer
		rule.WriteString(`.nechat-widget[data-nechat-theme="`)
		rule.WriteString(key)
		rule.WriteString(`"]{`)
		dark.write(&rule, "dark")
		light.write(&rule, "light")
		rule.WriteString("}")
		if err := cur.Init("style"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Bytes(rule.Bytes()); err != nil {
			return err
		}
		return cur.Close()
	}
}
