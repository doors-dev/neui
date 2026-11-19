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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/doors-dev/doors"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
	"neui.dev/internal/thread/toolkit"
)

func newLink(thread *thread, backend *common.Backend, page page.Page, meta any) *link {
	fs := filesystem.Open(thread.id, backend.WorkspaceStorage(), backend.UploadStorage(), backend.FilesConfig())
	ctx, cancel := context.WithCancel(backend.Context())
	toolkit := toolkit.NewToolkit(ctx, backend.Modules()...)
	linkCtx, linkCancel := context.WithCancelCause(ctx)
	pageModule := page.Module()
	linkToolkit := toolkit.Wrap(linkCtx, func(context.Context) common.Module { return pageModule })
	return &link{
		thread:      thread,
		backend:     backend,
		fs:          fs,
		ctx:         ctx,
		cancel:      cancel,
		linkCtx:     linkCtx,
		linkCancel:  linkCancel,
		ch:          make(chan Message),
		toolkit:     toolkit,
		linkToolkit: linkToolkit,
		page:        page,
		pageModule:  pageModule,
		linkTasks:   &linkTasks{thread: thread},
		meta:        meta,
	}
}

type link struct {
	thread      *thread
	backend     *common.Backend
	fs          filesystem.FS
	ctx         context.Context
	cancel      context.CancelFunc
	linkCtx     context.Context
	linkCancel  context.CancelCauseFunc
	ch          chan Message
	toolkit     toolkit.Toolkit
	linkToolkit toolkit.Toolkit
	page        page.Page
	pageModule  page.Module
	once        sync.Once
	meta        any
	guard       <-chan error
	*linkTasks
}

func (l *link) system() string {
	b := strings.Builder{}
	for _, s := range l.backend.System() {
		if b.Len() != 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}
	for s := range l.linkToolkit.System() {
		if b.Len() != 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}
	return b.String()
}

func (l *link) tools() []api.Tool {
	return slices.Collect(l.linkToolkit.Tools())
}

func (l *link) useTools(roundID api.RoundID, tools []api.ContentToolUse) (<-chan api.ContentToolResult, context.CancelCauseFunc) {
	ch := make(chan api.ContentToolResult, len(tools))
	stop, stopCancel := context.WithCancelCause(l.ctx)
	var mu sync.Mutex
	var running context.CancelCauseFunc
	limit := l.backend.Config().ToolCallLimit
	budget := replay.RoundItemBudget(l.backend.Config(), l.backend.Provider().Model())
	go func() {
		defer close(ch)
		total := 0
		for i, tool := range tools {
			if stop.Err() != nil {
				ch <- api.NewContentToolResult(tool.ID, nil, context.Cause(stop))
				continue
			}
			if limit >= 0 && i >= limit {
				ch <- api.NewContentToolResult(tool.ID, nil, fmt.Errorf("%q: not called, one round runs at most %d tool calls", tool.Name, limit))
				continue
			}
			t, ok := l.linkToolkit.Get(tool.Name)
			if !ok {
				ch <- api.NewContentToolResult(tool.ID, nil, fmt.Errorf("%q: %w", tool.Name, api.ErrToolNotFound))
				continue
			}
			if bytes.Equal(tool.Input, api.DiscardedArguments) {
				ch <- api.NewContentToolResult(tool.ID, nil, api.DiscardedArgumentsError{})
				continue
			}
			if total >= budget {
				ch <- api.NewContentToolResult(tool.ID, nil, errors.New("Not run: this turn's tool results reached their size limit. Call it again next turn."))
				continue
			}
			input := tool.Input
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			ctx, cancel := context.WithCancelCause(t.Context)
			ctx = context.WithValue(ctx, common.KeyController, controller{roundID, l, t.Module, t.Name, t.Context})
			mu.Lock()
			running = cancel
			if stop.Err() != nil {
				cancel(context.Cause(stop))
			}
			mu.Unlock()
			res := func() (res api.ContentToolResult) {
				defer func() {
					if r := recover(); r != nil {
						res = api.NewContentToolResult(tool.ID, nil, fmt.Errorf("%q: tool panicked: %v", tool.Name, r))
					}
				}()
				content, err := t.Func(ctx, input)
				return api.NewContentToolResult(tool.ID, content, err)
			}()
			size := 0
			for _, c := range res.Content {
				if text, ok := c.(api.ContentText); ok {
					size += len(text.Text)
				}
			}
			if size > budget {
				note := replay.OmittedNote(size)
				res = api.ContentToolResult{ID: res.ID, Content: api.UserContents{api.ContentText{Text: note}}, Error: res.Error}
				size = len(note)
			}
			total += size
			ch <- res
			cancel(nil)
		}
	}()
	return ch, func(cause error) {
		stopCancel(cause)
		mu.Lock()
		defer mu.Unlock()
		if running != nil {
			running(cause)
		}
	}
}

func (l *link) dispose() {
	l.once.Do(func() {
		l.cancel()
		if err := l.fs.Dispose(); err != nil {
			doors.Logger(l.backend.Context()).Error("thread: workspace dispose failed", "thread", l.thread.id, "err", err)
		}
	})
}

func (l *link) inherit(page page.Page, meta any) *link {
	l.linkCancel(common.ErrRelinked)
	linkCtx, linkCancel := context.WithCancelCause(l.ctx)
	pageModule := page.Module()
	linkToolkit := l.toolkit.Wrap(linkCtx, func(context.Context) common.Module { return pageModule })
	return &link{
		thread:      l.thread,
		backend:     l.backend,
		fs:          l.fs,
		ctx:         l.ctx,
		cancel:      l.cancel,
		linkCtx:     linkCtx,
		linkCancel:  linkCancel,
		ch:          make(chan Message),
		toolkit:     l.toolkit,
		linkToolkit: linkToolkit,
		page:        page,
		pageModule:  pageModule,
		linkTasks:   l.linkTasks,
		meta:        meta,
	}
}

func (l *link) link() Link {
	return Link{
		inner: l,
		state: api.InstanceThread{
			ID:   l.thread.id,
			Name: l.thread.replay.Name(),
			New:  !l.thread.replay.IsLoaded(),
		},
	}
}

func (l *link) send(m Message) {
	select {
	case <-l.linkCtx.Done():
		m.Ack()
	case l.ch <- m:
	}
}

func (l *link) backpressure() <-chan error {
	prev := l.guard
	l.guard = nil
	if ctx := l.page.Context(); doors.HasInstance(ctx) {
		l.guard = doors.Call(ctx, doors.ActionTime{})
	}
	if prev == nil {
		closed := make(chan error)
		close(closed)
		return closed
	}
	return prev
}
