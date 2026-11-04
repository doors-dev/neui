package thread_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/thread"
	"neui.dev/internal/tools"
)

func probe(t *testing.T, run func(ctx context.Context, input json.RawMessage) (any, error)) common.Option {
	t.Helper()
	m := tools.NewModule("probe", "")
	if _, err := m.AddRaw("run", "runs the probe", schema.ObjectRawSchema, schema.AnyRawSchema, run); err != nil {
		t.Fatal(err)
	}
	return common.OptionModule(func(context.Context) common.Module { return m })
}

func held(release <-chan struct{}, turn testkit.Turn) testkit.Turn {
	return testkit.Then(func(api.Request) testkit.Turn {
		<-release
		return turn
	})
}

func carries(req api.Request, text string) bool {
	for _, m := range req.Messages {
		for _, block := range m.Content {
			if c, ok := block.(api.ContentText); ok && strings.Contains(c.Text, text) {
				return true
			}
		}
	}
	return false
}

func expectTrace(t *testing.T, events []api.ChatEvent, want ...string) {
	t.Helper()
	if got := stack.Trace(events); !slices.Equal(got, want) {
		t.Fatalf("trace\n got %v\nwant %v", got, want)
	}
}

func TestSendOneRound(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("hello"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hi")
	expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 1 {
		t.Fatalf("one round made %d requests", n)
	}
	if !carries(p.Request(t, 0), "hi") {
		t.Fatalf("the request does not carry the input: %+v", p.Request(t, 0).Messages)
	}
}

func TestToolLoopReplaysResult(t *testing.T) {
	var got json.RawMessage
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		got = input
		return "got x", nil
	})
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", map[string]string{"x": "1"}), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("call it")
	expectTrace(t, c.Wait(),
		"ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
		"ToolResults", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if string(got) != `{"x":"1"}` {
		t.Fatalf("the tool ran on %s", got)
	}
	second := p.Request(t, 1)
	results := testkit.Results(second)
	if len(results) != 1 || results[0].ID != "call-probe__run" || results[0].Error != api.ToolOk {
		t.Fatalf("the second request answers with %+v, want the one result", results)
	}
	if text, ok := results[0].Content[0].(api.ContentText); !ok || text.Text != "got x" {
		t.Fatalf("the replayed result is %#v, want got x", results[0].Content)
	}
	use, ok := second.Messages[len(second.Messages)-2].Content[0].(api.ContentToolUse)
	if !ok || use.ID != "call-probe__run" || use.Name != "probe__run" {
		t.Fatalf("the message before the result is %+v, want the tool_use it answers", second.Messages[len(second.Messages)-2])
	}
}

func TestStopHangingTurn(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Hang(api.EventStart{ID: "hang", Model: "fake"}), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hang")
	c.Stop()
	expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "RoundEnd(abort,final)")
	c.Send("next")
	expectTrace(t, c.Wait(), "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 2 {
		t.Fatalf("the send after the stop made %d requests in all, want 2", n)
	}
}

func TestCutStreamEndsInError(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Events(api.EventStart{ID: "cut", Model: "fake"}, api.EventText{Index: 0, Text: "half"}),
		testkit.Say("whole"),
	)
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("go")
	expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "Indexed", "RoundEnd(error,unknown,final)")
	c.Retry()
	expectTrace(t, c.Wait(), "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if first, second := p.Request(t, 0), p.Request(t, 1); !reflect.DeepEqual(first.Messages, second.Messages) {
		t.Fatalf("the retry sent\n%+v\nwant the cut request again\n%+v", second.Messages, first.Messages)
	}
}

