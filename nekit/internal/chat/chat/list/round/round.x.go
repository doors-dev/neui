// Managed by GoX v0.4.0

//line round.gox:1
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

package round

import (
	"context"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/chat/rower"
	"neui.dev/nekit/internal/chat/chat/uploader"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/notify"
)

type Rewinder interface {
	Rewind() doors.Beam[api.RoundID]
	InitRewind(ctx context.Context, round api.RoundID, input api.Input)
}

type rewindState string

const (
	rewindNone rewindState = ""
	rewindTarget rewindState = "target"
	rewindAfter rewindState = "after"
)

func deriveRewind(rewinder Rewinder, id api.RoundID) doors.Beam[rewindState] {
	return doors.DeriveBeam(rewinder.Rewind(), func(target api.RoundID) rewindState {
		switch {
		case target == api.NoRoundID || id < target:
			return rewindNone
		case id == target:
			return rewindTarget
		default:
			return rewindAfter
		}
	})
}

type LiveRound = *liveRound

func NewLiveRound(ctx context.Context, params params.Params, thread api.ChatThread, rewinder Rewinder, latest doors.Beam[api.RoundID], r api.Round, span bool, chain rower.Chips) *liveRound {
	state := doors.NewSource(api.RoundInput)
	lead := chain
	if lead == nil {
		lead = rower.NewChips(params, nil, false)
	}
	lead.Records(ctx, len(r.PinnedRecords))
	return &liveRound{
		params: params,
		lead: lead,
		chained: chain != nil,
		tools: rower.NewChips(params, r.Response, false),
		rewind: deriveRewind(rewinder, r.ID),
		rewinder: rewinder,
		retriable: doors.NewSource(false),
		last: doors.DeriveBeam(latest, func(latest api.RoundID) bool {
			return latest == r.ID
		}),
		state: state,
		id: r.ID,
		input: r.Input,
		response: rower.NewResponse(ctx, params, r.Response, thread.FS(), lead),
		thread: thread,
		span: span,
	}
}

type liveRound struct {
	params params.Params
	last doors.Beam[bool]
	retriable doors.Source[bool]
	rewind doors.Beam[rewindState]
	rewinder Rewinder
	state doors.Source[api.RoundState]
	id api.RoundID
	input api.Input
	lead rower.Chips
	chained bool
	response rower.Response
	tools rower.Chips
	thread api.ChatThread
	span bool
}

func (l LiveRound) Tools() rower.Chips {
	return l.tools
}

func (l LiveRound) IsEmpty() bool {
	return l.input.Len() == 0 && l.response.IsEmpty()
}

func (l LiveRound) Head(ctx context.Context) rower.Chips {
	if l.response.IsEmpty() {
		l.lead.Merge(ctx, l.tools)
		return l.lead
	}
	if after := l.response.AfterText(); after != nil {
		after.Merge(ctx, l.tools)
		return after
	}
	l.tools.Reload(ctx)
	return l.tools
}

func (l LiveRound) IsErr() bool {
	switch l.state.Get() {
	case api.RoundAbort, api.RoundError, api.RoundRefuse:
		return true
	}
	return false
}

func (l LiveRound) Done(ctx context.Context, state api.RoundState, err string) {
	l.retriable.Update(ctx, state == api.RoundAbort || state == api.RoundError && !rejected(err))
	l.state.Update(ctx, state)
	l.response.Close(ctx)
	l.lead.Close(ctx)
	l.tools.Close(ctx)
}

func (l LiveRound) On(ctx context.Context, e api.IndexedEvent) {
	tool, ok := e.(api.EventTool)
	if !ok {
		l.response.On(ctx, e)
		return
	}
	l.tools.Append(tool)
}

