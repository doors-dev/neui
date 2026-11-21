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
	"neui.dev/internal/modules/js"
	"neui.dev/internal/replay"
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
	requested := make(chan struct{}, 1)
	p := testkit.NewProvider(api.Model{}, testkit.Then(func(api.Request) testkit.Turn {
		requested <- struct{}{}
		return testkit.Hang(api.EventStart{ID: "hang", Model: "fake"})
	}), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("hang")
	<-requested
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

const notRun = "Not run: this turn's tool results reached their size limit. Call it again next turn."

var budgetModel = api.Model{Name: "stub", Context: 1000}

func sizedProbe(t *testing.T, ran *atomic.Int64) common.Option {
	t.Helper()
	return probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		ran.Add(1)
		var arg struct{ Size int }
		if err := json.Unmarshal(input, &arg); err != nil {
			return nil, err
		}
		return strings.Repeat("x", arg.Size), nil
	})
}

func sizedCalls(sizes ...int) testkit.Turn {
	events := []api.Event{api.EventStart{ID: "sized", Model: "stub"}}
	for i, size := range sizes {
		events = append(events, api.EventTool{Index: i, ID: fmt.Sprintf("c%d", i), Name: "probe__run", Args: fmt.Sprintf(`{"size":%d}`, size)}, api.EventClose{Index: i})
	}
	return testkit.Events(append(events, api.EventOk{Code: api.CodeTurn})...)
}

