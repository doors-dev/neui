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

package neui_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	jsmod "neui.dev/internal/modules/js"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

func asker(t *testing.T, answers chan<- neui.Answer[string], errs chan<- error) common.Option {
	t.Helper()
	m := neui.NewModule("asker", "")
	if _, err := m.Tool("pick", "asks the user", func(ctx context.Context, _ struct{}) (string, error) {
		picked, err := neui.ToolAsk(ctx, struct{}{}, func(_ struct{}, answer neui.Answer[string]) api.ChatAsk {
			answers <- answer
			return api.ChatAsk{}
		})
		if errs != nil {
			errs <- err
		}
		return picked, err
	}); err != nil {
		t.Fatal(err)
	}
	return common.OptionModule(func(context.Context) common.Module { return m })
}

func isAsk(e api.ChatEvent) bool {
	_, ok := e.(api.ChatEventAsk)
	return ok
}

func carriesText(req api.Request, text string) bool {
	for _, m := range req.Messages {
		for _, block := range m.Content {
			switch c := block.(type) {
			case api.ContentText:
				if strings.Contains(c.Text, text) {
					return true
				}
			case api.ContentToolResult:
				for _, r := range c.Content {
					if t, ok := r.(api.ContentText); ok && strings.Contains(t.Text, text) {
						return true
					}
				}
			}
		}
	}
	return false
}

func TestToolAskFromACall(t *testing.T) {
	answers := make(chan neui.Answer[string], 1)
	p := testkit.NewProvider(api.Model{}, testkit.Call("asker__pick", struct{}{}), testkit.Say("done"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{asker(t, answers, nil)}})
	c.Send("ask me")
	c.Until(isAsk)
	if c.State().Ask == nil {
		t.Fatalf("the state of a call waiting on the user carries no ask")
	}
	(<-answers).Submit(context.Background(), "blue")
	done := false
	for _, e := range c.Wait() {
		if _, ok := e.(api.ChatEventAskDone); ok {
			done = true
		}
	}
	if !done {
		t.Fatalf("the answered ask was not taken away")
	}
	if !carriesText(p.Request(t, 1), "blue") {
		t.Fatalf("the tool result does not carry the answer: %+v", testkit.Results(p.Request(t, 1)))
	}
}

func TestToolAskFromAScript(t *testing.T) {
	answers := make(chan neui.Answer[string], 1)
	p := testkit.NewProvider(api.Model{},
		testkit.Call("js__run", map[string]string{"code": "return asker__pick({})", "comment": "ask"}),
		testkit.Say("done"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{asker(t, answers, nil), common.OptionModule(jsmod.Module)}})
	c.Send("ask from a script")
	c.Until(isAsk)
	(<-answers).Submit(context.Background(), "red")
	c.Wait()
	if !carriesText(p.Request(t, 1), "red") {
		t.Fatalf("the script's result does not carry the answer: %+v", testkit.Results(p.Request(t, 1)))
	}
}

func TestToolAskFromBackgroundWork(t *testing.T) {
	answers := make(chan neui.Answer[string], 1)
	release := make(chan struct{})
	m := neui.NewModule("asker", "")
	if _, err := m.Tool("later", "asks the user later", func(ctx context.Context, _ struct{}) (string, error) {
		work, cancel := context.WithCancelCause(neui.ToolThreadContext(ctx))
		task, err := neui.ToolTask(ctx, "asking", nil, cancel)
		if err != nil {
			cancel(nil)
			return "", err
		}
		go func() {
			defer cancel(nil)
			<-release
			picked, err := neui.ToolAsk(work, struct{}{}, func(_ struct{}, answer neui.Answer[string]) api.ChatAsk {
				answers <- answer
				return api.ChatAsk{}
			})
			if err != nil {
				task.Err(err)
				return
			}
			out, _ := json.Marshal(picked)
			task.Ok(out)
		}()
		return "asking", nil
	}); err != nil {
		t.Fatal(err)
	}
	p := testkit.NewProvider(api.Model{}, testkit.Call("asker__later", struct{}{}), testkit.Say("started"), testkit.Say("thanks"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModule(func(context.Context) common.Module { return m })}})
	c.Send("ask me later")
	c.Wait()
	close(release)
	c.Until(isAsk)
	if c.State().Ask == nil {
		t.Fatalf("the state of a waiting thread carries no ask")
	}
	(<-answers).Submit(context.Background(), "green")
	c.Wait()
	if !carriesText(p.Request(t, 2), "green") {
		t.Fatalf("the task result does not carry the answer: %+v", p.Request(t, 2).Messages)
	}
}

func TestRewindTakesTheAskOfACallAway(t *testing.T) {
	answers := make(chan neui.Answer[string], 1)
	errs := make(chan error, 1)
	p := testkit.NewProvider(api.Model{}, testkit.Call("asker__pick", struct{}{}), testkit.Say("again"))
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{asker(t, answers, errs)}})
	c.Send("ask me")
	var round api.RoundID
	for _, e := range c.Until(isAsk) {
		if start, ok := e.(api.ChatEventRoundStart); ok {
			round = start.Round.ID
		}
	}
	<-answers
	c.Rewind(round, "never mind")
	done, final := false, false
	c.Until(func(e api.ChatEvent) bool {
		switch e := e.(type) {
		case api.ChatEventAskDone:
			done = true
		case api.ChatEventRoundEnd:
			final = final || e.Final
		}
		return done && final
	})
	if err := <-errs; !errors.Is(err, api.ErrCanceledByUser) {
		t.Fatalf("the rewound call's ask returned %v, want ErrCanceledByUser", err)
	}
	if c.State().Ask != nil {
		t.Fatalf("the state still carries the ask of the rewound call")
	}
}
