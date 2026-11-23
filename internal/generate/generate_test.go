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

package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/response"
	"neui.dev/internal/schema"
	"neui.dev/internal/testkit"
)

type item struct {
	Name string `json:"name" required:"true"`
}

type invoice struct {
	Vendor string  `json:"vendor" required:"true"`
	Total  float64 `json:"total" required:"true"`
	Items  []item  `json:"items"`
}

type node struct {
	Name string `json:"name"`
	Kids []node `json:"kids"`
}

type label string

func (l label) String() string { return "label: " + string(l) }

var brew = invoice{Vendor: "Brew", Total: 47.5, Items: []item{{Name: "dripper"}}}

func calls(calls ...api.EventTool) testkit.Turn {
	events := []api.Event{api.EventStart{ID: "calls", Model: "fake"}}
	for _, c := range calls {
		events = append(events, c, api.EventClose{Index: c.Index})
	}
	return testkit.Events(append(events, api.EventUsage{Input: 1, Output: 1}, api.EventOk{Code: api.CodeTurn})...)
}

func args(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func texts[C api.Content](content []C) []string {
	var out []string
	for _, c := range content {
		if text, ok := any(c).(api.ContentText); ok {
			out = append(out, text.Text)
		}
	}
	return out
}

func roles(messages []api.Message) []api.Role {
	var out []api.Role
	for _, m := range messages {
		out = append(out, m.Role)
	}
	return out
}

func resultText(r api.ContentToolResult) string {
	return strings.Join(texts(r.Content), "\n")
}

func TestObjectDecodesTheResultCall(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call(resultTool, brew))
	got, err := Generate{Provider: p, System: "you extract invoices"}.Object[invoice](context.Background(), "Extract:", "Brew, 47.50")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, brew) {
		t.Fatalf("got %+v, want %+v", got, brew)
	}
	if n := len(p.Requests()); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
	req := p.Request(t, 0)
	if req.System != "you extract invoices" {
		t.Fatalf("system %q", req.System)
	}
	s, err := schema.NewSchema[invoice]()
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != resultTool || !bytes.Equal(req.Tools[0].Schema.(json.RawMessage), s.Object().View().Raw) {
		t.Fatalf("tools %+v, want the result tool with T's schema", req.Tools)
	}
	if req.ToolChoice != "" {
		t.Fatalf("tool choice %q, want the provider's default, as the thread sends", req.ToolChoice)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != api.RoleUser {
		t.Fatalf("messages %+v, want one user message", req.Messages)
	}
	if got, want := texts(req.Messages[0].Content), []string{"Extract:", "Brew, 47.50", resultInstruction}; !reflect.DeepEqual(got, want) {
		t.Fatalf("user content %q, want %q", got, want)
	}
}

func TestObjectWrapsAnyOtherType(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call(resultTool, map[string]any{"value": []string{"blue", "white", "red"}}))
	got, err := Generate{Provider: p}.Object[[]string](context.Background(), "The colours of the French flag")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"blue", "white", "red"}) {
		t.Fatalf("got %q", got)
	}
	if schema := string(p.Request(t, 0).Tools[0].Schema.(json.RawMessage)); !strings.Contains(schema, `"required":["value"]`) {
		t.Fatalf("schema %s does not wrap the list as value", schema)
	}
}

func TestObjectRemindsWhenTheToolIsNotCalled(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("Brew, 47.50"), testkit.Call(resultTool, brew))
	got, err := Generate{Provider: p}.Object[invoice](context.Background(), "Extract")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, brew) {
		t.Fatalf("got %+v", got)
	}
	req := p.Request(t, 1)
	if got, want := roles(req.Messages), []api.Role{api.RoleUser, api.RoleAssistant, api.RoleUser}; !reflect.DeepEqual(got, want) {
		t.Fatalf("roles %v, want %v", got, want)
	}
	if got := texts(req.Messages[1].Content); !reflect.DeepEqual(got, []string{"Brew, 47.50"}) {
		t.Fatalf("assistant turn %q, want the model's text as it came", got)
	}
	if got := req.Messages[2].Content; len(got) != 1 || !reflect.DeepEqual(texts(got), []string{resultReminder}) {
		t.Fatalf("follow-up %#v, want the reminder alone", got)
	}
}

