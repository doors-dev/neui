// Managed by GoX v0.3.3

//line row.gox:1
package aside

import (
	"context"
	"strings"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/params"
	"neui.dev/kchat/notify"
)

type renameForm struct {
	Name string `form:"name"`
}

type row struct {
	aside *Aside
	thread api.Thread
	instance string
	day *doors.Door
}

//line row.gox:26
func (r *row) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:27
			__e = __c.Set("class", "kchat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:28
			if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:29
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuOpen}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:30
					__e = __c.Any(r.actions(hide)); if __e != nil { return }
				return })
//line row.gox:31
			})); if __e != nil { return }
//line row.gox:32
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuRename}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:33
					__e = __c.Any(r.rename(hide)); if __e != nil { return }
				return })
//line row.gox:34
			})); if __e != nil { return }
//line row.gox:35
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuDelete}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:36
					__e = __c.Any(r.confirm(hide)); if __e != nil { return }
				return })
//line row.gox:37
			})); if __e != nil { return }
			}
//line row.gox:39
			__e = __c.Any(r.open()); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:41
}

//line row.gox:43
func (r *row) open() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:44
		if r.instance == r.aside.Instance.ID() {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:45
				__e = __c.Set("class", "kchat-thread kchat-thread_active"); if __e != nil { return }
//line row.gox:45
				__e = __c.Set("aria-current", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line row.gox:46
					__e = __c.Set("class", "kchat-dot"); if __e != nil { return }
//line row.gox:46
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line row.gox:47
				__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line row.gox:50
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				r.aside.Instance.ThreadOpen(ctx, r.thread.ID, nil)
				rp.After(doors.ActionEmit[any]{Name: "opened"})
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line row.gox:58
					__e = __c.Set("class", "kchat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:59
					if r.instance == "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:60
							__e = __c.Set("class", "kchat-dot kchat-dot_off"); if __e != nil { return }
//line row.gox:60
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:62
							__e = __c.Set("class", "kchat-dot"); if __e != nil { return }
//line row.gox:62
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line row.gox:64
					__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
//line row.gox:67
		if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:68
			__e = __c.Any(r.aside.menu.BindControls(
			chatMenu{id: r.thread.ID, mode: chatMenuOpen},
			func(show func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:71
					__e = __c.Any(r.kebab(show)); if __e != nil { return }
				return })
//line row.gox:72
			},
			func(hide func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:74
					__e = __c.Any(r.kebab(hide)); if __e != nil { return }
				return })
//line row.gox:75
			},
		)); if __e != nil { return }
		}
	return })
//line row.gox:78
}

//line row.gox:80
func (r *row) kebab(toggle func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:81
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			toggle(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line row.gox:88
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line row.gox:88
				__e = __c.Set("aria-label", "Chat actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:89
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:89
					__e = __c.Any(icons.DotsThreeVertical()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:91
}

//line row.gox:93
func (r *row) actions(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:94
			__e = __c.Set("class", "kchat-rowmenu kchat-popbody"); if __e != nil { return }
//line row.gox:94
			__e = __c.Set("role", "menu"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:95
			__e = __c.Any(r.action(icons.Pencil(), "Rename", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuRename})
		})); if __e != nil { return }
//line row.gox:98
			__e = __c.Any(r.action(icons.Copy(), "Clone", func(ctx context.Context) {
			id, err := r.aside.Instance.ThreadClone(ctx, r.thread.ID)
			if err != nil {
				doors.Logger(ctx).Error("aside: clone", "thread", r.thread.ID, "err", err)
				notify.Error(ctx, err)
				hide(ctx)
				return
			}
			r.aside.Instance.ThreadOpen(ctx, id, nil)
			hide(ctx)
		})); if __e != nil { return }
//line row.gox:109
			__e = __c.Any(r.action(icons.Trash(), "Delete chat", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuDelete})
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:113
}

//line row.gox:115
func (r *row) action(icon gox.Elem, label string, on func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:116
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			on(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line row.gox:123
				__e = __c.Set("class", "kchat-act"); if __e != nil { return }
//line row.gox:123
				__e = __c.Set("role", "menuitem"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:124
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:124
					__e = __c.Any(icon); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:125
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:127
}

//line row.gox:129
func (r *row) rename(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:130
		__e = (doors.ASubmit[renameForm]{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rf doors.RequestForm[renameForm]) bool {
			n := strings.TrimSpace(rf.Data().Name)
			if n != r.thread.Name {
				if err := r.aside.Instance.ThreadRename(ctx, r.thread.ID, n); err != nil {
					doors.Logger(ctx).Error("aside: rename", "thread", r.thread.ID, "err", err)
					notify.Error(ctx, err)
				} else {
					r.day.Reload(ctx)
				}
			}
			hide(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("form"); if __e != nil { return }
			{
//line row.gox:146
				__e = __c.Set("class", "kchat-form"); if __e != nil { return }
//line row.gox:146
				__e = __c.Set("aria-label", "Rename chat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line row.gox:147
					__e = __c.Set("class", "kchat-form-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:148
					__e = (doors.AKeyDown{
				Key: doors.Key{Key: "Escape"},
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rk doors.RequestKeyboard) bool {
					hide(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitVoid("input"); if __e != nil { return }
						{
//line row.gox:157
							__e = __c.Set("class", "kchat-text"); if __e != nil { return }
//line row.gox:158
							__e = __c.Set("name", "name"); if __e != nil { return }
//line row.gox:159
							__e = __c.Set("value", r.thread.Name); if __e != nil { return }
							__e = __c.Set("autofocus", true); if __e != nil { return }
//line row.gox:161
							__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line row.gox:162
							__e = __c.Set("aria-label", "Chat name"); if __e != nil { return }
						}
						__e = __c.Submit(); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("script"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Raw("const input = document.currentScript.previousElementSibling\n\t\t\t\tinput.focus()\n\t\t\t\tinput.setSelectionRange(input.value.length, input.value.length)"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line row.gox:169
					__e = __c.Set("class", "kchat-form-actions"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:170
					__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					hide(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line row.gox:177
							__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:177
							__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Cancel"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:178
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line row.gox:178
						__e = __c.Set("class", "kchat-btn"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Save"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:181
}

//line row.gox:183
func (r *row) confirm(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:185
		label := r.thread.Name
	if label == "" {
		label = "Untitled"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:190
			__e = __c.Set("class", "kchat-form"); if __e != nil { return }
//line row.gox:190
			__e = __c.Set("role", "group"); if __e != nil { return }
//line row.gox:190
			__e = __c.Set("aria-label", "Delete chat"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:191
				__e = __c.Set("class", "kchat-form-text"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Delete "); if __e != nil { return }
				__e = __c.Init("b"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:191
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("?"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:192
				__e = __c.Set("class", "kchat-form-actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:193
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					hide(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:200
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:200
						__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Cancel"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line row.gox:201
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					if err := r.aside.Instance.ThreadDelete(ctx, r.thread.ID); err != nil {
						doors.Logger(ctx).Error("aside: delete", "thread", r.thread.ID, "err", err)
						notify.Error(ctx, err)
					} else {
						r.day.Reload(ctx)
					}
					hide(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:214
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:214
						__e = __c.Set("class", "kchat-btn kchat-btn_del"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Delete"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:217
}
