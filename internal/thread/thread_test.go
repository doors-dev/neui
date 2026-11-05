package thread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/page"
	"neui.dev/internal/replay"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/thread/toolkit"
)

type probeTool struct {
	calls *atomic.Int64
}

func (p probeTool) Params() []byte { return []byte(`{"type":"object","properties":{}}`) }

func (p probeTool) Name() string { return "run" }

func (p probeTool) Desc() string { return "counts every dispatch it receives" }

func (p probeTool) Summary() string { return p.Desc() }

func (p probeTool) Output() []byte { return []byte(`{}`) }

func (p probeTool) Dispatched(ctx context.Context, arg json.RawMessage) (any, error) {
	return p.Agent(ctx, arg)
}

func (p probeTool) Agent(ctx context.Context, input json.RawMessage) (api.UserContents, error) {
	p.calls.Add(1)
	return api.UserContents{api.ContentText{Text: "ran " + string(input)}}, nil
}

func (p probeTool) JS(ctx context.Context) quickjs.HostFunc { return nil }

type probeModule struct {
	tool probeTool
}

func (p probeModule) Name() string { return "probe" }

func (p probeModule) Desc() string { return "" }

func (p probeModule) List() []common.Tool { return []common.Tool{p.tool} }

func (p probeModule) Get(name string) (common.Tool, bool) {
	if name != p.tool.Name() {
		return nil, false
	}
	return p.tool, true
}

func probeLink(limit int, calls *atomic.Int64) *link {
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, api.ThreadConfig{ToolCallLimit: limit})
	m := probeModule{probeTool{calls}}
	return &link{
		backend:     common.NewBackend(ctx, nil, nil, nil, nil),
		linkToolkit: toolkit.NewToolkit(ctx, func(context.Context) common.Module { return m }),
	}
}

func useProbe(t *testing.T, l *link, use []api.ContentToolUse) map[string]api.ContentToolResult {
	t.Helper()
	ch, cancel := l.useTools(api.NoRoundID, use)
	defer cancel(nil)
	results := make(map[string]api.ContentToolResult, len(use))
	for r := range ch {
		results[r.ID] = r
	}
	if len(results) != len(use) {
		t.Fatalf("%d calls got %d results: every tool_use needs a matching tool_result", len(use), len(results))
	}
	return results
}

func TestToolCallLimitAnswersEveryCallPastIt(t *testing.T) {
	const limit = 3
	var calls atomic.Int64
	l := probeLink(limit, &calls)
	use := make([]api.ContentToolUse, 0, limit+2)
	for i := range limit + 2 {
		use = append(use, api.ContentToolUse{ID: fmt.Sprintf("c%d", i), Name: "probe__run", Input: json.RawMessage(`{}`)})
	}
	results := useProbe(t, l, use)
	if got := calls.Load(); got != limit {
		t.Fatalf("the tool ran %d times, want %d: calls past the limit must not run", got, limit)
	}
	for i, u := range use {
		r := results[u.ID]
		if i < limit {
			if r.Error != api.ToolOk {
				t.Fatalf("call %d is within the limit and failed: %#v", i, r.Content)
			}
			continue
		}
		if r.Error != api.ToolErrRuntime {
			t.Fatalf("call %d is past the limit and was answered as %q", i, r.Error)
		}
		text, ok := r.Content[0].(api.ContentText)
		if !ok || !strings.Contains(text.Text, strconv.Itoa(limit)) {
			t.Fatalf("call %d past the limit: %#v does not name the limit", i, r.Content)
		}
	}
}

func TestDiscardedArgumentsAnswerWithoutRunning(t *testing.T) {
	var calls atomic.Int64
	l := probeLink(32, &calls)
	use := []api.ContentToolUse{
		{ID: "empty-object", Name: "probe__run", Input: json.RawMessage(`{}`)},
		{ID: "no-arguments", Name: "probe__run"},
		{ID: "discarded", Name: "probe__run", Input: api.DiscardedArguments},
	}
	results := useProbe(t, l, use)
	if got := calls.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2: a call sent no arguments runs on {}, a discarded accumulation does not run", got)
	}
	if r := results["empty-object"]; r.Error != api.ToolOk {
		t.Fatalf("a call carrying {} failed: %#v", r.Content)
	}
	if r := results["no-arguments"]; r.Error != api.ToolOk {
		t.Fatalf("a call that was sent no arguments failed: %#v", r.Content)
	} else if text, ok := r.Content[0].(api.ContentText); !ok || text.Text != "ran {}" {
		t.Fatalf("a call that was sent no arguments reached the tool as %#v, want {}", r.Content)
	}
	r := results["discarded"]
	if r.Error != api.ToolErrJSON {
		t.Fatalf("a call whose arguments were discarded was answered as %q, want %q", r.Error, api.ToolErrJSON)
	}
	want := (api.DiscardedArgumentsError{}).Error()
	if text, ok := r.Content[0].(api.ContentText); !ok || text.Text != want {
		t.Fatalf("discarded call answered with %#v, want %q", r.Content, want)
	}
}

