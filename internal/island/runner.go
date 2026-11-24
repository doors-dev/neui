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
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/swaggest/jsonschema-go"
	"modernc.org/quickjs"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/schema"
)

type Call[I any, O any] string

func (c *Call[I, O]) UnmarshalJSON(data []byte) error {
	var src string
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}
	if err := checkSyntax("\"use strict\";(" + src + "\n)"); err != nil {
		return err
	}
	*c = Call[I, O](src)
	return nil
}

func (Call[I, O]) InlineJSONSchema() {}

func (Call[I, O]) JSONSchemaAllOf() []any {
	return []any{struct {
		Arg     I `json:"arg"`
		Returns O `json:"returns,omitempty"`
	}{}}
}

func (Call[I, O]) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithExtraPropertiesItem("x-neui-call", true)
	return nil
}

type runnerKey struct{}

//go:embed runner.js
var runnerJS string

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
	go r.owner.task.Notify(data, map[string]any{"kind": "call_error", "note": callErrorNote})
}

func Report(ctx context.Context, err error) {
	store, ok := ctx.Value(common.KeyIslandStore).(doors.Store)
	if !ok {
		return
	}
	runner, ok := store.Load(runnerKey{}).(*runner)
	if !ok {
		return
	}
	widget, _ := ctx.Value(common.KeyIslandWidget).(common.IslandWidget)
	runner.reportWidget(widget, err)
}

func Ended(ctx context.Context) bool {
	store, ok := ctx.Value(common.KeyIslandStore).(doors.Store)
	if !ok {
		return false
	}
	runner, ok := store.Load(runnerKey{}).(*runner)
	return ok && runner.owner.ctx.Err() != nil
}

func (r *runner) reportWidget(widget common.IslandWidget, err error) {
	if logErr := js.Log(r.owner.fs, r.owner.log, "ERROR", fmt.Sprintf("widget %s #%d: %s", widget.Name, widget.ID, err)); logErr != nil {
		slog.Error("island: widget log write failed", "log", r.owner.log, "err", logErr)
	}
	key := fmt.Sprintf("widget:%d:%s", widget.ID, err)
	r.mu.Lock()
	if r.reported[key] || len(r.reported) >= fnReportsCap {
		r.mu.Unlock()
		return
	}
	r.reported[key] = true
	r.mu.Unlock()
	data, _ := json.Marshal(map[string]any{"widget": widget.Name, "id": widget.ID, "error": err.Error()})
	go r.owner.task.Notify(data, map[string]any{"kind": "widget_error", "note": widgetErrorNote})
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
	if fn.ctx.Err() != nil {
		fn.rej <- context.Cause(fn.ctx)
		return
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
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
