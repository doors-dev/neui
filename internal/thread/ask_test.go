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

package thread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/replay"
	"neui.dev/internal/tools"
)

type asked struct {
	answer string
	err    error
}

func askFrom(l *link, ctx context.Context) (common.Answer[string], <-chan asked) {
	answers := make(chan common.Answer[string], 1)
	result := make(chan asked, 1)
	go func() {
		answer, err := tools.Asking(func(_ struct{}, answer common.Answer[string]) api.ChatAsk {
			answers <- answer
			return api.ChatAsk{}
		})(context.WithValue(ctx, common.KeyController, common.Controller(controller{link: l})), struct{}{})
		result <- asked{answer, err}
	}()
	return <-answers, result
}

func kinds(events []api.ChatEvent) string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = strings.TrimPrefix(fmt.Sprintf("%T", e), "api.ChatEvent")
	}
	return strings.Join(names, " ")
}

func TestAskShowsInEveryState(t *testing.T) {
	call := api.Indexed[api.Content]{0: api.ContentToolUse{ID: "c", Name: "probe__run", Input: json.RawMessage(`{}`)}}
	for _, tc := range []struct {
		name  string
		enter func(t *testing.T, th *thread, r replay.Replay)
	}{
		{"waiting", func(t *testing.T, th *thread, r replay.Replay) { th.switchStateWait() }},
		{"round", func(t *testing.T, th *thread, r replay.Replay) { startRound(t, th, r, api.Input{Text: "go"}) }},
		{"tools", func(t *testing.T, th *thread, r replay.Replay) {
			startRound(t, th, r, api.Input{Text: "call"}).ok(api.CodeTurn, call, api.EventUsage{})
		}},
		{"compacting", func(t *testing.T, th *thread, r replay.Replay) {
			th.switchStateCompact(api.Round{}, "", api.EventUsage{}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th, r, _, _ := taskThread(t, api.ThreadID("ask-"+tc.name))
			events := captureEvents(th)
			tc.enter(t, th, r)
			events()
			on := th.state.(interface{ on(any) }).on
			answer, result := askFrom(th.link, context.Background())
			on(<-th.ch)
			if got := kinds(events()); got != "Ask" {
				t.Fatalf("an ask sent %q, want it shown", got)
			}
			if th.getState().Ask == nil {
				t.Fatalf("the state of a thread asking the user carries no ask")
			}
			answer.Submit(context.Background(), "yes")
			on(<-th.ch)
			if got := kinds(events()); got != "AskDone" {
				t.Fatalf("an answer sent %q, want the ask gone", got)
			}
			if a := <-result; a.err != nil || a.answer != "yes" {
				t.Fatalf("the ask returned %q, %v, want the answer", a.answer, a.err)
			}
			if th.getState().Ask != nil {
				t.Fatalf("the state still carries the answered ask")
			}
		})
	}
}

func TestAsksQueueInOrder(t *testing.T) {
	th, _, _, _ := taskThread(t, "ask-queue")
	events := captureEvents(th)
	th.switchStateWait()
	on := th.state.(interface{ on(any) }).on
	_, first := askFrom(th.link, context.Background())
	on(<-th.ch)
	leaving, leave := context.WithCancelCause(context.Background())
	_, second := askFrom(th.link, leaving)
	on(<-th.ch)
	_, third := askFrom(th.link, context.Background())
	on(<-th.ch)
	shown := events()
	if got := kinds(shown); got != "Ask" {
		t.Fatalf("three asks sent %q, want only the first shown", got)
	}
	gone := errors.New("caller gone")
	leave(gone)
	on(<-th.ch)
	if got := kinds(events()); got != "" {
		t.Fatalf("a queued ask whose caller left sent %q, want nothing", got)
	}
	if a := <-second; !errors.Is(a.err, gone) {
		t.Fatalf("the queued ask whose caller left returned %v, want its cause", a.err)
	}
	shown[0].(api.ChatEventAsk).Ask.Dismiss(context.Background())
	on(<-th.ch)
	if got := kinds(events()); got != "AskDone Ask" {
		t.Fatalf("dismissing the shown ask sent %q, want it gone and the next shown", got)
	}
	if a := <-first; !errors.Is(a.err, api.ErrCanceledByUser) {
		t.Fatalf("the dismissed ask returned %v, want ErrCanceledByUser", a.err)
	}
	select {
	case a := <-third:
		t.Fatalf("dismissing the first ask ended the next one too: %v", a.err)
	default:
	}
	th.getState().Ask.Dismiss(context.Background())
	on(<-th.ch)
	if a := <-third; !errors.Is(a.err, api.ErrCanceledByUser) {
		t.Fatalf("the ask the state carried returned %v, want the third ask dismissed", a.err)
	}
	if got := kinds(events()); got != "AskDone" {
		t.Fatalf("dismissing the last ask sent %q, want it gone", got)
	}
	if th.getState().Ask != nil {
		t.Fatalf("the state still carries an ask after the queue emptied")
	}
}

func TestAskFromAnotherLinkIsRefused(t *testing.T) {
	th, _, _, _ := taskThread(t, "ask-unlinked")
	events := captureEvents(th)
	th.switchStateWait()
	on := th.state.(interface{ on(any) }).on
	other := &link{thread: th, linkTasks: &linkTasks{thread: th}}
	_, refused := askFrom(other, context.Background())
	on(<-th.ch)
	if a := <-refused; !errors.Is(a.err, api.ErrUnlinked) {
		t.Fatalf("an ask from another link returned %v, want ErrUnlinked", a.err)
	}
	leaving, leave := context.WithCancelCause(context.Background())
	_, open := askFrom(th.link, leaving)
	on(<-th.ch)
	events()
	old := th.link
	th.disconnect()
	if len(th.asks) != 0 {
		t.Fatalf("the unlinked thread kept %d asks", len(th.asks))
	}
	_, late := askFrom(old, context.Background())
	on(<-th.ch)
	if a := <-late; !errors.Is(a.err, api.ErrUnlinked) {
		t.Fatalf("an ask after the unlink returned %v, want ErrUnlinked", a.err)
	}
	leave(api.ErrUnlinked)
	on(<-th.ch)
	if a := <-open; !errors.Is(a.err, api.ErrUnlinked) {
		t.Fatalf("the ask open at the unlink returned %v, want its cause", a.err)
	}
}
