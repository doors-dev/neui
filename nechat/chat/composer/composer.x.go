// Managed by GoX v0.3.4

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
	"neui.dev/nechat/chat/composer/controller"
	"neui.dev/nechat/pops"
	"neui.dev/nechat/icons"
	"neui.dev/nechat/params"
	"neui.dev/nechat/chat/uploader"
	"neui.dev/nechat/notify"
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

//line composer.gox:31
func (c Composer) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:33
		c.scope = new(doors.ScopeFrame)

//line composer.gox:35
		__e = __c.Any(c.Controller.Input().Bind(func(s controller.InputState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:36
			__e = __c.Any(c.composer(s)); if __e != nil { return }
		return })
//line composer.gox:37
	})); if __e != nil { return }
	return })
//line composer.gox:38
}

//line composer.gox:40
func (c *Composer) composer(state controller.InputState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:42
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
//line composer.gox:53
			__e = __c.Set("id", "composer" + c.id()); if __e != nil { return }
//line composer.gox:53
			__e = __c.Set("class", "nechat-composer"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line composer.gox:54
			if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:55
				__e = __c.Any(up.List()); if __e != nil { return }
			}
//line composer.gox:57
			if state.Ask == nil {
//line composer.gox:58
				__e = __c.Any(c.textarea(state.Input)); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:60
					__e = __c.Set("class", "nechat-panel"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:61
					__e = __c.Any(state.Ask.Form); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line composer.gox:64
				__e = __c.Set("class", "nechat-barrow"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:65
					__e = __c.Set("class", "nechat-barrow-left"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:66
					if state.Ask == nil && !c.Params.Off(params.Attachments) {
//line composer.gox:67
						__e = __c.Any(up.Form(c.Controller.Pending())); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:70
					__e = __c.Set("class", "nechat-barrow-center"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:71
					if state.Rewind {
//line composer.gox:72
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:73
						__e = __c.Any(c.rewind()); if __e != nil { return }
//line composer.gox:74
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
//line composer.gox:75
					} else if state.Ask != nil {
//line composer.gox:76
						if !c.Params.Off(params.Tasks) {
//line composer.gox:77
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:79
						__e = __c.Any(c.asking(state.Ask)); if __e != nil { return }
//line composer.gox:80
						if state.Ask.Error != nil {
//line composer.gox:81
							__e = __c.Any(state.Ask.Error.Bind(c.askError)); if __e != nil { return }
						}
					} else  {
//line composer.gox:84
						if c.Params.Off(params.Tokens) {
//line composer.gox:85
							__e = __c.Any(doors.DeriveBeam(c.Controller.Bar(), func(b controller.Bar) bool {
							return b.Compaction
						}).Bind(func(compacting bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:88
								if compacting {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:89
										__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:90
											__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Init("span"); if __e != nil { return }
											{
//line composer.gox:90
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
//line composer.gox:93
						})); if __e != nil { return }
						} else  {
//line composer.gox:95
							__e = __c.Any(c.Controller.HasContext().Bind(c.context)); if __e != nil { return }
						}
//line composer.gox:97
						__e = __c.Any(c.Controller.Active().Bind(c.activity)); if __e != nil { return }
//line composer.gox:98
						if c.Params.Off(params.Tasks) {
//line composer.gox:99
							__e = __c.Any(c.Controller.Active().Bind(func(active bool) gox.Elem {
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:100
								if !active {
//line composer.gox:101
									__e = __c.Any(c.Controller.HasTasks().Bind(func(has bool) gox.Elem {
									return gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
//line composer.gox:102
										if has {
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
														__e = __c.Text("working"); if __e != nil { return }
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
								}
							return })
//line composer.gox:109
						})); if __e != nil { return }
						} else  {
//line composer.gox:111
							__e = __c.Any(c.Controller.HasTasks().Bind(c.tasks)); if __e != nil { return }
						}
//line composer.gox:113
						__e = __c.Any(c.Controller.Pending().Bind(c.queued)); if __e != nil { return }
//line composer.gox:114
						__e = __c.Any(c.Controller.ExcessBytes().Bind(c.overshoot)); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line composer.gox:117
					__e = __c.Set("class", "nechat-barrow-right"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:118
					if state.Ask != nil {
//line composer.gox:119
						__e = __c.Any(state.Ask.Button); if __e != nil { return }
					} else  {
//line composer.gox:121
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
//line composer.gox:138
}

//line composer.gox:140
func (c *Composer) rewind() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:141
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:142
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:142
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:142
					__e = __c.Any(icons.Pencil()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:143
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:143
					__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("edit"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:144
				__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:145
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
//line composer.gox:152
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:152
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:152
						__e = __c.Set("title", "Cancel edit"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:153
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:153
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
//line composer.gox:157
}

//line composer.gox:159
func (c *Composer) asking(ask *api.ChatAsk) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:160
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:161
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:161
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:161
					__e = __c.Any(icons.CheckSquareFill()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:162
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:162
					__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("asking"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:163
				__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:164
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					ask.Cancel(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line composer.gox:171
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:171
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:171
						__e = __c.Set("title", "Dismiss question"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:172
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:172
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
//line composer.gox:176
}

//line composer.gox:178
func (c *Composer) askError(message string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:179
		if message != "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:180
				__e = __c.Set("class", "nechat-gauge nechat-gauge_err"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:181
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:181
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:181
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:182
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:183
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:183
						__e = __c.Any(message); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:187
}

//line composer.gox:189
func (c *Composer) activity(active bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:190
		if active {
//line composer.gox:191
			__e = __c.Any(c.running()); if __e != nil { return }
		}
	return })
//line composer.gox:193
}

//line composer.gox:195
func (c *Composer) context(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:197
		window := c.Controller.Model().Context
	config := c.Controller.Config()

//line composer.gox:200
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:201
				__e = __c.Set("class", "nechat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:202
				__e = __c.Any(c.Pops.BindControls("context",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:204
						__e = __c.Any(c.contextChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:205
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:207
						__e = __c.Any(c.contextChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:208
				},
			)); if __e != nil { return }
//line composer.gox:210
				__e = __c.Any(c.Pops.BindPop("context", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:211
						__e = __c.Set("class", "nechat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:212
						__e = __c.Any(c.Controller.Context().Bind(func(s controller.Context) gox.Elem {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:214
							settled := s.Usage.Total()

							__e = __c.Init("span"); if __e != nil { return }
							{
//line composer.gox:216
								__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:217
									__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:218
										__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Context"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line composer.gox:219
										__e = __c.Any(tokens(settled)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line composer.gox:220
									if window != 0 {
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:221
											__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:221
											__e = __c.Any(fmt.Sprintf("%s · %d%%", tokens(window), settled * 100 / window)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:224
								if window != 0 && config.CompactionTrigger <= 1 {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line composer.gox:225
										__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
//line composer.gox:226
											__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Text("Auto-compaction"); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("span"); if __e != nil { return }
										{
											__e = __c.Submit(); if __e != nil { return }
//line composer.gox:227
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
//line composer.gox:231
								__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line composer.gox:232
								__e = __c.Any(stat("Input", s.Usage.Input)); if __e != nil { return }
//line composer.gox:233
								__e = __c.Any(stat("Cache read", s.Usage.CacheRead)); if __e != nil { return }
//line composer.gox:234
								__e = __c.Any(stat("Cache write", s.Usage.CacheWrite)); if __e != nil { return }
//line composer.gox:235
								__e = __c.Any(stat("Output", s.Usage.Output)); if __e != nil { return }
//line composer.gox:236
								if !c.Params.Off(params.Thinking) {
//line composer.gox:237
									__e = __c.Any(stat("Reasoning", s.Usage.Reasoning)); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
//line composer.gox:240
							if s.Compaction {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:241
									__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("button"); if __e != nil { return }
									{
//line composer.gox:241
										__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:241
										__e = __c.Set("class", "nechat-btn"); if __e != nil { return }
										__e = __c.Set("disabled", true); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("compacting…"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line composer.gox:242
							} else if s.CanCompact {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line composer.gox:243
									__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line composer.gox:244
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
//line composer.gox:252
											__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:252
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
//line composer.gox:255
					})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })
//line composer.gox:257
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:260
}

//line composer.gox:262
func (c *Composer) contextChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:264
		window := c.Controller.Model().Context
	config := c.Controller.Config()
	class := "nechat-gauge nechat-gauge_opens"
	if open {
		class = "nechat-gauge nechat-gauge_opens nechat-gauge_open"
	}

//line composer.gox:271
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
//line composer.gox:278
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:278
				__e = __c.Set("class", class); if __e != nil { return }
//line composer.gox:278
				__e = __c.Set("title", "Context size"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:279
				if window != 0 {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:280
						__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:281
						__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("svg"); if __e != nil { return }
						{
//line composer.gox:282
							__e = __c.Set("class", "nechat-ring"); if __e != nil { return }
//line composer.gox:282
							__e = __c.Set("data-compacting", b.Compaction); if __e != nil { return }
//line composer.gox:282
							__e = __c.Set("viewBox", "0 0 16 16"); if __e != nil { return }
//line composer.gox:282
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:283
								__e = __c.Set("class", "nechat-ring-track"); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%.1f 100", min(config.CompactionTrigger, 1) * 100)); if __e != nil { return }
//line composer.gox:283
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("circle"); if __e != nil { return }
							{
//line composer.gox:284
								__e = __c.Set("class", "nechat-ring-fill"); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("cx", "8"); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("cy", "8"); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("r", "6"); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("pathLength", "100"); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("stroke-dasharray", fmt.Sprintf("%d 100", min((b.Settled + b.Unsettled) * 100 / window, 100))); if __e != nil { return }
//line composer.gox:284
								__e = __c.Set("transform", "rotate(-90 8 8)"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })
//line composer.gox:286
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:289
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:290
					__e = __c.Any(c.Controller.Bar().Bind(func(b controller.Bar) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:291
					if b.Compaction {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:292
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("compacting"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:294
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:294
							__e = __c.Any(tokens(b.Settled)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:296
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:299
}

//line composer.gox:301
func stat(label string, n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:302
		if n != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:303
				__e = __c.Set("class", "nechat-pop-stat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:304
					__e = __c.Set("class", "nechat-pop-label"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:304
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:305
					__e = __c.Any(tokens(n)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:308
}

//line composer.gox:310
func (c *Composer) generated(b controller.Bar) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:311
		if b.Unsettled != 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:312
				__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
//line composer.gox:312
				__e = __c.Set("title", "Tokens generated"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:312
				__e = __c.Any(tokens(b.Unsettled)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:313
				__e = __c.Set("class", "nechat-gauge-sep"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("·"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:315
}

//line composer.gox:317
func (c *Composer) queued(pending bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:318
		if pending {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:319
				__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:320
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:320
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:320
						__e = __c.Any(icons.Clock()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:321
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:321
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("queued"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:322
					__e = __c.Set("class", "nechat-gauge-trail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:323
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
//line composer.gox:330
							__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:330
							__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:330
							__e = __c.Set("title", "Cancel queued message"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:331
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line composer.gox:331
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
//line composer.gox:336
}

//line composer.gox:338
func (c *Composer) tasks(has bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:339
		if has {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:340
				__e = __c.Set("class", "nechat-pops"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:341
				__e = __c.Any(c.Pops.BindControls("tasks",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:343
						__e = __c.Any(c.tasksChip(show, false)); if __e != nil { return }
					return })
//line composer.gox:344
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:346
						__e = __c.Any(c.tasksChip(hide, true)); if __e != nil { return }
					return })
//line composer.gox:347
				},
			)); if __e != nil { return }
//line composer.gox:349
				__e = __c.Any(c.Pops.BindPop("tasks", func(hide func(context.Context)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:350
						__e = __c.Set("class", "nechat-pop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:351
							__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:352
							__e = __c.Any(c.Controller.Tasks().Bind(c.taskRows)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:354
							__e = __c.Set("class", "nechat-pop-foot"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:355
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
//line composer.gox:363
									__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:363
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
//line composer.gox:366
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:369
}

//line composer.gox:371
func (c *Composer) tasksChip(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:373
		class := "nechat-gauge nechat-gauge_opens"
	if open {
		class = "nechat-gauge nechat-gauge_opens nechat-gauge_open"
	}

//line composer.gox:378
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
//line composer.gox:385
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:385
				__e = __c.Set("class", class); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:386
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:386
					__e = (doors.Class("nechat-ic nechat-ic_line nechat-ic_beams")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:386
						__e = __c.Any(icons.Beams()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:387
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:388
					__e = __c.Any(c.Controller.TaskCount().Bind(func(n int) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:389
					if n == 1 {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:390
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("1 task"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:392
							__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:392
							__e = __c.Any(fmt.Sprintf("%d tasks", n)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line composer.gox:394
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line composer.gox:397
}

//line composer.gox:399
func (c *Composer) taskRows(tasks []api.Task) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:400
		for _, t := range tasks {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:401
				__e = __c.Set("class", "nechat-task"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:402
					__e = __c.Set("class", "nechat-task-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:403
					if t.Comment != "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:404
							__e = __c.Set("class", "nechat-task-comment"); if __e != nil { return }
//line composer.gox:404
							__e = __c.Set("title", t.Comment); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:404
							__e = __c.Any(t.Comment); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line composer.gox:406
							__e = __c.Set("class", "nechat-task-comment"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line composer.gox:406
							__e = __c.Any(t.Name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:408
						__e = __c.Set("class", "nechat-task-detail"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:408
						__e = __c.Any(t.Module); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line composer.gox:410
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
//line composer.gox:417
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:417
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:417
						__e = __c.Set("title", "Cancel task"); if __e != nil { return }
//line composer.gox:417
						__e = __c.Set("aria-label", "Cancel " + t.Module + " task"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:418
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:418
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:422
}

//line composer.gox:424
func (c *Composer) running() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:426
		start := time.Now()

		__e = __c.Init("span"); if __e != nil { return }
		{
//line composer.gox:428
			__e = __c.Set("class", "nechat-gauge"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:429
				__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:430
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
//line composer.gox:437
						__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:437
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line composer.gox:437
						__e = __c.Set("title", "Stop"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:438
						__e = (doors.Class("nechat-ic nechat-icon_breathe")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line composer.gox:438
							__e = __c.Any(icons.Stop()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:441
				__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:442
				if !c.Params.Off(params.Tokens) {
//line composer.gox:443
					__e = __c.Any(c.Controller.Bar().Bind(c.generated)); if __e != nil { return }
				}
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:445
					__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:445
					__e = __c.Any(fmt.Sprintf("%.1fs", time.Since(start).Seconds())); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("script"); if __e != nil { return }
		{
//line composer.gox:448
			__e = __c.Set("data:start", start.UnixMilli()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const el =\n\t\t\tdocument.currentScript.previousElementSibling.querySelector(\n\t\t\t\t\".nechat-gauge-meta\",\n\t\t\t)\n\t\tconst start = $data(\"start\")\n\t\tconst t = setInterval(() => {\n\t\t\tel.textContent = ((Date.now() - start) / 1000).toFixed(1) + \"s\"\n\t\t}, 50)\n\t\t$sys.clean(() => clearInterval(t))"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line composer.gox:459
}

//line composer.gox:461
func (c *Composer) overshoot(excess int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:462
		if excess > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line composer.gox:463
				__e = __c.Set("class", "nechat-gauge nechat-gauge_err"); if __e != nil { return }
//line composer.gox:463
				__e = __c.Set("title", "Message too long to send; text past the limit is not kept"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:464
					__e = __c.Set("class", "nechat-gauge-lead"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:464
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:464
						__e = __c.Any(icons.WarningFill()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line composer.gox:465
					__e = __c.Set("class", "nechat-gauge-body"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:466
						__e = __c.Set("class", "nechat-gauge-val"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("too long"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:467
						__e = __c.Set("class", "nechat-gauge-sep"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("·"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line composer.gox:468
						__e = __c.Set("class", "nechat-gauge-meta"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line composer.gox:468
						__e = __c.Any(byteCount(excess) + " over"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:472
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

//line composer.gox:496
func (c *Composer) send(enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:497
		if enabled {
//line composer.gox:498
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
//line composer.gox:506
					__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:506
					__e = __c.Set("id", "send" + c.id()); if __e != nil { return }
//line composer.gox:506
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line composer.gox:506
					__e = __c.Set("aria-label", "Send"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:507
					__e = (doors.Class("nechat-ic nechat-icon_idle")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:507
						__e = __c.Any(icons.Submit()); if __e != nil { return }
					return })); if __e != nil { return }
//line composer.gox:508
					__e = (doors.Class("nechat-ic nechat-icon_busy")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line composer.gox:508
						__e = __c.Any(icons.Spinner()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line composer.gox:511
				__e = __c.Set("type", "button"); if __e != nil { return }
//line composer.gox:511
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line composer.gox:511
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line composer.gox:512
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line composer.gox:512
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line composer.gox:515
}

//line composer.gox:517
func (c *Composer) textarea(input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line composer.gox:519
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

//line composer.gox:530
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
//line composer.gox:543
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
//line composer.gox:554
					__e = __c.Set("id", "text" + c.id()); if __e != nil { return }
//line composer.gox:555
					__e = __c.Set("class", "nechat-draft"); if __e != nil { return }
//line composer.gox:556
					__e = __c.Set("rows", "2"); if __e != nil { return }
//line composer.gox:557
					__e = __c.Modify(s); if __e != nil { return }
//line composer.gox:558
					__e = __c.Set("readonly", pending); if __e != nil { return }
//line composer.gox:559
					__e = __c.Set("aria-busy", func() any {
			if pending {
				return "true"
			}
			return "false"
		}()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line composer.gox:565
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
//line composer.gox:567
		__e = __c.Any(c.Controller.Pending().Bind(func(pending bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line composer.gox:568
			if !pending {
				__e = __c.Init("script"); if __e != nil { return }
				{
//line composer.gox:569
					__e = __c.Set("data:id", "text" + c.id()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const el = document.getElementById($data(\"id\"))\n\t\t\t\tel.focus()\n\t\t\t\tel.setSelectionRange(el.value.length, el.value.length)"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		return })
//line composer.gox:575
	})); if __e != nil { return }
	return })
//line composer.gox:576
}

func (c *Composer) sendIndication() doors.Indicators {
	return doors.JoinIndicators(
		doors.IndicateAttrQuery("#composer" + c.id(), "aria-busy", "true"),
		doors.IndicateAttrQuery("#send" + c.id(), "aria-busy", "true"),
	)
}
