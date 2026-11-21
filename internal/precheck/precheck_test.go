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

package precheck_test

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"neui.dev/internal/precheck"
)

func compile(t *testing.T, raw string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft7)
	if err := c.AddResource("urn:test", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("urn:test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testEnv(t *testing.T) precheck.Env {
	t.Helper()
	globals := map[string]bool{}
	for _, name := range strings.Fields("Object Array String Number JSON Math Date Error TypeError undefined NaN Infinity console print files__read") {
		globals[name] = true
	}
	return precheck.Env{
		Widgets: map[string]precheck.Widget{
			"markdown": {
				Params: compile(t, `{"type":"object","properties":{"text":{"type":"string","maxLength":20},"style":{"enum":["plain","note"]}},"required":["text"],"additionalProperties":false}`),
				Patch:  compile(t, `{"type":"object","properties":{"text":{"type":"string"},"style":{"enum":["plain","note"]}},"additionalProperties":false}`),
			},
			"button": {
				Params: compile(t, `{"type":"object","properties":{"text":{"type":"string"},"busy":{"type":"boolean"}},"additionalProperties":false}`),
				Patch:  compile(t, `{"type":"object","properties":{"busy":{"type":"boolean"}},"additionalProperties":false}`),
			},
			"form": {
				Params: compile(t, `{"type":"object","properties":{"fields":{"type":"array","items":{"anyOf":[
					{"type":"object","properties":{"type":{"const":"text"},"label":{"type":"string"},"key":{"type":"string"}},"required":["type"],"additionalProperties":false},
					{"type":"object","properties":{"type":{"const":"list"},"label":{"type":"string"},"options":{"type":"array","items":{"type":"string"}}},"required":["type"],"additionalProperties":false}
				]}}},"additionalProperties":false}`),
				Patch: compile(t, `{}`),
			},
			"table": {
				Params: compile(t, `{"type":"object","properties":{"query":{"type":"string"},"rows":{"type":"array","maxItems":2}},"additionalProperties":false}`),
				Patch:  compile(t, `{"type":"object","properties":{"rows":{"type":"array"}},"additionalProperties":false}`),
			},
		},
		Layouts: map[string]*jsonschema.Schema{
			"column": compile(t, `{"type":"object","properties":{"gap":{"enum":["small","large"]}},"additionalProperties":false}`),
		},
		Apps:    map[string]bool{"orders": true},
		Globals: globals,
	}
}

func check(t *testing.T, env precheck.Env, globals precheck.Globals, code string) ([]string, precheck.Globals) {
	t.Helper()
	findings, globals := precheck.Check(code, env, globals)
	messages := make([]string, len(findings))
	for i, f := range findings {
		messages[i] = f.Error()
	}
	return messages, globals
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHandleMethods(t *testing.T) {
	env := testEnv(t)
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"set on a view", "var note = ui.markdown({ text: \"hi\" })\nnote.set(ui.button({}))",
			[]string{"line 2, column 1: TypeError: note.set: ui.markdown has no set; set is for slots: to change what it shows, call update(patch)"}},
		{"update on a slot", "var body = ui.slot()\nbody.update({})",
			[]string{"line 2, column 1: TypeError: body.update: a slot has no update; update is for views and controls: to change what a slot shows, set it again"}},
		{"root is a slot", "ui.root.update({})",
			[]string{"line 1, column 1: TypeError: ui.root.update: a slot has no update; update is for views and controls: to change what a slot shows, set it again"}},
		{"unknown method", "var b = ui.button({})\nb.clear()",
			[]string{"line 2, column 1: TypeError: b.clear: ui.button has no clear; views and controls have only update(patch) and id"}},
		{"layout update", "var col = ui.column({})\ncol.update({})",
			[]string{"line 2, column 1: TypeError: col.update: ui.column is a layout and has no update; to change it, set its slot again"}},
		{"unknown ui member", "var t = ui.text({})",
			[]string{"line 1, column 9: TypeError: ui.text: no view, control or layout named text in the island's spec"}},
		{"returned from a function", "function make() { return ui.slot() }\nmake().update({})",
			[]string{"line 2, column 1: TypeError: make().update: a slot has no update; update is for views and controls: to change what a slot shows, set it again"}},
		{"fine", "var b = ui.button({}), s = ui.slot()\nui.root.set(ui.column({}, s))\ns.set(b)\nb.update({ busy: true })\nvar id = b.id\nb.toString()", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, precheck.Globals{}, c.code)
			expect(t, got, c.want...)
		})
	}
}

