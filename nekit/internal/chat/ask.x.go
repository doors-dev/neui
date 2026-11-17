// Managed by GoX v0.4.0

//line ask.gox:1
package chat

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/notify"
)

const askModuleDesc = `Questions for the user. One at a time: a new question cancels the one still open.`

const askOptionsDesc = `Ask the user to pick from a list. Use it for a choice that is theirs to make, not for anything you can work out yourself.`

// AskOption is one answer the user can pick in an AskOptions question.
type AskOption struct {
	// Name is the answer as shown and as returned in AskOutput.Picked.
	Name string `json:"name" required:"true" description:"the answer, a few words"`
	// Desc is an optional line under the name explaining the choice.
	Desc string `json:"description" description:"one line on what picking it means; omit when the name says it"`
}

// AskInput is what the agent sends to the ask tool: a question and the
// answers to offer.
type AskInput struct {
	// Question is shown above the options.
	Question string `json:"question" required:"true" description:"the question, one sentence"`
	// Options are listed in this order.
	Options []AskOption `json:"options" required:"true" description:"the answers to pick from, in order"`
	// Multiple shows checkboxes instead of radio buttons, so several answers can be picked.
	Multiple bool `json:"multiple,omitempty" description:"allow several answers instead of one"`
	// Own adds a free-text row for an answer of the user's own, up to 256 KiB.
	Own bool `json:"own,omitempty" description:"add a row for the user to type their own answer"`
}

// AskOutput is the user's answer as the agent receives it. At least one of
// Picked and Typed is set.
type AskOutput struct {
	// Picked holds the names of the chosen options, in option order.
	Picked []string `json:"picked,omitempty" description:"names of the options marked"`
	// Typed is the user's own answer, trimmed; empty unless the own row is
	// selected and has text.
	Typed string `json:"typed,omitempty" description:"the user's own typed answer; may come with picked when multiple"`
}

// ModuleAsk is the built-in module that lets the agent ask the user to pick
// from a list, rendered in the chat by Chat. Register it with
// neui.WithModules or neui.OptionModules:
//
//	neui.Enable(ctx, neui.OptionModules(nekit.ModuleAsk()))
func ModuleAsk() common.ModuleFactory {
	return func(context.Context) common.Module {
		m := tools.NewModule("asking", askModuleDesc)
		if _, err := m.Ask("options", askOptionsDesc, AskOptions); err != nil {
			panic(err)
		}
		return m
	}
}

const askOwnLimit = 256 * 1024

// AskOptions is the Ask handler behind ModuleAsk. It builds the form and send
// button for input. When the user sends with Enter or the button, it submits
// an AskOutput to answer, but only if at least one option is picked or own
// text is selected, and the own text is within 256 KiB. Use it to add the
// same question to a module of your own with neui.Module.Ask.
func AskOptions(input AskInput, answer common.Answer[AskOutput]) api.ChatAsk {
	state := doors.NewSourceEqual(askState{
		picked: make([]bool, len(input.Options)),
	}, func(old, new askState) bool {
		if old.own != new.own {
			return false
		}
		if old.text != new.text {
			return false
		}
		if old.bytes != new.bytes {
			return false
		}
		if !slices.Equal(old.picked, new.picked) {
			return false
		}
		return true
	})
	kind := "radio"
	if input.Multiple {
		kind = "checkbox"
	}
	a := &ask{
		input: input,
		answer: answer,
		state: state,
		arm: doors.DeriveBeam(state, func(s askState) askArm {
			n := 0
			for _, picked := range s.picked {
				if picked {
					n += 1
				}
			}
			if s.own && s.text != "" {
				n += 1
			}
			return askArm{count: n, over: s.own && s.bytes > askOwnLimit}
		}),
		frame: new(doors.ScopeFrame),
		kind: kind,
		group: "ask-" + doors.IDRand(),
	}
	return api.ChatAsk{
		Form: a.form(),
		Button: a.arm.Bind(a.button),
		Error: doors.DeriveBeam(state, func(s askState) string {
			if s.own && s.bytes > askOwnLimit {
				return "too long"
			}
			return ""
		}),
	}
}

type askState struct {
	picked []bool
	own bool
	text string
	bytes int
}

type askArm struct {
	count int
	over bool
}

type ask struct {
	input AskInput
	answer common.Answer[AskOutput]
	state doors.Source[askState]
	arm doors.Beam[askArm]
	frame *doors.ScopeFrame
	kind string
	group string
}

func (a *ask) submit(ctx context.Context) {
	state := a.state.Get()
	if state.own && state.bytes > askOwnLimit {
		return
	}
	var output AskOutput
	for i, picked := range state.picked {
		if picked {
			output.Picked = append(output.Picked, a.input.Options[i].Name)
		}
	}
	if state.own && state.text != "" {
		output.Typed = state.text
	}
	if len(output.Picked) == 0 && output.Typed == "" {
		return
	}
	a.answer.Submit(ctx, output)
}

//line ask.gox:173
func (a *ask) form() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ask.gox:175
		state, ok := a.state.Read(ctx)
	if !ok {
		return nil
	}

