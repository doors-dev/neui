package thread

import (
	"context"
	"errors"
	"maps"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/replay"
	"neui.dev/internal/thread/response"
)

func compactionThread(t *testing.T, id api.ThreadID, model api.Model, rounds int) (*thread, replay.Replay, func() []api.ChatEvent) {
	t.Helper()
	config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionAllowed: 0.2, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	th, _, r := stubThread(t, id, hangingProvider{stubProvider{model: model}}, config)
	th.close = make(chan struct{})
	t.Cleanup(func() {
		th.compaction.stop()
		close(th.close)
	})
	for i := range rounds {
		if _, _, err := r.RoundInit(api.Input{Text: string(rune('a' + i))}); err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
	}
	th.switchStateWait()
	return th, r, captureEvents(th)
}

func flushed(events ...api.Event) response.Response {
	b := response.NewBulder()
	b.On(api.EventStart{ID: "c", Model: "stub"})
	for _, e := range events {
		b.On(e)
	}
	return b.Flush()
}

func compacting(t *testing.T, events []api.ChatEvent) []api.ChatEventCompacting {
	t.Helper()
	var out []api.ChatEventCompacting
	for _, e := range events {
		if c, ok := e.(api.ChatEventCompacting); ok {
			out = append(out, c)
		}
	}
	return out
}

func TestCompactWithOneRoundAndAnUnknownContextIsWrongState(t *testing.T) {
	th, _, events := compactionThread(t, "compact-one-round", api.Model{Name: "stub"}, 1)
	if th.getState().CanCompact {
		t.Fatalf("one round: the snapshot says CanCompact")
	}
	op := NewOperation(context.Background())
	th.on(commandCompact{link: th.link, Operation: op})
	if err := op.Receive(); err != api.ErrWrongState {
		t.Fatalf("Compact returned %v, want ErrWrongState", err)
	}
	if got := compacting(t, events()); len(got) != 0 {
		t.Fatalf("a refused Compact reported %+v", got)
	}
}

func TestCompactingEventsCarryCanCompact(t *testing.T) {
	th, _, events := compactionThread(t, "compact-flag", api.Model{Name: "stub", Context: 1000}, 2)
	if !th.getState().CanCompact {
		t.Fatalf("two rounds over the threshold: the snapshot says no CanCompact")
	}
	op := NewOperation(context.Background())
	th.on(commandCompact{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if got := compacting(t, events()); len(got) != 1 || !got[0].Running || got[0].CanCompact {
		t.Fatalf("start reported %+v, want one Running event without CanCompact", got)
	}
	if th.getState().CanCompact {
		t.Fatalf("a compaction is running: the snapshot says CanCompact")
	}
	th.on(commandApplyCompaction{resp: flushed(api.EventText{Index: 0, Text: "the summary"}, api.EventOk{Code: api.CodeTurn}), seq: th.compaction.seq})
	if got := compacting(t, events()); len(got) != 1 || got[0].Running || got[0].CanCompact {
		t.Fatalf("an applied compaction reported %+v", got)
	}
}

func TestCappedCompactionIsAppliedAsText(t *testing.T) {
	th, r, events := compactionThread(t, "compact-capped", api.Model{Name: "stub", Context: 1000}, 2)
	if _, err := th.compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	th.on(commandApplyCompaction{resp: flushed(api.EventText{Index: 0, Text: "a summary cut short"}, api.EventErr{Code: api.CodeCapped, Err: errors.New("capped")}), seq: th.compaction.seq})
	got := compacting(t, events())
	if len(got) != 2 || got[1].Running || got[1].CanCompact {
		t.Fatalf("a capped compaction reported %+v, want it applied", got)
	}
	if r.CanCompact() {
		t.Fatalf("the capped summary did not become the compaction")
	}
}

func TestFailedCompactionIsNotRetriggeredRightAway(t *testing.T) {
	th, r, events := compactionThread(t, "compact-failed", api.Model{Name: "stub", Context: 1000}, 2)
	if r.NeedCompaction() == replay.CompactionNone {
		t.Fatalf("the setup needs a compaction")
	}
	if _, err := th.compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	th.on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeRefused, Err: errors.New("no")}), seq: th.compaction.seq})
	got := compacting(t, events())
	if len(got) != 2 || got[1].Running || !got[1].CanCompact {
		t.Fatalf("a failed compaction reported %+v, want it ended with CanCompact", got)
	}
	if cause := r.NeedCompaction(); cause != replay.CompactionNone {
		t.Fatalf("right after a failed compaction NeedCompaction = %v", cause)
	}
	th.compaction = compaction{}
	if _, err := th.compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	th.on(commandApplyCompaction{resp: flushed(api.EventThinking{Index: 0, Text: "hm"}, api.EventOk{Code: api.CodeTurn}), seq: th.compaction.seq})
	if cause := r.NeedCompaction(); cause != replay.CompactionNone {
		t.Fatalf("after an empty summary NeedCompaction = %v", cause)
	}
}

