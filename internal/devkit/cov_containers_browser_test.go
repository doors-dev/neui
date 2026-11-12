//go:build browser

package devkit_test

import (
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func contForm(t *testing.T, spec string) (*stack.IslandApp, stack.Form) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form(` + spec + `)); ui.notify(ui.draw())`})
	return a, a.Form()
}

func contFails(t *testing.T, spec string, want string) {
	t.Helper()
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form(` + spec + `)); ui.notify(ui.draw())`})
	if got := a.ContainerScriptError(); !strings.Contains(got, want) {
		t.Fatalf("ui.form failed with %q, want it to mention %q", got, want)
	}
}

const covGroupNestedSpec = `{fields: [
	{type: "group", label: "Profile", name: "profile", fields: [
		{type: "text", label: "Name", name: "name"},
		{type: "text", label: "Nick", name: "nick"},
		{type: "group", label: "Address", name: "address", fields: [
			{type: "text", label: "City", name: "city"},
			{type: "group", label: "Geo", name: "geo", fields: [
				{type: "number", label: "Lat", name: "lat"},
				{type: "checkbox", label: "Exact", name: "exact"}
			]}
		]}
	]}
]}`

func TestCovGroupNestsValuesAndLeavesEmptyFieldsOut(t *testing.T) {
	a, f := contForm(t, covGroupNestedSpec)
	profile := f.Group("Profile")
	profile.Field("Name").Type("Ada")
	profile.Group("Address").Field("City").Type("Lima")
	profile.Group("Address").Group("Geo").Field("Lat").Type("-12.5")
	profile.Field("Nick").Type("x")
	profile.Field("Nick").Type("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"profile": {"name": "Ada", "address": {"city": "Lima", "geo": {"lat": -12.5, "exact": false}}}}, "submit": ""}`)
}

const covGroupGatingSpec = `{fields: [
	{type: "group", label: "Outer", name: "outer", fields: [
		{type: "group", label: "Inner", name: "inner", fields: [
			{type: "text", label: "Must", name: "must", required: true},
			{type: "text", label: "Short", name: "short", maxLength: 2}
		]}
	]}
]}`

func TestCovGroupNestedRequiredAndErrorsGateSubmit(t *testing.T) {
	a, f := contForm(t, covGroupGatingSpec)
	inner := f.Group("Outer").Group("Inner")
	inner.Field("Must").ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	inner.Field("Short").Type("abc")
	inner.Field("Short").ExpectError("use at most 2 characters")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	inner.Field("Must").Type("ok")
	inner.Field("Must").ExpectRequiredMark(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	inner.Field("Short").Type("ab")
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
	inner.Field("Must").Type("")
	inner.Field("Must").ExpectRequiredMark(true)
	inner.Field("Must").ExpectNoError()
	inner.Field("Must").ExpectInvalid(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	inner.Field("Must").Type("ok")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"outer": {"inner": {"must": "ok", "short": "ab"}}}, "submit": ""}`)
}

const covGroupDefaultsSpec = `{fields: [{type: "list", value: [
	{name: "Ada", address: {city: "Lima", geo: {lat: 4}}, tags: ["x", "y"], plan: [{value: "pro", label: "Pro"}], ok: true},
	{address: {}}
], field: {type: "group", label: "People", name: "people", fields: [
	{type: "text", label: "Name", name: "name", value: "own"},
	{type: "group", label: "Address", name: "address", fields: [
		{type: "text", label: "City", name: "city", value: "Paris"},
		{type: "group", label: "Geo", name: "geo", fields: [{type: "number", label: "Lat", name: "lat", value: 1}]}
	]},
	{type: "list", value: ["own"], field: {type: "text", label: "Tags", name: "tags"}},
	{type: "select", label: "Plan", name: "plan", options: [{value: "free", label: "Free"}, {value: "pro", label: "Pro"}]},
	{type: "checkbox", label: "Ok", name: "ok"}
]}}]}`

func TestCovGroupDefaultsPropagateFromParentAndFallBackToOwn(t *testing.T) {
	a, f := contForm(t, covGroupDefaultsSpec)
	people := f.List("People")
	people.ExpectCount(2)
	first := people.Item(0)
	first.Field("Name").ExpectValue("Ada")
	first.Group("Address").Field("City").ExpectValue("Lima")
	first.Group("Address").Group("Geo").Field("Lat").ExpectValue("4")
	first.List("Tags").ExpectCount(2)
	first.List("Tags").Item(1).Control().ExpectValue("y")
	first.Field("Plan").ExpectValue("Pro")
	first.Field("Ok").ExpectValue("true")
	second := people.Item(1)
	second.Field("Name").ExpectValue("own")
	second.Group("Address").Field("City").ExpectValue("Paris")
	second.Group("Address").Group("Geo").Field("Lat").ExpectValue("1")
	second.List("Tags").ExpectCount(1)
	second.List("Tags").Item(0).Control().ExpectValue("own")
	second.Field("Plan").ExpectValue("")
	second.Field("Ok").ExpectValue("false")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"people": [
		{"name": "Ada", "address": {"city": "Lima", "geo": {"lat": 4}}, "tags": ["x", "y"], "plan": [{"value": "pro", "label": "Pro"}], "ok": true},
		{"name": "own", "address": {"city": "Paris", "geo": {"lat": 1}}, "tags": ["own"], "ok": false}
	]}, "submit": ""}`)
}

