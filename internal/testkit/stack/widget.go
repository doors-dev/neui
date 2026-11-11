//go:build browser

package stack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"neui.dev/internal/devkit"
)

// WaitTimeout is how long the widget assertions poll before they fail.
var WaitTimeout = 5 * time.Second

// Eventually polls cond until it reports ok, failing t with what and the
// last observed value once timeout passes.
func Eventually(t testing.TB, timeout time.Duration, cond func() (ok bool, observed string), what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ok, observed := cond()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: still not so after %v; last observed: %s", what, timeout, observed)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ExpectJSON fails t unless got and want hold the same JSON value.
func ExpectJSON(t testing.TB, got json.RawMessage, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got invalid JSON %s: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want invalid JSON %s: %v", want, err)
	}
	gn, _ := json.Marshal(g)
	wn, _ := json.Marshal(w)
	if !bytes.Equal(gn, wn) {
		t.Fatalf("got JSON\n\t%s\nwant\n\t%s", gn, wn)
	}
}

// Submitted waits for the next ui.notify report, as a script running
// ui.notify(ui.draw()) sends it, and returns the one notified value, such as
// a form's {"data": ..., "submit": ...}.
func (a *IslandApp) Submitted() json.RawMessage {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool { return r.Kind != "ok" })
	r := reports[len(reports)-1]
	if r.Kind != "notify" || r.Meta["kind"] != "notifications_list" {
		a.t.Fatalf("got %v, want a notifications_list", r)
	}
	var list []struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(r.Data, &list); err != nil {
		a.t.Fatalf("notification %s: %v", r.Data, err)
	}
	if len(list) != 1 {
		a.t.Fatalf("got %d notifications %s, want 1", len(list), r.Data)
	}
	return list[0].Data
}

// Find locates the first element matching selector on the page.
func (a *IslandApp) Find(selector string) Locator {
	return Locator{a: a}.Find(selector)
}

// Form locates the first form widget on the page.
func (a *IslandApp) Form() Form {
	return a.FormAt(".kisland-form")
}

// FormAt locates the form widget matching selector.
func (a *IslandApp) FormAt(selector string) Form {
	return Form{a.Find(selector)}
}

// Locator is a path to an element, resolved anew on every action, so it
// survives the DOM updates in between.
type Locator struct {
	a    *IslandApp
	path []step
	desc string
}

type step struct {
	Kind string `json:"kind"`
	Arg  any    `json:"arg"`
}

func (l Locator) with(kind string, arg any, desc string) Locator {
	path := append(append([]step(nil), l.path...), step{Kind: kind, Arg: arg})
	if l.desc != "" {
		desc = l.desc + " > " + desc
	}
	return Locator{a: l.a, path: path, desc: desc}
}

func (l Locator) eval(op string, arg any) (string, error) {
	res, err := l.a.Page.Evaluate(rod.Eval(widgetJS, l.path, op, arg))
	if err != nil {
		return "", err
	}
	return res.Value.Str(), nil
}

func (l Locator) expect(op string, arg any, want string, what string) {
	l.a.t.Helper()
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		got, err := l.eval(op, arg)
		if err != nil {
			return false, evalErr(err)
		}
		return got == want, strconv.Quote(got)
	}, fmt.Sprintf("%s: %s, want %q", l.desc, what, want))
}

func (l Locator) do(op string, arg any, what string) {
	l.a.t.Helper()
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		if _, err := l.eval(op, arg); err != nil {
			return false, evalErr(err)
		}
		return true, ""
	}, fmt.Sprintf("%s: %s", l.desc, what))
}

func (l Locator) read(op string, arg any, what string) string {
	l.a.t.Helper()
	var got string
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		v, err := l.eval(op, arg)
		if err != nil {
			return false, evalErr(err)
		}
		got = v
		return true, ""
	}, fmt.Sprintf("%s: %s", l.desc, what))
	return got
}

func evalErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "Error: "); i != -1 {
		msg = msg[i+len("Error: "):]
	}
	if i := strings.Index(msg, "\n"); i != -1 {
		msg = msg[:i]
	}
	return msg
}

