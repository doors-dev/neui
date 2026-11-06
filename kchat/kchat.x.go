// Managed by GoX v0.3.3

//line kchat.gox:1
package kchat

import (
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/instance"
	"neui.dev/kchat/aside"
	"neui.dev/kchat/chat"
	"neui.dev/kchat/css"
	"neui.dev/kchat/notify"
	"neui.dev/kchat/params"
	"neui.dev/kchat/pops"
)

// Theme overrides colours of the built-in palette for one face, dark or
// light; set it as Kchat.ThemeDark or Kchat.ThemeLight.
type Theme = css.Theme

// Feature is a part of the chat UI that can be hidden from the user.
type Feature = params.Feature

const (
	// Tokens hides the context gauge, token counts and the Compact now button; a plain "compacting" chip still shows while compaction runs.
	Tokens = Feature(params.Tokens)
	// Thinking hides reasoning and summary blocks and the Reasoning stat.
	Thinking = Feature(params.Thinking)
	// Tools hides tool call blocks and pinned record counts.
	Tools = Feature(params.Tools)
	// Tasks hides the background task chip and popover; the run chip shows "working" while only tasks run.
	Tasks = Feature(params.Tasks)
	// Edit hides the edit-and-resend button on user messages.
	Edit = Feature(params.Edit)
	// Retry hides the retry button on failed rounds.
	Retry = Feature(params.Retry)
	// Attachments hides the attach button and tray; large pastes stay in the draft as text.
	Attachments = Feature(params.Attachments)
	// Files hides the files popover in the top bar.
	Files = Feature(params.Files)
	// Sidebar hides the chat list sidebar and its menu button.
	Sidebar = Feature(params.Sidebar)
	// ThreadEdit hides rename, clone and delete in the chat list.
	ThreadEdit = Feature(params.ThreadEdit)
)

// Simple is the preset for non-technical users. It grows as technical features are added.
var Simple = []Feature{Tokens, Thinking, Tools, Tasks}

// Kchat is the ready-made chat widget: the chat list sidebar, the thread, the
// composer and its popovers. Render it on a page as ~kchat.Kchat{}; the zero
// value is a complete chat. It fills its container, so give that container a
// height.
type Kchat struct {
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
	ThemeDark Theme
	// ThemeLight recolours the light face, shown under a light system scheme or
	// inside data-theme="light". The zero value keeps the built-in palette.
	ThemeLight Theme
	// Disable lists features hidden from the user; nil shows everything.
	// Simple is the preset for non-technical users and grows with technical features added later.
	Disable []Feature
}

// Main renders the widget for the doors instance (browser tab) being
// rendered. It shows a disabled chat until neui.Enable succeeds for the
// session, and again after neui.Disable.
func (k Kchat) Main() gox.Elem {
	params := params.Params{
		MaxAttachmentSize: k.MaxAttachmentSize,
		MaxAttachments: k.MaxAttachments,
		AttachmentTimeout: k.AttachmentTimeout,
		AsideRight: k.AsideRight,
		Framed: k.Framed,
		ThemeDark: k.ThemeDark,
		ThemeLight: k.ThemeLight,
	}
	params.InitDefault()
	params.Disable(k.Disable)
	return gox.Elem(func(cur gox.Cursor) error {
		inst := instance.GetInstance(cur.Context())
		return cur.Comp(&kchat{
			params: params,
			inst: inst,
			synced: inst.Synced(),
			pops: pops.NewPopups(),
		})
	})
}

type kchat struct {
	params params.Params
	inst api.Instance
	synced doors.Beam[bool]
	pops pops.Popups
}

//line kchat.gox:110
func (k *kchat) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line kchat.gox:111
		__e = __c.Any(css.Style(k.params.ThemeDark, k.params.ThemeLight)); if __e != nil { return }
//line kchat.gox:112
		__e = (doors.Ctx(notify.Root(ctx))).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line kchat.gox:113
				__e = __c.Set("class", func() any {
			class := "kchat-widget kchat-widget_split"
			if k.params.Off(params.Sidebar) {
				class += " kchat-widget_noside"
			} else if k.params.AsideRight {
				class += " kchat-widget_aside-right"
			}
			if k.params.Framed {
				class += " kchat-widget_frame"
			}
			return class
		}()); if __e != nil { return }
//line kchat.gox:125
				__e = __c.Set("data-kchat-theme", func() any {
			key := css.FaceKey(k.params.ThemeDark, k.params.ThemeLight)
			if key == "" {
				return nil
			}
			return key
		}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line kchat.gox:132
				if !k.params.Off(params.Sidebar) {
//line kchat.gox:133
					__e = __c.Any(k.synced.Bind(func(synced bool) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line kchat.gox:134
					if synced {
//line kchat.gox:135
						__e = __c.Any(aside.Aside{
						Params: k.params,
						Instance: k.inst,
						Pops: k.pops,
					}); if __e != nil { return }
					}
				return })
//line kchat.gox:141
			})); if __e != nil { return }
				}
//line kchat.gox:143
				__e = __c.Any(chat.Chat{
			Params: k.params,
			Inst: k.inst,
			Pops: k.pops.Popup[string](),
		}); if __e != nil { return }
//line kchat.gox:148
				__e = __c.Any(k.pops.Script()); if __e != nil { return }
//line kchat.gox:149
				__e = __c.Any(notify.Script()); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line kchat.gox:151
}