func TestRenameUnlinkedThread(t *testing.T) {
	ctx := context.Background()
	s := local.NewMessageStorage()
	created, err := s.ThreadCreate(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	if err := Rename(ctx, s, "rename-cold", "named"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	th, err := s.ThreadGet(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "named" {
		t.Fatalf("name after Rename: got %q, want %q", th.Name, "named")
	}
	if th.Seq != created.Seq+1 {
		t.Fatalf("seq after Rename: got %d, want %d", th.Seq, created.Seq+1)
	}
	if _, ok := registry.Load(api.ThreadID("rename-cold")); !ok {
		t.Fatalf("Rename left no thread in the registry")
	}
	if err := Rename(ctx, s, "rename-cold", ""); err != nil {
		t.Fatalf("Rename back to unnamed: %v", err)
	}
	th, err = s.ThreadGet(ctx, "rename-cold")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if th.Name != "" || th.Seq != created.Seq+2 {
		t.Fatalf("thread after the second Rename: %+v", th)
	}
	if err := Rename(ctx, s, "rename-ghost", "x"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Rename of a thread that is not in storage: want ErrNotFound, got %v", err)
	}
}

func TestDeleteUnlinkedThread(t *testing.T) {
	ctx := context.Background()
	s := local.NewMessageStorage()
	if _, err := s.ThreadCreate(ctx, "delete-cold"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	// the thread runs before the command arrives, so the delete routes through it
	getThread("delete-cold")
	if err := Delete(ctx, s, "delete-cold"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.ThreadGet(ctx, "delete-cold"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadGet after Delete: want ErrNotFound, got %v", err)
	}
	if err := Delete(ctx, s, "delete-cold"); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if err := Delete(ctx, s, "delete-ghost"); err != nil {
		t.Fatalf("Delete of a thread that was never there: %v", err)
	}
}

func TestToolRoundEndWipesScratch(t *testing.T) {
	ctx := context.Background()
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	messages := local.NewMessageStorage()
	backend := common.NewBackend(ctx, messages, store, store, nil)
	const id api.ThreadID = "scratch-round"
	ws, err := store.WorkspaceOpen(id)
	if err != nil {
		t.Fatalf("WorkspaceOpen: %v", err)
	}
	t.Cleanup(func() { ws.Close() })
	fs := filesystem.New(id, ws, store, api.FilesConfig{})
	scratch, err := fs.TempDir()
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "unzipped.xml"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep, err := fs.Create("keep.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(keep, "keep"); err != nil {
		t.Fatal(err)
	}
	if err := keep.Close(); err != nil {
		t.Fatal(err)
	}
	linkCtx, linkCancel := context.WithCancel(ctx)
	linkCancel()
	th := &thread{id: id, ch: make(chan Command)}
	th.link = &link{
		thread:     th,
		backend:    backend,
		ws:         ws,
		fs:         fs,
		ctx:        ctx,
		cancel:     func() {},
		linkCtx:    linkCtx,
		linkCancel: linkCancel,
		ch:         make(chan Message),
		linkTasks:  &linkTasks{thread: th},
	}
	r := replay.NewReplay(ctx, id, messages, backend.Config(), api.Model{})
	if _, err := r.Create(); err != nil {
		t.Fatalf("replay Create: %v", err)
	}
	if _, _, err := r.RoundInit(api.Input{Text: "go"}); err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{}, nil); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	th.replay = r
	s := &threadTools{thread: th, stopped: true, results: map[string]api.ContentToolResult{}}
	s.finalize()
	if _, err := os.Stat(scratch); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch survived the round: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(ws.Dir, "keep.txt")); err != nil || string(b) != "keep" {
		t.Fatalf("workspace file after the round: %q, %v", b, err)
	}
}

type stubProvider struct {
	model  api.Model
	events []api.Event
}

func (stubProvider) Name() string { return "stub" }

func (p stubProvider) Model() api.Model { return p.model }

func (p stubProvider) Stream(context.Context, api.Request) <-chan api.Event {
	ch := make(chan api.Event, len(p.events))
	for _, e := range p.events {
		ch <- e
	}
	close(ch)
	return ch
}

type recordingProvider struct {
	stubProvider
	sent *[]api.Request
}

func (p recordingProvider) Stream(ctx context.Context, request api.Request) <-chan api.Event {
	*p.sent = append(*p.sent, request)
	return p.stubProvider.Stream(ctx, request)
}

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

func TestProviderRefusalOfMeasuredBytesIsTerminal(t *testing.T) {
	const sent, declared = 33_000_000, 32_000_000
	model := api.Model{Name: "stub", RequestBytes: declared}
	run := func(t *testing.T, id api.ThreadID, cause error) (*thread, replay.Replay) {
		t.Helper()
		p := stubProvider{model, []api.Event{api.EventErr{Code: api.CodeTooLarge, Err: cause}}}
		th, _, r := stubThread(t, id, p, api.ThreadConfig{})
		state := startRound(t, th, r, api.Input{Text: "go"})
		for i := 0; i < 8 && th.state == any(state); i++ {
			state.tick()
		}
		if th.state == any(state) {
			t.Fatalf("the round never ended: the stream carried one error and closed")
		}
		return th, r
	}

	refused := fmt.Errorf("stub: the request is %d bytes, over the %d the model %s takes", sent, declared, model.Name)
	th, r := run(t, "too-large-local", refused)
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("a refusal this process made itself left the thread in %T, want the round sent again under the tightened budget", th.state)
	}
	if r.Tighten() {
		t.Fatalf("Tighten was never spent: the retried round went out on the budget that was just refused")
	}

	measured := fmt.Errorf("%w: sent %d bytes, measured against the %d declared for %s: stub: 413", api.ErrMeasuredTooLarge, sent, declared, model.Name)
	th, r = run(t, "too-large-provider", measured)
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("a 413 answering a request measured as fitting left the thread in %T, want the round ended: nothing it sends instead is smaller enough", th.state)
	}
	if !r.Tighten() {
		t.Fatalf("Tighten was spent on a refusal no tightening fixes: the thread stays degraded for the rest of its life")
	}
	if !r.IsRetriable() {
		t.Fatalf("the round the provider refused is not retriable: the user cannot send it again by hand")
	}
}

func TestOversizedInputIsRefusedWhereItIsSubmitted(t *testing.T) {
	const limit = 4096
	ctx := context.Background()
	oversized := api.Input{Text: strings.Repeat("x", limit+1)}
	rounds := func(t *testing.T, s api.MessageStorage, id api.ThreadID) int {
		t.Helper()
		stored, err := s.RoundAfter(ctx, id, api.NoRoundID, 100)
		if err != nil {
			t.Fatalf("RoundAfter: %v", err)
		}
		return len(stored)
	}
	drafted := func(th *thread) bool {
		return th.input.Text != "" || len(th.input.Attachments) != 0
	}

	t.Run("draft", func(t *testing.T) {
		th, messages, _ := stubThread(t, "input-draft", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		op := NewOperation(ctx)
		th.on(commandInput{link: th.link, input: oversized, Operation: op})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte draft under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if drafted(th) {
			t.Fatalf("the refused draft was stored as %+v: a refused input has to leave no trace", th.input)
		}
		if n := rounds(t, messages, "input-draft"); n != 0 {
			t.Fatalf("the refused draft left %d rounds in storage", n)
		}
	})

	t.Run("send", func(t *testing.T) {
		th, messages, r := stubThread(t, "input-send", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		wait := threadWait{thread: th}
		th.state = wait
		op := NewOperation(ctx)
		wait.on(commandSend{link: th.link, Operation: op, input: oversized})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte send under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if drafted(th) || th.pending {
			t.Fatalf("the refused send left input %+v pending=%v: a refused input has to leave no trace", th.input, th.pending)
		}
		if _, ok := th.state.(threadWait); !ok {
			t.Fatalf("a refused send left the thread in %T, want it still waiting: no round may start on an input that cannot be sent", th.state)
		}
		if n := rounds(t, messages, "input-send"); n != 0 {
			t.Fatalf("the refused send created %d rounds in storage", n)
		}
		if r.IsRetriable() {
			t.Fatalf("the refused send left a retriable round behind")
		}
	})

	t.Run("rewind", func(t *testing.T) {
		th, messages, r := stubThread(t, "input-rewind", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		first, _, err := r.RoundInit(api.Input{Text: "one"})
		if err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "ok"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
		if _, _, err := r.RoundInit(api.Input{Text: "two"}); err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "ok"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
		wait := threadWait{thread: th}
		th.state = wait
		op := NewOperation(ctx)
		wait.on(commandRewind{link: th.link, round: first.ID, input: oversized, Operation: op})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a rewind carrying a %d-byte input under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if n := rounds(t, messages, "input-rewind"); n != 2 {
			t.Fatalf("the refused rewind left %d rounds, want the 2 it was asked to cut back through: a refused input may not cut anything", n)
		}
		if _, ok := th.state.(threadWait); !ok {
			t.Fatalf("a refused rewind left the thread in %T, want it still waiting", th.state)
		}
	})

	t.Run("rewind during a round", func(t *testing.T) {
		th, messages, r := stubThread(t, "input-rewind-round", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		first, _, err := r.RoundInit(api.Input{Text: "one"})
		if err != nil {
			t.Fatalf("replay RoundInit: %v", err)
		}
		if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "ok"}}); err != nil {
			t.Fatalf("replay RoundOk: %v", err)
		}
		state := startRound(t, th, r, api.Input{Text: "two"})
		op := NewOperation(ctx)
		state.on(commandRewind{link: th.link, round: first.ID, input: oversized, Operation: op})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a rewind carrying a %d-byte input under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if state.ctx.Err() != nil {
			t.Fatalf("the refused rewind canceled the round that was streaming: %v", context.Cause(state.ctx))
		}
		if th.state != any(state) {
			t.Fatalf("a refused rewind left the thread in %T, want the round it was already running", th.state)
		}
		if n := rounds(t, messages, "input-rewind-round"); n != 2 {
			t.Fatalf("the refused rewind left %d rounds, want the 2 it was asked to cut back through", n)
		}
	})

	t.Run("at the limit", func(t *testing.T) {
		th, _, _ := stubThread(t, "input-fits", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		fits := api.Input{Text: strings.Repeat("x", limit)}
		op := NewOperation(ctx)
		th.on(commandInput{link: th.link, input: fits, Operation: op})
		if err := op.Receive(); err != nil {
			t.Fatalf("an input of exactly the %d-byte limit answered %v, want it accepted", limit, err)
		}
		if th.input.Text != fits.Text {
			t.Fatalf("an accepted draft of %d bytes was stored as %+v", len(fits.Text), th.input)
		}
	})

	t.Run("attachments count toward the limit", func(t *testing.T) {
		th, _, _ := stubThread(t, "input-attach", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: limit})
		text := strings.Repeat("x", limit-10)
		op := NewOperation(ctx)
		th.on(commandInput{link: th.link, input: api.Input{Text: text}, Operation: op})
		if err := op.Receive(); err != nil {
			t.Fatalf("a %d-byte text under a %d limit answered %v: the text alone has to fit for this test to say anything", len(text), limit, err)
		}
		attached := api.Input{Text: text, Attachments: []string{"notes.txt"}}
		op = NewOperation(ctx)
		th.on(commandInput{link: th.link, input: attached, Operation: op})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte text plus the file link its attachment renders as answered %v, want ErrInputTooLarge", len(text), err)
		}
		if len(th.input.Attachments) != 0 {
			t.Fatalf("the refused draft was stored as %+v", th.input)
		}
	})

	t.Run("non-positive takes the default", func(t *testing.T) {
		th, _, _ := stubThread(t, "input-default", stubProvider{api.Model{Name: "stub"}, nil}, api.ThreadConfig{InputBytes: -1})
		if got := th.link.backend.Config().InputBytes; got != 256<<10 {
			t.Fatalf("a negative InputBytes resolved to %d, want the 256 KiB default: the input ceiling has no off switch", got)
		}
		op := NewOperation(ctx)
		th.on(commandInput{link: th.link, input: api.Input{Text: strings.Repeat("x", 256<<10+1)}, Operation: op})
		if err := op.Receive(); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte draft under a negative InputBytes answered %v, want ErrInputTooLarge", 256<<10+1, err)
		}
		if drafted(th) {
			t.Fatalf("the refused draft was stored under a negative InputBytes: %d bytes", len(th.input.Text))
		}
		op = NewOperation(ctx)
		th.on(commandInput{link: th.link, input: api.Input{Text: strings.Repeat("x", 256<<10)}, Operation: op})
		if err := op.Receive(); err != nil {
			t.Fatalf("a draft of exactly 256 KiB under a negative InputBytes answered %v, want it accepted", err)
		}
	})
}

