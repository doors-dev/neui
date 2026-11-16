//go:build browser

package devkit_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func formApp(t *testing.T, spec string) (*stack.IslandApp, stack.Form) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form(` + spec + `)); ui.notify(ui.draw())`})
	return a, a.Form()
}

const textSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "text", label: "Code", name: "code", minLength: 3, maxLength: 5},
	{type: "email", label: "Email", name: "email"},
	{type: "url", label: "Site", name: "site"},
	{type: "text", label: "Zip", name: "zip", pattern: "[0-9]{5}"},
	{type: "text", label: "Nick", name: "nick", validate: "(s) => s === 'root' ? 'taken' : ''"}
]}`

func TestFormText(t *testing.T) {
	a, f := formApp(t, textSpec)
	name := f.Field("Name")
	name.ExpectRequiredMark(true)
	name.Type("Ada")
	name.ExpectRequiredMark(false)
	name.Type("")
	name.ExpectRequiredMark(true)
	name.ExpectNoError()
	name.Type("Ada")

	code := f.Field("Code")
	code.ExpectRequiredMark(false)
	code.Type("ab")
	code.ExpectError("use at least 3 characters")
	code.ExpectInvalid(true)
	code.Type("abcdef")
	code.ExpectError("use at most 5 characters")
	code.Type("abcd")
	code.ExpectNoError()
	code.ExpectInvalid(false)

	email := f.Field("Email")
	email.Type("ada")
	email.ExpectError("enter an email address")
	email.Type("ada@example.com")
	email.ExpectNoError()

	site := f.Field("Site")
	site.Type("example.com")
	site.ExpectError("enter a full URL, like https://example.com")
	site.Type("")
	site.ExpectNoError()

	zip := f.Field("Zip")
	zip.Type("12a45")
	zip.ExpectError("use the requested format")
	zip.Type("12345")
	zip.ExpectNoError()

	nick := f.Field("Nick")
	nick.Type("root")
	nick.ExpectError("taken")
	nick.Type("ada")
	nick.ExpectNoError()

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ada", "code": "abcd", "email": "ada@example.com", "zip": "12345", "nick": "ada"}, "submit": ""}`)
}

const numberSpec = `{fields: [
	{type: "number", label: "Age", name: "age", min: 18, max: 99, step: 1},
	{type: "number", label: "Price", name: "price", step: 0.25},
	{type: "range", label: "Volume", name: "volume", min: 0, max: 10, step: 2},
	{type: "number", label: "Lucky", name: "lucky", validate: "(n) => n === 13 ? 'unlucky' : ''"}
]}`

func TestFormNumber(t *testing.T) {
	a, f := formApp(t, numberSpec)
	age := f.Field("Age")
	age.Type("17")
	age.ExpectError("enter 18 or more")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	age.Type("100")
	age.ExpectError("enter 99 or less")
	age.Type("20.5")
	age.ExpectError("use steps of 1")
	age.Type("30")
	age.ExpectNoError()

	price := f.Field("Price")
	price.Type("1.1")
	price.ExpectError("use steps of 0.25")
	price.Type("1.25")
	price.ExpectNoError()

	volume := f.Field("Volume")
	volume.ExpectValue("6")
	volume.Find("output").WaitText("6")
	volume.Type("8")
	volume.Find("output").WaitText("8")

	lucky := f.Field("Lucky")
	lucky.Type("13")
	lucky.ExpectError("unlucky")
	lucky.Type("")
	lucky.ExpectNoError()

	f.ExpectSubmitMessage("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"age": 30, "price": 1.25, "volume": 8}, "submit": ""}`)
}

const dateSpec = `{fields: [
	{type: "date", label: "Start", name: "start", required: true, min: "2026-01-01", max: "2026-12-31"},
	{type: "time", label: "When", name: "when", value: "09:30"},
	{type: "month", label: "Month", name: "month"}
]}`

func TestFormDate(t *testing.T) {
	a, f := formApp(t, dateSpec)
	start := f.Field("Start")
	start.ExpectRequiredMark(true)
	start.Type("2025-12-31")
	start.ExpectError("pick 2026-01-01 or later")
	start.ExpectRequiredMark(true)
	start.Type("2027-01-01")
	start.ExpectError("pick 2026-12-31 or earlier")
	start.ExpectRequiredMark(true)
	start.Type("2026-06-15")
	start.ExpectNoError()
	start.ExpectRequiredMark(false)
	f.Field("When").ExpectValue("09:30")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"start": "2026-06-15", "when": "09:30"}, "submit": ""}`)
}

const textareaSpec = `{fields: [
	{type: "textarea", label: "Bio", name: "bio", minLength: 5, maxLength: 20},
	{type: "textarea", label: "Notes", name: "notes", value: "line one\nline two"}
]}`

func TestFormTextarea(t *testing.T) {
	a, f := formApp(t, textareaSpec)
	bio := f.Field("Bio")
	count := bio.Find(".neisland-form-count")
	count.WaitText("0 / 5 min")
	bio.Type("abc")
	count.WaitText("3 / 5 min")
	bio.ExpectError("use at least 5 characters")
	bio.Type("hello world")
	count.WaitText("11 / 20")
	bio.ExpectNoError()
	bio.Type("this is longer than twenty")
	bio.ExpectError("use at most 20 characters")
	bio.Type("hello world")
	bio.ExpectNoError()
	bio.Find("textarea").WaitAttr("maxlength", "20")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"bio": "hello world", "notes": "line one\nline two"}, "submit": ""}`)
}