func TestParams(t *testing.T) {
	env := testEnv(t)
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"unknown key", `ui.markdown({ text: "a", size: 3 })`,
			[]string{"line 1, column 26: TypeError: ui.markdown params: additional properties 'size' not allowed"}},
		{"enum", `ui.markdown({ text: "a", style: "loud" })`,
			[]string{"line 1, column 33: TypeError: ui.markdown params at '/style': value must be one of 'plain', 'note'"}},
		{"type", `ui.button({ busy: "yes" })`,
			[]string{"line 1, column 19: TypeError: ui.button params at '/busy': got string, want boolean"}},
		{"required", `ui.markdown({ style: "note" })`,
			[]string{"line 1, column 13: TypeError: ui.markdown params: missing property 'text'"}},
		{"missing params", `ui.markdown()`,
			[]string{"line 1, column 1: TypeError: ui.markdown params: missing property 'text'"}},
		{"maxLength", `ui.markdown({ text: "a text longer than twenty" })`,
			[]string{"line 1, column 21: TypeError: ui.markdown params at '/text': maxLength: got 25, want 20"}},
		{"anyOf branch", `ui.form({ fields: [{ type: "text", label: "Name" }, { type: "list", label: "Tags", key: "tags" }] })`,
			[]string{"line 1, column 84: TypeError: ui.form params at '/fields/1': additional properties 'key' not allowed"}},
		{"no branch", `ui.form({ fields: [{ type: "date" }] })`,
			[]string{"line 1, column 28: TypeError: ui.form params at '/fields/0/type': value must be one of 'text', 'list'"}},
		{"no branch for the type", `ui.form({ fields: ["text"] })`,
			[]string{"line 1, column 20: TypeError: ui.form params at '/fields/0': got string, want object"}},
		{"no branch for a typed field", `ui.form({ fields: [{ type: 5, label: "Name" }] })`,
			[]string{"line 1, column 28: TypeError: ui.form params at '/fields/0/type': value must be one of 'text', 'list'"}},
		{"patch", "var n = ui.markdown({ text: \"a\" })\nn.update({ style: \"loud\" })",
			[]string{"line 2, column 19: TypeError: ui.markdown patch at '/style': value must be one of 'plain', 'note'"}},
		{"no patch", "var n = ui.markdown({ text: \"a\" })\nn.update()",
			[]string{"line 2, column 1: TypeError: n.update: update needs a patch object that matches the view's or control's patch schema"}},
		{"layout", `ui.column({ gap: "huge" }, ui.slot())`,
			[]string{"line 1, column 18: TypeError: ui.column params at '/gap': value must be one of 'small', 'large'"}},
		{"maxItems", `ui.table({ rows: [x(), y(), z()] })
function x() {} function y() {} function z() {}`,
			[]string{"line 1, column 18: TypeError: ui.table params at '/rows': maxItems: got 3, want 2"}},
		{"computed values", `var s = "loud", extra = {}
ui.markdown({ text: s, style: s })
ui.markdown({ text: "a", ...extra })
ui.markdown(JSON.parse("{}"))
ui.markdown(extra)
ui.markdown({ text: "a", [s]: 1 })
ui.markdown({ text: "a", style: s || "x" })
ui.button({ text: "a" + s, busy: !s })
ui.table({ query: String(function (p) { return p }) })
ui.form({ fields: [{ type: "list", label: s, options: [s, s] }] })
ui.form({ fields: [{ type: s, key: "k" }] })
ui.table({ query: function (p) { return p } })
ui.markdown({ __proto__: null, text: "a" })
ui.button({ text: "x", toJSON() { return { text: "y" } } })`, nil},
		{"maybe absent key", `var undef
ui.markdown({ text: "a", size: undef })`, nil},
		{"undefined dropped", `ui.markdown({ text: "a", size: undefined })`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, precheck.Globals{}, c.code)
			expect(t, got, c.want...)
		})
	}
}

