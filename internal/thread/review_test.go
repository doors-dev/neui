package thread

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
)

func TestLinkToAnotherBackendDropsThePendingInput(t *testing.T) {
	th, r, _, _ := taskThread(t, "switch-backend-pending")
	old := th.link.backend
	state := startRound(t, th, r, api.Input{Text: "first"})
	th.pending = true
	th.input = api.Input{Text: "queued"}
	other := common.NewBackend(th.link.ctx, old.Storage(), old.WorkspaceStorage(), old.FileStorage(), []common.Option{common.OptionProvider(old.Provider())})
	if _, err := openLink(state, commandLink{other, page.NewPage(context.Background()), true, nil, nil, NewResultResponse[Link]()}); err != nil {
		t.Fatalf("link to another backend: %v", err)
	}
	t.Cleanup(th.link.dispose)
	if th.pending {
		t.Fatalf("the input queued under the old backend is still pending on the new link")
	}
	if th.input.Text != "queued" {
		t.Fatalf("the queued input is %q, want it kept as the draft", th.input.Text)
	}
}

func TestStrayTaskResultStartsNoRound(t *testing.T) {
	th, r, sent, origin := taskThread(t, "stray-result")
	if err := r.Record(origin, "note", nil, "stashed"); err != nil {
		t.Fatalf("replay Record: %v", err)
	}
	wait := threadWait{thread: th}
	th.state = wait
	stray := commonTask{id: 42, link: th.link.linkTasks}
	wait.on(deliver(th, func() { stray.Err(api.ErrCanceledByUser) }))
	if _, ok := th.state.(threadWait); !ok || len(*sent) != 0 {
		t.Fatalf("a result from an unknown task left the thread in %T with %d requests sent", th.state, len(*sent))
	}
}

func TestFailedAbortOnStopIsCritical(t *testing.T) {
	th, r, _, _ := taskThread(t, "abort-fails")
	captureEvents(th)
	ch := th.link.ch
	state := startRound(t, th, r, api.Input{Text: "first"})
	thread, err := th.link.backend.Storage().ThreadGet(context.Background(), th.id)
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if _, err := th.link.backend.Storage().ThreadRename(context.Background(), th.id, thread.Seq, "drift"); err != nil {
		t.Fatalf("ThreadRename: %v", err)
	}
	state.on(commandStop{link: th.link, Operation: NewOperation(context.Background())})
	var critical bool
	close(ch)
	for m := range ch {
		e, ok := m.(MessageEvent)
		if !ok {
			continue
		}
		if _, ok := e.Event.(api.ChatEventCriticalError); ok {
			critical = true
		}
		if end, ok := e.Event.(api.ChatEventRoundEnd); ok && end.State == api.RoundAbort {
			t.Fatalf("Stop reported the round aborted although storing the abort failed")
		}
	}
	if !critical || th.link != nil {
		t.Fatalf("a failed abort on Stop was not a critical error")
	}
}

type hangingProvider struct {
	stubProvider
}

func (hangingProvider) Stream(ctx context.Context, _ api.Request) <-chan api.Event {
	ch := make(chan api.Event, 1)
	ch <- api.EventStart{ID: "c", Model: "stub"}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}

func TestCancelledCompactionReportsNothing(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	th, _, r := stubThread(t, "compact-cancelled", hangingProvider{stubProvider{model: api.Model{Name: "stub", Context: 1000}}}, config)
	th.close = make(chan struct{})
	for _, name := range []string{"one", "two"} {
		if _, _, err := r.RoundInit(api.Input{Text: name}); err != nil {
			t.Fatalf("replay RoundInit(%q): %v", name, err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: api.ContentText{Text: name}}); err != nil {
			t.Fatalf("replay RoundOk(%q): %v", name, err)
		}
	}
	captureEvents(th)
	started, err := th.compact()
	if err != nil || !started {
		t.Fatalf("compact: %v, %v", started, err)
	}
	th.compaction.stop()
	select {
	case cmd := <-th.ch:
		t.Fatalf("the cancelled compaction still reported %T", cmd)
	case <-time.After(200 * time.Millisecond):
	}
	close(th.close)
	if logs.Len() != 0 {
		t.Fatalf("the cancelled compaction logged %q", logs.String())
	}
}

func TestTaskWithoutCancelIsRefused(t *testing.T) {
	th, _, _, origin := taskThread(t, "task-nil-cancel")
	errs := make(chan error, 1)
	go func() {
		_, err := (controller{origin: origin, link: th.link}).Task("", nil, nil)
		errs <- err
	}()
	select {
	case <-th.ch:
		t.Fatalf("a task without a cancel func reached the thread")
	case err := <-errs:
		if err == nil {
			t.Fatalf("a task without a cancel func was accepted")
		}
	}
}

func TestCompactBeforeTheThreadIsStoredIsWrongState(t *testing.T) {
	th, messages, _ := stubThread(t, "compact-new", stubProvider{model: api.Model{Name: "stub"}}, api.ThreadConfig{})
	th.replay = replay.NewReplay(th.link.ctx, "compact-new-unsaved", messages, th.link.backend.Config(), th.link.backend.Provider().Model())
	op := NewOperation(context.Background())
	th.on(commandCompact{link: th.link, Operation: op})
	if err := op.Receive(); err != api.ErrWrongState {
		t.Fatalf("Compact on a thread not stored yet returned %v, want ErrWrongState", err)
	}
}

func TestEmptyRewindWithNothingPendingKeepsTheChat(t *testing.T) {
	th, messages, r := stubThread(t, "rewind-empty", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{})
	first, _, err := r.RoundInit(api.Input{Text: "one"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "ok"}}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	state := startRound(t, th, r, api.Input{Text: "two"})
	op := NewOperation(context.Background())
	state.on(commandRewind{link: th.link, round: first.ID, input: api.Input{}, Operation: op})
	if err := op.Receive(); err != api.ErrWrongState {
		t.Fatalf("an empty rewind with nothing pending answered %v, want ErrWrongState", err)
	}
	if th.link == nil || th.state != any(state) || state.ctx.Err() != nil {
		t.Fatalf("the refused rewind disturbed the running round: link %v, state %T, ctx %v", th.link != nil, th.state, state.ctx.Err())
	}
	stored, err := messages.RoundAfter(context.Background(), "rewind-empty", api.NoRoundID, 100)
	if err != nil || len(stored) != 2 {
		t.Fatalf("the refused rewind left %d rounds (%v), want 2", len(stored), err)
	}
}