func TestRewindReplacesRound(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("first"), testkit.Say("second"), testkit.Say("redone"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("one")
	events := c.Wait()
	first := events[1].(api.ChatEventRoundStart).Round.ID
	c.Send("two")
	c.Wait()
	c.Rewind(first, "again")
	events = c.Wait()
	expectTrace(t, events, "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if truncate := events[0].(api.ChatEventRoundStart).Truncate; truncate != first {
		t.Fatalf("the rewind truncates %v, want the first round %v", truncate, first)
	}
	last := p.Request(t, 2)
	if !carries(last, "again") || carries(last, "one") || carries(last, "two") || carries(last, "second") {
		t.Fatalf("the rewound request carries %+v, want only the new input", last.Messages)
	}
}

func TestToolCallLimitAnswersEveryCallPastIt(t *testing.T) {
	const limit = 3
	var calls atomic.Int64
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		calls.Add(1)
		return "ran " + string(input), nil
	})
	events := []api.Event{api.EventStart{ID: "many", Model: "fake"}}
	for i := range limit + 2 {
		events = append(events, api.EventTool{Index: i, ID: fmt.Sprintf("c%d", i), Name: "probe__run", Args: "{}"}, api.EventClose{Index: i})
	}
	events = append(events, api.EventOk{Code: api.CodeTurn})
	p := testkit.NewProvider(api.Model{}, testkit.Events(events...), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: api.ThreadConfig{ToolCallLimit: limit}, Options: []common.Option{run}})
	c.Send("call")
	c.Wait()
	if got := calls.Load(); got != limit {
		t.Fatalf("the tool ran %d times, want %d: calls past the limit must not run", got, limit)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != limit+2 {
		t.Fatalf("%d calls got %d results: every tool_use needs a matching tool_result", limit+2, len(results))
	}
	for i, r := range results {
		if r.ID != fmt.Sprintf("c%d", i) {
			t.Fatalf("result %d answers %q", i, r.ID)
		}
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
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		calls.Add(1)
		return "ran " + string(input), nil
	})
	p := testkit.NewProvider(api.Model{},
		testkit.Events(
			api.EventStart{ID: "args", Model: "fake"},
			api.EventTool{Index: 0, ID: "empty-object", Name: "probe__run", Args: "{}"},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "no-arguments", Name: "probe__run"},
			api.EventClose{Index: 1},
			api.EventTool{Index: 2, ID: "discarded", Name: "probe__run", Args: `{"broken`},
			api.EventClose{Index: 2},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Say("done"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("call")
	c.Wait()
	if got := calls.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2: a call sent no arguments runs on {}, a discarded accumulation does not run", got)
	}
	results := map[string]api.ContentToolResult{}
	for _, r := range testkit.Results(p.Request(t, 1)) {
		results[r.ID] = r
	}
	if len(results) != 3 {
		t.Fatalf("3 calls got %d results: every tool_use needs a matching tool_result", len(results))
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

func TestToolRoundEndWipesScratch(t *testing.T) {
	var fs api.FS
	var scratch string
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		fs = common.GetController(ctx).FS()
		var err error
		if scratch, err = fs.TempDir(); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(scratch, "unzipped.xml"), []byte("scratch"), 0o644); err != nil {
			return nil, err
		}
		keep, err := fs.Create("keep.txt")
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(keep, "keep"); err != nil {
			return nil, err
		}
		return "written", keep.Close()
	})
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("unpack")
	c.Wait()
	if r := testkit.Results(p.Request(t, 1)); len(r) != 1 || r[0].Error != api.ToolOk {
		t.Fatalf("the tool failed: %+v", r)
	}
	if _, err := os.Stat(scratch); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch survived the round: %v", err)
	}
	keep, err := fs.ReadAll(context.Background(), "keep.txt")
	if err != nil {
		t.Fatalf("workspace file after the round: %v", err)
	}
	defer keep.Close()
	if b, err := io.ReadAll(keep); err != nil || string(b) != "keep" {
		t.Fatalf("workspace file after the round: %q, %v", b, err)
	}
}

func TestProviderRefusalOfMeasuredBytesIsTerminal(t *testing.T) {
	const sent, declared = 33_000_000, 32_000_000
	model := api.Model{Name: "stub", RequestBytes: declared}
	refused := testkit.Events(api.EventErr{Code: api.CodeTooLarge, Err: fmt.Errorf("stub: the request is %d bytes, over the %d the model %s takes", sent, declared, model.Name)})
	measured := testkit.Events(api.EventErr{Code: api.CodeTooLarge, Err: fmt.Errorf("%w: sent %d bytes, measured against the %d declared for %s: stub: 413", api.ErrMeasuredTooLarge, sent, declared, model.Name)})

	t.Run("local", func(t *testing.T) {
		p := testkit.NewProvider(model, refused, refused)
		c := stack.Headless(t, stack.Host{Provider: p})
		c.Send("go")
		expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "RoundStart(truncate)", "RoundEnd(error,too_large,final)")
		if n := len(p.Requests()); n != 2 {
			t.Fatalf("a refusal this process made itself led to %d requests, want the round sent again once under the tightened budget, then ended once Tighten is spent", n)
		}
	})

	t.Run("measured", func(t *testing.T) {
		p := testkit.NewProvider(model, measured, refused, testkit.Say("fits"))
		c := stack.Headless(t, stack.Host{Provider: p})
		c.Send("go")
		expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "RoundEnd(error,too_large,final)")
		if n := len(p.Requests()); n != 1 {
			t.Fatalf("a 413 answering a request measured as fitting made %d requests, want the round ended: nothing it sends instead is smaller enough", n)
		}
		c.Retry()
		expectTrace(t, c.Wait(), "RoundStart(truncate)", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
		if n := len(p.Requests()); n != 3 {
			t.Fatalf("the retry made %d requests in all, want 3: Tighten was spent on a refusal no tightening fixes", n)
		}
	})
}

