//go:build browser

package devkit_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func TestTableRowsPatchKeepsSelectedRows(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var table = ui.table({columns: [{title: "Name"}], selectable: true, selected: ["c", "b"], rows: [
			{id: "a", cells: ["Ann"]}, {id: "b", cells: ["Bob"]}, {id: "c", cells: ["Cid"]}
		]})
		var patch = ui.button({text: "Patch"})
		ui.root.set(ui.column({}, table, patch))
		while (true) {
			var ev = ui.draw()
			if (ev.id === patch.id) {
				table.update({rows: [{id: "c", cells: ["Cid"]}, {id: "a", cells: ["Ann"]}]})
				continue
			}
			ui.notify(ev)
		}`})
	status := a.Find(".kisland-table-status-text")
	status.WaitText("2 selected")
	a.Find(".kisland-button-md").Click()
	status.WaitText("1 selected")
	a.Find(".kisland-table-grid").WaitCount("tbody .kisland-table-checkbox:checked", 1)
	a.Find(".kisland-table-grid").WaitCount(`input[aria-label="Select Cid"]:checked`, 1)
	a.Find(`input[aria-label="Select Ann"]`).Click()
	status.WaitText("2 selected")
	a.Find(".kisland-table-status .kisland-button-primary").Click()
	stack.ExpectJSON(t, a.Submitted(), `{"ids": ["a", "c"]}`)
}

func TestTableQueryWithoutTotalStaysOnLastFullPage(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		ui.root.set(ui.table({columns: [{title: "Entry"}], pageSize: 5, query: String(function (p) {
			var rows = []
			for (var i = p.offset; i < Math.min(p.offset + p.limit, 20); i++) rows.push({id: "e" + (i + 1), cells: ["entry " + (i + 1)]})
			return {rows: rows}
		})}))
		ui.notify(ui.draw())`})
	first := a.Find(".kisland-table-row > .kisland-table-cell")
	prev := a.Find(".kisland-table-pager > .kisland-table-page:first-child")
	next := a.Find(".kisland-table-pager > .kisland-table-page:last-child")
	status := a.Find(".kisland-table-footer > .kisland-table-status .kisland-table-status-text")
	first.WaitText("entry 1")
	status.WaitText("Showing 1–5")
	for _, want := range []string{"entry 6", "entry 11", "entry 16"} {
		next.Click()
		first.WaitText(want)
	}
	status.WaitText("Showing 16–20")
	next.Click()
	time.Sleep(time.Second)
	first.WaitText("entry 16")
	prev.WaitNoAttr("disabled")
	prev.Click()
	first.WaitText("entry 11")
}

func tableRows(n int) string {
	return `(function () { var rows = []; for (var i = 1; i <= ` + strconv.Itoa(n) + `; i++) rows.push({id: "r" + i, cells: ["Row " + i]}); return rows })()`
}

const footerJS = `() => {
	const box = (s) => document.querySelector(s).getBoundingClientRect()
	const f = box(".kisland-table-footer"), s = box(".kisland-table-status"), p = box(".kisland-table-pager")
	return [Math.round(s.width), Math.round(s.height), Math.round(s.left - f.left), Math.round(f.right - p.right), Math.round(p.left - f.left), p.top >= s.bottom, document.documentElement.scrollWidth <= window.innerWidth].join(" ")
}`

func TestTableFooterStatusSlotKeepsSizeWithSelection(t *testing.T) {
	a := layoutApp(t, 1280, `ui.table({columns: [{title: "Name"}], selectable: true, rows: `+tableRows(57)+`})`)
	status := a.Find(".kisland-table-footer > .kisland-table-status")
	status.Find(".kisland-table-status-text").WaitText("Showing 1–20 of 57")
	before := a.Page.MustEval(footerJS).String()
	if f := strings.Fields(before); f[0] != "208" || f[1] != "24" || f[2] != "0" || f[3] != "0" || f[5] != "false" {
		t.Fatalf("footer %q, want a 208 by 24 status at the left and the pager at the right on one line", before)
	}
	a.Find(`input[aria-label="Select Row 2"]`).Click()
	status.Find(".kisland-table-status-text").WaitText("1 selected")
	status.WaitCount(".kisland-table-clear", 1)
	status.WaitCount(".kisland-button", 1)
	expectEval(t, a, footerJS, before, "status slot and pager keep their place with a selection")
	status.Find(".kisland-table-clear").Click()
	status.Find(".kisland-table-status-text").WaitText("Showing 1–20 of 57")
	expectEval(t, a, footerJS, before, "status slot and pager keep their place after clear")
	a.Find(".kisland-table-pager > .kisland-table-page:last-child").Click()
	status.Find(".kisland-table-status-text").WaitText("Showing 41–57 of 57")
}

func TestTableFooterWrapsPagerOnNarrowPage(t *testing.T) {
	a := layoutApp(t, 390, `ui.table({columns: [{title: "Name"}], selectable: true, pageSize: 10, rows: `+tableRows(100)+`})`)
	a.Find(".kisland-table-status-text").WaitText("Showing 1–10 of 100")
	got := a.Page.MustEval(footerJS).String()
	if f := strings.Fields(got); f[0] != "208" || f[1] != "24" || f[2] != "0" || f[4] != "0" || f[5] != "true" || f[6] != "true" {
		t.Fatalf("footer %q, want the 208 by 24 status first and the pager wrapped below it at the left, no page scroll", got)
	}
}

