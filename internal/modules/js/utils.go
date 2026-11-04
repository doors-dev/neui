package js

import (
	"context"
	"encoding/json"
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
		if err := Log(fs, file, level, s); err != nil {
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
		try { return JSON.stringify(v); } catch (e) { return String(v); }
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

func RegisterTools(vm *quickjs.VM, ctx func() context.Context, prefix string, tools []common.Tool) error {
	b := strings.Builder{}
	for _, t := range tools {
		name := prefix + t.Name()
		host := "__" + name
		if err := vm.RegisterHostFunc(host, func(args []any) (any, error) { return t.JS(ctx())(args) }); err != nil {
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
		b.WriteString("\", {value: (f => arg => f(JSON.stringify(arg ?? ")
		b.WriteString(missing)
		b.WriteString(")).value)(")
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
	w, err := fs.Append(name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s %s %s\n", time.Now().UTC().Format(time.RFC3339), level, entry)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

func readScript(ctx context.Context, fs api.FS, file string, code string, arg json.RawMessage) (string, error) {
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
	if _, err := io.Copy(&b, src); err != nil {
		return "", err
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