const covGroupTemplate = `{type: "group", label: "People", name: "people", fields: [
	{type: "text", label: "Name", name: "name"},
	{type: "group", label: "Address", name: "address", fields: [{type: "text", label: "City", name: "city"}]}
]}`

func TestCovGroupWrongDefaultShapeFails(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"item not an object", `["Ada"]`, `group "people": default value is string, want an object`},
		{"nested group not an object", `[{address: "Lima"}]`, `group "address": default value is string, want an object`},
		{"field wrong type", `[{name: 5}]`, `field "name": default value is float64, want a string`},
		{"nested field wrong type", `[{address: {city: true}}]`, `field "city": default value is bool, want a string`},
	} {
		t.Run(c.name, func(t *testing.T) {
			contFails(t, `{fields: [{type: "list", value: `+c.value+`, field: `+covGroupTemplate+`}]}`, c.want)
		})
	}
}

const covListBoundsSpec = `{fields: [{type: "list", min: 2, max: 3, field: {type: "text", label: "Tags", name: "tags"}}]}`

func TestCovListStartsEmptyWithoutPadding(t *testing.T) {
	a, f := contForm(t, covListBoundsSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.ExpectCount(1)
	tags.ExpectRequiredMark(true)
	tags.ExpectError("add at least 2")
	tags.Item(0).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(0).Control().Type("a")
	tags.Item(0).Control().ExpectRequiredEmpty(false)
	tags.ExpectRequiredMark(true)
	tags.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(1).Control().Type("b")
	tags.Item(1).Control().ExpectRequiredEmpty(false)
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["a", "b"]}, "submit": ""}`)
}

func TestCovListRemovesAtAnyCount(t *testing.T) {
	_, f := contForm(t, covListBoundsSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.Add()
	tags.Add()
	tags.ExpectCount(2)
	tags.WaitCount("button.kisland-form-item-remove", 2)
	tags.Item(0).Control().Type("a")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Remove(1)
	tags.ExpectCount(1)
	tags.WaitCount("button.kisland-form-item-remove", 1)
	tags.ExpectError("add at least 2")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Remove(0)
	tags.ExpectCount(0)
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.ExpectCount(1)
	tags.ExpectError("add at least 2")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestCovListBelowMinCountsItemsAndEmptyItemsBlock(t *testing.T) {
	a, f := contForm(t, covListBoundsSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.Add()
	tags.Add()
	tags.Add()
	tags.ExpectCount(3)
	tags.Item(0).Control().Type("a")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.Item(1).Control().ExpectRequiredEmpty(true)
	tags.Item(2).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(2).Control().Type("c")
	tags.Item(2).Control().ExpectRequiredEmpty(false)
	tags.ExpectNoError()
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Remove(1)
	tags.ExpectCount(2)
	tags.ExpectRequiredMark(false)
	f.ExpectSubmitMessage("")
	tags.Remove(1)
	tags.ExpectCount(1)
	tags.ExpectRequiredMark(true)
	tags.ExpectError("add at least 2")
	tags.Add()
	tags.Item(1).Control().Type("b")
	tags.ExpectNoError()
	tags.ExpectRequiredMark(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["a", "b"]}, "submit": ""}`)
}

const covListItemErrorSpec = `{fields: [{type: "list", min: 2, field: {type: "text", label: "Tags", name: "tags", maxLength: 2}}]}`

