package thread

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
)

func stubThread(t *testing.T, id api.ThreadID, p api.Provider, config api.ThreadConfig) (*thread, api.MessageStorage, replay.Replay) {
	t.Helper()
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, config)
	store, err := local.NewFileStorage()
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	messages := local.NewMessageStorage()
	backend := common.NewBackend(ctx, messages, store, store, []common.Option{common.OptionProvider(p)})
	ws, err := store.WorkspaceOpen(id)
	if err != nil {
		t.Fatalf("WorkspaceOpen: %v", err)
	}
	t.Cleanup(func() { ws.Close() })
	linkCtx, linkCancel := context.WithCancel(ctx)
	linkCancel()
	th := &thread{id: id, ch: make(chan Command)}
	th.link = &link{
		thread:     th,
		backend:    backend,
		ws:         ws,
		fs:         filesystem.New(id, ws, store, api.FilesConfig{}),
		ctx:        ctx,
		cancel:     func() {},
		linkCtx:    linkCtx,
		linkCancel: linkCancel,
		ch:         make(chan Message),
		linkTasks:  &linkTasks{thread: th},
	}
	r := replay.NewReplay(ctx, id, messages, backend.Config(), p.Model())
	if _, err := r.Create(); err != nil {
		t.Fatalf("replay Create: %v", err)
	}
	th.replay = r
	return th, messages, r
}

func startRound(t *testing.T, th *thread, r replay.Replay, input api.Input) *threadRound {
	t.Helper()
	round, messages, err := r.RoundInit(input)
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	th.switchStateRoundReplay(round, messages)
	state, ok := th.state.(*threadRound)
	if !ok {
		t.Fatalf("the thread is in %T, want a round", th.state)
	}
	return state
}

func taskThread(t *testing.T, id api.ThreadID) (*thread, replay.Replay, *testkit.Provider, api.RoundID) {
	t.Helper()
	p := testkit.NewProvider(api.Model{Name: "stub"}, testkit.Say("ok"))
	th, _, r := stubThread(t, id, p, api.ThreadConfig{})
	round, _, err := r.RoundInit(api.Input{Text: "start the task"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "started"}}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	return th, r, p, round.ID
}

func deliver(th *thread, send func()) Command {
	go send()
	return <-th.ch
}

func captureEvents(th *thread) func() []api.ChatEvent {
	th.link.linkCtx = context.Background()
	th.link.ch = make(chan Message, 64)
	return func() []api.ChatEvent {
		var events []api.ChatEvent
		for {
			select {
			case m := <-th.link.ch:
				if e, ok := m.(MessageEvent); ok {
					events = append(events, e.Event)
				}
			default:
				return events
			}
		}
	}
}

func releasedPending(events []api.ChatEvent) bool {
	for _, e := range events {
		if p, ok := e.(api.ChatEventInputPending); ok && !p.Pending {
			return true
		}
	}
	return false
}