func rooted(t *testing.T, env precheck.Env) precheck.Globals {
	t.Helper()
	got, globals := check(t, env, precheck.Globals{}, "ui.root.set(ui.markdown({ text: \"a\" }))")
	expect(t, got)
	return globals
}

func TestAppAndDraw(t *testing.T) {
	env := testEnv(t)
	globals := rooted(t, env)
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"unknown app function", "var o = app.orders({})\nvar c = app.customers({})",
			[]string{"line 2, column 9: TypeError: app.customers: no app function named customers in the island's spec"}},
		{"draw fields", "var action = ui.draw()\nvar ids = action.ids\nvar v = action.output.value, id = action.id",
			[]string{"line 2, column 11: TypeError: action.ids: ui.draw() returns only id and output; read the control's field as action.output.ids"}},
		{"draw in a loop", "function work() {\n  while (true) {\n    var ev = ui.draw(1000)\n    if (ev === null) continue\n    if (ev.value) return\n  }\n}",
			[]string{"line 5, column 9: TypeError: ev.value: ui.draw() returns only id and output; read the control's field as ev.output.value"}},
		{"bracket", "var ev = ui.draw()\nev[\"first name\"]",
			[]string{"line 2, column 1: TypeError: ev[\"first name\"]: ui.draw() returns only id and output; read the control's field as ev.output[\"first name\"]"}},
		{"destructured draw", "var { id, output, ids } = ui.draw()",
			[]string{"line 1, column 19: TypeError: ids: ui.draw() returns only id and output; read the control's field as output.ids"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, globals, c.code)
			expect(t, got, c.want...)
		})
	}
}

func TestUnknownGlobals(t *testing.T) {
	env := testEnv(t)
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"undeclared", "var a = 1\nnotify(a)",
			[]string{"line 2, column 1: TypeError: notify is not defined: no part declared it, and the script has no such tool or built-in"}},
		{"tool", "var r = page__click({ path: \"x\" })\njs__run({})",
			[]string{
				"line 1, column 9: TypeError: page__click is not defined: the script's tools are its modules' tools except page__* and js__*: it reaches the page only through app functions",
				"line 2, column 1: TypeError: js__run is not defined: the script's tools are its modules' tools except page__* and js__*: it reaches the page only through app functions",
			}},
		{"declared later", "show()\nfunction show() { total = count + 1 }\nvar count = 0\nconsole.log(total)", nil},
		{"locals", "function f(a, { b }, ...c) { var d; let e; try { g() } catch (err) { err.message } return [a, b, c, d, e, arguments] }\nfunction g() {}\nfor (const k of [1]) k\nfor (var i in {}) i\nvar h = function named() { return named }\nclass K { m() { return this } }", nil},
		{"typeof guard", "if (typeof files__write === \"function\") files__write({})", nil},
		{"labels and keys", "outer: for (;;) { break outer }\nvar o = { notify: 1 }\no.notify", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, precheck.Globals{}, c.code)
			expect(t, got, c.want...)
		})
	}
}

