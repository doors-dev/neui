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

package replay_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

func TestNameInstructionStaysOnTheFirstMessage(t *testing.T) {
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__tree", map[string]any{"path": ""}),
		testkit.Say("done"),
		testkit.Say("again"),
	)
	page := func(cur gox.Cursor) error { return cur.Text("x") }
	c := stack.Page(t, stack.Host{Provider: p, Page: page})
	c.Send("show me orders")
	c.Wait()
	c.Send("more")
	c.Wait()
	first, _ := json.Marshal(p.Request(t, 0).Messages[0])
	if !strings.Contains(string(first), "Name this chat with chat__name") {
		t.Fatalf("the first request's message %s carries no name instruction", first)
	}
	for i := 1; i < 3; i++ {
		got, _ := json.Marshal(p.Request(t, i).Messages[0])
		if string(got) != string(first) {
			t.Fatalf("request %d replays the first message as %s, want it as sent: %s", i+1, got, first)
		}
	}
	messages := p.Request(t, 2).Messages
	if last, _ := json.Marshal(messages[len(messages)-1]); strings.Contains(string(last), "Name this chat with chat__name") {
		t.Fatalf("a later message carries the name instruction: %s", last)
	}
}
