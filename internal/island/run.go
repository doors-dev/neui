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

package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
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
	"neui.dev/internal/precheck"
	"neui.dev/internal/replay"
)

var builtins = sync.OnceValues(func() ([]string, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, err
	}
	defer vm.Close()
	names, err := vm.Eval(`Object.getOwnPropertyNames(globalThis).join(" ")`, quickjs.EvalGlobal)
	if err != nil {
		return nil, err
	}
	list, _ := names.(string)
	return append(strings.Fields(list), "console", "print"), nil
})

func scriptModules(ctrl common.Controller) iter.Seq2[string, []common.Tool] {
	return func(yield func(string, []common.Tool) bool) {
		for m := range ctrl.Modules() {
			if m.Name() == "page" || m.Name() == "js" {
				continue
			}
			if !yield(m.Name(), m.List()) {
				return
			}
		}
	}
}

func scriptEnv(ctx context.Context, lib core.Library) (precheck.Env, error) {
	names, err := builtins()
	if err != nil {
		return precheck.Env{}, err
	}
	env := lib.PrecheckEnv()
	env.Globals = map[string]bool{}
	for _, name := range names {
		env.Globals[name] = true
	}
	if ctrl, ok := ctx.Value(common.KeyController).(common.Controller); ok {
		for module, list := range scriptModules(ctrl) {
			for _, t := range list {
				env.Globals[module+"__"+t.Name()] = true
			}
		}
	}
	return env, nil
}