func TestCovListCountErrorBesideItemErrors(t *testing.T) {
	_, f := contForm(t, covListItemErrorSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.Add()
	tags.ExpectCount(1)
	tags.ExpectError("add at least 2")
	tags.Item(0).Control().ExpectRequiredEmpty(true)
	tags.Item(0).Control().Type("abc")
	tags.Item(0).Control().ExpectError("use at most 2 characters")
	tags.Item(0).Control().ExpectInvalid(true)
	tags.Item(0).Control().ExpectRequiredEmpty(false)
	tags.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(0).Control().Type("ab")
	tags.Item(0).Control().ExpectNoError()
	tags.ExpectError("add at least 2")
}

const covListRequiredItemSpec = `{fields: [{type: "list", min: 2, field: {type: "group", label: "People", name: "people", fields: [
	{type: "text", label: "Name", name: "name", required: true}
]}}]}`

func TestCovListCountErrorBesideRequiredItems(t *testing.T) {
	_, f := contForm(t, covListRequiredItemSpec)
	people := f.List("People")
	people.ExpectCount(0)
	people.Add()
	people.ExpectCount(1)
	people.Item(0).Field("Name").ExpectRequiredMark(true)
	people.ExpectRequiredMark(true)
	people.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	people.Item(0).Field("Name").Type("Ada")
	people.Item(0).Field("Name").ExpectRequiredMark(false)
	people.ExpectRequiredMark(true)
	people.ExpectError("add at least 2")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

const covListRemoveSpec = `{fields: [{type: "list", field: {type: "group", label: "People", name: "people", fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "number", label: "Age", name: "age", max: 150}
]}}]}`

func TestCovListRemovalDropsDataAndErrors(t *testing.T) {
	a, f := contForm(t, covListRemoveSpec)
	people := f.List("People")
	people.ExpectCount(0)
	for i, name := range []string{"Ada", "Bob", "Cy"} {
		people.Add()
		people.ExpectCount(i + 1)
		people.Item(i).Field("Name").Type(name)
	}
	people.Item(1).Field("Age").Type("200")
	people.Item(1).Field("Age").ExpectError("enter 150 or less")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	people.Remove(1)
	people.ExpectCount(2)
	f.ExpectSubmitMessage("")
	people.Add()
	people.ExpectCount(3)
	people.Item(2).Field("Name").ExpectValue("")
	people.Item(2).Field("Age").ExpectValue("")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	people.Remove(2)
	people.ExpectCount(2)
	f.ExpectSubmitDisabled(false)
	people.Item(1).Field("Name").ExpectValue("Cy")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"people": [{"name": "Ada"}, {"name": "Cy"}]}, "submit": ""}`)
}

const covListMaxSpec = `{fields: [{type: "list", max: 2, field: {type: "text", label: "Tags", name: "tags"}}]}`

func TestCovListMaxDisablesAddWithReason(t *testing.T) {
	_, f := contForm(t, covListMaxSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.ExpectHelp("None added.")
	tags.ExpectAddDisabled(false)
	tags.Add()
	tags.ExpectCount(1)
	tags.ExpectHelp("")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectAddDisabled(true)
	tags.ExpectHelp("Maximum of 2 reached.")
	add := tags.Find(":scope > .kisland-form-items-foot > .kisland-form-item-add")
	help := tags.Find(":scope > .kisland-form-items-foot .kisland-form-help")
	add.WaitAttr("aria-describedby", *help.Element().MustAttribute("id"))
	tags.Add()
	tags.ExpectCount(2)
	tags.Remove(0)
	tags.ExpectCount(1)
	tags.ExpectAddDisabled(false)
	tags.ExpectHelp("")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectAddDisabled(true)
}

func TestCovListDefaultsFromValue(t *testing.T) {
	a, f := contForm(t, `{fields: [
		{type: "list", min: 3, value: ["a"], field: {type: "text", label: "Tags", name: "tags", value: "z"}},
		{type: "list", value: [2, 3.5], field: {type: "number", label: "Nums", name: "nums"}},
		{type: "list", value: [[{value: "b", label: "B"}]], field: {type: "select", label: "Picks", name: "picks", options: [{value: "a", label: "A"}, {value: "b", label: "B"}]}},
		{type: "list", value: [[{value: 7, label: "Seven"}]], field: {type: "search", label: "Finds", name: "finds", search: "(q) => []"}}
	]}`)
	tags := f.List("Tags")
	tags.ExpectCount(1)
	tags.Item(0).Control().ExpectValue("a")
	tags.ExpectError("add at least 3")
	tags.Add()
	tags.Add()
	tags.ExpectCount(3)
	tags.Item(1).Control().ExpectValue("z")
	tags.Item(2).Control().ExpectValue("z")
	tags.ExpectNoError()
	f.List("Nums").ExpectCount(2)
	f.List("Nums").Item(1).Control().ExpectValue("3.5")
	f.List("Picks").Item(0).Control().ExpectValue("B")
	f.List("Finds").Item(0).Find(`button[aria-label="Remove Seven"]`).Element()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {
		"tags": ["a", "z", "z"],
		"nums": [2, 3.5],
		"picks": [[{"value": "b", "label": "B"}]],
		"finds": [[{"value": 7, "label": "Seven"}]]
	}, "submit": ""}`)
}

