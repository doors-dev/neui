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

package stat_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit"
)

func open(t *testing.T, script string) (*stack.IslandApp, *atomic.Int64) {
	t.Helper()
	ticks := new(atomic.Int64)
	a := stack.Island(t, devkit.App{
		Island: neui.Island{
			Name:        "board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := nekit.IslandRegisterStat(l); err != nil {
					return err
				}
				if err := nekit.IslandRegisterLayout(l); err != nil {
					return err
				}
				return l.Tool("tick", "counts a query run", func(context.Context, struct{}) (int64, error) {
					return ticks.Add(1), nil
				})
			},
		},
		Script: script + `; ui.draw(1); ui.notify("drawn")`,
	})
	notified(a)
	return a, ticks
}

func notified(a *stack.IslandApp) {
	a.Until(func(r devkit.Report) bool { return r.Meta["kind"] == "notifications_list" })
}

func then(t *testing.T, a *stack.IslandApp, code string) {
	t.Helper()
	if _, _, err := a.Continue(code+`; ui.draw(1); ui.notify("done")`, false); err != nil {
		t.Fatal(err)
	}
	notified(a)
}

func shows(t *testing.T, a *stack.IslandApp, selector string, read string, want string) {
	t.Helper()
	stack.Eventually(t, 10*time.Second, func() (bool, string) {
		got, err := a.Page.Eval(`(s, read) => Array.from(document.querySelectorAll(s), (e) => read === "text" ? e.textContent.replace(/\s+/g, " ").trim() : e.getAttribute(read) ?? "").join(" | ")`, selector, read)
		if err != nil {
			return false, err.Error()
		}
		return got.Value.Str() == want, got.Value.Str()
	}, "the "+read+" of "+selector+" is "+want)
}

func widgetError(t *testing.T, a *stack.IslandApp) []string {
	t.Helper()
	var errs []string
	for _, r := range a.Until(func(r devkit.Report) bool { return r.Meta["kind"] == "widget_error" }) {
		if r.Meta["kind"] != "widget_error" {
			continue
		}
		var data struct{ Widget, Error string }
		if err := json.Unmarshal(r.Data, &data); err != nil {
			t.Fatal(err)
		}
		errs = append(errs, data.Widget+": "+data.Error)
	}
	return errs
}

func TestStatStaticTileShowsValueAndChange(t *testing.T) {
	a, _ := open(t, `ui.root.set(ui.grid({},
		ui.stat({label: "Revenue", value: 12400, reference: 10000, format: "currency", unit: "USD", referenceLabel: "vs last month"}),
		ui.stat({label: "Errors", value: 12, reference: 10, good: "down"}),
		ui.stat({label: "Costs", value: 80, reference: 100, good: "down", format: "currency", unit: "EUR"}),
		ui.stat({label: "Churn", value: 0.04, reference: 0.05, format: "percent"}),
		ui.stat({label: "Flat", value: 5, reference: 5}),
		ui.stat({label: "Plain", value: 7, format: "duration"})))`)
	shows(t, a, ".neisland-stat-label", "text", "Revenue | Errors | Costs | Churn | Flat | Plain")
	shows(t, a, ".neisland-stat-value", "text", "$12.4K | 12 | €80.00 | 4% | 5 | 7s")
	shows(t, a, ".neisland-stat-value", "title", "$12,400 | 12 | €80 | 4% | 5 | 7 s")
	shows(t, a, ".neisland-stat-change", "text", "▲ +24.0% vs last month | ▲ +20.0% | ▼ -20.0% | ▼ -20.0% | 0%")
	shows(t, a, ".neisland-stat-delta", "class", "neisland-stat-delta neisland-stat-good | neisland-stat-delta neisland-stat-bad | neisland-stat-delta neisland-stat-good | neisland-stat-delta neisland-stat-bad | neisland-stat-delta")
	colours, err := a.Page.Eval(`() => Array.from(document.querySelectorAll(".neisland-stat-delta"), (e) => getComputedStyle(e).color)`)
	if err != nil {
		t.Fatal(err)
	}
	good, bad, neutral := colours.Value.Arr()[0].Str(), colours.Value.Arr()[1].Str(), colours.Value.Arr()[4].Str()
	if good == bad || good == neutral || bad == neutral || colours.Value.Arr()[2].Str() != good || colours.Value.Arr()[3].Str() != bad {
		t.Fatalf("change colours %v: want good, bad, good, bad and a neutral one, each distinct", colours.Value.Arr())
	}
}

