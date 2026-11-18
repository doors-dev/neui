// Managed by GoX v0.4.0

//line layouts.gox:1
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

package layout

import (
	"strconv"
	
	"github.com/doors-dev/gox"
)

// Column stacks children top to bottom, for Library.Layout.
func Column(p ColumnParams, children []gox.Comp) gox.Comp {
	gap := p.Gap
	if gap == "" {
		gap = "md"
	}
	class := "neisland-column neisland-gap-" + gap
	if p.Fill > 0 {
		class += " neisland-fill-" + strconv.Itoa(p.Fill)
	}
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line layouts.gox:33
			__e = __c.Set("class", class); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:33
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:33
	return })
}

// Row places children side by side and wraps them, for Library.Layout.
func Row(p RowParams, children []gox.Comp) gox.Comp {
	gap := p.Gap
	if gap == "" {
		gap = "md"
	}
	align := p.Align
	if align == "" {
		align = "start"
	}
	justify := p.Justify
	if justify == "" {
		justify = "start"
	}
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line layouts.gox:50
			__e = __c.Set("class", "neisland-row neisland-gap-" + gap + " neisland-align-" + align + " neisland-justify-" + justify); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:50
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:50
	return })
}

// Grid lays children out in equal-width cells, for Library.Layout.
func Grid(p GridParams, children []gox.Comp) gox.Comp {
	gap := p.Gap
	if gap == "" {
		gap = "md"
	}
	item := p.Item
	if item == "" {
		item = "md"
	}
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line layouts.gox:63
			__e = __c.Set("class", "neisland-grid neisland-gap-" + gap + " neisland-item-" + item); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:64
			for _, c := range children {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line layouts.gox:65
					__e = __c.Set("class", "neisland-grid-cell"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:65
					__e = __c.Any(c); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:67
	return })
}
