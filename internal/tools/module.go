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

package tools

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/utils"
)

type Module = *module

// ToolID identifies a tool within its own module, for Remove. A module numbers
// the tools it is given from 1, so an ID is only meaningful against the module
// that issued it.
type ToolID int

func NewModule(name string, desc string) Module {
	return &module{
		name:   utils.SanitizeName(name),
		desc:   desc,
		object: true,
	}
}

func NewScriptModule(name string, desc string) Module {
	return &module{
		name: utils.SanitizeName(name),
		desc: desc,
	}
}

type toolEntry struct {
	id   ToolID
	tool common.Tool
}

type module struct {
	id     ToolID
	name   string
	desc   string
	object bool
	tools  []toolEntry
}

func (m Module) Name() string {
	return m.name
}

func (m Module) Desc() string {
	return m.desc
}

func (m Module) List() []common.Tool {
	tools := make([]common.Tool, len(m.tools))
	for i, t := range m.tools {
		tools[i] = t.tool
	}
	return tools
}

func (m Module) Get(name string) (common.Tool, bool) {
	i, ok := m.search(utils.SanitizeName(name))
	if !ok {
		return tool{}, false
	}
	return m.tools[i].tool, true
}

func (m Module) Remove(id ToolID) {
	for i, t := range m.tools {
		if t.id != id {
			continue
		}
		m.tools = slices.Delete(m.tools, i, i+1)
		return
	}
}

func (m *module) search(name string) (int, bool) {
	return slices.BinarySearchFunc(m.tools, name, func(t toolEntry, name string) int {
		return strings.Compare(t.tool.Name(), name)
	})
}

func (m *module) add(id ToolID, tool common.Tool) {
	i, ok := m.search(tool.Name())
	if ok {
		m.tools[i] = toolEntry{id, tool}
		return
	}
	m.tools = slices.Insert(m.tools, i, toolEntry{id, tool})
}

func (m Module) AddTool(tool common.Tool) ToolID {
	m.id += 1
	m.add(m.id, tool)
	return m.id
}

func (m Module) AddRaw(
	name string,
	desc string,
	input schema.Schema[any],
	output schema.Schema[any],
	f func(ctx context.Context, input json.RawMessage) (any, error),
) (ToolID, error) {
	tool, err := NewRawTool(name, desc, input, output, f, m.object)
	if err != nil {
		return 0, err
	}
	return m.AddTool(tool), nil
}

const AskDesc = "\n\nThis shows the user something in the chat and waits for them to answer, which can take a long time. Do not restate what it shows in your reply. They can dismiss it instead of answering, and then the call fails; an answer that declines or says no is still a result, not a failure."

func Asking[I any, O any](f func(input I, reply common.Answer[O]) api.ChatAsk) func(ctx context.Context, input I) (O, error) {
	return func(ctx context.Context, input I) (O, error) {
		ctrl := common.GetController(ctx)
		answer := newAnswer[O](ctx)
		defer answer.ctxCancel()
		ask := f(input, answer)
		ask.Cancel = answer.cancel
		done, err := ctrl.Ask(ask)
		if err != nil {
			var o O
			return o, err
		}
		output, ctx, err := answer.receive()
		done(ctx)
		return output, err
	}
}

func (m Module) Ask[I any, O any](name string, desc string, f func(input I, reply common.Answer[O]) api.ChatAsk) (ToolID, error) {
	return m.Add(name, desc+AskDesc, Asking(f))
}

func (m Module) Add[I any, O any](name string, desc string, f func(ctx context.Context, input I) (O, error)) (ToolID, error) {
	tool, err := NewTool(name, desc, f, m.object)
	if err != nil {
		return 0, err
	}
	return m.AddTool(tool), nil
}