func TestOversizedInputIsRefusedWhereItIsSubmitted(t *testing.T) {
	const limit = 4096
	ctx := context.Background()
	oversized := api.Input{Text: strings.Repeat("x", limit+1)}
	open := func(t *testing.T, config api.ThreadConfig, turns ...testkit.Turn) (*stack.Chat, api.MessageStorage) {
		t.Helper()
		s := local.NewMessageStorage()
		c := stack.Headless(t, stack.Host{Provider: testkit.NewProvider(api.Model{}, turns...), Config: config, Options: []common.Option{common.OptionMessageStorage(s)}})
		return c, s
	}
	rounds := func(t *testing.T, c *stack.Chat, s api.MessageStorage) []api.Round {
		t.Helper()
		stored, err := s.RoundAfter(ctx, c.State().ID, api.NoRoundID, 100)
		if err != nil && !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("RoundAfter: %v", err)
		}
		return stored
	}
	input := func(c *stack.Chat, in api.Input) error {
		return c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Input(op, in) })
	}
	config := api.ThreadConfig{InputBytes: limit}

	t.Run("draft", func(t *testing.T) {
		c, s := open(t, config)
		if err := input(c, oversized); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte draft under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if draft := c.State().Input; draft.Text != "" || len(draft.Attachments) != 0 {
			t.Fatalf("the refused draft was stored as %+v: a refused input has to leave no trace", draft)
		}
		if n := len(rounds(t, c, s)); n != 0 {
			t.Fatalf("the refused draft left %d rounds in storage", n)
		}
	})

	t.Run("send", func(t *testing.T) {
		c, s := open(t, config)
		err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Send(op, oversized) })
		if !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte send under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		state := c.State()
		if state.Input.Text != "" || state.Input.Pending {
			t.Fatalf("the refused send left input %+v: a refused input has to leave no trace", state.Input)
		}
		if state.Round != nil {
			t.Fatalf("a refused send started round %+v: no round may start on an input that cannot be sent", state.Round)
		}
		if n := len(rounds(t, c, s)); n != 0 {
			t.Fatalf("the refused send created %d rounds in storage", n)
		}
		if err := c.Do(thread.Link.Retry); !errors.Is(err, api.ErrWrongState) {
			t.Fatalf("retry after the refused send answered %v: the refused send left a retriable round behind", err)
		}
	})

	t.Run("rewind", func(t *testing.T) {
		c, s := open(t, config, testkit.Say("ok"), testkit.Say("ok"))
		c.Send("one")
		c.Wait()
		c.Send("two")
		c.Wait()
		first := rounds(t, c, s)[0].ID
		err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Rewind(op, first, oversized) })
		if !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a rewind carrying a %d-byte input under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if n := len(rounds(t, c, s)); n != 2 {
			t.Fatalf("the refused rewind left %d rounds, want the 2 it was asked to cut back through: a refused input may not cut anything", n)
		}
		if round := c.State().Round; round != nil {
			t.Fatalf("a refused rewind started round %+v, want the thread still waiting", round)
		}
	})

	t.Run("rewind during a round", func(t *testing.T) {
		c, s := open(t, config, testkit.Say("ok"), testkit.Hang(api.EventStart{ID: "hang", Model: "fake"}))
		c.Send("one")
		c.Wait()
		c.Send("two")
		stored := rounds(t, c, s)
		first, streaming := stored[0].ID, stored[1].ID
		err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Rewind(op, first, oversized) })
		if !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a rewind carrying a %d-byte input under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if round := c.State().Round; round == nil || round.ID != streaming {
			t.Fatalf("a refused rewind left round %+v, want the round %v it was already running", round, streaming)
		}
		if n := len(rounds(t, c, s)); n != 2 {
			t.Fatalf("the refused rewind left %d rounds, want the 2 it was asked to cut back through", n)
		}
		c.Stop()
		expectTrace(t, c.Wait(), "RoundStart", "RoundEnd(abort,final)")
	})

	t.Run("at the limit", func(t *testing.T) {
		c, _ := open(t, config)
		fits := api.Input{Text: strings.Repeat("x", limit)}
		if err := input(c, fits); err != nil {
			t.Fatalf("an input of exactly the %d-byte limit answered %v, want it accepted", limit, err)
		}
		if draft := c.State().Input; draft.Text != fits.Text {
			t.Fatalf("an accepted draft of %d bytes was stored as %d bytes", len(fits.Text), len(draft.Text))
		}
	})

	t.Run("attachments count toward the limit", func(t *testing.T) {
		c, _ := open(t, config)
		text := strings.Repeat("x", limit-10)
		if err := input(c, api.Input{Text: text}); err != nil {
			t.Fatalf("a %d-byte text under a %d limit answered %v: the text alone has to fit for this test to say anything", len(text), limit, err)
		}
		if err := input(c, api.Input{Text: text, Attachments: []string{"notes.txt"}}); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte text plus the file link its attachment renders as answered %v, want ErrInputTooLarge", len(text), err)
		}
		if draft := c.State().Input; len(draft.Attachments) != 0 {
			t.Fatalf("the refused draft was stored as %+v", draft)
		}
	})

	t.Run("non-positive takes the default", func(t *testing.T) {
		c, _ := open(t, api.ThreadConfig{InputBytes: -1})
		if err := input(c, api.Input{Text: strings.Repeat("x", 256<<10+1)}); !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a %d-byte draft under a negative InputBytes answered %v, want ErrInputTooLarge: the input ceiling has no off switch", 256<<10+1, err)
		}
		if draft := c.State().Input; draft.Text != "" {
			t.Fatalf("the refused draft was stored under a negative InputBytes: %d bytes", len(draft.Text))
		}
		if err := input(c, api.Input{Text: strings.Repeat("x", 256<<10)}); err != nil {
			t.Fatalf("a draft of exactly 256 KiB under a negative InputBytes answered %v, want it accepted", err)
		}
	})
}

