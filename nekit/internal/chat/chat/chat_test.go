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

func TestRoundStartClearsOnlyWhatTheRoundTook(t *testing.T) {
	type look struct {
		name  string
		state controller.InputState
	}
	looks := make(chan []look, 1)
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		composer := controller.New(draftThread{}, api.ChatInput{}, true, api.EventUsage{}, nil, false, false, nil)
		c := &chat{
			thread:   draftThread{},
			composer: composer,
			list:     list.New(params.Params{}, nil, draftThread{}, composer, "t", nil, api.NoRoundID, func(context.Context, error) {}),
			badge:    doors.NewSource(Badge{}),
		}
		var got []look
		composer.UpdateInputText(ctx, "draft")
		composer.InitRewind(ctx, 1, api.Input{Text: "edit"})
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 2, Input: api.Input{Text: "hi"}}, Truncate: 1})
		got = append(got, look{"retry", composer.Input().Get()})
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 3, Input: api.Input{Text: "edit"}}, Truncate: 2, InputTaken: true})
		got = append(got, look{"rewind", composer.Input().Get()})
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 4, Input: api.Input{Text: "draft"}}, Truncate: api.NoRoundID, InputTaken: true})
		got = append(got, look{"send", composer.Input().Get()})
		composer.UpdateInputText(ctx, "queued")
		c.Update(ctx, api.ChatEventInputPending{Pending: true})
		composer.InitRewind(ctx, 3, api.Input{Text: "edit again"})
		c.Update(ctx, api.ChatEventRoundStart{Round: api.Round{ID: 5, Input: api.Input{Text: "queued"}}, Truncate: api.NoRoundID, InputTaken: true})
		got = append(got, look{"queue taken while editing", composer.Input().Get()})
		composer.CancelRewind(ctx)
		got = append(got, look{"edit closed", composer.Input().Get()})
		looks <- got
		return c.body(failed)
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	res.Body.Close()
	want := []struct {
		rewind bool
		text   string
	}{
		{true, "edit"},
		{false, "draft"},
		{false, ""},
		{true, "edit again"},
		{false, ""},
	}
	select {
	case got := <-looks:
		for i, w := range want {
			if s := got[i].state; s.Rewind != w.rewind || s.Input.Text != w.text || s.Pending {
				t.Errorf("after the %s the composer shows %+v, want edit mode %v with %q and nothing pending", got[i].name, s, w.rewind, w.text)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the page never rendered")
	}
}
