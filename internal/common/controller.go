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

type Task interface {
	ID() uint
	Ok(output json.RawMessage)
	Err(error)
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