func TestByteDrivenCompactionBlocksTheRoundItPrecedes(t *testing.T) {
	heavy := strings.Repeat("x", 40_000)
	half := strings.Repeat("x", 20_000)

	t.Run("bytes", func(t *testing.T) {
		release := make(chan struct{})
		s := local.NewMessageStorage()
		p := testkit.NewProvider(api.Model{Name: "stub"}, testkit.Say(half), testkit.Say(half), held(release, testkit.Say(heavy)), testkit.Say("after"))
		p.OnCompaction(testkit.Say("the summary"))
		c := stack.Headless(t, stack.Host{
			Provider: p,
			Config:   api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 2},
			Options:  []common.Option{common.OptionMessageStorage(s)},
		})
		c.Send("one")
		c.Wait()
		c.Send("two")
		c.Wait()
		c.Send("three")
		c.Send("next")
		close(release)
		c.Wait()
		c.Wait()
		if !testkit.IsCompaction(p.Request(t, 3)) {
			t.Fatalf("request 3 is not the compaction: the round over the byte budget never asked for one")
		}
		if next := p.Request(t, 4); !carries(next, "the summary") || !carries(next, "next") {
			t.Fatalf("the round after the byte overflow went out as %+v: it went out before the compaction it was waiting for", next.Messages)
		}
		record, err := s.CompactionGet(context.Background(), c.State().ID, api.NoRoundID)
		if err != nil {
			t.Fatalf("CompactionGet: %v", err)
		}
		if record.Text != "the summary" {
			t.Fatalf("stored compaction %+v", record)
		}
	})

	t.Run("tokens", func(t *testing.T) {
		release, compacted := make(chan struct{}), make(chan struct{})
		defer close(compacted)
		over := testkit.Events(
			api.EventStart{ID: "over", Model: "stub"},
			api.EventText{Index: 0, Text: heavy},
			api.EventClose{Index: 0},
			api.EventUsage{Input: 900},
			api.EventOk{Code: api.CodeTurn},
		)
		p := testkit.NewProvider(api.Model{Name: "stub", Context: 1000}, testkit.Say(half), testkit.Say(half), held(release, over), testkit.Say("after"))
		p.OnCompaction(held(compacted, testkit.Say("the summary")))
		c := stack.Headless(t, stack.Host{
			Provider: p,
			Config:   api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5},
		})
		c.Send("one")
		c.Wait()
		c.Send("two")
		c.Wait()
		c.Send("three")
		c.Send("next")
		close(release)
		c.Wait()
		c.Wait()
		if !testkit.IsCompaction(p.Request(t, 3)) {
			t.Fatalf("request 3 is not the compaction: the round over the token trigger never asked for one")
		}
		if next := p.Request(t, 4); carries(next, "the summary") || !carries(next, "next") {
			t.Fatalf("the round after the token overflow went out as %+v, want it under way while the compaction streams", next.Messages)
		}
	})
}

