// Managed by GoX v0.4.0

//line panel.gox:1
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
	menu pops.Popup[chatMenu]
	controller controller.Controller
}

//line panel.gox:40
func (p Aside) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:42
		p.controller = controller.New(p.Params.Off(params.MessageSearch))
	p.menu = p.Pops.Popup[chatMenu]()
	burger := new(doors.Setter)
	expanded, _ := p.controller.Expanded().ReadAndSub(ctx, func(ctx context.Context, v bool) bool {
		doors.Call(ctx, burger.Set("aria-expanded", strconv.FormatBool(v)))
		return false
	})

//line panel.gox:50
		__e = __c.Any(p.controller.Show(p.aside())); if __e != nil { return }
//line panel.gox:51
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
//line panel.gox:58
				__e = __c.Set("class", "nechat-iconbtn nechat-burger"); if __e != nil { return }
//line panel.gox:58
				__e = __c.Set("title", "Menu"); if __e != nil { return }
//line panel.gox:58
				__e = __c.Modify(burger); if __e != nil { return }
//line panel.gox:58
				__e = __c.Set("aria-expanded", strconv.FormatBool(expanded)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:59
				__e = __c.Any(p.controller.RouteToggle(
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:60
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:60
						__e = __c.Any(icons.List()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:60
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:61
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:61
						__e = __c.Any(icons.Close()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:61
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:62
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:62
						__e = __c.Any(icons.ArrowLeft()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:62
			return }),
		)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line panel.gox:65
}

//line panel.gox:67
func (p *Aside) aside() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("aside"); if __e != nil { return }
		{
//line panel.gox:68
			__e = __c.Set("class", "nechat-aside"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:69
				__e = __c.Set("class", "nechat-top"); if __e != nil { return }
//line panel.gox:69
				__e = __c.Set("role", "search"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("label"); if __e != nil { return }
				{
//line panel.gox:70
					__e = __c.Set("class", "nechat-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line panel.gox:71
					__e = (doors.Class("nechat-ic-sm")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:71
						__e = __c.Any(icons.MagnifyingGlass()); if __e != nil { return }
					return })); if __e != nil { return }
//line panel.gox:72
					__e = __c.Any(p.controller.Input(func(v string) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:73
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
//line panel.gox:85
								__e = __c.Set("value", v); if __e != nil { return }
//line panel.gox:86
								__e = __c.Set("class", "nechat-text"); if __e != nil { return }
//line panel.gox:87
								__e = __c.Set("type", "search"); if __e != nil { return }
//line panel.gox:88
								__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line panel.gox:89
								__e = __c.Set("aria-label", "Search chats"); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
						return })); if __e != nil { return }
					return })
//line panel.gox:90
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:93
				__e = __c.Set("class", "nechat-side-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:94
				__e = __c.Any(p.controller.RouteList(
				func(w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:96
						__e = __c.Any(&days{aside: p, window: w, save: set}); if __e != nil { return }
					return })
//line panel.gox:97
				},
				func(input string, open func(ctx context.Context, kind controller.SearchKind)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:99
						__e = __c.Any(p.overview(input, open)); if __e != nil { return }
					return })
//line panel.gox:100
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:102
						if input == "" {
//line panel.gox:103
							__e = __c.Any(p.noResults(controller.SearchNames)); if __e != nil { return }
						} else  {
//line panel.gox:105
							__e = __c.Any(&names{aside: p, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:107
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:109
						if input == "" {
//line panel.gox:110
							__e = __c.Any(p.noResults(controller.SearchMessagesCurrent)); if __e != nil { return }
						} else  {
//line panel.gox:112
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:114
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:116
						if input == "" {
//line panel.gox:117
							__e = __c.Any(p.noResults(controller.SearchMessagesAll)); if __e != nil { return }
						} else  {
//line panel.gox:119
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, exclude: true, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:121
				},
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line panel.gox:124
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
					__e = __c.Raw("const side = document.currentScript.closest(\".nechat-aside\")\n\t\t\tconst widget = side.closest(\".nechat-widget\")\n\t\t\t$on(\"opened\", () => {\n\t\t\t\tif (getComputedStyle(side).position !== \"absolute\") return\n\t\t\t\treturn $hook(\"collapse\", true).catch((err) => {\n\t\t\t\t\tif (err instanceof HookErr)\n\t\t\t\t\t\twidget.dispatchEvent(new CustomEvent(\"nechat_error\", { detail: err.kind }))\n\t\t\t\t})\n\t\t\t})"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line panel.gox:142
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
//line panel.gox:184
}

//line panel.gox:186
func (p *Aside) match(t api.Thread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line panel.gox:187
			__e = __c.Set("class", "nechat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line panel.gox:188
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
//line panel.gox:200
					__e = __c.Set("class", "nechat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:201
						__e = __c.Set("class", "nechat-dot nechat-dot_off"); if __e != nil { return }
//line panel.gox:201
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line panel.gox:202
					__e = __c.Any(name(t.Name)); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:203
						__e = __c.Set("class", "nechat-date"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line panel.gox:203
						__e = __c.Any(shortDate(t.Updated, p.Instance.Location())); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line panel.gox:206
}

//line panel.gox:208
func name(n string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:209
		if n == "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:210
				__e = __c.Set("class", "nechat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Untitled"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:212
				__e = __c.Set("class", "nechat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:212
				__e = __c.Any(n); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line panel.gox:214
}
