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

package common

import (
	"context"
	"encoding/json"
	"iter"

	"neui.dev/api"
)

func GetController(ctx context.Context) Controller {
	return ctx.Value(KeyController).(Controller)
}

func AttachThread(ctx context.Context, from context.Context) context.Context {
	c, ok := from.Value(KeyController).(Controller)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, KeyController, c)
}

// Task is a background task registered with ToolTask. Ok or Err delivers its
// final result and ends it. Notify delivers an interim record while the task
// keeps running; it is dropped once the task has finished. All methods are
// safe to call from any goroutine.
type Task interface {
	// ID is the task's id within the thread, as the agent and the UI see it.
	ID() uint
	// Ok ends the task with output, a JSON value, as its result; it reaches
	// the agent as a <task_result> block in the conversation.
	Ok(output json.RawMessage)
	// Err ends the task with the error as its result.
	Err(error)
	// Notify delivers data, a JSON value, as an interim report with meta
	// attributes, while the task keeps running.
	Notify(data json.RawMessage, meta map[string]any)
}

type Controller interface {
	ID() api.ThreadID
	Context() context.Context
	PageContext() context.Context
	Modules() iter.Seq[Module]
	Config() api.ThreadConfig
	FilesConfig() api.FilesConfig
	Model() api.Model
	FS() api.FS
	Task(comment string, attrs map[string]any, cancel context.CancelCauseFunc) (Task, error)
	Ask(ask api.ChatAsk) (done func(ctx context.Context), err error)
	Rename(ctx context.Context, name string) error
}
