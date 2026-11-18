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
// colour or layout differs from the defaults, rules applying them to widgets
// whose data-nechat-theme is their FaceKey and whose data-nechat-layout is
// their LayoutKey.
func Style(dark Theme, light Theme, layout Layout) gox.Elem {
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
		face := FaceKey(dark, light)
		place := LayoutKey(layout)
		if face == "" && place == "" {
			return nil
		}
		var rule bytes.Buffer
		if face != "" {
			rule.WriteString(`.nechat-widget[data-nechat-theme="`)
			rule.WriteString(face)
			rule.WriteString(`"]{`)
			dark.write(&rule, "dark")
			light.write(&rule, "light")
			rule.WriteString("}")
		}
		if place != "" {
			layout.write(&rule, place)
		}
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
