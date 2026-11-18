// Managed by GoX v0.4.0

//line notify.gox:1
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

package notify

import (
	"context"
	"errors"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/nekit/internal/chat/icons"
)

const event = "nechat_error"

var OnError doors.Actions = doors.ActionEmit[any]{Name: event}

func Error(ctx context.Context, err error) {
	k := kind(err)
	if k == "" || ctx.Err() != nil {
		return
	}
	doors.Call(ctx, doors.ActionEmit[any]{Name: event, Arg: k})
}

func kind(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	return "server"
}

//line notify.gox:45
func Script() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const root = document.currentScript.closest(\".nechat-widget\")\n\t\tconst timers = new Map()\n\t\tconst show = (kind) => {\n\t\t\tlet key = \"\"\n\t\t\tif (kind === \"network\") key = \"network\"\n\t\t\telse if (kind === \"server\" || kind === \"other\" || kind === \"bad_request\")\n\t\t\t\tkey = \"error\"\n\t\t\tif (!key) return\n\t\t\tconst box = root.querySelector(\".nechat-toasts\")\n\t\t\tif (!box) return\n\t\t\tlet toast = box.querySelector(`.nechat-toast[data-key=\"${key}\"]`)\n\t\t\tif (!toast) {\n\t\t\t\ttoast = box\n\t\t\t\t\t.querySelector(`template[data-key=\"${key}\"]`)\n\t\t\t\t\t.content.firstElementChild.cloneNode(true)\n\t\t\t\tbox.append(toast)\n\t\t\t}\n\t\t\tclearTimeout(timers.get(key))\n\t\t\ttimers.set(key, setTimeout(() => toast.remove(), 4000))\n\t\t}\n\t\tconst listen = (e) => show(e.detail)\n\t\troot.addEventListener(\"nechat_error\", listen)\n\t\t$sys.clean(() => root.removeEventListener(\"nechat_error\", listen))\n\t\t$on(\"nechat_error\", (arg, err) => show(err ? err.kind : arg))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line notify.gox:72
}

//line notify.gox:74
func Toasts() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line notify.gox:75
			__e = __c.Set("class", "nechat-toasts"); if __e != nil { return }
//line notify.gox:75
			__e = __c.Set("role", "status"); if __e != nil { return }
//line notify.gox:75
			__e = __c.Set("aria-live", "polite"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("template"); if __e != nil { return }
			{
//line notify.gox:76
				__e = __c.Set("data-key", "network"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line notify.gox:77
					__e = __c.Set("class", "nechat-toast"); if __e != nil { return }
//line notify.gox:77
					__e = __c.Set("data-key", "network"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:78
						__e = __c.Set("class", "nechat-toast-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line notify.gox:78
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line notify.gox:78
							__e = __c.Any(icons.Prohibit()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line notify.gox:79
						__e = __c.Set("class", "nechat-toast-text"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Network error occurred"); if __e != nil { return }
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
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line notify.gox:89
}
