// Managed by GoX v0.4.0

//line panel.gox:1
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

package aside

import (
	"context"
	"strconv"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/aside/controller"
	"neui.dev/nekit/internal/chat/pops"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/layout"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/notify"
)

type chatMenuMode int

const (
	chatMenuNone chatMenuMode = iota
	chatMenuOpen
	chatMenuRename
	chatMenuDelete
)

type chatMenu struct {
	id api.ThreadID
	mode chatMenuMode
}

type Aside struct {
	Params params.Params
	Instance api.Instance
	Pops pops.Popups
	Layout doors.Source[layout.State]
	menu pops.Popup[chatMenu]
	controller controller.Controller
}

//line panel.gox:56
func (p Aside) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:58
		p.controller = controller.New(p.Params.Off(params.MessageSearch), p.Layout.Get().Aside)
	p.menu = p.Pops.Popup[chatMenu]()
	burger := new(doors.Setter)
	expanded, _ := p.controller.Expanded().ReadAndSub(ctx, func(ctx context.Context, v bool) bool {
		doors.Call(ctx, burger.Set("aria-expanded", strconv.FormatBool(v)))
		p.Layout.Mutate(ctx, func(s layout.State) layout.State {
			s.Aside = v
			return s
		})
		return false
	})

//line panel.gox:70
		__e = __c.Any(p.controller.Show(p.aside())); if __e != nil { return }
