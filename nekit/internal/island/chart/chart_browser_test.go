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

package chart

import (
	"context"
	"encoding/json"
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
	"neui.dev/nekit/internal/island"
	"neui.dev/nekit/internal/island/button"
	"neui.dev/nekit/internal/island/layout"
)

type board struct {
	*stack.IslandApp
	t       *testing.T
	ticks   atomic.Int64
	mu      sync.Mutex
	reports []devkit.Report
	runs    []time.Time
}

func open(t *testing.T, script string) *board {
	t.Helper()
	b := &board{t: t}
	b.IslandApp = stack.Island(t, devkit.App{
		Island: neui.Island{
			Name:        "board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				l.Head(island.Style())
				if err := l.View("chart", Desc, New); err != nil {
					return err
				}
				if err := l.Layout("column", "", layout.Column); err != nil {
					return err
				}
				return l.Tool("tick", "counts its calls", func(ctx context.Context, _ struct{}) (int64, error) {
					b.mu.Lock()
					b.runs = append(b.runs, time.Now())
					b.mu.Unlock()
					return b.ticks.Add(1), nil
				})
			},
		},
		Script: script,
		Report: func(r devkit.Report) {
			b.mu.Lock()
			b.reports = append(b.reports, r)
			b.mu.Unlock()
		},
	})
	return b
}

func (b *board) reported(kind string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, r := range b.reports {
		if r.Meta["kind"] == kind {
			out = append(out, string(r.Data))
		}
	}
	return out
}

func (b *board) waitRuns(count int) []time.Time {
	b.t.Helper()
	var runs []time.Time
	stack.Eventually(b.t, 15*time.Second, func() (bool, string) {
		b.mu.Lock()
		runs = slices.Clone(b.runs)
		b.mu.Unlock()
		return len(runs) >= count, fmt.Sprint(len(runs))
	}, fmt.Sprintf("%d runs of the query", count))
	return runs
}

func (b *board) waitReport(kind string, want string) {
	b.t.Helper()
	stack.Eventually(b.t, 10*time.Second, func() (bool, string) {
		got := b.reported(kind)
		return strings.Contains(strings.Join(got, "\n"), want), strings.Join(got, "\n")
	}, fmt.Sprintf("a %s report with %q", kind, want))
}

func (b *board) aria(index int) string {
	return b.Page.MustEval(`(i) => document.querySelectorAll(".neisland-chart-plot")[i]?.getAttribute("aria-label") ?? ""`, index).Str()
}

func (b *board) waitAria(index int, want string) {
	b.t.Helper()
	stack.Eventually(b.t, 10*time.Second, func() (bool, string) {
		got := b.aria(index)
		return strings.Contains(got, want), got
	}, fmt.Sprintf("chart %d draws %q", index, want))
}

func (b *board) shown(selector string) bool {
	return b.Page.MustEval(`(s) => { const e = document.querySelector(s); return !!e && !e.hidden }`, selector).Bool()
}

func (b *board) waitShown(selector string, want bool) {
	b.t.Helper()
	stack.Eventually(b.t, 10*time.Second, func() (bool, string) {
		got := b.shown(selector)
		return got == want, fmt.Sprint(got)
	}, fmt.Sprintf("%s shown is %v", selector, want))
}

func (b *board) then(code string) {
	b.t.Helper()
	if _, _, err := b.Continue(code+`; while (true) ui.draw()`, true); err != nil {
		b.t.Fatal(err)
	}
}

const idle = `; ui.draw(1); ui.notify("drawn"); while (true) ui.draw()`

func TestChartDrawsRows(t *testing.T) {
	b := open(t, `ui.root.set(ui.chart({type: "bar", title: "Sales", subtitle: "2026", x: "q", y: "v", splitBy: "g", rows: [
		{q: "Q1", v: 12, g: "north"}, {q: "Q2", v: 18, g: "south"}
	]}))`+idle)
	b.waitAria(0, "Bar chart, 2 series. north: Q1 is 12; Q2 is –. south: Q1 is –; Q2 is 18.")
	b.Find(".neisland-chart-title").WaitText("Sales")
	b.Find(".neisland-chart-subtitle").WaitText("2026")
	b.waitShown(".neisland-chart-empty", false)
	b.waitShown(".neisland-chart-stale", false)
}

