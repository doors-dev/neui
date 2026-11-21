// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thread_test

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/replay"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/thread"
)

func emptyTurn(usage api.EventUsage, blocks ...api.Event) testkit.Turn {
	events := append([]api.Event{api.EventStart{ID: "empty", Model: "fake"}}, blocks...)
	return testkit.Events(append(events, usage, api.EventOk{Code: api.CodeTurn})...)
}

func storedRounds(t *testing.T, c *stack.Chat) []api.Round {
	t.Helper()
	var rounds []api.Round
	for round, err := range c.Backend().Storage().RoundRange(context.Background(), c.State().ID, api.NoRoundID, api.NoRoundID) {
		if err != nil {
			t.Fatalf("RoundRange: %v", err)
		}
		rounds = append(rounds, round)
	}
	return rounds
}

func sameAfterLoad(t *testing.T, c *stack.Chat, p *testkit.Provider) []api.Message {
	t.Helper()
	ctx := context.Background()
	b := c.Backend()
	if _, err := b.Storage().ThreadClone(ctx, c.State().ID, "loaded"); err != nil {
		t.Fatalf("ThreadClone: %v", err)
	}
	loaded := replay.NewReplay(ctx, "loaded", b.Storage(), b.Config(), p.Model())
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, messages, err := loaded.RoundInit(api.Input{Text: "after load"})
	if err != nil {
		t.Fatalf("RoundInit after Load: %v", err)
	}
	p.Push(testkit.Say("after load"))
	c.Send("after load")
	c.Wait()
	sent := p.Request(t, len(p.Requests())-1).Messages
	want, _ := json.Marshal(messages)
	got, _ := json.Marshal(sent)
	if !bytes.Equal(got, want) {
		t.Fatalf("the session sent\n%s\nLoad builds\n%s", got, want)
	}
	return sent
}

func sameMessages(t *testing.T, got, want []api.Message, what string) {
	t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if !bytes.Equal(g, w) {
		t.Fatalf("%s\n got %s\nwant %s", what, g, w)
	}
}

