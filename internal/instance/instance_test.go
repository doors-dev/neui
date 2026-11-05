package instance

import (
	"context"
	"errors"
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
