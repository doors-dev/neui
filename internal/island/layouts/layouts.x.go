// Managed by GoX v0.3.3

//line layouts.gox:1
package layouts

import (
	"strconv"
	
	"github.com/doors-dev/gox"
)

func Column(p ColumnParams, children []gox.Comp) gox.Comp {
	gap := p.Gap
	if gap == "" {
		gap = "md"
	}
	class := "kisland-column kisland-gap-" + gap
	if p.Fill > 0 {
		class += " kisland-fill-" + strconv.Itoa(p.Fill)
	}
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line layouts.gox:18
			__e = __c.Set("class", class); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:18
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:18
	return })
}

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
//line layouts.gox:34
			__e = __c.Set("class", "kisland-row kisland-gap-" + gap + " kisland-align-" + align + " kisland-justify-" + justify); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:34
			__e = __c.Any(children); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:34
	return })
}

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
//line layouts.gox:46
			__e = __c.Set("class", "kisland-grid kisland-gap-" + gap + " kisland-item-" + item); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:47
			for _, c := range children {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line layouts.gox:48
					__e = __c.Set("class", "kisland-grid-cell"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line layouts.gox:48
					__e = __c.Any(c); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
//line layouts.gox:50
	return })
}
