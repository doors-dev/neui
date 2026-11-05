// Managed by GoX v0.3.3

//line chat.gox:1
package chat

import (
	"context"
	"errors"
	"sync"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/anchor"
	"neui.dev/kchat/chat/composer"
	"neui.dev/kchat/chat/composer/controller"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/chat/list"
	"neui.dev/kchat/pops"
	"neui.dev/kchat/params"
)

type Chat struct {
	Params params.Params
	Inst api.Instance
	Pops pops.Popup[string]
}

const pasteAsFile = 64 * 1024

func (c Chat) Main() gox.Elem {
	return c.Inst.Render(c)
}

func (c Chat) Chat(thread api.ChatThread, s api.ChatState) api.Chat {
	controller := controller.NewComposerController(thread, s.Input, s.Round != nil, s.Usage, s.Tasks, s.Compaction, s.Ask)
	at := anchor.Read(s.Meta)
	return &chat{
		params: c.Params,
		thread: thread,
		inst: c.Inst,
		composer: controller,
		pops: c.Pops,
		list: list.NewList(c.Params, c.Inst, thread, controller, s.ID, s.Round, at),
		name: doors.NewSource(s.Name),
		dead: doors.NewSource(alive),
	}
}

type closedKind int

const (
	alive closedKind = iota
	failed
	deleted
)

func closedKindOf(err error) closedKind {
	if errors.Is(err, api.ErrNotFound) {
		return deleted
	}
	return failed
}