func TestByteDrivenCompactionBlocksTheRoundItPrecedes(t *testing.T) {
	summary := []api.Event{
		api.EventStart{ID: "c", Model: "stub"},
		api.EventText{Index: 0, Text: "the summary"},
		api.EventOk{Code: api.CodeTurn},
	}
	heavy := api.Indexed[api.Content]{0: api.ContentText{Text: strings.Repeat("x", 40_000)}}
	grow := func(t *testing.T, r replay.Replay, usage api.EventUsage) {
		t.Helper()
		for _, name := range []string{"one", "two"} {
			if _, _, err := r.RoundInit(api.Input{Text: name}); err != nil {
				t.Fatalf("replay RoundInit(%q): %v", name, err)
			}
			if _, err := r.RoundOk(usage, heavy); err != nil {
				t.Fatalf("replay RoundOk(%q): %v", name, err)
			}
		}
	}

	t.Run("bytes", func(t *testing.T) {
		config := api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 2}
		th, messages, r := stubThread(t, "compact-bytes", stubProvider{api.Model{Name: "stub"}, summary}, config)
		grow(t, r, api.EventUsage{Input: 10})
		state := startRound(t, th, r, api.Input{Text: "three"})
		th.pending = true
		th.input = api.Input{Text: "next"}
		state.ok(api.CodeTurn, heavy, api.EventUsage{Input: 10})
		if got := r.NeedCompaction(); got != replay.CompactionBytes {
			t.Fatalf("the thread reports %v: it has to be over its byte budget for this test to say anything", got)
		}
		compact, ok := th.state.(*threadCompact)
		if !ok {
			t.Fatalf("a round over the byte budget left the thread in %T, want it blocked until the compaction applies", th.state)
		}
		compact.on(<-th.ch)
		if _, ok := th.state.(*threadRound); !ok {
			t.Fatalf("the blocked thread is in %T after the compaction applied, want the round it was holding", th.state)
		}
		record, err := messages.CompactionGet(context.Background(), "compact-bytes", api.NoRoundID)
		if err != nil {
			t.Fatalf("CompactionGet: %v", err)
		}
		if record.Text != "the summary" {
			t.Fatalf("stored compaction %+v: the round went out before the compaction it was waiting for", record)
		}
	})

	t.Run("tokens", func(t *testing.T) {
		config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
		th, _, r := stubThread(t, "compact-tokens", stubProvider{api.Model{Name: "stub", Context: 1000}, summary}, config)
		grow(t, r, api.EventUsage{Input: 900})
		state := startRound(t, th, r, api.Input{Text: "three"})
		th.pending = true
		th.input = api.Input{Text: "next"}
		state.ok(api.CodeTurn, heavy, api.EventUsage{Input: 900})
		if _, ok := th.state.(*threadRound); !ok {
			t.Fatalf("a token-driven compaction left the thread in %T, want the next round under way while it streams", th.state)
		}
		<-th.ch
	})
}

