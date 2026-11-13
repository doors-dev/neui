// Managed by GoX v0.3.6-0.20261113082228-02db5fed593a

//line layouts.gox:1
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
//line layouts.gox:19
			__e = __c.Set("class", class); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:19
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:19
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
//line layouts.gox:36
			__e = __c.Set("class", "neisland-row neisland-gap-" + gap + " neisland-align-" + align + " neisland-justify-" + justify); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:36
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:36
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
//line layouts.gox:49
			__e = __c.Set("class", "neisland-grid neisland-gap-" + gap + " neisland-item-" + item); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:50
			for _, c := range children {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line layouts.gox:51
					__e = __c.Set("class", "neisland-grid-cell"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:51
					__e = __c.Any(c); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:53
	return })
}