const selectSpec = `{fields: [
	{type: "select", label: "Size", name: "size", options: [{value: "s", label: "S"}, {value: "m", label: "M"}, {value: "l", label: "L"}]},
	{type: "select", label: "Plan", name: "plan", min: 1, options: [{value: "free", label: "Free"}, {value: {tier: "pro"}, label: "Pro"}]},
	{type: "select", label: "Country", name: "country", options: [
		{value: "ar", label: "Argentina"}, {value: "br", label: "Brazil"}, {value: "cl", label: "Chile"},
		{value: "co", label: "Colombia"}, {value: "pe", label: "Peru"}, {value: "uy", label: "Uruguay"}
	]},
	{type: "select", label: "Toppings", name: "toppings", min: 2, max: 3, options: [
		{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}, {value: "d", label: "D"}, {value: "e", label: "E"}
	]}
]}`

func TestFormSelect(t *testing.T) {
	a, f := formApp(t, selectSpec)
	size := f.Field("Size")
	size.WaitCount(`input[type="radio"]`, 4)
	size.ExpectRequiredMark(false)
	size.ExpectValue("")
	size.Choose("M")
	size.ExpectValue("M")
	size.Choose("None")
	size.ExpectValue("")

	plan := f.Field("Plan")
	plan.WaitCount(`input[type="radio"]`, 2)
	plan.ExpectRequiredMark(true)
	plan.Choose("Pro")
	plan.ExpectRequiredMark(false)

	country := f.Field("Country")
	country.WaitCount("select", 1)
	country.ExpectValue("")
	country.Choose("Peru")
	country.ExpectValue("Peru")

	toppings := f.Field("Toppings")
	toppings.WaitCount(`input[type="checkbox"]`, 5)
	toppings.ExpectRequiredMark(true)
	toppings.Choose("D")
	toppings.ExpectError("choose at least 2")
	toppings.ExpectInvalid(true)
	toppings.Choose("A")
	toppings.ExpectNoError()
	toppings.Choose("C")
	toppings.Choose("E")
	toppings.ExpectError("choose at most 3")
	toppings.Unchoose("E")
	toppings.ExpectNoError()
	toppings.ExpectValue("A, C, D")

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {
		"plan": [{"value": {"tier": "pro"}, "label": "Pro"}],
		"country": [{"value": "pe", "label": "Peru"}],
		"toppings": [{"value": "a", "label": "A"}, {"value": "c", "label": "C"}, {"value": "d", "label": "D"}]
	}, "submit": ""}`)
}

const checkboxSpec = `{fields: [
	{type: "checkbox", label: "I accept the terms", name: "terms", required: true},
	{type: "checkbox", label: "Send news", name: "news", checked: true}
]}`

func TestFormCheckbox(t *testing.T) {
	a, f := formApp(t, checkboxSpec)
	terms := f.Field("I accept the terms")
	terms.ExpectRequiredMark(true)
	terms.ExpectValue("false")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	terms.Check()
	terms.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	terms.Uncheck()
	terms.ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	terms.Check()
	news := f.Field("Send news")
	news.ExpectValue("true")
	news.Uncheck()
	news.ExpectValue("false")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"terms": true, "news": false}, "submit": ""}`)
}

