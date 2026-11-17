//go:build browser

package table_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
	"neui.dev/neisland/table"
)

type agent struct {
	t     *testing.T
	p     *testkit.Provider
	c     *stack.Chat
	got   chan table.Submit
	space string
}

func board(got chan<- table.Submit) gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := neisland.RegisterTable(l); err != nil {
					return err
				}
				return l.Tool("got", "takes what the table submitted", func(ctx context.Context, s table.Submit) (struct{}, error) {
					got <- s
					return struct{}{}, nil
				})
			},
		})
	}
}

func open(t *testing.T, script string) *agent {
	t.Helper()
	a := &agent{t: t, p: testkit.NewProvider(api.Model{}), got: make(chan table.Submit, 4), space: "board.screen.table_1"}
	a.c = stack.Browser(t, stack.Host{Provider: a.p, Page: board(a.got)})
	spec := a.result("page__island_spec", map[string]any{"path": "board"})
	text, _ := spec.Content[0].(api.ContentText)
	revision, _, _ := strings.Cut(strings.TrimPrefix(text.Text, "revision "), "\n")
	if r := a.result("page__island_begin", map[string]any{"path": "board", "revision": revision, "code": script, "comment": "the table app"}); r.Error != api.ToolOk {
		t.Fatalf("island_begin failed: %#v", r.Content)
	}
	stack.Eventually(t, 10*time.Second, func() (bool, string) {
		r := a.result("page__sync", map[string]any{"path": a.space})
		return r.Error == api.ToolOk, fmt.Sprint(r.Content)
	}, "the table space shows up")
	return a
}

func (a *agent) result(name string, args any) api.ContentToolResult {
	a.t.Helper()
	a.p.Push(testkit.Call(name, args), testkit.Say("done"))
	a.c.Send("go on")
	a.c.Wait()
	requests := a.p.Requests()
	return testkit.Results(requests[len(requests)-1])[0]
}

func (a *agent) call(tool string, arg any) json.RawMessage {
	a.t.Helper()
	r := a.result("page__tool", map[string]any{"path": a.space, "tool": tool, "arg": arg})
	if r.Error != api.ToolOk {
		a.t.Fatalf("%s(%v) failed: %v", tool, arg, r.Content)
	}
	var out struct{ Output json.RawMessage }
	testkit.Decode(a.t, r, &out)
	return out.Output
}

func (a *agent) then(tool string, arg any) view {
	a.t.Helper()
	stack.ExpectJSON(a.t, a.call(tool, arg), `{}`)
	return decode(a.t, a.call("get_page", map[string]any{}))
}

func (a *agent) refused(tool string, arg any, want string) {
	a.t.Helper()
	r := a.result("page__tool", map[string]any{"path": a.space, "tool": tool, "arg": arg})
	if r.Error == api.ToolOk {
		a.t.Fatalf("%s(%v) succeeded with %v, want it to fail with %q", tool, arg, r.Content, want)
	}
	if !strings.Contains(fmt.Sprint(r.Content), want) {
		a.t.Fatalf("%s(%v) failed with %v, want %q", tool, arg, r.Content, want)
	}
}

func (a *agent) submitted(want table.Submit) {
	a.t.Helper()
	select {
	case got := <-a.got:
		if got.ID != want.ID || got.Value != want.Value || !slices.Equal(got.IDs, want.IDs) {
			a.t.Fatalf("the script got %+v, want %+v", got, want)
		}
	case <-time.After(10 * time.Second):
		a.t.Fatalf("the script never got %+v", want)
	}
}

func (a *agent) shows(selector string, want string) {
	a.t.Helper()
	stack.Eventually(a.t, 10*time.Second, func() (bool, string) {
		got, err := a.c.Page.Eval(`(s) => Array.from(document.querySelectorAll(s), (e) => e.matches("input") ? e.value : e.textContent.trim()).join(" | ")`, selector)
		if err != nil {
			return false, err.Error()
		}
		return got.Value.Str() == want, got.Value.Str()
	}, "the user sees "+want+" in "+selector)
}