func TestByteDrivenCompactionBlocksTheRoundAfterTheTools(t *testing.T) {
	summary := []api.Event{
		api.EventStart{ID: "c", Model: "stub"},
		api.EventText{Index: 0, Text: "the summary"},
		api.EventOk{Code: api.CodeTurn},
	}
	call := api.Indexed[api.Content]{0: api.ContentToolUse{ID: "big", Name: "probe__run", Input: json.RawMessage(`{}`)}}
	result := map[string]api.ContentToolResult{
		"big": {ID: "big", Content: api.UserContents{api.ContentText{Text: strings.Repeat("x", 40_000)}}},
	}
	grow := func(t *testing.T, r replay.Replay, usage api.EventUsage) {
		t.Helper()
		for _, name := range []string{"one", "two"} {
			if _, _, err := r.RoundInit(api.Input{Text: name}); err != nil {
				t.Fatalf("replay RoundInit(%q): %v", name, err)
			}
			if _, err := r.RoundOk(usage, api.Indexed[api.Content]{0: api.ContentText{Text: strings.Repeat("x", 20_000)}}); err != nil {
				t.Fatalf("replay RoundOk(%q): %v", name, err)
			}
		}
	}
	toolRound := func(t *testing.T, th *thread, r replay.Replay, usage api.EventUsage) *threadTools {
		t.Helper()
		state := startRound(t, th, r, api.Input{Text: "three"})
		state.ok(api.CodeTurn, call, usage)
		tools, ok := th.state.(*threadTools)
		if !ok {
			t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
		}
		tools.results = result
		return tools
	}

	t.Run("bytes", func(t *testing.T) {
		config := api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 2}
		th, messages, r := stubThread(t, "tools-bytes", stubProvider{api.Model{Name: "stub"}, summary}, config)
		grow(t, r, api.EventUsage{Input: 10})
		tools := toolRound(t, th, r, api.EventUsage{Input: 10})
		if got := r.NeedCompaction(); got != replay.CompactionNone {
			t.Fatalf("the thread reports %v before its tool output is stashed: the pressure has to appear with the results for this test to say anything", got)
		}
		tools.finalize()
		if got := r.NeedCompaction(); got != replay.CompactionBytes {
			t.Fatalf("the thread reports %v once the tool output is stashed, want %v", got, replay.CompactionBytes)
		}
		compact, ok := th.state.(*threadCompact)
		if !ok {
			t.Fatalf("tool output over the byte budget left the thread in %T, want it blocked until the compaction applies", th.state)
		}
		compact.on(<-th.ch)
		if _, ok := th.state.(*threadRound); !ok {
			t.Fatalf("the blocked thread is in %T after the compaction applied, want the round it was holding", th.state)
		}
		record, err := messages.CompactionGet(context.Background(), "tools-bytes", api.NoRoundID)
		if err != nil {
			t.Fatalf("CompactionGet: %v", err)
		}
		if record.Text != "the summary" {
			t.Fatalf("stored compaction %+v: the round went out before the compaction it was waiting for", record)
		}
	})

	t.Run("tokens", func(t *testing.T) {
		config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
		th, _, r := stubThread(t, "tools-tokens", stubProvider{api.Model{Name: "stub", Context: 1000}, summary}, config)
		grow(t, r, api.EventUsage{Input: 900})
		tools := toolRound(t, th, r, api.EventUsage{Input: 900})
		tools.finalize()
		if _, ok := th.state.(*threadRound); !ok {
			t.Fatalf("a token-driven compaction left the thread in %T after the tools, want the next round under way while it streams", th.state)
		}
		<-th.ch
	})
}