// Find locates the first element matching selector within l.
func (l Locator) Find(selector string) Locator {
	return l.with("css", selector, selector)
}

// Element returns the element l resolves to now, waiting until it exists.
func (l Locator) Element() *rod.Element {
	l.a.t.Helper()
	var el *rod.Element
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		e, err := l.a.Page.Sleeper(rod.NotFoundSleeper).ElementByJS(rod.Eval(widgetJS, l.path, "element", nil))
		if err != nil {
			return false, evalErr(err)
		}
		el = e
		return true, ""
	}, l.desc+": exists")
	return el
}

// Click clicks the element.
func (l Locator) Click() {
	l.a.t.Helper()
	l.do("click", nil, "click")
}

// WaitText waits until the element's whitespace-collapsed text is want.
func (l Locator) WaitText(want string) {
	l.a.t.Helper()
	l.expect("text", nil, want, "text")
}

// WaitAttr waits until the element's attribute name is want.
func (l Locator) WaitAttr(name, want string) {
	l.a.t.Helper()
	l.expect("attr", name, want, "attribute "+name)
}

// WaitNoAttr waits until the element has no attribute name.
func (l Locator) WaitNoAttr(name string) {
	l.a.t.Helper()
	l.expect("hasattr", name, "false", "has attribute "+name)
}

// WaitCount waits until n elements within the element match selector.
func (l Locator) WaitCount(selector string, n int) {
	l.a.t.Helper()
	l.expect("count", selector, strconv.Itoa(n), "count of "+selector)
}

// WaitGone waits until no element within the element matches selector.
func (l Locator) WaitGone(selector string) {
	l.a.t.Helper()
	l.WaitCount(selector, 0)
}

// Field locates the first field within l whose label reads label, without
// its required mark: a label for a control, a legend of a radio or checkbox
// group, the text beside a single checkbox, or a file field's label.
func (l Locator) Field(label string) Field {
	return Field{l.with("field", label, fmt.Sprintf("field %q", label))}
}

// Group locates the first group within l with legend label.
func (l Locator) Group(label string) Locator {
	return l.with("group", label, fmt.Sprintf("group %q", label))
}

// List locates the first list within l with legend label.
func (l Locator) List(label string) List {
	return List{l.with("list", label, fmt.Sprintf("list %q", label))}
}

// Series locates the first series within l with legend label.
func (l Locator) Series(label string) Series {
	return Series{l.with("series", label, fmt.Sprintf("series %q", label))}
}

// ExpectError waits until the field, list or series shows the error text.
func (l Locator) ExpectError(text string) {
	l.a.t.Helper()
	l.expect("error", nil, text, "error")
}

// ExpectNoError waits until the field, list or series shows no error.
func (l Locator) ExpectNoError() {
	l.a.t.Helper()
	l.ExpectError("")
}

// ExpectRequiredMark waits until the label shows the required "*", or
// stops showing it.
func (l Locator) ExpectRequiredMark(shown bool) {
	l.a.t.Helper()
	l.expect("required", nil, strconv.FormatBool(shown), "required mark shown")
}

// Field is a form field found by its label.
type Field struct {
	Locator
}

// Type replaces the value of a text-like, number, date or textarea control
// with s and fires input, as typing does.
func (f Field) Type(s string) {
	f.a.t.Helper()
	f.do("type", s, fmt.Sprintf("type %q", s))
}

// Value is the shown value: the control's value, "true" or "false" for a
// single checkbox, the chosen option labels joined by ", " for a select,
// "" when none, and the summary line for a file field.
func (f Field) Value() string {
	f.a.t.Helper()
	return f.read("value", nil, "value")
}

// ExpectValue waits until Value is want.
func (f Field) ExpectValue(want string) {
	f.a.t.Helper()
	f.expect("value", nil, want, "value")
}

// Check checks a single checkbox, clicking it unless checked.
func (f Field) Check() {
	f.a.t.Helper()
	f.do("check", true, "check")
}

// Uncheck unchecks a single checkbox, clicking it if checked.
func (f Field) Uncheck() {
	f.a.t.Helper()
	f.do("check", false, "uncheck")
}

