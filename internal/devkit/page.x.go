// Managed by GoX v0.4.0

//line page.gox:1
package devkit

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
)

//line page.gox:12
func (k *Kit) page() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line page.gox:14
		c := &controller{kit: k, page: page.NewPage(doors.InstanceContext(ctx)), path: k.path()}
	k.mu.Lock()
	k.current = c
	k.mu.Unlock()

		__e = __c.Raw("<!doctype html>"); if __e != nil { return }
		__e = __c.Init("html"); if __e != nil { return }
		{
//line page.gox:20
			__e = __c.Set("lang", "en"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("head"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("meta"); if __e != nil { return }
				{
//line page.gox:22
					__e = __c.Set("charset", "utf-8"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("meta"); if __e != nil { return }
				{
//line page.gox:23
					__e = __c.Set("name", "viewport"); if __e != nil { return }
//line page.gox:23
					__e = __c.Set("content", "width=device-width, initial-scale=1"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("title"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("devkit"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("body"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line page.gox:27
				__e = (doors.Ctx(context.WithValue(ctx, common.KeySpace, c.page.Space()))).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("main"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line page.gox:28
						__e = __c.Any(k.app.Island); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line page.gox:30
				if k.app.Script != "" {
//line page.gox:31
					__e = __c.Any(doors.Go(func(ctx context.Context) {
					if _, _, err := c.start(ctx, k.app.Script); err != nil {
						k.app.Report(Report{Kind: "error", Err: err})
					}
				})); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line page.gox:39
}
