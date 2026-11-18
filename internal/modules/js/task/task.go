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

package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"neui.dev/internal/common"
)

type Params struct {
	Name       string
	Script     string
	Comment    string
	Timeout    time.Duration
	Memory     int
	AppendLogs bool
	Controller common.Controller
}

type Input struct {
	Params
	Context context.Context
}

const interruptTickDuration = 10 * time.Millisecond

var ErrTimeout = errors.New("the script ran past its time limit and was stopped; tool calls in flight were cancelled; pass a larger timeoutMs or split the work")

func NewTask(params Params, done func(), asyncAfter time.Duration) (WaitingTask, <-chan Result, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(params.Controller.Context())
	task := &task{
		task: Input{
			Params: params,
		},
		name:       params.Name,
		memory:     params.Memory,
		timeout:    params.Timeout,
		ctx:        ctx,
		cancel:     cancel,
		asyncAfter: asyncAfter,
		interrupt:  make(chan func()),
		res:        make(chan string),
		rej:        make(chan error),
		done:       done,
	}
	return task, task.start(), cancel
}

type Result struct {
	Ok    string
	Async bool
	Err   error
}

type task struct {
	task       Input
	name       string
	memory     int
	timeout    time.Duration
	ctx        context.Context
	cancel     context.CancelCauseFunc
	exec       context.Context
	cancelExec context.CancelFunc
	asyncAfter time.Duration
	interrupt  chan func()
	res        chan string
	rej        chan error
	done       func()
}

type WaitingTask interface {
	Name() string
	Memory() int
	Done() <-chan struct{}
	Reject(err error)
	Execute(interrupt func()) (ExecutingTask, bool)
}

type ExecutingTask interface {
	Input() Input
	Ok(output string)
	Err(err error)
}

func (t *task) Execute(interrupt func()) (ExecutingTask, bool) {
	t.exec, t.cancelExec = context.WithTimeoutCause(t.ctx, t.timeout, ErrTimeout)
	select {
	case t.interrupt <- interrupt:
		close(t.interrupt)
		return t, true
	case <-t.ctx.Done():
		t.cancelExec()
		return nil, false
	}
}

func (t *task) Name() string {
	return t.name
}

func (t *task) Memory() int {
	return t.memory
}

func (t *task) Done() <-chan struct{} {
	return t.ctx.Done()
}

func (t *task) Reject(err error) {
	select {
	case t.rej <- err:
		close(t.rej)
		close(t.res)
	case <-t.ctx.Done():
	}
}

func (t *task) Input() Input {
	input := t.task
	input.Context = t.exec
	return input
}

func (t *task) Err(err error) {
	t.rej <- err
	close(t.rej)
	close(t.res)

}

func (t *task) Ok(output string) {
	t.res <- output
	close(t.rej)
	close(t.res)
}

func (t *task) start() <-chan Result {
	ch := make(chan Result, 1)
	go t.waitSync(ch)
	return ch
}

func (t *task) waitSync(ch chan<- Result) {
	defer t.cancel(nil)
	asyncAfter := time.After(t.asyncAfter)
	select {
	case <-t.ctx.Done():
		t.task = Input{}
		t.done()
		ch <- Result{Err: context.Cause(t.ctx)}
	case err := <-t.rej:
		t.task = Input{}
		t.done()
		ch <- Result{Err: err}
	case f := <-t.interrupt:
		t.executeSync(ch, asyncAfter, f)
	case <-asyncAfter:
		task, err := t.task.Controller.Task(t.task.Comment, nil, t.cancel)
		if err != nil {
			t.cancel(err)
			t.done()
			ch <- Result{Err: err}
			return
		}
		ch <- Result{Async: true}
		t.waitAsync(task)
	}
}

func (t *task) waitAsync(task common.Task) {
	select {
	case <-t.ctx.Done():
		t.done()
		task.Err(context.Cause(t.ctx))
	case err := <-t.rej:
		t.done()
		task.Err(err)
	case f := <-t.interrupt:
		t.executeAsync(f, task)
	}
}

func (t *task) executeSync(ch chan<- Result, asyncAfter <-chan time.Time, interrupt func()) {
	defer t.cancelExec()
	done := t.exec.Done()
	var handle common.Task
	var interruptTick <-chan time.Time
repeat:
	select {
	case <-done:
		done = nil
		asyncAfter = nil
		interrupt()
		interruptTick = time.Tick(interruptTickDuration)
		goto repeat
	case <-interruptTick:
		interrupt()
		goto repeat
	case <-asyncAfter:
		asyncAfter = nil
		task, err := t.task.Controller.Task(t.task.Comment, nil, t.cancel)
		if err != nil {
			ch <- Result{Err: err}
			ch = nil
			t.cancel(err)
			goto repeat
		}
		handle = task
		ch <- Result{Async: true}
		ch = nil
		goto repeat
	case ok := <-t.res:
		t.done()
		if ch != nil {
			ch <- Result{Ok: ok}
			return
		}
		if handle != nil {
			handle.Ok(json.RawMessage(ok))
		}
	case err := <-t.rej:
		t.done()
		if t.ctx.Err() != nil {
			err = context.Cause(t.ctx)
		}
		if ch != nil {
			ch <- Result{Err: err}
			return
		}
		if handle != nil {
			handle.Err(err)
		}
	}
}

func (t *task) executeAsync(interrupt func(), task common.Task) {
	defer t.cancelExec()
	done := t.exec.Done()
	var interruptTick <-chan time.Time
repeat:
	select {
	case <-done:
		done = nil
		interrupt()
		interruptTick = time.Tick(interruptTickDuration)
		goto repeat
	case <-interruptTick:
		interrupt()
		goto repeat
	case output := <-t.res:
		t.done()
		task.Ok(json.RawMessage(output))
	case err := <-t.rej:
		t.done()
		if t.ctx.Err() != nil {
			err = context.Cause(t.ctx)
		}
		task.Err(err)
	}
}
