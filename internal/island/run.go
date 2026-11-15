package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/gammazero/deque"
	"modernc.org/libquickjs"
	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/island/core"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/replay"
	"neui.dev/internal/tools"
)

func newRun(ctx context.Context, island *island, seq uint, space string, comment string) (*run, error) {
	ctrl := common.GetController(ctx)
	log := logFile(space)
	config := common.AppIslandConfig(ctrl.Context())
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, err
	}
	vm.SetMemoryLimit(uintptr(config.Memory))
	vm.SetMaxStackSize(uintptr(config.MaxCallDepth))
	if err := js.RegisterSerializers(vm); err != nil {
		vm.Close()
		return nil, err
	}
	if err := js.ClearLog(ctrl.FS(), log); err != nil {
		vm.Close()
		return nil, err
	}
	if err := ctrl.FS().Remove(ctx, islandDir(space)); err != nil {
		vm.Close()
		return nil, err
	}
	if err := js.RegisterLoggers(vm, ctrl.FS(), log); err != nil {
		vm.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancelCause(tools.WithRun(ctrl.Context()))
	r := &run{
		island:           island,
		seq:              seq,
		config:           config,
		notificationsCap: replay.RoundItemBudget(ctrl.Config(), ctrl.Model()) / 2,
		vm:               vm,
		fs:               ctrl.FS(),
		log:              log,
		pageContext:      ctrl.PageContext(),
		ctx:              ctx,
		exec:             ctx,
		cancel:           cancel,
		end:              make(chan struct{}),
		store:            doors.NewStore(),
	}
	r.store.Save(common.IslandFSKey{}, ctrl.FS())
	r.store.Save(common.IslandDirKey{}, islandDir(space))
	r.store.Save(common.IslandFilesConfigKey{}, ctrl.FilesConfig())
	r.store.Save(common.IslandScopeKey{}, new(doors.ScopeBlocking))
	r.runner = newRunner(r)
	r.store.Save(runnerKey{}, r.runner)
	r.state = core.NewState(r)
	for m := range ctrl.Modules() {
		if err := js.RegisterTools(vm, r.TurnContext, m.Name()+"__", m.List()); err != nil {
			r.clean(false)
			return nil, err
		}
	}
	if err := island.lib.RegisterApp(r.TurnContext, vm); err != nil {
		r.clean(false)
		return nil, err
	}
	if err := r.state.RegisterTools(vm); err != nil {
		r.clean(false)
		return nil, err
	}
	if _, err := vm.Eval(core.UIJS, quickjs.EvalGlobal); err != nil {
		r.clean(false)
		return nil, err
	}
	task, err := ctrl.Task(comment, map[string]any{
		"path": space,
		"log":  log,
	}, cancel)
	if err != nil {
		r.clean(false)
		return nil, err
	}
	r.task = task
	go r.runner.run()
	r.run()
	return r, nil
}

type eval struct {
	code      string
	script    uint
	interrupt bool
}

type run struct {
	island           *island
	seq              uint
	endErr           error
	notificationsCap int
	config           api.IslandConfig
	vm               *quickjs.VM
	fs               api.FS
	log              string
	task             common.Task
	scripts          uint
	current          uint
	pageContext      context.Context
	ctx              context.Context
	exec             context.Context
	turn             *time.Timer
	cancel           context.CancelCauseFunc
	state            core.State
	end              chan struct{}
	store            doors.Store
	runner           *runner
	mu               sync.Mutex
	queue            deque.Deque[eval]
	trigger          *chan struct{}
	handover         bool
}

func (r *run) Store() doors.Store {
	return r.store
}

func (r *run) PageContext() context.Context {
	return r.pageContext
}

func (r *run) Context() context.Context {
	return r.ctx
}

func (r *run) TurnContext() context.Context {
	return r.exec
}

func (r *run) PauseTurn() {
	r.turn.Stop()
}

func (r *run) ResumeTurn() {
	r.turn.Reset(r.config.TurnTime)
}

func (r *run) Seed(ctx context.Context, root gox.Comp) {
	r.island.updateScreen(ctx, screen{content: r.island.template(root), store: r.store})
}

func (r *run) eval(code string, interrupt bool) (uint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx.Err() != nil {
		return 0, context.Cause(r.ctx)
	}
	if interrupt {
		r.queue.Clear()
	} else if r.queue.Len() >= r.config.MaxQueue {
		return 0, errQueueFull
	}
	r.scripts++
	script := r.scripts
	r.queue.PushBack(eval{
		code:      code,
		script:    script,
		interrupt: interrupt,
	})
	if interrupt {
		r.state.ArmHandover()
	}
	if r.trigger != nil {
		close(*r.trigger)
		r.trigger = nil
	}
	return script, nil
}

func (r *run) AckHandover() {
	r.interrupt()
	r.handover = true
}

func (r *run) resetHandover() {
	r.handover = false
	r.state.ResetHandover()
}

func (r *run) done() <-chan struct{} {
	r.cancel(errStopped)
	return r.end
}

func (r *run) End() {
	r.cancel(errEnded)
}

func (r *run) overtake() <-chan struct{} {
	r.cancel(errOvertaken)
	return r.end
}

