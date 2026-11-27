// Managed by GoX v0.4.0

//line composer.gox:1
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

package composer

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/composer/controller"
	"neui.dev/nekit/internal/chat/pops"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/chat/uploader"
	"neui.dev/nekit/internal/chat/notify"
)

type Composer struct {
	Controller controller.Controller
	Params params.Params
	Pops pops.Popup[string]
	scope *doors.ScopeFrame
}

func (c Composer) id() string {
	return c.Params.ID
}

//line composer.gox:45
func (c Composer) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:47
		c.scope = new(doors.ScopeFrame)

//line composer.gox:49
		__e = __c.Any(c.Controller.Input().Bind(func(s controller.InputState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:50
			__e = __c.Any(c.composer(s)); if __e != nil { return }
		return })
//line composer.gox:51
	})); if __e != nil { return }
	return })
//line composer.gox:52
}

//line composer.gox:54
func (c *Composer) composer(state controller.InputState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:56
		var up uploader.Uploader
	if state.Ask == nil {
		up = uploader.New(
			state.Input.Attachments,
			c.Controller.FS(),
			c.Controller,
			state.Seq,
			c.Params,
		)
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line composer.gox:67
			__e = __c.Set("id", "composer" + c.id()); if __e != nil { return }
//line composer.gox:67
			__e = __c.Set("class", "nechat-composer"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line composer.gox:68
			if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:69
				__e = __c.Any(up.List()); if __e != nil { return }
			}
//line composer.gox:71
			if state.Ask == nil {
//line composer.gox:72
				__e = __c.Any(c.textarea(state.Input)); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:74
					__e = __c.Set("class", "nechat-panel"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:75
					__e = __c.Any(state.Ask.Form); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line composer.gox:78
				__e = __c.Set("class", "nechat-barrow"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:79
					__e = __c.Set("class", "nechat-barrow-left"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:80
					if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:81
						__e = __c.Any(up.Form(c.Controller.Pending())); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:84
					__e = __c.Set("class", "nechat-barrow-center"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:85
					if state.Rewind {
//line composer.gox:86
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:87
						__e = __c.Any(c.rewind()); if __e != nil { return }
//line composer.gox:88
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
//line composer.gox:89
					} else if state.Ask != nil {
//line composer.gox:90
						if !c.Params.Off(params.Tasks) {
//line composer.gox:91
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:93
						__e = __c.Any(c.asking(state.Ask)); if __e != nil { return }
//line composer.gox:94
						if state.Ask.Error != nil {
//line composer.gox:95
							__e = __c.Any(state.Ask.Error.Bind(c.askError)); if __e != nil { return }
						}
					} else  {
//line composer.gox:98
						if c.Params.Off(params.Tokens) {
//line composer.gox:99
							__e = __c.Any(doors.DeriveBeam(c.Controller.Bar(), func(b controller.Bar) bool {
							return b.Compaction
						}).Bind(func(compacting bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:102
								if compacting {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:103
										__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:104
											__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Init("span"); if __e != nil { return }
											{
//line composer.gox:104
												__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
												__e = __c.Text("compacting"); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							return })
//line composer.gox:107
						})); if __e != nil { return }
						} else  {
//line composer.gox:109
							__e = __c.Any(c.Controller.HasContext().Bind(c.context)); if __e != nil { return }
						}
//line composer.gox:111
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:112
						if c.Params.Off(params.Tasks) {
//line composer.gox:113
							__e = __c.Any(c.Controller.Active().Bind(func(active bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:114
								if !active {
//line composer.gox:115
									__e = __c.Any(c.Controller.HasTasks().Bind(func(has bool) gox.Elem {
									return gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
//line composer.gox:116
										if has {
											__e = __c.Init("span"); if __e != nil { return }
											{
//line composer.gox:117
												__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
												__e = __c.Init("span"); if __e != nil { return }
												{
//line composer.gox:118
													__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
													__e = __c.Init("span"); if __e != nil { return }
													{
//line composer.gox:118
														__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
														__e = __c.Submit(); if __e != nil { return }
														__e = __c.Text("working"); if __e != nil { return }
													}
													__e = __c.Close(); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
									return })
//line composer.gox:121
								})); if __e != nil { return }
								}
							return })
//line composer.gox:123
						})); if __e != nil { return }
						} else  {
//line composer.gox:125
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:127
						__e = __c.Any(c.Controller.Pending().Bind(c.queued)); if __e != nil { return }
//line composer.gox:128
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:131
					__e = __c.Set("class", "nechat-barrow-right"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:132
					if state.Ask != nil {
//line composer.gox:133
						__e = __c.Any(state.Ask.Button); if __e != nil { return }
					} else  {
//line composer.gox:135
						__e = __c.Any(c.Controller.BindSend(c.send)); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const composer = document.currentScript.parentElement\n\t\t\tconst input = composer.querySelector(\".nechat-draft\")\n\t\t\tcomposer.addEventListener(\"click\", (e) => {\n\t\t\t\tif (e.target.closest(\"button, a, select, label\")) return\n\t\t\t\tinput?.focus()\n\t\t\t})\n\t\t\tcomposer.addEventListener(\"pointerdown\", (e) => {\n\t\t\t\tif (!e.target.closest(\".nechat-gauge_opens\")) return\n\t\t\t\te.preventDefault()\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:152
}

//line composer.gox:154
func (c *Composer) rewind() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:155
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:156
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:156
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:156
					__e = __c.Any(icons.Pencil()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:157
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:157
					__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("edit"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:158
				__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:159
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.CancelRewind(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:166
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:166
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:166
						__e = __c.Set("title", "Cancel edit"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:167
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:167
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:171
}

//line composer.gox:173
func (c *Composer) asking(ask *api.ChatAskView) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:174
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:175
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:175
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:175
					__e = __c.Any(icons.CheckSquareFill()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:176
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:176
					__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("asking"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:177
				__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:178
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					ask.Dismiss(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:185
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:185
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:185
						__e = __c.Set("title", "Dismiss question"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:186
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:186
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:190
}

//line composer.gox:192
func (c *Composer) askError(message string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:193
		if message != "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:194
				__e = __c.Set("class", "nechat-gauge nechat-gauge_err"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:195
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:195
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:195
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:196
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:197
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:197
						__e = __c.Any(message); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:201
}

//line composer.gox:203
func (c *Composer) activity(active bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:204
		if active {
//line composer.gox:205
			__e = __c.Any(c.running()); if __e != nil { return }
		}
	return })
//line composer.gox:207
}

//line composer.gox:209
func (c *Composer) context(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:211
		window := c.Controller.Model().Context
	config := c.Controller.Config()

//line composer.gox:214
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:215
				__e = __c.Set("class", "nechat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:216
				__e = __c.Any(c.Pops.BindControls("context",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:218
						__e = __c.Any(c.contextChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:219
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:221
						__e = __c.Any(c.contextChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:222
				},
			)); if __e != nil { return }
//line composer.gox:224
				__e = __c.Any(c.Pops.BindPop("context", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:225
						__e = __c.Set("class", "nechat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:226
						__e = __c.Any(c.Controller.Context().Bind(func(s controller.Context) gox.Elem {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:228
							settled := s.Usage.Total()

							__e = __c.Init("span"); if __e != nil { return }
							{
//line composer.gox:230
								__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:231
									__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:232
										__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Context"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line composer.gox:233
										__e = __c.Any(tokens(settled)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line composer.gox:234
									if window != 0 {
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:235
											__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:235
											__e = __c.Any(fmt.Sprintf("%s · %d%%", tokens(window), settled * 100 / window)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:238
								if window != 0 && config.CompactionTrigger <= 1 {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:239
										__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:240
											__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Text("Auto-compaction"); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:241
											__e = __c.Any(fmt.Sprintf("%.0f%%", config.CompactionTrigger * 100)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
//line composer.gox:245
								__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line composer.gox:246
								__e = __c.Any(stat("Input", s.Usage.Input)); if __e != nil { return }
//line composer.gox:247
								__e = __c.Any(stat("Cache read", s.Usage.CacheRead)); if __e != nil { return }
//line composer.gox:248
								__e = __c.Any(stat("Cache write", s.Usage.CacheWrite)); if __e != nil { return }
//line composer.gox:249
								__e = __c.Any(stat("Output", s.Usage.Output)); if __e != nil { return }
//line composer.gox:250
								if !c.Params.Off(params.Thinking) {
//line composer.gox:251
									__e = __c.Any(stat("Reasoning", s.Usage.Reasoning)); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
//line composer.gox:254
							if s.Compaction {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:255
									__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("button"); if __e != nil { return }
									{
//line composer.gox:255
										__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:255
										__e = __c.Set("class", "nechat-btn"); if __e != nil { return }
										__e = __c.Set("disabled", true); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("compacting…"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:256
							} else if s.CanCompact {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:257
									__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line composer.gox:258
									__e = (doors.AClick{
									Scope: new(doors.ScopeBlocking),
									On: func(ctx context.Context, rp doors.RequestPointer) bool {
										c.Controller.Compact(ctx)
										hide(ctx)
										return false
									},
									OnError: notify.OnError,
								}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
										__e = __c.Init("button"); if __e != nil { return }
										{
//line composer.gox:266
											__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:266
											__e = __c.Set("class", "nechat-btn"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Text("Compact now"); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									return })); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						return })
//line composer.gox:269
					})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })
//line composer.gox:271
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:274
}

//line composer.gox:276
func (c *Composer) contextChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:278
		window := c.Controller.Model().Context
	config := c.Controller.Config()
	class := "nechat-gauge nechat-gauge_opens"
	expanded := "false"
	if open {
		class = "nechat-gauge nechat-gauge_opens nechat-gauge_open"
		expanded = "true"
	}

//line composer.gox:287
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
//line composer.gox:294
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:294
				__e = __c.Set("class", class); if __e != nil { return }
//line composer.gox:294
				__e = __c.Set("title", "Context size"); if __e != nil { return }
//line composer.gox:294
				__e = __c.Set("aria-expanded", expanded); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:295
				if window != 0 {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:296
						__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:297
						__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("svg"); if __e != nil { return }
						{
//line composer.gox:298
							__e = __c.Set("class", "nechat-ring"); if __e != nil { return }
//line composer.gox:298
							__e = __c.Set("data-compacting", b.Compaction); if __e != nil { return }
//line composer.gox:298
							__e = __c.Set("viewBox", "0 0 16 16"); if __e != nil { return }
//line composer.gox:298
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:299
								__e = __c.Set("class", "nechat-ring-track"); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%.1f 100", min(config.CompactionTrigger, 1) * 100)); if __e != nil { return }
//line composer.gox:299
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:300
								__e = __c.Set("class", "nechat-ring-fill"); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%d 100", min((b.Settled + b.Unsettled) * 100 / window, 100))); if __e != nil { return }
//line composer.gox:300
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })
//line composer.gox:302
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:305
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:306
					__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:307
					if b.Compaction {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:308
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("compacting"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:310
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:310
							__e = __c.Any(tokens(b.Settled)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:312
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:315
}

//line composer.gox:317
func stat(label string, n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:318
		if n != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:319
				__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:320
					__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:320
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:321
					__e = __c.Any(tokens(n)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:324
}

//line composer.gox:326
func (c *Composer) generated(b controller.Bar) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:327
		if b.Unsettled != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:328
				__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
//line composer.gox:328
				__e = __c.Set("title", "Tokens generated"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:328
				__e = __c.Any(tokens(b.Unsettled)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:329
				__e = __c.Set("class", "nechat-gauge-sep"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("·"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:331
}

//line composer.gox:333
func (c *Composer) queued(pending bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:334
		if pending {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:335
				__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:336
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:336
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:336
						__e = __c.Any(icons.Clock()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:337
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:337
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("queued"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:338
					__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:339
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						c.Controller.Unsend(ctx)
						return false
					},
					OnError: notify.OnError,
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line composer.gox:346
							__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:346
							__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:346
							__e = __c.Set("title", "Cancel queued message"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:347
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:347
								__e = __c.Any(icons.Close()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:352
}

//line composer.gox:354
func (c *Composer) tasks(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:355
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:356
				__e = __c.Set("class", "nechat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:357
				__e = __c.Any(c.Pops.BindControls("tasks",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:359
						__e = __c.Any(c.tasksChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:360
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:362
						__e = __c.Any(c.tasksChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:363
				},
			)); if __e != nil { return }
//line composer.gox:365
				__e = __c.Any(c.Pops.BindPop("tasks", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:366
						__e = __c.Set("class", "nechat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:367
							__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:368
							__e = __c.Any(c.Controller.Tasks().Bind(c.taskRows)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:370
							__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:371
							__e = (doors.AClick{
							Scope: new(doors.ScopeBlocking),
							On: func(ctx context.Context, rp doors.RequestPointer) bool {
								c.Controller.CancelTasks()
								hide(ctx)
								return false
							},
							OnError: notify.OnError,
						}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line composer.gox:379
									__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:379
									__e = __c.Set("class", "nechat-btn"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Text("Cancel all"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })
//line composer.gox:382
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:385
}

//line composer.gox:387
func (c *Composer) tasksChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:389
		class := "nechat-gauge nechat-gauge_opens"
	expanded := "false"
	if open {
		class = "nechat-gauge nechat-gauge_opens nechat-gauge_open"
		expanded = "true"
	}

//line composer.gox:396
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
//line composer.gox:403
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:403
				__e = __c.Set("class", class); if __e != nil { return }
//line composer.gox:403
				__e = __c.Set("aria-expanded", expanded); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:404
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:404
					__e = (doors.Class("nechat-ic nechat-ic_line nechat-ic_beams")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:404
						__e = __c.Any(icons.Beams()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:405
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:406
					__e = __c.Any(c.Controller.TaskCount().Bind(func(n int) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:407
					if n == 1 {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:408
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("1 task"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:410
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:410
							__e = __c.Any(fmt.Sprintf("%d tasks", n)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:412
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:415
}

//line composer.gox:417
func (c *Composer) taskRows(tasks []api.Task) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:418
		for _, t := range tasks {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:419
				__e = __c.Set("class", "nechat-task"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:420
					__e = __c.Set("class", "nechat-task-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:421
					if t.Comment != "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:422
							__e = __c.Set("class", "nechat-task-comment"); if __e != nil { return }
//line composer.gox:422
							__e = __c.Set("title", t.Comment); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:422
							__e = __c.Any(t.Comment); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:424
							__e = __c.Set("class", "nechat-task-comment"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:424
							__e = __c.Any(t.Name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:426
						__e = __c.Set("class", "nechat-task-detail"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:426
						__e = __c.Any(t.Module); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line composer.gox:428
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.CancelTask(t.ID)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:435
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:435
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:435
						__e = __c.Set("title", "Cancel task"); if __e != nil { return }
//line composer.gox:435
						__e = __c.Set("aria-label", "Cancel " + t.Module + " task"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:436
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:436
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:440
}

//line composer.gox:442
func (c *Composer) running() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:444
		start := time.Now()

		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:446
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:447
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:448
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.Stop(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:455
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:455
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:455
						__e = __c.Set("title", "Stop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:456
						__e = (doors.Class("nechat-ic nechat-icon_breathe")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:456
							__e = __c.Any(icons.Stop()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:459
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:460
				if !c.Params.Off(params.Tokens) {
//line composer.gox:461
					__e = __c.Any(c.Controller.Bar().Bind(c.generated)); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:463
					__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:463
					__e = __c.Any(fmt.Sprintf("%.1fs", time.Since(start).Seconds())); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("script"); if __e != nil { return }
		{
//line composer.gox:466
			__e = __c.Set("data:start", start.UnixMilli()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const el =\n\t\t\tdocument.currentScript.previousElementSibling.querySelector(\n\t\t\t\t\".nechat-gauge-meta\",\n\t\t\t)\n\t\tconst start = $data(\"start\")\n\t\tconst t = setInterval(() => {\n\t\t\tel.textContent = ((Date.now() - start) / 1000).toFixed(1) + \"s\"\n\t\t}, 50)\n\t\t$sys.clean(() => clearInterval(t))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:477
}

//line composer.gox:479
func (c *Composer) overshoot(excess int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:480
		if excess > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:481
				__e = __c.Set("class", "nechat-gauge nechat-gauge_err"); if __e != nil { return }
//line composer.gox:481
				__e = __c.Set("title", "Message too long to send; text past the limit is not kept"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:482
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:482
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:482
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:483
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:484
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("too long"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:485
						__e = __c.Set("class", "nechat-gauge-sep"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("·"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:486
						__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:486
						__e = __c.Any(byteCount(excess) + " over"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:490
}

func byteCount(n int) string {
	digits := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i := range digits {
		if i > 0 && (len(digits) - i) % 3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(digits[i])
	}
	return b.String() + " bytes"
}

func tokens(n int) string {
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n) / 1000)
	}
	if n < 1000000 {
		return fmt.Sprintf("%dk", n / 1000)
	}
	return fmt.Sprintf("%.1fM", float64(n) / 1000000)
}

//line composer.gox:514
func (c *Composer) send(enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:515
		if enabled {
//line composer.gox:516
			__e = (doors.AClick{
			Scope: c.scope.Scope(true),
			Indicator: c.sendIndication(),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				c.Controller.Send(ctx)
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line composer.gox:524
					__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:524
					__e = __c.Set("id", "send" + c.id()); if __e != nil { return }
//line composer.gox:524
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line composer.gox:524
					__e = __c.Set("aria-label", "Send"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:525
					__e = (doors.Class("nechat-ic nechat-icon_idle")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:525
						__e = __c.Any(icons.Submit()); if __e != nil { return }
					return })); if __e != nil { return }
//line composer.gox:526
					__e = (doors.Class("nechat-ic nechat-icon_busy")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:526
						__e = __c.Any(icons.Spinner()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line composer.gox:529
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:529
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line composer.gox:529
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:530
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:530
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:533
}

//line composer.gox:535
func (c *Composer) textarea(input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:537
		s := new(doors.Setter)
	pending, _ := c.Controller.Pending().ReadAndSub(ctx, func(ctx context.Context, b bool) bool {
		value := "false"
		if b {
			value = "true"
		}
		doors.Call(ctx, s.Set("aria-busy", value))
		doors.Call(ctx, s.Set("readonly", b))
		return false
	})

//line composer.gox:548
		__e = (doors.AKeyDown{
		Key: doors.Key{
			Key: "Enter",
			ShiftMod: doors.ModOff,
		},
		PreventDefault: true,
		Scope: c.scope.Scope(true),
		Indicator: c.sendIndication(),
		On: func(ctx context.Context, rk doors.RequestKeyboard) bool {
			c.Controller.Send(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:561
			__e = (doors.AInput{
		Scope: c.scope.Scope(false).And(&doors.ScopeRate{
			Tick: 100 * time.Millisecond,
		}).And(&doors.ScopeSerial{}),
		On: func(ctx context.Context, ri doors.RequestInput) bool {
			value := strings.TrimSpace(ri.Event().Value)
			c.Controller.UpdateInputText(ctx, value)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line composer.gox:572
					__e = __c.Set("id", "text" + c.id()); if __e != nil { return }
//line composer.gox:573
					__e = __c.Set("class", "nechat-draft"); if __e != nil { return }
//line composer.gox:574
					__e = __c.Set("rows", "2"); if __e != nil { return }
//line composer.gox:575
					__e = __c.Set("aria-label", "Message"); if __e != nil { return }
//line composer.gox:576
					__e = __c.Modify(s); if __e != nil { return }
//line composer.gox:577
					__e = __c.Set("readonly", pending); if __e != nil { return }
//line composer.gox:578
					__e = __c.Set("aria-busy", func() any {
			if pending {
				return "true"
			}
			return "false"
		}()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:584
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
//line composer.gox:586
		__e = __c.Any(c.Controller.Pending().Bind(func(pending bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:587
			if !pending {
				__e = __c.Init("script"); if __e != nil { return }
				{
//line composer.gox:588
					__e = __c.Set("data:id", "text" + c.id()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const el = document.getElementById($data(\"id\"))\n\t\t\t\tel.focus()\n\t\t\t\tel.setSelectionRange(el.value.length, el.value.length)"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		return })
//line composer.gox:594
	})); if __e != nil { return }
	return })
//line composer.gox:595
}

func (c *Composer) sendIndication() doors.Indicators {
	return doors.JoinIndicators(
		doors.IndicateAttrQuery("#composer" + c.id(), "aria-busy", "true"),
		doors.IndicateAttrQuery("#send" + c.id(), "aria-busy", "true"),
	)
}