const groupSpec = `{fields: [
	{type: "group", label: "Address", name: "address", fields: [
		{type: "text", label: "Street", name: "street", required: true},
		{type: "text", label: "City", name: "city"},
		{type: "group", label: "Geo", name: "geo", fields: [{type: "number", label: "Lat", name: "lat", min: -90, max: 90}]}
	]}
]}`

func TestFormGroup(t *testing.T) {
	a, f := formApp(t, groupSpec)
	address := f.Group("Address")
	address.Field("Street").ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	address.Field("Street").Type("Main 1")
	address.Field("City").Type("Lima")
	lat := address.Group("Geo").Field("Lat")
	lat.Type("100")
	lat.ExpectError("enter 90 or less")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	lat.Type("-12")
	lat.ExpectNoError()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"address": {"street": "Main 1", "city": "Lima", "geo": {"lat": -12}}}, "submit": ""}`)
}

const listSpec = `{fields: [
	{type: "list", min: 2, max: 3, field: {type: "text", label: "Tags", name: "tags", maxLength: 4}}
]}`

func TestFormList(t *testing.T) {
	a, f := formApp(t, listSpec)
	tags := f.List("Tags")
	tags.ExpectCount(0)
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.Add()
	tags.ExpectCount(2)
	tags.WaitCount("button.neisland-form-item-remove", 2)
	tags.Find("button.neisland-form-item-remove").WaitAttr("aria-label", "Remove item 1 of Tags")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.Item(0).Control().ExpectRequiredEmpty(true)
	tags.Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")

	tags.Item(0).Control().Type("go")
	tags.Remove(1)
	tags.ExpectCount(1)
	tags.ExpectError("add at least 2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)
	tags.Item(1).Control().ExpectRequiredEmpty(true)

	tags.Item(0).Control().Type("go")
	tags.Item(1).Control().Type("toolong")
	tags.Item(1).Control().ExpectError("use at most 4 characters")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(1).Control().Type("js")
	tags.ExpectRequiredMark(false)
	tags.ExpectAddDisabled(false)

	tags.Add()
	tags.ExpectCount(3)
	tags.ExpectAddDisabled(true)
	tags.Find(".neisland-form-items-foot .neisland-form-help").WaitText("Maximum of 3 reached.")
	tags.Add()
	tags.ExpectCount(3)
	tags.WaitCount("button.neisland-form-item-remove", 3)
	tags.Item(2).Control().Type("rs")
	tags.Remove(0)
	tags.ExpectCount(2)
	tags.ExpectAddDisabled(false)
	tags.Find("button.neisland-form-item-remove").WaitAttr("aria-label", "Remove item 1 of Tags")
	tags.Item(0).Control().ExpectValue("js")

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["js", "rs"]}, "submit": ""}`)
}

const listGroupsSpec = `{fields: [
	{type: "list", min: 1, field: {type: "group", label: "People", name: "people", fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "number", label: "Age", name: "age"}
	]}}
]}`