func TestRoundItemBudgetRunsCallsUntilTheirTextCrossesIt(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(budgetModel, sizedCalls(150, 150, 150, 150), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{sizedProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 3 {
		t.Fatalf("the tool ran %d times, want 3: the third result crosses the 400-byte budget and the call after it must not run", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 4 {
		t.Fatalf("4 calls got %d results: every tool_use needs a matching tool_result", len(results))
	}
	for i, r := range results {
		if r.ID != fmt.Sprintf("c%d", i) {
			t.Fatalf("result %d answers %q", i, r.ID)
		}
		text, _ := r.Content[0].(api.ContentText)
		if i < 3 && (r.Error != api.ToolOk || len(text.Text) != 150) {
			t.Fatalf("call %d ran and came back as %q with %d bytes, want its whole 150", i, r.Error, len(text.Text))
		}
		if i == 3 && (r.Error != api.ToolErrRuntime || text.Text != notRun) {
			t.Fatalf("the call after the budget was spent came back as %q with %#v, want %q", r.Error, r.Content, notRun)
		}
	}
}

func TestRoundItemBudgetOmitsAResultOverIt(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(budgetModel, sizedCalls(1468006, 10), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{sizedProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2: an omitted result leaves the budget to the calls after it", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 2 {
		t.Fatalf("2 calls got %d results", len(results))
	}
	want := "Result omitted: it was 1.4 MB, more than a turn's tool results can hold. Ask for less, or process it in a script."
	if text, ok := results[0].Content[0].(api.ContentText); !ok || len(results[0].Content) != 1 || results[0].Error != api.ToolOk || text.Text != want {
		t.Fatalf("the 1.4 MB result came back as %q with %#v, want ok with %q", results[0].Error, results[0].Content, want)
	}
	if text, ok := results[1].Content[0].(api.ContentText); !ok || results[1].Error != api.ToolOk || len(text.Text) != 10 {
		t.Fatalf("the call after the omitted result came back as %q with %#v, want its 10 bytes", results[1].Error, results[1].Content)
	}
}

func mediaProbe(t *testing.T, ran *atomic.Int64) common.Option {
	t.Helper()
	return probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		ran.Add(1)
		var arg struct{ Size int }
		if err := json.Unmarshal(input, &arg); err != nil {
			return nil, err
		}
		return api.UserContents{
			api.ContentText{Text: "ok"},
			api.ContentImage{Data: make([]byte, arg.Size/2), Mime: "image/png"},
			api.ContentDocument{Data: make([]byte, arg.Size-arg.Size/2), Mime: "text/plain", Name: "a.txt"},
		}, nil
	})
}

var mediaModel = api.Model{Name: "stub", Context: 10_000}

var mediaConfig = api.ThreadConfig{ThreadMemoryLimit: 200_000, ReplayMediaShare: 0.2}

func TestMediaBudgetNotesResultsThatDoNotFit(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(mediaModel, sizedCalls(8000, 8000, 8000, 4000), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{mediaProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 4 {
		t.Fatalf("the tool ran %d times, want 4: a result that does not fit the 20000-byte media budget is noted and the budget stays open", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 4 {
		t.Fatalf("4 calls got %d results", len(results))
	}
	for _, i := range []int{0, 1, 3} {
		r := results[i]
		image, isImage := r.Content[1].(api.ContentImage)
		document, isDocument := r.Content[2].(api.ContentDocument)
		if r.Error != api.ToolOk || !isImage || !isDocument || len(image.Data)+len(document.Data) == 0 {
			t.Fatalf("call %d fit the media budget and came back as %q with %#v, want its text, image and document whole", i, r.Error, r.Content)
		}
	}
	want := api.UserContents{api.ContentText{Text: "ok"}, api.ContentText{Text: replay.OmittedMediaNote(8000)}}
	if results[2].Error != api.ToolOk || !reflect.DeepEqual(results[2].Content, want) {
		t.Fatalf("the third result would take the round over the media budget and came back as %q with %#v, want its text and one note", results[2].Error, results[2].Content)
	}
}

func TestMediaBudgetReachedExactlyStopsTheCalls(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(mediaModel, sizedCalls(10000, 10000, 10), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{mediaProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2: two results reach the 20000-byte media budget exactly and the call after them must not run", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if text, ok := results[2].Content[0].(api.ContentText); len(results) != 3 || !ok || text.Text != notRun {
		t.Fatalf("the call after the media budget was reached came back as %#v, want %q", results[2].Content, notRun)
	}
}

func TestMediaBudgetNotesAResultOverIt(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(mediaModel, sizedCalls(30000, 8000), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{mediaProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2: a result turned into notes leaves the media budget to the calls after it", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 2 {
		t.Fatalf("2 calls got %d results", len(results))
	}
	want := api.UserContents{api.ContentText{Text: "ok"}, api.ContentText{Text: replay.OmittedMediaNote(30000)}}
	if results[0].Error != api.ToolOk || !reflect.DeepEqual(results[0].Content, want) {
		t.Fatalf("the result over the media budget came back as %q with %#v, want its text and one note for its image and document", results[0].Error, results[0].Content)
	}
	if _, ok := results[1].Content[1].(api.ContentImage); !ok || results[1].Error != api.ToolOk {
		t.Fatalf("the call after the noted result came back as %q with %#v, want its media", results[1].Error, results[1].Content)
	}
}

func TestReconnectAfterAMediaCutSendsTheSameRounds(t *testing.T) {
	model := api.Model{Name: "stub", Context: 10_000, ImagesPerRequest: 4}
	images := func(req api.Request) int {
		n := 0
		for _, m := range req.Messages {
			for _, block := range m.Content {
				if r, ok := block.(api.ContentToolResult); ok {
					for _, c := range r.Content {
						if _, ok := c.(api.ContentImage); ok {
							n++
						}
					}
				}
			}
		}
		return n
	}
	run := func(reconnect bool) api.Request {
		var ran atomic.Int64
		var turns []testkit.Turn
		for i := range 6 {
			id := fmt.Sprintf("c%d", i)
			turns = append(turns, testkit.Events(api.EventStart{ID: id, Model: "stub"}, api.EventTool{Index: 0, ID: id, Name: "probe__run", Args: `{"size":2000}`}, api.EventClose{Index: 0}, api.EventOk{Code: api.CodeTurn}))
		}
		p := testkit.NewProvider(model, append(turns, testkit.Say("done"), testkit.Say("next"))...)
		c := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{mediaProbe(t, &ran)}})
		c.Send("call")
		c.Wait()
		if reconnect {
			c.Switch(mediaProbe(t, &ran))
		}
		c.Send("next")
		c.Wait()
		if got := len(p.Requests()); got != 8 {
			t.Fatalf("%d requests, want 8", got)
		}
		return p.Request(t, 7)
	}
	stayed, reconnected := run(false), run(true)
	if n := images(stayed); n == 0 || n >= 6 {
		t.Fatalf("the last request carries %d of the 6 images, want a cut that keeps some", n)
	}
	s, _ := json.Marshal(stayed.Messages)
	r, _ := json.Marshal(reconnected.Messages)
	if string(s) != string(r) {
		t.Fatalf("after a reconnect the rounds go out as\n%s\nwant what the session that stayed sends\n%s", r, s)
	}
}

func TestMediaNoteCountsTowardTheTextBudget(t *testing.T) {
	const media = 30000
	note := replay.OmittedMediaNote(media)
	budget := replay.RoundItemBudget(api.ThreadConfig{ReplayItemShare: 0.1}, mediaModel)
	for _, c := range []struct {
		name   string
		text   int
		images int
		kept   bool
	}{
		{"text and note fit", budget - len(note), 1, true},
		{"many images, one note", budget - len(note), 30, true},
		{"text and note over", budget - len(note) + 1, 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			text := strings.Repeat("x", c.text)
			run := probe(t, func(context.Context, json.RawMessage) (any, error) {
				content := api.UserContents{api.ContentText{Text: text}}
				for range c.images {
					content = append(content, api.ContentImage{Data: make([]byte, media/c.images), Mime: "image/png"})
				}
				return content, nil
			})
			p := testkit.NewProvider(mediaModel, testkit.Call("probe__run", struct{}{}), testkit.Say("done"))
			h := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{run}})
			h.Send("call")
			h.Wait()
			want := api.UserContents{api.ContentText{Text: text}, api.ContentText{Text: note}}
			if !c.kept {
				want = api.UserContents{api.ContentText{Text: replay.OmittedNote(c.text+len(note), false)}}
			}
			if results := testkit.Results(p.Request(t, 1)); len(results) != 1 || !reflect.DeepEqual(results[0].Content, want) {
				t.Fatalf("the result went out as %#v, want %#v", results, want)
			}
		})
	}
}

func TestTextDocumentIsTextForAModelWithoutDocuments(t *testing.T) {
	textOnly := api.Model{Name: "stub", Context: 1000, Inputs: api.Modalities{api.ModalityText}}
	reading := api.Model{Name: "stub", Context: 1000, Inputs: api.Modalities{api.ModalityText, api.ModalityDocument}}
	ok := api.ContentText{Text: "ok"}
	doc := func(size int, mime string) api.ContentDocument {
		return api.ContentDocument{Data: []byte(strings.Repeat("x", size)), Mime: mime, Name: "a.txt"}
	}
	latin := api.ContentDocument{Data: []byte(strings.Repeat("\xff", 500)), Mime: "text/plain", Name: "l.txt"}
	pdf := api.ContentDocument{Data: []byte("%PDF-1.4\n" + strings.Repeat(" ", 500)), Mime: string(api.DocumentPDF), Name: "a.pdf"}
	image := api.ContentImage{Data: make([]byte, 30000), Mime: "image/png"}
	noDocument := func(d api.ContentDocument) api.ContentText {
		return api.ContentText{Text: fmt.Sprintf(`<nodocument name=%q mime=%q bytes=%d reason="this model reads no documents of type %s"/>`, d.Name, d.Mime, len(d.Data), d.Mime)}
	}
	for _, c := range []struct {
		name   string
		model  api.Model
		result api.UserContents
		stored api.UserContents
		sent   api.UserContents
	}{
		{"fits the budget", textOnly, api.UserContents{ok, doc(398, "text/plain")}, api.UserContents{ok, doc(398, "text/plain")}, api.UserContents{ok, api.ContentText{Text: strings.Repeat("x", 398)}}},
		{"over the budget", textOnly, api.UserContents{ok, doc(399, "text/plain")}, api.UserContents{api.ContentText{Text: replay.OmittedNote(401, false)}}, api.UserContents{api.ContentText{Text: replay.OmittedNote(401, false)}}},
		{"mime as written", textOnly, api.UserContents{ok, doc(399, " Text/Markdown")}, api.UserContents{api.ContentText{Text: replay.OmittedNote(401, false)}}, api.UserContents{api.ContentText{Text: replay.OmittedNote(401, false)}}},
		{"reading model", reading, api.UserContents{ok, doc(399, "text/plain")}, api.UserContents{ok, doc(399, "text/plain")}, api.UserContents{ok, doc(399, "text/plain")}},
		{"not UTF-8", textOnly, api.UserContents{ok, latin}, api.UserContents{ok, latin}, api.UserContents{ok, noDocument(latin)}},
		{"not a text type", textOnly, api.UserContents{ok, pdf}, api.UserContents{ok, pdf}, api.UserContents{ok, noDocument(pdf)}},
		{"media noted around it", textOnly, api.UserContents{image, doc(100, "text/plain")}, api.UserContents{api.ContentText{Text: replay.OmittedMediaNote(30000)}, doc(100, "text/plain")}, api.UserContents{api.ContentText{Text: replay.OmittedMediaNote(30000)}, api.ContentText{Text: strings.Repeat("x", 100)}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			run := probe(t, func(context.Context, json.RawMessage) (any, error) { return c.result, nil })
			s := local.NewMessageStorage()
			p := testkit.NewProvider(c.model, testkit.Call("probe__run", struct{}{}), testkit.Say("done"))
			h := stack.Headless(t, stack.Host{Provider: p, Config: mediaConfig, Options: []common.Option{run, common.OptionMessageStorage(s)}})
			h.Send("call")
			h.Wait()
			for round, err := range s.RoundRange(context.Background(), h.State().ID, api.NoRoundID, api.NoRoundID) {
				if err != nil {
					t.Fatalf("RoundRange: %v", err)
				}
				if round.ID == 0 && (len(round.Results) != 1 || !reflect.DeepEqual(round.Results["call-probe__run"].Content, c.stored)) {
					t.Fatalf("the result is stored as %#v, want %#v", round.Results, c.stored)
				}
			}
			if results := testkit.Results(p.Request(t, 1)); len(results) != 1 || !reflect.DeepEqual(results[0].Content, c.sent) {
				t.Fatalf("the result went out as %#v, want %#v", results, c.sent)
			}
		})
	}
}

func TestMediaBudgetIsOffWithoutAThreadLimit(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(mediaModel, sizedCalls(30000, 30000), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: api.ThreadConfig{ThreadMemoryLimit: -1}, Options: []common.Option{mediaProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	for _, r := range testkit.Results(p.Request(t, 1)) {
		if _, ok := r.Content[1].(api.ContentImage); !ok || r.Error != api.ToolOk {
			t.Fatalf("call %s came back as %q with %#v, want its media: a negative ThreadMemoryLimit sets no media budget", r.ID, r.Error, r.Content)
		}
	}
	if got := ran.Load(); got != 2 {
		t.Fatalf("the tool ran %d times, want 2", got)
	}
}

func TestRoundItemBudgetNotesReplayAsSent(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(budgetModel, sizedCalls(1000, 150, 150, 150), testkit.Say("done"), testkit.Say("next"), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{sizedProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	c.Send("next")
	next := c.Wait()[0].(api.ChatEventRoundStart).Round.ID
	c.Rewind(next, "again")
	c.Wait()
	results := func(req api.Request) []api.ContentToolResult {
		var results []api.ContentToolResult
		for _, m := range req.Messages {
			for _, block := range m.Content {
				if r, ok := block.(api.ContentToolResult); ok {
					results = append(results, r)
				}
			}
		}
		return results
	}
	sent := results(p.Request(t, 1))
	if len(sent) != 4 {
		t.Fatalf("4 calls got %d results", len(sent))
	}
	if text, ok := sent[0].Content[0].(api.ContentText); !ok || !strings.HasPrefix(text.Text, "Result omitted") {
		t.Fatalf("the 1000-byte result went out as %#v, want the omission note", sent[0].Content)
	}
	if text, ok := sent[3].Content[0].(api.ContentText); !ok || text.Text != notRun {
		t.Fatalf("the call after the budget was spent went out as %#v, want %q", sent[3].Content, notRun)
	}
	for i := 2; i <= 3; i++ {
		if got := results(p.Request(t, i)); !reflect.DeepEqual(got, sent) {
			t.Fatalf("request %d replays the tool results as\n%+v\nwant them as first sent\n%+v", i, got, sent)
		}
	}
}

func TestToolCallLimitAnswersBeforeTheRoundItemBudget(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(budgetModel, sizedCalls(300, 300, 300), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Config: api.ThreadConfig{ToolCallLimit: 2}, Options: []common.Option{sizedProbe(t, &ran)}})
	c.Send("call")
	c.Wait()
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 3 {
		t.Fatalf("3 calls got %d results", len(results))
	}
	if text, ok := results[2].Content[0].(api.ContentText); !ok || !strings.Contains(text.Text, "at most 2 tool calls") {
		t.Fatalf("the call past both the limit and the budget came back as %#v, want the limit named", results[2].Content)
	}
}

func TestRoundItemBudgetSkipsAnAskWithoutShowingIt(t *testing.T) {
	var ran atomic.Int64
	asker := tools.NewModule("asker", "")
	if _, err := asker.Ask("ask", "asks the user", func(struct{}, common.Answer[string]) api.ChatAsk { return api.ChatAsk{} }); err != nil {
		t.Fatal(err)
	}
	p := testkit.NewProvider(budgetModel,
		testkit.Events(
			api.EventStart{ID: "ask", Model: "stub"},
			api.EventTool{Index: 0, ID: "spend", Name: "probe__run", Args: `{"size":400}`},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "ask", Name: "asker__ask", Args: "{}"},
			api.EventClose{Index: 1},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Say("done"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{
		sizedProbe(t, &ran),
		common.OptionModule(func(context.Context) common.Module { return asker }),
	}})
	c.Send("call")
	for _, e := range c.Wait() {
		if _, ok := e.(api.ChatEventAsk); ok {
			t.Fatalf("the ask after the budget was spent was shown to the user")
		}
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 2 {
		t.Fatalf("2 calls got %d results", len(results))
	}
	if text, ok := results[1].Content[0].(api.ContentText); !ok || results[1].Error != api.ToolErrRuntime || text.Text != notRun {
		t.Fatalf("the ask after the budget was spent came back as %q with %#v, want %q", results[1].Error, results[1].Content, notRun)
	}
}

func TestRoundItemBudgetDoesNotCountScriptCalls(t *testing.T) {
	var ran atomic.Int64
	p := testkit.NewProvider(budgetModel,
		testkit.Events(
			api.EventStart{ID: "script", Model: "stub"},
			api.EventTool{Index: 0, ID: "script", Name: "js__run", Args: `{"code":"probe__run({size: 300}); probe__run({size: 300}); probe__run({size: 300}); return 'done'","comment":"three probes"}`},
			api.EventClose{Index: 0},
			api.EventTool{Index: 1, ID: "direct", Name: "probe__run", Args: `{"size":300}`},
			api.EventClose{Index: 1},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Say("done"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{sizedProbe(t, &ran), common.OptionModule(js.Module)}})
	c.Send("call")
	c.Wait()
	if got := ran.Load(); got != 4 {
		t.Fatalf("the tool ran %d times, want 3 from the script and 1 direct", got)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 2 || results[0].Error != api.ToolOk {
		t.Fatalf("the round answered with %+v, want the script's result and the direct call's", results)
	}
	if text, ok := results[1].Content[0].(api.ContentText); !ok || results[1].Error != api.ToolOk || len(text.Text) != 300 {
		t.Fatalf("the direct call after the script came back as %q with %#v, want its 300 bytes: a script's tool calls do not count toward the budget", results[1].Error, results[1].Content)
	}
}

func TestRecordsOverTheRoundItemBudgetStartAnotherTurn(t *testing.T) {
	release := make(chan struct{})
	p := testkit.NewProvider(budgetModel, held(release, testkit.Say("busy")), testkit.Say("first"), testkit.Say("second"))
	c := stack.Headless(t, stack.Host{Provider: p})
	c.Send("go")
	for _, data := range []string{strings.Repeat("a", 300), strings.Repeat("b", 200)} {
		if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Record(op, "app_event", nil, data) }); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	close(release)
	expectTrace(t, c.Wait(),
		"ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
		"RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
		"RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok,final)")
	for i, body := range []string{strings.Repeat("a", 300), strings.Repeat("b", 200)} {
		messages := p.Request(t, i+1).Messages
		content := messages[len(messages)-1].Content
		if text, ok := content[0].(api.ContentText); len(content) != 1 || !ok || text.Text != "<app_event>"+body+"</app_event>" {
			t.Fatalf("request %d carries %#v, want the %d-byte event alone: the two together are over the 400-byte budget", i+1, content, len(body))
		}
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

func TestRejectedRoundRefusesTheWaitingResults(t *testing.T) {
	refused := api.ContentToolResult{}.Refused().Content[0].(api.ContentText).Text
	for _, code := range []api.ErrCode{api.CodeInvalid, api.CodeTooLarge} {
		t.Run(string(code), func(t *testing.T) {
			run := probe(t, func(context.Context, json.RawMessage) (any, error) {
				return api.UserContents{api.ContentText{Text: "shot"}, api.ContentImage{Data: make([]byte, 64), Mime: "image/png"}}, nil
			})
			rejected := testkit.Events(api.EventErr{Code: code, Err: errors.New("stub: rejected")})
			p := testkit.NewProvider(api.Model{Name: "stub"}, testkit.Call("probe__run", struct{}{}), rejected, testkit.Say("moved on"))
			c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{run}})
			c.Send("go")
			expectTrace(t, c.Wait(),
				"ThreadCreated", "RoundStart", "Indexed", "Indexed", "RoundUsage", "RoundEnd(ok)",
				"ToolResults", "RoundStart", "RoundEnd(error,"+string(code)+",final)")
			if n := len(p.Requests()); n != 2 {
				t.Fatalf("the rejected round led to %d requests, want 2: it ends the turn", n)
			}
			if results := testkit.Results(p.Request(t, 1)); len(results) != 1 || len(results[0].Content) != 2 {
				t.Fatalf("the rejected request carried %+v, want the result as the tool returned it", results)
			}
			if err := c.Do(thread.Link.Retry); !errors.Is(err, api.ErrWrongState) {
				t.Fatalf("retry after the rejected round answered %v, want ErrWrongState: it would send the rejected request again", err)
			}
			c.Send("next")
			c.Wait()
			results := testkit.Results(p.Request(t, 2))
			if len(results) != 1 || len(results[0].Content) != 1 {
				t.Fatalf("the next request answers with %+v, want the one refused result", results)
			}
			if text, ok := results[0].Content[0].(api.ContentText); !ok || text.Text != refused {
				t.Fatalf("the next request carries %#v, want the refused note in place of the rejected result", results[0].Content)
			}
		})
	}
}

func TestFailedRoundKeepsItsCause(t *testing.T) {
	for _, c := range []struct {
		code  api.ErrCode
		cause error
		want  string
	}{
		{api.CodeUnknown, errors.New("stub: http 520: origin error"), "unknown: stub: http 520: origin error"},
		{api.CodeInvalid, errors.New("stub: rejected"), "invalid: stub: rejected"},
		{api.CodeNetwork, nil, "network"},
	} {
		t.Run(string(c.code), func(t *testing.T) {
			s := local.NewMessageStorage()
			p := testkit.NewProvider(api.Model{Name: "stub"}, testkit.Events(api.EventErr{Code: c.code, Err: c.cause}), testkit.Say("again"))
			chat := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionMessageStorage(s)}})
			chat.Send("go")
			expectTrace(t, chat.Wait(), "ThreadCreated", "RoundStart", "RoundEnd(error,"+string(c.code)+",final)")
			stored, err := s.RoundAfter(context.Background(), chat.State().ID, api.NoRoundID, 10)
			if err != nil || len(stored) != 1 {
				t.Fatalf("RoundAfter: %d rounds, %v", len(stored), err)
			}
			if stored[0].State != api.RoundError || stored[0].Error != c.want {
				t.Fatalf("the failed round is stored as %s %q, want an error %q", stored[0].State, stored[0].Error, c.want)
			}
			err = chat.Do(thread.Link.Retry)
			if (err == nil) != (c.code != api.CodeInvalid) {
				t.Fatalf("retry after a %s round answered %v", c.code, err)
			}
			if err == nil {
				chat.Wait()
			}
		})
	}
}

func TestOversizedInputIsRefusedWhereItIsSubmitted(t *testing.T) {
	const limit = 400
	ctx := context.Background()
	oversized := api.Input{Text: strings.Repeat("x", limit+1)}
	open := func(t *testing.T, turns ...testkit.Turn) (*stack.Chat, api.MessageStorage) {
		t.Helper()
		s := local.NewMessageStorage()
		c := stack.Headless(t, stack.Host{Provider: testkit.NewProvider(budgetModel, turns...), Options: []common.Option{common.OptionMessageStorage(s)}})
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

	t.Run("draft", func(t *testing.T) {
		c, s := open(t)
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
		c, s := open(t)
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
		c, s := open(t, testkit.Say("ok"), testkit.Say("ok"))
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

	t.Run("rewind during a round stops it", func(t *testing.T) {
		c, s := open(t, testkit.Say("ok"), testkit.Hang(api.EventStart{ID: "hang", Model: "fake"}))
		c.Send("one")
		c.Wait()
		c.Send("two")
		stored := rounds(t, c, s)
		first, streaming := stored[0].ID, stored[1].ID
		err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Rewind(op, first, oversized) })
		if !errors.Is(err, api.ErrInputTooLarge) {
			t.Fatalf("a rewind carrying a %d-byte input under a %d limit answered %v, want ErrInputTooLarge", len(oversized.Text), limit, err)
		}
		if round := c.State().Round; round != nil {
			t.Fatalf("a refused rewind left round %+v running, want the round %v stopped", round, streaming)
		}
		if stored := rounds(t, c, s); len(stored) != 2 || stored[1].State != api.RoundAbort {
			t.Fatalf("the refused rewind left %d rounds, want the 2 with round %v aborted", len(stored), streaming)
		}
		expectTrace(t, c.Wait(), "RoundStart", "RoundEnd(abort,final)")
	})

	t.Run("at the limit", func(t *testing.T) {
		c, _ := open(t)
		fits := api.Input{Text: strings.Repeat("x", limit)}
		if err := input(c, fits); err != nil {
			t.Fatalf("an input of exactly the %d-byte limit answered %v, want it accepted", limit, err)
		}
		if draft := c.State().Input; draft.Text != fits.Text {
			t.Fatalf("an accepted draft of %d bytes was stored as %d bytes", len(fits.Text), len(draft.Text))
		}
	})

	t.Run("attachments count toward the limit", func(t *testing.T) {
		c, _ := open(t)
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
	half := strings.Repeat("x", 25_000)
	compacted := make(chan struct{})
	run := probe(t, func(context.Context, json.RawMessage) (any, error) {
		return api.UserContents{api.ContentImage{Data: make([]byte, 14<<10), Mime: "image/png"}}, nil
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
		Config:   api.ThreadConfig{ThreadMemoryLimit: 60_000, ReplayItemShare: 0.25, CompactionPreserve: 0.25, CompactionTrigger: 0.5},
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
		t.Fatalf("the resumed round carries %d images, want the 1 the tool produced: the park the byte cause exists for never engaged", images)
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
