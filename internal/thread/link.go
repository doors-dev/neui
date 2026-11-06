package thread

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/page"
	"neui.dev/internal/thread/toolkit"
)

func newLink(thread *thread, backend *common.Backend, page page.Page, meta any) *link {
	fs := filesystem.Open(thread.id, backend.WorkspaceStorage(), backend.FileStorage(), backend.FilesConfig())
	ctx, cancel := context.WithCancel(backend.Context())
	toolkit := toolkit.NewToolkit(ctx, backend.Modules()...)
	linkCtx, linkCancel := context.WithCancel(ctx)
	linkToolkit := toolkit.Wrap(linkCtx, page.Module)
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
	linkCancel  context.CancelFunc
	ch          chan Message
	toolkit     toolkit.Toolkit
	linkToolkit toolkit.Toolkit
	page        page.Page
	once        sync.Once
	meta        any
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
	stop, stopCancel := context.WithCancelCause(context.Background())
	var mu sync.Mutex
	var running context.CancelCauseFunc
	limit := l.backend.Config().ToolCallLimit
	go func() {
		defer close(ch)
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
			ch <- func() (res api.ContentToolResult) {
				defer func() {
					if r := recover(); r != nil {
						res = api.NewContentToolResult(tool.ID, nil, fmt.Errorf("%q: tool panicked: %v", tool.Name, r))
					}
				}()
				content, err := t.Func(ctx, input)
				return api.NewContentToolResult(tool.ID, content, err)
			}()
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
		l.fs.Dispose()
	})
}

func (l *link) inherit(page page.Page, meta any) *link {
	l.linkCancel()
	linkCtx, linkCancel := context.WithCancel(l.ctx)
	linkToolkit := l.toolkit.Wrap(linkCtx, page.Module)
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