func TestFormListGroups(t *testing.T) {
	a, f := formApp(t, listGroupsSpec)
	people := f.List("People")
	people.ExpectCount(0)
	people.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	people.Add()
	people.ExpectCount(1)
	people.Find(".neisland-form-items-foot").WaitGone(".neisland-form-help")
	people.Item(0).Field("Name").ExpectRequiredMark(true)
	people.Item(0).Field("Name").Type("Ada")
	people.Item(0).Field("Age").Type("36")
	people.Add()
	people.ExpectCount(2)
	people.Item(1).Find(".neisland-form-item-body").WaitAttr("aria-label", "People, item 2")
	people.Item(1).Field("Name").ExpectRequiredMark(true)
	f.ExpectSubmitDisabled(true)
	people.Item(1).Field("Name").Type("Alan")
	people.Add()
	people.ExpectCount(3)
	f.ExpectSubmitDisabled(true)
	people.Remove(2)
	people.ExpectCount(2)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"people": [{"name": "Ada", "age": 36}, {"name": "Alan"}]}, "submit": ""}`)
}

const listKindsSpec = `{fields: [
	{type: "list", min: 2, field: {type: "text", label: "Texts", name: "texts", required: true}},
	{type: "list", min: 2, field: {type: "number", label: "Numbers", name: "numbers"}},
	{type: "list", min: 2, field: {type: "range", label: "Ranges", name: "ranges", min: 0, max: 10}},
	{type: "list", min: 2, field: {type: "date", label: "Dates", name: "dates"}},
	{type: "list", min: 2, field: {type: "textarea", label: "Notes", name: "notes"}},
	{type: "list", min: 2, field: {type: "select", label: "Radios", name: "radios", options: [{value: "a", label: "A"}, {value: "b", label: "B"}]}},
	{type: "list", min: 2, field: {type: "select", label: "Dropdowns", name: "dropdowns", options: [
		{value: 1, label: "One"}, {value: 2, label: "Two"}, {value: 3, label: "Three"}, {value: 4, label: "Four"}, {value: 5, label: "Five"}, {value: 6, label: "Six"}
	]}},
	{type: "list", min: 2, field: {type: "select", label: "Boxes", name: "boxes", max: 2, options: [{value: "x", label: "X"}, {value: "y", label: "Y"}]}},
	{type: "list", min: 2, field: {type: "search", label: "Finds", name: "finds", search: "(q) => [{value: 'kale', label: 'Kale'}]"}},
	{type: "list", min: 2, field: {type: "file", label: "Files", name: "files"}}
]}`

func TestFormListItemNames(t *testing.T) {
	_, f := formApp(t, listKindsSpec)
	for _, label := range []string{"Texts", "Numbers", "Ranges", "Dates", "Notes", "Radios", "Dropdowns", "Boxes", "Finds", "Files"} {
		list := f.List(label)
		list.ExpectCount(0)
		list.Add()
		list.Add()
		list.ExpectCount(2)
		list.Item(0).WaitGone(".neisland-form-label, legend, label.neisland-form-check > span")
		list.Item(0).Control().ExpectName(label + ", item 1")
		list.Item(1).Control().ExpectName(label + ", item 2")
		list.Remove(0)
		list.ExpectCount(1)
		list.Item(0).Control().ExpectName(label + ", item 1")
		list.Add()
		list.ExpectCount(2)
		list.Item(1).Control().ExpectName(label + ", item 2")
	}
	texts := f.List("Texts")
	texts.Item(0).Control().Type("a")
	texts.Item(0).Control().ExpectValue("a")
	f.List("Radios").Item(0).Control().Choose("B")
	f.List("Radios").Item(0).Control().ExpectValue("B")
	f.List("Dropdowns").Item(1).Control().Choose("Six")
	f.List("Dropdowns").Item(1).Control().ExpectValue("Six")
}

func TestFormListDefaults(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: ["go", "js"], field: {type: "text", label: "Tags", name: "tags"}}]}`)
	tags := f.List("Tags")
	tags.ExpectCount(2)
	tags.Item(0).Control().ExpectValue("go")
	tags.Item(1).Control().ExpectValue("js")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["go", "js"]}, "submit": ""}`)
}

func TestFormNumberFraction(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "number", label: "Weight", name: "weight"},
		{type: "number", label: "Offset", name: "offset", min: -1.5, max: 1.5}
	]}`)
	f.Field("Weight").Type("1.5")
	f.Field("Weight").ExpectNoError()
	f.Field("Offset").Type("0.25")
	f.Field("Offset").ExpectNoError()
	f.WaitCount(":invalid", 0)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"weight": 1.5, "offset": 0.25}, "submit": ""}`)
}

func TestFormRangeDefaultOnBrowserStep(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "range", label: "Ratio", name: "ratio", min: 0, max: 1},
		{type: "range", label: "Five", name: "five", min: 0, max: 5},
		{type: "range", label: "Below", name: "below", min: -5, max: 0}
	]}`)
	f.Field("Ratio").ExpectValue("1")
	f.Field("Ratio").Find("output").WaitText("1")
	f.Field("Five").ExpectValue("3")
	f.Field("Five").Find("output").WaitText("3")
	f.Field("Below").ExpectValue("-2")
	f.Field("Below").Find("output").WaitText("-2")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"ratio": 1, "five": 3, "below": -2}, "submit": ""}`)
}

const markJS = `() => {
	const box = this.getBoundingClientRect()
	const mark = getComputedStyle(this, "::after")
	return [Math.round(box.height), mark.content, mark.width, mark.backgroundImage.split("linear-gradient").length - 1, mark.rotate === "none" ? "straight" : "turned"].join(" ")
}`

func TestFormListRemoveShowsCross(t *testing.T) {
	_, f := formApp(t, listSpec)
	tags := f.List("Tags")
	tags.Add()
	tags.ExpectCount(1)
	if got := tags.Find("button.neisland-form-item-remove").Element().MustEval(markJS).String(); got != `24 "" 24px 2 turned` {
		t.Fatalf("the remove button draws %q, want a 24px mark of two turned bars", got)
	}
	if got := tags.Find("button.neisland-form-item-add").Element().MustEval(markJS).String(); got != `24 "" 24px 2 straight` {
		t.Fatalf("the add button draws %q, want a 24px mark of two straight bars", got)
	}
}

const markGapJS = `() => {
	let node = this
	while (!node.previousSibling) {
		node = node.parentNode
	}
	const range = document.createRange()
	range.selectNodeContents(node.previousSibling)
	const rects = range.getClientRects()
	return this.getBoundingClientRect().left - rects[rects.length - 1].right
}`

func TestFormRequiredMarkBesideLegend(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "list", min: 1, field: {type: "text", label: "Tags", name: "tags"}},
		{type: "series", label: "Pick", name: "pick", required: true, fields: ["(p) => ({type: 'text', label: 'A', name: 'a'})", "(p) => ({type: 'text', label: 'B', name: 'b'})"]}
	]}`)
	f.List("Tags").ExpectRequiredMark(true)
	f.Series("Pick").ExpectRequiredMark(true)
	field := f.Field("Name").Find(".neisland-form-required").Element().MustEval(markGapJS).Num()
	list := f.List("Tags").Find(":scope > legend .neisland-form-required").Element().MustEval(markGapJS).Num()
	series := f.Series("Pick").Find(":scope > legend .neisland-form-required").Element().MustEval(markGapJS).Num()
	if math.Abs(list-field) > 0.5 || math.Abs(series-field) > 0.5 {
		t.Fatalf("the * sits %.1fpx after a list legend and %.1fpx after a series legend, want %.1fpx as after a field label", list, series, field)
	}
}

