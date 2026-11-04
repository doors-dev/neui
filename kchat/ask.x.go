// Managed by GoX v0.3.3

//line ask.gox:1
package kchat

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
	"neui.dev/kchat/icons"
)

const askModuleDesc = `Questions for the user. One at a time: a new question cancels the one still open.`

const askOptionsDesc = `Ask the user to pick from a list. Use it for a choice that is theirs to make, not for anything you can work out yourself.`

type AskOption struct {
	Name string `json:"name" required:"true" description:"the answer, a few words"`
	Desc string `json:"description" description:"one line on what picking it means; omit when the name says it"`
}

type AskInput struct {
	Question string `json:"question" required:"true" description:"the question, one sentence"`
	Options []AskOption `json:"options" required:"true" description:"the answers to pick from, in order"`
	Multiple bool `json:"multiple,omitempty" description:"allow several answers instead of one"`
	Own bool `json:"own,omitempty" description:"add a row for the user to type their own answer"`
}

type AskOutput struct {
	Picked []string `json:"picked,omitempty" description:"names of the options marked"`
	Typed string `json:"typed,omitempty" description:"what the user typed instead"`
}

func ModuleAsk() common.ModuleFactory {
	return func(context.Context) common.Module {
		m := tools.NewModule("asking", askModuleDesc)
		if _, err := m.Ask("options", askOptionsDesc, AskOptions); err != nil {
			panic(err)
		}
		return m
	}
}

// should come from conf
const askOwnLimit = 256 * 1024

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

//line ask.gox:149
func (a *ask) form() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ask.gox:151
		state, ok := a.state.Read(ctx)
	if !ok {
		return nil
	}

//line ask.gox:156
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
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line ask.gox:167
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
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ask.gox:195
					__e = __c.Set("class", "kchat-ask"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ask.gox:196
						__e = __c.Set("class", "kchat-ask-q"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ask.gox:196
						__e = __c.Any(a.input.Question); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ask.gox:197
					for i, option := range a.input.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line ask.gox:198
							__e = __c.Set("class", "kchat-pick"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line ask.gox:200
								__e = __c.Set("type", a.kind); if __e != nil { return }
//line ask.gox:201
								__e = __c.Set("name", func() any {
						if a.input.Multiple {
							return nil
						}
						return a.group
					}()); if __e != nil { return }
//line ask.gox:207
								__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line ask.gox:208
								__e = __c.Set("checked", state.picked[i]); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line ask.gox:209
							if a.input.Multiple {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:210
									__e = __c.Set("class", "kchat-mark kchat-mark_check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:210
									__e = __c.Any(icons.Check()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							} else  {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:212
									__e = __c.Set("class", "kchat-mark kchat-mark_radio"); if __e != nil { return }
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
//line ask.gox:214
								__e = __c.Set("class", "kchat-pick-label"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:215
									__e = __c.Set("class", "kchat-pick-name"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:215
									__e = __c.Any(option.Name); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line ask.gox:216
								if option.Desc != "" {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line ask.gox:217
										__e = __c.Set("class", "kchat-pick-desc"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line ask.gox:217
										__e = __c.Any(option.Desc); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line ask.gox:222
					if a.input.Own {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line ask.gox:223
							__e = __c.Set("class", "kchat-pick kchat-pick_own"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line ask.gox:225
								__e = __c.Set("type", a.kind); if __e != nil { return }
//line ask.gox:226
								__e = __c.Set("name", func() any {
						if a.input.Multiple {
							return nil
						}
						return a.group
					}()); if __e != nil { return }
//line ask.gox:232
								__e = __c.Set("value", "own"); if __e != nil { return }
//line ask.gox:233
								__e = __c.Set("checked", state.own); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line ask.gox:234
							if a.input.Multiple {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:235
									__e = __c.Set("class", "kchat-mark kchat-mark_check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:235
									__e = __c.Any(icons.Check()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							} else  {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line ask.gox:237
									__e = __c.Set("class", "kchat-mark kchat-mark_radio"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("span"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
//line ask.gox:239
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
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("textarea"); if __e != nil { return }
								{
//line ask.gox:255
									__e = __c.Set("rows", "1"); if __e != nil { return }
//line ask.gox:255
									__e = __c.Set("placeholder", "Type your own…"); if __e != nil { return }
//line ask.gox:255
									__e = __c.Set("aria-label", "Type your own answer"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ask.gox:255
									__e = __c.Any(state.text); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("script"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Raw("const own =\n\t\t\t\t\tdocument.currentScript.parentElement.querySelector(\".kchat-pick_own\")\n\t\t\t\tconst mark = own.querySelector(\"input\")\n\t\t\t\town.querySelector(\"textarea\").addEventListener(\"input\", (e) => {\n\t\t\t\t\tconst on = e.target.value.trim() !== \"\"\n\t\t\t\t\tif (mark.checked === on) return\n\t\t\t\t\tmark.checked = on\n\t\t\t\t\tmark.dispatchEvent(new Event(\"change\", { bubbles: true }))\n\t\t\t\t})"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line ask.gox:270
}

//line ask.gox:272
func (a *ask) button(arm askArm) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ask.gox:273
		if arm.over {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line ask.gox:274
				__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:274
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line ask.gox:274
				__e = __c.Set("title", "Answer too long"); if __e != nil { return }
//line ask.gox:274
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ask.gox:275
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line ask.gox:275
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ask.gox:277
		} else if arm.count == 0 {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line ask.gox:278
				__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:278
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line ask.gox:278
				__e = __c.Set("title", "Nothing picked"); if __e != nil { return }
//line ask.gox:278
				__e = __c.Set("aria-label", "Send"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ask.gox:279
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line ask.gox:279
					__e = __c.Any(icons.Submit()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line ask.gox:283
			label := "Send 1 answer"
		if arm.count > 1 {
			label = fmt.Sprintf("Send %d answers", arm.count)
		}

//line ask.gox:288
			__e = (doors.AClick{
			Scope: a.frame.Scope(true),
			On: func(ctx context.Context, rp doors.RequestPointer) bool {
				a.submit(ctx)
				return false
			},
		}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line ask.gox:294
					__e = __c.Set("type", "button"); if __e != nil { return }
//line ask.gox:294
					__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line ask.gox:294
					__e = __c.Set("title", label); if __e != nil { return }
//line ask.gox:294
					__e = __c.Set("aria-label", label); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ask.gox:295
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line ask.gox:295
						__e = __c.Any(icons.Submit()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line ask.gox:298
}
