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

package chat

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/chat/composer/controller"
	"neui.dev/nekit/internal/chat/chat/list"
	"neui.dev/nekit/internal/chat/params"
)

func TestClosedKindOf(t *testing.T) {
	cases := []struct {
		err  error
		want closedKind
	}{
		{api.ErrNotFound, deleted},
		{fmt.Errorf("thread: %w", api.ErrNotFound), deleted},
		{api.ErrCompactionFailed, tooLong},
		{fmt.Errorf("thread: %w", api.ErrCompactionFailed), tooLong},
		{errors.New("disk full"), failed},
		{nil, failed},
	}
	for _, c := range cases {
		if got := closedKindOf(c.err); got != c.want {
			t.Errorf("closedKindOf(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

type reloadThread struct {
	api.ChatThread
}

func (reloadThread) Reload(context.Context) {}

func TestTooLongChatShowsItsOwnCard(t *testing.T) {
	c := &chat{thread: reloadThread{}, badge: doors.NewSource(Badge{})}
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return c.body(closedKindOf(api.ErrCompactionFailed))
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	page := html.UnescapeString(string(body))
	for _, want := range []string{"This chat is too long", "It no longer fits the model's context window. Start a new chat to continue.", "nechat-closed-card", ">OK<"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the closed card misses %q: %s", want, page)
		}
	}
	if strings.Contains(page, "Something went wrong") {
		t.Fatalf("the card for a failed compaction shows the generic error: %s", page)
	}
}

type draftThread struct {
	api.ChatThread
}

func (draftThread) Config() api.ThreadConfig { return api.ThreadConfig{ReplayItemShare: 0.1} }

func (draftThread) Model() api.Model { return api.Model{Context: 10_000} }

func (draftThread) Input(context.Context, api.Input) error { return nil }

func TestRoundStartClearsTheComposerOnlyWhenTheRoundTookTheInput(t *testing.T) {
	drafts := make(chan [2]string, 1)
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		composer := controller.New(draftThread{}, api.ChatInput{}, true, api.EventUsage{}, nil, false, false, nil)
		c := &chat{
			thread:   draftThread{},
			composer: composer,
			list:     list.New(params.Params{}, nil, draftThread{}, composer, "t", nil, api.NoRoundID, func(context.Context, error) {}),
			badge:    doors.NewSource(Badge{}),
		}
		composer.UpdateInputText(ctx, "typed during the resend")
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 2, Input: api.Input{Text: "hi"}}, Truncate: 1})
		resent := composer.Input().Get().Input.Text
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 3, Input: api.Input{Text: "typed during the resend"}}, Truncate: api.NoRoundID, InputTaken: true})
		drafts <- [2]string{resent, composer.Input().Get().Input.Text}
		return c.body(failed)
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	res.Body.Close()
	select {
	case d := <-drafts:
		if d[0] != "typed during the resend" {
			t.Fatalf("a resend of the user's round left the composer with %q, want the draft typed meanwhile", d[0])
		}
		if d[1] != "" {
			t.Fatalf("a round that took the sent input left the composer with %q, want it cleared", d[1])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the page never rendered")
	}
}
