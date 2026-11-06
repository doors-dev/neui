package instance

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/thread"
	"neui.dev/internal/utils"
)

var _ api.Instance = (*instance)(nil)

func (inst *instance) ID() string {
	return inst.id
}

func (inst *instance) State() doors.Beam[api.InstanceState] {
	return inst.state.Beam()
}

func (inst *instance) Chats() doors.Beam[map[api.ThreadID]api.InstanceChat] {
	return inst.session.Chats()
}

func (inst *instance) Location() *time.Location {
	return inst.session.Location()
}

func (inst *instance) ThreadOpen(ctx context.Context, id api.ThreadID, meta any) error {
	backend := inst.backend.Load()
	if backend == nil {
		return api.ErrDisabled
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	if !inst.synced.Get() {
		return api.ErrUnlinked
	}
	if _, err := backend.Storage().ThreadGet(ctx, id); err != nil {
		return err
	}
	op := thread.NewOperation(ctx)
	uplink, err := thread.OpenLink(id, backend, inst.page, meta, op)
	if errors.Is(err, thread.ErrLinked) {
		return op.Receive()
	}
	if err != nil {
		return err
	}
	setCmd := commandSetUplink{
		ctx:       ctx,
		backend:   backend,
		uplink:    uplink,
		Operation: op,
	}
	if err := inst.send(setCmd); err != nil {
		uplink.Close()
		return err
	}
	return op.Receive()
}

func (inst *instance) ThreadNew(ctx context.Context) error {
	backend := inst.backend.Load()
	if backend == nil {
		return api.ErrDisabled
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	if !inst.synced.Get() {
		return api.ErrUnlinked
	}
	op := thread.NewOperation(ctx)
	uplink := thread.NewLink(backend, inst.page)
	setCmd := commandSetUplink{
		ctx:       ctx,
		backend:   backend,
		uplink:    uplink,
		Operation: op,
	}
	if err := inst.send(setCmd); err != nil {
		uplink.Close()
		return err
	}
	return op.Receive()
}

func (inst *instance) ThreadRename(ctx context.Context, id api.ThreadID, name string) error {
	backend := inst.backend.Load()
	if backend == nil {
		return api.ErrDisabled
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	name, err := chatmod.CheckName(name)
	if err != nil {
		return err
	}
	if _, err := backend.Storage().ThreadGet(ctx, id); err != nil {
		return err
	}
	return thread.Rename(context.WithoutCancel(ctx), backend.Storage(), id, name)
}

func (inst *instance) ThreadClone(ctx context.Context, id api.ThreadID) (api.ThreadID, error) {
	backend := inst.backend.Load()
	if backend == nil {
		return "", api.ErrDisabled
	}
	if ctx.Err() != nil {
		return "", context.Canceled
	}
	clone, err := backend.Storage().ThreadClone(ctx, id, api.ThreadID(utils.NewID()))
	if err != nil {
		return "", err
	}
	files := backend.FileStorage()
	var f api.File
copyFiles:
	for _, origin := range []api.Origin{api.OriginUser, api.OriginAgent} {
		for f, err = range files.FileList(ctx, id, origin) {
			if err == nil && f.Draft {
				continue
			}
			var r io.ReadCloser
			if err == nil {
				r, err = files.FileRead(ctx, id, origin, f.Name, 0, -1)
			}
			if err == nil {
				_, err = files.FilePut(ctx, clone.ID, origin, f.Name, f.MIME, r, false)
				r.Close()
			}
			if err != nil {
				break copyFiles
			}
		}
	}
	if err != nil {
		cleanup := context.WithoutCancel(ctx)
		if derr := thread.Delete(cleanup, backend.Storage(), clone.ID); derr != nil {
			err = errors.Join(err, derr)
		}
		if derr := files.FileDeleteAll(cleanup, clone.ID); derr != nil {
			err = errors.Join(err, derr)
		}
		return "", err
	}
	return clone.ID, nil
}

func (inst *instance) ThreadDelete(ctx context.Context, id api.ThreadID) error {
	backend := inst.backend.Load()
	if backend == nil {
		return api.ErrDisabled
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	op := thread.NewOperation(ctx)
	if err := inst.send(commandDrop{ctx, id, op}); err != nil {
		return err
	}
	if err := op.Receive(); err != nil {
		return err
	}
	if err := thread.Delete(context.WithoutCancel(ctx), backend.Storage(), id); err != nil {
		return err
	}
	return backend.FileStorage().FileDeleteAll(context.WithoutCancel(ctx), id)
}

func (inst *instance) ThreadGet(ctx context.Context, id api.ThreadID) (api.Thread, error) {
	backend := inst.backend.Load()
	if backend == nil {
		return api.Thread{}, api.ErrDisabled
	}
	if ctx.Err() != nil {
		return api.Thread{}, context.Canceled
	}
	return backend.Storage().ThreadGet(ctx, id)
}

func (inst *instance) ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]api.Thread, error) {
	backend := inst.backend.Load()
	if backend == nil {
		return nil, api.ErrDisabled
	}
	if ctx.Err() != nil {
		return nil, context.Canceled
	}
	return backend.Storage().ThreadList(ctx, from, to, asc, limit, name)
}

func (inst *instance) RoundSearch(ctx context.Context, thread api.ThreadID, exclude bool, query string, from *api.Cursor, inclusive bool, asc bool, limit int) ([]api.Hit, error) {
	backend := inst.backend.Load()
	if backend == nil {
		return nil, api.ErrDisabled
	}
	if ctx.Err() != nil {
		return nil, context.Canceled
	}
	return backend.Storage().RoundSearch(ctx, thread, exclude, query, from, inclusive, asc, limit)
}

func (inst *instance) Render(factory api.ChatFactory) gox.Elem {
	door := new(doors.Door)
	return chat{
		factory: factory,
		inst:    inst,
		door:    door,
	}.Main()
}

func (inst *instance) Emit(ctx context.Context, name string, attrs map[string]string, data any) error {
	if inst.backend.Load() == nil {
		return api.ErrDisabled
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	a := make(map[string]any, len(attrs)+1)
	for k, v := range attrs {
		a[k] = v
	}
	a["name"] = name
	op := thread.NewOperation(ctx)
	if err := inst.send(commandRecord{"app_event", a, data, op}); err != nil {
		return err
	}
	return op.Receive()
}