// Choose picks the option labeled option in a radio group, a dropdown or a
// checkbox group; "None" picks a radio group's empty choice.
func (f Field) Choose(option string) {
	f.a.t.Helper()
	f.do("choose", option, fmt.Sprintf("choose %q", option))
}

// Unchoose clears the option labeled option in a checkbox group.
func (f Field) Unchoose(option string) {
	f.a.t.Helper()
	f.do("unchoose", option, fmt.Sprintf("unchoose %q", option))
}

// SetFiles picks the files at paths in a file field.
func (f Field) SetFiles(paths ...string) {
	f.a.t.Helper()
	f.Find(`input[type="file"]`).Element().MustSetFiles(paths...)
}

// ExpectInvalid waits until the control's aria-invalid is true, or unset.
func (f Field) ExpectInvalid(invalid bool) {
	f.a.t.Helper()
	f.expect("invalid", nil, strconv.FormatBool(invalid), "aria-invalid")
}

// ExpectDisabled waits until the control is disabled, or enabled.
func (f Field) ExpectDisabled(disabled bool) {
	f.a.t.Helper()
	f.expect("disabled", nil, strconv.FormatBool(disabled), "disabled")
}

// List is a list field found by its legend.
type List struct {
	Locator
}

// Items is how many items the list shows now.
func (l List) Items() int {
	l.a.t.Helper()
	n, _ := strconv.Atoi(l.read("items", nil, "items"))
	return n
}

// ExpectCount waits until the list shows n items.
func (l List) ExpectCount(n int) {
	l.a.t.Helper()
	l.expect("items", nil, strconv.Itoa(n), "item count")
}

// Add presses the list's Add button.
func (l List) Add() {
	l.a.t.Helper()
	l.do("add", nil, "add")
}

// Remove presses the remove button of item i, counted from 0.
func (l List) Remove(i int) {
	l.a.t.Helper()
	l.do("remove", i, fmt.Sprintf("remove item %d", i))
}

// ExpectAddDisabled waits until the Add button is aria-disabled, or not.
func (l List) ExpectAddDisabled(disabled bool) {
	l.a.t.Helper()
	l.expect("adddisabled", nil, strconv.FormatBool(disabled), "Add disabled")
}

// Item scopes item i, counted from 0, to find its fields.
func (l List) Item(i int) Locator {
	return l.with("item", i, fmt.Sprintf("item %d", i))
}

// Series is a series field found by its legend.
type Series struct {
	Locator
}

// ExpectSteps waits until the series shows n steps.
func (s Series) ExpectSteps(n int) {
	s.a.t.Helper()
	s.expect("steps", nil, strconv.Itoa(n), "step count")
}

// ExpectCompleted waits until the series shows it is completed, or not.
func (s Series) ExpectCompleted(completed bool) {
	s.a.t.Helper()
	s.expect("completed", nil, strconv.FormatBool(completed), "completed")
}

// Step scopes step i, counted from 0, to find its field.
func (s Series) Step(i int) Locator {
	return s.with("step", i, fmt.Sprintf("step %d", i))
}

// Form is a form widget.
type Form struct {
	Locator
}

// Submit waits until the submit button labeled text is enabled and
// presses it.
func (f Form) Submit(text string) {
	f.a.t.Helper()
	f.do("submit", text, fmt.Sprintf("press enabled submit %q", text))
}

// Cancel presses the cancel button.
func (f Form) Cancel() {
	f.a.t.Helper()
	f.do("cancel", nil, "press cancel")
}

// ExpectSubmitDisabled waits until every submit button is disabled, or
// every one is enabled.
func (f Form) ExpectSubmitDisabled(disabled bool) {
	f.a.t.Helper()
	f.expect("submitdisabled", nil, strconv.FormatBool(disabled), "submit buttons disabled")
}

// ExpectSubmitMessage waits until the line by the submit buttons reads
// text; "" expects none.
func (f Form) ExpectSubmitMessage(text string) {
	f.a.t.Helper()
	f.expect("submitmessage", nil, text, "submit message")
}