func TestObjectSendsTheValidationErrorBack(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Call(resultTool, map[string]any{"vendor": "Brew"}), testkit.Call(resultTool, brew))
	if _, err := (Generate{Provider: p}).Object[invoice](context.Background(), "Extract"); err != nil {
		t.Fatal(err)
	}
	req := p.Request(t, 1)
	use, ok := req.Messages[1].Content[0].(api.ContentToolUse)
	if !ok || use.ID != "call-"+resultTool {
		t.Fatalf("assistant turn %#v, want the call as it came", req.Messages[1].Content)
	}
	follow := req.Messages[2].Content
	if len(follow) != 1 {
		t.Fatalf("follow-up %#v, want one tool result and no reminder", follow)
	}
	result := follow[0].(api.ContentToolResult)
	if result.ID != use.ID || result.Error != api.ToolErrRuntime || !strings.Contains(resultText(result), "total") {
		t.Fatalf("result %+v, want the schema error naming total", result)
	}
}

func TestObjectAnswersEveryCall(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		calls(
			api.EventTool{Index: 0, ID: "a", Name: "lookup", Args: "{}"},
			api.EventTool{Index: 1, ID: "b", Name: resultTool, Args: `{"vendor":`},
		),
		testkit.Call(resultTool, brew),
	)
	if _, err := (Generate{Provider: p}).Object[invoice](context.Background(), "Extract"); err != nil {
		t.Fatal(err)
	}
	results := testkit.Results(p.Request(t, 1))
	if len(results) != 2 {
		t.Fatalf("%d results, want one per call", len(results))
	}
	if results[0].ID != "a" || results[0].Error != api.ToolErrRuntime || !strings.Contains(resultText(results[0]), api.ErrToolNotFound.Error()) {
		t.Fatalf("unknown tool answered %+v", results[0])
	}
	if results[1].ID != "b" || results[1].Error != api.ToolErrJSON {
		t.Fatalf("broken arguments answered %+v", results[1])
	}
}

func TestObjectTakesTheFirstValidCall(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, calls(
		api.EventTool{Index: 0, ID: "a", Name: resultTool, Args: `{"vendor":"Brew"}`},
		api.EventTool{Index: 1, ID: "b", Name: resultTool, Args: args(t, brew)},
	))
	got, err := Generate{Provider: p}.Object[invoice](context.Background(), "Extract")
	if err != nil || !reflect.DeepEqual(got, brew) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if n := len(p.Requests()); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
}

func TestObjectGivesUpAfterAttempts(t *testing.T) {
	cases := []struct {
		name     string
		attempts int
		turns    []testkit.Turn
		requests int
		cause    error
	}{
		{"default without a call", 0, []testkit.Turn{testkit.Say("a"), testkit.Say("b"), testkit.Say("c"), testkit.Say("d")}, 3, errNoResultCall},
		{"negative is the default", -1, []testkit.Turn{testkit.Say("a"), testkit.Say("b"), testkit.Say("c"), testkit.Say("d")}, 3, errNoResultCall},
		{"one attempt", 1, []testkit.Turn{testkit.Say("a"), testkit.Say("b")}, 1, errNoResultCall},
		{"invalid input", 2, []testkit.Turn{testkit.Call(resultTool, map[string]any{}), testkit.Call(resultTool, map[string]any{"vendor": "Brew"})}, 2, schema.ErrInvalidArguments},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := testkit.NewProvider(api.Model{}, c.turns...)
			_, err := Generate{Provider: p, Attempts: c.attempts}.Object[invoice](context.Background(), "Extract")
			if !errors.Is(err, api.ErrInvalidOutput) || !errors.Is(err, c.cause) {
				t.Fatalf("err %v, want ErrInvalidOutput wrapping %v", err, c.cause)
			}
			if n := len(p.Requests()); n != c.requests {
				t.Fatalf("%d requests, want %d", n, c.requests)
			}
		})
	}
}

func TestObjectWithSimpleSchemas(t *testing.T) {
	model := api.Model{Name: "fake", SimpleSchemas: true}
	p := testkit.NewProvider(model, testkit.Call(resultTool, brew))
	if _, err := (Generate{Provider: p}).Object[invoice](context.Background(), "Extract"); err != nil {
		t.Fatal(err)
	}
	s, err := schema.NewSchema[invoice]()
	if err != nil {
		t.Fatal(err)
	}
	sent := p.Request(t, 0).Tools[0].Schema.(json.RawMessage)
	if !bytes.Equal(sent, s.Object().View().Simple) || bytes.Contains(sent, []byte("$ref")) {
		t.Fatalf("sent %s, want the simple form", sent)
	}
	p = testkit.NewProvider(model, testkit.Call(resultTool, node{}))
	if _, err := (Generate{Provider: p}).Object[node](context.Background(), "Draw a tree"); err == nil {
		t.Fatal("a type that contains itself was accepted")
	}
	if n := len(p.Requests()); n != 0 {
		t.Fatalf("%d requests, want none before the schema is refused", n)
	}
}

