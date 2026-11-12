//go:build browser

package devkit_test

import (
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func layoutApp(t *testing.T, width int, root string) *stack.IslandApp {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(` + root + `); ui.draw(1); ui.notify("drawn"); while (true) ui.draw()`})
	a.Page.MustSetViewport(width, 800, 1, false)
	a.Notified()
	return a
}

func expectEval(t *testing.T, a *stack.IslandApp, js string, want string, what string) {
	t.Helper()
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		got := a.Page.MustEval(js).String()
		return got == want, got
	}, what)
}

const overflowJS = `() => {
	const out = Array.from(document.querySelectorAll(".neisland-button")).filter((b) => {
		let parent = b.parentElement
		while (getComputedStyle(parent).display === "contents") {
			parent = parent.parentElement
		}
		const r = b.getBoundingClientRect()
		const p = parent.getBoundingClientRect()
		return r.left < p.left - 0.5 || r.right > p.right + 0.5
	})
	return [document.documentElement.scrollWidth <= window.innerWidth, document.querySelectorAll(".neisland-button").length, out.length].join(" ")
}`

func TestLayoutLongButtonWrapsOnNarrowPage(t *testing.T) {
	a := layoutApp(t, 390, `ui.column({},
		ui.button({text: "A very long button label that keeps going to check how it wraps at narrow width"}),
		ui.row({}, ui.button({text: "short"}), ui.button({text: "Another long label inside a row that has to wrap once the page gets narrow", style: "primary"})),
		ui.grid({item: "sm"}, ui.button({text: "A long label inside a grid cell that cannot fit on one line"}), ui.button({text: "ok"}))
	)`)
	expectEval(t, a, overflowJS, "true 5 0", "no page scroll and every button inside its parent")
	expectEval(t, a, `() => String(document.querySelector(".neisland-row .neisland-button-primary").getBoundingClientRect().height > 30)`, "true", "the long label wraps onto more lines")
	expectEval(t, a, `() => String(Math.round(document.querySelector(".neisland-row .neisland-button").getBoundingClientRect().height))`, "28", "a short label keeps one line")
}

func TestLayoutButtonHeightsMatchBoard(t *testing.T) {
	a := layoutApp(t, 1280, `ui.row({}, ui.button({text: "Save", size: "sm"}), ui.button({text: "Save"}), ui.button({text: "Save", size: "lg"}))`)
	expectEval(t, a, `() => Array.from(document.querySelectorAll(".neisland-button")).map((b) => Math.round(b.getBoundingClientRect().height)).join(" ")`, "24 28 36", "sm, md and lg buttons are 24, 28 and 36px tall")
}

const fillJS = `() => Array.from(document.querySelectorAll(".neisland-row")).map((row) => {
	const width = row.getBoundingClientRect().width
	return Array.from(row.querySelectorAll(":scope > .neisland-column")).map((c) => {
		const w = c.getBoundingClientRect().width
		return w + 0.5 >= Math.min(width, 192) ? "ok" : "crushed " + Math.round(w)
	}).join(",")
}).join(" | ")`

func TestLayoutFillColumnsWrapOnNarrowPage(t *testing.T) {
	a := layoutApp(t, 390, `ui.column({},
		ui.row({}, ui.column({fill: 1}, ui.text({text: "fill 1"})), ui.column({fill: 2}, ui.text({text: "fill 2"})), ui.column({fill: 3}, ui.text({text: "fill 3"})), ui.button({text: "fixed"})),
		ui.row({}, ui.column({fill: 6}, ui.text({text: "fill 6"})), ui.column({fill: 1}, ui.text({text: "fill 1"}))),
		ui.row({}, ui.column({fill: 1}, ui.form({fields: [{type: "date", label: "When", name: "when"}]})), ui.column({fill: 2}, ui.form({fields: [{type: "text", label: "Note", name: "note"}]})))
	)`)
	expectEval(t, a, fillJS, "ok,ok,ok | ok,ok | ok,ok", "fill columns at least 12rem wide or wrapped")
	expectEval(t, a, `() => String(document.documentElement.scrollWidth <= window.innerWidth)`, "true", "no page scroll")
}

func TestLayoutFillColumnsKeepWeightsWithRoom(t *testing.T) {
	a := layoutApp(t, 1280, `ui.row({}, ui.column({fill: 1}, ui.text({text: "fill 1"})), ui.column({fill: 2}, ui.text({text: "fill 2"})))`)
	expectEval(t, a, `() => {
		const [one, two] = Array.from(document.querySelectorAll(".neisland-row > .neisland-column")).map((c) => c.getBoundingClientRect())
		return String(Math.abs(two.width - 2 * one.width) < 1 && one.top === two.top)
	}`, "true", "fill 2 twice as wide as fill 1 on one line")
}

const formLayoutSpec = `ui.form({fields: [
	{type: "text", label: "Name", name: "name"},
	{type: "textarea", label: "Notes", name: "notes"},
	{type: "group", label: "Address", name: "address", fields: [
		{type: "text", label: "City", name: "city"},
		{type: "list", value: ["1"], field: {type: "text", label: "Phones", name: "phones"}}
	]},
	{type: "list", value: ["a", "b"], field: {type: "text", label: "Tags", name: "tags"}},
	{type: "series", label: "Pick", name: "pick", fields: ["(p) => ({type: 'text', label: 'Kind', name: 'kind'})", "(p) => ({type: 'text', label: 'Size', name: 'size'})"]}
]})`

const formControlsJS = `() => {
	const form = document.querySelector(".neisland-form")
	const s = getComputedStyle(form)
	const box = form.getBoundingClientRect()
	const right = box.right - parseFloat(s.paddingRight) - parseFloat(s.borderRightWidth)
	const width = box.width - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight) - parseFloat(s.borderLeftWidth) - parseFloat(s.borderRightWidth)
	return Array.from(form.querySelectorAll(".neisland-form-input, .neisland-form-textarea")).map((c) => {
		let depth = 0
		for (let n = c.parentElement; n !== form; n = n.parentElement) {
			if (n.matches(".neisland-form-rail, .neisland-form-item, .neisland-form-step")) {
				depth++
			}
		}
		const r = c.getBoundingClientRect()
		return depth + ":" + Math.round(width - r.width) + (Math.abs(r.right - right) < 0.5 ? "" : ":off")
	}).join(" ")
}`

const formRowsJS = `() => {
	const rows = Array.from(document.querySelector(".neisland-form").children).filter((c) => c.matches(".neisland-form-field, .neisland-form-box")).map((c) => c.getBoundingClientRect())
	return String(rows.length) + " " + rows.every((r, i) => r.left === rows[0].left && (i === 0 || r.top >= rows[i - 1].bottom))
}`

func TestLayoutFormControlsShareOneWidth(t *testing.T) {
	a := layoutApp(t, 1280, formLayoutSpec)
	expectEval(t, a, formControlsJS, "0:0 0:0 1:28 2:56 1:28 1:28 1:28", "every control the form width less 28px per level, right edges aligned")
	expectEval(t, a, `() => {
		const form = document.querySelector(".neisland-form")
		let column = form.parentElement
		while (getComputedStyle(column).display === "contents") {
			column = column.parentElement
		}
		return String(Math.abs(form.getBoundingClientRect().width - column.getBoundingClientRect().width) < 0.5)
	}`, "true", "the form fills its column")
	expectEval(t, a, formRowsJS, "5 true", "one field per row")
	expectEval(t, a, `() => {
		const form = document.querySelector(".neisland-form").getBoundingClientRect()
		const band = document.querySelector(".neisland-form-actions-band").getBoundingClientRect()
		return String(band.left >= form.left && band.right <= form.right && band.bottom <= form.bottom)
	}`, "true", "the footer band inside the card")
}

func TestLayoutFormIndentFloorsOnNarrowPage(t *testing.T) {
	a := layoutApp(t, 220, formLayoutSpec)
	expectEval(t, a, `() => Array.from(document.querySelectorAll(".neisland-form-rail, .neisland-form-item, .neisland-form-step")).map((n) => {
		const min = n.matches(".neisland-form-item") ? 24 : 16
		const c = n.parentElement.closest(".neisland-form-box, .neisland-form-items, .neisland-form-steps, .neisland-form")
		const cs = getComputedStyle(c)
		const width = c.getBoundingClientRect().width - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) - parseFloat(cs.borderLeftWidth) - parseFloat(cs.borderRightWidth)
		const want = Math.min(28, Math.max(min, width - 192))
		const got = parseFloat(getComputedStyle(n).paddingLeft)
		return Math.abs(got - want) < 0.5 && got === min ? "ok" : "off " + got + " want " + want
	}).join(" ")`, "ok ok ok ok ok ok", "the indent floors at 1rem, 1.5rem for list items")
	expectEval(t, a, `() => {
		const form = document.querySelector(".neisland-form")
		const s = getComputedStyle(form)
		const right = form.getBoundingClientRect().right - parseFloat(s.paddingRight) - parseFloat(s.borderRightWidth)
		return String(Array.from(form.querySelectorAll(".neisland-form-input, .neisland-form-textarea")).every((c) => Math.abs(c.getBoundingClientRect().right - right) < 0.5) && document.documentElement.scrollWidth <= window.innerWidth)
	}`, "true", "right edges aligned with no page scroll")
}