//line chat.gox:63
func (c Chat) Placeholder(enabled doors.Beam[bool]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:64
		__e = __c.Any(enabled.Bind(func(loading bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:66
				__e = __c.Set("class", "kchat-pane kchat-widget_off"); if __e != nil { return }
//line chat.gox:67
				__e = __c.Set("aria-disabled", "true"); if __e != nil { return }
//line chat.gox:68
				__e = __c.Set("aria-busy", func() any {
				if loading {
					return "true"
				}
				return "false"
			}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chat.gox:74
					__e = __c.Set("class", "kchat-top"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chat.gox:75
					__e = __c.Set("class", "kchat-log kchat-log_scroll"); if __e != nil { return }
//line chat.gox:75
					__e = __c.Set("role", "log"); if __e != nil { return }
//line chat.gox:75
					__e = __c.Set("aria-label", "Chat messages"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chat.gox:76
					__e = __c.Set("class", "kchat-composer"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chat.gox:77
						__e = __c.Set("class", "kchat-tray"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("textarea"); if __e != nil { return }
					{
//line chat.gox:79
						__e = __c.Set("class", "kchat-draft"); if __e != nil { return }
//line chat.gox:79
						__e = __c.Set("rows", "2"); if __e != nil { return }
						__e = __c.Set("disabled", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chat.gox:80
						__e = __c.Set("class", "kchat-barrow"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line chat.gox:81
							__e = __c.Set("class", "kchat-barrow-left"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line chat.gox:82
							if !c.Params.Off(params.Attachments) {
								__e = __c.Init("button"); if __e != nil { return }
								{
//line chat.gox:83
									__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
									__e = __c.Set("disabled", true); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line chat.gox:84
									__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
//line chat.gox:84
										__e = __c.Any(icons.Attach()); if __e != nil { return }
									return })); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line chat.gox:88
							__e = __c.Set("class", "kchat-barrow-right"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("button"); if __e != nil { return }
							{
//line chat.gox:89
								__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
								__e = __c.Set("disabled", true); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line chat.gox:90
								__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
//line chat.gox:90
									__e = __c.Any(icons.Submit()); if __e != nil { return }
								return })); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })
//line chat.gox:96
	})); if __e != nil { return }
	return })
//line chat.gox:97
}

type chat struct {
	params params.Params
	thread api.ChatThread
	inst api.Instance
	composer controller.ComposerController
	pops pops.Popup[string]
	list list.List
	name doors.Source[string]
	dead doors.Source[closedKind]
}

func (c *chat) Update(ctx context.Context, e api.ChatEvent) {
	doors.OnBatch(ctx, func(context.Context) {}, func(ctx context.Context) {
		if api.Final(e) {
			c.composer.UpdateActive(ctx, false)
		}
		switch e := e.(type) {
		case api.ChatEventName:
			c.name.Update(ctx, e.Name)
		case api.ChatEventRoundEnd:
			c.composer.RoundDone(ctx, e.State, e.Usage)
			c.list.RoundDone(ctx, e.State, e.Error)
		case api.ChatEventRoundStart:
			c.composer.UpdateActive(ctx, true)
			if e.Round.Input.Len() != 0 {
				c.composer.FlushInput(ctx)
			}
			c.list.RoundStart(ctx, e.Round, e.Truncate)
		case api.ChatEventInputPending:
			c.composer.UpdatePending(ctx, e.Pending)
		case api.ChatEventTasks:
			c.composer.UpdateTasks(ctx, e.Tasks)
		case api.ChatEventCompacting:
			c.composer.UpdateCompaction(ctx, e.Running, e.Usage)
		case api.ChatEventRoundUsage:
			c.composer.UpdateUsage(ctx, e.Usage)
		case api.ChatEventAsk:
			c.composer.Ask(ctx, e.Ask)
		case api.ChatEventAskDone:
			c.composer.AskDone(ctx)
		case api.ChatEventIndexed:
			c.list.IndexedEvent(e.Event)
		case api.ChatEventToolResults:
			c.list.ToolsDone()
		case api.ChatEventCriticalError:
			c.dead.Update(ctx, closedKindOf(e.Err))
			c.pops.Hide(ctx, "files")
		}
	})
}

//line chat.gox:150
func (c *chat) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:151
		__e = (doors.AKeyDown{
		Key: doors.Key{Key: "Escape"},
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rk doors.RequestKeyboard) bool {
			c.composer.Escape(ctx)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:159
				__e = __c.Set("class", "kchat-pane"); if __e != nil { return }
//line chat.gox:160
				__e = __c.Set("data-attach", func() any {
			if c.params.Off(params.Attachments) {
				return "off"
			}
			return nil
		}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chat.gox:166
					__e = __c.Set("class", "kchat-top kchat-top_menu"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line chat.gox:167
					__e = __c.Any(c.name.Bind(title)); if __e != nil { return }
//line chat.gox:168
					if !c.params.Off(params.Files) {
//line chat.gox:169
						__e = __c.Any(c.files()); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line chat.gox:172
				__e = __c.Any(c.dead.Bind(c.body)); if __e != nil { return }
				__e = __c.Init("script"); if __e != nil { return }
				{
//line chat.gox:173
					__e = __c.Set("data:id", c.params.ID); if __e != nil { return }
//line chat.gox:173
					__e = __c.Set("data:paste", pasteAsFile); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const main = document.currentScript.parentElement\n\t\t\tconst id = $data(\"id\")\n\t\t\tconst pasteAsFile = $data(\"paste\")\n\t\t\tconst attach = (files) => {\n\t\t\t\tconst composer = main.querySelector(\"#composer\" + id)\n\t\t\t\tif (!composer || composer.getAttribute(\"aria-busy\") === \"true\") return false\n\t\t\t\tconst input = main.querySelector(\"#attach\" + id + \" input\")\n\t\t\t\tif (!input || !files.length) return false\n\t\t\t\tinput.files = files\n\t\t\t\tinput.dispatchEvent(new Event(\"change\"))\n\t\t\t\treturn true\n\t\t\t}\n\t\t\tmain.addEventListener(\"paste\", (e) => {\n\t\t\t\tconst files = e.clipboardData.files\n\t\t\t\tif (files.length) {\n\t\t\t\t\tif (attach(files)) e.preventDefault()\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tconst text = e.clipboardData.getData(\"text/plain\")\n\t\t\t\tif (main.dataset.attach === \"off\") return\n\t\t\t\tif (!text || new TextEncoder().encode(text).length <= pasteAsFile) return\n\t\t\t\tconst head = text.slice(0, 64).split(\"\\n\", 1)[0]\n\t\t\t\tconst base = head\n\t\t\t\t\t.replace(/[\\\\/:*?\"<>|\\x00-\\x1f]+/g, \" \")\n\t\t\t\t\t.replace(/\\s+/g, \" \")\n\t\t\t\t\t.replace(/^[.\\s]+/, \"\")\n\t\t\t\t\t.trim()\n\t\t\t\t\t.slice(0, 40)\n\t\t\t\tconst dt = new DataTransfer()\n\t\t\t\tdt.items.add(\n\t\t\t\t\tnew File([text], (base || \"pasted\") + \".txt\", { type: \"text/plain\" }),\n\t\t\t\t)\n\t\t\t\tif (attach(dt.files)) e.preventDefault()\n\t\t\t})\n\t\t\tmain.addEventListener(\"dragenter\", (e) => {\n\t\t\t\tif (!e.dataTransfer.types.includes(\"Files\")) return\n\t\t\t\te.preventDefault()\n\t\t\t})\n\t\t\tmain.addEventListener(\"dragover\", (e) => {\n\t\t\t\tif (!e.dataTransfer.types.includes(\"Files\")) return\n\t\t\t\te.preventDefault()\n\t\t\t})\n\t\t\tmain.addEventListener(\"drop\", (e) => {\n\t\t\t\tif (!e.dataTransfer.types.includes(\"Files\")) return\n\t\t\t\te.preventDefault()\n\t\t\t\tconst dt = new DataTransfer()\n\t\t\t\tfor (const item of e.dataTransfer.items) {\n\t\t\t\t\tif (item.kind !== \"file\") continue\n\t\t\t\t\tconst entry = item.webkitGetAsEntry()\n\t\t\t\t\tif (entry && entry.isDirectory) continue\n\t\t\t\t\tdt.items.add(item.getAsFile())\n\t\t\t\t}\n\t\t\t\tattach(dt.files)\n\t\t\t})"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line chat.gox:230
}

//line chat.gox:232
func (c *chat) body(kind closedKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:233
		if kind == alive {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:234
				__e = __c.Set("class", "kchat-log kchat-log_scroll"); if __e != nil { return }
//line chat.gox:234
				__e = __c.Set("role", "log"); if __e != nil { return }
//line chat.gox:234
				__e = __c.Set("aria-label", "Chat messages"); if __e != nil { return }
//line chat.gox:234
				__e = __c.Set("data-seeding", ""); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:235
				__e = __c.Any(c.list.Main()); if __e != nil { return }
//line chat.gox:236
				__e = __c.Any(c.list.Script()); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line chat.gox:238
			__e = __c.Any(composer.Composer{
			Controller: c.composer,
			Params: c.params,
			Pops: c.pops,
		}); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:244
				__e = __c.Set("class", "kchat-closed-area"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:245
				__e = __c.Any(c.closed(kind)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line chat.gox:248
}

//line chat.gox:250
func (c *chat) closed(kind closedKind) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:252
		icon, head, text := icons.WarningFill(), "Something went wrong", "This chat hit an error."
	if kind == deleted {
		icon, head, text = icons.Trash(), "This chat was deleted.", "It was deleted elsewhere."
	}
	var once sync.Once
	confirm := func(ctx context.Context) {
		once.Do(func() {
			c.thread.Reload(ctx)
		})
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line chat.gox:263
			__e = __c.Set("class", "kchat-closed-card"); if __e != nil { return }
//line chat.gox:263
			__e = __c.Set("role", "alert"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line chat.gox:264
			__e = (doors.Class("kchat-ic kchat-closed-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line chat.gox:264
				__e = __c.Any(icon); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line chat.gox:265
				__e = __c.Set("class", "kchat-closed-title"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:265
				__e = __c.Any(head); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line chat.gox:266
				__e = __c.Set("class", "kchat-closed-text"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:266
				__e = __c.Any(text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chat.gox:267
				__e = __c.Set("class", "kchat-closed-acts"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:268
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					confirm(ctx)
					return true
				},
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line chat.gox:274
						__e = __c.Set("type", "button"); if __e != nil { return }
//line chat.gox:274
						__e = __c.Set("class", "kchat-btn kchat-btn_auto"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("OK"); if __e != nil { return }
						__e = __c.Init("i"); if __e != nil { return }
						{
//line chat.gox:274
							__e = __c.Set("class", "kchat-file-fill"); if __e != nil { return }
//line chat.gox:274
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line chat.gox:276
			__e = __c.Any(doors.Go(func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case <-time.After(10 * time.Second):
				confirm(ctx)
			}
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line chat.gox:284
}

//line chat.gox:286
func title(name string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chat.gox:287
		if name == "" {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line chat.gox:288
				__e = __c.Set("class", "kchat-top-title kchat-top-title_quiet"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("New Chat"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line chat.gox:290
				__e = __c.Set("class", "kchat-top-title"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chat.gox:290
				__e = __c.Any(name); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line chat.gox:292
}