func TestCovListWrongDefaultShapeFails(t *testing.T) {
	for _, c := range []struct{ name, spec, want string }{
		{"item wrong type", `{type: "list", value: [5], field: {type: "text", label: "Tags", name: "tags"}}`, `field "tags": default value is float64, want a string`},
		{"more than max", `{type: "list", max: 2, value: ["a", "b", "c"], field: {type: "text", label: "Tags", name: "tags"}}`, `list "tags": 3 default items, most is 2`},
		{"select not an option", `{type: "list", value: [[{value: "q", label: "Q"}]], field: {type: "select", label: "Picks", name: "picks", options: [{value: "a", label: "A"}]}}`, `is not one of the options`},
		{"inner list not an array", `{type: "list", value: [["a"], "b"], field: {type: "list", field: {type: "text", label: "Tags", name: "tags"}}}`, `list "tags": default value is string, want an array`},
		{"inner list in group not an array", `{type: "list", value: [{tags: "a"}], field: {type: "group", label: "Teams", name: "teams", fields: [{type: "list", field: {type: "text", label: "Tags", name: "tags"}}]}}`, `list "tags": default value is string, want an array`},
	} {
		t.Run(c.name, func(t *testing.T) {
			contFails(t, `{fields: [`+c.spec+`]}`, c.want)
		})
	}
}

func TestCovListKeyLabelAndNamesFromTemplate(t *testing.T) {
	a, f := contForm(t, `{fields: [
		{type: "list", min: 2, field: {type: "text", label: "Tag words", name: "tagKeys"}},
		{type: "list", min: 2, field: {type: "group", label: "People", name: "people", fields: [{type: "text", label: "Name", name: "name"}]}}
	]}`)
	tags := f.List("Tag words")
	tags.ExpectCount(0)
	tags.Add()
	tags.Add()
	tags.ExpectCount(2)
	tags.Find(":scope > .kisland-form-items-foot > .kisland-form-item-add").WaitAttr("aria-label", "Add to Tag words")
	tags.ExpectRemoveName(0, "Remove item 1 of Tag words")
	tags.ExpectRemoveName(1, "Remove item 2 of Tag words")
	tags.Item(0).Control().ExpectName("Tag words, item 1")
	tags.Item(1).Control().ExpectName("Tag words, item 2")
	tags.Item(1).Control().Type("b")
	tags.Remove(0)
	tags.ExpectRemoveName(0, "Remove item 1 of Tag words")
	tags.Item(0).Control().ExpectName("Tag words, item 1")
	tags.Item(0).Control().ExpectValue("b")

	people := f.List("People")
	people.Add()
	people.Add()
	people.ExpectItemName(0, "People, item 1")
	people.ExpectItemName(1, "People, item 2")
	people.Item(0).Find(".kisland-form-item-body").WaitAttr("role", "group")
	people.Item(1).Field("Name").Type("Bob")
	people.Remove(0)
	people.ExpectCount(1)
	people.ExpectItemName(0, "People, item 1")
	people.ExpectRemoveName(0, "Remove item 1 of People")
	people.Add()
	people.ExpectItemName(1, "People, item 2")
	people.Item(1).Field("Name").Type("Cy")
	tags.Add()
	tags.Item(1).Control().Type("c")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tagKeys": ["b", "c"], "people": [{"name": "Bob"}, {"name": "Cy"}]}, "submit": ""}`)
}

const covSeriesEchoSpec = `{fields: [{type: "series", label: "Chain", name: "chain", required: true, fields: [
	"(p) => ({type: 'text', label: 'First ' + (p === null ? 'null' : typeof p), name: 'first'})",
	"(p) => ({type: 'number', label: 'Echo ' + p, name: 'second'})",
	"(p) => ({type: 'select', label: 'Pick ' + (p + 1), name: 'third', options: [{value: {n: p}, label: 'N'}]})",
	"(p) => ({type: 'text', label: 'Got ' + JSON.stringify(p), name: 'fourth'})"
]}]}`