func TestFormSeriesSearchStep(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "series", label: "Pick", name: "pick", required: true, fields: [
		"(p) => ({type: 'search', label: 'Veg', name: 'veg', max: 2, search: \"(q) => [{value: 'kale', label: 'Kale'}, {value: 'leek', label: 'Leek'}]\"})",
		"(p) => ({type: 'text', label: 'Note ' + p.length, name: 'note'})"
	]}]}`)
	pick := f.Series("Pick")
	veg := pick.Step(0).Field("Veg")
	pick.ExpectSteps(1)
	for range 3 {
		veg.Type("k")
		veg.Find(`button[aria-label="Choose Kale"]`).Click()
		pick.Step(1).Field("Note 1").ExpectValue("")
		veg.Find(`button[aria-label="Remove Kale"]`).Click()
		pick.ExpectSteps(1)
	}
	veg.Type("k")
	veg.Find(`button[aria-label="Choose Kale"]`).Click()
	pick.Step(1).Field("Note 1").ExpectValue("")
	veg.Type("l")
	veg.Find(`button[aria-label="Choose Leek"]`).Click()
	pick.Step(1).Field("Note 2").ExpectValue("")
	veg.Find(`button[aria-label="Remove Kale"]`).Click()
	pick.Step(1).Field("Note 1").Type("fresh")
	pick.ExpectCompleted(true)
	pick.ExpectNoError()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"pick": {"veg": [{"value": "leek", "label": "Leek"}], "note": "fresh"}}, "submit": ""}`)
}