func TestChartRowsPatchRedraws(t *testing.T) {
	b := open(t, `var sales = ui.chart({type: "line", x: "m", y: "v", rows: [{m: "Jan", v: 1}, {m: "Feb", v: 2}]})
ui.root.set(sales)`+idle)
	b.waitAria(0, "Feb is 2.")
	b.then(`sales.update({rows: [{m: "Jan", v: 7}, {m: "Feb", v: 9}, {m: "Mar", v: 11}]})`)
	b.waitAria(0, "Mar is 11.")
	b.then(`sales.update({rows: [{m: "Apr"}]})`)
	b.waitReport("widget_error", `rows[0] has no y field \"v\"`)
	b.waitShown(".neisland-chart-stale", true)
	if got := b.aria(0); !strings.Contains(got, "Mar is 11.") {
		t.Fatalf("after rows that do not fit, the chart draws %q, want the rows before", got)
	}
	b.then(`sales.update({query: "() => ({ rows: [] })"})`)
	b.waitReport("widget_error", "this chart was made with rows: patch it with rows, not query")
	b.then(`sales.update({rows: [{m: "May", v: 3}]})`)
	b.waitAria(0, "May is 3.")
	b.waitShown(".neisland-chart-stale", false)
}

func TestChartQueryRefreshes(t *testing.T) {
	b := open(t, `var live = ui.chart({type: "bar", x: "k", y: "n", refresh: 5, query: String(function () {
		return { rows: [{ k: "calls", n: app.tick({}) }] }
	})})
ui.root.set(live)`+idle)
	b.waitAria(0, "calls is 1.")
	b.waitAria(0, "calls is 2.")
	b.then(`live.update({query: String(function () { return { rows: [{ k: "fixed", n: 100 + app.tick({}) }] } })})`)
	began := time.Now()
	stack.Eventually(t, 10*time.Second, func() (bool, string) {
		got := b.aria(0)
		return strings.Contains(got, "fixed is 1"), got
	}, "the new query draws")
	if d := time.Since(began); d > 2*time.Second {
		t.Fatalf("the new query drew after %v, want it to run right away", d)
	}
	b.then(`live.update({rows: [{k: "a", n: 1}]})`)
	b.waitReport("widget_error", "this chart was made with query: patch it with query, not rows")
}

func TestChartRefreshHasAFloor(t *testing.T) {
	floor := refreshFloor
	refreshFloor = 2 * time.Second
	t.Cleanup(func() { refreshFloor = floor })
	b := open(t, `ui.root.set(ui.chart({type: "bar", x: "k", y: "n", refresh: 1, query: String(function () {
		return { rows: [{ k: "calls", n: app.tick({}) }] }
	})}))`+idle)
	runs := b.waitRuns(3)
	for i := 1; i < 3; i++ {
		if gap := runs[i].Sub(runs[i-1]); gap < 1800*time.Millisecond {
			t.Fatalf("with refresh 1 and a floor of 2s, run %d came %v after the one before", i, gap)
		}
	}
}

func TestChartQueryBacksOff(t *testing.T) {
	b := open(t, `ui.root.set(ui.chart({type: "bar", x: "k", y: "n", refresh: 1, query: String(function () {
		app.tick({})
		throw new Error("down")
	})}))`+idle)
	runs := b.waitRuns(4)
	doubled, capped := runs[2].Sub(runs[1]), runs[3].Sub(runs[2])
	if doubled < 1800*time.Millisecond {
		t.Fatalf("after a failed run of a 1s refresh, the next came after %v, want 2s", doubled)
	}
	if capped < 1800*time.Millisecond || capped > 3*time.Second {
		t.Fatalf("after two failed runs with a ceiling of 2s, the next came after %v, want 2s", capped)
	}
}

