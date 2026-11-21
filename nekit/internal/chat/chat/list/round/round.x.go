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
	"strings"
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

func NewLiveRound(params params.Params, thread api.ChatThread, rewinder Rewinder, latest doors.Beam[api.RoundID], r api.Round, span bool) *liveRound {
	state := doors.NewSource(api.RoundInput)
	return &liveRound{
		params: params,
		tools: rower.NewTools(r.Response, false),
		rewind: deriveRewind(rewinder, r.ID),
		rewinder: rewinder,
		retriable: doors.NewSource(false),
		last: doors.DeriveBeam(latest, func(latest api.RoundID) bool {
			return latest == r.ID
		}),
		state: state,
		id: r.ID,
		input: r.Input,
		records: r.PinnedRecords,
		response: rower.NewResponse(params, r.Response, thread.FS()),
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
	records []api.Record
	response rower.Response
	tools rower.Tools
	thread api.ChatThread
	span bool
}

func (l LiveRound) Tools() rower.Tools {
	return l.tools
}

func (l LiveRound) IsEmpty() bool {
	return l.input.Len() == 0 && len(l.records) == 0 && l.response.IsEmpty()
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
	l.response.Close()
	l.tools.Close(ctx)
}

func (l LiveRound) On(e api.IndexedEvent) {
	tool, ok := e.(api.EventTool)
	if !ok {
		l.response.On(e)
		return
	}
	l.tools.Append(tool)
}

//line round.gox:129
func (r LiveRound) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:131
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
//line round.gox:141
			__e = __c.Set("class", "nechat-turn"); if __e != nil { return }
//line round.gox:141
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:141
			__e = __c.Set("data-state", string(state)); if __e != nil { return }
//line round.gox:141
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:142
			if r.input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:143
					__e = __c.Set("class", "nechat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:144
					if r.span {
//line round.gox:145
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:147
					__e = __c.Any(bubble(r.input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:148
						__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:149
						if !r.params.Off(params.Edit) {
//line round.gox:150
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.id, r.input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:155
				__e = __c.Set("class", "nechat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:156
				if !r.params.Off(params.Tools) {
//line round.gox:157
					__e = __c.Any(records(r.records)); if __e != nil { return }
				}
//line round.gox:159
				__e = __c.Any(r.response); if __e != nil { return }
//line round.gox:160
				if !r.params.Off(params.Tools) {
//line round.gox:161
					__e = __c.Any(r.tools); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:164
			__e = __c.Any(r.state.Bind(func(rs api.RoundState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:165
				if r.params.Off(params.Retry) {
//line round.gox:166
					__e = __c.Any(status(rs, time.Now(), nil)); if __e != nil { return }
				} else  {
//line round.gox:168
					__e = __c.Any(status(rs, time.Now(), r.retry())); if __e != nil { return }
				}
			return })
//line round.gox:170
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:172
}

//line round.gox:174
func (r LiveRound) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:175
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:175
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
//line round.gox:186
}

func rejected(err string) bool {
	code, _, _ := strings.Cut(err, ":")
	return code == string(api.CodeInvalid) || code == string(api.CodeTooLarge)
}

type Round = *round

func NewRound(params params.Params, thread api.ChatThread, rewinder Rewinder, latest doors.Beam[api.RoundID], r api.Round, tools rower.Tools, span bool) Round {
	retriable := r.State == api.RoundAbort || r.State == api.RoundError && !rejected(r.Error) || r.State == api.RoundInput
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
		tools: tools,
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
	tools rower.Tools
}

//line round.gox:224
func (r Round) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:226
		s := new(doors.Setter)
	rewind, _ := r.rewind.ReadAndSub(ctx, func(ctx context.Context, rs rewindState) bool {
		doors.Call(ctx, s.Set("data-rewind", string(rs)))
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:232
			__e = __c.Set("class", "nechat-turn"); if __e != nil { return }
//line round.gox:232
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:232
			__e = __c.Set("data-state", string(r.state())); if __e != nil { return }
//line round.gox:232
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:233
			if r.round.Input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:234
					__e = __c.Set("class", "nechat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:235
					if r.span {
//line round.gox:236
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:238
					__e = __c.Any(bubble(r.round.Input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:239
						__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:240
						if !r.params.Off(params.Edit) {
//line round.gox:241
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.round.ID, r.round.Input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:246
				__e = __c.Set("class", "nechat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:247
				if !r.params.Off(params.Tools) {
//line round.gox:248
					__e = __c.Any(records(r.round.PinnedRecords)); if __e != nil { return }
				}
//line round.gox:250
				__e = __c.Any(func() any {
				r := rower.NewResponse(r.params, r.round.Response, r.thread.FS())
				r.Close()
				return r
			}()); if __e != nil { return }
//line round.gox:255
				if !r.params.Off(params.Tools) {
//line round.gox:256
					__e = __c.Any(r.tools); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:259
			if r.params.Off(params.Retry) {
//line round.gox:260
				__e = __c.Any(status(r.round.State, time.Time{}, nil)); if __e != nil { return }
			} else  {
//line round.gox:262
				__e = __c.Any(status(r.round.State, time.Time{}, r.retry())); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:265
}

func (r Round) state() api.RoundState {
	if r.round.State == api.RoundInput {
		return api.RoundError
	}
	return r.round.State
}

//line round.gox:274
func (r Round) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:275
		if r.retriable {
//line round.gox:276
			__e = __c.Any(r.last.Bind(func(b bool) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:277
				if b {
//line round.gox:278
					__e = __c.Any(retryButton(r.thread)); if __e != nil { return }
				}
			return })
//line round.gox:280
		})); if __e != nil { return }
		}
	return })
//line round.gox:282
}

//line round.gox:284
func warnMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:285
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:285
			__e = __c.Any(icons.Warning()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:286
}

//line round.gox:288
func stopMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:289
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:289
			__e = __c.Any(icons.Square()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:290
}

//line round.gox:292
func rewindButton(composer Rewinder, rewind doors.Beam[rewindState], id api.RoundID, input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:293
		__e = __c.Any(rewind.Bind(func(rs rewindState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:294
			if rs != rewindTarget {
//line round.gox:295
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
//line round.gox:302
						__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line round.gox:302
						__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:302
						__e = __c.Set("title", "Edit and resend this message"); if __e != nil { return }
//line round.gox:302
						__e = __c.Set("aria-label", "Edit and resend this message"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:303
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line round.gox:303
							__e = __c.Any(icons.Pencil()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
		return })
//line round.gox:306
	})); if __e != nil { return }
	return })
//line round.gox:307
}

//line round.gox:309
func retryButton(thread api.ChatThread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("i"); if __e != nil { return }
		{
//line round.gox:310
			__e = __c.Set("class", "nechat-rule"); if __e != nil { return }
//line round.gox:310
			__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line round.gox:311
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
//line round.gox:318
				__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line round.gox:318
				__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:318
				__e = __c.Set("aria-label", "Run this round again"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:319
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line round.gox:319
					__e = __c.Any(icons.Retry()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:321
}

//line round.gox:323
func status(s api.RoundState, live time.Time, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:324
		if s == api.RoundInput && live.IsZero() {
//line round.gox:325
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("no response received"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:325
		return }), retry)); if __e != nil { return }
//line round.gox:326
		} else if s == api.RoundAbort {
//line round.gox:327
			__e = __c.Any(statusBlock(stopMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("stopped"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:327
		return }), retry)); if __e != nil { return }
//line round.gox:328
		} else if s == api.RoundError {
//line round.gox:329
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("request failed"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:329
		return }), retry)); if __e != nil { return }
//line round.gox:330
		} else if s == api.RoundRefuse {
//line round.gox:331
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("provider declined to answer"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:331
		return }), retry)); if __e != nil { return }
		}
	return })
//line round.gox:333
}