func TestProviderErrorsEndTheCall(t *testing.T) {
	cause := errors.New("rate limited")
	failing := testkit.Events(api.EventStart{ID: "x", Model: "fake"}, api.EventErr{Code: api.CodeBusy, Err: cause})
	p := testkit.NewProvider(api.Model{}, failing, testkit.Call(resultTool, brew))
	if _, err := (Generate{Provider: p}).Object[invoice](context.Background(), "Extract"); !errors.Is(err, api.CodeBusy) || !errors.Is(err, cause) {
		t.Fatalf("Object answered %v, want CodeBusy wrapping the cause", err)
	}
	if n := len(p.Requests()); n != 1 {
		t.Fatalf("Object made %d requests, want no retry", n)
	}
	p = testkit.NewProvider(api.Model{}, failing)
	if _, err := (Generate{Provider: p}).Text(context.Background(), "Hi"); !errors.Is(err, api.CodeBusy) || !errors.Is(err, cause) {
		t.Fatalf("Text answered %v, want CodeBusy wrapping the cause", err)
	}
}

func TestACutStreamIsAnError(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Events(api.EventStart{ID: "x", Model: "fake"}, api.EventText{Index: 0, Text: "Hel"}))
	if _, err := (Generate{Provider: p}).Text(context.Background(), "Hi"); !errors.Is(err, api.CodeUnknown) || !errors.Is(err, response.ErrNotEnded) {
		t.Fatalf("answered %v, want CodeUnknown wrapping ErrNotEnded", err)
	}
}

func TestCancellationEndsTheCall(t *testing.T) {
	cause := errors.New("the page closed")
	ctx, cancel := context.WithCancelCause(context.Background())
	started := make(chan struct{})
	p := testkit.NewProvider(api.Model{}, func(ctx context.Context, req api.Request, out chan<- api.Event) {
		out <- api.EventStart{ID: "x", Model: "fake"}
		close(started)
		<-ctx.Done()
	})
	go func() {
		<-started
		cancel(cause)
	}()
	if _, err := (Generate{Provider: p}).Text(ctx, "Hi"); !errors.Is(err, cause) {
		t.Fatalf("answered %v, want the cancel cause", err)
	}
}

func TestUsageCoversEveryRound(t *testing.T) {
	var usage api.EventUsage
	p := testkit.NewProvider(api.Model{}, testkit.Say("no"), testkit.Call(resultTool, brew))
	if _, err := (Generate{Provider: p, Usage: &usage}).Object[invoice](context.Background(), "Extract"); err != nil {
		t.Fatal(err)
	}
	if usage != (api.EventUsage{Input: 2, Output: 2}) {
		t.Fatalf("usage %+v, want both rounds", usage)
	}
	usage = api.EventUsage{Input: 100}
	p = testkit.NewProvider(api.Model{}, testkit.Say("no"))
	if _, err := (Generate{Provider: p, Usage: &usage, Attempts: 1}).Object[invoice](context.Background(), "Extract"); err == nil {
		t.Fatal("a reply without a call was accepted")
	}
	if usage != (api.EventUsage{Input: 1, Output: 1}) {
		t.Fatalf("usage %+v, want the failed call's, overwritten", usage)
	}
	p = testkit.NewProvider(api.Model{}, testkit.Say("hi"))
	if _, err := (Generate{Provider: p}).Text(context.Background(), "Hi"); err != nil {
		t.Fatalf("a nil Usage failed the call: %v", err)
	}
}

func TestReplayKeepsEachTurnAsItCame(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Events(
			api.EventStart{ID: "1", Model: "fake"},
			api.EventThinking{Index: 0, Text: "the user wants an invoice"},
			api.EventClose{Index: 0},
			api.EventText{Index: 1, Text: "first"},
			api.EventClose{Index: 1},
			api.EventOk{Code: api.CodeTurn},
		),
		testkit.Say("second"),
		testkit.Call(resultTool, brew),
	)
	if _, err := (Generate{Provider: p}).Object[invoice](context.Background(), "Extract"); err != nil {
		t.Fatal(err)
	}
	req := p.Request(t, 2)
	if got, want := roles(req.Messages), []api.Role{api.RoleUser, api.RoleAssistant, api.RoleUser, api.RoleAssistant, api.RoleUser}; !reflect.DeepEqual(got, want) {
		t.Fatalf("roles %v, want %v", got, want)
	}
	first := req.Messages[1].Content
	reasoning, ok := first[0].(api.ContentReasoning)
	if len(first) != 2 || !ok || reasoning.Model != "fake" || !reflect.DeepEqual(texts(first), []string{"first"}) {
		t.Fatalf("first turn replayed as %#v, want its reasoning and text", first)
	}
	if got := req.Messages[3].Content; !reflect.DeepEqual(texts(got), []string{"second"}) || len(got) != 1 {
		t.Fatalf("second turn replayed as %#v, want only its own text", got)
	}
}