//line panel.gox:71
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			p.controller.Toggle(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line panel.gox:78
				__e = __c.Set("class", "nechat-iconbtn nechat-burger"); if __e != nil { return }
//line panel.gox:78
				__e = __c.Set("title", "Menu"); if __e != nil { return }
//line panel.gox:78
				__e = __c.Modify(burger); if __e != nil { return }
//line panel.gox:78
				__e = __c.Set("aria-expanded", strconv.FormatBool(expanded)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:79
				__e = __c.Any(p.controller.RouteToggle(
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:80
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:80
						__e = __c.Any(icons.List()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:80
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:81
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:81
						__e = __c.Any(icons.List()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:81
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:82
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:82
						__e = __c.Any(icons.ArrowLeft()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:82
			return }),
		)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line panel.gox:85
}

//line panel.gox:87
func (p *Aside) aside() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("aside"); if __e != nil { return }
		{
//line panel.gox:88
			__e = __c.Set("class", "nechat-aside"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:89
				__e = __c.Set("class", "nechat-top"); if __e != nil { return }
//line panel.gox:89
				__e = __c.Set("role", "search"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("label"); if __e != nil { return }
				{
//line panel.gox:90
					__e = __c.Set("class", "nechat-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line panel.gox:91
					__e = (doors.Class("nechat-ic-sm")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:91
						__e = __c.Any(icons.MagnifyingGlass()); if __e != nil { return }
					return })); if __e != nil { return }
//line panel.gox:92
					__e = __c.Any(p.controller.Input(func(v string) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:93
						__e = (doors.AInput{
						Scope: &doors.ScopeDebounce{
							Duration: time.Second / 5,
							Limit: time.Second / 3,
						},
						Race: doors.RaceStrict,
						On: func(ctx context.Context, ri doors.RequestInput) bool {
							p.controller.SearchInput(ctx, ri.Event().Value)
							return false
						},
						OnError: notify.OnError,
					}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line panel.gox:105
								__e = __c.Set("value", v); if __e != nil { return }
//line panel.gox:106
								__e = __c.Set("class", "nechat-text"); if __e != nil { return }
//line panel.gox:107
								__e = __c.Set("type", "search"); if __e != nil { return }
//line panel.gox:108
								__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line panel.gox:109
								__e = __c.Set("aria-label", "Search chats"); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
						return })); if __e != nil { return }
					return })
//line panel.gox:110
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:113
				__e = __c.Set("class", "nechat-side-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:114
				__e = __c.Any(p.controller.RouteList(
				func(w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:116
						__e = __c.Any(&days{aside: p, window: w, save: set}); if __e != nil { return }
					return })
//line panel.gox:117
				},
				func(input string, open func(ctx context.Context, kind controller.SearchKind)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:119
						__e = __c.Any(p.overview(input, open)); if __e != nil { return }
					return })
//line panel.gox:120
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:122
						if input == "" {
//line panel.gox:123
							__e = __c.Any(p.noResults(controller.SearchNames)); if __e != nil { return }
						} else  {
//line panel.gox:125
							__e = __c.Any(&names{aside: p, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:127
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:129
						if input == "" {
//line panel.gox:130
							__e = __c.Any(p.noResults(controller.SearchMessagesCurrent)); if __e != nil { return }
						} else  {
//line panel.gox:132
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:134
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:136
						if input == "" {
//line panel.gox:137
							__e = __c.Any(p.noResults(controller.SearchMessagesAll)); if __e != nil { return }
						} else  {
//line panel.gox:139
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, exclude: true, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:141
				},
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line panel.gox:144
			__e = (doors.AHook[bool]{
			Name: "collapse",
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, r doors.RequestHook[bool]) (any, bool) {
				p.controller.Collapse(ctx)
				return nil, false
			},
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("script"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const side = document.currentScript.closest(\".nechat-aside\")\n\t\t\tconst widget = side.closest(\".nechat-widget\")\n\t\t\tconst focusDraft = (tries = 20) => {\n\t\t\t\tif (!matchMedia(\"(pointer: fine)\").matches) return\n\t\t\t\tconst draft = widget.querySelector(\".nechat-draft:not(:disabled)\")\n\t\t\t\tif (draft) return draft.focus({ preventScroll: true })\n\t\t\t\tif (tries > 0) requestAnimationFrame(() => focusDraft(tries - 1))\n\t\t\t}\n\t\t\tconst collapse = () => {\n\t\t\t\tif (getComputedStyle(side).position !== \"absolute\") return focusDraft()\n\t\t\t\treturn $hook(\"collapse\", true)\n\t\t\t\t\t.then(() => focusDraft())\n\t\t\t\t\t.catch((err) => {\n\t\t\t\t\t\tif (err instanceof HookErr)\n\t\t\t\t\t\t\twidget.dispatchEvent(\n\t\t\t\t\t\t\t\tnew CustomEvent(\"nechat_error\", { detail: err.kind }),\n\t\t\t\t\t\t\t)\n\t\t\t\t\t})\n\t\t\t}\n\t\t\t$on(\"opened\", collapse)\n\t\t\tside.addEventListener(\"click\", (e) => {\n\t\t\t\tif (e.target.closest(\".nechat-thread_active\")) collapse()\n\t\t\t})"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line panel.gox:176
			if !p.Params.Off(params.ThreadEdit) {
				__e = __c.Init("script"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const side = document.currentScript.closest(\".nechat-aside\")\n\t\t\t\tlet timer = null\n\t\t\t\tlet fired = false\n\t\t\t\tconst cancel = () => {\n\t\t\t\t\tclearTimeout(timer)\n\t\t\t\t\ttimer = null\n\t\t\t\t}\n\t\t\t\tside.addEventListener(\n\t\t\t\t\t\"touchstart\",\n\t\t\t\t\t(e) => {\n\t\t\t\t\t\tconst row = e.target.closest(\".nechat-row\")\n\t\t\t\t\t\tif (!row) return\n\t\t\t\t\t\tfired = false\n\t\t\t\t\t\ttimer = setTimeout(() => {\n\t\t\t\t\t\t\ttimer = null\n\t\t\t\t\t\t\tconst kebab = row.querySelector(\".nechat-iconbtn\")\n\t\t\t\t\t\t\tif (!kebab) return\n\t\t\t\t\t\t\tfired = true\n\t\t\t\t\t\t\tkebab.click()\n\t\t\t\t\t\t}, 500)\n\t\t\t\t\t},\n\t\t\t\t\t{ passive: true },\n\t\t\t\t)\n\t\t\t\tside.addEventListener(\"touchmove\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\"touchend\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\"touchcancel\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\n\t\t\t\t\t\"click\",\n\t\t\t\t\t(e) => {\n\t\t\t\t\t\tif (!fired) return\n\t\t\t\t\t\tfired = false\n\t\t\t\t\t\tif (e.target.closest(\".nechat-rowmenu\")) return\n\t\t\t\t\t\te.preventDefault()\n\t\t\t\t\t\te.stopPropagation()\n\t\t\t\t\t},\n\t\t\t\t\ttrue,\n\t\t\t\t)"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line panel.gox:218
}

//line panel.gox:220
func (p *Aside) match(t api.Thread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line panel.gox:221
			__e = __c.Set("class", "nechat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line panel.gox:222
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				if err := p.Instance.ThreadOpen(ctx, t.ID, nil); err != nil {
					doors.Logger(ctx).Error("aside: open", "thread", t.ID, "err", err)
					notify.Error(ctx, err)
					return false
				}
				rp.After(doors.ActionEmit[any]{Name: "opened"})
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line panel.gox:234
					__e = __c.Set("class", "nechat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:235
						__e = __c.Set("class", "nechat-dot nechat-dot_off"); if __e != nil { return }
//line panel.gox:235
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line panel.gox:236
					__e = __c.Any(name(t.Name)); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:237
						__e = __c.Set("class", "nechat-date"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line panel.gox:237
						__e = __c.Any(shortDate(t.Updated, doors.BrowserLocale(ctx).Get().TimeZone)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line panel.gox:240
}

//line panel.gox:242
func name(n string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:243
		if n == "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:244
				__e = __c.Set("class", "nechat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Untitled"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:246
				__e = __c.Set("class", "nechat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:246
				__e = __c.Any(n); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line panel.gox:248
}