func newRun(ctx context.Context, island *island, seq uint, space string, comment string, env precheck.Env) (*run, error) {
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
	ctx, cancel := context.WithCancelCause(ctrl.Context())
	r := &run{
		island:           island,
		seq:              seq,
		config:           config,
		notificationsCap: replay.RoundItemBudget(ctrl.Config(), ctrl.Model()) / 2,
		vm:               vm,
		fs:               ctrl.FS(),
		log:              log,
		ctx:              ctx,
		turnCtx:          ctx,
		cancel:           cancel,
		end:              make(chan struct{}),
		store:            doors.NewStore(),
		env:              env,
	}
	r.store.Save(common.IslandFSKey{}, ctrl.FS())
	r.store.Save(common.IslandDirKey{}, islandDir(space))
	r.store.Save(common.IslandFilesConfigKey{}, ctrl.FilesConfig())
	r.store.Save(common.IslandScopeKey{}, new(doors.ScopeBlocking))
	r.store.Save(common.IslandIDKey{}, island.id)
	r.runner = newRunner(r)
	r.store.Save(runnerKey{}, r.runner)
	r.state = core.NewState(r)
	for module, list := range scriptModules(ctrl) {
		if err := js.RegisterTools(vm, r.TurnContext, module+"__", list); err != nil {
			r.clean()
			return nil, err
		}
	}
	if err := island.lib.RegisterApp(r.TurnContext, vm); err != nil {
		r.clean()
		return nil, err
	}
	if err := r.state.RegisterTools(vm); err != nil {
		r.clean()
		return nil, err
	}
	if _, err := vm.Eval(core.UIJS, quickjs.EvalGlobal); err != nil {
		r.clean()
		return nil, err
	}
	task, err := ctrl.Task(comment, map[string]any{
		"path": space,
		"log":  log,
	}, cancel)
	if err != nil {
		r.clean()
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
	ctx              context.Context
	turnCtx          context.Context
	turnTimer        *time.Timer
	cancel           context.CancelCauseFunc
	state            core.State
	end              chan struct{}
	store            doors.Store
	runner           *runner
	mu               sync.Mutex
	queue            deque.Deque[eval]
	trigger          *chan struct{}
	handover         bool
	undrawn          bool
	env              precheck.Env
	globals          precheck.Globals
}

func (r *run) Store() doors.Store {
	return r.store
}

func (r *run) IslandContext() context.Context {
	return r.island.ctx
}

func (r *run) TurnContext() context.Context {
	return r.turnCtx
}

func (r *run) PauseTurn() {
	r.turnTimer.Stop()
}

func (r *run) ResumeTurn() {
	r.turnTimer.Reset(r.config.TurnTime)
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
	globals, err := precheckPart(code, r.env, r.globals)
	if err != nil {
		return 0, err
	}
	if interrupt {
		r.queue.Clear()
	} else if r.queue.Len() >= r.config.MaxQueue {
		return 0, errQueueFull
	}
	r.globals = globals
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

func (r *run) clean() {
	r.cancel(context.Canceled)
	r.vm.Close()
	r.state.Release()
}

func (r *run) run() {
	go func() {
		var result json.RawMessage
		defer func() {
			r.clean()
			r.island.ended(r.island.ctx, r.seq, r.task.ID(), r.endErr)
			if r.endErr != nil {
				r.task.Err(r.endErr)
			} else {
				r.task.Ok(result)
			}
			close(r.end)
		}()
		for {
			if r.ctx.Err() != nil {
				switch cause := context.Cause(r.ctx); {
				case errors.Is(cause, errStopped):
					result = stoppedResult
				case errors.Is(cause, errEnded):
					result, _ = json.Marshal(struct {
						Ended         string          `json:"ended"`
						Notifications json.RawMessage `json:"notifications,omitempty"`
					}{endedMessage, r.state.TakeNotifications()})
				case errors.Is(cause, errOvertaken):
					result = overtakenResult
				case errors.Is(cause, common.ErrRelinked):
					r.endErr = errRelinked
					if notifications := r.state.TakeNotifications(); notifications != nil {
						output, _ := json.Marshal(struct {
							Notifications json.RawMessage `json:"notifications"`
						}{notifications})
						r.endErr = common.ErrorWithOutput{Err: errRelinked, Output: output}
					}
				default:
					r.endErr = cause
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
			r.state.ForgetAction()
			r.current = eval.script
			r.runner.reset()
			r.mu.Unlock()
			turnCtx, cancelTurn := context.WithCancelCause(common.AttachThread(r.island.ctx, r.ctx))
			stopRun := context.AfterFunc(r.ctx, func() {
				cancelTurn(context.Cause(r.ctx))
			})
			r.turnCtx = turnCtx
			r.turnTimer = time.AfterFunc(r.config.TurnTime, func() {
				cancelTurn(fmt.Errorf("%s: %w", r.config.TurnTime, errTurnTime))
			})
			done := make(chan struct{})
			exited := make(chan struct{})
			stop := context.AfterFunc(turnCtx, func() {
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
			r.turnTimer.Stop()
			close(done)
			if !stop() {
				<-exited
			}
			if err != nil && errors.Is(context.Cause(turnCtx), errTurnTime) {
				err = context.Cause(turnCtx)
			}
			cancelTurn(nil)
			stopRun()
			switch {
			case r.ctx.Err() != nil:
			case err != nil && !r.isAlive():
				if logErr := js.Log(r.fs, r.log, "ERROR", err.Error()); logErr != nil {
					slog.Error("island: script log write failed", "log", r.log, "err", logErr)
				}
				r.state.FlushNotifications("")
				err = fmt.Errorf("script %d: %w", eval.script, err)
				r.endErr = err
				r.cancel(err)
				return
			case err != nil && !r.handover:
				if logErr := js.Log(r.fs, r.log, "ERROR", err.Error()); logErr != nil {
					slog.Error("island: script log write failed", "log", r.log, "err", logErr)
				}
				r.state.FlushNotifications("")
				r.mu.Lock()
				dropped := make([]uint, 0, r.queue.Len())
				for r.queue.Len() > 0 {
					dropped = append(dropped, r.queue.PopFront().script)
				}
				r.mu.Unlock()
				meta := map[string]any{"script": eval.script, "kind": "script_error", "note": scriptErrorNote}
				if action := r.state.LastAction(); action != "" {
					meta["last_action"] = action
					meta["note"] = scriptErrorActionNote
				}
				if !r.state.Drawn() {
					meta["note"] = meta["note"].(string) + "; " + undrawnErrorNote
					r.undrawn = true
				}
				if len(dropped) > 0 {
					meta["dropped"] = dropped
				}
				data, _ := json.Marshal(err.Error())
				r.task.Notify(data, meta)
			case err == nil:
				r.mu.Lock()
				idle := r.queue.Len() == 0
				r.mu.Unlock()
				note := ""
				if idle && r.state.Drawn() {
					note = fmt.Sprintf(idleNote, eval.script)
				}
				r.state.FlushNotifications(note)
				if idle && !r.undrawn && !r.state.Drawn() {
					r.undrawn = true
					r.Note(undrawnNote)
				}
			}
			if r.island.entered.Load() > 0 {
				r.state.Release()
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
	vmCtx := *(*unsafe.Pointer)(unsafe.Pointer(r.vm))
	(*libquickjs.TJSContext)(vmCtx).Finterrupt_counter = 1
}

func (r *run) Notify(script uint, data json.RawMessage, note string) {
	meta := map[string]any{"script": script, "kind": "notifications_list"}
	if note != "" {
		meta["note"] = note
	}
	r.task.Notify(data, meta)
}

func (r *run) Note(note string) {
	data, _ := json.Marshal(note)
	go r.task.Notify(data, map[string]any{"kind": "script_note", "script": r.current})
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