func TestTableFooterCentersStatusWithPager(t *testing.T) {
	a := layoutApp(t, 640, `ui.table({columns: [{title: "Name"}], pageSize: 10, rows: `+tableRows(100)+`})`)
	a.Find(".kisland-table-status-text").WaitText("Showing 1–10 of 100")
	expectEval(t, a, `() => {
		const center = (n) => {
			const r = n.getBoundingClientRect()
			return r.top + r.height / 2
		}
		const status = document.querySelector(".kisland-table-status")
		const pager = document.querySelector(".kisland-table-pager")
		return String(Math.abs(center(status) - center(pager)) < 0.5)
	}`, "true", "status and pager centered on one line below the 48rem container width")
}

func TestTableFooterStatusWhenEmpty(t *testing.T) {
	a := layoutApp(t, 1280, `ui.table({columns: [{title: "Name"}], rows: []})`)
	a.Find(".kisland-table-footer > .kisland-table-status .kisland-table-status-text").WaitText("Nothing to show")
	a.Find(".kisland-table-footer").WaitGone(".kisland-table-pager")
}

const fillerJS = `() => {
	const g = document.querySelector(".kisland-table-grid")
	const row = g.querySelector("tbody > .kisland-table-row")
	const f = Array.from(g.querySelectorAll("tbody > .kisland-table-filler"))
	const plain = f.every((r) => r.getAttribute("aria-hidden") === "true" && r.cells.length === row.cells.length &&
		!r.querySelector("button, input, a, [tabindex]") && r.textContent.trim() === "" &&
		r.getBoundingClientRect().height === row.getBoundingClientRect().height && getComputedStyle(r).backgroundColor === "rgba(0, 0, 0, 0)")
	return [Math.round(g.getBoundingClientRect().height), g.querySelectorAll("tbody > .kisland-table-row").length, f.length, plain].join(" ")
}`

func tableRowsWithActions(n int) string {
	return `(function () { var rows = []; for (var i = 1; i <= ` + strconv.Itoa(n) + `; i++) rows.push({id: "r" + i, cells: ["Row " + i, "x"], actions: [{value: "open", text: "Open"}]}); return rows })()`
}

func TestTableLastPageOfRowsPadsWithFillers(t *testing.T) {
	a := layoutApp(t, 1280, `ui.table({columns: [{title: "Name"}, {title: "Note"}], selectable: true, pageSize: 5, rows: `+tableRowsWithActions(12)+`})`)
	status := a.Find(".kisland-table-status-text")
	status.WaitText("Showing 1–5 of 12")
	first := strings.Fields(a.Page.MustEval(fillerJS).String())
	if first[1] != "5" || first[2] != "0" {
		t.Fatalf("page 1 %v, want 5 rows and no fillers", first)
	}
	a.Find(".kisland-table-pager > .kisland-table-page:last-child").Click()
	status.WaitText("Showing 11–12 of 12")
	expectEval(t, a, fillerJS, first[0]+" 2 3 true", "last page keeps the height with 3 plain hidden fillers")
	a.Find(`thead .kisland-table-checkbox`).Click()
	status.WaitText("2 selected")
	a.Find("thead").WaitCount(".kisland-table-checkbox:checked", 1)
	a.Find(".kisland-table-grid").WaitCount("tbody .kisland-table-checkbox:checked", 2)
	a.Find(".kisland-table-grid").WaitCount("tbody .kisland-table-checkbox", 2)
}

func TestTableLastPageOfQueryPadsWithFillers(t *testing.T) {
	for _, c := range []struct{ name, total, suffix string }{
		{"total", "12", " of 12"},
		{"no total", "undefined", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := layoutApp(t, 1280, `ui.table({columns: [{title: "Entry"}], pageSize: 5, query: String(function (p) {
				var rows = []
				for (var i = p.offset; i < Math.min(p.offset + p.limit, 12); i++) rows.push({id: "e" + (i + 1), cells: ["entry " + (i + 1)]})
				return {rows: rows, total: `+c.total+`}
			})})`)
			status := a.Find(".kisland-table-status-text")
			status.WaitText("Showing 1–5" + c.suffix)
			first := strings.Fields(a.Page.MustEval(fillerJS).String())
			if first[1] != "5" || first[2] != "0" {
				t.Fatalf("page 1 %v, want 5 rows and no fillers", first)
			}
			next := a.Find(".kisland-table-pager > .kisland-table-page:nth-last-child(2)")
			if c.total == "undefined" {
				next = a.Find(".kisland-table-pager > .kisland-table-page:last-child")
			}
			next.Click()
			status.WaitText("Showing 6–10" + c.suffix)
			next.Click()
			status.WaitText("Showing 11–12" + c.suffix)
			expectEval(t, a, fillerJS, first[0]+" 2 3 true", "last page keeps the height with 3 plain hidden fillers")
		})
	}
}

func TestTableSinglePageHasNoFillers(t *testing.T) {
	a := layoutApp(t, 1280, `ui.column({}, ui.table({columns: [{title: "Name"}], selectable: true, pageSize: 5, rows: `+tableRows(3)+`}), ui.table({columns: [{title: "Entry"}], pageSize: 5, query: String(function (p) {
		return {rows: [{id: "a", cells: ["a"]}, {id: "b", cells: ["b"]}]}
	})}))`)
	a.Find(".kisland-table:nth-of-type(1) .kisland-table-status-text").WaitText("Showing 1–3 of 3")
	expectEval(t, a, `() => Array.from(document.querySelectorAll(".kisland-table-status-text")).map((s) => s.textContent).join("|") + " " + document.querySelectorAll(".kisland-table-filler").length`, "Showing 1–3 of 3|Showing 1–2 0", "single-page tables have no fillers")
}