func TestSwitchOff(t *testing.T) {
	env := testEnv(t)
	got, globals := check(t, env, precheck.Globals{}, "eval(\"var x = 1\")\nx\nvar n = ui.markdown({ text: \"a\" })\nn.set()")
	expect(t, got)
	got, _ = check(t, env, globals, "y")
	expect(t, got)
	got, globals = check(t, env, precheck.Globals{}, "var n = ui.markdown({ text: \"a\" })\nwith ({ x: 1 }) { x\n  n.set() }\nn.set()")
	expect(t, got, "line 4, column 1: TypeError: n.set: ui.markdown has no set; set is for slots: to change what it shows, call update(patch)")
	got, _ = check(t, env, globals, "y")
	expect(t, got, "line 1, column 1: TypeError: y is not defined: no part declared it, and the script has no such tool or built-in")
	got, globals = check(t, env, precheck.Globals{}, "var store = { add(x) { this.items = x } }\nfunction Box() { this.size = 1 }")
	expect(t, got)
	got, _ = check(t, env, globals, "items + size\ny")
	expect(t, got, "line 2, column 1: TypeError: y is not defined: no part declared it, and the script has no such tool or built-in")
	got, globals = check(t, env, precheck.Globals{}, "globalThis.x = 1\nx\nui.slot().update({})")
	expect(t, got, "line 3, column 1: TypeError: ui.slot().update: a slot has no update; update is for views and controls: to change what a slot shows, set it again")
	got, _ = check(t, env, globals, "x + y")
	expect(t, got)
}

func TestGlobalsAcrossParts(t *testing.T) {
	env := testEnv(t)
	got, first := check(t, env, precheck.Globals{}, "var note = ui.markdown({ text: \"a\" })\nvar body = ui.slot()\nlet seen = 0\nfunction show() {}")
	expect(t, got)
	got, _ = check(t, env, first, "note.set()\nbody.set(note)\nseen++\nshow()")
	expect(t, got, "line 1, column 1: TypeError: note.set: ui.markdown has no set; set is for slots: to change what it shows, call update(patch)")
	got, _ = check(t, env, first, "note = ui.slot()\nnote.set()")
	expect(t, got)
	got, second := check(t, env, first, "var extra = 1")
	expect(t, got)
	got, _ = check(t, env, second, "extra + seen\nmissing")
	expect(t, got, "line 2, column 1: TypeError: missing is not defined: no part declared it, and the script has no such tool or built-in")
	got, _ = check(t, env, first, "function show() { return ui.slot() }\nshow().set()\nshow().update({})")
	expect(t, got, "line 3, column 1: TypeError: show().update: a slot has no update; update is for views and controls: to change what a slot shows, set it again")
}

func TestNeverGuess(t *testing.T) {
	env := testEnv(t)
	for _, c := range []struct {
		name string
		code string
	}{
		{"union", "var w = ui.slot()\nif (Math.random() > 0.5) w = ui.button({})\nw.set()\nw.update({})"},
		{"held in arrays and objects", "var all = [ui.button({})]\nall[0].set()\nvar o = { b: ui.slot() }\no.b.update({})"},
		{"parameters", "function f(w) { w.set() }\nf(ui.button({}))"},
		{"written fields", "var b = ui.button({})\nb.tag = \"x\"\nb.tag"},
		{"optional call", "var b = ui.button({})\nb.set?.()\ntypeof b.value"},
		{"extended", "var b = ui.button({})\nObject.assign(b, { set: function () {} })\nb.set()"},
		{"shadowed ui", "function f(ui) { ui.anything() }"},
		{"parse failure", "var x = ui.slot()\nx.update({})\n#!"},
		{"feature tests", "var rows = app.exportCsv ? app.exportCsv({}) : []\nif (app.archive) app.archive({})\nvar make = ui.chart || ui.markdown\nvar w = ui.markdown({ text: \"a\" })\nif (w.set) { w.set() } else { w.update({ text: \"b\" }) }\nconsole.log(ui.button({}).name)\nvar s = ui.slot()\nif (typeof s.clear === \"function\") s.clear()\nvar { set } = w"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, precheck.Globals{}, c.code)
			expect(t, got)
		})
	}
	_, lost := check(t, env, precheck.Globals{}, "var x = 1\n)")
	got, _ := check(t, env, lost, "anything\nui.slot().update({})\nui.text({})")
	expect(t, got, "line 3, column 1: TypeError: ui.text: no view, control or layout named text in the island's spec")
}