func TestCovSeriesStepsGetPreviousValue(t *testing.T) {
	a, f := contForm(t, covSeriesEchoSpec)
	chain := f.Series("Chain")
	chain.ExpectSteps(1)
	chain.Step(0).Field("First null").Type("ab")
	chain.ExpectSteps(2)
	chain.Step(1).Field("Echo ab").Type("4")
	chain.ExpectSteps(3)
	chain.Step(2).Field("Pick 5").Choose("N")
	chain.ExpectSteps(4)
	chain.Step(3).Field(`Got [{"value":{"n":4},"label":"N"}]`).Type("done")
	chain.ExpectCompleted(true)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"chain": {"first": "ab", "second": 4, "third": [{"value": {"n": 4}, "label": "N"}], "fourth": "done"}}, "submit": ""}`)
}

func TestCovSeriesChangingAnEarlierAnswerRebuildsLaterSteps(t *testing.T) {
	a, f := contForm(t, covSeriesEchoSpec)
	chain := f.Series("Chain")
	chain.Step(0).Field("First null").Type("ab")
	chain.Step(1).Field("Echo ab").Type("4")
	chain.Step(2).Field("Pick 5").Choose("N")
	chain.Step(3).Field(`Got [{"value":{"n":4},"label":"N"}]`).Type("done")
	chain.ExpectCompleted(true)
	chain.Step(1).Field("Echo ab").Type("6")
	chain.ExpectCompleted(false)
	chain.ExpectSteps(3)
	chain.Step(2).Field("Pick 7").ExpectValue("")
	chain.ExpectRequiredMark(true)
	chain.Step(0).Field("First null").Type("xy")
	chain.ExpectSteps(2)
	chain.Step(1).Field("Echo xy").ExpectValue("")
	chain.Step(0).Field("First null").Type("")
	chain.ExpectSteps(1)
	chain.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	chain.Step(0).Field("First null").Type("q")
	chain.Step(1).Field("Echo q").Type("1")
	chain.Step(2).Field("Pick 2").Choose("N")
	chain.Step(3).Field(`Got [{"value":{"n":1},"label":"N"}]`).Type("z")
	chain.ExpectCompleted(true)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"chain": {"first": "q", "second": 1, "third": [{"value": {"n": 1}, "label": "N"}], "fourth": "z"}}, "submit": ""}`)
}

const covSeriesRequiredSpec = `{fields: [{type: "series", label: "Order", name: "order", required: true, fields: [
	"(p) => ({type: 'text', label: 'Item', name: 'item', required: true})",
	"(p) => ({type: 'select', label: 'Size', name: 'size', min: 1, options: [{value: 's', label: 'S'}, {value: 'l', label: 'L'}]})",
	"(p) => ({type: 'select', label: 'Gift', name: 'gift', options: [{value: true, label: 'Yes'}, {value: false, label: 'No'}]})"
]}]}`

func TestCovSeriesRequiredMarksLegendUntilComplete(t *testing.T) {
	a, f := contForm(t, covSeriesRequiredSpec)
	order := f.Series("Order")
	order.ExpectRequiredMark(true)
	order.ExpectNoError()
	f.ExpectSubmitMessage("form is not completed or contains errors")
	order.Step(0).Field("Item").Type("tea")
	order.ExpectSteps(2)
	order.ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	order.Step(1).Field("Size").Choose("L")
	order.ExpectSteps(3)
	order.ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	order.Step(2).Field("Gift").Choose("No")
	order.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	order.ExpectCompletedText("Completed.")
	order.ExpectCompleted(true)
	order.ExpectNoError()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"order": {"item": "tea", "size": [{"value": "l", "label": "L"}], "gift": [{"value": false, "label": "No"}]}}, "submit": ""}`)
}

func TestCovSeriesRequiredSeriesRequiresEveryStep(t *testing.T) {
	_, f := contForm(t, covSeriesRequiredSpec)
	order := f.Series("Order")
	item := order.Step(0).Field("Item")
	item.ExpectRequiredMark(true)
	item.Find("input").WaitAttr("aria-required", "true")
	item.ExpectRequiredEmpty(true)
	item.ExpectInvalid(false)
	item.Type("tea")
	item.ExpectRequiredMark(false)
	item.ExpectRequiredEmpty(false)
	size := order.Step(1).Field("Size")
	size.ExpectRequiredMark(true)
	size.ExpectRequiredEmpty(true)
	size.WaitCount(`input[type="radio"]`, 2)
	size.Choose("S")
	size.ExpectRequiredEmpty(false)
	gift := order.Step(2).Field("Gift")
	gift.ExpectRequiredMark(true)
	gift.ExpectRequiredEmpty(true)
	gift.WaitCount(`input[type="radio"]`, 2)
	gift.Choose("Yes")
	order.ExpectCompleted(true)
}

const covSeriesOptionalSpec = `{fields: [{type: "series", label: "Extra", name: "extra", fields: [
	"(p) => ({type: 'text', label: 'Want', name: 'want'})",
	"(p) => ({type: 'text', label: 'What', name: 'what'})",
	"(p) => ({type: 'text', label: 'When', name: 'when'})"
]}]}`

func TestCovSeriesOptionalSkipsUntilStarted(t *testing.T) {
	a, f := contForm(t, covSeriesOptionalSpec)
	extra := f.Series("Extra")
	extra.ExpectSteps(1)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	extra.Step(0).Field("Want").Type("yes")
	extra.ExpectSteps(2)
	extra.ExpectRequiredMark(false)
	extra.Step(1).Field("What").ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	extra.Step(1).Field("What").Type("cake")
	extra.ExpectSteps(3)
	extra.ExpectRequiredMark(false)
	extra.Step(1).Field("What").Type("")
	extra.ExpectSteps(2)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(true)
	extra.Step(0).Field("Want").Type("")
	extra.ExpectSteps(1)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitMessage("")
	extra.Step(0).Field("Want").Type("yes")
	extra.Step(1).Field("What").Type("pie")
	extra.Step(2).Field("When").Type("now")
	extra.ExpectCompleted(true)
	extra.ExpectRequiredMark(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"extra": {"want": "yes", "what": "pie", "when": "now"}}, "submit": ""}`)
}