func TestStatStaticPatchReplacesTheData(t *testing.T) {
	a, _ := open(t, `var s = ui.stat({label: "Signups", value: 10, reference: 5, referenceLabel: "vs yesterday"}); ui.root.set(s)`)
	shows(t, a, ".neisland-stat-value", "text", "10")
	shows(t, a, ".neisland-stat-change", "text", "▲ +100.0% vs yesterday")
	then(t, a, `s.update({value: 15, reference: 20})`)
	shows(t, a, ".neisland-stat-value", "text", "15")
	shows(t, a, ".neisland-stat-change", "text", "▼ -25.0% vs yesterday")
	then(t, a, `s.update({value: 20})`)
	shows(t, a, ".neisland-stat-value", "text", "20")
	shows(t, a, ".neisland-stat-change", "text", "")
}

func TestStatPatchInTheOtherModeIsReported(t *testing.T) {
	a, _ := open(t, `var s = ui.stat({label: "Fixed", value: 10}); var q = ui.stat({label: "Live", query: "() => ({value: 1})"}); ui.root.set(ui.row({}, s, q))`)
	shows(t, a, ".neisland-stat-value", "text", "10 | 1")
	if _, _, err := a.Continue(`s.update({query: "() => ({value: 2})"}); ui.draw(1)`, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(widgetError(t, a), "; "); got != "stat: this tile was made with value, so an update takes value and an optional reference" {
		t.Fatalf("reported %q", got)
	}
	if _, _, err := a.Continue(`q.update({value: 3}); ui.draw(1)`, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(widgetError(t, a), "; "); got != "stat: this tile was made with query, so an update takes query only" {
		t.Fatalf("reported %q", got)
	}
	then(t, a, `0`)
	shows(t, a, ".neisland-stat-value", "text", "10 | 1")
}

func TestStatQueryTileShowsItsValueOnFirstPaint(t *testing.T) {
	a, ticks := open(t, `ui.root.set(ui.stat({label: "Load", query: "() => ({value: app.tick({}) * 42, reference: 40})", referenceLabel: "vs target"}))`)
	shows(t, a, ".neisland-stat-value", "text", "42")
	shows(t, a, ".neisland-stat-change", "text", "▲ +5.0% vs target")
	if n := ticks.Load(); n != 1 {
		t.Fatalf("the query ran %d times, want once, while the tile rendered", n)
	}
}

func TestStatQueryTileRefreshes(t *testing.T) {
	a, _ := open(t, `ui.root.set(ui.stat({label: "Count", query: "() => ({value: app.tick({})})", refresh: 1}))`)
	shows(t, a, ".neisland-stat-value", "text", "1")
	shows(t, a, ".neisland-stat-value", "text", "3")
}

func TestStatQueryPatchRunsTheNewQuery(t *testing.T) {
	a, _ := open(t, `var q = ui.stat({label: "Load", query: "() => ({value: 1})"}); ui.root.set(q)`)
	shows(t, a, ".neisland-stat-value", "text", "1")
	then(t, a, `q.update({query: "() => ({value: 2, reference: 1})"})`)
	shows(t, a, ".neisland-stat-value", "text", "2")
	shows(t, a, ".neisland-stat-change", "text", "▲ +100.0%")
}

func TestStatReRenderRunsTheQueryAgain(t *testing.T) {
	a, ticks := open(t, `var q = ui.stat({label: "Load", query: "() => ({value: app.tick({})})"}); ui.root.set(q)`)
	shows(t, a, ".neisland-stat-value", "text", "1")
	then(t, a, `ui.root.set(ui.row({}, q))`)
	shows(t, a, ".neisland-row .neisland-stat-value", "text", "2")
	if n := ticks.Load(); n != 2 {
		t.Fatalf("the query ran %d times, want twice: once for each render", n)
	}
}

func TestStatFailedQueryShowsNoData(t *testing.T) {
	a, _ := open(t, `ui.root.set(ui.row({},
		ui.stat({label: "Up then down", query: "() => { if (app.tick({}) > 1) throw new Error('down'); return {value: 7} }", refresh: 1}),
		ui.stat({label: "Never", query: "() => { throw new Error('never') }"})))`)
	shows(t, a, ".neisland-stat-value", "text", "7 | No data")
	shows(t, a, ".neisland-stat-value", "text", "No data | No data")
}

func TestStatQueryBacksOff(t *testing.T) {
	var mu sync.Mutex
	var runs []time.Time
	a := stack.Island(t, devkit.App{
		Island: neui.Island{
			Name:        "board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := nekit.IslandRegisterStat(l); err != nil {
					return err
				}
				return l.Tool("fail", "records a failing run", func(context.Context, struct{}) (struct{}, error) {
					mu.Lock()
					runs = append(runs, time.Now())
					mu.Unlock()
					return struct{}{}, errors.New("down")
				})
			},
		},
		Script: `ui.root.set(ui.stat({label: "Down", query: "() => { app.fail({}); return {value: 1} }", refresh: 1})); ui.draw(1); ui.notify("drawn")`,
	})
	notified(a)
	var got []time.Time
	stack.Eventually(t, 15*time.Second, func() (bool, string) {
		mu.Lock()
		got = slices.Clone(runs)
		mu.Unlock()
		return len(got) >= 4, fmt.Sprint(len(got))
	}, "4 runs of the query")
	if doubled := got[2].Sub(got[1]); doubled < 1800*time.Millisecond {
		t.Fatalf("after a failed run of a 1s refresh, the next came after %v, want 2s", doubled)
	}
	if capped := got[3].Sub(got[2]); capped < 1800*time.Millisecond || capped > 3*time.Second {
		t.Fatalf("after two failed runs with a ceiling of 2s, the next came after %v, want 2s", capped)
	}
	shows(t, a, ".neisland-stat-value", "text", "No data")
}

