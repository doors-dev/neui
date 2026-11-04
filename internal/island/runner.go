package island

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/doors-dev/doors"
	"modernc.org/quickjs"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/schema"
)

// Call is a function an island script hands a widget as a param or patch
// field: the source of a single-argument JavaScript function, such as
// `(row) => app.price(row.sku) * row.qty`. Its syntax is checked when the
// params are decoded, so a broken source throws at the script's line.
type Call[I any, O any] string

// UnmarshalJSON decodes the source from a JSON string and fails when it is
// not a syntactically valid JavaScript expression.
func (c *Call[I, O]) UnmarshalJSON(data []byte) error {
	var src string
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}
	if err := checkSyntax("(" + src + "\n)"); err != nil {
		return err
	}
	*c = Call[I, O](src)
	return nil
}

// InlineJSONSchema marks Call to be inlined where it appears in a schema.
func (Call[I, O]) InlineJSONSchema() {}

// JSONSchemaBytes describes Call to the model as a string holding a function
// from the schema of I to the schema of O.
func (Call[I, O]) JSONSchemaBytes() ([]byte, error) {
	in, err := schema.NewSchema[I]()
	if err != nil {
		return nil, err
	}
	out, err := schema.NewSchema[O]()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{
		"type":  "string",
		"title": "function as JS source: (arg: " + string(in.Raw()) + ") => " + string(out.Raw()),
	})
}

type runnerKey struct{}

//go:embed runner.js
var runnerJS string

// Call runs the function with i and decodes its result into O. Calls of one
// app run one at a time on a JS runtime separate from the island script,
// limited by IslandConfig.FnMemory, with each call limited by
// IslandConfig.FnTime and IslandConfig.MaxCallDepth. Its globals are frozen,
// so no call leaves state for the next, and a call sees only its argument,
// the app functions, console and the standard built-ins. ctx must come from
// the island (a widget's render, Update or event handlers); elsewhere Call
// returns an error. A failure caused by the function, including a result that
// does not decode into O, is written to the app's log and, the first time for
// each source during a script turn and for at most 16 sources, reported to
// the agent as a call_error notification.
func (c Call[I, O]) Call(ctx context.Context, i I) (O, error) {
	json, err := json.Marshal(i)
	if err != nil {
		var o O
		return o, err
	}
	schema, err := schema.NewSchema[O]()
	if err != nil {
		var o O
		return o, err
	}
	store, ok := ctx.Value(common.KeyIslandStore).(doors.Store)
	if !ok {
		var o O
		return o, errors.New("no store")
	}
	runner := store.Load(runnerKey{}).(*runner)
	return runner.execute(ctx, string(c), json, schema)
}

type fn struct {
	ctx  context.Context
	arg  json.RawMessage
	res  chan<- json.RawMessage
	rej  chan<- error
	code string
}

type runner struct {
	owner    *run
	ch       chan fn
	mu       sync.Mutex
	reported map[string]bool
	vm       *quickjs.VM
	ctx      context.Context
}

func newRunner(owner *run) *runner {
	return &runner{
		owner:    owner,
		ctx:      owner.ctx,
		ch:       make(chan fn),
		reported: map[string]bool{},
	}
}

func (r *runner) reset() {
	r.mu.Lock()
	clear(r.reported)
	r.mu.Unlock()
}

func (r *runner) run() {
	defer func() {
		if r.vm != nil {
			r.vm.Close()
		}
	}()
	for {
		select {
		case <-r.owner.ctx.Done():
			return
		case fn := <-r.ch:
			r.evaluate(fn)
		}
	}
}

func (r *runner) report(code string, err error) {
	if logErr := js.Log(r.owner.fs, r.owner.log, "ERROR", "function: "+err.Error()); logErr != nil {
		slog.Error("island: function log write failed", "log", r.owner.log, "err", logErr)
	}
	r.mu.Lock()
	if r.reported[code] || len(r.reported) >= fnReportsCap {
		r.mu.Unlock()
		return
	}
	r.reported[code] = true
	r.mu.Unlock()
	data, _ := json.Marshal(map[string]string{"error": err.Error(), "src": code})
	go r.owner.task.Notify(data, map[string]any{"kind": "call_error"})
}

