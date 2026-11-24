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

//go:build browser

package instance_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/thread"
)

func state(t *testing.T, c *stack.Chat) api.InstanceState {
	t.Helper()
	s, ok := c.Instance().State().Read(context.Background())
	if !ok {
		t.Fatal("the instance state is gone")
	}
	return s
}

func listed(t *testing.T, c *stack.Chat) []api.ThreadID {
	t.Helper()
	threads, err := c.Instance().ThreadList(context.Background(), time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]api.ThreadID, len(threads))
	for i, th := range threads {
		ids[i] = th.ID
	}
	return ids
}

func TestInstanceLinksAThread(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("hello"), testkit.Say("pong"))
	c := stack.Browser(t, stack.Host{Provider: p})
	fresh := state(t, c)
	if fresh.Status != api.InstanceEnabled || !fresh.Thread.New || fresh.Thread.ID == "" {
		t.Fatalf("the linked instance is %+v, want enabled on a new thread", fresh)
	}
	c.Send("hi")
	if trace := stack.Trace(c.Wait()); !slices.Contains(trace, "ThreadCreated") {
		t.Fatalf("the first send traced %v, want the thread created", trace)
	}
	created := state(t, c)
	if created.Thread.ID != fresh.Thread.ID || created.Thread.New || created.Thread.Updated.IsZero() {
		t.Fatalf("after the first round the instance is %+v, want thread %s stored and touched", created, fresh.Thread.ID)
	}
	if ids := listed(t, c); !slices.Equal(ids, []api.ThreadID{fresh.Thread.ID}) {
		t.Fatalf("the session lists %v, want only %s", ids, fresh.Thread.ID)
	}
	if err := c.Instance().Emit(context.Background(), "ping", map[string]string{"from": "test"}, "are you there"); err != nil {
		t.Fatal(err)
	}
	c.Wait()
	content := p.Request(t, 1).Messages[2].Content
	event, ok := content[len(content)-1].(api.ContentText)
	if !ok || !strings.HasPrefix(event.Text, "<app_event") {
		t.Fatalf("the emitted round carries %#v, want the app_event", content)
	}
	for _, want := range []string{`name="ping"`, `from="test"`, "are you there"} {
		if !strings.Contains(event.Text, want) {
			t.Fatalf("the app_event %q does not carry %s", event.Text, want)
		}
	}
}

func TestThreadNewSwitchesChat(t *testing.T) {
	p := testkit.NewProvider(api.Model{}, testkit.Say("first"), testkit.Say("second"))
	c := stack.Browser(t, stack.Host{Provider: p})
	c.Send("one")
	c.Wait()
	first := state(t, c).Thread.ID
	if err := c.Instance().ThreadNew(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh := c.Linked()
	second := state(t, c).Thread
	if second.ID == first || !second.New || fresh.ID != second.ID {
		t.Fatalf("after ThreadNew the instance is on %+v and the chat on %s, want both on one new thread other than %s", second, fresh.ID, first)
	}
	c.Send("two")
	if trace := stack.Trace(c.Wait()); !slices.Contains(trace, "ThreadCreated") {
		t.Fatalf("the send on the new chat traced %v, want its thread created", trace)
	}
	if ids := listed(t, c); len(ids) != 2 || !slices.Contains(ids, first) || !slices.Contains(ids, second.ID) {
		t.Fatalf("the session lists %v, want %s and %s", ids, first, second.ID)
	}
	if n := len(p.Request(t, 1).Messages); n != 1 {
		t.Fatalf("the new chat's first request carries %d messages, want only its own", n)
	}
}

func TestRoundsPassTheGateOnABrowserPage(t *testing.T) {
	thread.Backpressure.Store(true)
	t.Cleanup(func() { thread.Backpressure.Store(false) })
	p := testkit.NewProvider(api.Model{}, testkit.Say("one"), testkit.Say("two"), testkit.Say("three"))
	c := stack.Browser(t, stack.Host{Provider: p})
	if _, err := (proto.DebuggerEnable{}).Call(c.Page); err != nil {
		t.Fatal(err)
	}
	if err := (proto.DebuggerPause{}).Call(c.Page); err != nil {
		t.Fatal(err)
	}
	c.Send("first")
	if trace := stack.Trace(c.Wait()); trace[len(trace)-1] != "RoundEnd(ok,final)" {
		t.Fatalf("the first round traced %v, want it ended ok", trace)
	}
	c.Send("second")
	time.Sleep(500 * time.Millisecond)
	if n := len(p.Requests()); n != 1 {
		t.Fatalf("the provider got %d requests while the paused page could not take the first round's time call, want the second round held at the gate", n)
	}
	if err := (proto.DebuggerResume{}).Call(c.Page); err != nil {
		t.Fatal(err)
	}
	if trace := stack.Trace(c.Wait()); trace[len(trace)-1] != "RoundEnd(ok,final)" {
		t.Fatalf("the second round traced %v after the page resumed, want it ended ok", trace)
	}
	c.Send("third")
	if trace := stack.Trace(c.Wait()); trace[len(trace)-1] != "RoundEnd(ok,final)" {
		t.Fatalf("the third round traced %v, want it ended ok", trace)
	}
	if n := len(p.Requests()); n != 3 {
		t.Fatalf("the provider got %d requests, want one per round", n)
	}
}
