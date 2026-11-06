// Managed by GoX v0.3.3

//line panel.gox:1
package aside

import (
	"context"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/aside/controller"
	"neui.dev/kchat/pops"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/params"
	"neui.dev/kchat/notify"
)

type chatMenuMode int

const (
	chatMenuNone chatMenuMode = iota
	chatMenuOpen
	chatMenuRename
	chatMenuDelete
)

// chatMenu is which row has an overlay up and which one; one row at a time
// falls out of there being a single value.
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

//line panel.gox:41
func (p Aside) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:43
		p.controller = controller.New()
	p.menu = p.Pops.Popup[chatMenu]()

//line panel.gox:46
		__e = __c.Any(p.controller.Show(p.aside())); if __e != nil { return }
//line panel.gox:47
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
//line panel.gox:54
				__e = __c.Set("class", "kchat-iconbtn kchat-burger"); if __e != nil { return }
//line panel.gox:54
				__e = __c.Set("title", "Menu"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:55
				__e = __c.Any(p.controller.RouteToggle(
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:56
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:56
						__e = __c.Any(icons.List()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:56
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:57
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:57
						__e = __c.Any(icons.Close()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:57
			return }),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line panel.gox:58
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:58
						__e = __c.Any(icons.ArrowLeft()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line panel.gox:58
			return }),
		)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line panel.gox:61
}

//line panel.gox:63
func (p *Aside) aside() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("aside"); if __e != nil { return }
		{
//line panel.gox:64
			__e = __c.Set("class", "kchat-aside"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:65
				__e = __c.Set("class", "kchat-top"); if __e != nil { return }
//line panel.gox:65
				__e = __c.Set("role", "search"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("label"); if __e != nil { return }
				{
//line panel.gox:66
					__e = __c.Set("class", "kchat-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line panel.gox:67
					__e = (doors.Class("kchat-ic-sm")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:67
						__e = __c.Any(icons.MagnifyingGlass()); if __e != nil { return }
					return })); if __e != nil { return }
//line panel.gox:68
					__e = __c.Any(p.controller.Input(func(v string) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:69
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
//line panel.gox:81
								__e = __c.Set("value", v); if __e != nil { return }
//line panel.gox:82
								__e = __c.Set("class", "kchat-text"); if __e != nil { return }
//line panel.gox:83
								__e = __c.Set("type", "search"); if __e != nil { return }
//line panel.gox:84
								__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line panel.gox:85
								__e = __c.Set("aria-label", "Search chats"); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
						return })); if __e != nil { return }
					return })
//line panel.gox:86
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line panel.gox:89
				__e = __c.Set("class", "kchat-side-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:90
				__e = __c.Any(p.controller.RouteList(
				func(w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:92
						__e = __c.Any(&days{aside: p, window: w, save: set}); if __e != nil { return }
					return })
//line panel.gox:93
				},
				func(input string, open func(ctx context.Context, kind controller.SearchKind)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:95
						__e = __c.Any(p.overview(input, open)); if __e != nil { return }
					return })