func TestStatQueryLoopStopsWhenTheAppEnds(t *testing.T) {
	a, ticks := open(t, `ui.root.set(ui.stat({label: "Count", query: "() => ({value: app.tick({})})", refresh: 1}))`)
	shows(t, a, ".neisland-stat-value", "text", "2")
	if _, _, err := a.Continue(`ui.end()`, false); err != nil {
		t.Fatal(err)
	}
	a.Until(func(r devkit.Report) bool { return r.Kind == "ok" })
	ended := ticks.Load()
	time.Sleep(2500 * time.Millisecond)
	if n := ticks.Load(); n != ended {
		t.Fatalf("the query ran %d times after the app ended", n-ended)
	}
}

func TestGetStatReadsWhatTheTileShows(t *testing.T) {
	p := testkit.NewProvider(api.Model{})
	c := stack.Browser(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := nekit.IslandRegisterStat(l); err != nil {
					return err
				}
				if err := nekit.IslandRegisterLayout(l); err != nil {
					return err
				}
				return nekit.IslandRegisterButton(l)
			},
		})
	}})
	result := func(name string, args any) api.ContentToolResult {
		t.Helper()
		p.Push(testkit.Call(name, args), testkit.Say("done"))
		c.Send("go on")
		c.Wait()
		requests := p.Requests()
		return testkit.Results(requests[len(requests)-1])[0]
	}
	spec := result("page__island_spec", map[string]any{"path": "board"})
	text, _ := spec.Content[0].(api.ContentText)
	revision, _, _ := strings.Cut(strings.TrimPrefix(text.Text, "revision "), "\n")
	script := `ui.root.set(ui.column({}, ui.stat({label: "Revenue", value: 120, reference: 100}), ui.stat({label: "Open", value: 3}), ui.button({text: "OK"})))
ui.draw()`
	if r := result("page__island_begin", map[string]any{"path": "board", "revision": revision, "code": script, "comment": "the stat app"}); r.Error != api.ToolOk {
		t.Fatalf("island_begin failed: %#v", r.Content)
	}
	for space, want := range map[string]string{
		"board.screen.stat_1": `{"label":"Revenue","state":"shown","value":120,"reference":100,"change":0.2}`,
		"board.screen.stat_2": `{"label":"Open","state":"shown","value":3}`,
	} {
		stack.Eventually(t, 10*time.Second, func() (bool, string) {
			if r := result("page__sync", map[string]any{"path": space}); r.Error != api.ToolOk {
				return false, fmt.Sprint(r.Content)
			}
			r := result("page__tool", map[string]any{"path": space, "tool": "get_stat", "arg": map[string]any{}})
			var out struct{ Output json.RawMessage }
			if r.Error == api.ToolOk {
				testkit.Decode(t, r, &out)
			}
			return string(out.Output) == want, fmt.Sprint(r.Content)
		}, "get_stat of "+space+" reads "+want)
	}
}
