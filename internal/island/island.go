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
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"modernc.org/libquickjs"
	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/island/core"
	"neui.dev/internal/precheck"
)

type Library = core.Library

func NewLibrary() Library {
	return core.NewLibrary()
}

type Island = *island

func NewIsland(
	ctx context.Context,
	placeholder gox.Elem,
	template func(slot gox.Comp) gox.Elem,
	success func(close func(ctx context.Context)) gox.Elem,
	failure func(close func(ctx context.Context), err error) gox.Elem,
	lib Library,
) Island {
	island := &island{
		ctx:         ctx,
		id:          doors.IDRand(),
		placeholder: placeholder,
		template:    template,
		success:     success,
		failure:     failure,
		screen:      new(doors.Door),
		status:      new(doors.Door),
		lib:         lib,
	}
	island.updateStatusScreen(context.Background(), noAppStatus, placeholder)
	return island
}

type island struct {
	ctx         context.Context
	mu          sync.Mutex
	id          string
	placeholder gox.Elem
	template    func(slot gox.Comp) gox.Elem
	success     func(close func(ctx context.Context)) gox.Elem
	failure     func(close func(ctx context.Context), err error) gox.Elem
	lib         Library
	unmounted   bool
	screen      *doors.Door
	status      *doors.Door
	run         *run
	seq         uint
	entered     atomic.Int32
}

func (i *island) Spec(ctx context.Context, space string) (string, []int, []string) {
	b := strings.Builder{}
	fmt.Fprintf(&b, "Logs and files\nconsole.log, info, warn and error append to the workspace file %s, which is emptied when an app starts. Views and controls keep their files, such as uploads, under the workspace dir %s, also emptied when an app starts: copy the files you want to keep elsewhere in the workspace.\n", logFile(space), islandDir(space))
	logs := b.Len()
	b.WriteString(specReference)
	spec, ends, items := i.lib.Spec()
	start := b.Len()
	b.WriteString(spec)
	all := []int{logs, start}
	for _, end := range ends {
		all = append(all, start+end)
	}
	return b.String(), all, append([]string{"logs and files", "reading the reference"}, items...)
}

func (i *island) Enter() {
	i.entered.Add(1)
}

func (i *island) Leave() {
	i.entered.Add(-1)
}

func (i *island) Unmount() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.unmounted = true
	i.seq++
	if i.run == nil {
		return
	}
	i.run.unmount()
	i.run = nil
}

func (i *island) StartFile(ctx context.Context, space string, comment string, file string) (uint, uint, error) {
	code, err := readFile(ctx, file)
	if err != nil {
		return 0, 0, err
	}
	return i.StartCode(ctx, space, comment, code)
}

func (i *island) StartCode(ctx context.Context, space string, comment string, code string) (uint, uint, error) {
	if err := check(code); err != nil {
		return 0, 0, err
	}
	env, err := scriptEnv(ctx, i.lib)
	if err != nil {
		return 0, 0, err
	}
	if _, err := precheckPart(code, env, precheck.Globals{}); err != nil {
		return 0, 0, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.unmounted {
		return 0, 0, errUnmounted
	}
	overtake := i.run != nil
	i.seq++
	seq := i.seq
	if overtake {
		old := i.run
		ch := old.overtake()
		i.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			go func() {
				select {
				case <-ch:
				case <-i.ctx.Done():
					return
				}
				i.mu.Lock()
				defer i.mu.Unlock()
				if i.seq != seq {
					return
				}
				i.run = nil
				i.updateStatusScreen(i.ctx, noAppStatus, i.placeholder)
			}()
			i.mu.Lock()
			return 0, 0, context.Cause(ctx)
		}
		i.mu.Lock()
		if seq != i.seq {
			if i.unmounted {
				return 0, 0, errUnmounted
			}
			return 0, 0, errOvertaken
		}
		i.run = nil
	}
	run, err := newRun(ctx, i, i.seq, space, comment, env)
	if err != nil {
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
		return 0, 0, err
	}
	script, err := run.eval(code, false)
	if err != nil {
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
		return 0, 0, err
	}
	i.run = run
	i.updateStatus(ctx, runningStatus(run.task.ID()))
	return i.run.task.ID(), script, nil
}

func (i *island) ContinueFile(ctx context.Context, file string, interrupt bool) (uint, uint, error) {
	code, err := readFile(ctx, file)
	if err != nil {
		return 0, 0, err
	}
	return i.ContinueCode(code, interrupt)
}

