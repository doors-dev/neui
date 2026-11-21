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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit"
)

type lockedLogs struct {
	sync.Mutex
	bytes.Buffer
}

func (l *lockedLogs) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.Buffer.Write(p)
}

func (l *lockedLogs) String() string {
	l.Lock()
	defer l.Unlock()
	return l.Buffer.String()
}

type lightNode struct {
	Name     string
	Island   bool
	Children []lightNode
}

func (n lightNode) names(prefix string) []string {
	var out []string
	for _, c := range n.Children {
		name := prefix + c.Name
		if c.Island {
			name += "*"
		}
		out = append(out, name)
		out = append(out, c.names(prefix+c.Name+".")...)
	}
	return out
}

func TestLightModeHidesIslands(t *testing.T) {
	logged := &lockedLogs{}
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logged, nil)))
	t.Cleanup(func() { slog.SetDefault(logger) })
	late := func(name string) gox.Elem {
		return func(cur gox.Cursor) error {
			return cur.Comp(neui.Island{
				Name:        name,
				Desc:        "a late island",
				Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
				Template: func(slot gox.Comp) gox.Elem {
					return func(cur gox.Cursor) error { return cur.Comp(slot) }
				},
				Library: nekit.IslandRegisterLayout,
			})
		}
	}
	for _, light := range []bool{false, true} {
		t.Run(fmt.Sprintf("light=%v", light), func(t *testing.T) {
			shown := doors.NewSource(false)
			marks := make(chan int, 1)
			when := func(name string) gox.Elem {
				return func(cur gox.Cursor) error {
					return cur.Any(shown.Bind(func(on bool) gox.Elem {
						if !on {
							return func(gox.Cursor) error { return nil }
						}
						return late(name)
					}))
				}
			}
			page := func(cur gox.Cursor) error {
				if err := cur.Any(board(marks, nil)); err != nil {
					return err
				}
				if err := (neui.Space{Name: "panel", Desc: "the panel"}).Proxy(cur, func(cur gox.Cursor) error {
					if err := cur.Comp(neui.Tool[struct{}, string]{Name: "open", Desc: "opens the late islands", Handler: func(ctx context.Context, _ struct{}) (string, error) {
						shown.Mutate(ctx, func(bool) bool { return true })
						return fmt.Sprint(neui.LightMode(ctx)), nil
					}}); err != nil {
						return err
					}
					return cur.Any(when("inner"))
				}); err != nil {
					return err
				}
				return cur.Any(when("outer"))
			}
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__tree", map[string]any{}),
				testkit.Call("page__sync", map[string]any{"path": ""}),
				testkit.Call("page__sync", map[string]any{"path": "board"}),
				testkit.Call("page__tool", map[string]any{"path": "board.app", "tool": "mark", "arg": map[string]any{"N": 1}}),
				testkit.Call("page__sync", map[string]any{"path": "panel"}),
				testkit.Call("page__tool", map[string]any{"path": "panel", "tool": "open", "arg": map[string]any{}}),
				testkit.Call("page__tree", map[string]any{"depth": 1}),
				testkit.Say("done"),
			)
			c := stack.Page(t, stack.Host{Provider: p, Page: page, Options: []common.Option{common.OptionLightMode(light)}})
			c.Send("look")
			c.Wait()
			result := func(i int) api.ContentToolResult { return testkit.Results(p.Request(t, i))[0] }
			tree := func(i int) []string {
				var root lightNode
				testkit.Decode(t, result(i), &root)
				return root.names("")
			}
			var root struct{ Children []lightNode }
			testkit.Decode(t, result(2), &root)
			var opened struct {
				Output  string
				Diff    json.RawMessage
				Changes json.RawMessage
			}
			testkit.Decode(t, result(6), &opened)
			if opened.Output != fmt.Sprint(light) {
				t.Fatalf("the page tool's handler saw light mode %s, want %v", opened.Output, light)
			}
			if light {
				if got := tree(1); !slices.Equal(got, []string{"panel"}) {
					t.Fatalf("tree shows %v, want the panel alone", got)
				}
				if got := (lightNode{Children: root.Children}).names(""); !slices.Equal(got, []string{"panel"}) {
					t.Fatalf("the root's children are %v, want the panel alone", got)
				}
				refused(t, result(3), "no such space")
				refused(t, result(4), "no such space")
				if len(marks) != 0 {
					t.Fatal("an island's app function ran in light mode")
				}
				if string(opened.Diff) != "{}" || opened.Changes != nil {
					t.Fatalf("opening answered diff %s and changes %s, want no change at all", opened.Diff, opened.Changes)
				}
				if got := tree(7); !slices.Equal(got, []string{"panel"}) {
					t.Fatalf("tree after opening shows %v, want the panel alone", got)
				}
				for _, space := range []string{"board", "panel.inner", "outer"} {
					if n := strings.Count(logged.String(), "msg=\"page: island hidden in light mode\" space="+space+"\n"); n != 1 {
						t.Fatalf("the warning for %s was logged %d times, want once:\n%s", space, n, logged)
					}
				}
				return
			}
			if got := tree(1); !slices.Equal(got, []string{"board*", "board.app", "board.screen", "panel"}) {
				t.Fatalf("tree shows %v, want the board island and the panel", got)
			}
			if got := (lightNode{Children: root.Children}).names(""); !slices.Equal(got, []string{"board*", "panel"}) {
				t.Fatalf("the root's children are %v, want the board island and the panel", got)
			}
			if r := result(3); r.Error != api.ToolOk {
				t.Fatalf("the board's sync answered %#v", r.Content)
			}
			if r := result(4); r.Error != api.ToolOk || <-marks != 1 {
				t.Fatalf("the board's mark answered %#v", r.Content)
			}
			if string(opened.Diff) != `{"children":{"added":[{"name":"inner","desc":"a late island","island":true}]}}` || string(opened.Changes) != `[{"path":"outer","added":true,"desc":"a late island"}]` {
				t.Fatalf("opening answered diff %s and changes %s, want the inner island in the diff and the outer one in changes", opened.Diff, opened.Changes)
			}
			if got := tree(7); !slices.Equal(got, []string{"board*", "outer*", "panel"}) {
				t.Fatalf("tree after opening shows %v, want both islands at the root", got)
			}
			if strings.Contains(logged.String(), "light mode") {
				t.Fatalf("full mode logged a light-mode warning:\n%s", logged)
			}
		})
	}
}