const widgetJS = `function (path, op, arg) {
	const norm = (s) => s.replace(/\s+/g, " ").trim()
	const text = (el) => {
		const c = el.cloneNode(true)
		c.querySelectorAll(".kisland-form-required, .kisland-form-count, script").forEach((n) => n.remove())
		return norm(c.textContent)
	}
	const box = (el) => el.matches("fieldset.kisland-form-box")
	const labelOf = (el) => box(el)
		? el.querySelector(":scope > legend")
		: el.querySelector(":scope > .kisland-form-label, :scope > label.kisland-form-check > span")
	const find = (scope, sel, kind, label) => {
		const all = Array.from(scope.querySelectorAll(sel))
		const hit = all.find((n) => labelOf(n) && text(labelOf(n)) === label)
		if (!hit) {
			throw new Error("no " + kind + " labeled " + JSON.stringify(label) + "; there are " + JSON.stringify(all.map((n) => labelOf(n) ? text(labelOf(n)) : "")))
		}
		return hit
	}
	const boxes = (scope, inner) => Array.from(scope.querySelectorAll("fieldset.kisland-form-box")).filter((n) => n.querySelector(":scope > " + inner))
	const findBox = (scope, inner, kind, label) => {
		const all = boxes(scope, inner)
		const hit = all.find((n) => text(labelOf(n)) === label)
		if (!hit) {
			throw new Error("no " + kind + " labeled " + JSON.stringify(label) + "; there are " + JSON.stringify(all.map((n) => text(labelOf(n)))))
		}
		return hit
	}
	const items = (list) => {
		const ol = list.querySelector("ol.kisland-form-items")
		return Array.from(ol.querySelectorAll("li.kisland-form-item")).filter((n) => n.closest("ol.kisland-form-items") === ol)
	}
	const steps = (series) => {
		const ol = series.querySelector("ol.kisland-form-steps")
		return Array.from(ol.querySelectorAll("li.kisland-form-step")).filter((n) => n.closest("ol.kisland-form-steps") === ol)
	}
	const nth = (all, i, kind) => {
		if (i >= all.length) {
			throw new Error("no " + kind + " " + i + "; there are " + all.length)
		}
		return all[i]
	}
	const resolve = (scope, s) => {
		switch (s.kind) {
		case "css": {
			const el = scope.querySelector(s.arg)
			if (!el) {
				throw new Error("nothing matches " + s.arg)
			}
			return el
		}
		case "field":
			return find(scope, ".kisland-form-field, fieldset.kisland-form-group", "field", s.arg)
		case "group":
			return findBox(scope, ".kisland-form-rail", "group", s.arg)
		case "list":
			return findBox(scope, "ol.kisland-form-items", "list", s.arg)
		case "series":
			return findBox(scope, "ol.kisland-form-steps", "series", s.arg)
		case "item":
			return nth(items(scope), s.arg, "item")
		case "step":
			return nth(steps(scope).filter((n) => !n.hasAttribute("data-complete")), s.arg, "step")
		}
		throw new Error("unknown step " + s.kind)
	}
	const el = path.reduce(resolve, document)
	const control = () => el.matches("fieldset") ? el.querySelector(".kisland-form-options") : el.querySelector("input, select, textarea")
	const options = () => Array.from(el.querySelectorAll("label.kisland-form-check")).map((l) => ({label: text(l), input: l.querySelector("input")}))
	const option = (label) => {
		const o = options().find((o) => o.label === label)
		if (!o) {
			throw new Error("no option " + JSON.stringify(label) + "; there are " + JSON.stringify(options().map((o) => o.label)))
		}
		return o.input
	}
	const actions = () => el.querySelector(".kisland-form-actions")
	const submits = () => Array.from(actions().querySelectorAll("button[type=submit]"))
	const fire = (c, ...types) => types.forEach((t) => c.dispatchEvent(new Event(t, {bubbles: true})))
	switch (op) {
	case "element":
		return el
	case "click":
		el.click()
		return ""
	case "text":
		return norm(el.textContent)
	case "attr":
		return el.getAttribute(arg) ?? ""
	case "hasattr":
		return String(el.hasAttribute(arg))
	case "count":
		return String(el.querySelectorAll(arg).length)
	case "error": {
		const owner = box(el) ? el : control()
		const ids = (owner.getAttribute("aria-describedby") || "").split(" ").filter(Boolean)
		const err = ids.map((id) => document.getElementById(id)).find((n) => n && n.classList.contains("kisland-form-error"))
		return err ? norm(err.textContent) : ""
	}
	case "required":
		return String(!!labelOf(el)?.querySelector(".kisland-form-required"))
	case "invalid":
		return String(control().getAttribute("aria-invalid") === "true")
	case "disabled": {
		const c = control()
		return String(!!c.disabled || c.getAttribute("aria-disabled") === "true")
	}
	case "value": {
		const c = control()
		if (el.querySelector(".kisland-form-file-summary")) {
			return text(el.querySelector(".kisland-form-file-summary"))
		}
		if (el.matches("fieldset")) {
			return options().filter((o) => o.input.checked && o.input.value !== "").map((o) => o.label).join(", ")
		}
		if (c.matches("select")) {
			return c.value === "" ? "" : norm(c.selectedOptions[0].textContent)
		}
		if (c.type === "checkbox") {
			return String(c.checked)
		}
		return c.value
	}
	case "type": {
		const c = control()
		if (!c.matches("input, textarea") || ["checkbox", "radio", "file"].includes(c.type)) {
			throw new Error("can't type into " + c.outerHTML.slice(0, 80))
		}
		c.focus()
		c.value = arg
		fire(c, "input")
		return ""
	}
	case "check": {
		const c = control()
		if (c.type !== "checkbox") {
			throw new Error("not a single checkbox")
		}
		if (c.checked !== arg) {
			c.click()
		}
		return ""
	}
	case "choose":
	case "unchoose": {
		const c = control()
		if (c.matches("select")) {
			if (op === "unchoose") {
				throw new Error("a dropdown can't unchoose")
			}
			const o = Array.from(c.options).find((o) => norm(o.textContent) === arg)
			if (!o) {
				throw new Error("no option " + JSON.stringify(arg))
			}
			c.value = o.value
			fire(c, "input", "change")
			return ""
		}
		const input = option(arg)
		if (input.checked !== (op === "choose")) {
			if (input.type === "radio" && op === "unchoose") {
				throw new Error("a radio can't unchoose")
			}
			input.click()
		}
		return ""
	}
	case "items":
		return String(items(el).length)
	case "add":
		el.querySelector(".kisland-form-items-foot > .kisland-form-item-add").click()
		return ""
	case "adddisabled":
		return String(el.querySelector(".kisland-form-items-foot > .kisland-form-item-add").getAttribute("aria-disabled") === "true")
	case "remove": {
		const item = nth(items(el), arg, "item")
		Array.from(item.querySelectorAll("button.kisland-form-item-remove")).find((b) => b.closest("li.kisland-form-item") === item).click()
		return ""
	}
	case "steps":
		return String(steps(el).filter((n) => !n.hasAttribute("data-complete")).length)
	case "completed":
		return String(steps(el).some((n) => n.hasAttribute("data-complete")))
	case "submit": {
		const b = submits().find((b) => norm(b.textContent) === arg)
		if (!b) {
			throw new Error("no submit button " + JSON.stringify(arg) + "; there are " + JSON.stringify(submits().map((b) => norm(b.textContent))))
		}
		if (b.disabled) {
			const help = actions().querySelector(".kisland-form-help")
			throw new Error("submit " + JSON.stringify(arg) + " is disabled: " + (help ? norm(help.textContent) : "no message"))
		}
		b.click()
		return ""
	}
	case "cancel": {
		const b = actions().querySelector("button[type=button]")
		if (!b) {
			throw new Error("no cancel button")
		}
		b.click()
		return ""
	}
	case "submitdisabled": {
		const states = submits().map((b) => b.disabled)
		return states.every((d) => d) ? "true" : states.every((d) => !d) ? "false" : "mixed " + JSON.stringify(states)
	}
	case "submitmessage": {
		const help = actions().querySelector(".kisland-form-help")
		return help ? norm(help.textContent) : ""
	}
	}
	throw new Error("unknown op " + op)
}`
