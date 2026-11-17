// Managed by GoX v0.4.0

//line chat.gox:1
package chat

import (
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/instance"
	"neui.dev/nekit/internal/chat/aside"
	"neui.dev/nekit/internal/chat/chat"
	"neui.dev/nekit/internal/chat/css"
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
		GreetingTitle: k.GreetingTitle,
		GreetingBody: k.GreetingBody,
	}
	params.InitDefault()
	params.Disable(k.Disable)
	return gox.Elem(func(cur gox.Cursor) error {
		inst := instance.GetInstance(cur.Context())
		return cur.Comp(&widget{
			params: params,
			inst: inst,
			synced: inst.Synced(),
			pops: pops.NewPopups(),
		})
	})
}

type widget struct {
	params params.Params
	inst api.Instance
	synced doors.Beam[bool]
	pops pops.Popups
}

//line chat.gox:86
func (k *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:87
		__e = __c.Any(css.Style(k.params.ThemeDark, k.params.ThemeLight)); if __e != nil { return }
//line chat.gox:88
		__e = (doors.Ctx(notify.Root(ctx))).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:89
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
//line chat.gox:101
				__e = __c.Set("data-nechat-theme", func() any {
			key := css.FaceKey(k.params.ThemeDark, k.params.ThemeLight)
			if key == "" {
				return nil
			}
			return key
		}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:108
				if !k.params.Off(params.Sidebar) {
//line chat.gox:109
					__e = __c.Any(k.synced.Bind(func(synced bool) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line chat.gox:110
					if synced {
//line chat.gox:111
						__e = __c.Any(aside.Aside{
						Params: k.params,
						Instance: k.inst,
						Pops: k.pops,
					}); if __e != nil { return }
					}
				return })
//line chat.gox:117
			})); if __e != nil { return }
				}
//line chat.gox:119
				__e = __c.Any(chat.Chat{
			Params: k.params,
			Inst: k.inst,
			Pops: k.pops.Popup[string](),
		}); if __e != nil { return }
//line chat.gox:124
				__e = __c.Any(k.pops.Script()); if __e != nil { return }
//line chat.gox:125
				__e = __c.Any(notify.Script()); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line chat.gox:127
}