const covSeriesInListSpec = `{fields: [{type: "list", value: %s, field: {type: "series", label: "Pick", name: "pick", required: true, fields: [
	"(p) => ({type: 'select', label: 'Kind', name: 'kind', options: [{value: 'fruit', label: 'Fruit'}, {value: 'veg', label: 'Veg'}]})",
	"(p) => p[0].value === 'fruit' ? {type: 'select', label: 'Fruit', name: 'item', options: [{value: 'apple', label: 'Apple'}, {value: 'pear', label: 'Pear'}]} : {type: 'select', label: 'Veg', name: 'item', options: [{value: 'kale', label: 'Kale'}]}",
	"(p) => ({type: 'text', label: 'Note', name: 'note'})"
]}}]}`

func contSeriesInList(value string) string {
	return strings.Replace(covSeriesInListSpec, "%s", value, 1)
}

func TestCovSeriesDefaultsReplayFullAndPartial(t *testing.T) {
	a, f := contForm(t, contSeriesInList(`[
		{kind: [{value: "fruit", label: "Fruit"}], item: [{value: "pear", label: "Pear"}], note: "ripe"},
		{kind: [{value: "veg", label: "Veg"}]},
		{}
	]`))
	picks := f.List("Pick")
	picks.ExpectCount(3)
	full := picks.Item(0).Series("Pick")
	full.ExpectCompleted(true)
	full.ExpectRequiredMark(false)
	full.Step(1).Field("Fruit").ExpectValue("Pear")
	full.Step(2).Field("Note").ExpectValue("ripe")
	partial := picks.Item(1).Series("Pick")
	partial.ExpectSteps(2)
	partial.ExpectCompleted(false)
	partial.ExpectRequiredMark(true)
	partial.Step(0).Field("Kind").ExpectValue("Veg")
	partial.Step(1).Field("Veg").ExpectValue("")
	empty := picks.Item(2).Series("Pick")
	empty.ExpectSteps(1)
	empty.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	partial.Step(1).Field("Veg").Choose("Kale")
	partial.Step(2).Field("Note").Type("green")
	picks.Remove(2)
	picks.ExpectCount(2)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"pick": [
		{"kind": [{"value": "fruit", "label": "Fruit"}], "item": [{"value": "pear", "label": "Pear"}], "note": "ripe"},
		{"kind": [{"value": "veg", "label": "Veg"}], "item": [{"value": "kale", "label": "Kale"}], "note": "green"}
	]}, "submit": ""}`)
}

func TestCovSeriesWrongDefaultShapeFails(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"not an object", `["fruit"]`, `series "pick": default value is string, want an object`},
		{"first step wrong type", `[{kind: "fruit"}]`, `field "kind": default value is string, want an array`},
		{"later step wrong type", `[{kind: [{value: "fruit", label: "Fruit"}], item: [{value: "plum", label: "Plum"}]}]`, `is not one of the options`},
		{"last step wrong type", `[{kind: [{value: "veg", label: "Veg"}], item: [{value: "kale", label: "Kale"}], note: 3}]`, `field "note": default value is float64, want a string`},
	} {
		t.Run(c.name, func(t *testing.T) {
			contFails(t, contSeriesInList(c.value), c.want)
		})
	}
}

func TestCovSeriesStepFunctionFailingAtCreationFailsForm(t *testing.T) {
	t.Run("first step", func(t *testing.T) {
		contFails(t, `{fields: [{type: "series", label: "S", name: "s", fields: [
			"(p) => { throw new Error('boom first') }",
			"(p) => ({type: 'text', label: 'B', name: 'b'})"
		]}]}`, "boom first")
	})
	t.Run("step replayed from defaults", func(t *testing.T) {
		contFails(t, `{fields: [{type: "list", value: [{a: "x"}], field: {type: "series", label: "S", name: "s", fields: [
			"(p) => ({type: 'text', label: 'A', name: 'a'})",
			"(p) => { throw new Error('boom second') }"
		]}}]}`, "boom second")
	})
	t.Run("step returns a group", func(t *testing.T) {
		contFails(t, `{fields: [{type: "series", label: "S", name: "s", fields: [
			"(p) => ({type: 'group', label: 'G', name: 'g', fields: [{type: 'text', label: 'A', name: 'a'}]})",
			"(p) => ({type: 'text', label: 'B', name: 'b'})"
		]}]}`, "invalid input")
	})
}

const covSeriesFailLaterSpec = `{fields: [{type: "series", label: "S", name: "s", required: true, fields: [
	"(p) => ({type: 'select', label: 'Path', name: 'path', options: [{value: 'good', label: 'Good'}, {value: 'bad', label: 'Bad'}, {value: 'group', label: 'Group'}]})",
	"(p) => { if (p[0].value === 'bad') throw new Error('boom later'); if (p[0].value === 'group') return {type: 'list', field: {type: 'text', label: 'L', name: 'l'}}; return {type: 'text', label: 'Next', name: 'next'} }"
]}]}`

func TestCovSeriesStepFunctionFailingAfterCreationIsReported(t *testing.T) {
	a, f := contForm(t, covSeriesFailLaterSpec)
	s := f.Series("S")
	s.Step(0).Field("Path").Choose("Bad")
	if got := string(a.ChoiceReport("call_error")); !strings.Contains(got, "boom later") {
		t.Fatalf("the call_error reads %s, want the step's error", got)
	}
	s.ExpectSteps(1)
	f.ExpectSubmitDisabled(true)
	s.Step(0).Field("Path").Choose("Group")
	s.ExpectSteps(1)
	f.ExpectSubmitDisabled(true)
	s.Step(0).Field("Path").Choose("Good")
	s.Step(1).Field("Next").Type("on")
	s.ExpectCompleted(true)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"s": {"path": [{"value": "good", "label": "Good"}], "next": "on"}}, "submit": ""}`)
}