func TestBytePressureBlocksTheRoundWhileTokensAlsoOverflow(t *testing.T) {
	summary := []api.Event{
		api.EventStart{ID: "c", Model: "stub"},
		api.EventText{Index: 0, Text: "the summary"},
		api.EventOk{Code: api.CodeTurn},
	}
	config := api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	var sent []api.Request
	th, _, r := stubThread(t, "both-causes", recordingProvider{stubProvider{api.Model{Name: "stub", Context: 1000}, summary}, &sent}, config)
	for _, name := range []string{"one", "two"} {
		if _, _, err := r.RoundInit(api.Input{Text: name}); err != nil {
			t.Fatalf("replay RoundInit(%q): %v", name, err)
		}
		if _, err := r.RoundOk(api.EventUsage{Input: 900}, api.Indexed[api.Content]{0: api.ContentText{Text: strings.Repeat("x", 20_000)}}); err != nil {
			t.Fatalf("replay RoundOk(%q): %v", name, err)
		}
	}
	state := startRound(t, th, r, api.Input{Text: "three"})
	call := api.Indexed[api.Content]{0: api.ContentToolUse{ID: "shot", Name: "probe__run", Input: json.RawMessage(`{}`)}}
	state.ok(api.CodeTurn, call, api.EventUsage{Input: 900})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	tools.results = map[string]api.ContentToolResult{
		"shot": {ID: "shot", Content: api.UserContents{api.ContentImage{Data: make([]byte, 20<<10), Mime: "image/png"}}},
	}
	tools.finalize()
	if got := r.NeedCompaction(); got != replay.CompactionBytes {
		t.Fatalf("a thread over both thresholds reports %v: a token overflow still sends, a byte overflow cannot, so the stricter cause has to win", got)
	}
	compact, ok := th.state.(*threadCompact)
	if !ok {
		t.Fatalf("tool output over the byte budget left the thread in %T, want it blocked until the compaction applies", th.state)
	}
	compact.on(<-th.ch)
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("the blocked thread is in %T after the compaction applied, want the round it was holding", th.state)
	}
	images := 0
	for _, m := range sent[len(sent)-1].Messages {
		for _, block := range m.Content {
			result, ok := block.(api.ContentToolResult)
			if !ok {
				continue
			}
			for _, inner := range result.Content {
				if _, ok := inner.(api.ContentImage); ok {
					images++
				}
			}
		}
	}
	if images != 1 {
		t.Fatalf("the resumed round carries %d images, want the 1 the tool produced: the park the byte cause exists for never engaged and the stash was shed instead", images)
	}
}