func (i *island) ContinueCode(code string, interrupt bool) (uint, uint, error) {
	if err := check(code); err != nil {
		return 0, 0, err
	}
	i.mu.Lock()
	if i.unmounted {
		i.mu.Unlock()
		return 0, 0, errUnmounted
	}
	run := i.run
	i.mu.Unlock()
	if run == nil {
		return 0, 0, errNothingToContinue
	}
	script, err := run.eval(code, interrupt)
	if err != nil {
		return 0, 0, err
	}
	return run.task.ID(), script, nil
}

func (i *island) ended(ctx context.Context, seq uint, task uint, err error) {
	i.mu.Lock()
	if i.seq != seq {
		i.mu.Unlock()
		return
	}
	i.run = nil
	i.seq++
	newSeq := i.seq
	dismiss := func(ctx context.Context) {
		i.mu.Lock()
		defer i.mu.Unlock()
		if i.seq != newSeq {
			return
		}
		i.seq++
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
	}
	settled := make(chan struct{})
	doors.OnSettle(ctx, func(context.Context) {
		close(settled)
	}, func(ctx context.Context) {
		switch {
		case errors.Is(err, api.ErrCanceledByUser):
			i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
		case err == nil && i.success != nil:
			i.updateStatusScreen(ctx, endedStatus(task), i.success(dismiss))
		case err != nil && i.failure != nil:
			i.updateStatusScreen(ctx, failedStatus(task, err), i.failure(dismiss, err))
		default:
			i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
		}
	})
	i.mu.Unlock()
	<-settled
}

func (i *island) updateStatus(ctx context.Context, s string) {
	i.status.Outer(ctx, status(s))
}

func (i *island) updateScreen(ctx context.Context, s screen) {
	i.screen.Outer(ctx, s)
}

func (i *island) updateStatusScreen(ctx context.Context, s string, content gox.Elem) {
	i.updateStatus(ctx, s)
	i.updateScreen(ctx, screen{content: content})
}

func readFile(ctx context.Context, file string) (string, error) {
	rc, err := common.GetController(ctx).FS().ReadAll(ctx, file)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	code, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(code), nil
}

func (i *island) Done(ctx context.Context) error {
	i.mu.Lock()
	if i.run == nil {
		i.mu.Unlock()
		return errNothingToStop
	}
	ch := i.run.done()
	i.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-ch:
	}
	return nil
}

type status string

func (s status) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		ctx := cur.Context()
		return getSpace(ctx).AddData(ctx, "status", string(s))
	}
}

type screen struct {
	content gox.Elem
	store   doors.Store
}

func (s screen) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		if s.store == nil {
			return cur.Comp(s.content)
		}
		ctx := cur.Context()
		return doors.Ctx(context.WithValue(ctx, common.KeyIslandStore, s.store)).Proxy(cur, s.content)
	}
}

func (i *island) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		space := getSpace(cur.Context())
		for _, t := range core.Tools(i.lib) {
			if err := space.AddTool(cur.Context(), t); err != nil {
				return err
			}
		}
		ctx, err := space.AddSpace(cur.Context(), screenSpaceName, screenSpaceDesc)
		if err != nil {
			return err
		}
		return doors.Ctx(ctx).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(i.status); err != nil {
				return err
			}
			return doors.ProxyMod(gox.ModifyFunc(func(_ context.Context, _ string, attrs gox.Attrs) error {
				attrs.Get(common.IslandIDAttr).Set(i.id)
				return nil
			})).Proxy(cur, func(cur gox.Cursor) error {
				return cur.Comp(i.screen)
			})
		})
	}
}

func check(code string) error {
	if err := checkSyntax(code); err != nil {
		return fmt.Errorf("%w: %w", err, errScriptRejected)
	}
	return nil
}

func precheckPart(code string, env precheck.Env, globals precheck.Globals) (precheck.Globals, error) {
	findings, globals := precheck.Check(code, env, globals)
	if len(findings) == 0 {
		return globals, nil
	}
	errs := make([]error, len(findings))
	for i, f := range findings {
		errs[i] = f
	}
	return globals, fmt.Errorf("%w: %w", errors.Join(errs...), errScriptRejected)
}

func checkSyntax(code string) error {
	vm, err := quickjs.NewVM()
	if err != nil {
		return err
	}
	defer vm.Close()
	v, err := vm.EvalValue(code, quickjs.EvalGlobal|libquickjs.MJS_EVAL_FLAG_COMPILE_ONLY)
	if err == nil {
		v.Free()
		return nil
	}
	var qe *quickjs.Error
	if errors.As(err, &qe) && qe.LineNumber > 0 {
		return fmt.Errorf("line %d, column %d: %s: %s", qe.LineNumber, qe.ColumnNumber, qe.Name, qe.Message)
	}
	return err
}
