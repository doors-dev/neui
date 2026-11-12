// Managed by GoX v0.3.4

//line notify.gox:1
package notify

import (
	"context"
	"errors"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/nechat/icons"
)

const event = "nechat_error"

var OnError doors.Actions = doors.ActionEmit[any]{Name: event}

type rootKey struct{}

func Root(ctx context.Context) context.Context {
	return context.WithValue(ctx, rootKey{}, doors.DetachedContext(ctx))
}

func Error(ctx context.Context, err error) {
	root, ok := ctx.Value(rootKey{}).(context.Context)
	k := kind(err)
	if !ok || k == "" {
		return
	}
	doors.Call(root, doors.ActionEmit[any]{Name: event, Arg: k})
}

func kind(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	return "server"
}

//line notify.gox:38
func Script() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const root = document.currentScript.closest(\".nechat-widget\")\n\t\tconst show = (kind) => {\n\t\t\tlet key = \"\"\n\t\t\tif (kind === \"network\") key = \"network\"\n\t\t\telse if (kind === \"server\" || kind === \"other\" || kind === \"bad_request\")\n\t\t\t\tkey = \"error\"\n\t\t\tif (!key) return\n\t\t\tconst box = root.querySelector(\".nechat-toasts\")\n\t\t\tif (!box) return\n\t\t\tconst shown = box.querySelector(`.nechat-toast[data-key=\"${key}\"]`)\n\t\t\tif (shown) {\n\t\t\t\tfor (const el of [shown, ...shown.querySelectorAll(\".nechat-file-fill\")]) {\n\t\t\t\t\tel.style.animation = \"none\"\n\t\t\t\t\tvoid el.offsetWidth\n\t\t\t\t\tel.style.animation = \"\"\n\t\t\t\t}\n\t\t\t\treturn\n\t\t\t}\n\t\t\tconst toast = box\n\t\t\t\t.querySelector(`template[data-key=\"${key}\"]`)\n\t\t\t\t.content.firstElementChild.cloneNode(true)\n\t\t\ttoast.addEventListener(\"animationend\", (e) => {\n\t\t\t\tif (e.target === toast) toast.remove()\n\t\t\t})\n\t\t\tbox.append(toast)\n\t\t}\n\t\tconst listen = (e) => show(e.detail)\n\t\troot.addEventListener(\"nechat_error\", listen)\n\t\t$sys.clean(() => root.removeEventListener(\"nechat_error\", listen))\n\t\t$on(\"nechat_error\", (arg, err) => show(err ? err.kind : arg))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line notify.gox:71
}

//line notify.gox:73
func Toasts() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line notify.gox:74
			__e = __c.Set("class", "nechat-toasts"); if __e != nil { return }
//line notify.gox:74
			__e = __c.Set("role", "status"); if __e != nil { return }
//line notify.gox:74
			__e = __c.Set("aria-live", "polite"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("template"); if __e != nil { return }
			{
//line notify.gox:75
				__e = __c.Set("data-key", "network"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line notify.gox:76
					__e = __c.Set("class", "nechat-toast"); if __e != nil { return }
//line notify.gox:76
					__e = __c.Set("data-key", "network"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:77
						__e = __c.Set("class", "nechat-toast-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line notify.gox:77
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line notify.gox:77
							__e = __c.Any(icons.Prohibit()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:78
						__e = __c.Set("class", "nechat-toast-text"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Network error occurred"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
//line notify.gox:79
						__e = __c.Set("class", "nechat-file-fill"); if __e != nil { return }
//line notify.gox:79
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("template"); if __e != nil { return }
			{
//line notify.gox:82
				__e = __c.Set("data-key", "error"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line notify.gox:83
					__e = __c.Set("class", "nechat-toast"); if __e != nil { return }
//line notify.gox:83
					__e = __c.Set("data-key", "error"); if __e != nil { return }
//line notify.gox:83
					__e = __c.Set("role", "alert"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:84
						__e = __c.Set("class", "nechat-toast-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line notify.gox:84
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line notify.gox:84
							__e = __c.Any(icons.WarningFill()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:85
						__e = __c.Set("class", "nechat-toast-text"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Something went wrong"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
//line notify.gox:86
						__e = __c.Set("class", "nechat-file-fill"); if __e != nil { return }
//line notify.gox:86
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line notify.gox:90
}
