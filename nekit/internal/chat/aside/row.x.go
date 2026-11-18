// Managed by GoX v0.4.0

//line row.gox:1
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
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/notify"
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

//line row.gox:40
func (r *row) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:41
			__e = __c.Set("class", "nechat-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:42
			if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:43
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuOpen}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:44
					__e = __c.Any(r.actions(hide)); if __e != nil { return }
				return })
//line row.gox:45
			})); if __e != nil { return }
//line row.gox:46
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuRename}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:47
					__e = __c.Any(r.rename(hide)); if __e != nil { return }
				return })
//line row.gox:48
			})); if __e != nil { return }
//line row.gox:49
				__e = __c.Any(r.aside.menu.BindPop(chatMenu{id: r.thread.ID, mode: chatMenuDelete}, func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:50
					__e = __c.Any(r.confirm(hide)); if __e != nil { return }
				return })
//line row.gox:51
			})); if __e != nil { return }
			}
//line row.gox:53
			__e = __c.Any(r.open()); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:55
}

//line row.gox:57
func (r *row) open() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:58
		if r.instance == r.aside.Instance.ID() {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:59
				__e = __c.Set("class", "nechat-thread nechat-thread_active"); if __e != nil { return }
//line row.gox:59
				__e = __c.Set("aria-current", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line row.gox:60
					__e = __c.Set("class", "nechat-dot"); if __e != nil { return }
//line row.gox:60
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line row.gox:61
				__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line row.gox:64
			__e = (doors.AClick{
			Scope: new(doors.ScopeBlocking),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				if err := r.aside.Instance.ThreadOpen(ctx, r.thread.ID, nil); err != nil {
					doors.Logger(ctx).Error("aside: open", "thread", r.thread.ID, "err", err)
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
//line row.gox:76
					__e = __c.Set("class", "nechat-thread"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:77
					if r.instance == "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:78
							__e = __c.Set("class", "nechat-dot nechat-dot_off"); if __e != nil { return }
//line row.gox:78
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line row.gox:80
							__e = __c.Set("class", "nechat-dot"); if __e != nil { return }
//line row.gox:80
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line row.gox:82
					__e = __c.Any(name(r.thread.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
//line row.gox:85
		if !r.aside.Params.Off(params.ThreadEdit) {
//line row.gox:86
			__e = __c.Any(r.aside.menu.BindControls(
			chatMenu{id: r.thread.ID, mode: chatMenuOpen},
			func(show func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:89
					__e = __c.Any(r.kebab(show)); if __e != nil { return }
				return })
//line row.gox:90
			},
			func(hide func(ctx context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:92
					__e = __c.Any(r.kebab(hide)); if __e != nil { return }
				return })
//line row.gox:93
			},
		)); if __e != nil { return }
		}
	return })
//line row.gox:96
}

//line row.gox:98
func (r *row) kebab(toggle func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:99
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
//line row.gox:106
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line row.gox:106
				__e = __c.Set("aria-label", "Chat actions"); if __e != nil { return }
//line row.gox:106
				__e = __c.Set("aria-haspopup", "menu"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:107
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:107
					__e = __c.Any(icons.DotsThreeVertical()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:109
}

//line row.gox:111
func (r *row) actions(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:112
			__e = __c.Set("class", "nechat-rowmenu nechat-popbody"); if __e != nil { return }
//line row.gox:112
			__e = __c.Set("role", "menu"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line row.gox:113
			__e = __c.Any(r.action(icons.Pencil(), "Rename", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuRename})
		})); if __e != nil { return }
//line row.gox:116
			__e = __c.Any(r.action(icons.Copy(), "Clone", func(ctx context.Context) {
			id, err := r.aside.Instance.ThreadClone(ctx, r.thread.ID)
			if err != nil {
				doors.Logger(ctx).Error("aside: clone", "thread", r.thread.ID, "err", err)
				notify.Error(ctx, err)
				hide(ctx)
				return
			}
			if err := r.aside.Instance.ThreadOpen(ctx, id, nil); err != nil {
				doors.Logger(ctx).Error("aside: open", "thread", id, "err", err)
				notify.Error(ctx, err)
			}
			hide(ctx)
		})); if __e != nil { return }
//line row.gox:130
			__e = __c.Any(r.action(icons.Trash(), "Delete chat", func(ctx context.Context) {
			r.aside.menu.Show(ctx, chatMenu{id: r.thread.ID, mode: chatMenuDelete})
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line row.gox:134
}

//line row.gox:136
func (r *row) action(icon gox.Elem, label string, on func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:137
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
//line row.gox:144
				__e = __c.Set("class", "nechat-act"); if __e != nil { return }
//line row.gox:144
				__e = __c.Set("role", "menuitem"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:145
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line row.gox:145
					__e = __c.Any(icon); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:146
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line row.gox:148
}

//line row.gox:150
func (r *row) rename(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:151
		__e = (doors.ASubmit[renameForm]{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rf doors.RequestForm[renameForm]) bool {
			n, err := chatmod.CheckName(rf.Data().Name)
			if err == nil && n != r.thread.Name {
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
//line row.gox:167
				__e = __c.Set("class", "nechat-form"); if __e != nil { return }
//line row.gox:167
				__e = __c.Set("aria-label", "Rename chat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line row.gox:168
					__e = __c.Set("class", "nechat-form-field"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:169
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
//line row.gox:178
							__e = __c.Set("class", "nechat-text"); if __e != nil { return }
//line row.gox:179
							__e = __c.Set("name", "name"); if __e != nil { return }
//line row.gox:180
							__e = __c.Set("value", r.thread.Name); if __e != nil { return }
//line row.gox:181
							__e = __c.Set("maxlength", "80"); if __e != nil { return }
							__e = __c.Set("autofocus", true); if __e != nil { return }
//line row.gox:183
							__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line row.gox:184
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
//line row.gox:191
					__e = __c.Set("class", "nechat-form-actions"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:192
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
//line row.gox:199
							__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:199
							__e = __c.Set("class", "nechat-btn nechat-btn_2nd"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Cancel"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line row.gox:200
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line row.gox:200
						__e = __c.Set("class", "nechat-btn"); if __e != nil { return }
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
//line row.gox:203
}

//line row.gox:205
func (r *row) confirm(hide func(ctx context.Context)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line row.gox:207
		label := r.thread.Name
	if label == "" {
		label = "Untitled"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line row.gox:212
			__e = __c.Set("class", "nechat-form"); if __e != nil { return }
//line row.gox:212
			__e = __c.Set("role", "group"); if __e != nil { return }
//line row.gox:212
			__e = __c.Set("aria-label", "Delete chat"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:213
				__e = __c.Set("class", "nechat-form-text"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Delete "); if __e != nil { return }
				__e = __c.Init("b"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line row.gox:213
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("?"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line row.gox:214
				__e = __c.Set("class", "nechat-form-actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line row.gox:215
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
//line row.gox:222
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:222
						__e = __c.Set("class", "nechat-btn nechat-btn_2nd"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Cancel"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line row.gox:223
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
//line row.gox:236
						__e = __c.Set("type", "button"); if __e != nil { return }
//line row.gox:236
						__e = __c.Set("class", "nechat-btn nechat-btn_del"); if __e != nil { return }
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
//line row.gox:239
}
