package thread

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/replay"
	"neui.dev/internal/testkit"
	"neui.dev/internal/thread/response"
)

func compactionThread(t *testing.T, id api.ThreadID, model api.Model, rounds int) (*thread, replay.Replay, func() []api.ChatEvent) {
	t.Helper()
	config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionAllowed: 0.2, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	th, _, r := stubThread(t, id, hangingProvider{model}, config)
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
	if _, err := th.compact(false); err != nil {
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
	if _, err := th.compact(false); err != nil {
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
	if _, err := th.compact(false); err != nil {
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
	if _, err := th.compact(false); err != nil || !th.compaction.isActive() {
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
	if _, err := th.compact(false); err != nil || !th.compaction.isActive() {
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
	if _, err := th.compact(false); err != nil {
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

func growingThread(t *testing.T, id api.ThreadID, rounds int) (*thread, replay.Replay, func() []api.ChatEvent) {
	t.Helper()
	th, r, events := compactionThread(t, id, api.Model{Name: "stub", Context: 1000}, 0)
	for i := range rounds {
		if _, _, err := r.RoundInit(api.Input{Text: "q"}); err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: (i + 1) * 900 / rounds}, api.Indexed[api.Content]{0: api.ContentText{Text: "a"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
	}
	return th, r, events
}

func TestContextLengthCompactionIsHalvedInPlace(t *testing.T) {
	for _, tc := range []struct {
		name string
		resp []api.Event
	}{
		{"rejected", []api.Event{api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")}}},
		{"cut off", []api.Event{api.EventText{Index: 0, Text: "a summary cut"}, api.EventErr{Code: api.CodeContextLength, Err: errors.New("model_context_window_exceeded")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th, r, events := growingThread(t, api.ThreadID("halve-"+tc.name), 8)
			if _, err := th.compact(false); err != nil {
				t.Fatalf("compact: %v", err)
			}
			first := th.compaction.compaction
			events()
			th.on(commandApplyCompaction{resp: flushed(tc.resp...), seq: th.compaction.seq})
			if !th.compaction.isActive() {
				t.Fatalf("no smaller pass started")
			}
			if second := th.compaction.compaction; second.Target >= first.Target {
				t.Fatalf("the smaller pass targets %d, the first %d", second.Target, first.Target)
			}
			for _, e := range compacting(t, events()) {
				if !e.Running {
					t.Fatalf("the chip went off between the passes: %+v", e)
				}
			}
			th.on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeNetwork, Err: errors.New("reset")}), seq: th.compaction.seq})
			if r.NeedCompaction() != replay.CompactionNone {
				t.Fatalf("a failure other than the length did not arm the growth gate")
			}
		})
	}
}

func TestHalvingDoesNotArmTheGrowthGate(t *testing.T) {
	th, r, _ := growingThread(t, "halve-gate", 8)
	if _, err := th.compact(false); err != nil {
		t.Fatalf("compact: %v", err)
	}
	th.on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")}), seq: th.compaction.seq})
	th.compaction.stop()
	if r.NeedCompaction() == replay.CompactionNone {
		t.Fatalf("a halving armed the growth gate")
	}
}

func TestOneRoundCompactionTooLongEndsContextFull(t *testing.T) {
	th, r, events := compactionThread(t, "halve-full", api.Model{Name: "stub", Context: 1000}, 2)
	round, _, err := r.RoundInit(api.Input{Text: "too long"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := th.compact(true); err != nil || !th.compaction.isActive() {
		t.Fatalf("compact: %v", err)
	}
	state := th.switchStateCompact(round, api.EventUsage{}, nil)
	th.pending = true
	events()
	state.on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")}), seq: th.compaction.seq})
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("the thread is in %T, want idle", th.state)
	}
	if !r.IsContextFull() || th.canCompact() || th.pending {
		t.Fatalf("full %v CanCompact %v pending %v", r.IsContextFull(), th.canCompact(), th.pending)
	}
	send, retry := NewOperation(context.Background()), NewOperation(context.Background())
	th.state.(threadWait).on(commandSend{link: th.link, Operation: send, input: api.Input{Text: "more"}})
	th.state.(threadWait).on(commandRetry{link: th.link, Operation: retry})
	for _, op := range []Operation{send, retry} {
		if err := op.Receive(); !errors.Is(err, api.ErrContextFull) {
			t.Fatalf("got %v, want ErrContextFull", err)
		}
	}
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("a refused command left the thread in %T", th.state)
	}
}

func TestContextLengthWithNothingToCompactEndsContextFull(t *testing.T) {
	th, r, events := compactionThread(t, "halve-nothing", api.Model{Name: "stub", Context: 1000}, 1)
	state := startRound(t, th, r, api.Input{Text: "too long"})
	events()
	state.err(api.CodeContextLength, nil, nil, api.EventUsage{})
	if !r.IsContextFull() {
		t.Fatalf("the round ended without the context-full state")
	}
	var end []api.ChatEventRoundEnd
	for _, e := range events() {
		if e, ok := e.(api.ChatEventRoundEnd); ok {
			end = append(end, e)
		}
	}
	if len(end) != 1 || end[0].Error != string(api.CodeContextLength) || end[0].CanCompact {
		t.Fatalf("the round ended with %+v", end)
	}
}

func TestRecordWhileContextFullStartsNoRound(t *testing.T) {
	th, r, _ := growingThread(t, "full-record", 3)
	r.ContextFull()
	op := NewOperation(context.Background())
	th.state.(threadWait).on(commandRecord{link: th.link, Operation: op, origin: api.NoRoundID, kind: "note", data: "x"})
	if err := op.Receive(); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, ok := th.state.(threadWait); !ok || !r.IsPending() {
		t.Fatalf("the thread is in %T with the record pending %v, want idle with it pending", th.state, r.IsPending())
	}
}

func TestTokenCompactionAfterASuccessfulRoundIsNotCapped(t *testing.T) {
	th, r, _ := growingThread(t, "halve-episode", 8)
	startRound(t, th, r, api.Input{Text: "too long"}).err(api.CodeContextLength, nil, nil, api.EventUsage{})
	first := th.compaction.compaction
	th.state.(*threadCompact).on(commandApplyCompaction{resp: flushed(api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")}), seq: th.compaction.seq})
	if th.compaction.compaction.Target >= first.Target {
		t.Fatalf("the smaller pass targets %d, the first %d", th.compaction.compaction.Target, first.Target)
	}
	th.state.(*threadCompact).on(commandApplyCompaction{resp: flushed(api.EventText{Index: 0, Text: "the summary"}, api.EventOk{Code: api.CodeTurn}), seq: th.compaction.seq})
	answer := api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}
	th.state.(*threadRound).ok(api.CodeTurn, answer, api.EventUsage{Input: 600})
	last := startRound(t, th, r, api.Input{Text: "more"})
	last.ok(api.CodeTurn, answer, api.EventUsage{Input: 900})
	if !th.compaction.isActive() || th.compaction.compaction.Target != last.round.ID {
		t.Fatalf("the token-triggered pass targets %d, want the uncapped %d", th.compaction.compaction.Target, last.round.ID)
	}
}

func requestTokens(req api.Request) int {
	size := len(req.System)
	for _, m := range req.Messages {
		for _, c := range m.Content {
			if text, ok := c.(api.ContentText); ok {
				size += len(text.Text)
			}
		}
	}
	return size / 4
}

func TestOverflowOnASmallerModelCompactsInHalvedPasses(t *testing.T) {
	const window = 200_000
	turn := testkit.Then(func(req api.Request) testkit.Turn {
		if requestTokens(req) > window {
			return testkit.Events(api.EventStart{ID: "x", Model: "small"}, api.EventErr{Code: api.CodeContextLength, Err: errors.New("prompt is too long")})
		}
		text := "answer"
		if testkit.IsCompaction(req) {
			text = strings.Repeat("s", 8000)
		}
		return testkit.Events(api.EventStart{ID: "x", Model: "small"}, api.EventText{Index: 0, Text: text}, api.EventClose{Index: 0}, api.EventUsage{Input: requestTokens(req), Output: 2000}, api.EventOk{Code: api.CodeTurn})
	})
	p := testkit.NewProvider(api.Model{Name: "small", Context: window, RequestBytes: 32 << 20})
	for range 16 {
		p.Push(turn)
		p.OnCompaction(turn)
	}
	th, _, r := stubThread(t, "smaller-model", p, api.ThreadConfig{CompactionTrigger: 0.8, CompactionAllowed: 0.5, CompactionPreserve: 0.1, ReplayItemShare: 0.1, ThreadMemoryLimit: 18 << 20})
	th.close = make(chan struct{})
	t.Cleanup(func() {
		th.compaction.stop()
		close(th.close)
	})
	body := api.Indexed[api.Content]{0: api.ContentText{Text: strings.Repeat("x", 80_000)}}
	for i := range 40 {
		if _, _, err := r.RoundInit(api.Input{Text: fmt.Sprintf("q%d", i)}); err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: 20_000 * i, Output: 20_000}, body); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
	}
	th.switchStateWait()
	op := NewOperation(context.Background())
	th.state.(threadWait).on(commandSend{link: th.link, Operation: op, input: api.Input{Text: "continue"}})
	if err := op.Receive(); err != nil {
		t.Fatalf("Send: %v", err)
	}
	for i := 0; ; i++ {
		if _, ok := th.state.(threadWait); ok {
			break
		}
		if i == 1000 {
			t.Fatalf("the thread never settled, in %T", th.state)
		}
		th.state.tick()
	}
	var log []string
	for _, req := range p.Requests() {
		entry := fmt.Sprintf("R%dk", requestTokens(req)/1000)
		if testkit.IsCompaction(req) {
			entry = "C" + entry[1:]
		}
		if requestTokens(req) > window {
			entry += "!"
		}
		log = append(log, entry)
	}
	if got, want := strings.Join(log, " "), "R800k! C720k! C360k! C180k R622k! C182k R442k! C182k R262k! C182k R82k"; got != want {
		t.Fatalf("provider log\n got %s\nwant %s", got, want)
	}
}