func TestCovNestListInGroup(t *testing.T) {
	a, f := contForm(t, `{fields: [{type: "group", label: "Team", name: "team", fields: [
		{type: "text", label: "Title", name: "title"},
		{type: "list", min: 2, max: 3, field: {type: "text", label: "Members", name: "members", maxLength: 3}}
	]}]}`)
	members := f.Group("Team").List("Members")
	members.ExpectCount(0)
	members.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	members.Add()
	members.Item(0).Control().Type("Ann")
	members.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	members.Add()
	members.Item(1).Control().Type("Robert")
	members.Item(1).Control().ExpectError("use at most 3 characters")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	members.Item(1).Control().Type("Bob")
	f.Group("Team").Field("Title").Type("Red")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"team": {"title": "Red", "members": ["Ann", "Bob"]}}, "submit": ""}`)
}

func TestCovNestSeriesInListItemsAreIndependent(t *testing.T) {
	a, f := contForm(t, contSeriesInList(`[]`))
	picks := f.List("Pick")
	picks.ExpectCount(0)
	f.ExpectSubmitDisabled(false)
	picks.Add()
	picks.Add()
	picks.ExpectCount(2)
	picks.ExpectItemName(0, "Pick, item 1")
	picks.ExpectItemName(1, "Pick, item 2")
	first := picks.Item(0).Series("Pick")
	second := picks.Item(1).Series("Pick")
	first.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	first.Step(0).Field("Kind").Choose("Fruit")
	first.ExpectSteps(2)
	second.ExpectSteps(1)
	second.Step(0).Field("Kind").Choose("Veg")
	second.Step(1).Field("Veg").Choose("Kale")
	second.Step(2).Field("Note").Type("green")
	second.ExpectCompleted(true)
	first.ExpectCompleted(false)
	f.ExpectSubmitDisabled(true)
	picks.Remove(0)
	picks.ExpectCount(1)
	picks.ExpectItemName(0, "Pick, item 1")
	picks.Item(0).Series("Pick").ExpectCompleted(true)
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"pick": [{"kind": [{"value": "veg", "label": "Veg"}], "item": [{"value": "kale", "label": "Kale"}], "note": "green"}]}, "submit": ""}`)
}

