package task

import (
	"context"
	"encoding/json"
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

func NewTask(params Params, done func(), asyncAfter time.Duration) (WaitingTask, <-chan Result, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(params.Controller.Context())
	task := &task{
		task: Input{
			Params:  params,
			Context: ctx,
		},
		memory:     params.Memory,
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
	memory     int
	ctx        context.Context
	cancel     context.CancelCauseFunc
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
	select {
	case t.interrupt <- interrupt:
		close(t.interrupt)
		return t, true
	case <-t.ctx.Done():
		return nil, false
	}
}

func (t *task) Name() string {
	return t.task.Name
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
	return t.task
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
	asyncAfter := time.After(t.asyncAfter)
	select {
	case <-t.ctx.Done():
		ch <- Result{Err: context.Cause(t.ctx)}
		t.task = Input{}
		t.done()
	case err := <-t.rej:
		ch <- Result{Err: err}
		t.task = Input{}
		t.done()
	case f := <-t.interrupt:
		t.executeSync(ch, asyncAfter, f)
	case <-asyncAfter:
		task, err := t.task.Controller.Task(t.task.Comment, nil, t.cancel)
		if err != nil {
			t.cancel(err)
			ch <- Result{Err: err}
			t.done()
			return
		}
		ch <- Result{Async: true}
		t.waitAsync(task)
	}
}

func (t *task) waitAsync(task common.Task) {
	select {
	case <-t.ctx.Done():
		task.Err(context.Cause(t.ctx))
		t.done()
	case err := <-t.rej:
		task.Err(err)
		t.done()
	case f := <-t.interrupt:
		t.executeAsync(f, task)
	}
}

func (t *task) executeSync(ch chan<- Result, asyncAfter <-chan time.Time, interrupt func()) {
	defer t.done()
	done := t.ctx.Done()
	var handle common.Task
	canceled := false
	var interruptTick <-chan time.Time
repeat:
	select {
	case <-done:
		done = nil
		asyncAfter = nil
		interrupt()
		interruptTick = time.Tick(interruptTickDuration)
		canceled = true
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
		if ch != nil {
			ch <- Result{Ok: ok}
			return
		}
		if handle != nil {
			handle.Ok(json.RawMessage(ok))
		}
	case err := <-t.rej:
		if ch != nil {
			if canceled {
				ch <- Result{Err: context.Cause(t.ctx)}
				return
			}
			ch <- Result{Err: err}
			return
		}
		if handle == nil {
			return
		}
		if canceled {
			handle.Err(context.Cause(t.ctx))
			return
		}
		handle.Err(err)
	}
}

func (t *task) executeAsync(interrupt func(), task common.Task) {
	defer t.done()
	done := t.ctx.Done()
	canceled := false
	var interruptTick <-chan time.Time
repeat:
	select {
	case <-done:
		done = nil
		interrupt()
		interruptTick = time.Tick(interruptTickDuration)
		canceled = true
		goto repeat
	case <-interruptTick:
		interrupt()
		goto repeat
	case output := <-t.res:
		task.Ok(json.RawMessage(output))
	case err := <-t.rej:
		if canceled {
			task.Err(context.Cause(t.ctx))
			return
		}
		task.Err(err)
	}
}
