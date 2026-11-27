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

package island_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

func TestIslandAppFunctionAsksTheUser(t *testing.T) {
	for _, tc := range []struct {
		name  string
		turn  time.Duration
		delay time.Duration
	}{
		{"answered at once", 0, 0},
		{"answered after the turn time", 200 * time.Millisecond, 600 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answers := make(chan neui.Answer[string], 1)
			page := func(cur gox.Cursor) error {
				return cur.Comp(neui.Island{
					Name:        "board",
					Desc:        "a board",
					Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
					Template: func(slot gox.Comp) gox.Elem {
						return func(cur gox.Cursor) error { return cur.Comp(slot) }
					},
					Library: func(l neui.Library) error {
						return l.Tool("pick", "asks the user", func(ctx context.Context, _ struct{}) (string, error) {
							return neui.ToolAsk(ctx, struct{}{}, func(_ struct{}, answer neui.Answer[string]) api.ChatAsk {
								answers <- answer
								return api.ChatAsk{}
							})
						})
					},
				})
			}
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__island_spec", map[string]any{"path": "board"}),
				revised(t, func(revision string) testkit.Turn {
					return start(revision, `ui.notify({picked: app.pick({})})`)
				}),
				testkit.Say("started"),
				testkit.Say("noted"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Island: api.IslandConfig{TurnTime: tc.turn}, Page: page})
			c.Send("open the board")
			c.Until(func(e api.ChatEvent) bool {
				_, ok := e.(api.ChatEventAsk)
				return ok
			})
			answer := <-answers
			time.Sleep(tc.delay)
			answer.Submit(context.Background(), "blue")
			if notify := recorded(t, c, p, `kind="notifications_list"`); !strings.Contains(notify, `[{"picked":"blue"}]`) {
				t.Fatalf("the script's end notified %q, want the user's answer", notify)
			}
		})
	}
}