func TestFormListOfGroupsWithLists(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", min: 1, max: 3, field: {type: "group", label: "Teams", name: "teams", fields: [
		{type: "text", label: "Team", name: "team"},
		{type: "list", value: ["Ann"], field: {type: "text", label: "Members", name: "members"}}
	]}}]}`)
	teams := f.List("Teams")
	teams.ExpectCount(0)
	teams.ExpectRequiredMark(true)
	teams.Add()
	teams.ExpectCount(1)
	teams.Add()
	teams.ExpectCount(2)
	teams.Item(0).List("Members").ExpectCount(1)
	teams.Item(1).List("Members").ExpectCount(1)
	teams.Item(0).List("Members").Add()
	teams.Item(0).List("Members").ExpectCount(2)
	teams.Item(1).List("Members").ExpectCount(1)
	teams.ExpectCount(2)
	teams.Item(0).Field("Team").Type("Red")
	teams.Item(0).List("Members").Item(1).Control().Type("Bob")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"teams": [{"team": "Red", "members": ["Ann", "Bob"]}, {"members": ["Ann"]}]}, "submit": ""}`)
}

const seriesSpec = `{fields: [
	{type: "series", label: "Pick", name: "pick", required: true, fields: [
		"(p) => ({type: 'select', label: 'Kind', name: 'kind', options: [{value: 'fruit', label: 'Fruit'}, {value: 'veg', label: 'Veg'}]})",
		"(p) => p[0].value === 'fruit' ? {type: 'select', label: 'Fruit', name: 'item', options: [{value: 'apple', label: 'Apple'}, {value: 'pear', label: 'Pear'}]} : {type: 'select', label: 'Veg', name: 'item', options: [{value: 'kale', label: 'Kale'}]}",
		"(p) => ({type: 'text', label: 'Note', name: 'note'})"
	]}
]}`

func TestFormSeries(t *testing.T) {
	a, f := formApp(t, seriesSpec)
	pick := f.Series("Pick")
	pick.ExpectSteps(1)
	pick.ExpectRequiredMark(true)
	pick.ExpectCompleted(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	pick.Step(0).Field("Kind").ExpectRequiredMark(true)
	pick.Step(0).Field("Kind").ExpectRequiredEmpty(true)

	pick.Step(0).Field("Kind").Choose("Fruit")
	pick.ExpectSteps(2)
	pick.Step(1).Field("Fruit").Choose("Apple")
	pick.ExpectSteps(3)
	pick.Step(2).Field("Note").Type("ripe")
	pick.ExpectCompleted(true)
	pick.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)

	pick.Step(0).Field("Kind").Choose("Veg")
	pick.ExpectCompleted(false)
	pick.ExpectSteps(2)
	pick.Step(1).Field("Veg").Choose("Kale")
	pick.ExpectSteps(3)
	pick.Step(2).Field("Note").ExpectValue("")
	f.ExpectSubmitDisabled(true)
	pick.Step(2).Field("Note").Type("green")
	pick.ExpectCompleted(true)

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"pick": {
		"kind": [{"value": "veg", "label": "Veg"}],
		"item": [{"value": "kale", "label": "Kale"}],
		"note": "green"
	}}, "submit": ""}`)
}

const optionalSeriesSpec = `{fields: [
	{type: "series", label: "Extra", name: "extra", fields: [
		"(p) => ({type: 'select', label: 'Want', name: 'want', options: [{value: 'yes', label: 'Yes'}]})",
		"(p) => ({type: 'text', label: 'What', name: 'what'})"
	]}
]}`

func TestFormSeriesOptional(t *testing.T) {
	a, f := formApp(t, optionalSeriesSpec)
	extra := f.Series("Extra")
	extra.ExpectSteps(1)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	extra.Step(0).Field("Want").Choose("Yes")
	extra.ExpectSteps(2)
	f.ExpectSubmitDisabled(true)
	extra.Step(0).Field("Want").Choose("None")
	extra.ExpectSteps(1)
	f.ExpectSubmitDisabled(false)
	extra.Step(0).Field("Want").Choose("Yes")
	extra.Step(1).Field("What").Type("cake")
	extra.ExpectCompleted(true)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"extra": {"want": [{"value": "yes", "label": "Yes"}], "what": "cake"}}, "submit": ""}`)
}