func (r *runner) newVM() (*quickjs.VM, error) {
	config := r.owner.config
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, err
	}
	vm.SetMemoryLimit(uintptr(config.FnMemory))
	vm.SetMaxStackSize(uintptr(config.MaxCallDepth))
	if err := js.RegisterSerializers(vm); err != nil {
		vm.Close()
		return nil, err
	}
	if err := js.RegisterLoggers(vm, r.owner.fs, r.owner.log); err != nil {
		vm.Close()
		return nil, err
	}
	if err := r.owner.island.lib.RegisterApp(func() context.Context { return r.ctx }, vm); err != nil {
		vm.Close()
		return nil, err
	}
	if _, err := vm.Eval(runnerJS, quickjs.EvalGlobal); err != nil {
		vm.Close()
		return nil, err
	}
	vm.SetEvalTimeout(config.FnTime)
	return vm, nil
}

func (r *runner) evaluate(fn fn) {
	config := r.owner.config
	if r.vm == nil {
		vm, err := r.newVM()
		if err != nil {
			fn.rej <- err
			return
		}
		r.vm = vm
	}
	vm := r.vm
	ctx, cancel := context.WithTimeout(fn.ctx, config.FnTime)
	defer cancel()
	defer context.AfterFunc(r.owner.ctx, cancel)()
	code := "\"use strict\";\n__run((" + fn.code + "\n), " + string(fn.arg) + ")"
	if r.owner.ctx.Err() != nil {
		fn.rej <- context.Cause(r.owner.ctx)
		return
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(r.owner.ctx, func() {
		vm.Interrupt()
		close(interrupted)
	})
	r.ctx = ctx
	res, err := vm.Eval(code, quickjs.EvalGlobal)
	r.ctx = r.owner.ctx
	if !stop() {
		<-interrupted
	}
	if err != nil {
		if _, probeErr := vm.Eval("0", quickjs.EvalGlobal); probeErr != nil {
			vm.Close()
			r.vm = nil
		}
	}
	if r.owner.ctx.Err() != nil {
		fn.rej <- context.Cause(r.owner.ctx)
		return
	}
	if fn.ctx.Err() != nil {
		fn.rej <- context.Cause(fn.ctx)
		return
	}
	var qe *quickjs.Error
	switch {
	case errors.As(err, &qe) && qe.Name == "InternalError" && qe.Message == "interrupted":
		err = fmt.Errorf("%s: %w", config.FnTime, errFnTime)
		r.report(fn.code, err)
		fn.rej <- err
		return
	case errors.As(err, &qe) && qe.Stack != "":
		err = fmt.Errorf("%w\n%s", err, strings.TrimRight(qe.Stack, "\n"))
		r.report(fn.code, err)
		fn.rej <- err
		return
	case err != nil:
		r.report(fn.code, err)
		fn.rej <- err
		return
	}
	out, ok := res.(string)
	if !ok {
		r.report(fn.code, errFnResult)
		fn.rej <- errFnResult
		return
	}
	fn.res <- json.RawMessage(out)
}

func (r *runner) execute[O any](ctx context.Context, script string, input json.RawMessage, schema schema.Schema[O]) (O, error) {
	res := make(chan json.RawMessage, 1)
	rej := make(chan error, 1)
	select {
	case <-ctx.Done():
		var o O
		return o, context.Cause(ctx)
	case <-r.owner.ctx.Done():
		var o O
		return o, context.Cause(r.owner.ctx)
	case r.ch <- fn{
		ctx:  ctx,
		arg:  input,
		res:  res,
		rej:  rej,
		code: script,
	}:
	}
	select {
	case <-ctx.Done():
		var o O
		return o, context.Cause(ctx)
	case err := <-rej:
		var o O
		return o, err
	case j := <-res:
		o, err := schema.Decode(j)
		if err != nil {
			r.report(script, err)
			return o, err
		}
		return o, nil
	}
}