//line round.gox:335
func records(recs []api.Record) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:336
		if len(recs) == 1 {
//line round.gox:337
			__e = __c.Any(recordsBlock(gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("1 record"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:337
		return }))); if __e != nil { return }
//line round.gox:338
		} else if len(recs) > 1 {
//line round.gox:339
			__e = __c.Any(recordsBlock(gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line round.gox:339
				__e = __c.Any(len(recs)); if __e != nil { return }
				__e = __c.Text(" records"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:339
		return }))); if __e != nil { return }
		}
	return })
//line round.gox:341
}

//line round.gox:343
func recordsBlock(text any) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:344
			__e = __c.Set("class", "nechat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:345
				__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
//line round.gox:345
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:346
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line round.gox:346
					__e = __c.Any(icons.FloppyDisk()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:348
				__e = __c.Set("class", "nechat-col nechat-records"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:348
				__e = __c.Any(text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:350
}

//line round.gox:352
func statusBlock(mark gox.Elem, text any, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:353
			__e = __c.Set("class", "nechat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:354
				__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
//line round.gox:354
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:354
				__e = __c.Any(mark); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:355
				__e = __c.Set("class", "nechat-col"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:356
					__e = __c.Set("class", "nechat-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:357
						__e = __c.Any(text); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line round.gox:358
					__e = __c.Any(retry); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:362
}

//line round.gox:364
func span() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:365
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
//line round.gox:386
}

//line round.gox:388
func bubble(input api.Input, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:389
			__e = __c.Set("class", "nechat-bubble"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:390
			if input.Text != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:391
					__e = __c.Set("class", "nechat-bubble-text"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:391
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line round.gox:393
			__e = __c.Any(files(input.Attachments, fs)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:395
}

//line round.gox:397
func files(names []string, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:398
		if len(names) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:399
				__e = __c.Set("class", "nechat-bubble-files"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:400
				for _, name := range names {
//line round.gox:401
					__e = __c.Any(uploader.Sent{FS: fs, Name: name}); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line round.gox:405
}
