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
	if f := strings.Fields(before); f[0] != "208" || f[1] != "24" || f[2] != "12" || f[3] != "12" || f[5] != "false" {
		t.Fatalf("footer %q, want a 208 by 24 status at the 12px inset on the left and the pager at the right one on one line", before)
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
	if f := strings.Fields(got); f[0] != "208" || f[1] != "24" || f[2] != "12" || f[4] != "12" || f[5] != "true" || f[6] != "true" {
		t.Fatalf("footer %q, want the 208 by 24 status first and the pager wrapped below it at the 12px inset on the left, no page scroll", got)
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
	a.Find(".kisland-table-card:nth-of-type(1) .kisland-table-status-text").WaitText("Showing 1–3 of 3")
	expectEval(t, a, `() => Array.from(document.querySelectorAll(".kisland-table-status-text")).map((s) => s.textContent).join("|") + " " + document.querySelectorAll(".kisland-table-filler").length`, "Showing 1–3 of 3|Showing 1–2 0", "single-page tables have no fillers")
}

const narrowCellsJS = `() => {
	const lines = (n) => Math.round(n.getBoundingClientRect().height / parseFloat(getComputedStyle(n).lineHeight))
	const cell = (s) => Array.from(document.querySelectorAll(".kisland-table-prose")).find((n) => n.textContent === s)
	const one = ["INV-20418", "$1,412.00", "2026-11-12"].every((s) => lines(cell(s)) === 1)
	const long = cell("Quarterly freight for the north region, including two late deliveries that were credited back on the next statement")
	const max = parseFloat(getComputedStyle(long).maxWidth)
	const heads = Array.from(document.querySelectorAll(".kisland-table-head-cell")).filter((h) => h.textContent.trim() !== "")
	const head = heads.every((h) => h.getBoundingClientRect().height === heads[0].getBoundingClientRect().height)
	const buttons = Array.from(document.querySelector(".kisland-table-row .kisland-table-actions").children).map((b) => b.getBoundingClientRect().top)
	return [one, lines(long) > 1, long.getBoundingClientRect().width <= max, head, buttons.length === 2 && buttons[0] === buttons[1], document.documentElement.scrollWidth <= window.innerWidth].join(" ")
}`

func TestTableNarrowKeepsShortValuesOnOneLine(t *testing.T) {
	for _, width := range []int{570, 390} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			a := layoutApp(t, width, `ui.table({columns: [{title: "Invoice"}, {title: "Amount"}, {title: "Due date"}, {title: "Description"}], selectable: true, rows: [
				{id: "a", cells: ["INV-20418", "$1,412.00", "2026-11-12", "Quarterly freight for the north region, including two late deliveries that were credited back on the next statement"], actions: [{value: "view", text: "View"}, {value: "pay", text: "Mark paid", style: "primary"}]},
				{id: "b", cells: ["INV-20419", "$412.00", "2026-11-13", "Spare parts"], actions: [{value: "view", text: "View"}, {value: "pay", text: "Mark paid", style: "primary"}]}
			]})`)
			expectEval(t, a, narrowCellsJS, "true true true true true true", "short values on one line, long text wrapped within the column max, headers and actions on one line, no page scroll")
		})
	}
}

const cardJS = `() => {
	const card = document.querySelector(".kisland-table-card")
	const box = card.querySelector(".kisland-table-card-box")
	const head = card.querySelector(".kisland-table-card-head")
	const search = card.querySelector(".kisland-table-search")
	const foot = card.querySelector(".kisland-table-footer")
	const rows = Array.from(card.querySelectorAll("tbody > .kisland-table-row"))
	const style = (n) => getComputedStyle(n)
	const r = (n) => n.getBoundingClientRect()
	const borders = (n) => ["Top", "Right", "Bottom", "Left"].map((s) => style(n)["border" + s + "Width"]).join(",")
	const c = r(card)
	const first = rows[0].cells[0].querySelector(".kisland-table-prose")
	const last = rows[0].cells[rows[0].cells.length - 1].querySelector(".kisland-table-prose")
	const th = card.querySelectorAll(".kisland-table-head-cell")
	const checks = {
		card: borders(card) === "1px,1px,1px,1px",
		box: borders(box) === "0px,0px,0px,0px" && style(box).borderTopLeftRadius === "0px" && r(box).left === c.left + 1 && r(box).right === c.right - 1,
		search: head === card.firstElementChild && head.contains(search) && r(search).left === c.left + 13 && r(search).top === c.top + 13 && r(box).top >= r(search).bottom,
		head: borders(head) === "0px,0px,0px,0px" && style(head).backgroundColor === "rgba(0, 0, 0, 0)",
		foot: borders(foot) === "1px,0px,0px,0px" && style(foot).backgroundColor !== "rgba(0, 0, 0, 0)" && r(foot).left === c.left + 1 && r(foot).right === c.right - 1 && r(foot).bottom === c.bottom - 1 && r(foot).top >= r(box).bottom,
		status: r(card.querySelector(".kisland-table-status")).left === c.left + 13,
		lastRow: Array.from(rows[rows.length - 1].cells).every((d) => style(d).borderBottomWidth === "0px") && Array.from(rows[0].cells).every((d) => style(d).borderBottomWidth === "1px"),
		inset: r(first).left === c.left + 13 && r(last).right === c.right - 13 && style(th[0]).paddingLeft === "12px" && style(th[th.length - 1]).paddingRight === "12px" && r(th[0]).left === c.left + 1,
	}
	return Object.keys(checks).filter((k) => !checks[k]).join(" ")
}`

func TestTableRendersAsOneCard(t *testing.T) {
	a := layoutApp(t, 1280, `ui.table({columns: [{title: "Name"}, {title: "Note"}], search: true, rows: [
		{id: "a", cells: ["Ann", "one"]}, {id: "b", cells: ["Bob", "two"]}, {id: "c", cells: ["Cid", "three"]}
	]})`)
	a.Find(".kisland-table-status-text").WaitText("Showing 1–3 of 3")
	expectEval(t, a, cardJS, "", "one bordered card: search inside on top, borderless box edge to edge, tinted footer band with the hairline at the bottom, no rule under the last row, text at the 12px inset")
}

func TestTableCardCheckColumnKeepsPadding(t *testing.T) {
	a := layoutApp(t, 1280, `ui.table({columns: [{title: "Name"}], selectable: true, rows: [{id: "a", cells: ["Ann"]}, {id: "b", cells: ["Bob"]}]})`)
	a.Find(".kisland-table-status-text").WaitText("Showing 1–2 of 2")
	expectEval(t, a, `() => {
		const card = document.querySelector(".kisland-table-card")
		const box = card.querySelector(".kisland-table-card-box")
		const cell = (s) => getComputedStyle(card.querySelector(s))
		return [card.querySelector(".kisland-table-card-head") === null, getComputedStyle(box).borderTopLeftRadius, cell("tbody .kisland-table-check").paddingLeft, cell("tbody .kisland-table-cell:last-child").paddingRight].join(" ")
	}`, "true 5px 6px 12px", "no search head so the box takes the card's top corners, the check column keeps its 6px padding and the last column takes the 12px inset")
}
