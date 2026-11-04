// Managed by GoX v0.3.3

//line composer.gox:1
package composer

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/chat/composer/controller"
	"neui.dev/kchat/pops"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/params"
	"neui.dev/kchat/chat/uploader"
)

type Composer struct {
	Controller controller.ComposerController
	Params params.Params
	Pops pops.Popup[string]
	scope *doors.ScopeFrame
}

func (c Composer) id() string {
	return c.Params.ID
}

//line composer.gox:30
func (c Composer) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:32
		c.scope = new(doors.ScopeFrame)

//line composer.gox:34
		__e = __c.Any(c.Controller.Input().Bind(func(s controller.InputState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:35
			__e = __c.Any(c.composer(s)); if __e != nil { return }
		return })
//line composer.gox:36
	})); if __e != nil { return }
	return })
//line composer.gox:37
}

//line composer.gox:39
func (c *Composer) composer(state controller.InputState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:41
		var up uploader.Uploader
	if state.Ask == nil {
		up = uploader.NewUploader(
			state.Input.Attachments,
			c.Controller.FS(),
			c.Controller,
			state.Seq,
			c.Params,
		)
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line composer.gox:52
			__e = __c.Set("id", "composer" + c.id()); if __e != nil { return }
//line composer.gox:52
			__e = __c.Set("class", "kchat-composer"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line composer.gox:53
			if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:54
				__e = __c.Any(up.List()); if __e != nil { return }
			}
//line composer.gox:56
			if state.Ask == nil {
//line composer.gox:57
				__e = __c.Any(c.textarea(state.Input)); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:59
					__e = __c.Set("class", "kchat-panel"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:60
					__e = __c.Any(state.Ask.Form); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line composer.gox:63
				__e = __c.Set("class", "kchat-barrow"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:64
					__e = __c.Set("class", "kchat-barrow-left"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:65
					if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:66
						__e = __c.Any(up.Form(c.Controller.Pending())); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:69
					__e = __c.Set("class", "kchat-barrow-center"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:70
					if state.Rewind {
//line composer.gox:71
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:72
						__e = __c.Any(c.rewind()); if __e != nil { return }
//line composer.gox:73
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
//line composer.gox:74
					} else if state.Ask != nil {
//line composer.gox:75
						if !c.Params.Off(params.Tasks) {
//line composer.gox:76
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:78
						__e = __c.Any(c.asking(state.Ask)); if __e != nil { return }
//line composer.gox:79
						if state.Ask.Error != nil {
//line composer.gox:80
							__e = __c.Any(state.Ask.Error.Bind(c.askError)); if __e != nil { return }
						}
					} else  {
//line composer.gox:83
						if c.Params.Off(params.Tokens) {
//line composer.gox:84
							__e = __c.Any(doors.DeriveBeam(c.Controller.Bar(), func(b controller.Bar) bool {
							return b.Compaction
						}).Bind(func(compacting bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:87
								if compacting {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:88
										__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:89
											__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Init("span"); if __e != nil { return }
											{
//line composer.gox:89
												__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
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
//line composer.gox:92
						})); if __e != nil { return }
						} else  {
//line composer.gox:94
							__e = __c.Any(c.Controller.HasContext().Bind(c.context)); if __e != nil { return }
						}
//line composer.gox:96
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:97
						if c.Params.Off(params.Tasks) {
//line composer.gox:98
							__e = __c.Any(c.Controller.Active().Bind(func(active bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:99
								if !active {
//line composer.gox:100
									__e = __c.Any(c.Controller.HasTasks().Bind(func(has bool) gox.Elem {
									return gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
//line composer.gox:101
										if has {
											__e = __c.Init("span"); if __e != nil { return }
											{
//line composer.gox:102
												__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
												__e = __c.Init("span"); if __e != nil { return }
												{
//line composer.gox:103
													__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
													__e = __c.Init("span"); if __e != nil { return }
													{
//line composer.gox:103
														__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
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
//line composer.gox:106
								})); if __e != nil { return }
								}
							return })
//line composer.gox:108
						})); if __e != nil { return }
						} else  {
//line composer.gox:110
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:112
						__e = __c.Any(c.Controller.Pending().Bind(c.queued)); if __e != nil { return }
//line composer.gox:113
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:116
					__e = __c.Set("class", "kchat-barrow-right"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:117
					if state.Ask != nil {
//line composer.gox:118
						__e = __c.Any(state.Ask.Button); if __e != nil { return }
					} else  {
//line composer.gox:120
						__e = __c.Any(c.Controller.BindSend(c.send)); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const composer = document.currentScript.parentElement\n\t\t\tconst input = composer.querySelector(\".kchat-draft\")\n\t\t\tcomposer.addEventListener(\"click\", (e) => {\n\t\t\t\tif (e.target.closest(\"button, a, select, label\")) return\n\t\t\t\tinput?.focus()\n\t\t\t})\n\t\t\tcomposer.addEventListener(\"pointerdown\", (e) => {\n\t\t\t\tif (!e.target.closest(\".kchat-gauge_opens\")) return\n\t\t\t\te.preventDefault()\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:137
}

//line composer.gox:139
func (c *Composer) rewind() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:140
			__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:141
				__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:141
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:141
					__e = __c.Any(icons.Pencil()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:142
				__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:142
					__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("edit"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:143
				__e = __c.Set("class", "kchat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:144
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.CancelRewind(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:150
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:150
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line composer.gox:150
						__e = __c.Set("title", "Cancel edit"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:151
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:151
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
//line composer.gox:155
}

//line composer.gox:157
func (c *Composer) asking(ask *api.ChatAsk) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:158
			__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:159
				__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:159
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:159
					__e = __c.Any(icons.CheckSquareFill()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:160
				__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:160
					__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("asking"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:161
				__e = __c.Set("class", "kchat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:162
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					ask.Cancel(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:168
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:168
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line composer.gox:168
						__e = __c.Set("title", "Dismiss question"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:169
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:169
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
//line composer.gox:173
}

//line composer.gox:175
func (c *Composer) askError(message string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:176
		if message != "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:177
				__e = __c.Set("class", "kchat-gauge kchat-gauge_err"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:178
					__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:178
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:178
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:179
					__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:180
						__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:180
						__e = __c.Any(message); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:184
}

//line composer.gox:186
func (c *Composer) activity(active bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:187
		if active {
//line composer.gox:188
			__e = __c.Any(c.running()); if __e != nil { return }
		}
	return })
//line composer.gox:190
}

//line composer.gox:192
func (c *Composer) context(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:194
		window := c.Controller.Model().Context
	config := c.Controller.Config()

//line composer.gox:197
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:198
				__e = __c.Set("class", "kchat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:199
				__e = __c.Any(c.Pops.BindControls("context",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:201
						__e = __c.Any(c.contextChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:202
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:204
						__e = __c.Any(c.contextChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:205
				},
			)); if __e != nil { return }
//line composer.gox:207
				__e = __c.Any(c.Pops.BindPop("context", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:208
						__e = __c.Set("class", "kchat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:209
						__e = __c.Any(c.Controller.Context().Bind(func(s controller.Context) gox.Elem {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:211
							settled := s.Usage.Total()

							__e = __c.Init("span"); if __e != nil { return }
							{
//line composer.gox:213
								__e = __c.Set("class", "kchat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:214
									__e = __c.Set("class", "kchat-pop-stat"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:215
										__e = __c.Set("class", "kchat-pop-label"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Context"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line composer.gox:216
										__e = __c.Any(tokens(settled)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line composer.gox:217
									if window != 0 {
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:218
											__e = __c.Set("class", "kchat-gauge-meta"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:218
											__e = __c.Any(fmt.Sprintf("%s · %d%%", tokens(window), settled * 100 / window)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:221
								if window != 0 && config.CompactionTrigger <= 1 {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:222
										__e = __c.Set("class", "kchat-pop-stat"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:223
											__e = __c.Set("class", "kchat-pop-label"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Text("Auto-compaction"); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:224
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
//line composer.gox:228
								__e = __c.Set("class", "kchat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line composer.gox:229
								__e = __c.Any(stat("Input", s.Usage.Input)); if __e != nil { return }
//line composer.gox:230
								__e = __c.Any(stat("Cache read", s.Usage.CacheRead)); if __e != nil { return }
//line composer.gox:231
								__e = __c.Any(stat("Cache write", s.Usage.CacheWrite)); if __e != nil { return }
//line composer.gox:232
								__e = __c.Any(stat("Output", s.Usage.Output)); if __e != nil { return }
//line composer.gox:233
								if !c.Params.Off(params.Thinking) {
//line composer.gox:234
									__e = __c.Any(stat("Reasoning", s.Usage.Reasoning)); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
//line composer.gox:237
							if s.Compaction {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:238
									__e = __c.Set("class", "kchat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("button"); if __e != nil { return }
									{
//line composer.gox:238
										__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:238
										__e = __c.Set("class", "kchat-btn"); if __e != nil { return }
										__e = __c.Set("disabled", true); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("compacting…"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:239
							} else if window == 0 || float32(settled) >= config.CompactionAllowed * float32(window) {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:240
									__e = __c.Set("class", "kchat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line composer.gox:241
									__e = (doors.AClick{
									Scope: new(doors.ScopeBlocking),
									On: func(ctx context.Context, rp doors.RequestPointer) bool {
										c.Controller.Compact(ctx)
										hide(ctx)
										return false
									},
								}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
										__e = __c.Init("button"); if __e != nil { return }
										{
//line composer.gox:248
											__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:248
											__e = __c.Set("class", "kchat-btn"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Text("Compact now"); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									return })); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						return })
//line composer.gox:251
					})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })
//line composer.gox:253
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:256
}

//line composer.gox:258
func (c *Composer) contextChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:260
		window := c.Controller.Model().Context
	config := c.Controller.Config()
	class := "kchat-gauge kchat-gauge_opens"
	if open {
		class = "kchat-gauge kchat-gauge_opens kchat-gauge_open"
	}

//line composer.gox:267
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
//line composer.gox:273
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:273
				__e = __c.Set("class", class); if __e != nil { return }
//line composer.gox:273
				__e = __c.Set("title", "Context size"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:274
				if window != 0 {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:275
						__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:276
						__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("svg"); if __e != nil { return }
						{
//line composer.gox:277
							__e = __c.Set("class", "kchat-ring"); if __e != nil { return }
//line composer.gox:277
							__e = __c.Set("data-compacting", b.Compaction); if __e != nil { return }
//line composer.gox:277
							__e = __c.Set("viewBox", "0 0 16 16"); if __e != nil { return }
//line composer.gox:277
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:278
								__e = __c.Set("class", "kchat-ring-track"); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%.1f 100", min(config.CompactionTrigger, 1) * 100)); if __e != nil { return }
//line composer.gox:278
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:279
								__e = __c.Set("class", "kchat-ring-fill"); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%d 100", min((b.Settled + b.Unsettled) * 100 / window, 100))); if __e != nil { return }
//line composer.gox:279
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })
//line composer.gox:281
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:284
					__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:285
					__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:286
					if b.Compaction {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:287
							__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("compacting"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:289
							__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:289
							__e = __c.Any(tokens(b.Settled)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:291
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:294
}

//line composer.gox:296
func stat(label string, n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:297
		if n != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:298
				__e = __c.Set("class", "kchat-pop-stat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:299
					__e = __c.Set("class", "kchat-pop-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:299
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:300
					__e = __c.Any(tokens(n)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:303
}

//line composer.gox:305
func (c *Composer) generated(b controller.Bar) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:306
		if b.Unsettled != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:307
				__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
//line composer.gox:307
				__e = __c.Set("title", "Tokens generated"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:307
				__e = __c.Any(tokens(b.Unsettled)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:308
				__e = __c.Set("class", "kchat-gauge-sep"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("·"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:310
}

//line composer.gox:312
func (c *Composer) queued(pending bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:313
		if pending {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:314
				__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:315
					__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:315
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:315
						__e = __c.Any(icons.Clock()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:316
					__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:316
						__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("queued"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:317
					__e = __c.Set("class", "kchat-gauge-trail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:318
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						c.Controller.Unsend(ctx)
						return false
					},
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line composer.gox:324
							__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:324
							__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line composer.gox:324
							__e = __c.Set("title", "Cancel queued message"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:325
							__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:325
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
//line composer.gox:330
}

//line composer.gox:332
func (c *Composer) tasks(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:333
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:334
				__e = __c.Set("class", "kchat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:335
				__e = __c.Any(c.Pops.BindControls("tasks",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:337
						__e = __c.Any(c.tasksChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:338
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:340
						__e = __c.Any(c.tasksChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:341
				},
			)); if __e != nil { return }
//line composer.gox:343
				__e = __c.Any(c.Pops.BindPop("tasks", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:344
						__e = __c.Set("class", "kchat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:345
							__e = __c.Set("class", "kchat-pop-group"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:346
							__e = __c.Any(c.Controller.Tasks().Bind(c.taskRows)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:348
							__e = __c.Set("class", "kchat-pop-foot"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:349
							__e = (doors.AClick{
							Scope: new(doors.ScopeBlocking),
							On: func(ctx context.Context, rp doors.RequestPointer) bool {
								c.Controller.CancelTasks()
								hide(ctx)
								return false
							},
						}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line composer.gox:356
									__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:356
									__e = __c.Set("class", "kchat-btn"); if __e != nil { return }
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
//line composer.gox:359
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:362
}

//line composer.gox:364
func (c *Composer) tasksChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:366
		class := "kchat-gauge kchat-gauge_opens"
	if open {
		class = "kchat-gauge kchat-gauge_opens kchat-gauge_open"
	}

//line composer.gox:371
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
//line composer.gox:377
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:377
				__e = __c.Set("class", class); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:378
					__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:378
					__e = (doors.Class("kchat-ic kchat-ic_line kchat-ic_beams")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:378
						__e = __c.Any(icons.Beams()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:379
					__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:380
					__e = __c.Any(c.Controller.TaskCount().Bind(func(n int) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:381
					if n == 1 {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:382
							__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("1 task"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:384
							__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:384
							__e = __c.Any(fmt.Sprintf("%d tasks", n)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:386
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:389
}

//line composer.gox:391
func (c *Composer) taskRows(tasks []api.Task) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:392
		for _, t := range tasks {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:393
				__e = __c.Set("class", "kchat-task"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:394
					__e = __c.Set("class", "kchat-task-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:395
					if t.Comment != "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:396
							__e = __c.Set("class", "kchat-task-comment"); if __e != nil { return }
//line composer.gox:396
							__e = __c.Set("title", t.Comment); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:396
							__e = __c.Any(t.Comment); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:398
							__e = __c.Set("class", "kchat-task-comment"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:398
							__e = __c.Any(t.Name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:400
						__e = __c.Set("class", "kchat-task-detail"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:400
						__e = __c.Any(t.Module); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line composer.gox:402
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.CancelTask(t.ID)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:408
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:408
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line composer.gox:408
						__e = __c.Set("title", "Cancel task"); if __e != nil { return }
//line composer.gox:408
						__e = __c.Set("aria-label", "Cancel " + t.Module + " task"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:409
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:409
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:413
}

//line composer.gox:415
func (c *Composer) running() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:417
		start := time.Now()

		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:419
			__e = __c.Set("class", "kchat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:420
				__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:421
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					c.Controller.Stop(ctx)
					return false
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:427
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:427
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line composer.gox:427
						__e = __c.Set("title", "Stop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:428
						__e = (doors.Class("kchat-ic kchat-icon_breathe")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:428
							__e = __c.Any(icons.Stop()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:431
				__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:432
				if !c.Params.Off(params.Tokens) {
//line composer.gox:433
					__e = __c.Any(c.Controller.Bar().Bind(c.generated)); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:435
					__e = __c.Set("class", "kchat-gauge-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:435
					__e = __c.Any(fmt.Sprintf("%.1fs", time.Since(start).Seconds())); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("script"); if __e != nil { return }
		{
//line composer.gox:438
			__e = __c.Set("data:start", start.UnixMilli()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const el =\n\t\t\tdocument.currentScript.previousElementSibling.querySelector(\n\t\t\t\t\".kchat-gauge-meta\",\n\t\t\t)\n\t\tconst start = $data(\"start\")\n\t\tconst t = setInterval(() => {\n\t\t\tel.textContent = ((Date.now() - start) / 1000).toFixed(1) + \"s\"\n\t\t}, 50)\n\t\t$sys.clean(() => clearInterval(t))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:449
}

//line composer.gox:451
func (c *Composer) overshoot(excess int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:452
		if excess > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:453
				__e = __c.Set("class", "kchat-gauge kchat-gauge_err"); if __e != nil { return }
//line composer.gox:453
				__e = __c.Set("title", "Message too long to send; text past the limit is not kept"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:454
					__e = __c.Set("class", "kchat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:454
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:454
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:455
					__e = __c.Set("class", "kchat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:456
						__e = __c.Set("class", "kchat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("too long"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:457
						__e = __c.Set("class", "kchat-gauge-sep"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("·"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:458
						__e = __c.Set("class", "kchat-gauge-meta"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:458
						__e = __c.Any(byteCount(excess) + " over"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:462
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

//line composer.gox:486
func (c *Composer) send(enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:487
		if enabled {
//line composer.gox:488
			__e = (doors.AClick{
			Scope: c.scope.Scope(true),
			Indicator: c.sendIndication(),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				c.Controller.Send(ctx)
				return false
			},
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line composer.gox:495
					__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:495
					__e = __c.Set("id", "send" + c.id()); if __e != nil { return }
//line composer.gox:495
					__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line composer.gox:495
					__e = __c.Set("aria-label", "Send"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:496
					__e = (doors.Class("kchat-ic kchat-icon_idle")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:496
						__e = __c.Any(icons.Submit()); if __e != nil { return }
					return })); if __e != nil { return }
//line composer.gox:497
					__e = (doors.Class("kchat-ic kchat-icon_busy")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:497
						__e = __c.Any(icons.Spinner()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line composer.gox:500
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:500
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line composer.gox:500
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:501
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:501
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:504
}

//line composer.gox:506
func (c *Composer) textarea(input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:508
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

//line composer.gox:519
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
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:531
			__e = (doors.AInput{
		Scope: c.scope.Scope(false).And(&doors.ScopeRate{
			Tick: 100 * time.Millisecond,
		}).And(&doors.ScopeSerial{}),
		On: func(ctx context.Context, ri doors.RequestInput) bool {
			value := strings.TrimSpace(ri.Event().Value)
			c.Controller.UpdateInputText(ctx, value)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line composer.gox:541
					__e = __c.Set("id", "text" + c.id()); if __e != nil { return }
//line composer.gox:542
					__e = __c.Set("class", "kchat-draft"); if __e != nil { return }
//line composer.gox:543
					__e = __c.Set("rows", "2"); if __e != nil { return }
//line composer.gox:544
					__e = __c.Modify(s); if __e != nil { return }
//line composer.gox:545
					__e = __c.Set("readonly", pending); if __e != nil { return }
//line composer.gox:546
					__e = __c.Set("aria-busy", func() any {
			if pending {
				return "true"
			}
			return "false"
		}()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:552
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
//line composer.gox:554
		__e = __c.Any(c.Controller.Pending().Bind(func(pending bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:555
			if !pending {
				__e = __c.Init("script"); if __e != nil { return }
				{
//line composer.gox:556
					__e = __c.Set("data:id", "text" + c.id()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const el = document.getElementById($data(\"id\"))\n\t\t\t\tel.focus()\n\t\t\t\tel.setSelectionRange(el.value.length, el.value.length)"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		return })
//line composer.gox:562
	})); if __e != nil { return }
	return })
//line composer.gox:563
}

func (c *Composer) sendIndication() doors.Indicators {
	return doors.JoinIndicators(
		doors.IndicateAttrQuery("#composer" + c.id(), "aria-busy", "true"),
		doors.IndicateAttrQuery("#send" + c.id(), "aria-busy", "true"),
	)
}