func TestEmptyResponseIsSentAgain(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, emptyTurn(api.EventUsage{Input: 5, Output: 1}), testkit.Say("hello"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hi")
	events := c.Wait()
	expectTrace(t, events, "ThreadCreated", "RoundStart", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 2 {
		t.Fatalf("%d requests, want the empty one and one resend", n)
	}
	sameMessages(t, p.Request(t, 1).Messages, p.Request(t, 0).Messages, "the resend is not the same request")
	if spent, want := events[len(events)-2].(api.ChatEventRoundUsage).Usage, (api.EventUsage{Input: 6, Output: 2}); spent != want {
		t.Fatalf("the round reported %+v spent, want both attempts %+v", spent, want)
	}
	want := api.EventUsage{Input: 1, Output: 1}
	if end := events[len(events)-1].(api.ChatEventRoundEnd); end.Usage != want {
		t.Fatalf("the round ended with usage %+v, want the answer's own %+v", end.Usage, want)
	}
	rounds := storedRounds(t, c)
	if len(rounds) != 1 || rounds[0].Usage != want || rounds[0].Input.Note != "" || len(rounds[0].Response) != 1 || !carries(api.Request{Messages: []api.Message{{Content: rounds[0].Response.Ordered()}}}, "hello") {
		t.Fatalf("storage holds %+v, want one round with the answer, its usage and no note", rounds)
	}
	if usage := c.State().Usage; usage != want {
		t.Fatalf("the context is %+v, want the answer's usage %+v", usage, want)
	}
	sameAfterLoad(t, c, p)
}

func TestSecondEmptyResponseSendsTheNoteAndKeepsIt(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, emptyTurn(api.EventUsage{Input: 1}), emptyTurn(api.EventUsage{Input: 1}), testkit.Say("hello"), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hi")
	events := c.Wait()
	expectTrace(t, events, "ThreadCreated", "RoundStart", "RoundUsage", "RoundStart(truncate)", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if start := events[len(events)-5].(api.ChatEventRoundStart); start.Round.Input.Text != "hi" || start.Round.Input.Len() != 1 {
		t.Fatalf("the noted round reaches the chat with input %+v, want the user's own", start.Round.Input)
	}
	first, noted := p.Request(t, 0), p.Request(t, 2)
	sameMessages(t, p.Request(t, 1).Messages, first.Messages, "the first resend is not the same request")
	rounds := storedRounds(t, c)
	if len(rounds) != 1 || !strings.Contains(rounds[0].Input.Note, "Your previous response had no visible output. Please continue and give the user a visible response.") {
		t.Fatalf("storage holds %+v, want one round that keeps the note", rounds)
	}
	user := first.Messages[len(first.Messages)-1]
	user.Content = append(user.Content, api.ContentText{Text: rounds[0].Input.Note})
	sameMessages(t, noted.Messages, append(first.Messages[:len(first.Messages)-1:len(first.Messages)-1], user), "the second resend is not the request with the note after the input")
	c.Send("next")
	c.Wait()
	later := p.Request(t, 3)
	sameMessages(t, later.Messages[:len(noted.Messages)], noted.Messages, "the next request does not replay the noted round as sent")
	loaded := sameAfterLoad(t, c, p)
	sameMessages(t, loaded[:len(noted.Messages)], noted.Messages, "the request after Load does not replay the noted round as sent")
}

func TestThreeEmptyResponsesEndTheRoundOk(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		emptyTurn(api.EventUsage{Input: 3, Output: 1}),
		emptyTurn(api.EventUsage{Input: 5, Output: 2}, api.EventText{Index: 0, Text: " \n\t"}, api.EventClose{Index: 0}),
		emptyTurn(api.EventUsage{Input: 7, Reasoning: 4}, api.EventThinking{Index: 0, Text: "nothing to say"}, api.EventClose{Index: 0}),
	)
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hi")
	events := c.Wait()
	expectTrace(t, events, "ThreadCreated", "RoundStart", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 3 {
		t.Fatalf("%d requests, want at most two resends", n)
	}
	if spent, want := events[len(events)-2].(api.ChatEventRoundUsage).Usage, (api.EventUsage{Input: 15, Output: 3, Reasoning: 4}); spent != want {
		t.Fatalf("the round reported %+v spent, want the three attempts %+v", spent, want)
	}
	want := api.EventUsage{Input: 7, Reasoning: 4}
	if end := events[len(events)-1].(api.ChatEventRoundEnd); end.Usage != want {
		t.Fatalf("the round ended with usage %+v, want the last attempt's %+v", end.Usage, want)
	}
	rounds := storedRounds(t, c)
	if len(rounds) != 1 || rounds[0].State != api.RoundOk || rounds[0].Usage != want || len(rounds[0].Response) != 1 {
		t.Fatalf("storage holds %+v, want one ok round with the last attempt and its usage", rounds)
	}
	if _, ok := rounds[0].Response[0].(api.ContentReasoning); !ok {
		t.Fatalf("the stored response is %+v, want the last attempt's thinking alone", rounds[0].Response)
	}
	sameAfterLoad(t, c, p)
}

func TestTextEarlierInTheTurnIsNotSentAgain(t *testing.T) {
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		return "ran", nil
	})
	p := testkit.NewProvider(api.Model{},
		testkit.Events(
			api.EventStart{ID: "call", Model: "fake"},
			api.EventText{Index: 0, Text: "Checking."},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "call-probe", Name: "probe__run", Args: "{}"},
			api.EventClose{Index: 1},
			api.EventUsage{Input: 1, Output: 1},
			api.EventOk{Code: api.CodeTurn},
		),
		emptyTurn(api.EventUsage{Input: 1}),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("check")
	expectTrace(t, c.Wait(),
		"ThreadCreated", "RoundStart", "Indexed", "Indexed", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
		"ToolResults", "RoundStart", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 2 {
		t.Fatalf("%d requests, want no resend after the turn already showed text", n)
	}
	sameAfterLoad(t, c, p)
}

func TestTextBeforeACompactionInTheTurnIsNotSentAgain(t *testing.T) {
	compacted := make(chan struct{})
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		if strings.Contains(string(input), "wait") {
			<-compacted
		}
		return "ran", nil
	})
	call := func(id, args string, usage int) testkit.Turn {
		return testkit.Events(
			api.EventStart{ID: id, Model: "stub"},
			api.EventTool{Index: 0, ID: id, Name: "probe__run", Args: args},
			api.EventClose{Index: 0},
			api.EventUsage{Input: usage},
			api.EventOk{Code: api.CodeTurn},
		)
	}
	p := testkit.NewProvider(api.Model{Name: "stub", Context: 1000},
		testkit.Events(
			api.EventStart{ID: "first", Model: "stub"},
			api.EventText{Index: 0, Text: "Looking into it."},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "first", Name: "probe__run", Args: "{}"},
			api.EventClose{Index: 1},
			api.EventUsage{Input: 100},
			api.EventOk{Code: api.CodeTurn},
		),
		call("second", `{"wait":true}`, 850),
		call("third", "{}", 300),
		emptyTurn(api.EventUsage{Input: 300}),
	)
	p.OnCompaction(func(ctx context.Context, req api.Request, out chan<- api.Event) {
		testkit.Say("the summary")(ctx, req, out)
		close(compacted)
	})
	c := stack.Headless(t, stack.Host{
		Provider: p,
		Config:   api.ThreadConfig{ThreadMemoryLimit: -1, CompactionPreserve: 0.25, CompactionTrigger: 0.8},
		Options:  []common.Option{run},
	})
	c.Send("check")
	events := c.Wait()
	var turns []api.Request
	for _, req := range p.Requests() {
		if !testkit.IsCompaction(req) {
			turns = append(turns, req)
		}
	}
	if len(turns) != 4 || !carries(turns[3], "the summary") || carries(turns[3], "Looking into it.") {
		t.Fatalf("%d turn requests, the last %+v; want four, the last after the compaction that summarized the text", len(turns), turns[len(turns)-1].Messages)
	}
	if slices.Contains(stack.Trace(events), "RoundStart(truncate)") {
		t.Fatalf("the turn was sent again although it showed text before the compaction: %v", stack.Trace(events))
	}
	if end := events[len(events)-1].(api.ChatEventRoundEnd); end.State != api.RoundOk {
		t.Fatalf("the turn ended %+v, want ok", end)
	}
	sameAfterLoad(t, c, p)
}

