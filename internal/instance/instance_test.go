package instance

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance/ui"
	"neui.dev/internal/page"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/thread"
)

type stubProvider struct{}

func (stubProvider) Name() string { return "stub" }

func (stubProvider) Model() api.Model { return api.Model{Name: "stub"} }

func (stubProvider) Stream(context.Context, api.Request) <-chan api.Event {
	ch := make(chan api.Event)
	close(ch)
	return ch
}

func deadTestInstance(t *testing.T) *instance {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{}))
	t.Cleanup(cancel)
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	inst := &instance{
		ctx:     ctx,
		ch:      make(chan command),
		links:   make(map[*link]struct{}),
		dead:    make(map[*link]struct{}),
		page:    page.NewPage(ctx),
		state:   ui.NewChats().Add(ctx, "inst", api.InstanceState{}),
		current: doors.NewSource[*api.ThreadID](nil),
		synced:  doors.NewSource(true),
	}
	inst.backend.Store(common.NewBackend(ctx, local.NewMessageStorage(), store, store, []common.Option{common.OptionProvider(stubProvider{})}))
	inst.uplink = thread.NewLink(inst.backend.Load(), inst.page)
	t.Cleanup(func() { inst.uplink.Close() })
	return inst
}

func TestCriticalErrorKeepsThePaneAsADeadLinkWithoutReloadingIt(t *testing.T) {
	inst := deadTestInstance(t)
	pane := newLink(inst)
	pane.initialized = true
	inst.links[pane] = struct{}{}
	old := inst.uplink.ID()
	dropped := inst.uplink
	dropped.Close()

	got := make(chan thread.Message, 1)
	go func() {
		got <- <-pane.Ch()
	}()
	done := make(chan struct{})
	go func() {
		inst.onMessage(thread.MessageEvent{Event: api.ChatEventCriticalError{Err: errors.New("disk")}})
		inst.onMessage(thread.MessageEvent{Event: api.ChatEventTasks{}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the instance blocked sending to the dead pane: the fallback reloaded it or an event reached it")
	}
	m := <-got
	if e, ok := m.(thread.MessageEvent); !ok {
		t.Fatalf("the pane got %T before going dead", m)
	} else if _, ok := e.Event.(api.ChatEventCriticalError); !ok {
		t.Fatalf("the pane got %T instead of the critical error", e.Event)
	}
	if len(inst.links) != 0 {
		t.Fatalf("the pane is still live")
	}
	if _, ok := inst.dead[pane]; !ok {
		t.Fatalf("the pane was dropped instead of kept dead")
	}
	if !inst.uplink.Valid() || inst.uplink.ID() == old {
		t.Fatalf("the fallback did not move the instance to a fresh thread")
	}
	select {
	case _, ok := <-pane.Ch():
		if !ok {
			t.Fatal("the dead pane was disconnected")
		}
		t.Fatal("the dead pane got a message")
	default:
	}
}

func TestReloadLinksReloadsAndDisconnectsDeadLinks(t *testing.T) {
	inst := deadTestInstance(t)
	pane := newLink(inst)
	pane.initialized = true
	inst.dead[pane] = struct{}{}

	got := make(chan thread.Message, 1)
	closed := make(chan bool, 1)
	go func() {
		got <- <-pane.Ch()
		_, ok := <-pane.Ch()
		closed <- !ok
	}()
	inst.reloadLinks(thread.NewMessageReload(nil))
	if _, ok := (<-got).(thread.MessageReload); !ok {
		t.Fatal("the dead pane did not get the reload")
	}
	select {
	case ok := <-closed:
		if !ok {
			t.Fatal("the dead pane got a second message instead of being disconnected")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the dead pane was not disconnected")
	}
	if len(inst.dead) != 0 {
		t.Fatal("the dead set was not cleared")
	}
}

func TestDeadLinkCommandsAreUnlinked(t *testing.T) {
	inst := deadTestInstance(t)
	pane := newLink(inst)
	inst.dead[pane] = struct{}{}

	op := thread.NewOperation(context.Background())
	inst.onCommand(commandUplink{link: pane, Operation: op, make: func(uplink thread.Link, op thread.Operation) thread.Command {
		return uplink.Stop(op)
	}})
	if err := op.Receive(); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("a dead pane's command returned %v", err)
	}
	if inst.queue.Len() != 0 {
		t.Fatal("a dead pane's command reached the uplink")
	}
	inst.onCommand(commandUnlink{pane})
	if len(inst.dead) != 0 {
		t.Fatal("unlink left the dead pane in the set")
	}
}

func TestReloadFromTheUplinkReloadsThePanes(t *testing.T) {
	inst := deadTestInstance(t)
	pane := newLink(inst)
	pane.initialized = true
	inst.links[pane] = struct{}{}
	before := inst.uplink.ID()
	got := make(chan thread.Message, 1)
	go func() {
		got <- <-pane.Ch()
	}()
	op := thread.NewOperation(context.Background())
	inst.onMessage(thread.NewMessageReload(op))
	if err := op.Receive(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := (<-got).(thread.MessageReload); !ok {
		t.Fatalf("the pane was not reloaded")
	}
	if _, ok := inst.links[pane]; ok {
		t.Fatalf("the reloaded pane is still linked")
	}
	if inst.uplink.ID() != before {
		t.Fatalf("a reload replaced the uplink")
	}
}

func TestCriticalErrorReloadsAPaneThatWasNeverInitialised(t *testing.T) {
	inst := deadTestInstance(t)
	live := newLink(inst)
	live.initialized = true
	fresh := newLink(inst)
	inst.links[live] = struct{}{}
	inst.links[fresh] = struct{}{}
	dropped := inst.uplink
	dropped.Close()

	gotLive := make(chan thread.Message, 1)
	go func() {
		gotLive <- <-live.Ch()
	}()
	gotFresh := make(chan thread.Message, 1)
	closed := make(chan bool, 1)
	go func() {
		gotFresh <- <-fresh.Ch()
		_, ok := <-fresh.Ch()
		closed <- !ok
	}()
	done := make(chan struct{})
	go func() {
		inst.onMessage(thread.MessageEvent{Event: api.ChatEventCriticalError{Err: errors.New("disk")}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the critical error blocked the instance")
	}
	if e, ok := (<-gotLive).(thread.MessageEvent); !ok {
		t.Fatal("the initialised pane did not get the critical error")
	} else if _, ok := e.Event.(api.ChatEventCriticalError); !ok {
		t.Fatalf("the initialised pane got %T instead of the critical error", e.Event)
	}
	if _, ok := inst.dead[live]; !ok {
		t.Fatal("the initialised pane did not go dead")
	}
	if _, ok := (<-gotFresh).(thread.MessageReload); !ok {
		t.Fatal("the uninitialised pane did not get a reload")
	}
	if !<-closed {
		t.Fatal("the uninitialised pane was not disconnected after the reload")
	}
	if _, ok := inst.dead[fresh]; ok {
		t.Fatal("the uninitialised pane was kept dead")
	}
	select {
	case m := <-live.Ch():
		t.Fatalf("the dead pane also got %T", m)
	default:
	}
}

func TestUnsyncedInstanceRefusesUplinkCommands(t *testing.T) {
	inst := deadTestInstance(t)
	inst.synced.Update(context.Background(), false)
	ctx := context.Background()
	before := inst.uplink.ID()

	uplink := thread.NewLink(inst.backend.Load(), inst.page)
	op := thread.NewOperation(ctx)
	inst.onCommand(commandSetUplink{ctx, inst.backend.Load(), uplink, op})
	if err := op.Receive(); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("set uplink before sync returned %v", err)
	}
	op = thread.NewOperation(ctx)
	inst.onCommand(commandRecord{"app_event", nil, nil, op})
	if err := op.Receive(); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("record before sync returned %v", err)
	}
	op = thread.NewOperation(ctx)
	inst.onCommand(commandDrop{ctx, before, op})
	if err := op.Receive(); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("drop before sync returned %v", err)
	}
	if inst.uplink.ID() != before || inst.queue.Len() != 0 {
		t.Fatal("a command before sync reached the uplink")
	}
	if err := inst.ThreadNew(ctx); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("ThreadNew before sync returned %v", err)
	}
	if err := inst.ThreadOpen(ctx, before, nil); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("ThreadOpen before sync returned %v", err)
	}
}