func taskRecords(request api.Request) []string {
	var records []string
	for _, m := range request.Messages {
		for _, block := range m.Content {
			text, ok := block.(api.ContentText)
			if ok && (strings.HasPrefix(text.Text, "<task_notify") || strings.HasPrefix(text.Text, "<task_result")) {
				records = append(records, text.Text)
			}
		}
	}
	return records
}

func taskThread(t *testing.T, id api.ThreadID) (*thread, replay.Replay, *[]api.Request, api.RoundID) {
	t.Helper()
	sent := &[]api.Request{}
	reply := []api.Event{
		api.EventStart{ID: "r", Model: "stub"},
		api.EventText{Index: 0, Text: "ok"},
		api.EventOk{Code: api.CodeTurn},
	}
	th, _, r := stubThread(t, id, recordingProvider{stubProvider{api.Model{Name: "stub"}, reply}, sent}, api.ThreadConfig{})
	round, _, err := r.RoundInit(api.Input{Text: "start the task"})
	if err != nil {
		t.Fatalf("replay RoundInit: %v", err)
	}
	if _, err := r.RoundOk(api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "started"}}); err != nil {
		t.Fatalf("replay RoundOk: %v", err)
	}
	return th, r, sent, round.ID
}

func deliver(th *thread, send func()) Command {
	go send()
	return <-th.ch
}