func (r *run) unmount() {
	r.cancel(errUnmounted)
}

func (r *run) clean(end bool) {
	r.cancel(context.Canceled)
	r.vm.Close()
	r.state.Release()
	close(r.end)
	if end {
		r.island.ended(r.pageContext, r.seq, r.task.ID(), r.endErr)
	}
}

func (r *run) run() {
	go func() {
		defer r.clean(true)
		for {
			if r.ctx.Err() != nil {
				switch cause := context.Cause(r.ctx); {
				case errors.Is(cause, errStopped):
					r.task.Ok(stoppedResult)
				case errors.Is(cause, errEnded):
					r.task.Ok(endedResult)
				case errors.Is(cause, errOvertaken):
					r.task.Ok(overtakenResult)
				default:
					r.endErr = cause
					r.task.Err(cause)
				}
				return
			}
			r.mu.Lock()
			if r.queue.Len() == 0 {
				ch := make(chan struct{})
				r.trigger = &ch
				r.mu.Unlock()
				select {
				case <-ch:
				case <-r.ctx.Done():
				}
				continue
			}
			eval := r.queue.PopFront()
			r.resetHandover()
			r.current = eval.script
			r.runner.reset()
			r.mu.Unlock()
			exec, cancelExec := context.WithCancelCause(r.ctx)
			r.exec = exec
			r.turn = time.AfterFunc(r.config.TurnTime, func() {
				cancelExec(fmt.Errorf("%s: %w", r.config.TurnTime, errTurnTime))
			})
			done := make(chan struct{})
			exited := make(chan struct{})
			stop := context.AfterFunc(exec, func() {
				defer close(exited)
				for {
					r.interrupt()
					select {
					case <-done:
						return
					case <-time.After(20 * time.Millisecond):
					}
				}
			})
			err := r.execute(eval.code)
			r.turn.Stop()
			close(done)
			if !stop() {
				<-exited
			}
			if err != nil && errors.Is(context.Cause(exec), errTurnTime) {
				err = context.Cause(exec)
			}
			cancelExec(nil)
			switch {
			case r.ctx.Err() != nil:
			case err != nil && !r.isAlive():
				if logErr := js.Log(r.fs, r.log, "ERROR", err.Error()); logErr != nil {
					slog.Error("island: script log write failed", "log", r.log, "err", logErr)
				}
				r.state.FlushNotifications(false)
				err = fmt.Errorf("script %d: %w", eval.script, err)
				r.endErr = err
				r.cancel(err)
				r.task.Err(err)
				return
			case err != nil && !r.handover:
				if logErr := js.Log(r.fs, r.log, "ERROR", err.Error()); logErr != nil {
					slog.Error("island: script log write failed", "log", r.log, "err", logErr)
				}
				r.state.FlushNotifications(false)
				r.mu.Lock()
				dropped := make([]uint, 0, r.queue.Len())
				for r.queue.Len() > 0 {
					dropped = append(dropped, r.queue.PopFront().script)
				}
				r.mu.Unlock()
				meta := map[string]any{"script": eval.script, "kind": "script_error"}
				if len(dropped) > 0 {
					meta["dropped"] = dropped
				}
				data, _ := json.Marshal(err.Error())
				r.task.Notify(data, meta)
			case err == nil:
				r.state.FlushNotifications(false)
			}
		}
	}()
}

func (r *run) isAlive() bool {
	_, err := r.vm.Eval(`0`, quickjs.EvalGlobal)
	return err == nil
}

func (r *run) execute(code string) error {
	_, err := r.vm.Eval(code, quickjs.EvalGlobal)
	if err == nil {
		return nil
	}
	var qe *quickjs.Error
	if errors.As(err, &qe) && qe.Stack != "" {
		return fmt.Errorf("%w\n%s", err, strings.TrimRight(qe.Stack, "\n"))
	}
	return err
}

func (r *run) Rearm() error {
	r.ResumeTurn()
	_, err := r.vm.ExecutePendingJobs()
	return err
}

func (r *run) GC() error {
	if _, err := r.vm.ExecutePendingJobs(); err != nil {
		return err
	}
	r.vm.SetGCThreshold(1)
	v, err := r.vm.NewObjectValue()
	if err != nil {
		return err
	}
	v.Free()
	_, err = r.vm.ExecutePendingJobs()
	return err
}

func (r *run) Config() api.IslandConfig {
	return r.config
}

func (r *run) Library() core.Library {
	return r.island.lib
}

func (r *run) interrupt() {
	r.vm.Interrupt()
	vmCtx := *(*uintptr)(unsafe.Pointer(r.vm))
	(*libquickjs.TJSContext)(unsafe.Pointer(vmCtx)).Finterrupt_counter = 1
}

func (r *run) Notify(script uint, data json.RawMessage, idle bool) {
	meta := map[string]any{"script": script, "kind": "notifications_list"}
	if idle {
		meta["note"] = idleNote
	}
	r.task.Notify(data, meta)
}

func (r *run) NotificationsCap() int {
	return r.notificationsCap
}

func (r *run) Script() uint {
	return r.current
}

var _ core.Run = (*run)(nil)

func logFile(space string) string {
	return space + ".island.log"
}

func islandDir(space string) string {
	return space + ".island"
}