func TestRecoveredRoundKeepsTheContextOfItsLastRequest(t *testing.T) {
	answer := func(text string, usage int) testkit.Turn {
		return testkit.Events(
			api.EventStart{ID: text, Model: "stub"},
			api.EventText{Index: 0, Text: text},
			api.EventClose{Index: 0},
			api.EventUsage{Input: usage},
			api.EventOk{Code: api.CodeTurn},
		)
	}
	p := testkit.NewProvider(api.Model{Name: "stub", Context: 1000}, answer("first", 300), emptyTurn(api.EventUsage{Input: 450}), answer("second", 450))
	c := stack.Headless(t, stack.Host{
		Provider: p,
		Config:   api.ThreadConfig{ThreadMemoryLimit: -1, CompactionAllowed: 0.6, CompactionPreserve: 0.25, CompactionTrigger: 0.8},
	})
	c.Send("one")
	c.Wait()
	c.Send("next")
	events := c.Wait()
	expectTrace(t, events, "RoundStart", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	want := api.EventUsage{Input: 450}
	if end := events[len(events)-1].(api.ChatEventRoundEnd); end.Usage != want || end.CanCompact {
		t.Fatalf("the round ended with %+v, want the context of its last request %+v and no CanCompact", end, want)
	}
	if usage := c.State().Usage; usage != want {
		t.Fatalf("the context is %+v, want %+v", usage, want)
	}
	sameAfterLoad(t, c, p)
}

func TestThinkingOnlyResponseIsEmpty(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		emptyTurn(api.EventUsage{Input: 4, Reasoning: 3}, api.EventThinking{Index: 0, Text: "hidden plan"}, api.EventClose{Index: 0}),
		testkit.Say("answer"),
	)
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hi")
	expectTrace(t, c.Wait(), "ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	sameMessages(t, p.Request(t, 1).Messages, p.Request(t, 0).Messages, "the resend is not the same request")
	rounds := storedRounds(t, c)
	if len(rounds) != 1 || len(rounds[0].Response) != 1 || !carries(api.Request{Messages: []api.Message{{Content: rounds[0].Response.Ordered()}}}, "answer") {
		t.Fatalf("storage holds %+v, want the answer without the discarded thinking", rounds)
	}
	if later := sameAfterLoad(t, c, p); carries(api.Request{Messages: later}, "hidden plan") {
		t.Fatalf("a later request replays the discarded thinking: %+v", later)
	}
}

func TestToolCallIsNotEmpty(t *testing.T) {
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		return "ran", nil
	})
	p := testkit.NewProvider(api.Model{}, testkit.Call("probe__run", map[string]any{}), emptyTurn(api.EventUsage{Input: 1}), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
	c.Send("check")
	expectTrace(t, c.Wait(),
		"ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
		"ToolResults", "RoundStart", "RoundUsage", "RoundStart(truncate)", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	if n := len(p.Requests()); n != 3 {
		t.Fatalf("%d requests, want the call, the empty answer to its result and one resend", n)
	}
	sameMessages(t, p.Request(t, 2).Messages, p.Request(t, 1).Messages, "the resend after the tool result is not the same request")
	sameAfterLoad(t, c, p)
}

func TestSendAndRewindDropTheNote(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("first"), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p})
	input := api.Input{Text: "hi", Note: "planted note"}
	if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Send(op, input) }); err != nil {
		t.Fatalf("send: %v", err)
	}
	events := c.Wait()
	round := events[1].(api.ChatEventRoundStart).Round.ID
	if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Rewind(op, round, input) }); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	c.Wait()
	for i, req := range p.Requests() {
		if carries(req, "planted note") {
			t.Fatalf("request %d carries the note the user sent: %+v", i, req.Messages)
		}
	}
	if rounds := storedRounds(t, c); len(rounds) != 1 || rounds[0].Input.Note != "" {
		t.Fatalf("storage holds %+v, want the rewound round without the note", rounds)
	}
}
