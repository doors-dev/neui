// Managed by GoX v0.3.3

//line round.gox:1
package round

import (
	"context"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/icons"
	"neui.dev/kchat/chat/rower"
	"neui.dev/kchat/chat/uploader"
	"neui.dev/kchat/params"
	"neui.dev/kchat/notify"
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
		errored: doors.DeriveBeam(state, func(s api.RoundState) bool {
			return s == api.RoundAbort || s == api.RoundError
		}),
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
	errored doors.Beam[bool]
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

//line round.gox:115
func (r LiveRound) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:117
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
//line round.gox:127
			__e = __c.Set("class", "kchat-turn"); if __e != nil { return }
//line round.gox:127
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:127
			__e = __c.Set("data-state", string(state)); if __e != nil { return }
//line round.gox:127
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:128
			if r.input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:129
					__e = __c.Set("class", "kchat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:130
					if r.span {
//line round.gox:131
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:133
					__e = __c.Any(bubble(r.input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:134
						__e = __c.Set("class", "kchat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:135
						if !r.params.Off(params.Edit) {
//line round.gox:136
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.id, r.input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:141
				__e = __c.Set("class", "kchat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:142
				if !r.params.Off(params.Tools) {
//line round.gox:143
					__e = __c.Any(records(r.records)); if __e != nil { return }
				}
//line round.gox:145
				__e = __c.Any(r.response); if __e != nil { return }
//line round.gox:146
				if !r.params.Off(params.Tools) {
//line round.gox:147
					__e = __c.Any(r.tools); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:150
			__e = __c.Any(r.state.Bind(func(rs api.RoundState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:151
				if r.params.Off(params.Retry) {
//line round.gox:152
					__e = __c.Any(status(rs, time.Now(), nil)); if __e != nil { return }
				} else  {
//line round.gox:154
					__e = __c.Any(status(rs, time.Now(), r.retry())); if __e != nil { return }
				}
			return })
//line round.gox:156
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:158
}

//line round.gox:160
func (r LiveRound) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:161
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:161
			__e = __c.Any(func() any {
		errored, ok := r.errored.Effect(ctx)
		if !ok || !errored {
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
//line round.gox:172
}

type Round = *round

func NewRound(params params.Params, thread api.ChatThread, rewinder Rewinder, latest doors.Beam[api.RoundID], r api.Round, tools rower.Tools, span bool) Round {
	errored := r.State == api.RoundAbort || r.State == api.RoundError || r.State == api.RoundInput
	return &round{
		params: params,
		errored: errored,
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
	errored bool
	last doors.Beam[bool]
	round api.Round
	thread api.ChatThread
	span bool
	rewind doors.Beam[rewindState]
	rewinder Rewinder
	tools rower.Tools
}

//line round.gox:205
func (r Round) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:207
		s := new(doors.Setter)
	rewind, _ := r.rewind.ReadAndSub(ctx, func(ctx context.Context, rs rewindState) bool {
		doors.Call(ctx, s.Set("data-rewind", string(rs)))
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:213
			__e = __c.Set("class", "kchat-turn"); if __e != nil { return }
//line round.gox:213
			__e = __c.Modify(s); if __e != nil { return }
//line round.gox:213
			__e = __c.Set("data-state", string(r.state())); if __e != nil { return }
//line round.gox:213
			__e = __c.Set("data-rewind", string(rewind)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:214
			if r.round.Input.Len() != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:215
					__e = __c.Set("class", "kchat-user"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:216
					if r.span {
//line round.gox:217
						__e = __c.Any(span()); if __e != nil { return }
					}
//line round.gox:219
					__e = __c.Any(bubble(r.round.Input, r.thread.FS())); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line round.gox:220
						__e = __c.Set("class", "kchat-gut"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:221
						if !r.params.Off(params.Edit) {
//line round.gox:222
							__e = __c.Any(rewindButton(r.rewinder, r.rewind, r.round.ID, r.round.Input)); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:227
				__e = __c.Set("class", "kchat-agent"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:228
				if !r.params.Off(params.Tools) {
//line round.gox:229
					__e = __c.Any(records(r.round.PinnedRecords)); if __e != nil { return }
				}
//line round.gox:231
				__e = __c.Any(func() any {
				r := rower.NewResponse(r.params, r.round.Response, r.thread.FS())
				r.Close()
				return r
			}()); if __e != nil { return }
//line round.gox:236
				if !r.params.Off(params.Tools) {
//line round.gox:237
					__e = __c.Any(r.tools); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:240
			if r.params.Off(params.Retry) {
//line round.gox:241
				__e = __c.Any(status(r.round.State, time.Time{}, nil)); if __e != nil { return }
			} else  {
//line round.gox:243
				__e = __c.Any(status(r.round.State, time.Time{}, r.retry())); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:246
}

func (r Round) state() api.RoundState {
	if r.round.State == api.RoundInput {
		return api.RoundError
	}
	return r.round.State
}

//line round.gox:255
func (r Round) retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:256
		if r.errored {
//line round.gox:257
			__e = __c.Any(r.last.Bind(func(b bool) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line round.gox:258
				if b {
//line round.gox:259
					__e = __c.Any(retryButton(r.thread)); if __e != nil { return }
				}
			return })
//line round.gox:261
		})); if __e != nil { return }
		}
	return })
//line round.gox:263
}

//line round.gox:265
func warnMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:266
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:266
			__e = __c.Any(icons.Warning()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:267
}

//line round.gox:269
func stopMark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:270
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:270
			__e = __c.Any(icons.Square()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:271
}

//line round.gox:273
func rewindButton(composer Rewinder, rewind doors.Beam[rewindState], id api.RoundID, input api.Input) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:274
		__e = __c.Any(rewind.Bind(func(rs rewindState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line round.gox:275
			if rs != rewindTarget {
//line round.gox:276
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
//line round.gox:283
						__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line round.gox:283
						__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:283
						__e = __c.Set("title", "Edit and resend this message"); if __e != nil { return }
//line round.gox:283
						__e = __c.Set("aria-label", "Edit and resend this message"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:284
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line round.gox:284
							__e = __c.Any(icons.Pencil()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
		return })
//line round.gox:287
	})); if __e != nil { return }
	return })
//line round.gox:288
}

//line round.gox:290
func retryButton(thread api.ChatThread) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("i"); if __e != nil { return }
		{
//line round.gox:291
			__e = __c.Set("class", "kchat-rule"); if __e != nil { return }
//line round.gox:291
			__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line round.gox:292
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
//line round.gox:299
				__e = __c.Set("class", "kchat-iconbtn"); if __e != nil { return }
//line round.gox:299
				__e = __c.Set("type", "button"); if __e != nil { return }
//line round.gox:299
				__e = __c.Set("aria-label", "Run this round again"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:300
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line round.gox:300
					__e = __c.Any(icons.Retry()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line round.gox:302
}

//line round.gox:304
func status(s api.RoundState, live time.Time, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:305
		if s == api.RoundInput && live.IsZero() {
//line round.gox:306
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("no response received"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:306
		return }), retry)); if __e != nil { return }
//line round.gox:307
		} else if s == api.RoundAbort {
//line round.gox:308
			__e = __c.Any(statusBlock(stopMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("stopped"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:308
		return }), retry)); if __e != nil { return }
//line round.gox:309
		} else if s == api.RoundError {
//line round.gox:310
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("request failed"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:310
		return }), retry)); if __e != nil { return }
//line round.gox:311
		} else if s == api.RoundRefuse {
//line round.gox:312
			__e = __c.Any(statusBlock(warnMark(), gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("provider declined to answer"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:312
		return }), retry)); if __e != nil { return }
		}
	return })
//line round.gox:314
}

//line round.gox:316
func records(recs []api.Record) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:317
		if len(recs) == 1 {
//line round.gox:318
			__e = __c.Any(recordsBlock(gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Text("1 record"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:318
		return }))); if __e != nil { return }
//line round.gox:319
		} else if len(recs) > 1 {
//line round.gox:320
			__e = __c.Any(recordsBlock(gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line round.gox:320
				__e = __c.Any(len(recs)); if __e != nil { return }
				__e = __c.Text(" records"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line round.gox:320
		return }))); if __e != nil { return }
		}
	return })