func TestStopDuringACompactionAfterToolsEndsTheTurn(t *testing.T) {
	th, r, events := compactionThread(t, "compact-stop-tools", api.Model{Name: "stub", Context: 1000}, 2)
	round, _, err := r.RoundInit(api.Input{Text: "use a tool"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	use := api.ContentToolUse{ID: "t1", Name: "probe__run", Input: []byte(`{}`)}
	if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: use}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	results := map[string]api.ContentToolResult{"t1": api.NewContentToolResult("t1", api.UserContents{api.ContentText{Text: "done"}}, nil)}
	if err := r.RoundResult(results); err != nil {
		t.Fatalf("replay RoundResult: %v", err)
	}
	if _, err := th.compact(); err != nil || !th.compaction.isActive() {
		t.Fatalf("compact: %v", err)
	}
	held := round
	held.State = api.RoundOk
	held.Results = maps.Clone(results)
	th.pending = true
	state := th.switchStateCompact(held, api.EventUsage{}, held.Response)
	events()
	op := NewOperation(context.Background())
	state.on(commandStop{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("the thread is in %T after Stop, want idle", th.state)
	}
	if th.compaction.isActive() || th.pending {
		t.Fatalf("Stop left the compaction running or the input pending")
	}
	var final []api.ChatEvent
	for _, e := range events() {
		if api.Final(e) {
			final = append(final, e)
		}
		switch e.(type) {
		case api.ChatEventRoundEnd, api.ChatEventToolResults:
			t.Fatalf("Stop after the tools round repeated %+v", e)
		}
	}
	if len(final) != 1 || final[0] != (api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}, Final: true, CanCompact: th.canCompact()}) {
		t.Fatalf("Stop sent the Final events %+v, want one final Compacting", final)
	}
	if !r.IsPending() {
		t.Fatalf("Stop dropped the stashed tool results")
	}
}

func TestStopDuringACompactionAfterAnOkRoundEndsTheTurn(t *testing.T) {
	th, r, events := compactionThread(t, "compact-stop-ok", api.Model{Name: "stub", Context: 1000}, 2)
	round, _, err := r.RoundInit(api.Input{Text: "answer me"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	if _, err := th.compact(); err != nil || !th.compaction.isActive() {
		t.Fatalf("compact: %v", err)
	}
	held := round
	held.State = api.RoundOk
	th.pending = true
	th.input = api.Input{Text: "queued"}
	state := th.switchStateCompact(held, api.EventUsage{Input: 900}, held.Response)
	events()
	op := NewOperation(context.Background())
	state.on(commandStop{link: th.link, Operation: op})
	if err := op.Receive(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("the thread is in %T after Stop, want idle", th.state)
	}
	if th.compaction.isActive() || th.pending {
		t.Fatalf("Stop left the compaction running or the input pending")
	}
	got := events()
	var final []api.ChatEvent
	for _, e := range got {
		if api.Final(e) {
			final = append(final, e)
		}
		switch e.(type) {
		case api.ChatEventRoundEnd, api.ChatEventToolResults:
			t.Fatalf("Stop after the ok round repeated %+v", e)
		}
	}
	if len(final) != 1 || final[0] != (api.ChatEventCompacting{Running: false, Usage: api.EventUsage{}, Final: true, CanCompact: th.canCompact()}) {
		t.Fatalf("Stop sent the Final events %+v, want one final Compacting", final)
	}
	if !releasedPending(got) {
		t.Fatalf("the pending input was not released")
	}
}

func TestNotAppliedCompactionReleasesThePendingInput(t *testing.T) {
	th, r, events := compactionThread(t, "compact-pending", api.Model{Name: "stub", Context: 1000}, 2)
	round, _, err := r.RoundInit(api.Input{Text: "too long"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := th.compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	state := th.switchStateCompact(round, api.EventUsage{}, nil)
	th.pending = true
	th.input = api.Input{Text: "queued"}
	events()
	state.on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeRefused, Err: errors.New("no")}), seq: th.compaction.seq})
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("the thread is in %T, want idle", th.state)
	}
	if th.pending {
		t.Fatalf("the thread is idle with the input still pending")
	}
	if !releasedPending(events()) {
		t.Fatalf("the pending input was not released")
	}
}

func TestRewindRoundStartCarriesTheRemainingUsage(t *testing.T) {
	th, r, events := compactionThread(t, "compact-rewind", api.Model{Name: "stub", Context: 1000}, 0)
	var ids []api.RoundID
	for i, total := range []int{300, 600, 900} {
		round, _, err := r.RoundInit(api.Input{Text: string(rune('a' + i))})
		if err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: total}, api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
		ids = append(ids, round.ID)
	}
	op := NewOperation(context.Background())
	th.state.(threadWait).on(commandRewind{link: th.link, round: ids[2], input: api.Input{Text: "again"}, Operation: op})
	var start *api.ChatEventRoundStart
	for _, e := range events() {
		if s, ok := e.(api.ChatEventRoundStart); ok {
			start = &s
		}
	}
	if start == nil || start.Truncate != ids[2] || start.Usage.Total() != 600 || !start.CanCompact {
		t.Fatalf("the rewind started %+v, want usage 600 and CanCompact", start)
	}
	th.state.(*threadRound).cancel(nil)
	th.switchStateWait()
	op = NewOperation(context.Background())
	th.state.(threadWait).on(commandRewind{link: th.link, round: ids[1], input: api.Input{Text: "again"}, Operation: op})
	start = nil
	for _, e := range events() {
		if s, ok := e.(api.ChatEventRoundStart); ok {
			start = &s
		}
	}
	if start == nil || start.Usage.Total() != 300 || start.CanCompact {
		t.Fatalf("the rewind started %+v, want usage 300 without CanCompact", start)
	}
}
