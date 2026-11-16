package thread

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	"neui.dev/internal/thread/toolkit"
	"neui.dev/internal/tools"
)

func stubThread(t *testing.T, id api.ThreadID, p api.Provider, config api.ThreadConfig) (*thread, api.MessageStorage, replay.Replay) {
	t.Helper()
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, config)
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	messages := local.NewMessageStorage()
	backend := common.NewBackend(ctx, messages, store, store, []common.Option{common.OptionProvider(p)})
	fs := filesystem.Open(id, store, store, api.FilesConfig{})
	t.Cleanup(func() { fs.Dispose() })
	linkCtx, linkCancel := context.WithCancelCause(ctx)
	linkCancel(nil)
	th := &thread{id: id, ch: make(chan Command)}
	th.link = &link{
		thread:     th,
		backend:    backend,
		fs:         fs,
		ctx:        ctx,
		cancel:     func() {},
		linkCtx:    linkCtx,
		linkCancel: linkCancel,
		ch:         make(chan Message),
		pageModule: page.NewPage(ctx).Module(),
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
	th.close = make(chan struct{})
	t.Cleanup(func() { close(th.close) })
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

func TestFailedRoundReleasesPendingInput(t *testing.T) {
	th, r, _, _ := taskThread(t, "fail-pending")
	events := captureEvents(th)
	state := startRound(t, th, r, api.Input{Text: "first"})
	th.pending = true
	th.input = api.Input{Text: "queued"}
	events()
	state.err(api.CodeBusy, nil, api.Indexed[api.Content]{}, api.EventUsage{})
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("a failed round left the thread in %T, want waiting", th.state)
	}
	if th.pending || !releasedPending(events()) {
		t.Fatalf("a failed round left pending=%v or did not tell the composer", th.pending)
	}
	if th.input.Text != "queued" {
		t.Fatalf("a failed round dropped the queued input, got %q", th.input.Text)
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
		fs:         old.fs,
		ctx:        old.ctx,
		cancel:     old.cancel,
		linkCtx:    old.linkCtx,
		linkCancel: old.linkCancel,
		ch:         make(chan Message),
		pageModule: old.pageModule,
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
	type issued struct {
		task common.Task
		err  error
	}
	issue := func(from api.RoundID) issued {
		result := make(chan issued, 1)
		go func() {
			task, err := controller{origin: from, link: th.link}.Task("", nil, func(error) {})
			result <- issued{task, err}
		}()
		tools.on(<-th.ch)
		return <-result
	}
	stale := issue(origin)
	if stale.err == nil {
		t.Fatalf("a task from round %v was accepted during the tools round %v", origin, tools.round.ID)
	}
	if len(th.link.taskList()) != 0 {
		t.Fatalf("the refused task was filed: %v", th.link.taskList())
	}
	own := issue(tools.round.ID)
	if own.err != nil {
		t.Fatalf("Task from the tools round: %v", own.err)
	}
	got, ok := th.link.taskGet(own.task.ID())
	if !ok || got.origin != tools.round.ID {
		t.Fatalf("the task was filed under round %v, want its tool's round %v", got.origin, tools.round.ID)
	}
}

type countingWorkspaces struct {
	api.WorkspaceStorage
	opened  *atomic.Int64
	deleted *atomic.Int64
}

func (c countingWorkspaces) WorkspaceOpen(id api.ThreadID) (api.Workspace, error) {
	c.opened.Add(1)
	return c.WorkspaceStorage.WorkspaceOpen(id)
}

func (c countingWorkspaces) WorkspaceDelete(id api.ThreadID) error {
	c.deleted.Add(1)
	return c.WorkspaceStorage.WorkspaceDelete(id)
}

func TestFailedOpenDisposesItsLink(t *testing.T) {
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{})
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	var opened, deleted atomic.Int64
	backend := common.NewBackend(ctx, local.NewMessageStorage(), countingWorkspaces{store, &opened, &deleted}, store, []common.Option{common.OptionProvider(testkit.NewProvider(api.Model{Name: "stub"}))})
	th := &thread{id: "open-missing", ch: make(chan Command)}
	if _, err := th.connect(backend, page.NewPage(ctx), true, nil); err == nil {
		t.Fatalf("opening a thread that is not in storage succeeded")
	}
	if opened.Load() != 0 || deleted.Load() != 0 {
		t.Fatalf("the failed open touched the workspace (%d opens, %d deletes)", opened.Load(), deleted.Load())
	}
	if th.link != nil {
		t.Fatalf("the failed open kept a link")
	}
}