func TestByteDrivenCompactionBlocksTheRoundAfterTheTools(t *testing.T) {
	half := strings.Repeat("x", 20_000)
	run := func(t *testing.T) common.Option {
		return probe(t, func(context.Context, json.RawMessage) (any, error) {
			return strings.Repeat("x", 40_000), nil
		})
	}

	t.Run("bytes", func(t *testing.T) {
		p := testkit.NewProvider(api.Model{Name: "stub"}, testkit.Say(half), testkit.Say(half), testkit.Call("probe__run", struct{}{}), testkit.Say("after"))
		p.OnCompaction(testkit.Say("the summary"))
		c := stack.Headless(t, stack.Host{
			Provider: p,
			Config:   api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 2},
			Options:  []common.Option{run(t)},
		})
		c.Send("one")
		c.Wait()
		c.Send("two")
		c.Wait()
		c.Send("three")
		c.Wait()
		if !testkit.IsCompaction(p.Request(t, 3)) {
			t.Fatalf("request 3 is not the compaction: the tool output over the byte budget never asked for one")
		}
		if next := p.Request(t, 4); !carries(next, "the summary") {
			t.Fatalf("the round after the tools went out as %+v: it went out before the compaction it was waiting for", next.Messages)
		}
	})

	t.Run("tokens", func(t *testing.T) {
		compacted := make(chan struct{})
		defer close(compacted)
		call := testkit.Events(
			api.EventStart{ID: "call", Model: "stub"},
			api.EventTool{Index: 0, ID: "big", Name: "probe__run", Args: "{}"},
			api.EventClose{Index: 0},
			api.EventUsage{Input: 900},
			api.EventOk{Code: api.CodeTurn},
		)
		p := testkit.NewProvider(api.Model{Name: "stub", Context: 1000}, testkit.Say(half), testkit.Say(half), call, testkit.Say("after"))
		p.OnCompaction(held(compacted, testkit.Say("the summary")))
		c := stack.Headless(t, stack.Host{
			Provider: p,
			Config:   api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.5},
			Options:  []common.Option{run(t)},
		})
		c.Send("one")
		c.Wait()
		c.Send("two")
		c.Wait()
		c.Send("three")
		c.Wait()
		if !testkit.IsCompaction(p.Request(t, 3)) {
			t.Fatalf("request 3 is not the compaction: the round over the token trigger never asked for one")
		}
		if next := p.Request(t, 4); carries(next, "the summary") {
			t.Fatalf("the round after the tools went out as %+v, want it under way while the compaction streams", next.Messages)
		}
	})
}

