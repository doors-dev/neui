package testkit

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"sync"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/storage/local"
)

const threadID api.ThreadID = "thread"

type Setup struct {
	Model api.Model
	Files api.FilesConfig
}

type Controller struct {
	setup Setup
	ctx   context.Context
	fs    filesystem.FS
	store *local.FileStorage
	mu    sync.Mutex
	tasks []*Task
}

func NewController(t *testing.T, s Setup) *Controller {
	t.Helper()
	store, err := local.NewFileStorage()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen(threadID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	c := &Controller{setup: s, fs: filesystem.New(threadID, ws, store, common.FilesConfigDefaults(s.Files)), store: store}
	c.ctx = context.WithValue(context.Background(), common.KeyController, common.Controller(c))
	return c
}

func (c *Controller) Ctx() context.Context { return c.ctx }

func (c *Controller) Store() *local.FileStorage { return c.store }

func (c *Controller) Tasks() []*Task {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*Task(nil), c.tasks...)
}

func (c *Controller) ID() api.ThreadID { return threadID }

func (c *Controller) Context() context.Context { return context.Background() }

func (c *Controller) PageContext() context.Context { return context.Background() }

func (c *Controller) Modules() iter.Seq[common.Module] {
	return func(func(common.Module) bool) {}
}

func (c *Controller) Config() api.ThreadConfig { return common.ConfigDefaults(api.ThreadConfig{}) }

func (c *Controller) FilesConfig() api.FilesConfig { return common.FilesConfigDefaults(c.setup.Files) }

func (c *Controller) Model() api.Model { return c.setup.Model }

func (c *Controller) FS() api.FS { return c.fs }

func (c *Controller) Task(string, map[string]any, context.CancelCauseFunc) (common.Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	task := &Task{id: uint(len(c.tasks) + 1), noted: make(chan struct{}, 1)}
	c.tasks = append(c.tasks, task)
	return task, nil
}

func (c *Controller) Ask(api.ChatAsk) (func(context.Context), error) {
	return nil, errors.New("testkit: no asks")
}

func (c *Controller) Rename(context.Context, string) error { return nil }

type Note struct {
	Data json.RawMessage
	Meta map[string]any
}

type Task struct {
	id    uint
	mu    sync.Mutex
	notes []Note
	noted chan struct{}
}

func (t *Task) ID() uint { return t.id }

func (t *Task) Ok(json.RawMessage) {}

func (t *Task) Err(error) {}

func (t *Task) Notify(data json.RawMessage, meta map[string]any) {
	t.mu.Lock()
	t.notes = append(t.notes, Note{Data: data, Meta: meta})
	t.mu.Unlock()
	select {
	case t.noted <- struct{}{}:
	default:
	}
}

func (t *Task) Notes() []Note {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Note(nil), t.notes...)
}

func (t *Task) WaitNotes(tb testing.TB, n int) []Note {
	tb.Helper()
	guard := time.After(5 * time.Second)
	for {
		notes := t.Notes()
		if len(notes) > n {
			tb.Fatalf("task %d: want %d notes, got %d: %v", t.id, n, len(notes), notes)
		}
		if len(notes) == n {
			return notes
		}
		select {
		case <-t.noted:
		case <-guard:
			tb.Fatalf("task %d: want %d notes, got %d after 5s: %v", t.id, n, len(notes), notes)
		}
	}
}