//line ask.gox:180
		__e = (doors.AKeyDown{
		Key: doors.Key{
			Key: "Enter",
			ShiftMod: doors.ModOff,
		},
		PreventDefault: true,
		Scope: a.frame.Scope(true),
		On: func(ctx context.Context, rk doors.RequestKeyboard) bool {
			a.submit(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line ask.gox:192
			__e = (doors.AChange{
		Scope: a.frame.Scope(false),
		On: func(ctx context.Context, rc doors.RequestChange) bool {
			value := rc.Event().Value
			checked := rc.Event().Checked
			a.state.Mutate(ctx, func(s askState) askState {
				if value == "own" {
					s.own = checked
					if checked && !a.input.Multiple {
						s.picked = make([]bool, len(s.picked))
					}
					return s
				}
				i, err := strconv.Atoi(value)
				if err != nil || i < 0 || i >= len(s.picked) {
					return s
				}
				picked := slices.Clone(s.picked)
				if !a.input.Multiple {
					picked = make([]bool, len(picked))
					s.own = false
				}
				picked[i] = checked
				s.picked = picked
				return s
			})
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ask.gox:221
					__e = __c.Set("class", "nechat-ask"); if __e != nil { return }
//line ask.gox:221
					__e = __c.Set("role", "group"); if __e != nil { return }
//line ask.gox:221
					__e = __c.Set("aria-label", a.input.Question); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ask.gox:222
						__e = __c.Set("class", "nechat-ask-q"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ask.gox:222
						__e = __c.Any(a.input.Question); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ask.gox:223
					for i, option := range a.input.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line ask.gox:224
							__e = __c.Set("class", "nechat-pick"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line ask.gox:226
								__e = __c.Set("type", a.kind); if __e != nil { return }
//line ask.gox:227
								__e = __c.Set("name", func() any {
						if a.input.Multiple {
							return nil
						}
						return a.group
					}()); if __e != nil { return }
//line ask.gox:233
								__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line ask.gox:234
								__e = __c.Set("checked", state.picked[i]); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line ask.gox:235
							if a.input.Multiple {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:236
									__e = __c.Set("class", "nechat-mark nechat-mark_check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:236
									__e = __c.Any(icons.Check()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							} else  {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:238
									__e = __c.Set("class", "nechat-mark nechat-mark_radio"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Init("span"); if __e != nil { return }
							{
//line ask.gox:240
								__e = __c.Set("class", "nechat-pick-label"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:241
									__e = __c.Set("class", "nechat-pick-name"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:241
									__e = __c.Any(option.Name); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line ask.gox:242
								if option.Desc != "" {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line ask.gox:243
										__e = __c.Set("class", "nechat-pick-desc"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line ask.gox:243
										__e = __c.Any(option.Desc); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line ask.gox:248
					if a.input.Own {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line ask.gox:249
							__e = __c.Set("class", "nechat-pick nechat-pick_own"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line ask.gox:251
								__e = __c.Set("type", a.kind); if __e != nil { return }
//line ask.gox:252
								__e = __c.Set("name", func() any {
						if a.input.Multiple {
							return nil
						}
						return a.group
					}()); if __e != nil { return }
//line ask.gox:258
								__e = __c.Set("value", "own"); if __e != nil { return }
//line ask.gox:259
								__e = __c.Set("checked", state.own); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line ask.gox:260
							if a.input.Multiple {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:261
									__e = __c.Set("class", "nechat-mark nechat-mark_check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:261
									__e = __c.Any(icons.Check()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							} else  {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:263
									__e = __c.Set("class", "nechat-mark nechat-mark_radio"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
//line ask.gox:265
							__e = (doors.AInput{
					Scope: a.frame.Scope(false).And(&doors.ScopeRate{
						Tick: 100 * time.Millisecond,
					}),
					On: func(ctx context.Context, ri doors.RequestInput) bool {
						text := strings.TrimSpace(ri.Event().Value)
						a.state.Mutate(ctx, func(s askState) askState {
							s.bytes = len(text)
							if s.bytes > askOwnLimit {
								return s
							}
							s.text = text
							return s
						})
						return false
					},
					OnError: notify.OnError,
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("textarea"); if __e != nil { return }
								{
//line ask.gox:282
									__e = __c.Set("rows", "1"); if __e != nil { return }
//line ask.gox:282
									__e = __c.Set("placeholder", "Type your own…"); if __e != nil { return }
//line ask.gox:282
									__e = __c.Set("aria-label", "Type your own answer"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:282
									__e = __c.Any(state.text); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("script"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Raw("const own =\n\t\t\t\t\tdocument.currentScript.parentElement.querySelector(\".nechat-pick_own\")\n\t\t\t\tconst mark = own.querySelector(\"input\")\n\t\t\t\town.querySelector(\"textarea\").addEventListener(\"input\", (e) => {\n\t\t\t\t\tconst on = e.target.value.trim() !== \"\"\n\t\t\t\t\tif (mark.checked === on) return\n\t\t\t\t\tmark.checked = on\n\t\t\t\t\tmark.dispatchEvent(new Event(\"change\", { bubbles: true }))\n\t\t\t\t})"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line ask.gox:297
}

//line ask.gox:299
func (a *ask) button(arm askArm) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ask.gox:300
		if arm.over {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line ask.gox:301
				__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:301
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line ask.gox:301
				__e = __c.Set("title", "Answer too long"); if __e != nil { return }
//line ask.gox:301
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ask.gox:302
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line ask.gox:302
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ask.gox:304
		} else if arm.count == 0 {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line ask.gox:305
				__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:305
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line ask.gox:305
				__e = __c.Set("title", "Nothing picked"); if __e != nil { return }
//line ask.gox:305
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ask.gox:306
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line ask.gox:306
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line ask.gox:310
			label := "Send 1 answer"
		if arm.count > 1 {
			label = fmt.Sprintf("Send %d answers", arm.count)
		}

//line ask.gox:315
			__e = (doors.AClick{
			Scope: a.frame.Scope(true),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				a.submit(ctx)
				return false
			},
			OnError: notify.OnError,
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line ask.gox:322
					__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:322
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line ask.gox:322
					__e = __c.Set("title", label); if __e != nil { return }
//line ask.gox:322
					__e = __c.Set("aria-label", label); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ask.gox:323
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line ask.gox:323
						__e = __c.Any(icons.Submit()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line ask.gox:326
}