func TestBytePressureBlocksTheRoundWhileTokensAlsoOverflow(t *testing.T) {
	half := strings.Repeat("x", 20_000)
	compacted := make(chan struct{})
	run := probe(t, func(context.Context, json.RawMessage) (any, error) {
		return api.UserContents{api.ContentImage{Data: make([]byte, 20<<10), Mime: "image/png"}}, nil
	})
	call := testkit.Events(
		api.EventStart{ID: "call", Model: "stub"},
		api.EventTool{Index: 0, ID: "shot", Name: "probe__run", Args: "{}"},
		api.EventClose{Index: 0},
		api.EventUsage{Input: 900},
		api.EventOk{Code: api.CodeTurn},
	)
	p := testkit.NewProvider(api.Model{Name: "stub", Context: 1000}, testkit.Say(half), testkit.Say(half), call, testkit.Say("after"))
	p.OnCompaction(held(compacted, testkit.Say("the summary")))
	c := stack.Headless(t, stack.Host{
		Provider: p,
		Config:   api.ThreadConfig{ThreadMemoryLimit: 60_000, CompactionPreserve: 0.25, CompactionTrigger: 0.5},
		Options:  []common.Option{run},
	})
	c.Send("one")
	c.Wait()
	c.Send("two")
	c.Wait()
	c.Send("three")
	c.Until(func(e api.ChatEvent) bool {
		_, ok := e.(api.ChatEventToolResults)
		return ok
	})
	if round := c.State().Round; round == nil || len(round.Results) != 1 {
		t.Fatalf("after the tools the thread shows round %+v, want the tool round held until the compaction applies", round)
	}
	if n := len(p.Requests()); n != 4 {
		t.Fatalf("%d requests went out before the compaction applied, want the 3 rounds and the compaction: a byte overflow cannot send", n)
	}
	close(compacted)
	c.Wait()
	images := 0
	for _, m := range p.Request(t, 4).Messages {
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

func taskChat(t *testing.T, p *testkit.Provider, comment string, meta map[string]any, during func(task common.Task)) (*stack.Chat, common.Task) {
	t.Helper()
	var task common.Task
	run := probe(t, func(ctx context.Context, _ json.RawMessage) (any, error) {
		var err error
		if task, err = common.GetController(ctx).Task(comment, meta, func(error) {}); err != nil {
			return nil, err
		}
		if during != nil {
			during(task)
		}
		return "started", nil
	})
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("start the task")
	c.Wait()
	return c, task
}

func TestTaskNotifyWhileIdleStartsARound(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("started"), testkit.Say("seen"))
	meta := map[string]any{"island": "chart"}
	c, task := taskChat(t, p, "drawing", meta, nil)
	task.Notify(json.RawMessage(`{"clicked":1}`), nil)
	expectTrace(t, c.Wait(), "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	records := taskRecords(p.Request(t, 2))
	if len(records) != 1 || !strings.HasPrefix(records[0], "<task_notify") || !strings.Contains(records[0], `"clicked":1`) || !strings.Contains(records[0], "drawing") {
		t.Fatalf("the round's request carries %q, want the one task_notify", records)
	}
	if len(meta) != 1 {
		t.Fatalf("the notify wrote into the task's attrs, which the UI shows as its meta: %v", meta)
	}
	if tasks := c.State().Tasks; len(tasks) != 1 {
		t.Fatalf("the notify left tasks %+v: it keeps running until Ok or Err", tasks)
	}
}

func TestTaskNotifyDuringToolsIsDeliveredWithTheNextRound(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("started"))
	taskChat(t, p, "", nil, func(task common.Task) {
		task.Notify(json.RawMessage(`"mid-tools"`), nil)
	})
	if n := len(p.Requests()); n != 2 {
		t.Fatalf("the notify during the tools made %d requests, want the tool round and the one after it", n)
	}
	records := taskRecords(p.Request(t, 1))
	if len(records) != 1 || !strings.Contains(records[0], "mid-tools") {
		t.Fatalf("the round after the tools carries %q, want the notify that arrived during them", records)
	}
}

func TestTaskNotifyAfterOkIsDropped(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("started"), testkit.Say("finished"))
	c, task := taskChat(t, p, "", nil, nil)
	task.Ok(json.RawMessage(`"final"`))
	c.Wait()
	task.Notify(json.RawMessage(`"late"`), nil)
	if round := c.State().Round; round != nil {
		t.Fatalf("a notify from a finished task started round %+v", round)
	}
	if n := len(p.Requests()); n != 3 {
		t.Fatalf("a notify from a finished task made %d requests in all, want 3", n)
	}
	records := taskRecords(p.Request(t, 2))
	if len(records) != 1 || !strings.HasPrefix(records[0], "<task_result") {
		t.Fatalf("the round after Ok carries %q, want only the task_result", records)
	}
}

func TestTaskNotifyAfterUnlinkIsDropped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("started"))
		c, task := taskChat(t, p, "", nil, nil)
		c.Close()
		task.Notify(json.RawMessage(`"orphan"`), nil)
		synctest.Wait()
		if n := len(p.Requests()); n != 2 {
			t.Fatalf("a notify after unlink made %d requests in all, want 2", n)
		}
		time.Sleep(2 * time.Minute)
		task.Notify(json.RawMessage(`"closed"`), nil)
	})
}

func TestTaskNotifiesThenOkArriveInOrder(t *testing.T) {
	release := make(chan struct{})
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", struct{}{}), testkit.Say("started"), held(release, testkit.Say("seen")), testkit.Say("all"))
	c, task := taskChat(t, p, "", nil, nil)
	task.Notify(json.RawMessage(`"first"`), nil)
	task.Notify(json.RawMessage(`"second"`), nil)
	task.Ok(json.RawMessage(`"third"`))
	close(release)
	expectTrace(t, c.Wait(), "RoundStart", "Tasks", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	records := taskRecords(p.Request(t, 3))
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