type view struct {
	Columns []struct {
		Title string
		Sort  bool
	}
	Rows []struct {
		ID       string
		Cells    []string
		Actions  []struct{ Value, Text string }
		Selected bool
	}
	Page  int
	Pages *int
	Sort  *struct {
		Column int
		Desc   bool
	}
	Search   string
	Selected int
}

func decode(t *testing.T, raw json.RawMessage) view {
	t.Helper()
	var v view
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%v in %s", err, raw)
	}
	return v
}

func ids(v view) string {
	var out []string
	for _, r := range v.Rows {
		out = append(out, r.ID)
	}
	return strings.Join(out, " ")
}

const twelve = `var rows = []
for (var i = 1; i <= 12; i++) rows.push({id: "r" + i, cells: ["Row " + i, "[spec " + i + "](https://example.com/" + i + ")"], actions: i % 2 ? [{value: "open", text: "Open"}] : [{value: "open", text: "Open"}, {value: "drop", text: "Drop", style: "danger"}]})
`

const loop = `
while (true) {
	var ev = ui.draw()
	app.got(ev.output)
}`

func full(t *testing.T) *agent {
	return open(t, twelve+`ui.root.set(ui.table({columns: [{title: "Name", sort: true}, {title: "Spec"}], rows: rows, pageSize: 5, search: true, clickable: true, select: {}}))`+loop)
}