type flakyWorkspaces struct {
	*local.FileStorage
	broken *atomic.Bool
}

func (w flakyWorkspaces) WorkspaceOpen(id api.ThreadID) (api.Workspace, error) {
	if w.broken.Load() {
		return api.Workspace{}, errors.New("disk")
	}
	return w.FileStorage.WorkspaceOpen(id)
}

func flakyBackend(t *testing.T, broken bool) (*common.Backend, string) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{}))
	t.Cleanup(cancel)
	dir := t.TempDir()
	store, err := local.NewFileStorage(dir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	var flag atomic.Bool
	flag.Store(broken)
	backend := common.NewBackend(ctx, local.NewMessageStorage(), flakyWorkspaces{store, &flag}, store, []common.Option{common.OptionProvider(stubProvider{})})
	return backend, dir
}

func workspaceDirs(t *testing.T, dir string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(dir, "*", "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestNewLinkSurvivesABrokenWorkspace(t *testing.T) {
	backend, _ := flakyBackend(t, true)
	link := thread.NewLink(backend, page.NewPage(backend.Context()))
	link.Close()
}

func TestATabThatNeverUsesFilesLeavesNoWorkspace(t *testing.T) {
	backend, dir := flakyBackend(t, false)
	link := thread.NewLink(backend, page.NewPage(backend.Context()))
	if found := workspaceDirs(t, dir); len(found) != 0 {
		t.Fatalf("a new tab made %v", found)
	}
	link.Close()
	if found := workspaceDirs(t, dir); len(found) != 0 {
		t.Fatalf("a closed tab left %v", found)
	}
}

func TestOpenLinkWithABrokenWorkspace(t *testing.T) {
	backend, _ := flakyBackend(t, true)
	if _, err := backend.Storage().ThreadCreate(context.Background(), "broken-workspace"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	open, err := thread.OpenLink("broken-workspace", backend, page.NewPage(backend.Context()), nil, nil)
	if err != nil {
		t.Fatalf("OpenLink with a broken workspace: %v", err)
	}
	open.Close()
	if _, err := thread.OpenLink("broken-workspace-missing", backend, page.NewPage(backend.Context()), nil, nil); err == nil {
		t.Fatalf("OpenLink of a thread that is not in storage succeeded")
	}
}

func TestRenameChecksTheCallersStorage(t *testing.T) {
	inst := deadTestInstance(t)
	ctx := context.Background()
	other := local.NewMessageStorage()
	if _, err := other.ThreadCreate(ctx, "rename-elsewhere"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	backend := common.NewBackend(inst.ctx, other, store, store, []common.Option{common.OptionProvider(stubProvider{})})
	open, err := thread.OpenLink("rename-elsewhere", backend, page.NewPage(inst.ctx), nil, nil)
	if err != nil {
		t.Fatalf("OpenLink: %v", err)
	}
	t.Cleanup(func() { open.Close() })
	if err := inst.ThreadRename(ctx, "rename-elsewhere", "stolen"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("a rename of a thread outside the caller's storage returned %v", err)
	}
	th, err := other.ThreadGet(ctx, "rename-elsewhere")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "" {
		t.Fatalf("the open thread was renamed to %q through another storage", th.Name)
	}
}

func TestThreadDeleteRemovesTheThreadsFiles(t *testing.T) {
	inst := deadTestInstance(t)
	ctx := context.Background()
	go func() {
		for {
			select {
			case <-inst.ctx.Done():
				return
			case cmd := <-inst.ch:
				inst.onCommand(cmd)
			}
		}
	}()
	backend := inst.backend.Load()
	if _, err := backend.Storage().ThreadCreate(ctx, "delete-files"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	if _, err := backend.FileStorage().FilePut(ctx, "delete-files", api.OriginUser, "a.txt", "text/plain", strings.NewReader("a"), false); err != nil {
		t.Fatalf("FilePut: %v", err)
	}
	if err := inst.ThreadDelete(ctx, "delete-files"); err != nil {
		t.Fatalf("ThreadDelete: %v", err)
	}
	if _, err := backend.FileStorage().FileStat(ctx, "delete-files", api.OriginUser, "a.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("the deleted thread's upload is still there: %v", err)
	}
}

func TestRenameRefusesEmptyAndNonPlainNames(t *testing.T) {
	inst := deadTestInstance(t)
	ctx := context.Background()
	storage := inst.backend.Load().Storage()
	if _, err := storage.ThreadCreate(ctx, "rename-plain"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	for _, name := range []string{"", "  ", "​", "a‮b", "a\nb", "a\x00b"} {
		if err := inst.ThreadRename(ctx, "rename-plain", name); err == nil {
			t.Errorf("%q: the rename was accepted", name)
		}
	}
	if err := inst.ThreadRename(ctx, "rename-plain", "  Trip plan "); err != nil {
		t.Fatalf("ThreadRename: %v", err)
	}
	th, err := storage.ThreadGet(ctx, "rename-plain")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "Trip plan" {
		t.Fatalf("the thread is named %q", th.Name)
	}
}

func TestDeadLinkReadsAreUnlinked(t *testing.T) {
	inst := deadTestInstance(t)
	pane := newLink(inst)
	pane.backend = inst.backend.Load()
	pane.thread = inst.uplink.ID()
	pane.initialized = true
	inst.links[pane] = struct{}{}
	ctx := context.Background()
	if _, err := pane.RoundBefore(ctx, 0, 10); errors.Is(err, api.ErrUnlinked) {
		t.Fatal("a live pane's read was unlinked")
	}
	inst.uplink.Close()
	inst.onCriticalErr()
	if _, err := pane.RoundBefore(ctx, 0, 10); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("a dead pane's RoundBefore returned %v", err)
	}
	if _, err := pane.RoundAfter(ctx, 0, 10); !errors.Is(err, api.ErrUnlinked) {
		t.Fatalf("a dead pane's RoundAfter returned %v", err)
	}
}

type failingPut struct {
	api.FileStorage
}

func (failingPut) FilePut(context.Context, api.ThreadID, api.Origin, string, string, io.Reader, bool) (api.File, error) {
	return api.File{}, errors.New("disk full")
}

func TestThreadCloneCopiesCommittedFiles(t *testing.T) {
	inst := deadTestInstance(t)
	ctx := context.Background()
	backend := inst.backend.Load()
	files := backend.FileStorage()
	if _, err := backend.Storage().ThreadCreate(ctx, "clone-files"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	for _, f := range []struct {
		origin api.Origin
		name   string
		draft  bool
	}{{api.OriginUser, "up.txt", false}, {api.OriginAgent, "out.txt", false}, {api.OriginUser, "draft.txt", true}} {
		if _, err := files.FilePut(ctx, "clone-files", f.origin, f.name, "text/plain", strings.NewReader(f.name), f.draft); err != nil {
			t.Fatalf("FilePut: %v", err)
		}
	}
	id, err := inst.ThreadClone(ctx, "clone-files")
	if err != nil {
		t.Fatalf("ThreadClone: %v", err)
	}
	for _, f := range []struct {
		origin api.Origin
		name   string
	}{{api.OriginUser, "up.txt"}, {api.OriginAgent, "out.txt"}} {
		got, err := files.FileStat(ctx, id, f.origin, f.name)
		if err != nil {
			t.Fatalf("the clone lacks %s/%s: %v", f.origin, f.name, err)
		}
		if got.Draft || got.MIME != "text/plain" || got.Size != int64(len(f.name)) {
			t.Fatalf("the clone's %s/%s is %+v", f.origin, f.name, got)
		}
		r, err := files.FileRead(ctx, id, f.origin, f.name, 0, -1)
		if err != nil {
			t.Fatalf("FileRead: %v", err)
		}
		b, _ := io.ReadAll(r)
		r.Close()
		if string(b) != f.name {
			t.Fatalf("the clone's %s/%s holds %q", f.origin, f.name, b)
		}
	}
	if _, err := files.FileStat(ctx, id, api.OriginUser, "draft.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("the clone got the draft: %v", err)
	}
}

func TestThreadCloneIsDeletedWhenCopyingFilesFails(t *testing.T) {
	inst := deadTestInstance(t)
	ctx := context.Background()
	backend := inst.backend.Load()
	if _, err := backend.Storage().ThreadCreate(ctx, "clone-fail"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	if _, err := backend.FileStorage().FilePut(ctx, "clone-fail", api.OriginUser, "a.txt", "text/plain", strings.NewReader("a"), false); err != nil {
		t.Fatalf("FilePut: %v", err)
	}
	store := backend.FileStorage()
	inst.backend.Store(common.NewBackend(inst.ctx, backend.Storage(), backend.WorkspaceStorage(), failingPut{store}, []common.Option{common.OptionProvider(stubProvider{})}))
	before, err := backend.Storage().ThreadList(ctx, time.Time{}, time.Now().Add(time.Hour), true, 100, "")
	if err != nil {
		t.Fatalf("ThreadList: %v", err)
	}
	if _, err := inst.ThreadClone(ctx, "clone-fail"); err == nil {
		t.Fatal("the clone succeeded without its files")
	}
	after, err := backend.Storage().ThreadList(ctx, time.Time{}, time.Now().Add(time.Hour), true, 100, "")
	if err != nil {
		t.Fatalf("ThreadList: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("the failed clone was left in storage: %d threads before, %d after", len(before), len(after))
	}
}
