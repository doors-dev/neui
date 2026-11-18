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

package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
)

type ToolFunc = func(ctx context.Context, arg json.RawMessage) (api.UserContents, error)

func NewToolkit(ctx context.Context, factories ...common.ModuleFactory) Toolkit {
	return newToolkit(ctx, make(map[string]struct{}), factories)
}

func newToolkit(ctx context.Context, overwrite map[string]struct{}, factories []common.ModuleFactory) Toolkit {
	modules := make([]common.Module, 0, len(factories))
	names := maps.Clone(overwrite)
	block := make(map[string]struct{})
	for _, factory := range factories {
		m := factory(ctx)
		name := m.Name()
		i, found := searchModules(modules, name)
		if found {
			doors.Logger(ctx).Error("thread: duplicate module name, skipped", "module", name)
			continue
		}
		if _, ok := names[name]; ok {
			block[name] = struct{}{}
			doors.Logger(ctx).Error("thread: duplicate module name, overwriten", "module", name)
		}
		names[name] = struct{}{}
		modules = slices.Insert(modules, i, m)
	}
	return Toolkit{
		ctx:     ctx,
		names:   names,
		modules: modules,
		block:   block,
	}
}

type Toolkit struct {
	ctx     context.Context
	names   map[string]struct{}
	modules []common.Module
	block   map[string]struct{}
	inner   *Toolkit
}

type Tool struct {
	Module  string
	Name    string
	Context context.Context
	Func    ToolFunc
}

func (t Toolkit) Get(name string) (Tool, bool) {
	moduleName, toolName, ok := strings.Cut(name, "__")
	if !ok {
		return Tool{}, false
	}
	if i, found := searchModules(t.modules, moduleName); found {
		tool, ok := t.modules[i].Get(toolName)
		if !ok {
			return Tool{}, false
		}
		return Tool{
			Module:  moduleName,
			Name:    toolName,
			Context: t.ctx,
			Func:    tool.Agent,
		}, true
	}
	if t.inner != nil {
		return t.inner.Get(name)
	}
	return Tool{}, false
}

func (t Toolkit) Wrap(ctx context.Context, factories ...common.ModuleFactory) Toolkit {
	toolkit := newToolkit(ctx, t.names, factories)
	toolkit.inner = &t
	return toolkit
}

func (t Toolkit) System() iter.Seq[string] {
	return func(yield func(string) bool) {
		for m := range t.all() {
			desc := m.Desc()
			if desc == "" {
				continue
			}
			if defs, _ := outputDefs(m); len(defs) > 0 {
				b := strings.Builder{}
				b.WriteString(desc)
				b.WriteString(outputDefsHead)
				for _, name := range slices.Sorted(maps.Keys(defs)) {
					fmt.Fprintf(&b, "\n%s %s", name, defs[name])
				}
				desc = b.String()
			}
			if !yield(fmt.Sprintf("<module name=%q>\n%s\n</module>", m.Name(), desc)) {
				return
			}
		}
	}
}

func (t Toolkit) Tools() iter.Seq[api.Tool] {
	return func(yield func(api.Tool) bool) {
		for m := range t.all() {
			name := m.Name()
			_, brief := outputDefs(m)
			for _, tool := range m.List() {
				desc := tool.Summary()
				if brief[tool.Name()] {
					desc = tool.Brief()
				}
				if !yield(api.Tool{
					Name:   fmt.Sprintf("%s__%s", name, tool.Name()),
					Desc:   desc,
					Schema: json.RawMessage(tool.Params()),
				}) {
					return
				}
			}
		}
	}
}

const outputDefsHead = "\n\nOutput definitions, referred to as {\"$ref\":\"#/definitions/X\"} in this module's output schemas:"

func outputDefs(m common.Module) (map[string]json.RawMessage, map[string]bool) {
	defs := map[string]json.RawMessage{}
	brief := map[string]bool{}
	if m.Desc() == "" {
		return defs, brief
	}
	for _, t := range m.List() {
		_, own := t.OutputSpec()
		if len(own) == 0 {
			continue
		}
		fits := true
		for name, raw := range own {
			if seen, ok := defs[name]; ok && !bytes.Equal(seen, raw) {
				fits = false
			}
		}
		if !fits {
			continue
		}
		maps.Copy(defs, own)
		brief[t.Name()] = true
	}
	return defs, brief
}

func (t Toolkit) Modules() iter.Seq[common.Module] {
	return t.all()
}

func (t Toolkit) all() iter.Seq[common.Module] {
	return func(yield func(common.Module) bool) {
		t.yieldAll(make(map[string]struct{}), yield)
	}
}

func (t Toolkit) yieldAll(blocks map[string]struct{}, yield func(common.Module) bool) bool {
	for _, m := range t.modules {
		if _, ok := blocks[m.Name()]; ok {
			continue
		}
		if !yield(m) {
			return false
		}
	}
	if t.inner != nil {
		maps.Copy(blocks, t.block)
		return t.inner.yieldAll(blocks, yield)
	}
	return true
}

func searchModules(modules []common.Module, name string) (int, bool) {
	return slices.BinarySearchFunc(modules, name, func(m common.Module, name string) int {
		return strings.Compare(m.Name(), name)
	})
}