func TestAfterEndlessCall(t *testing.T) {
	env := testEnv(t)
	start := rooted(t, env)
	got, globals := check(t, env, start, `var x = ui.slot()
function loop() {
  while (true) {
    var action = ui.draw()
    if (action.output) continue
  }
}
loop()
x = ui.button({})
x.set()
var later = 1
function after() {}`)
	expect(t, got)
	got, _ = check(t, env, globals, "x.set()\nafter()\nlater")
	expect(t, got)
	got, _ = check(t, env, globals, "x.update({})")
	expect(t, got, "line 1, column 1: TypeError: x.update: a slot has no update; update is for views and controls: to change what a slot shows, set it again")
	got, _ = check(t, env, globals, "loop()\nx.update({})")
	expect(t, got)
	got, _ = check(t, env, globals, "loop = function () {}\nloop()\nx.update({})")
	expect(t, got, "line 3, column 1: TypeError: x.update: a slot has no update; update is for views and controls: to change what a slot shows, set it again")
	got, globals = check(t, env, start, `var y = ui.slot()
function work() {
  while (true) {
    if (ui.draw().output) return
  }
}
work()
y = ui.button({})`)
	expect(t, got)
	got, _ = check(t, env, globals, "y.update({})\ny.set()")
	expect(t, got)
	got, _ = check(t, env, start, "var z = ui.slot()\nui.end()\nz = ui.button({})\nz.update({})")
	expect(t, got)
	got, globals = check(t, env, start, "var w = ui.slot()\nfunction loop() { while (true) ui.draw() }\nfunction start() { loop() }\nstart()\nw.update({})")
	expect(t, got)
	_, globals = check(t, env, globals, "loop = function () {}")
	_, globals = check(t, env, globals, "start()\nw = ui.button({})")
	got, _ = check(t, env, globals, "w.update({})")
	expect(t, got)
	for _, code := range []string{
		"function* ticks() { while (true) { yield ui.draw(1000) } }\nticks()\nlater = 1",
		"async function poll() { while (true) { await null; ui.draw(1000) } }\npoll()\nlater = 1",
	} {
		got, globals = check(t, env, start, code)
		expect(t, got)
		got, _ = check(t, env, globals, "console.log(later)")
		expect(t, got)
	}
}

func TestUncalledBodies(t *testing.T) {
	env := testEnv(t)
	got, first := check(t, env, precheck.Globals{}, "var body = ui.slot()\nui.root.set(body)\nfunction work() { while (true) { var a = ui.draw(); handle(a) } }\nfunction render() { panel.set(ui.markdown({ text: \"x\" })) }\nclass K { size = later }\nvar each = [1].map(function (n) { return missing(n) })")
	expect(t, got)
	got, _ = check(t, env, first, "function handle(a) { body.set(ui.markdown({ text: String(a.id) })) }\nvar panel = ui.slot()\nrender()\nwork()")
	expect(t, got)
	got, _ = check(t, env, precheck.Globals{}, "function a() { b() }\nfunction b() { gone() }\nvar c = () => lost()\na()\nc()\nnew function () { away() }()\n;(function () { absent() })()")
	expect(t, got,
		"line 2, column 16: TypeError: gone is not defined: no part declared it, and the script has no such tool or built-in",
		"line 3, column 15: TypeError: lost is not defined: no part declared it, and the script has no such tool or built-in",
		"line 6, column 19: TypeError: away is not defined: no part declared it, and the script has no such tool or built-in",
		"line 7, column 17: TypeError: absent is not defined: no part declared it, and the script has no such tool or built-in",
	)
}

