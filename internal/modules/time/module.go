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

package timemod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const layout = "2006-01-02T15:04:05.000Z07:00"
const syncAfter = 10 * time.Second
const clientTimeout = 10 * time.Second
const maxDelay = 24 * time.Hour

var errCancelled = errors.New("cancelled by time__cancel")

const moduleDesc = `The clock: read the date and time here, never assume them. The server's clock and the user's browser clock may differ in offset and in wall time.`
const nowDesc = `Current time of the server, for anything the server timestamps.`
const clientDesc = `Current time of the user's browser, the one to quote when talking to the user.`

func timerDesc(tier common.ModelTier) string {
	desc := `Wait until a moment, to continue work then, not to pause the conversation. Pass exactly one of afterMs and at. A wait of up to 10 s returns when the moment arrives; a longer one returns at once, deferred. One deferred timer at a time; short waits always work.`
	if tier != common.TierAssistant {
		desc += ` From a script it never defers: it blocks, within the script's time limit.`
	}
	return desc
}

const cancelDesc = `Drop the pending deferred timer; its task_result reports the cancellation.`

func Module(ctx context.Context) common.Module {
	m := tools.NewModule("time", moduleDesc)
	t := &module{}
	if _, err := m.Add("now", nowDesc, t.now); err != nil {
		panic(err)
	}
	if _, err := m.Add("client", clientDesc, t.client); err != nil {
		panic(err)
	}
	if _, err := m.Add("timer", timerDesc(common.Tier(ctx)), t.timer); err != nil {
		panic(err)
	}
	if _, err := m.Add("cancel", cancelDesc, t.cancel); err != nil {
		panic(err)
	}
	return m
}

type module struct {
	pending atomic.Pointer[pending]
}

type pending struct {
	comment string
	cancel  context.CancelCauseFunc
	done    chan struct{}
	claimed atomic.Bool
}

type NowResult struct {
	Time   string `json:"time" format:"date-time"`
	UTC    string `json:"utc" format:"date-time"`
	Zone   string `json:"zone" description:"abbreviation such as CET, else the offset"`
	Offset int    `json:"offsetMinutes" description:"ahead of UTC"`
}

func (m *module) now(ctx context.Context, _ struct{}) (NowResult, error) {
	t := time.Now()
	zone, off := t.Zone()
	if zone == "" {
		zone = t.Format("-07:00")
	}
	return NowResult{
		Time:   t.Format(layout),
		UTC:    t.UTC().Format(layout),
		Zone:   zone,
		Offset: off / 60,
	}, nil
}

type ClientResult struct {
	Time   string `json:"time" format:"date-time"`
	UTC    string `json:"utc" format:"date-time"`
	Offset int    `json:"offsetMinutes" description:"ahead of UTC"`
	Skew   int64  `json:"skewMs" description:"the browser's clock minus the server's, round trip included; a large value means the user's clock is off"`
}

func (m *module) client(ctx context.Context, _ struct{}) (ClientResult, error) {
	callCtx, cancel := context.WithTimeout(common.GetController(ctx).PageContext(), clientTimeout)
	defer cancel()
	var now time.Time
	sent := time.Now()
	ch := doors.Call(callCtx, doors.ActionTime{}.Into(&now))
	for range 2 {
		select {
		case <-ctx.Done():
			return ClientResult{}, context.Cause(ctx)
		case err := <-ch:
			switch {
			case errors.Is(err, context.Canceled) && errors.Is(context.Cause(callCtx), context.DeadlineExceeded):
				return ClientResult{}, errors.New("the browser did not answer in time: the page may be closed, hidden or offline; use time__now for the server clock")
			case errors.Is(err, context.Canceled):
				return ClientResult{}, errors.New("the page link is gone: retry, or use time__now for the server clock")
			case err != nil:
				return ClientResult{}, err
			}
		}
	}
	_, off := now.Zone()
	return ClientResult{
		Time:   now.Format(layout),
		UTC:    now.UTC().Format(layout),
		Offset: off / 60,
		Skew:   now.Sub(sent).Milliseconds(),
	}, nil
}

type TimerArg struct {
	After   uint   `json:"afterMs"`
	At      string `json:"at" description:"RFC 3339 instant, e.g. 2026-09-28T16:00:00+02:00, by the server's clock"`
	Comment string `json:"comment" required:"true" description:"why you are waiting, in a few words; shown to the user while a deferred timer is pending, and carried by its task_result"`
}

