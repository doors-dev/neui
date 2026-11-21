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

package js

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strings"
	"time"

	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
)

const (
	scriptLimit = 4 << 20
	logCap      = 1 << 20
	logKeep     = 512 << 10
)

func logName(name string) string {
	return name + ".log"
}

func RegisterSerializers(vm *quickjs.VM) error {
	_, err := vm.Eval(serializers, quickjs.EvalGlobal)
	return err
}

const serializers = `Uint8Array.prototype.toJSON = function () { return this.toBase64(); };
ArrayBuffer.prototype.toJSON = function () { return new Uint8Array(this).toBase64(); };
Map.prototype.toJSON = function () { return Object.fromEntries(this); };
Set.prototype.toJSON = function () { return Array.from(this); };`

func ClearLog(fs api.FS, file string) error {
	w, err := fs.Create(file)
	if err != nil {
		return err
	}
	return w.Close()
}

func RegisterLoggers(vm *quickjs.VM, fs api.FS, file string) error {
	err := vm.RegisterFunc("__log", func(level string, s string) {
		if err := Log(fs, file, level, s); err != nil && !errors.Is(err, api.ErrQuota) {
			slog.Error("js: script log write failed", "log", file, "err", err)
		}
	}, false)
	if err != nil {
		return err
	}
	if _, err := vm.Eval(loggers, quickjs.EvalGlobal); err != nil {
		return err
	}
	return nil
}

const loggers = `(function () {
	const __log = globalThis.__log;
	delete globalThis.__log;
	function fmt(v) {
		if (typeof v === 'string') return v;
		if (v instanceof Error) return String(v) + (v.stack ? '\n' + v.stack : '');
		try { const s = JSON.stringify(v); return s === undefined ? String(v) : s; } catch (e) { return String(v); }
	}
	const join = (a) => a.map(fmt).join(' ');
	globalThis.console = {
		log:   (...a) => __log('INFO', join(a)),
		info:  (...a) => __log('INFO', join(a)),
		debug: (...a) => __log('DEBUG', join(a)),
		warn:  (...a) => __log('WARN', join(a)),
		error: (...a) => __log('ERROR', join(a)),
	};
	globalThis.print = (...a) => __log('INFO', join(a));
})();`

type hostResult struct {
	Value any    `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

func RegisterTools(vm *quickjs.VM, ctx func() context.Context, prefix string, list []common.Tool) error {
	b := strings.Builder{}
	for _, t := range list {
		name := prefix + t.Name()
		host := "__" + name
		if err := vm.RegisterHostFunc(host, func(args []any) (any, error) {
			out, err := t.JS(ctx())(args)
			if err == nil {
				return hostResult{Value: out}, nil
			}
			var value any
			var container api.Container
			var multi api.MultiContainer
			if errors.As(err, &container) {
				value = container
			} else if errors.As(err, &multi) {
				value = multi
			}
			if raw, merr := json.Marshal(value); value != nil && merr == nil {
				return hostResult{Value: json.RawMessage(raw), Error: err.Error()}, nil
			}
			return hostResult{Error: err.Error()}, nil
		}); err != nil {
			return fmt.Errorf("registering %s: %w", host, err)
		}
		missing := "null"
		var params struct {
			Type any `json:"type"`
		}
		if json.Unmarshal(t.Params(), &params) == nil && params.Type == "object" {
			missing = "{}"
		}
		b.WriteString("Object.defineProperty(globalThis, \"")
		b.WriteString(name)
		b.WriteString("\", {value: (f => arg => { const r = f(JSON.stringify(arg ?? ")
		b.WriteString(missing)
		b.WriteString(")); if (\"error\" in r) throw \"value\" in r ? Object.assign(new TypeError(r.error), {value: r.value}) : new TypeError(r.error); return r.value ?? null })(")
		b.WriteString(host)
		b.WriteString("), enumerable: true, configurable: true})\ndelete globalThis.")
		b.WriteString(host)
		b.WriteRune('\n')
	}
	_, err := vm.Eval(b.String(), quickjs.EvalGlobal)
	if err != nil {
		return fmt.Errorf("binding tools: %w", err)
	}
	return nil
}

func RegisterModules(vm *quickjs.VM, ctx context.Context, modules iter.Seq[common.Module]) error {
	for m := range modules {
		if err := RegisterTools(vm, func() context.Context { return ctx }, m.Name()+"__", m.List()); err != nil {
			return err
		}
	}
	return nil
}

func Log(fs api.FS, name string, level string, entry string) error {
	line := fmt.Sprintf("%s %s %s\n", time.Now().UTC().Format(time.RFC3339), level, entry)
	if e, err := fs.Stat(context.Background(), name); err == nil && e.Size+int64(len(line)) > logCap {
		rc, err := fs.ReadRange(context.Background(), name, max(e.Size-logKeep, 0), -1)
		if err != nil {
			return err
		}
		tail, err := io.ReadAll(rc)
		if cerr := rc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		if i := bytes.IndexByte(tail, '\n'); i >= 0 {
			tail = tail[i+1:]
		} else {
			tail = nil
		}
		w, err := fs.Create(name)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s INFO earlier output dropped\n", time.Now().UTC().Format(time.RFC3339))
		if err == nil {
			_, err = w.Write(tail)
		}
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	w, err := fs.Append(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, line)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

func readScript(ctx context.Context, fs api.FS, file string, code string, arg json.RawMessage) (string, error) {
	label := file
	if file == "" {
		label = "inline code"
	}
	var src io.Reader = strings.NewReader(code)
	if file != "" {
		rc, err := fs.ReadAll(ctx, file)
		if err != nil {
			return "", err
		}
		defer rc.Close()
		src = rc
	}
	if len(arg) == 0 {
		arg = json.RawMessage("null")
	}
	b := strings.Builder{}
	b.WriteString("(() => { try { const res = ((arg) => { \"use strict\";\n")
	n, err := io.Copy(&b, io.LimitReader(src, scriptLimit+1))
	if err != nil {
		return "", err
	}
	if n > scriptLimit {
		return "", fmt.Errorf("%q is over the %d MiB script size limit; move its data into arg or a file the script reads", label, scriptLimit>>20)
	}
	quoted, err := json.Marshal(string(arg))
	if err != nil {
		return "", err
	}
	b.WriteString("\n})(JSON.parse(")
	b.Write(quoted)
	b.WriteString("));\nreturn JSON.stringify(res ?? null);\n} catch (e) { console.error(e); throw e; }\n})()")
	return b.String(), nil
}