func TestChartsKeepTheirOwnUpdates(t *testing.T) {
	b := open(t, `var left = ui.chart({type: "bar", x: "k", y: "n", rows: [{k: "left", n: 1}]})
var right = ui.chart({type: "bar", x: "k", y: "n", rows: [{k: "right", n: 2}]})
ui.root.set(ui.column({}, left, right))`+idle)
	b.waitAria(0, "left is 1.")
	b.waitAria(1, "right is 2.")
	b.then(`left.update({rows: [{k: "left", n: 5}]})`)
	b.waitAria(0, "left is 5.")
	b.then(`right.update({rows: [{k: "right", n: 7}]})`)
	b.waitAria(1, "right is 7.")
	b.then(`left.update({rows: [{k: "left", n: 9}]})`)
	b.waitAria(0, "left is 9.")
	time.Sleep(300 * time.Millisecond)
	if got := b.aria(1); !strings.Contains(got, "right is 7.") {
		t.Fatalf("after updates of the left chart, the right one draws %q", got)
	}
}

func TestChartQueryStopsWhenTheAppEnds(t *testing.T) {
	b := open(t, `ui.root.set(ui.chart({type: "bar", x: "k", y: "n", refresh: 1, query: String(function () {
		return { rows: [{ k: "calls", n: app.tick({}) }] }
	})}))`+idle)
	b.waitAria(0, "calls is 2.")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := b.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	ended := b.ticks.Load()
	time.Sleep(3500 * time.Millisecond)
	if got := b.ticks.Load(); got > ended+1 {
		t.Fatalf("the query ran %d times after the app ended, want it stopped", got-ended)
	}
	if got := append(b.reported("widget_error"), b.reported("call_error")...); len(got) > 0 {
		t.Fatalf("the end of the app was reported as %v", got)
	}
}

func TestChartQueryFailures(t *testing.T) {
	b := open(t, `var flaky = ui.chart({type: "bar", x: "k", y: "n", query: String(function () { throw new Error("down") })})
ui.root.set(flaky)`+idle)
	b.waitReport("call_error", "down")
	b.waitShown(".neisland-chart-empty", true)
	b.waitShown(".neisland-chart-stale", false)
	b.then(`flaky.update({query: String(function () {
		var n = app.tick({})
		return n === 1 ? { rows: [{ k: "ok", n: 4 }] } : n === 2 ? { rows: [{ k: "ok" }] } : { rows: [] }
	})})`)
	b.waitAria(0, "ok is 4.")
	b.waitShown(".neisland-chart-empty", false)
	b.then(`flaky.update({query: String(function () { app.tick({}); return { rows: [{ k: "ok" }] } })})`)
	b.waitReport("widget_error", `query result: rows[0] has no y field \"n\"`)
	b.waitShown(".neisland-chart-stale", true)
	if got := b.aria(0); !strings.Contains(got, "ok is 4.") {
		t.Fatalf("after a result that does not fit, the chart draws %q, want the data before", got)
	}
	b.then(`flaky.update({query: String(function () { return { rows: [] } })})`)
	b.waitShown(".neisland-chart-empty", true)
	b.waitShown(".neisland-chart-stale", false)
}

func TestChartRefusesParamsAtTheScriptLine(t *testing.T) {
	b := open(t, `ui.root.set(ui.chart({type: "pie", x: "k", y: "n", splitBy: "g", rows: [{k: "a", n: 1, g: "x"}]}))`+idle)
	stack.Eventually(t, 10*time.Second, func() (bool, string) {
		var messages []string
		for _, data := range b.reported("script_error") {
			var message string
			json.Unmarshal([]byte(data), &message)
			messages = append(messages, message)
		}
		text := strings.Join(messages, "\n")
		return strings.Contains(text, "ui.chart: pie takes no splitBy: x already names each slice\n    at <anonymous> (<eval>:1:"), text
	}, "the script fails at the ui.chart line")
}

