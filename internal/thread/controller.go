package thread

import (
	"context"
	"iter"
	"log/slog"

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

func (c controller) Ask(ask api.ChatAsk) (func(ctx context.Context), error) {
	a := &ask
	slog.Info("ask: controller.Ask", "module", c.module, "name", c.name)
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
		slog.Info("ask: controller.Ask accepted", "err", err)
		return func(ctx context.Context) {
			op := NewOperation(ctx)
			cmd := commandAskDone{a, op}
			select {
			case <-c.link.thread.close:
			case c.link.thread.ch <- cmd:
			}
			op.Receive()
		}, err
	}
}

func (c controller) Task(comment string, attrs map[string]any, cancel context.CancelCauseFunc) (common.Task, error) {
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
