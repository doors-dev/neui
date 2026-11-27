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

package thread

import (
	"context"
	"errors"
	"iter"

	"neui.dev/api"
	"neui.dev/internal/common"
)

type controller struct {
	origin api.RoundID
	link   *link
	module string
	name   string
	ctx    context.Context
}

func (c controller) ID() api.ThreadID {
	return c.link.thread.id
}

func (c controller) Ask(ask api.ChatAsk, dismiss func(context.Context)) (func(ctx context.Context), error) {
	a := &api.ChatAskView{ChatAsk: ask, Dismiss: dismiss}
	r := NewResultResponse[struct{}]()
	select {
	case <-c.link.thread.close:
		return nil, context.Canceled
	case c.link.thread.ch <- commandAsk{
		link:           c.link,
		ask:            a,
		ResultResponse: r,
	}:
		_, err := r.Receive()
		return func(ctx context.Context) {
			op := NewOperation(ctx)
			cmd := commandAskDone{a, op}
			select {
			case <-c.link.thread.close:
			case c.link.thread.ch <- cmd:
				op.Receive()
			}
		}, err
	}
}

func (c controller) Task(comment string, attrs map[string]any, cancel context.CancelCauseFunc) (common.Task, error) {
	if cancel == nil {
		return nil, errors.New("a background task needs a cancel func")
	}
	if attrs == nil {
		attrs = make(map[string]any)
	}
	r := NewResultResponse[common.Task]()
	select {
	case <-c.link.thread.close:
		return nil, context.Canceled
	case c.link.thread.ch <- commandTask{
		linkTasks:      c.link.linkTasks,
		origin:         c.origin,
		module:         c.module,
		name:           c.name,
		comment:        comment,
		attrs:          attrs,
		ResultResponse: r,
		cancel:         cancel,
	}:
		return r.Receive()
	}
}

func (c controller) Rename(ctx context.Context, name string) error {
	op := NewOperation(ctx)
	select {
	case <-c.link.thread.close:
		return context.Canceled
	case c.link.thread.ch <- commandRename{c.link, name, op}:
		return op.Receive()
	}
}

func (c controller) Config() api.ThreadConfig {
	return c.link.backend.Config()
}

func (c controller) FilesConfig() api.FilesConfig {
	return c.link.backend.FilesConfig()
}

func (c controller) Model() api.Model {
	return c.link.backend.Provider().Model()
}

func (c controller) FS() api.FS {
	return c.link.fs
}

func (c controller) Modules() iter.Seq[common.Module] {
	return c.link.linkToolkit.Modules()
}

func (c controller) Context() context.Context {
	return context.WithValue(c.ctx, common.KeyController, c)
}

func (c controller) PageContext() context.Context {
	return c.link.page.Context()
}

var _ common.Controller = controller{}