func TestTaskNotifyWhileIdleStartsARound(t *testing.T) {
	th, _, sent, origin := taskThread(t, "notify-idle")
	meta := map[string]any{"island": "chart"}
	task := th.link.taskAdd(origin, "js", "run", "drawing", meta, func(error) {})
	wait := threadWait{thread: th}
	th.state = wait
	wait.on(deliver(th, func() { task.Notify(json.RawMessage(`{"clicked":1}`), nil) }))
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("a notify while idle left the thread in %T, want a round", th.state)
	}
	records := taskRecords((*sent)[len(*sent)-1])
	if len(records) != 1 || !strings.HasPrefix(records[0], "<task_notify") || !strings.Contains(records[0], `"clicked":1`) || !strings.Contains(records[0], "drawing") {
		t.Fatalf("the round's request carries %q, want the one task_notify", records)
	}
	if len(meta) != 1 {
		t.Fatalf("the notify wrote into the task's attrs, which the UI shows as its meta: %v", meta)
	}
	if len(th.link.taskList()) != 1 {
		t.Fatalf("the notify removed the task: it keeps running until Ok or Err")
	}
}

func TestTaskNotifyDuringToolsIsDeliveredWithTheNextRound(t *testing.T) {
	th, r, sent, origin := taskThread(t, "notify-tools")
	task := th.link.taskAdd(origin, "js", "run", "", map[string]any{}, func(error) {})
	state := startRound(t, th, r, api.Input{Text: "call"})
	state.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}, api.EventUsage{})
	tools, ok := th.state.(*threadTools)
	if !ok {
		t.Fatalf("a round that asked for a tool left the thread in %T, want the tool calls", th.state)
	}
	tools.on(deliver(th, func() { task.Notify(json.RawMessage(`"mid-tools"`), nil) }))
	if th.state != any(tools) {
		t.Fatalf("the notify moved the thread out of its tool calls into %T", th.state)
	}
	tools.results = map[string]api.ContentToolResult{"c": {ID: "c", Content: api.UserContents{api.ContentText{Text: "done"}}}}
	tools.finalize()
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("the tools ended into %T, want the next round", th.state)
	}
	records := taskRecords((*sent)[len(*sent)-1])
	if len(records) != 1 || !strings.Contains(records[0], "mid-tools") {
		t.Fatalf("the round after the tools carries %q, want the notify that arrived during them", records)
	}
}

