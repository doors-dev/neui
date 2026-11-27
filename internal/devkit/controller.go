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

package devkit

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"os"
	"path/filepath"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
)

const threadID api.ThreadID = "devkit"

type controller struct {
	kit  *Kit
	page page.Page
	path string
}

func (c *controller) start(ctx context.Context, code string) (uint, uint, error) {
	island, path, err := c.page.Island(c.page.Context(), c.path)
	if err != nil {
		return 0, 0, err
	}
	return island.StartCode(context.WithValue(ctx, common.KeyController, common.Controller(c)), path, "devkit", code)
}

func (c *controller) ID() api.ThreadID { return threadID }

func (c *controller) Context() context.Context {
	return context.WithValue(context.WithValue(c.page.Context(), common.IslandConfigKey{}, c.kit.app.Config), common.KeyController, common.Controller(c))
}

func (c *controller) PageContext() context.Context { return c.page.Context() }

func (c *controller) Modules() iter.Seq[common.Module] {
	return func(func(common.Module) bool) {}
}

func (c *controller) Config() api.ThreadConfig { return common.ConfigDefaults(api.ThreadConfig{}) }

func (c *controller) FilesConfig() api.FilesConfig {
	return common.FilesConfigDefaults(c.kit.app.Files)
}

func (c *controller) Model() api.Model { return api.Model{} }

func (c *controller) FS() api.FS { return c.kit.fs }

func (c *controller) Task(string, map[string]any, context.CancelCauseFunc) (common.Task, error) {
	c.kit.mu.Lock()
	defer c.kit.mu.Unlock()
	c.kit.tasks++
	return task{id: c.kit.tasks, report: c.kit.app.Report}, nil
}

func (c *controller) Ask(api.ChatAsk, func(context.Context)) (func(context.Context), error) {
	return nil, errors.New("devkit: no asks")
}

func (c *controller) Rename(context.Context, string) error { return nil }

type task struct {
	id     uint
	report func(Report)
}

func (t task) ID() uint { return t.id }

func (t task) Ok(output json.RawMessage) {
	t.report(Report{Task: t.id, Kind: "ok", Data: output})
}

func (t task) Err(err error) {
	t.report(Report{Task: t.id, Kind: "error", Err: err})
}

func (t task) Notify(data json.RawMessage, meta map[string]any) {
	t.report(Report{Task: t.id, Kind: "notify", Data: data, Meta: meta})
}

type workspace string

func (w workspace) WorkspaceOpen(api.ThreadID) (api.Workspace, error) {
	dir, err := filepath.Abs(string(w))
	if err != nil {
		return api.Workspace{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return api.Workspace{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return api.Workspace{}, err
	}
	return api.Workspace{Dir: dir, Root: root}, nil
}

func (w workspace) WorkspaceDelete(api.ThreadID) error { return nil }