func TestChartQueryStopsWhenOvertaken(t *testing.T) {
	b := open(t, `ui.root.set(ui.chart({type: "bar", x: "k", y: "n", refresh: 1, query: String(function () {
		return { rows: [{ k: "calls", n: app.tick({}) }] }
	})}))`+idle)
	b.waitAria(0, "calls is 2.")
	if _, _, err := b.Start(context.Background(), `var waiting = true`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3500 * time.Millisecond)
	if !strings.Contains(b.aria(0), "calls is") {
		t.Fatalf("the chart of the overtaken app is gone: %q", b.aria(0))
	}
	if b.shown(".neisland-chart-stale") {
		t.Fatal("the chart of the overtaken app is marked out of date")
	}
	if got := append(b.reported("widget_error"), b.reported("call_error")...); len(got) > 0 {
		t.Fatalf("the overtaken app reported %v", got)
	}
}

func TestChartRendersAgainFromItsLatestRows(t *testing.T) {
	b := open(t, `var sales = ui.chart({type: "line", x: "m", y: "v", rows: [{m: "Jan", v: 1}]})
ui.root.set(sales)`+idle)
	b.waitAria(0, "Jan is 1.")
	b.then(`sales.update({rows: [{m: "Jan", v: 7}]})`)
	b.waitAria(0, "Jan is 7.")
	b.Page.MustEval(`() => { document.querySelector(".neisland-chart-plot").dataset.before = "" }`)
	b.then(`ui.root.set(ui.column({}, ui.chart({type: "bar", x: "k", y: "n", rows: [{k: "first", n: 1}]}), sales))`)
	b.waitAria(1, "Jan is 7.")
	if b.Page.MustEval(`() => document.querySelectorAll(".neisland-chart-plot[data-before]").length`).Int() != 0 {
		t.Fatal("the chart was moved, not rendered again")
	}
	b.then(`sales.update({rows: [{m: "Jan", v: 8}]})`)
	b.waitAria(1, "Jan is 8.")
}

func TestChartDescribesFormattedValues(t *testing.T) {
	b := open(t, `ui.root.set(ui.column({}, ui.chart({type: "line", x: "m", y: "v", format: "currency", unit: "USD", rows: [
		{m: "2026-01", v: 1}, {m: "2026-02", v: 2}, {m: "2026-03", v: 3}, {m: "2026-04", v: 4}, {m: "2026-05", v: 5}, {m: "2026-06", v: 6},
		{m: "2026-07", v: 7}, {m: "2026-08", v: 8}, {m: "2026-09", v: 9}, {m: "2026-10", v: 10}, {m: "2026-11", v: 11}, {m: "2026-12", v: null}
	]}), ui.chart({type: "pie", x: "k", y: "n", rows: [{k: "a", n: 1}, {k: "b", n: 3}]})))`+idle)
	want := b.Page.MustEval(`() => {
		const month = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "short", timeZone: "UTC" })
		const money = new Intl.NumberFormat(undefined, { style: "currency", currency: "USD" })
		const items = []
		for (let i = 2; i < 12; i++) items.push(month.format(Date.UTC(2026, i)) + " is " + (i === 11 ? "–" : money.format(i + 1)))
		return "Line chart, 1 series. v: 2 earlier; " + items.join("; ") + "."
	}`).Str()
	b.waitAria(0, want)
	share := b.Page.MustEval(`() => new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 1 }).format(0.25)`).Str()
	b.waitAria(1, "Pie chart, 2 slices: a is 1 ("+share+"); b is 3")
}