func TestTextReturnsTheReplyText(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Events(
		api.EventStart{ID: "1", Model: "fake"},
		api.EventThinking{Index: 0, Text: "short answer"},
		api.EventClose{Index: 0},
		api.EventText{Index: 1, Text: "Hello."},
		api.EventClose{Index: 1},
		api.EventOk{Code: api.CodeTurn},
	))
	got, err := Generate{Provider: p, System: "be brief"}.Text(context.Background(), "Greet me")
	if err != nil || got != "Hello." {
		t.Fatalf("got %q, %v", got, err)
	}
	req := p.Request(t, 0)
	if req.System != "be brief" || len(req.Tools) != 0 {
		t.Fatalf("request %+v, want the system prompt and no tools", req)
	}
	if got := texts(req.Messages[0].Content); !reflect.DeepEqual(got, []string{"Greet me"}) {
		t.Fatalf("user content %q, want the parts alone", got)
	}
}

func TestTextJoinsBlocksAndTrims(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Events(
		api.EventStart{ID: "1", Model: "fake"},
		api.EventText{Index: 0, Text: "\n\nAccording to the note, "},
		api.EventClose{Index: 0},
		api.EventText{Index: 1, Text: "the grass is green.\n"},
		api.EventClose{Index: 1},
		api.EventOk{Code: api.CodeTurn},
	))
	got, err := Generate{Provider: p}.Text(context.Background(), "What colour is the grass?")
	if err != nil || got != "According to the note, the grass is green." {
		t.Fatalf("got %q, %v, want the blocks joined as written and trimmed", got, err)
	}
}

func TestTextOfAnEmptyReply(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Events(api.EventStart{ID: "1", Model: "fake"}, api.EventOk{Code: api.CodeTurn}))
	got, err := Generate{Provider: p}.Text(context.Background(), "Hi")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v, want an empty text", got, err)
	}
}

func TestPartsBecomeContent(t *testing.T) {
	image := api.Image{Data: []byte("png"), Mime: api.ImagePNG}
	p := testkit.NewProvider(api.Model{}, testkit.Say("ok"))
	_, err := Generate{Provider: p}.Text(context.Background(),
		"plain",
		label("x"),
		struct {
			Stars int `json:"stars"`
		}{2},
		image,
		api.UserContents{api.ContentText{Text: "as is"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := api.Contents{
		api.ContentText{Text: "plain"},
		api.ContentText{Text: "label: x"},
		api.ContentText{Text: `{"stars":2}`},
		image.Content(),
		api.ContentText{Text: "as is"},
	}
	if got := p.Request(t, 0).Messages[0].Content; !reflect.DeepEqual(got, want) {
		t.Fatalf("content %#v, want %#v", got, want)
	}
	p = testkit.NewProvider(api.Model{}, testkit.Say("ok"))
	if _, err := (Generate{Provider: p}).Text(context.Background(), func() {}); err == nil {
		t.Fatal("a part with no JSON form was accepted")
	}
	if n := len(p.Requests()); n != 0 {
		t.Fatalf("%d requests, want none", n)
	}
}

func TestTheProviderComesFromTheCallOrTheApp(t *testing.T) {
	if _, err := (Generate{}).Text(context.Background(), "Hi"); !errors.Is(err, api.ErrNoProvider) {
		t.Fatalf("Text without a provider answered %v", err)
	}
	if _, err := (Generate{}).Object[invoice](context.Background(), "Hi"); !errors.Is(err, api.ErrNoProvider) {
		t.Fatalf("Object without a provider answered %v", err)
	}
	app := testkit.NewProvider(api.Model{}, testkit.Say("from the app"))
	ctx := context.WithValue(context.Background(), common.ProviderKey{}, api.Provider(app))
	if got, err := (Generate{}).Text(ctx, "Hi"); err != nil || got != "from the app" {
		t.Fatalf("got %q, %v from the app's provider", got, err)
	}
	own := testkit.NewProvider(api.Model{}, testkit.Say("from the call"))
	if got, err := (Generate{Provider: own}).Text(ctx, "Hi"); err != nil || got != "from the call" {
		t.Fatalf("got %q, %v, want the call's provider over the app's", got, err)
	}
}