//line round.gox:149
func (r LiveRound) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:151
		s := new(doors.Setter)
	state, _ := r.state.ReadAndSub(ctx, func(ctx context.Context, rs api.RoundState) bool {
		doors.Call(ctx, s.Set("data-state", string(rs)))
		return false
	})
	rewind, _ := r.rewind.ReadAndSub(ctx, func(ctx context.Context, rs rewindState) bool {
		doors.Call(ctx, s.Set("data-rewind", string(rs)))
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:161
			__e = __c.Set("class", "nechat-turn"); if __e != nil { return }
//line round.gox:161
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:161
			__e = __c.Set("data-state", string(state)); if __e != nil { return }
//line round.gox:161
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:162
			if r.input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:163
					__e = __c.Set("class", "nechat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:164
					if r.span {
//line round.gox:165
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:167
					__e = __c.Any(bubble(r.input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:168
						__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:169
						if !r.params.Off(params.Edit) {
//line round.gox:170
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.id, r.input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:175
				__e = __c.Set("class", "nechat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:176
				if !r.chained {
//line round.gox:177
					__e = __c.Any(r.lead); if __e != nil { return }
				}
//line round.gox:179
				__e = __c.Any(r.response); if __e != nil { return }
//line round.gox:180
				__e = __c.Any(r.tools); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:182
			__e = __c.Any(r.state.Bind(func(rs api.RoundState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:183
				if r.params.Off(params.Retry) {
//line round.gox:184
					__e = __c.Any(status(rs, time.Now(), nil)); if __e != nil { return }
				} else  {
//line round.gox:186
					__e = __c.Any(status(rs, time.Now(), r.retry())); if __e != nil { return }
				}
			return })
//line round.gox:188
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:190
}

//line round.gox:192
func (r LiveRound) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:193
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:193
			__e = __c.Any(func() any {
		retriable, ok := r.retriable.Effect(ctx)
		if !ok || !retriable {
			return nil
		}
		last, ok := r.last.Effect(ctx)
		if !ok || !last {
			return nil
		}
		return retryButton(r.thread)
	}()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:204
}

func rejected(err string) bool {
	return err == string(api.CodeInvalid) || err == string(api.CodeTooLarge)
}

type Round = *round

func NewRound(ctx context.Context, params params.Params, thread api.ChatThread, rewinder Rewinder, latest doors.Beam[api.RoundID], r api.Round, chain rower.Chips, span bool) Round {
	retriable := r.State == api.RoundAbort || r.State == api.RoundError && !rejected(r.Error) || r.State == api.RoundInput
	lead := chain
	if lead == nil {
		lead = rower.NewChips(params, nil, false)
	}
	lead.Records(ctx, len(r.PinnedRecords))
	response := rower.NewResponse(ctx, params, r.Response, thread.FS(), lead)
	response.Close(ctx)
	lead.Close(ctx)
	return &round{
		params: params,
		retriable: retriable,
		last: doors.DeriveBeam(latest, func(latest api.RoundID) bool {
			return latest == r.ID
		}),
		round: r,
		thread: thread,
		span: span,
		rewind: deriveRewind(rewinder, r.ID),
		rewinder: rewinder,
		lead: lead,
		chained: chain != nil,
		response: response,
		tools: rower.NewChips(params, r.Response, true),
	}
}

type round struct {
	params params.Params
	retriable bool
	last doors.Beam[bool]
	round api.Round
	thread api.ChatThread
	span bool
	rewind doors.Beam[rewindState]
	rewinder Rewinder
	lead rower.Chips
	chained bool
	response rower.Response
	tools rower.Chips
}

func (r Round) Tools() rower.Chips {
	return r.tools
}

func (r Round) IsEmpty() bool {
	return r.round.Input.Len() == 0 && r.response.IsEmpty()
}

func (r Round) Head(ctx context.Context) rower.Chips {
	if r.response.IsEmpty() {
		r.lead.Merge(ctx, r.tools)
		return r.lead
	}
	if after := r.response.AfterText(); after != nil {
		after.Merge(ctx, r.tools)
		return after
	}
	return r.tools
}

//line round.gox:275
func (r Round) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:277
		s := new(doors.Setter)
	rewind, _ := r.rewind.ReadAndSub(ctx, func(ctx context.Context, rs rewindState) bool {
		doors.Call(ctx, s.Set("data-rewind", string(rs)))
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:283
			__e = __c.Set("class", "nechat-turn"); if __e != nil { return }
//line round.gox:283
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:283
			__e = __c.Set("data-state", string(r.state())); if __e != nil { return }
//line round.gox:283
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:284
			if r.round.Input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:285
					__e = __c.Set("class", "nechat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:286
					if r.span {
//line round.gox:287
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:289
					__e = __c.Any(bubble(r.round.Input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:290
						__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:291
						if !r.params.Off(params.Edit) {
//line round.gox:292
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.round.ID, r.round.Input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:297
				__e = __c.Set("class", "nechat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:298
				if !r.chained {
//line round.gox:299
					__e = __c.Any(r.lead); if __e != nil { return }
				}
//line round.gox:301
				__e = __c.Any(r.response); if __e != nil { return }
//line round.gox:302
				__e = __c.Any(r.tools); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:304
			if r.params.Off(params.Retry) {
//line round.gox:305
				__e = __c.Any(status(r.round.State, time.Time{}, nil)); if __e != nil { return }
			} else  {
//line round.gox:307
				__e = __c.Any(status(r.round.State, time.Time{}, r.retry())); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:310
}

func (r Round) state() api.RoundState {
	if r.round.State == api.RoundInput {
		return api.RoundError
	}
	return r.round.State
}

//line round.gox:319
func (r Round) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:320
		if r.retriable {
//line round.gox:321
			__e = __c.Any(r.last.Bind(func(b bool) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:322
				if b {
//line round.gox:323
					__e = __c.Any(retryButton(r.thread)); if __e != nil { return }
				}
			return })
//line round.gox:325
		})); if __e != nil { return }
		}
	return })
//line round.gox:327
}

//line round.gox:329
func warnMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:330
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:330
			__e = __c.Any(icons.Warning()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:331
}

//line round.gox:333
func stopMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:334
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:334
			__e = __c.Any(icons.Square()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:335
}

//line round.gox:337
func rewindButton(composer Rewinder, rewind doors.Beam[rewindState], id api.RoundID, input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:338
		__e = __c.Any(rewind.Bind(func(rs rewindState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:339
			if rs != rewindTarget {
//line round.gox:340
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					composer.InitRewind(ctx, id, input)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line round.gox:347
						__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line round.gox:347
						__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:347
						__e = __c.Set("title", "Edit and resend this message"); if __e != nil { return }
//line round.gox:347
						__e = __c.Set("aria-label", "Edit and resend this message"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:348
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line round.gox:348
							__e = __c.Any(icons.Pencil()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
		return })
//line round.gox:351
	})); if __e != nil { return }
	return })
//line round.gox:352
}

//line round.gox:354
func retryButton(thread api.ChatThread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("i"); if __e != nil { return }
		{
//line round.gox:355
			__e = __c.Set("class", "nechat-rule"); if __e != nil { return }
//line round.gox:355
			__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line round.gox:356
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			thread.Retry(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line round.gox:363
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line round.gox:363
				__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:363
				__e = __c.Set("aria-label", "Run this round again"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:364
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line round.gox:364
					__e = __c.Any(icons.Retry()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:366
}

//line round.gox:368
func status(s api.RoundState, live time.Time, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:369
		if s == api.RoundInput && live.IsZero() {
//line round.gox:370
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("no response received"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:370
		return }), retry)); if __e != nil { return }
//line round.gox:371
		} else if s == api.RoundAbort {
//line round.gox:372
			__e = __c.Any(statusBlock(stopMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("stopped"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:372
		return }), retry)); if __e != nil { return }
//line round.gox:373
		} else if s == api.RoundError {
//line round.gox:374
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("request failed"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:374
		return }), retry)); if __e != nil { return }
//line round.gox:375
		} else if s == api.RoundRefuse {
//line round.gox:376
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("provider declined to answer"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:376
		return }), retry)); if __e != nil { return }
		}
	return })
//line round.gox:378
}

//line round.gox:380
func statusBlock(mark gox.Elem, text any, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:381
			__e = __c.Set("class", "nechat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:382
				__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
//line round.gox:382
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:382
				__e = __c.Any(mark); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:383
				__e = __c.Set("class", "nechat-col"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:384
					__e = __c.Set("class", "nechat-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:385
						__e = __c.Any(text); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line round.gox:386
					__e = __c.Any(retry); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:390
}

//line round.gox:392
func span() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:393
			__e = __c.Set("class", "nechat-span"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const span = document.currentScript.closest(\".nechat-span\")\n\t\t\tspan.scrollIntoView({ block: \"start\", behavior: \"smooth\" })\n\t\t\tlet seen = false\n\t\t\tconst observer = new IntersectionObserver(\n\t\t\t\t([entry]) => {\n\t\t\t\t\tif (entry.isIntersecting) {\n\t\t\t\t\t\tseen = true\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tif (!seen) return\n\t\t\t\t\tobserver.disconnect()\n\t\t\t\t\tspan.remove()\n\t\t\t\t},\n\t\t\t\t{ root: span.closest(\".nechat-log\") },\n\t\t\t)\n\t\t\tobserver.observe(span)\n\t\t\t$sys.clean(() => observer.disconnect())"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:414
}

//line round.gox:416
func bubble(input api.Input, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:417
			__e = __c.Set("class", "nechat-bubble"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:418
			if input.Text != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:419
					__e = __c.Set("class", "nechat-bubble-text"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:419
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line round.gox:421
			__e = __c.Any(files(input.Attachments, fs)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:423
}

//line round.gox:425
func files(names []string, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:426
		if len(names) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:427
				__e = __c.Set("class", "nechat-bubble-files"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:428
				for _, name := range names {
//line round.gox:429
					__e = __c.Any(uploader.Sent{FS: fs, Name: name}); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line round.gox:433
}
