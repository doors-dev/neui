// Managed by GoX v0.4.0

//line chat.gox:1
package chat

import (
	"context"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/instance"
	"neui.dev/nekit/internal/chat/aside"
	"neui.dev/nekit/internal/chat/chat"
	"neui.dev/nekit/internal/chat/css"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/layout"
	"neui.dev/nekit/internal/chat/notify"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/pops"
)

// Chat is the ready-made chat widget: the chat list sidebar, the thread, the
// composer and its popovers. Render it on a page as ~nekit.Chat{}; the zero
// value is a complete chat. It fills its container, so give that container a
// height.
type Chat struct {
	// MaxAttachmentSize is the largest file, in bytes, a user may attach. Zero means 12 MiB.
	MaxAttachmentSize int
	// MaxAttachments is how many files one draft may hold. Zero means 10.
	MaxAttachments int
	// AttachmentTimeout bounds one file upload. Zero means 10 minutes.
	AttachmentTimeout time.Duration
	// AsideRight puts the chat list sidebar on the right instead of the left.
	AsideRight bool
	// Framed draws a rounded border around the widget.
	Framed bool
	// ThemeDark recolours the dark face, shown under a dark system scheme or
	// inside data-theme="dark". The zero value keeps the built-in palette.
	ThemeDark css.Theme
	// ThemeLight recolours the light face, shown under a light system scheme or
	// inside data-theme="light". The zero value keeps the built-in palette.
	ThemeLight css.Theme
	// Layout sets the chat's side and sizes; the zero value keeps the defaults.
	Layout css.Layout
	// GreetingTitle is the heading a new chat shows before its first message:
	// a string or a GoX element. nil shows "How can I help?"; "" leaves it out.
	GreetingTitle any
	// GreetingBody is what shows under GreetingTitle: a string or a GoX element,
	// such as the host's own template. nil shows "Ask a question or describe
	// what you need."; "" leaves it out.
	GreetingBody any
	// Disable lists features hidden from the user; nil shows everything.
	// ChatSimple is the preset for non-technical users and grows with technical features added later.
	Disable []params.Feature
}

type windowStateWrapper struct {
	State *layout.State
}

type windowStateKey struct{}

// Main renders the widget for the doors instance (browser tab) being
// rendered. It shows a disabled chat until neui.Enable succeeds for the
// session, and again after neui.Disable.
func (k Chat) Main() gox.Elem {
	params := params.Params{
		MaxAttachmentSize: k.MaxAttachmentSize,
		MaxAttachments: k.MaxAttachments,
		AttachmentTimeout: k.AttachmentTimeout,
		AsideRight: k.AsideRight,
		Framed: k.Framed,
		ThemeDark: k.ThemeDark,
		ThemeLight: k.ThemeLight,
		Layout: k.Layout,
		GreetingTitle: k.GreetingTitle,
		GreetingBody: k.GreetingBody,
	}
	params.InitDefault()
	params.Disable(k.Disable)
	return gox.Elem(func(cur gox.Cursor) error {
		state := doors.TabState[windowStateWrapper](cur.Context(), "neui-chat")
		initlialized := false
		state.Sub(cur.Context(), func(ctx context.Context, ws *windowStateWrapper) bool {
			if ws == nil {
				return false
			}
			if !initlialized && ws.State != nil {
				initlialized = true
				return false
			}
			if initlialized {
				doors.SessionStore(ctx).Save(windowStateKey{}, *ws.State)
				return false
			}
			s, _ := doors.SessionStore(ctx).Load(windowStateKey{}).(layout.State)
			state.Update(ctx, &windowStateWrapper{State: &s})
			return false
		})
		inst := instance.GetInstance(cur.Context())
		enabled := doors.DeriveBeam(inst.State(), func(s api.InstanceState) bool {
			return s.Status != api.InstanceDisabled
		})
		return cur.Comp(state.Route(
			doors.RouteMatch(
				func(ws *windowStateWrapper) bool {
					return ws != nil && ws.State != nil
				},
				doors.RouteDerive(func(source doors.Source[*windowStateWrapper]) doors.Source[layout.State] {
					return doors.DeriveSource(source, func(w *windowStateWrapper) layout.State {
						if w == nil || w.State == nil {
							return layout.State{}
						}
						return *w.State
					}, func(_ *windowStateWrapper, w layout.State) *windowStateWrapper {
						return &windowStateWrapper{State: &w}
					})
				}, doors.RouteSource(func(s doors.Source[layout.State]) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line chat.gox:119
						enabled, _ := enabled.Effect(ctx)
					if !enabled {
						return
					}

//line chat.gox:124
						__e = __c.Any(&widget{
						params: params,
						state: s,
						inst: inst,
						pops: pops.NewPopups(),
					}); if __e != nil { return }
					return })
//line chat.gox:130
				})),
			),
		))
	})
}