func TestTaskNotifyAfterOkIsDropped(t *testing.T) {
	th, _, sent, origin := taskThread(t, "notify-after-ok")
	task := th.link.taskAdd(origin, "js", "run", "", map[string]any{}, func(error) {})
	th.on(deliver(th, func() { task.Ok(json.RawMessage(`"final"`)) }))
	wait := threadWait{thread: th}
	th.state = wait
	wait.on(deliver(th, func() { task.Notify(json.RawMessage(`"late"`), nil) }))
	if _, ok := th.state.(threadWait); !ok {
		t.Fatalf("a notify from a finished task left the thread in %T, want it still waiting", th.state)
	}
	if len(*sent) != 0 {
		t.Fatalf("a notify from a finished task sent %d requests", len(*sent))
	}
	if _, err := th.switchStateRound(); err != nil {
		t.Fatalf("switchStateRound: %v", err)
	}
	records := taskRecords((*sent)[len(*sent)-1])
	if len(records) != 1 || !strings.HasPrefix(records[0], "<task_result") {
		t.Fatalf("the next round carries %q, want only the task_result", records)
	}
}

func TestTaskNotifyAfterUnlinkIsDropped(t *testing.T) {
	th, _, sent, origin := taskThread(t, "notify-unlink")
	task := th.link.taskAdd(origin, "js", "run", "", map[string]any{}, func(error) {})
	wait := threadWait{thread: th}
	th.state = wait
	wait.on(commandUnlink{link: th.link})
	if th.link != nil {
		t.Fatalf("the unlink left the link in place")
	}
	wait.on(deliver(th, func() { task.Notify(json.RawMessage(`"orphan"`), nil) }))
	if _, ok := th.state.(threadWait); !ok || len(*sent) != 0 {
		t.Fatalf("a notify after unlink left the thread in %T with %d requests sent", th.state, len(*sent))
	}
	th.close = make(chan struct{})
	close(th.close)
	task.Notify(json.RawMessage(`"closed"`), nil)
}

func TestTaskNotifiesThenOkArriveInOrder(t *testing.T) {
	th, _, sent, origin := taskThread(t, "notify-order")
	task := th.link.taskAdd(origin, "js", "run", "", map[string]any{}, func(error) {})
	wait := threadWait{thread: th}
	th.state = wait
	wait.on(deliver(th, func() { task.Notify(json.RawMessage(`"first"`), nil) }))
	round, ok := th.state.(*threadRound)
	if !ok {
		t.Fatalf("the first notify left the thread in %T, want a round", th.state)
	}
	round.on(deliver(th, func() { task.Notify(json.RawMessage(`"second"`), nil) }))
	round.on(deliver(th, func() { task.Ok(json.RawMessage(`"third"`)) }))
	round.ok(api.CodeTurn, api.Indexed[api.Content]{0: api.ContentText{Text: "seen"}}, api.EventUsage{})
	if _, ok := th.state.(*threadRound); !ok {
		t.Fatalf("the round ended into %T with records stashed, want the next round", th.state)
	}
	records := taskRecords((*sent)[len(*sent)-1])
	want := []string{"first", "second", "third"}
	if len(records) != len(want) {
		t.Fatalf("the last request carries %q, want %d task records", records, len(want))
	}
	for i, w := range want {
		if !strings.Contains(records[i], w) {
			t.Fatalf("record %d is %q, want %q", i, records[i], w)
		}
	}
	if !strings.HasPrefix(records[2], "<task_result") {
		t.Fatalf("the last record is %q, want the task_result", records[2])
	}
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
	config := api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5}
	th, _, r := stubThread(t, "stop-compact-pending", stubProvider{api.Model{Name: "stub", Context: 1000}, summary}, config)
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
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	t.Cleanup(func() { store.Remove() })
	var deleted atomic.Int64
	backend := common.NewBackend(ctx, local.NewMessageStorage(), countingWorkspaces{store, &deleted}, store, []common.Option{common.OptionProvider(stubProvider{model: api.Model{Name: "stub"}})})
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