func TestGetChartReadsWhatTheChartShows(t *testing.T) {
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
				l.Head(island.Style())
				if err := l.View("chart", Desc, New); err != nil {
					return err
				}
				if err := l.Layout("column", "", layout.Column); err != nil {
					return err
				}
				return l.Control("button", "", button.New)
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
	script := `ui.root.set(ui.column({}, ui.chart({type: "line", title: "Sales", x: "m", y: "v", splitBy: "g", rows: [{m: "2026-02", v: 2, g: "a"}, {m: "2026-01", v: 1, g: "b"}]}), ui.chart({type: "pie", x: "k", y: "v", rows: [{k: "x", v: 1}, {k: "y", v: 3}]}), ui.button({text: "OK"})))
ui.draw()`
	if r := result("page__island_begin", map[string]any{"path": "board", "revision": revision, "code": script, "comment": "the chart app"}); r.Error != api.ToolOk {
		t.Fatalf("island_begin failed: %#v", r.Content)
	}
	for space, want := range map[string]string{
		"board.screen.chart_1": `{"type":"line","title":"Sales","state":"shown","series":["b","a"],"points":2,"first":"2026-01","last":"2026-02"}`,
		"board.screen.chart_2": `{"type":"pie","state":"shown","series":["x","y"],"points":2}`,
	} {
		stack.Eventually(t, 10*time.Second, func() (bool, string) {
			if r := result("page__sync", map[string]any{"path": space}); r.Error != api.ToolOk {
				return false, fmt.Sprint(r.Content)
			}
			r := result("page__tool", map[string]any{"path": space, "tool": "get_chart", "arg": map[string]any{}})
			var out struct{ Output json.RawMessage }
			if r.Error == api.ToolOk {
				testkit.Decode(t, r, &out)
			}
			return string(out.Output) == want, fmt.Sprint(r.Content)
		}, "get_chart of "+space+" reads "+want)
	}
}

func TestGetChartFollowsTheDrawnState(t *testing.T) {
	var mu sync.Mutex
	var charts []*widget
	a := stack.Island(t, devkit.App{
		Island: neui.Island{
			Name:        "board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				l.Head(island.Style())
				if err := l.Layout("column", "", layout.Column); err != nil {
					return err
				}
				return l.View("chart", Desc, func(ctx context.Context, p Params) (api.Widget[Patch], error) {
					w, err := New(ctx, p)
					if err == nil {
						mu.Lock()
						charts = append(charts, w.(*widget))
						mu.Unlock()
					}
					return w, err
				})
			},
		},
		Script: `var sales = ui.chart({type: "bar", x: "k", y: "n", rows: [{k: "a", n: 1}]})
var live = ui.chart({type: "bar", x: "k", y: "n", query: "() => ({ rows: [{ k: 'b', n: 2 }] })"})
var down = ui.chart({type: "bar", x: "k", y: "n", query: "() => { throw new Error('down') }"})
ui.root.set(ui.column({}, sales, live, down))` + idle,
	})
	b := &board{IslandApp: a, t: t}
	b.waitAria(1, "b is 2.")
	read := func(i int) string {
		mu.Lock()
		w := charts[i]
		mu.Unlock()
		got, _ := w.getChart(context.Background(), struct{}{})
		return jsonOf(t, got)
	}
	waitState := func(i int, want string) {
		t.Helper()
		stack.Eventually(t, 10*time.Second, func() (bool, string) {
			got := read(i)
			return got == want, got
		}, fmt.Sprintf("chart %d reads %s", i, want))
	}
	waitState(0, `{"type":"bar","state":"shown","series":["n"],"points":1,"first":"a","last":"a"}`)
	waitState(1, `{"type":"bar","state":"shown","series":["n"],"points":1,"first":"b","last":"b"}`)
	waitState(2, `{"type":"bar","state":"no data"}`)
	b.then(`sales.update({rows: [{k: "a"}]}); live.update({query: "() => { throw new Error('gone') }"})`)
	waitState(0, `{"type":"bar","state":"out of date","series":["n"],"points":1,"first":"a","last":"a"}`)
	waitState(1, `{"type":"bar","state":"out of date","series":["n"],"points":1,"first":"b","last":"b"}`)
	b.then(`sales.update({rows: [{k: "c", n: 3}, {k: "d", n: 4}]})`)
	waitState(0, `{"type":"bar","state":"shown","series":["n"],"points":2,"first":"c","last":"d"}`)
}
