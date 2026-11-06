package island

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"modernc.org/libquickjs"
	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/island/core"
)

type Library = core.Library

func NewLibrary() Library {
	return core.NewLibrary()
}

type Island = *island

func NewIsland(
	placeholder gox.Elem,
	template func(slot gox.Comp) gox.Elem,
	success func(close func(ctx context.Context)) gox.Elem,
	failure func(close func(ctx context.Context), err error) gox.Elem,
	lib Library,
) Island {
	island := &island{
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
	mu          sync.Mutex
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
}

func (i *island) Spec(ctx context.Context, space string) string {
	c := common.AppIslandConfig(ctx)
	b := strings.Builder{}
	b.WriteString(specNotation)
	b.WriteString("\n\nRuntime\n")
	b.WriteString(specRuntime)
	fmt.Fprintf(&b, "- console.log, info, warn and error append to the workspace file %s, emptied when an app starts.\n", logFile(space))
	ctrl := common.GetController(ctx)
	fmt.Fprintf(&b, "- Limits: %d slots, views, controls and layouts alive at once; %d set, update and notify calls between ui.draw calls; %d bytes of ui.notify values between ui.draw calls; %s between ui.draw calls, app function and tool calls included; %d MiB memory; call depth %d; %d scripts queued; %s and %d MiB per function call.\n", c.MaxItems, c.MaxOps, notificationsCap(ctrl.Config(), ctrl.Model()), c.TurnTime, c.Memory>>20, c.MaxCallDepth, c.MaxQueue, c.FnTime, c.FnMemory>>20)
	i.lib.WriteSpec(&b)
	return b.String()
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
				case <-old.pageContext.Done():
					return
				}
				i.mu.Lock()
				defer i.mu.Unlock()
				if i.seq != seq {
					return
				}
				i.run = nil
				i.updateStatusScreen(old.pageContext, noAppStatus, i.placeholder)
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
	run, err := newRun(ctx, i, i.seq, space, comment)
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
	defer i.mu.Unlock()
	if i.seq != seq {
		return
	}
	i.run = nil
	i.seq++
	newSeq := i.seq
	close := func(ctx context.Context) {
		i.mu.Lock()
		defer i.mu.Unlock()
		if i.seq != newSeq {
			return
		}
		i.seq++
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
	}
	switch {
	case errors.Is(err, api.ErrCanceledByUser):
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
	case err == nil && i.success != nil:
		i.updateStatusScreen(ctx, endedStatus(task), i.success(close))
	case err != nil && i.failure != nil:
		i.updateStatusScreen(ctx, failedStatus(task, err), i.failure(close, err))
	default:
		i.updateStatusScreen(ctx, noAppStatus, i.placeholder)
	}
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

func (i *island) Done(ctx context.Context, code string, contunue bool) error {
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
		ctx, err := getSpace(cur.Context()).AddSpace(cur.Context(), screenSpaceName, screenSpaceDesc)
		if err != nil {
			return err
		}
		return doors.Ctx(ctx).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(i.status); err != nil {
				return err
			}
			return cur.Comp(i.screen)
		})
	}
}

func check(code string) error {
	if err := checkSyntax(code); err != nil {
		return fmt.Errorf("%w: %w", err, errScriptRejected)
	}
	return nil
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