func TestDisposedLinkRunsNoMoreTools(t *testing.T) {
	var calls atomic.Int64
	probe := tools.NewModule("probe", "")
	if _, err := probe.Add("run", "", func(context.Context, struct{}) (struct{}, error) {
		calls.Add(1)
		return struct{}{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	base := context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{ToolCallLimit: -1})
	ctx, cancel := context.WithCancel(base)
	cancel()
	l := &link{
		backend:     common.NewBackend(base, nil, nil, nil, []common.Option{common.OptionProvider(testkit.NewProvider(api.Model{}))}),
		ctx:         ctx,
		linkToolkit: toolkit.NewToolkit(base, func(context.Context) common.Module { return probe }),
	}
	use := []api.ContentToolUse{
		{ID: "a", Name: "probe__run", Input: json.RawMessage(`{}`)},
		{ID: "b", Name: "probe__run", Input: json.RawMessage(`{}`)},
	}
	ch, stop := l.useTools(api.NoRoundID, use)
	defer stop(nil)
	results := make(map[string]api.ContentToolResult, len(use))
	for r := range ch {
		results[r.ID] = r
	}
	if len(results) != len(use) {
		t.Fatalf("%d calls got %d results: every tool_use needs a matching tool_result", len(use), len(results))
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("the tool ran %d times after the link was disposed", got)
	}
	for _, u := range use {
		if results[u.ID].Error == api.ToolOk {
			t.Fatalf("call %s was answered ok after the link was disposed", u.ID)
		}
	}
}

func openLink(st interface{ on(any) }, cmd commandLink) (Link, error) {
	go st.on(cmd)
	return cmd.Receive()
}

func livePage(th *thread) {
	th.link.linkCtx, th.link.linkCancel = context.WithCancelCause(context.Background())
	th.link.page = page.NewPage(context.Background())
	th.link.toolkit = toolkit.NewToolkit(context.Background())
}

func TestOpenFromTheSamePageKeepsTheLink(t *testing.T) {
	th, r, _, _ := taskThread(t, "open-same-page")
	state := startRound(t, th, r, api.Input{Text: "call"})
	state.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}, api.EventUsage{})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	livePage(th)
	old := th.link
	cmd := commandLink{old.backend, old.page, true, "anchor", NewOperation(context.Background()), NewResultResponse[Link]()}
	go tools.on(cmd)
	reload, ok := (<-old.ch).(MessageReload)
	if !ok {
		t.Fatalf("a reopen from the same page sent no reload to the instance")
	}
	reload.Ack()
	_, err := cmd.Receive()
	if !errors.Is(err, ErrLinked) {
		t.Fatalf("a reopen from the same page returned %v, want ErrLinked", err)
	}
	if th.link != old || old.linkCtx.Err() != nil {
		t.Fatalf("a reopen from the same page replaced or cancelled the link")
	}
	if old.meta != "anchor" {
		t.Fatalf("the reopen meta is %v, want it on the link for the next init", old.meta)
	}
}

func TestOpenFromAnotherPageInheritsTheLink(t *testing.T) {
	th, _, _, _ := taskThread(t, "open-other-page")
	livePage(th)
	old := th.link
	link, err := openLink(threadWait{thread: th}, commandLink{old.backend, page.NewPage(context.Background()), true, nil, nil, NewResultResponse[Link]()})
	if err != nil {
		t.Fatalf("an open from another page: %v", err)
	}
	if link.inner == old || old.linkCtx.Err() == nil {
		t.Fatalf("an open from another page kept the old link")
	}
	if cause := context.Cause(old.linkCtx); !errors.Is(cause, common.ErrRelinked) {
		t.Fatalf("an open from another page ended the old page's work with %v, want the relink", cause)
	}
}