type TimerResult struct {
	Async bool   `json:"deferred,omitempty" description:"the wait goes on in the background, and when the moment arrives a <task_result> element starts a round"`
	Fires string `json:"firesAt" format:"date-time"`
	Fired string `json:"firedAt,omitempty" format:"date-time"`
}

func (m *module) timer(ctx context.Context, arg TimerArg) (TimerResult, error) {
	if (arg.After != 0) == (arg.At != "") {
		return TimerResult{}, errors.New("pass exactly one of afterMs or at")
	}
	if arg.After > uint(maxDelay.Milliseconds()) {
		return TimerResult{}, fmt.Errorf("a timer waits at most %d hours from now", int(maxDelay/time.Hour))
	}
	now := time.Now()
	fires := now.Add(time.Duration(arg.After) * time.Millisecond)
	requested := fmt.Sprintf("afterMs=%d", arg.After)
	if arg.At != "" {
		at, err := time.Parse(time.RFC3339, arg.At)
		if err != nil {
			return TimerResult{}, errors.New("at must be an RFC 3339 instant, e.g. 2026-09-28T16:00:00+02:00")
		}
		fires = at
		requested = "at=" + arg.At
	}
	delay := fires.Sub(now)
	if delay <= 0 {
		return TimerResult{}, errors.New("that moment has already passed")
	}
	if delay > maxDelay {
		return TimerResult{}, fmt.Errorf("a timer waits at most %d hours from now", int(maxDelay/time.Hour))
	}
	if delay <= syncAfter || tools.FromJS(ctx) {
		if deadline, ok := ctx.Deadline(); ok && tools.FromJS(ctx) && fires.After(deadline) {
			return TimerResult{}, fmt.Errorf("%s is past the script's time limit, which ends at %s: call time__timer outside the script to wait that long", fires.Format(layout), deadline.Format(layout))
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return TimerResult{}, context.Cause(ctx)
		case <-timer.C:
			return TimerResult{Fires: fires.Format(layout), Fired: time.Now().Format(layout)}, nil
		}
	}
	thread := common.GetController(ctx)
	waitCtx, cancel := context.WithCancelCause(thread.Context())
	p := &pending{comment: arg.Comment, cancel: cancel, done: make(chan struct{})}
	for !m.pending.CompareAndSwap(nil, p) {
		if q := m.pending.Load(); q != nil {
			cancel(nil)
			return TimerResult{}, fmt.Errorf("a timer is already pending (%s): time__cancel it or wait for it to fire", q.comment)
		}
	}
	task, err := thread.Task(arg.Comment, map[string]any{
		"requested": requested,
		"firesAt":   fires.Format(layout),
	}, cancel)
	if err != nil {
		cancel(err)
		m.pending.CompareAndSwap(p, nil)
		close(p.done)
		return TimerResult{}, err
	}
	go m.wait(waitCtx, p, task, fires)
	return TimerResult{Async: true, Fires: fires.Format(layout)}, nil
}

func (m *module) wait(ctx context.Context, p *pending, task common.Task, fires time.Time) {
	defer close(p.done)
	defer m.pending.CompareAndSwap(p, nil)
	timer := time.NewTimer(time.Until(fires))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		task.Err(context.Cause(ctx))
	case <-timer.C:
		if !p.claimed.CompareAndSwap(false, true) {
			<-ctx.Done()
			task.Err(context.Cause(ctx))
			return
		}
		fired := time.Now()
		out, _ := json.Marshal(map[string]any{
			"firesAt": fires.Format(layout),
			"firedAt": fired.Format(layout),
			"lateMs":  fired.Sub(fires).Milliseconds(),
		})
		m.pending.CompareAndSwap(p, nil)
		task.Ok(out)
	}
}

func (m *module) cancel(ctx context.Context, _ struct{}) (struct{}, error) {
	p := m.pending.Load()
	if p == nil {
		return struct{}{}, errors.New("no timer is pending")
	}
	if !p.claimed.CompareAndSwap(false, true) {
		return struct{}{}, errors.New("the timer is already ending: its task_result reports the outcome")
	}
	m.pending.CompareAndSwap(p, nil)
	p.cancel(errCancelled)
	select {
	case <-p.done:
		return struct{}{}, nil
	case <-ctx.Done():
		return struct{}{}, context.Cause(ctx)
	}
}