func TestCovNestListOfLists(t *testing.T) {
	a, f := contForm(t, `{fields: [{type: "list", value: [["a"], ["b", "c"]], field: {type: "list", min: 1, max: 2, field: {type: "text", label: "Tags", name: "tags"}}}]}`)
	outer := f.List("Tags")
	outer.ExpectCount(2)
	first := outer.Item(0).List("Tags")
	second := outer.Item(1).List("Tags")
	first.ExpectCount(1)
	second.ExpectCount(2)
	second.ExpectAddDisabled(true)
	second.Item(1).Control().ExpectValue("c")
	second.Item(1).Control().ExpectName("Tags, item 2")
	outer.ExpectRemoveName(1, "Remove item 2 of Tags")
	second.Remove(0)
	second.ExpectCount(1)
	second.Item(0).Control().ExpectValue("c")
	outer.Add()
	outer.ExpectCount(3)
	third := outer.Item(2).List("Tags")
	third.ExpectCount(0)
	third.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	third.Add()
	third.ExpectCount(1)
	third.Item(0).Control().ExpectValue("")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	third.Item(0).Control().Type("d")
	f.ExpectSubmitMessage("")
	outer.Remove(0)
	outer.ExpectCount(2)
	outer.Item(0).List("Tags").Item(0).Control().ExpectValue("c")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": [["c"], ["d"]]}, "submit": ""}`)
}

func TestCovNestOuterListCountBesideInnerList(t *testing.T) {
	_, f := contForm(t, `{fields: [{type: "list", min: 2, field: {type: "list", min: 1, field: {type: "text", label: "Tags", name: "tags"}}}]}`)
	outer := f.List("Tags")
	outer.ExpectCount(0)
	outer.Add()
	outer.ExpectCount(1)
	outer.Item(0).List("Tags").ExpectCount(0)
	outer.Item(0).List("Tags").ExpectRequiredMark(true)
	outer.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	outer.Item(0).List("Tags").Add()
	outer.Item(0).List("Tags").Item(0).Control().Type("a")
	outer.Item(0).List("Tags").ExpectRequiredMark(false)
	outer.ExpectError("add at least 2")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestCovNestListCountBesideSeriesItems(t *testing.T) {
	_, f := contForm(t, `{fields: [{type: "list", min: 2, field: {type: "series", label: "S", name: "s", required: true, fields: [
		"(p) => ({type: 'text', label: 'A', name: 'a'})",
		"(p) => ({type: 'text', label: 'B', name: 'b'})"
	]}}]}`)
	list := f.List("S")
	list.ExpectCount(0)
	list.Add()
	list.ExpectCount(1)
	list.ExpectError("add at least 2")
	series := list.Item(0).Series("S")
	series.ExpectRequiredMark(true)
	series.Step(0).Field("A").Type("1")
	series.Step(1).Field("B").Type("2")
	series.ExpectCompleted(true)
	list.ExpectError("add at least 2")
	f.ExpectSubmitDisabled(true)
}

func TestCovListCountHiddenWhenEmpty(t *testing.T) {
	_, f := contForm(t, covListBoundsSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.Find(".kisland-form-count").WaitText("")
	tags.Add()
	tags.Find(".kisland-form-count").WaitText("1 of 2–3")
	tags.Remove(0)
	tags.Find(".kisland-form-count").WaitText("")
}

func TestCovListMarkWhileAnyItemError(t *testing.T) {
	_, f := contForm(t, covListBoundsSpec)
	tags := f.List("Tags")
	tags.Add()
	tags.Add()
	tags.Item(0).Control().Type("a")
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)
	tags.Item(1).Control().Type("b")
	tags.ExpectRequiredMark(false)
}

func TestCovListMarkWhileGroupItemError(t *testing.T) {
	_, f := contForm(t, covListRequiredItemSpec)
	people := f.List("People")
	people.Add()
	people.Add()
	people.Item(0).Field("Name").Type("Ada")
	people.ExpectRequiredMark(true)
	people.Item(1).Field("Name").Type("Bob")
	people.ExpectRequiredMark(false)
}