//line panel.gox:96
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:98
						if input == "" {
//line panel.gox:99
							__e = __c.Any(p.noResults(controller.SearchNames)); if __e != nil { return }
						} else  {
//line panel.gox:101
							__e = __c.Any(&names{aside: p, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:103
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:105
						if input == "" {
//line panel.gox:106
							__e = __c.Any(p.noResults(controller.SearchMessagesCurrent)); if __e != nil { return }
						} else  {
//line panel.gox:108
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:110
				},
				func(input string, w controller.Window, set func(ctx context.Context, w controller.Window)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line panel.gox:112
						if input == "" {
//line panel.gox:113
							__e = __c.Any(p.noResults(controller.SearchMessagesAll)); if __e != nil { return }
						} else  {
//line panel.gox:115
							__e = __c.Any(&hits{aside: p, thread: p.Instance.State().Get().Thread.ID, exclude: true, query: input, window: w, save: set}); if __e != nil { return }
						}
					return })
//line panel.gox:117
				},
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line panel.gox:120
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
					__e = __c.Raw("const side = document.currentScript.closest(\".kchat-aside\")\n\t\t\tconst widget = side.closest(\".kchat-widget\")\n\t\t\t$on(\"opened\", () => {\n\t\t\t\tif (getComputedStyle(side).position !== \"absolute\") return\n\t\t\t\treturn $hook(\"collapse\", true).catch((err) => {\n\t\t\t\t\tif (err instanceof HookErr)\n\t\t\t\t\t\twidget.dispatchEvent(new CustomEvent(\"kchat_error\", { detail: err.kind }))\n\t\t\t\t})\n\t\t\t})"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line panel.gox:138
			if !p.Params.Off(params.ThreadEdit) {
				__e = __c.Init("script"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const side = document.currentScript.closest(\".kchat-aside\")\n\t\t\t\t// touch has no hover, so a press that outlasts the timer opens the\n\t\t\t\t// same menu the kebab does and swallows the click that follows\n\t\t\t\tlet timer = null\n\t\t\t\tlet fired = false\n\t\t\t\tconst cancel = () => {\n\t\t\t\t\tclearTimeout(timer)\n\t\t\t\t\ttimer = null\n\t\t\t\t}\n\t\t\t\tside.addEventListener(\n\t\t\t\t\t\"touchstart\",\n\t\t\t\t\t(e) => {\n\t\t\t\t\t\tconst row = e.target.closest(\".kchat-row\")\n\t\t\t\t\t\tif (!row) return\n\t\t\t\t\t\tfired = false\n\t\t\t\t\t\ttimer = setTimeout(() => {\n\t\t\t\t\t\t\ttimer = null\n\t\t\t\t\t\t\tconst kebab = row.querySelector(\".kchat-iconbtn\")\n\t\t\t\t\t\t\tif (!kebab) return\n\t\t\t\t\t\t\tfired = true\n\t\t\t\t\t\t\tkebab.click()\n\t\t\t\t\t\t}, 500)\n\t\t\t\t\t},\n\t\t\t\t\t{ passive: true },\n\t\t\t\t)\n\t\t\t\tside.addEventListener(\"touchmove\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\"touchend\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\"touchcancel\", cancel, { passive: true })\n\t\t\t\tside.addEventListener(\n\t\t\t\t\t\"click\",\n\t\t\t\t\t(e) => {\n\t\t\t\t\t\tif (!fired) return\n\t\t\t\t\t\tfired = false\n\t\t\t\t\t\tif (e.target.closest(\".kchat-rowmenu\")) return\n\t\t\t\t\t\te.preventDefault()\n\t\t\t\t\t\te.stopPropagation()\n\t\t\t\t\t},\n\t\t\t\t\ttrue,\n\t\t\t\t)"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line panel.gox:182
}

//line panel.gox:184
func (p *Aside) match(t api.Thread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line panel.gox:185
			__e = __c.Set("class", "kchat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line panel.gox:186
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				p.Instance.ThreadOpen(ctx, t.ID, nil)
				rp.After(doors.ActionEmit[any]{Name: "opened"})
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line panel.gox:194
					__e = __c.Set("class", "kchat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:195
						__e = __c.Set("class", "kchat-dot kchat-dot_off"); if __e != nil { return }
//line panel.gox:195
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line panel.gox:196
					__e = __c.Any(name(t.Name)); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line panel.gox:197
						__e = __c.Set("class", "kchat-date"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line panel.gox:197
						__e = __c.Any(shortDate(t.Updated, p.Instance.Location())); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line panel.gox:200
}

//line panel.gox:202
func name(n string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line panel.gox:203
		if n == "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:204
				__e = __c.Set("class", "kchat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Untitled"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line panel.gox:206
				__e = __c.Set("class", "kchat-thread-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line panel.gox:206
				__e = __c.Any(n); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line panel.gox:208
}
