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

//line row.gox:25
func (r *row) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:26
			__e = __c.Set("class", "kchat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:27
			if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:28
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuOpen}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:29
					__e = __c.Any(r.actions(hide)); if __e != nil { return }
				return })
//line row.gox:30
			})); if __e != nil { return }
//line row.gox:31
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuRename}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:32
					__e = __c.Any(r.rename(hide)); if __e != nil { return }
				return })
//line row.gox:33
			})); if __e != nil { return }
//line row.gox:34
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuDelete}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:35
					__e = __c.Any(r.confirm(hide)); if __e != nil { return }
				return })
//line row.gox:36
			})); if __e != nil { return }
			}
//line row.gox:38
			__e = __c.Any(r.open()); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:40
}

//line row.gox:42
func (r *row) open() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:43
		if r.instance == r.aside.Instance.ID() {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:44
				__e = __c.Set("class", "kchat-thread kchat-thread_active"); if __e != nil { return }
//line row.gox:44
				__e = __c.Set("aria-current", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line row.gox:45
					__e = __c.Set("class", "kchat-dot"); if __e != nil { return }
//line row.gox:45
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line row.gox:46
				__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line row.gox:49
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				r.aside.Instance.ThreadOpen(ctx, r.thread.ID, nil)
				rp.After(doors.ActionEmit[any]{Name: "opened"})
				return false
			},
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line row.gox:56
					__e = __c.Set("class", "kchat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:57
					if r.instance == "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:58
							__e = __c.Set("class", "kchat-dot kchat-dot_off"); if __e != nil { return }
//line row.gox:58
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:60
							__e = __c.Set("class", "kchat-dot"); if __e != nil { return }
//line row.gox:60
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line row.gox:62
					__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
//line row.gox:65
		if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:66
			__e = __c.Any(r.aside.menu.BindControls(
			chatMenu{id: r.thread.ID, mode: chatMenuOpen},
			func(show func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:69
					__e = __c.Any(r.kebab(show)); if __e != nil { return }
				return })
//line row.gox:70
			},
			func(hide func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:72
					__e = __c.Any(r.kebab(hide)); if __e != nil { return }
				return })
//line row.gox:73
			},
		)); if __e != nil { return }
		}
	return })
//line row.gox:76
}

//line row.gox:78
func (r *row) kebab(toggle func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:79
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			toggle(ctx)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line row.gox:85
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line row.gox:85
				__e = __c.Set("aria-label", "Chat actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:86
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:86
					__e = __c.Any(icons.DotsThreeVertical()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:88
}

//line row.gox:90
func (r *row) actions(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:91
			__e = __c.Set("class", "kchat-rowmenu kchat-popbody"); if __e != nil { return }
//line row.gox:91
			__e = __c.Set("role", "menu"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:92
			__e = __c.Any(r.action(icons.Pencil(), "Rename", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuRename})
		})); if __e != nil { return }
//line row.gox:95
			__e = __c.Any(r.action(icons.Copy(), "Clone", func(ctx context.Context) {
			id, err := r.aside.Instance.ThreadClone(ctx, r.thread.ID)
			if err != nil {
				doors.Logger(ctx).Error("aside: clone", "thread", r.thread.ID, "err", err)
				hide(ctx)
				return
			}
			r.aside.Instance.ThreadOpen(ctx, id, nil)
			hide(ctx)
		})); if __e != nil { return }
//line row.gox:105
			__e = __c.Any(r.action(icons.Trash(), "Delete chat", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuDelete})
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:109
}

//line row.gox:111
func (r *row) action(icon gox.Elem, label string, on func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:112
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			on(ctx)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line row.gox:118
				__e = __c.Set("class", "kchat-act"); if __e != nil { return }
//line row.gox:118
				__e = __c.Set("role", "menuitem"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:119
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:119
					__e = __c.Any(icon); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:120
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:122
}

//line row.gox:124
func (r *row) rename(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:125
		__e = (doors.ASubmit[renameForm]{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rf doors.RequestForm[renameForm]) bool {
			n := strings.TrimSpace(rf.Data().Name)
			if n != r.thread.Name {
				if err := r.aside.Instance.ThreadRename(ctx, r.thread.ID, n); err != nil {
					doors.Logger(ctx).Error("aside: rename", "thread", r.thread.ID, "err", err)
				} else {
					r.day.Reload(ctx)
				}
			}
			hide(ctx)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("form"); if __e != nil { return }
			{
//line row.gox:139
				__e = __c.Set("class", "kchat-form"); if __e != nil { return }
//line row.gox:139
				__e = __c.Set("aria-label", "Rename chat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line row.gox:140
					__e = __c.Set("class", "kchat-form-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:141
					__e = (doors.AKeyDown{
				Key: doors.Key{Key: "Escape"},
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rk doors.RequestKeyboard) bool {
					hide(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitVoid("input"); if __e != nil { return }
						{
//line row.gox:149
							__e = __c.Set("class", "kchat-text"); if __e != nil { return }
//line row.gox:150
							__e = __c.Set("name", "name"); if __e != nil { return }
//line row.gox:151
							__e = __c.Set("value", r.thread.Name); if __e != nil { return }
							__e = __c.Set("autofocus", true); if __e != nil { return }
//line row.gox:153
							__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line row.gox:154
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
//line row.gox:161
					__e = __c.Set("class", "kchat-form-actions"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:162
					__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					hide(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line row.gox:168
							__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:168
							__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Cancel"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:169
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line row.gox:169
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
//line row.gox:172
}

//line row.gox:174
func (r *row) confirm(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:176
		label := r.thread.Name
	if label == "" {
		label = "Untitled"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:181
			__e = __c.Set("class", "kchat-form"); if __e != nil { return }
//line row.gox:181
			__e = __c.Set("role", "group"); if __e != nil { return }
//line row.gox:181
			__e = __c.Set("aria-label", "Delete chat"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:182
				__e = __c.Set("class", "kchat-form-text"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Delete "); if __e != nil { return }
				__e = __c.Init("b"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:182
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("?"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:183
				__e = __c.Set("class", "kchat-form-actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:184
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					hide(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:190
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:190
						__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Cancel"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line row.gox:191
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					if err := r.aside.Instance.ThreadDelete(ctx, r.thread.ID); err != nil {
						doors.Logger(ctx).Error("aside: delete", "thread", r.thread.ID, "err", err)
					} else {
						r.day.Reload(ctx)
					}
					hide(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:202
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:202
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
//line row.gox:205
}