func TestFormSeriesOptionalStartedMarksRequired(t *testing.T) {
	_, f := formApp(t, optionalSeriesSpec)
	f.Series("Extra").Step(0).Field("Want").Choose("Yes")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Series("Extra").Step(1).Field("What").ExpectRequiredMark(true)
	f.Series("Extra").ExpectRequiredMark(false)
}

const gatingSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "number", label: "Count", name: "count", min: 1}
]}`

func TestFormSubmitGating(t *testing.T) {
	a, f := formApp(t, gatingSpec)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Count").Type("0")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Count").Type("2")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectSubmitDisabled(true)
	f.Field("Name").Type("Ada")
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ada", "count": 2}, "submit": ""}`)
}

const submitsSpec = `{fields: [{type: "text", label: "Title", name: "title"}],
	submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish", style: "success"}]}`

func TestFormSubmits(t *testing.T) {
	a, f := formApp(t, submitsSpec)
	f.Find(".neisland-form-actions").WaitCount("button[type=submit]", 2)
	f.Field("Title").Type("Hi")
	f.Submit("Publish")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"title": "Hi"}, "submit": "publish"}`)
}

func TestFormFile(t *testing.T) {
	dir := t.TempDir()
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Script: `ui.root.set(ui.form({fields: [
		{type: "file", label: "Doc", name: "doc", min: 1, accept: ".txt"}
	]})); ui.notify(ui.draw())`})
	f := a.Form()
	src := t.TempDir()
	note := filepath.Join(src, "note.txt")
	image := filepath.Join(src, "image.png")
	if err := os.WriteFile(note, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := f.Field("Doc")
	doc.ExpectRequiredMark(true)
	doc.ExpectValue("")
	doc.SetFiles(image)
	doc.ExpectFileFailed("image.png", "not .txt")
	doc.ExpectNoError()
	f.ExpectSubmitDisabled(true)
	doc.SetFiles(note)
	doc.ExpectValue("note.txt")
	doc.Find(".neisland-form-file-size").WaitText("5 B")
	f.Submit("Submit")
	var got struct {
		Data struct{ Doc []string }
	}
	if err := json.Unmarshal(a.Submitted(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Doc) != 1 || filepath.Base(got.Data.Doc[0]) != "note.txt" || filepath.Dir(filepath.Dir(got.Data.Doc[0])) != "board.island" {
		t.Fatalf("the file field gave %q, want [island/<id>/note.txt]", got.Data.Doc)
	}
	saved, err := os.ReadFile(filepath.Join(dir, got.Data.Doc[0]))
	if err != nil || string(saved) != "hello" {
		t.Fatalf("the workspace has %q (%v), want hello", saved, err)
	}
}

func TestFormEditsWhileASubmitIsHeldReachTheNextSubmit(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var f = ui.form({fields: [
			{type: "text", label: "Name", name: "name", required: true},
			{type: "checkbox", label: "Agree", name: "agree"},
			{type: "select", label: "Size", name: "size", options: [{value: "s", label: "S"}, {value: "l", label: "L"}]},
			{type: "list", field: {type: "text", label: "Tags", name: "tags"}}
		]})
		ui.root.set(f)
		var ev = ui.draw()
		var t0 = Date.now()
		while (Date.now() - t0 < 3000) {}
		ui.notify(ev)
		ui.notify(ui.draw(1500))
		ui.notify(ui.draw())`})
	f := a.Form()
	f.Field("Name").Type("Ann")
	f.Submit("Submit")
	f.Field("Name").Type("Bea")
	f.Field("Agree").Check()
	f.Field("Size").Choose("L")
	f.List("Tags").Add()
	f.List("Tags").Item(0).Control().Type("t")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ann", "agree": false, "tags": []}, "submit": ""}`)
	stack.ExpectJSON(t, a.Notified().Data, `[null]`)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Bea", "agree": true, "size": [{"value": "l", "label": "L"}], "tags": ["t"]}, "submit": ""}`)
}

func TestFormSubmitRightAfterTypingSendsTheTypedText(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "text", label: "Name", name: "name"}, {type: "textarea", label: "Notes", name: "notes"}]}`)
	f.Field("Name").Type("Ann")
	f.Field("Notes").Type("typed last")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ann", "notes": "typed last"}, "submit": ""}`)
}