//line round.gox:322
}

//line round.gox:324
func recordsBlock(text any) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:325
			__e = __c.Set("class", "kchat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:326
				__e = __c.Set("class", "kchat-gut"); if __e != nil { return }
//line round.gox:326
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:327
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line round.gox:327
					__e = __c.Any(icons.FloppyDisk()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:329
				__e = __c.Set("class", "kchat-col kchat-records"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:329
				__e = __c.Any(text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:331
}

//line round.gox:333
func statusBlock(mark gox.Elem, text any, retry gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:334
			__e = __c.Set("class", "kchat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:335
				__e = __c.Set("class", "kchat-gut"); if __e != nil { return }
//line round.gox:335
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:335
				__e = __c.Any(mark); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:336
				__e = __c.Set("class", "kchat-col"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:337
					__e = __c.Set("class", "kchat-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line round.gox:338
						__e = __c.Any(text); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line round.gox:339
					__e = __c.Any(retry); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:343
}

//line round.gox:345
func span() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:346
			__e = __c.Set("class", "kchat-span"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const span = document.currentScript.closest(\".kchat-span\")\n\t\t\tspan.scrollIntoView({ block: \"start\", behavior: \"smooth\" })\n\t\t\tlet seen = false\n\t\t\tconst observer = new IntersectionObserver(\n\t\t\t\t([entry]) => {\n\t\t\t\t\tif (entry.isIntersecting) {\n\t\t\t\t\t\tseen = true\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tif (!seen) return\n\t\t\t\t\tobserver.disconnect()\n\t\t\t\t\tspan.remove()\n\t\t\t\t},\n\t\t\t\t{ root: span.closest(\".kchat-log\") },\n\t\t\t)\n\t\t\tobserver.observe(span)\n\t\t\t$sys.clean(() => observer.disconnect())"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:367
}

//line round.gox:369
func bubble(input api.Input, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line round.gox:370
			__e = __c.Set("class", "kchat-bubble"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line round.gox:371
			if input.Text != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line round.gox:372
					__e = __c.Set("class", "kchat-bubble-text"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line round.gox:372
					__e = __c.Any(input.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line round.gox:374
			__e = __c.Any(files(input.Attachments, fs)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line round.gox:376
}

//line round.gox:378
func files(names []string, fs api.FS) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line round.gox:379
		if len(names) != 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line round.gox:380
				__e = __c.Set("class", "kchat-bubble-files"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line round.gox:381
				for _, name := range names {
//line round.gox:382
					__e = __c.Any(uploader.Sent{FS: fs, Name: name}); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line round.gox:386
}