func TestStopDuringToolsReleasesPendingInput(t *testing.T) {
	th, r, _, _ := taskThread(t, "stop-tools-pending")
	events := captureEvents(th)
	state := startRound(t, th, r, api.Input{Text: "call"})
	state.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}, api.EventUsage{})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	th.pending = true
	events()
	op := NewOperation(context.Background())
	tools.on(commandStop{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if th.pending || !releasedPending(events()) {
		t.Fatalf("Stop during tools cleared pending=%v without telling the composer", th.pending)
	}
}

func TestStopDuringCompactionReleasesPendingInput(t *testing.T) {
	summary := []api.Event{
		api.EventStart{ID: "c", Model: "stub"},
		api.EventText{Index: 0, Text: "the summary"},
		api.EventOk{Code: api.CodeTurn},
	}
	compacting := testkit.NewProvider(api.Model{Name: "stub", Context: 1000}, testkit.Events(summary...))
	compacting.OnCompaction(testkit.Events(summary...))
	config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	th, _, r := stubThread(t, "stop-compact-pending", compacting, config)
	for _, name := range []string{"one", "two"} {
		if _, _, err := r.RoundInit(api.Input{Text: name}); err != nil {
			t.Fatalf("replay RoundInit(%q): %v", name, err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: api.ContentText{Text: name}}); err != nil {
			t.Fatalf("replay RoundOk(%q): %v", name, err)
		}
	}
	events := captureEvents(th)
	state := startRound(t, th, r, api.Input{Text: "three"})
	th.pending = true
	th.input = api.Input{Text: "next"}
	state.err(api.CodeContextLength, nil, api.Indexed[api.Content]{}, api.EventUsage{Input: 900})
	compact, ok := th.state.(*threadCompact)
	if !ok {
		t.Fatalf("a context-length error left the thread in %T, want the compaction", th.state)
	}
	events()
	op := NewOperation(context.Background())
	compact.on(commandStop{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if th.pending || !releasedPending(events()) {
		t.Fatalf("Stop during compaction cleared pending=%v without telling the composer", th.pending)
	}
	<-th.ch
}

func TestUnsendWhileWaitingReleasesPendingInput(t *testing.T) {
	th, _, _, _ := taskThread(t, "unsend-wait")
	events := captureEvents(th)
	th.pending = true
	th.input = api.Input{Text: "queued"}
	op := NewOperation(context.Background())
	threadWait{thread: th}.on(commandUnsend{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Unsend while waiting: %v", err)
	}
	if th.pending || !releasedPending(events()) {
		t.Fatalf("Unsend while waiting left pending=%v", th.pending)
	}
}

func TestCriticalErrorClearsPendingAndCompaction(t *testing.T) {
	th, _, _, _ := taskThread(t, "critical-reset")
	th.pending = true
	ctx, _ := th.compaction.start(context.Background(), replay.Compaction{})
	th.criticalErr(nil, errors.New("boom"))
	if th.pending {
		t.Fatalf("a critical error left the input pending for the next link")
	}
	if th.compaction.isActive() || ctx.Err() == nil {
		t.Fatalf("a critical error left the compaction streaming")
	}
}

func TestAskDoneAfterThreadExitReturns(t *testing.T) {
	th, _, _, _ := taskThread(t, "ask-done-closed")
	th.close = make(chan struct{})
	c := controller{link: th.link}
	type asked struct {
		done func(context.Context)
		err  error
	}
	result := make(chan asked, 1)
	go func() {
		done, err := c.Ask(api.ChatAsk{})
		result <- asked{done, err}
	}()
	cmd := (<-th.ch).(commandAsk)
	cmd.Ok(struct{}{})
	a := <-result
	if a.err != nil {
		t.Fatalf("Ask: %v", a.err)
	}
	close(th.close)
	returned := make(chan struct{})
	go func() {
		a.done(context.Background())
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatalf("done blocked after the thread exited")
	}
}

func TestAskFromBeforeReloadReachesTheNewPage(t *testing.T) {
	th, r, _, _ := taskThread(t, "ask-reload")
	state := startRound(t, th, r, api.Input{Text: "call"})
	state.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}, api.EventUsage{})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	old := th.link
	th.link = &link{
		thread:     th,
		backend:    old.backend,
		ws:         old.ws,
		fs:         old.fs,
		ctx:        old.ctx,
		cancel:     old.cancel,
		linkCtx:    old.linkCtx,
		linkCancel: old.linkCancel,
		ch:         make(chan Message),
		linkTasks:  old.linkTasks,
	}
	result := make(chan error, 1)
	go func() {
		_, err := controller{link: old}.Ask(api.ChatAsk{})
		result <- err
	}()
	tools.on(<-th.ch)
	if err := <-result; err != nil {
		t.Fatalf("an ask from a tool started before the reload: %v", err)
	}
	if tools.ask == nil {
		t.Fatalf("the ask did not reach the reloaded page")
	}
}

func TestTaskResultLeavesTheCallersAttrs(t *testing.T) {
	th, _, _, origin := taskThread(t, "result-attrs")
	meta := map[string]any{"island": "chart"}
	task := th.link.taskAdd(origin, "js", "run", "drawing", meta, func(error) {})
	th.on(deliver(th, func() { task.Err(errors.New("failed")) }))
	if len(meta) != 1 {
		t.Fatalf("the task result wrote into the caller's attrs: %v", meta)
	}
}

func TestTaskKeepsTheRoundOfItsTool(t *testing.T) {
	th, r, _, origin := taskThread(t, "task-origin")
	state := startRound(t, th, r, api.Input{Text: "call"})
	state.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}, api.EventUsage{})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	if tools.round.ID == origin {
		t.Fatalf("the tools round has the earlier round's id: the test cannot tell them apart")
	}
	result := make(chan common.Task, 1)
	go func() {
		task, err := controller{origin: origin, link: th.link}.Task("", nil, func(error) {})
		if err != nil {
			t.Errorf("Task: %v", err)
		}
		result <- task
	}()
	tools.on(<-th.ch)
	task := <-result
	got, ok := th.link.taskGet(task.ID())
	if !ok || got.origin != origin {
		t.Fatalf("the task was filed under round %v, want its tool's round %v", got.origin, origin)
	}
}

type countingWorkspaces struct {
	api.WorkspaceStorage
	deleted *atomic.Int64
}

func (c countingWorkspaces) WorkspaceDelete(id api.ThreadID) error {
	c.deleted.Add(1)
	return c.WorkspaceStorage.WorkspaceDelete(id)
}

func TestFailedOpenDisposesItsLink(t *testing.T) {
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{})
	store, err := local.NewFileStorage()
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	var deleted atomic.Int64
	backend := common.NewBackend(ctx, local.NewMessageStorage(), countingWorkspaces{store, &deleted}, store, []common.Option{common.OptionProvider(testkit.NewProvider(api.Model{Name: "stub"}))})
	th := &thread{id: "open-missing", ch: make(chan Command)}
	if _, err := th.connect(backend, page.NewPage(ctx), true, nil); err == nil {
		t.Fatalf("opening a thread that is not in storage succeeded")
	}
	if deleted.Load() != 1 {
		t.Fatalf("the failed open left its workspace open (%d deletes)", deleted.Load())
	}
	if th.link != nil {
		t.Fatalf("the failed open kept a link")
	}
}