type widget struct {
	params params.Params
	state doors.Source[layout.State]
	inst api.Instance
	pops pops.Popups
}

//line chat.gox:143
func (k *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:145
		shell := new(doors.Setter)
	state, _ := k.state.Read(ctx)
	badge := doors.NewSource(chat.Badge{Collapsed: !state.Expanded, Block: "text"})
	mode, _ := doors.DeriveBeam(k.state, func(s layout.State) string {
		return layout.Mode(s, !k.params.Off(params.Dock))
	}).ReadAndSub(ctx, func(ctx context.Context, m string) bool {
		doors.Call(ctx, shell.Set("data-mode", m))
		badge.Mutate(ctx, func(b chat.Badge) chat.Badge {
			b.Collapsed = m == "collapsed"
			if !b.Collapsed {
				b.Ended = false
				b.Failed = false
			}
			return b
		})
		return false
	})
	launcher := new(doors.Setter)
	label := func(b string) string {
		switch b {
		case "ask":
			return "Open chat, question for you"
		case "run-text", "run-thinking", "run-tools":
			return "Open chat, agent working"
		case "failed":
			return "Open chat, round failed"
		case "ended":
			return "Open chat, answer ready"
		}
		return "Open chat"
	}
	current, _ := doors.DeriveBeam(badge, func(b chat.Badge) string {
		switch {
		case b.Asking:
			return "ask"
		case b.Running:
			return "run-" + b.Block
		case b.Failed:
			return "failed"
		case b.Ended:
			return "ended"
		}
		return ""
	}).ReadAndSub(ctx, func(ctx context.Context, b string) bool {
		doors.Call(ctx, launcher.Set("data-badge", b))
		doors.Call(ctx, launcher.Set("aria-label", label(b)))
		return false
	})
	scope := new(doors.ScopeBlocking)
	toggle := doors.AClick{
		Scope: scope,
		On: func(ctx context.Context, _ doors.RequestPointer) bool {
			k.state.Mutate(ctx, func(s layout.State) layout.State {
				s.Expanded = !s.Expanded
				return s
			})
			return false
		},
		OnError: notify.OnError,
	}
	dock := doors.AClick{
		Scope: scope,
		On: func(ctx context.Context, _ doors.RequestPointer) bool {
			if k.params.Off(params.Dock) {
				return false
			}
			k.state.Mutate(ctx, func(s layout.State) layout.State {
				s.Dock = !s.Dock
				return s
			})
			return false
		},
		OnError: notify.OnError,
	}
	dockIcon, undockIcon := icons.DockRight, icons.Undock
	if k.params.Layout.Side == css.Left {
		dockIcon, undockIcon = icons.DockLeft, icons.UndockLeft
	}

//line chat.gox:224
		__e = __c.Any(css.Style(k.params.ThemeDark, k.params.ThemeLight, k.params.Layout)); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line chat.gox:226
			__e = __c.Modify(shell); if __e != nil { return }
//line chat.gox:227
			__e = __c.Set("data-mode", mode); if __e != nil { return }
//line chat.gox:228
			__e = __c.Set("class", func() any {
			class := "nechat-widget nechat-widget_split"
			if k.params.Off(params.Sidebar) {
				class += " nechat-widget_noside"
			} else if k.params.AsideRight {
				class += " nechat-widget_aside-right"
			}
			if k.params.Framed {
				class += " nechat-widget_frame"
			}
			return class
		}()); if __e != nil { return }
//line chat.gox:240
			__e = __c.Set("data-nechat-theme", func() any {
			key := css.FaceKey(k.params.ThemeDark, k.params.ThemeLight)
			if key == "" {
				return nil
			}
			return key
		}()); if __e != nil { return }
//line chat.gox:247
			__e = __c.Set("data-nechat-layout", func() any {
			key := css.LayoutKey(k.params.Layout)
			if key == "" {
				return nil
			}
			return key
		}()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:254
				__e = __c.Set("class", "nechat-wctl"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:255
				if !k.params.Off(params.Dock) {
//line chat.gox:256
					__e = (dock).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line chat.gox:256
							__e = __c.Set("type", "button"); if __e != nil { return }
//line chat.gox:256
							__e = __c.Set("class", "nechat-iconbtn nechat-wctl-dock"); if __e != nil { return }
//line chat.gox:256
							__e = __c.Set("title", "Dock to the side"); if __e != nil { return }
//line chat.gox:256
							__e = __c.Set("aria-label", "Dock to the side"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line chat.gox:257
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line chat.gox:257
								__e = __c.Any(dockIcon()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
//line chat.gox:259
					__e = (dock).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line chat.gox:259
							__e = __c.Set("type", "button"); if __e != nil { return }
//line chat.gox:259
							__e = __c.Set("class", "nechat-iconbtn nechat-wctl-undock"); if __e != nil { return }
//line chat.gox:259
							__e = __c.Set("title", "Undock"); if __e != nil { return }
//line chat.gox:259
							__e = __c.Set("aria-label", "Undock"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line chat.gox:260
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line chat.gox:260
								__e = __c.Any(undockIcon()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
//line chat.gox:263
				__e = (toggle).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line chat.gox:263
						__e = __c.Set("type", "button"); if __e != nil { return }
//line chat.gox:263
						__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line chat.gox:263
						__e = __c.Set("title", "Collapse"); if __e != nil { return }
//line chat.gox:263
						__e = __c.Set("aria-label", "Collapse"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chat.gox:264
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line chat.gox:264
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line chat.gox:267
			if !k.params.Off(params.Sidebar) {
//line chat.gox:268
				__e = __c.Any(aside.Aside{
				Params: k.params,
				Instance: k.inst,
				Pops: k.pops,
				Layout: k.state,
			}); if __e != nil { return }
			}
//line chat.gox:275
			__e = __c.Any(chat.Chat{
			Params: k.params,
			Inst: k.inst,
			Pops: k.pops.Popup[string](),
			Badge: badge,
		}); if __e != nil { return }
//line chat.gox:281
			__e = __c.Any(k.pops.Script()); if __e != nil { return }
//line chat.gox:282
			__e = __c.Any(notify.Script()); if __e != nil { return }
//line chat.gox:283
			__e = (toggle).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line chat.gox:284
					__e = __c.Set("type", "button"); if __e != nil { return }
//line chat.gox:285
					__e = __c.Set("class", "nechat-launcher"); if __e != nil { return }
//line chat.gox:286
					__e = __c.Set("title", "Open chat"); if __e != nil { return }
//line chat.gox:287
					__e = __c.Modify(launcher); if __e != nil { return }
//line chat.gox:288
					__e = __c.Set("data-badge", current); if __e != nil { return }
//line chat.gox:289
					__e = __c.Set("aria-label", label(current)); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line chat.gox:290
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line chat.gox:290
						__e = __c.Any(icons.ChatCircle()); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line chat.gox:291
						__e = __c.Set("class", "nechat-badge nechat-badge_run"); if __e != nil { return }
//line chat.gox:291
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chat.gox:292
						__e = (doors.Class("nechat-ic nechat-ic_line nechat-build nechat-badge-text")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line chat.gox:292
							__e = __c.Any(icons.TextAlignLeftLines()); if __e != nil { return }
						return })); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line chat.gox:293
							__e = __c.Set("class", "nechat-blink nechat-badge-thinking"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line chat.gox:294
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line chat.gox:294
								__e = __c.Any(icons.Lightbulb()); if __e != nil { return }
							return })); if __e != nil { return }
//line chat.gox:295
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line chat.gox:295
								__e = __c.Any(icons.LightbulbFill()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line chat.gox:297
						__e = (doors.Class("nechat-ic nechat-ic_spin nechat-badge-tools")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line chat.gox:297
							__e = __c.Any(icons.Gear()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line chat.gox:299
						__e = __c.Set("class", "nechat-badge nechat-badge_ask"); if __e != nil { return }
//line chat.gox:299
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chat.gox:300
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line chat.gox:300
							__e = __c.Any(icons.CheckSquareFill()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line chat.gox:302
						__e = __c.Set("class", "nechat-badge nechat-badge_ended"); if __e != nil { return }
//line chat.gox:302
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line chat.gox:303
						__e = __c.Set("class", "nechat-badge nechat-badge_failed"); if __e != nil { return }
//line chat.gox:303
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chat.gox:304
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line chat.gox:304
							__e = __c.Any(icons.Exclamation()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const widget = document.currentScript.closest(\".nechat-widget\")\n\t\t\tconst observer = new MutationObserver(() => {\n\t\t\t\tif (widget.dataset.mode === \"collapsed\") {\n\t\t\t\t\twidget.querySelector(\".nechat-launcher\").focus()\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\twidget.querySelector(\".nechat-draft\")?.focus({ preventScroll: true })\n\t\t\t})\n\t\t\tobserver.observe(widget, { attributes: true, attributeFilter: [\"data-mode\"] })\n\t\t\t$sys.clean(() => observer.disconnect())"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line chat.gox:320
}
