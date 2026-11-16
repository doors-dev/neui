//go:build browser

package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(r.Data, &list); err != nil {
		a.t.Fatalf("notification %s: %v", r.Data, err)
	}
	if len(list) != 1 {
		a.t.Fatalf("got %d notifications %s, want 1", len(list), r.Data)
	}
	return list[0].Output
}

// Find locates the first element matching selector on the page.
func (a *IslandApp) Find(selector string) Locator {
	return Locator{a: a}.Find(selector)
}

// Form locates the first form widget on the page.
func (a *IslandApp) Form() Form {
	return a.FormAt(".neisland-form")
}

// FormAt locates the form widget matching selector.
func (a *IslandApp) FormAt(selector string) Form {
	return Form{a.Find(selector)}
}

// ThrottleUploads limits what the page sends to bytesPerSecond, so uploads
// stay in progress long enough to watch; 0 lifts the limit.
func (a *IslandApp) ThrottleUploads(bytesPerSecond float64) {
	a.t.Helper()
	if bytesPerSecond == 0 {
		bytesPerSecond = -1
	}
	if err := (proto.NetworkEnable{}).Call(a.Page); err != nil {
		a.t.Fatal(err)
	}
	if err := (proto.NetworkEmulateNetworkConditions{Latency: 0, DownloadThroughput: -1, UploadThroughput: bytesPerSecond}).Call(a.Page); err != nil {
		a.t.Fatal(err)
	}
}