func TestRewindCancelsTheTasksOfTheCutRounds(t *testing.T) {
	th, r, _, first := taskThread(t, "rewind-tasks")
	second, _, err := r.RoundInit(api.Input{Text: "start another"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "started"}}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	var keptCause, cutCause error
	kept := th.link.taskAdd(first, "js", "run", "kept", nil, func(err error) { keptCause = err })
	cut := th.link.taskAdd(second.ID, "js", "run", "cut", nil, func(err error) { cutCause = err })
	events := captureEvents(th)
	wait := threadWait{thread: th}
	th.state = wait
	op := NewOperation(context.Background())
	wait.on(commandRewind{th.link, second.ID, api.Input{Text: "again"}, op})
	if err := op.Receive(); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	if !errors.Is(cutCause, api.ErrCanceledByUser) || keptCause != nil {
		t.Fatalf("the rewind cancelled the cut round's task with %v and the earlier one with %v", cutCause, keptCause)
	}
	tasks := th.link.taskList()
	if len(tasks) != 1 || tasks[0].ID != kept.ID() {
		t.Fatalf("the task list after the rewind is %v, want only the earlier round's task", tasks)
	}
	var listed bool
	for _, e := range events() {
		if e, ok := e.(api.ChatEventTasks); ok && len(e.Tasks) == 1 {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("the rewind did not send the shortened task list")
	}
	round, ok := th.state.(*threadRound)
	if !ok {
		t.Fatalf("the rewind left the thread in %T, want a round", th.state)
	}
	round.on(deliver(th, func() { cut.Ok(json.RawMessage(`"late"`)) }))
	if len(events()) != 0 || len(th.link.taskList()) != 1 {
		t.Fatalf("a result from the cancelled task was not dropped")
	}
}

func TestOnlyUserInputRoundsSweepDrafts(t *testing.T) {
	th, _, _, origin := taskThread(t, "sweep-drafts")
	files := th.link.backend.FileStorage()
	ctx := context.Background()
	if _, err := files.FilePut(ctx, th.id, api.OriginUser, "edit.png", "image/png", strings.NewReader("png"), true); err != nil {
		t.Fatalf("FilePut: %v", err)
	}
	task := th.link.taskAdd(origin, "js", "run", "", map[string]any{}, func(error) {})
	wait := threadWait{thread: th}
	th.state = wait
	wait.on(deliver(th, func() { task.Ok(json.RawMessage(`"done"`)) }))
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("a task result while idle left the thread in %T, want a round", th.state)
	}
	if f, err := files.FileStat(ctx, th.id, api.OriginUser, "edit.png"); err != nil || !f.Draft {
		t.Fatalf("the open edit's draft after a task-result round: %+v, %v, want it kept", f, err)
	}
	wait = threadWait{thread: th}
	th.state = wait
	op := NewOperation(ctx)
	wait.on(commandSend{link: th.link, Operation: op, input: api.Input{Text: "next"}})
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("a send left the thread in %T, want a round", th.state)
	}
	if _, err := files.FileStat(ctx, th.id, api.OriginUser, "edit.png"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("the leftover draft after a send: %v, want it swept", err)
	}
}

func TestToolWriteOpensTheWorkspaceAndDisposeRemovesIt(t *testing.T) {
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{})
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	var opened, deleted atomic.Int64
	backend := common.NewBackend(ctx, local.NewMessageStorage(), countingWorkspaces{store, &opened, &deleted}, store, []common.Option{common.OptionProvider(testkit.NewProvider(api.Model{Name: "stub"}))})
	th := &thread{id: "lazy-workspace", ch: make(chan Command)}
	l := newLink(th, backend, page.NewPage(ctx), nil)
	if opened.Load() != 0 {
		t.Fatalf("a new link opened its workspace")
	}
	fs := controller{link: l}.FS()
	f, err := fs.Create("a.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := fs.LocalPath(ctx, "a.txt")
	if err != nil {
		t.Fatalf("LocalPath: %v", err)
	}
	dir := filepath.Dir(file)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the write left no workspace: %v", err)
	}
	l.dispose()
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dispose left the workspace: %v", err)
	}
	if opened.Load() != 1 || deleted.Load() != 1 {
		t.Fatalf("workspace opened %d and deleted %d times, want 1 and 1", opened.Load(), deleted.Load())
	}
}
