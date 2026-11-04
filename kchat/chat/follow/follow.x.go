// Managed by GoX v0.3.3

//line follow.gox:1
package follow

import (
	"github.com/doors-dev/gox"
)

//line follow.gox:7
func Mark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line follow.gox:8
			__e = __c.Set("class", "kchat-follow"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const mark = document.currentScript.parentElement\n\t\t\tconst log = mark.closest(\".kchat-log\")\n\t\t\tif (!log) return\n\t\t\tconst below = mark.getBoundingClientRect().top - log.getBoundingClientRect().bottom\n\t\t\tmark.remove()\n\t\t\tif (below > 48) return\n\t\t\trequestAnimationFrame(() => log.scrollTo({ top: log.scrollHeight, behavior: \"smooth\" }))"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line follow.gox:19
}