// FailUploads makes every file upload of the page fail with a network
// error before it reaches the server, until the returned func is called.
func (a *IslandApp) FailUploads() func() {
	a.t.Helper()
	router := a.Page.Context(context.Background()).HijackRequests()
	if err := router.Add("*?k=*", "", func(h *rod.Hijack) {
		h.Response.Fail(proto.NetworkErrorReasonConnectionFailed)
	}); err != nil {
		a.t.Fatal(err)
	}
	go router.Run()
	return func() {
		router.Stop()
	}
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
// "" when none, and the names of the uploaded files joined by ", " for a
// file field.
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

// SetFiles picks the files at paths in a file field and waits until each
// shows as a chip and none is uploading.
func (f Field) SetFiles(paths ...string) {
	f.a.t.Helper()
	before := f.read("maxkey", nil, "last chip key")
	f.PickFiles(paths...)
	f.waitChips(before, len(paths))
}

// PickFiles picks the files at paths in a file field without waiting for
// their uploads.
func (f Field) PickFiles(paths ...string) {
	f.a.t.Helper()
	f.Find(`input[type="file"]`).Element().MustSetFiles(paths...)
}

// DropFiles drops files on a file field's box, given as name and content
// pairs, and waits until each shows as a chip and none is uploading.
func (f Field) DropFiles(pairs ...string) {
	f.a.t.Helper()
	before := f.read("maxkey", nil, "last chip key")
	f.do("drop", pairs, "drop files")
	f.waitChips(before, len(pairs)/2)
}

func (f Field) waitChips(after string, n int) {
	f.a.t.Helper()
	f.expect("settled", after, strconv.Itoa(n), "new chips with no upload running")
}

// ExpectFiles waits until the file field shows exactly the uploaded files
// names, in order, ignoring failed chips, with no upload running.
func (f Field) ExpectFiles(names ...string) {
	f.a.t.Helper()
	f.expect("files", nil, strings.Join(names, ", "), "uploaded files")
}

// ExpectFileChips waits until the file field's chips read want, in order:
// the name of an uploaded file, "name uploading", or "name failed: reason".
func (f Field) ExpectFileChips(want ...string) {
	f.a.t.Helper()
	f.expect("chipstates", nil, strings.Join(want, ", "), "chips")
}

// ExpectFileFailed waits until a chip of the file field shows name failed
// with reason.
func (f Field) ExpectFileFailed(name, reason string) {
	f.a.t.Helper()
	f.expect("failed", name, reason, fmt.Sprintf("failure of %q", name))
}

// RemoveFile presses the × of the uploaded file name.
func (f Field) RemoveFile(name string) {
	f.a.t.Helper()
	f.Find(`button[aria-label="Remove ` + name + `"]`).Click()
}

// CancelFile presses the × of the file name while it uploads.
func (f Field) CancelFile(name string) {
	f.a.t.Helper()
	f.Find(`button[aria-label="Cancel upload of ` + name + `"]`).Click()
}

// DismissFile presses the × of the failed file name.
func (f Field) DismissFile(name string) {
	f.a.t.Helper()
	f.Find(`button[aria-label="Dismiss ` + name + `"]`).Click()
}

// RetryFile presses Retry on the failed file name once it shows.
func (f Field) RetryFile(name string) {
	f.a.t.Helper()
	f.do("retry", name, fmt.Sprintf("retry %q", name))
}

// ExpectName waits until the control's accessible name is want: its
// aria-label, the text of the elements its aria-labelledby names, or its
// label; for a radio or checkbox group, the group's aria-label or legend.
func (f Field) ExpectName(want string) {
	f.a.t.Helper()
	f.expect("name", nil, want, "accessible name")
}

// ExpectInvalid waits until the control's aria-invalid is true, or unset.
func (f Field) ExpectInvalid(invalid bool) {
	f.a.t.Helper()
	f.expect("invalid", nil, strconv.FormatBool(invalid), "aria-invalid")
}

// ExpectRequiredEmpty waits until the control carries data-required-empty,
// the underline of a required field still empty, or doesn't; the mark never
// counts as carried together with aria-invalid or on a disabled control.
func (f Field) ExpectRequiredEmpty(empty bool) {
	f.a.t.Helper()
	f.expect("requiredempty", nil, strconv.FormatBool(empty), "data-required-empty")
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
func (l List) Item(i int) Item {
	return Item{l.with("item", i, fmt.Sprintf("item %d", i))}
}

// Item is a list item; a group item's fields are found by label as within
// any locator, while a field item has no label of its own.
type Item struct {
	Locator
}

// Control locates the field that makes up a list of fields' item, which
// shows no label of its own.
func (i Item) Control() Field {
	return Field{i.with("control", nil, "control")}
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
		c.querySelectorAll(".neisland-form-required, .neisland-form-count, script").forEach((n) => n.remove())
		return norm(c.textContent)
	}
	const box = (el) => el.matches("fieldset.neisland-form-box")
	const labelOf = (el) => box(el)
		? el.querySelector(":scope > legend")
		: el.querySelector(":scope > .neisland-form-label, :scope > label.neisland-form-check > span")
	const find = (scope, sel, kind, label) => {
		const all = Array.from(scope.querySelectorAll(sel))
		const hit = all.find((n) => labelOf(n) && text(labelOf(n)) === label)
		if (!hit) {
			throw new Error("no " + kind + " labeled " + JSON.stringify(label) + "; there are " + JSON.stringify(all.map((n) => labelOf(n) ? text(labelOf(n)) : "")))
		}
		return hit
	}
	const boxes = (scope, inner) => Array.from(scope.querySelectorAll("fieldset.neisland-form-box")).filter((n) => n.querySelector(":scope > " + inner))
	const findBox = (scope, inner, kind, label) => {
		const all = boxes(scope, inner)
		const hit = all.find((n) => text(labelOf(n)) === label)
		if (!hit) {
			throw new Error("no " + kind + " labeled " + JSON.stringify(label) + "; there are " + JSON.stringify(all.map((n) => text(labelOf(n)))))
		}
		return hit
	}
	const items = (list) => {
		const ol = list.querySelector("ol.neisland-form-items")
		return Array.from(ol.querySelectorAll("li.neisland-form-item")).filter((n) => n.closest("ol.neisland-form-items") === ol)
	}
	const steps = (series) => {
		const ol = series.querySelector("ol.neisland-form-steps")
		return Array.from(ol.querySelectorAll("li.neisland-form-step")).filter((n) => n.closest("ol.neisland-form-steps") === ol)
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
			return find(scope, ".neisland-form-field, fieldset.neisland-form-group", "field", s.arg)
		case "group":
			return findBox(scope, ".neisland-form-rail", "group", s.arg)
		case "list":
			return findBox(scope, "ol.neisland-form-items", "list", s.arg)
		case "series":
			return findBox(scope, "ol.neisland-form-steps", "series", s.arg)
		case "item":
			return nth(items(scope), s.arg, "item")
		case "control": {
			const hit = scope.querySelector(":scope > .neisland-form-item-body > .neisland-form-field, :scope > .neisland-form-item-body > fieldset.neisland-form-group")
			if (!hit) {
				throw new Error("the item is not a field")
			}
			return hit
		}
		case "step":
			return nth(steps(scope).filter((n) => !n.hasAttribute("data-complete") && !n.hasAttribute("data-incomplete")), s.arg, "step")
		}
		throw new Error("unknown step " + s.kind)
	}
	const el = path.reduce(resolve, document)
	const control = () => el.matches("fieldset") ? el.querySelector(".neisland-form-options") : el.querySelector("input, select, textarea")
	const options = () => Array.from(el.querySelectorAll("label.neisland-form-check")).map((l) => ({label: text(l), input: l.querySelector("input")}))
	const option = (label) => {
		const o = options().find((o) => o.label === label)
		if (!o) {
			throw new Error("no option " + JSON.stringify(label) + "; there are " + JSON.stringify(options().map((o) => o.label)))
		}
		return o.input
	}
	const fileField = () => el.matches(".neisland-form-field") && !!el.querySelector(":scope > .neisland-form-file-zone, :scope > .neisland-form-search-chosen > .neisland-form-file-chip, :scope > .neisland-form-search-chosen .neisland-form-file-chip")
	const chips = () => Array.from(el.querySelectorAll("li.neisland-form-file-chip"))
	const chipName = (li) => norm(li.querySelector(".neisland-form-file-link, .neisland-form-file-text").textContent)
	const chipState = (li) => li.hasAttribute("data-uploading")
		? chipName(li) + " uploading"
		: li.hasAttribute("data-failed")
			? chipName(li) + " failed: " + norm(li.querySelector(".neisland-form-file-reason").textContent)
			: chipName(li)
	const actions = () => el.querySelector(".neisland-form-actions")
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
		const err = ids.map((id) => document.getElementById(id)).find((n) => n && n.classList.contains("neisland-form-error"))
		return err ? norm(err.textContent) : ""
	}
	case "required":
		return String(!!labelOf(el)?.querySelector(".neisland-form-required"))
	case "name": {
		const c = el.matches("fieldset") ? el : control()
		if (c.hasAttribute("aria-label")) {
			return norm(c.getAttribute("aria-label"))
		}
		if (c.hasAttribute("aria-labelledby")) {
			return c.getAttribute("aria-labelledby").split(" ").map((id) => document.getElementById(id)).filter(Boolean).map(text).join(" ")
		}
		const l = c.matches("fieldset") ? c.querySelector(":scope > legend") : c.closest("label") || (c.id && document.querySelector("label[for=\"" + c.id + "\"]"))
		return l ? text(l) : ""
	}
	case "invalid":
		return String(control().getAttribute("aria-invalid") === "true")
	case "requiredempty": {
		const c = control()
		if (!c || !c.hasAttribute("data-required-empty")) {
			return "false"
		}
		if (c.getAttribute("aria-invalid") === "true") {
			return "together with aria-invalid"
		}
		if (c.disabled) {
			return "on a disabled control"
		}
		return "true"
	}
	case "disabled": {
		const c = control()
		return String(!!c.disabled || c.getAttribute("aria-disabled") === "true")
	}
	case "value": {
		const c = control()
		if (fileField()) {
			return chips().filter((li) => !li.hasAttribute("data-uploading") && !li.hasAttribute("data-failed")).map(chipName).join(", ")
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
	case "maxkey":
		return String(Math.max(0, ...chips().map((li) => Number(li.dataset.key))))
	case "settled": {
		const all = chips()
		return all.some((li) => li.hasAttribute("data-uploading")) ? "uploading " + all.map(chipState).join(", ") : String(all.filter((li) => Number(li.dataset.key) > Number(arg)).length)
	}
	case "files": {
		const all = chips()
		if (all.some((li) => li.hasAttribute("data-uploading"))) {
			return "uploading " + all.map(chipState).join(", ")
		}
		return all.filter((li) => !li.hasAttribute("data-failed")).map(chipName).join(", ")
	}
	case "chipstates":
		return chips().map(chipState).join(", ")
	case "failed": {
		const li = chips().find((li) => li.hasAttribute("data-failed") && chipName(li) === arg)
		return li ? norm(li.querySelector(".neisland-form-file-reason").textContent) : "no failed chip; chips: " + chips().map(chipState).join(", ")
	}
	case "retry": {
		const li = chips().find((li) => li.hasAttribute("data-failed") && chipName(li) === arg)
		const b = li && li.querySelector("button.neisland-form-file-retry")
		if (!b || b.hidden) {
			throw new Error("no Retry for " + JSON.stringify(arg) + "; chips: " + chips().map(chipState).join(", "))
		}
		b.click()
		return ""
	}
	case "drop": {
		const zone = el.querySelector(".neisland-form-file-zone")
		if (!zone) {
			throw new Error("no box to drop on")
		}
		const transfer = new DataTransfer()
		for (let i = 0; i < arg.length; i += 2) {
			transfer.items.add(new File([arg[i + 1]], arg[i]))
		}
		for (const type of ["dragenter", "dragover", "drop"]) {
			zone.dispatchEvent(new DragEvent(type, {bubbles: true, cancelable: true, dataTransfer: transfer}))
		}
		return ""
	}
	case "items":
		return String(items(el).length)
	case "add":
		el.querySelector(":scope > .neisland-form-items-foot > .neisland-form-item-add").click()
		return ""
	case "adddisabled":
		return String(el.querySelector(":scope > .neisland-form-items-foot > .neisland-form-item-add").getAttribute("aria-disabled") === "true")
	case "remove": {
		const item = nth(items(el), arg, "item")
		Array.from(item.querySelectorAll("button.neisland-form-item-remove")).find((b) => b.closest("li.neisland-form-item") === item).click()
		return ""
	}
	case "steps":
		return String(steps(el).filter((n) => !n.hasAttribute("data-complete") && !n.hasAttribute("data-incomplete")).length)
	case "completed":
		return String(steps(el).some((n) => n.hasAttribute("data-complete")))
	case "submit": {
		const b = submits().find((b) => norm(b.textContent) === arg)
		if (!b) {
			throw new Error("no submit button " + JSON.stringify(arg) + "; there are " + JSON.stringify(submits().map((b) => norm(b.textContent))))
		}
		if (b.disabled) {
			const help = actions().querySelector(".neisland-form-help")
			throw new Error("submit " + JSON.stringify(arg) + " is disabled: " + (help ? norm(help.textContent) : "no message"))
		}
		b.click()
		return ""
	}
	case "submitdisabled": {
		const states = submits().map((b) => b.disabled)
		return states.every((d) => d) ? "true" : states.every((d) => !d) ? "false" : "mixed " + JSON.stringify(states)
	}
	case "submitmessage": {
		const help = actions().querySelector(".neisland-form-help")
		return help ? norm(help.textContent) : ""
	}
	}
	throw new Error("unknown op " + op)
}`