func TestPositions(t *testing.T) {
	env := testEnv(t)
	got, _ := check(t, env, precheck.Globals{}, "var s = \"ééé😀\"; missing\r\n\tother")
	expect(t, got,
		"line 1, column 17: TypeError: missing is not defined: no part declared it, and the script has no such tool or built-in",
		"line 2, column 2: TypeError: other is not defined: no part declared it, and the script has no such tool or built-in",
	)
}

func TestEmptyScreen(t *testing.T) {
	env := testEnv(t)
	empty := "TypeError: %s: the screen is empty, as no part sets ui.root: set it to what the user should see, such as ui.root.set(form), before you draw"
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"draw", "var b = ui.button({})\nvar action = ui.draw()", []string{"line 2, column 14: " + strings.ReplaceAll(empty, "%s", "ui.draw()")}},
		{"timer draw", "ui.draw(500)", []string{"line 1, column 1: " + strings.ReplaceAll(empty, "%s", "ui.draw(500)")}},
		{"in a called loop, once", "function loop() { while (true) { ui.draw(); ui.draw() } }\nloop()", []string{"line 1, column 34: " + strings.ReplaceAll(empty, "%s", "ui.draw()")}},
		{"uncalled", "function loop() { while (true) ui.draw() }", nil},
		{"root set after", "function loop() { while (true) ui.draw() }\nui.root.set(ui.button({}))\nloop()", nil},
		{"root set in a method", "var screen = { show: function () { ui.root.set(ui.button({})) } }\nscreen.show()\nui.draw()", nil},
		{"root through a variable", "var u = ui\nu.root.set(ui.button({}))\nui.draw()", nil},
		{"ui handed on", "function show(target) { target.root.set(target.button({})) }\nshow(ui)\nui.draw()", nil},
		{"root by a computed key", "var k = \"root\"\nui[k].set(ui.button({}))\nui.draw()", nil},
		{"ui in an object", "var o = { ui }\no.ui.root.set(ui.button({}))\nui.draw()", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, precheck.Globals{}, c.code)
			expect(t, got, c.want...)
		})
	}
	_, first := check(t, env, precheck.Globals{}, "var b = ui.button({})\nui.root.set(b)")
	got, _ := check(t, env, first, "ui.draw()")
	expect(t, got)
	_, first = check(t, env, precheck.Globals{}, "var b = ui.button({})")
	got, _ = check(t, env, first, "ui.draw()")
	expect(t, got, "line 1, column 1: "+strings.ReplaceAll(empty, "%s", "ui.draw()"))
}

func TestDrawIDComparedWithLiteral(t *testing.T) {
	env := testEnv(t)
	globals := rooted(t, env)
	literal := "TypeError: %s: a control's id is a number set when it is created, never a literal such as %s: compare it with that control, such as form.id, kept in a variable the loop can reach"
	message := func(at, id, lit string) string {
		return at + ": " + strings.Replace(strings.Replace(literal, "%s", id, 1), "%s", lit, 1)
	}
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"string", "var action = ui.draw()\nif (action.id === \"submit\") {}", []string{message("line 2, column 5", "action.id", `"submit"`)}},
		{"reversed and loose", "var action = ui.draw()\nif (\"save\" == action.id) {}", []string{message("line 2, column 5", "action.id", `"save"`)}},
		{"number in a variable", "var id = ui.draw().id\nif (id !== 3) {}", []string{message("line 2, column 5", "id", "3")}},
		{"template", "if (ui.draw().id === `save`) {}", []string{message("line 1, column 5", "ui.draw().id", "`save`")}},
		{"switch", "switch (ui.draw().id) {\ncase \"save\":\n  break\ncase 2:\n  break\ndefault:\n}", []string{message("line 2, column 6", "ui.draw().id", `"save"`), message("line 4, column 6", "ui.draw().id", "2")}},
		{"control id", "var b = ui.button({})\nvar action = ui.draw()\nif (action.id === b.id) {}\nswitch (action.id) { case b.id: }", nil},
		{"null", "var action = ui.draw()\nif (action.id === null) {}", nil},
		{"id written", "var action = ui.draw()\nvar row = {}\nrow.id = \"save\"\nif (action.id === \"save\") {}", nil},
		{"reassigned", "var id = ui.draw().id\nid = \"save\"\nif (id === \"save\") {}", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, globals, c.code)
			expect(t, got, c.want...)
		})
	}
}