func toolsOf(t *testing.T, a *agent) []string {
	t.Helper()
	r := a.result("page__sync", map[string]any{"path": a.space})
	var s struct{ Tools []struct{ Name string } }
	testkit.Decode(t, r, &s)
	var names []string
	for _, tool := range s.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestTableToolsFollowOptions(t *testing.T) {
	plain := open(t, twelve+`ui.root.set(ui.table({columns: [{title: "Name"}, {title: "Spec"}], rows: rows}))`+loop)
	if got := strings.Join(toolsOf(t, plain), " "); got != "click_action get_page set_page" {
		t.Fatalf("a table with no options has tools %q, want click_action get_page set_page", got)
	}
	a := full(t)
	if got := strings.Join(toolsOf(t, a), " "); got != "clear_selected click_action click_row get_page get_selected set_page set_search set_selected set_sort submit_selected" {
		t.Fatalf("a table with every option has tools %q", got)
	}
}

func TestTableGetPageReadsTheUsersView(t *testing.T) {
	a := full(t)
	v := decode(t, a.call("get_page", map[string]any{}))
	if len(v.Columns) != 2 || v.Columns[0].Title != "Name" || !v.Columns[0].Sort || v.Columns[1].Sort {
		t.Fatalf("columns %+v, want Name sortable and Spec not", v.Columns)
	}
	if ids(v) != "r1 r2 r3 r4 r5" || v.Page != 1 || v.Pages == nil || *v.Pages != 3 || v.Sort != nil || v.Search != "" || v.Selected != 0 {
		t.Fatalf("view %+v, want rows r1 to r5 on page 1 of 3, unsorted, no search, nothing selected", v)
	}
	if got := strings.Join(v.Rows[1].Cells, " | "); got != "Row 2 | [spec 2](https://example.com/2)" {
		t.Fatalf("row r2 cells %q, want their markdown with the link target", got)
	}
	if len(v.Rows[1].Actions) != 2 || v.Rows[1].Actions[1].Value != "drop" || v.Rows[1].Actions[1].Text != "Drop" {
		t.Fatalf("row r2 actions %+v, want open and drop", v.Rows[1].Actions)
	}
}

func TestTableGetPageReadsOnlyWhatTheUserSees(t *testing.T) {
	a := open(t, `ui.root.set(ui.table({columns: [{title: "Name"}, {title: "Spec"}], selected: ["r1"], rows: [
	{id: "r1", cells: ["Ann", "a", "hidden"]},
	{id: "r2", cells: ["Bob"]}
]}))`+loop)
	stack.ExpectJSON(t, a.call("get_page", map[string]any{}), `{
	"columns": [{"title": "Name"}, {"title": "Spec"}],
	"rows": [{"id": "r1", "cells": ["Ann", "a"]}, {"id": "r2", "cells": ["Bob", ""]}],
	"page": 1,
	"pages": 1
}`)
}

func TestTableSetPageTurnsTheUsersPage(t *testing.T) {
	a := full(t)
	v := a.then("set_page", map[string]any{"page": 3})
	if ids(v) != "r11 r12" || v.Page != 3 {
		t.Fatalf("set_page 3 read %+v, want rows r11 r12 on page 3", v)
	}
	a.shows(".neisland-table-status-text", "Showing 11–12 of 12")
	a.refused("set_page", map[string]any{"page": 4}, "no page 4: the last is 3")
	a.shows(".neisland-table-status-text", "Showing 11–12 of 12")
}

func TestTableSetPageWithoutTotalNeedsRows(t *testing.T) {
	a := open(t, `ui.root.set(ui.table({columns: [{title: "Entry"}], pageSize: 5, query: String(function (p) {
		var rows = []
		for (var i = p.offset; i < Math.min(p.offset + p.limit, 8); i++) rows.push({id: "e" + (i + 1), cells: ["entry " + (i + 1)]})
		return {rows: rows}
	})}))`+loop)
	v := a.then("set_page", map[string]any{"page": 2})
	if ids(v) != "e6 e7 e8" || v.Page != 2 || v.Pages != nil {
		t.Fatalf("set_page 2 read %+v, want e6 to e8 on page 2 with no page count", v)
	}
	a.refused("set_page", map[string]any{"page": 3}, "no page 3: it has no rows")
	a.shows(".neisland-table-row > .neisland-table-cell:first-child", "entry 6 | entry 7 | entry 8")
}

func TestTableSetSearchTypesForTheUser(t *testing.T) {
	a := full(t)
	a.call("set_page", map[string]any{"page": 2})
	v := a.then("set_search", map[string]any{"text": " row 1 "})
	if ids(v) != "r1 r10 r11 r12" || v.Page != 1 || v.Search != "row 1" {
		t.Fatalf("set_search read %+v, want r1 r10 r11 r12 on page 1 for row 1", v)
	}
	a.shows(".neisland-table-search", "row 1")
	a.shows(".neisland-table-status-text", "Showing 1–4 of 4")
	v = a.then("set_search", map[string]any{"text": ""})
	if v.Search != "" || *v.Pages != 3 {
		t.Fatalf("an empty search read %+v, want every row back", v)
	}
	a.shows(".neisland-table-search", "")
}

func TestTableSetSortSortsForTheUser(t *testing.T) {
	a := full(t)
	a.call("set_page", map[string]any{"page": 2})
	v := a.then("set_sort", map[string]any{"column": 0, "desc": true})
	if ids(v) != "r12 r11 r10 r9 r8" || v.Page != 1 || v.Sort == nil || v.Sort.Column != 0 || !v.Sort.Desc {
		t.Fatalf("set_sort read %+v, want r12 down to r8 on page 1, sorted by Name descending", v)
	}
	a.shows(`th[aria-sort="descending"]`, "Name▼")
	a.refused("set_sort", map[string]any{"column": 1}, "column 1 is not sortable")
	a.refused("set_sort", map[string]any{"column": 5}, "column 5 is not sortable")
}

func TestTableSelectionTools(t *testing.T) {
	a := full(t)
	v := a.then("set_selected", map[string]any{"ids": []string{"r2", "r4"}, "selected": true})
	if v.Selected != 2 || !v.Rows[1].Selected || !v.Rows[3].Selected || v.Rows[0].Selected {
		t.Fatalf("set_selected read %+v, want r2 and r4 selected", v)
	}
	a.shows(".neisland-table-status-text", "2 selected")
	a.refused("set_selected", map[string]any{"ids": []string{"r9"}, "selected": true}, `row "r9" is not on the current page`)
	a.call("set_page", map[string]any{"page": 2})
	a.call("set_selected", map[string]any{"ids": []string{"r6"}, "selected": true})
	v = a.then("set_selected", map[string]any{"ids": []string{"r6"}, "selected": false})
	if v.Selected != 2 || v.Rows[0].Selected {
		t.Fatalf("unselecting r6 read %+v, want r6 unselected and 2 left", v)
	}
	a.call("set_selected", map[string]any{"ids": []string{"r7"}, "selected": true})
	stack.ExpectJSON(t, a.call("get_selected", map[string]any{}), `{"ids": ["r2", "r4", "r7"]}`)
	a.call("submit_selected", map[string]any{})
	a.submitted(table.Submit{IDs: []string{"r2", "r4", "r7"}})
	a.call("clear_selected", map[string]any{})
	a.shows(".neisland-table-status-text", "Showing 6–10 of 12")
	stack.ExpectJSON(t, a.call("get_selected", map[string]any{}), `{"ids": []}`)
	a.refused("submit_selected", map[string]any{}, "nothing is selected")
}

func TestTableClickRowSubmitsTheRow(t *testing.T) {
	a := full(t)
	a.call("click_row", map[string]any{"id": "r3"})
	a.submitted(table.Submit{ID: "r3"})
	a.refused("click_row", map[string]any{"id": "r12"}, `row "r12" is not on the current page`)
}

func TestTableClickActionReachesTheScript(t *testing.T) {
	a := full(t)
	a.call("click_action", map[string]any{"id": "r4", "value": "drop"})
	a.submitted(table.Submit{ID: "r4", Value: "drop"})
	a.refused("click_action", map[string]any{"id": "r3", "value": "drop"}, `row "r3" has no action "drop"`)
	a.refused("click_action", map[string]any{"id": "r8", "value": "open"}, `row "r8" is not on the current page`)
}

func TestTableSubmitFailsWithNoDrawWaiting(t *testing.T) {
	a := open(t, twelve+`ui.root.set(ui.table({columns: [{title: "Name"}, {title: "Spec"}], rows: rows, clickable: true}))
var ev = ui.draw()
app.got(ev.output)`)
	a.call("click_action", map[string]any{"id": "r1", "value": "open"})
	a.submitted(table.Submit{ID: "r1", Value: "open"})
	a.refused("click_row", map[string]any{"id": "r2"}, "not submitted: the script is not waiting in ui.draw()")
}

func TestTableSetSelectedKeepsTheCap(t *testing.T) {
	a := open(t, twelve+`var held = []
for (var i = 0; i < 999; i++) held.push("x" + i)
ui.root.set(ui.table({columns: [{title: "Name"}, {title: "Spec"}], rows: rows, pageSize: 5, select: {}, selected: held}))`+loop)
	a.refused("set_selected", map[string]any{"ids": []string{"r1", "r2"}, "selected": true}, "the selection holds at most 1000 rows, and 999 are selected")
	v := a.then("set_selected", map[string]any{"ids": []string{"r1"}, "selected": true})
	if v.Selected != 1000 || !v.Rows[0].Selected {
		t.Fatalf("selecting r1 read %+v, want it selected as the 1000th", v)
	}
	a.shows(".neisland-table-status-text", "1000 selected")
}
func TestTableSubmitReturnsWhenThePartEndsWithoutADraw(t *testing.T) {
	a := open(t, twelve+`ui.root.set(ui.table({columns: [{title: "Name"}, {title: "Spec"}], rows: rows, pageSize: 5, clickable: true})); app.got(ui.draw().output)`)
	began := time.Now()
	a.call("click_row", map[string]any{"id": "r3"})
	if d := time.Since(began); d > 2*time.Second {
		t.Fatalf("click_row returned after %v, want it back once the part ends", d)
	}
	a.submitted(table.Submit{ID: "r3"})
}
