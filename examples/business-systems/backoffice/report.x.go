// Managed by GoX v0.4.0

//line report.gox:1
package main

import (
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line report.gox:8
func reportPanel() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line report.gox:9
		__e = __c.Any(card{
		Title: "Report",
		Desc: "Ask the assistant for a report and it appears here.",
		Body: neui.Island{
			Name: "report",
			Desc: "Report panel on the dashboard, for read-only reports built from layouts, markdown and tables; no forms or buttons. Scripts read data with app.orders(filter), app.customers() and app.products()",
			Placeholder: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line report.gox:15
					__e = __c.Set("class", "flex items-center gap-3 rounded-lg border border-dashed border-stone-300 px-4 py-6 text-sm text-stone-500"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line report.gox:16
					__e = __c.Any(icon(iconSparkles, "size-5 text-amber-600")); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Try “which customers order the most?” or “revenue by product this month”."); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line report.gox:18
			return }),
			Template: func(slot gox.Comp) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line report.gox:20
						__e = __c.Any(slot); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line report.gox:20
				return })
			},
			Library: reportLibrary,
		}.Main(),
	}); if __e != nil { return }
	return })
//line report.gox:25
}