func TestOutputFields(t *testing.T) {
	env := testEnv(t)
	set := func(name string, output string) {
		w := env.Widgets[name]
		w.Output = compile(t, output)
		env.Widgets[name] = w
	}
	set("form", `{"type":"object","properties":{"data":{"type":"object","additionalProperties":{}},"submit":{"type":"string"}},"required":["data","submit"]}`)
	set("table", `{"type":"object","properties":{"row":{"type":"string"},"value":{"type":"string"},"rows":{"type":"array"}}}`)
	set("button", `{"type":"string"}`)
	globals := rooted(t, env)
	missing := "TypeError: %s: no control's output has %s; the outputs have only data, row, rows, submit, value"
	message := func(at, label, name string) string {
		return at + ": " + strings.Replace(strings.Replace(missing, "%s", label, 1), "%s", name, 1)
	}
	for _, c := range []struct {
		name string
		code string
		want []string
	}{
		{"unknown field", "var action = ui.draw()\nvar qty = action.output.new_qty", []string{message("line 2, column 11", "action.output.new_qty", "new_qty")}},
		{"bracket", "var action = ui.draw()\naction.output[\"new qty\"]", []string{message("line 2, column 1", `action.output["new qty"]`, "new qty")}},
		{"destructured", "var { data, qty } = ui.draw().output", []string{message("line 1, column 13", "qty", "qty")}},
		{"nested pattern", "var { id, output: { submit, qty } } = ui.draw()", []string{message("line 1, column 29", "qty", "qty")}},
		{"through a variable", "var { output } = ui.draw()\noutput.qty", []string{message("line 2, column 1", "output.qty", "qty")}},
		{"known fields", "var action = ui.draw()\nvar d = action.output.data, s = action.output.submit, r = action.output.rows\nvar q = action.output.data.new_qty", nil},
		{"string members", "var action = ui.draw()\nif (action.output.length > 0) action.output.toLowerCase()", nil},
		{"object members", "var action = ui.draw()\naction.output.hasOwnProperty(\"data\")", nil},
		{"guarded", "var action = ui.draw()\nif (typeof action.output.qty === \"number\") {}", nil},
		{"written", "var action = ui.draw()\nvar o = {}\no.qty = 1\naction.output.qty", nil},
		{"computed write", "var action = ui.draw()\nvar k = \"note\"\naction.output[k] = 1\nui.notify(action.output.note)", nil},
		{"index", "var action = ui.draw()\naction.output[\"0\"]", nil},
		{"not an index", "var action = ui.draw()\naction.output[\"01\"]", []string{message("line 2, column 1", `action.output["01"]`, "01")}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := check(t, env, globals, c.code)
			expect(t, got, c.want...)
		})
	}
	set("button", `{"type":"object","properties":{"value":{"type":"string"}}}`)
	got, _ := check(t, env, globals, "var action = ui.draw()\naction.output[\"0\"]")
	expect(t, got, message("line 2, column 1", `action.output["0"]`, "0"))
	set("table", `{"type":"object","additionalProperties":{}}`)
	got, _ = check(t, env, globals, "ui.draw().output.anything")
	expect(t, got)
	set("table", `{"description":"any JSON value"}`)
	got, _ = check(t, env, globals, "ui.draw().output.anything")
	expect(t, got)
	got, _ = check(t, testEnv(t), rooted(t, testEnv(t)), "ui.draw().output.anything")
	expect(t, got)
}
