//go:build browser

package stack

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"neui.dev"
	"neui.dev/internal/devkit"
)

// ChoiceCreateError sets widget, a script expression, as the island root and
// returns the error message its creation fails with, "" when it shows.
func ChoiceCreateError(t *testing.T, island neui.Island, widget string) string {
	t.Helper()
	a := Island(t, devkit.App{Island: island, Script: `var out
		try {
			ui.root.set(` + widget + `)
			ui.draw(1)
			out = {created: true}
		} catch (e) {
			out = {error: String(e && e.message || e)}
		}
		ui.notify(out)`})
	var got struct {
		Created bool
		Error   string
	}
	reports := a.Until(func(r devkit.Report) bool { return r.Kind != "ok" })
	data := reports[len(reports)-1].Data
	var list []json.RawMessage
	if err := json.Unmarshal(data, &list); err != nil || len(list) != 1 || json.Unmarshal(list[0], &got) != nil {
		t.Fatalf("got %s, want one notification", data)
	}
	if !got.Created && got.Error == "" {
		t.Fatalf("got %s, want created or an error", data)
	}
	return got.Error
}

// ChoiceReport waits for the next report that is a notification of kind,
// such as "widget_error", skipping the ones in between, and returns its data.
func (a *IslandApp) ChoiceReport(kind string) json.RawMessage {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool { return r.Kind == "notify" && r.Meta["kind"] == kind })
	return reports[len(reports)-1].Data
}

func (l Locator) choiceEval(fn string) (string, error) {
	res, err := l.a.Page.Evaluate(rod.Eval(`function (path) { return (`+fn+`)((`+widgetJS+`)(path, "element", null)) }`, l.path))
	if err != nil {
		return "", err
	}
	return res.Value.Str(), nil
}

func (l Locator) choiceExpect(fn string, want string, what string) {
	l.a.t.Helper()
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		got, err := l.choiceEval(fn)
		if err != nil {
			return false, evalErr(err)
		}
		return got == want, fmt.Sprintf("%q", got)
	}, fmt.Sprintf("%s: %s, want %q", l.desc, what, want))
}

func (l Locator) choiceDo(fn string, what string) {
	l.a.t.Helper()
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		if _, err := l.choiceEval(fn); err != nil {
			return false, evalErr(err)
		}
		return true, ""
	}, fmt.Sprintf("%s: %s", l.desc, what))
}

const choiceNormJS = `const norm = (s) => s.replace(/\s+/g, " ").trim();`

// ExpectLook waits until a select shows as "radio", "dropdown" or
// "checkbox".
func (f Field) ExpectLook(want string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => el.querySelector("select") ? "dropdown" : el.querySelector("input.neisland-form-radio") ? "radio" : el.querySelector("input.neisland-form-checkbox") ? "checkbox" : "none"`, want, "look")
}

// ExpectOptions waits until a select offers exactly these option texts in
// order, including "None" and "Choose…".
func (f Field) ExpectOptions(want ...string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => { `+choiceNormJS+`
		const s = el.querySelector("select")
		const all = s ? Array.from(s.options).map((o) => norm(o.textContent)) : Array.from(el.querySelectorAll("label.neisland-form-check")).map((l) => norm(l.textContent))
		return JSON.stringify(all)
	}`, choiceJSON(append([]string{}, want...)), "options")
}

// ExpectChips waits until a search shows exactly these chosen labels in
// order.
func (f Field) ExpectChips(want ...string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => { `+choiceNormJS+`
		return JSON.stringify(Array.from(el.querySelectorAll(".neisland-form-search-chip")).map((c) => { const n = c.cloneNode(true); n.querySelectorAll("button").forEach((b) => b.remove()); return norm(n.textContent) }))
	}`, choiceJSON(append([]string{}, want...)), "chips")
}

// ExpectResults waits until a search offers exactly these result labels in
// order.
func (f Field) ExpectResults(want ...string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => { `+choiceNormJS+`
		return JSON.stringify(Array.from(el.querySelectorAll(".neisland-form-search-result")).map((b) => norm(b.textContent)))
	}`, choiceJSON(append([]string{}, want...)), "results")
}

// ExpectSearchStatus waits until a search's status line reads want.
func (f Field) ExpectSearchStatus(want string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => { `+choiceNormJS+` return norm(el.querySelector(".neisland-form-search-status").textContent) }`, want, "status")
}

// ExpectSearchInput waits until a search shows its input, or hides it.
func (f Field) ExpectSearchInput(shown bool) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => String(!!el.querySelector("input.neisland-form-search-input"))`, fmt.Sprint(shown), "search input shown")
}

// ExpectMaxLine waits until a search's max line reads want; "" expects none.
func (f Field) ExpectMaxLine(want string) {
	f.a.t.Helper()
	f.choiceExpect(`(el) => { `+choiceNormJS+` const m = el.querySelector(".neisland-form-search-max"); return m ? norm(m.textContent) : "" }`, want, "max line")
}

// SearchFor types q into a search's input and fires input.
func (f Field) SearchFor(q string) {
	f.a.t.Helper()
	f.choiceDo(`(el) => {
		const c = el.querySelector("input.neisland-form-search-input")
		if (!c) {
			throw new Error("no search input")
		}
		c.focus()
		c.value = `+choiceJSON(q)+`
		c.dispatchEvent(new Event("input", {bubbles: true}))
	}`, fmt.Sprintf("search %q", q))
}

// PickResult presses the search result labeled label.
func (f Field) PickResult(label string) {
	f.a.t.Helper()
	f.Find(`button[aria-label=` + choiceJSON("Choose "+label) + `]`).Click()
}

// DropChip presses the remove button of the chip labeled label.
func (f Field) DropChip(label string) {
	f.a.t.Helper()
	f.Find(`button[aria-label=` + choiceJSON("Remove "+label) + `]`).Click()
}

func choiceJSON(v any) string {
	b, _ := json.Marshal(v)
	return strings.ReplaceAll(string(b), `&`, "&")
}
