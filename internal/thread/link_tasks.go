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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"neui.dev/api"
	"neui.dev/internal/common"
)

type commonTask struct {
	id   uint
	link *linkTasks
}

func (t commonTask) ID() uint {
	return t.id
}

func (t commonTask) Ok(output json.RawMessage) {
	select {
	case <-t.link.thread.close:
	case t.link.thread.ch <- commandTaskResult{
		linkTasks: t.link,
		id:        t.id,
		output:    output,
	}:
	}
}

func (t commonTask) Err(err error) {
	var output any
	var withOutput common.ErrorWithOutput
	if errors.As(err, &withOutput) {
		output = withOutput.Output
	}
	select {
	case <-t.link.thread.close:
	case t.link.thread.ch <- commandTaskResult{
		linkTasks: t.link,
		id:        t.id,
		output:    output,
		err:       err,
	}:
	}
}

func (t commonTask) Notify(data json.RawMessage, meta map[string]any) {
	select {
	case <-t.link.thread.close:
	case t.link.thread.ch <- commandTaskNotify{
		linkTasks: t.link,
		id:        t.id,
		data:      data,
		meta:      meta,
	}:
	}
}

type linkTask struct {
	id      uint
	origin  api.RoundID
	cancel  context.CancelCauseFunc
	name    string
	comment string
	attrs   map[string]any
	module  string
}

func (l linkTask) ui() api.Task {
	return api.Task{
		ID:      l.id,
		Module:  l.module,
		Comment: l.comment,
		Meta:    l.attrs,
		Name:    l.name,
		Cancel:  l.cancel,
	}
}

type linkTasks struct {
	mu      sync.Mutex
	thread  *thread
	counter uint
	tasks   []linkTask
}

func (m *linkTasks) taskAdd(origin api.RoundID, module string, name string, comment string, meta map[string]any, cancel context.CancelCauseFunc) common.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter += 1
	m.tasks = append(m.tasks, linkTask{
		id:      m.counter,
		origin:  origin,
		name:    name,
		module:  module,
		comment: comment,
		attrs:   meta,
		cancel:  cancel,
	})
	return commonTask{
		id:   m.counter,
		link: m,
	}
}

func (m *linkTasks) taskRemove(id uint) (linkTask, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := slices.BinarySearchFunc(m.tasks, id, func(e linkTask, id uint) int {
		return cmp.Compare(e.id, id)
	})
	if !ok {
		return linkTask{}, false
	}
	task := m.tasks[i]
	m.tasks = slices.Delete(m.tasks, i, i+1)
	return task, true
}

func (m *linkTasks) taskCut(round api.RoundID) bool {
	m.mu.Lock()
	var cut []linkTask
	m.tasks = slices.DeleteFunc(m.tasks, func(e linkTask) bool {
		if e.origin < round {
			return false
		}
		cut = append(cut, e)
		return true
	})
	m.mu.Unlock()
	for _, e := range cut {
		e.cancel(api.ErrCanceledByUser)
	}
	return len(cut) != 0
}

func (m *linkTasks) taskGet(id uint) (linkTask, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := slices.BinarySearchFunc(m.tasks, id, func(e linkTask, id uint) int {
		return cmp.Compare(e.id, id)
	})
	if !ok {
		return linkTask{}, false
	}
	return m.tasks[i], true
}

func (m *linkTasks) taskList() []api.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks := make([]api.Task, len(m.tasks))
	for i, e := range m.tasks {
		tasks[i] = e.ui()
	}
	return tasks
}
