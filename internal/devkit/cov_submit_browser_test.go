//go:build browser

package devkit_test

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func covSubmitApp(t *testing.T, dir string, spec string) (*stack.IslandApp, stack.Form) {
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Script: `ui.root.set(ui.form(` + spec + `)); while (true) ui.notify(ui.draw())`})
	return a, a.Form()
}

func covSubmitted(a *stack.IslandApp) json.RawMessage {
	got := a.Submitted()
	a.Settle()
	return got
}

func covSubmitFile(t *testing.T, dir, name, content string) string {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func covSubmitPick(search stack.Field, query, option string) {
	search.Type(query)
	search.Find(`button[aria-label="Choose ` + option + `"]`).Click()
	search.Find(`button[aria-label="Remove ` + option + `"]`).Element()
}

const covSubmitRequiredSpec = `{fields: [
	{type: "text", label: "Text", name: "text", required: true},
	{type: "number", label: "Number", name: "number", required: true},
	{type: "date", label: "Date", name: "date", required: true},
	{type: "textarea", label: "Notes", name: "notes", required: true},
	{type: "select", label: "Boxes", name: "boxes", min: 1, max: 2, options: [{value: "a", label: "A"}, {value: "b", label: "B"}]},
	{type: "search", label: "Find", name: "find", min: 1, search: "(q) => [{value: 'kale', label: 'Kale'}]"},
	{type: "checkbox", label: "Agree", name: "agree", required: true},
	{type: "list", min: 1, field: {type: "text", label: "Tags", name: "tags"}},
	{type: "group", label: "Address", name: "address", fields: [{type: "text", label: "City", name: "city", required: true}]},
	{type: "series", label: "Pick", name: "pick", required: true, fields: [
		"(p) => ({type: 'select', label: 'Kind', name: 'kind', max: 2, options: [{value: 'x', label: 'X'}, {value: 'z', label: 'Z'}]})",
		"(p) => ({type: 'text', label: 'Why', name: 'why'})"
	]},
	{type: "file", label: "Doc", name: "doc", min: 1}
]}`

func TestSubmitDisabledWhileAnyRequiredFieldIsEmpty(t *testing.T) {
	src := t.TempDir()
	note := covSubmitFile(t, src, "note.txt", "hi")
	a, f := covSubmitApp(t, t.TempDir(), covSubmitRequiredSpec)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")

	f.Field("Text").Type("a")
	f.Field("Number").Type("1")
	f.Field("Date").Type("2026-01-02")
	f.Field("Notes").Type("n")
	f.Field("Boxes").Choose("B")
	covSubmitPick(f.Field("Find"), "k", "Kale")
	f.Field("Agree").Check()
	f.List("Tags").ExpectCount(0)
	f.List("Tags").ExpectRequiredMark(true)
	f.List("Tags").Add()
	f.List("Tags").Item(0).Control().Type("t")
	f.Group("Address").Field("City").Type("Lima")
	f.Series("Pick").Step(0).Field("Kind").Choose("X")
	f.Series("Pick").Step(1).Field("Why").Type("y")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Doc").SetFiles(note)
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)

	empty := []struct {
		what  string
		clear func()
		fill  func()
	}{
		{"text", func() { f.Field("Text").Type("") }, func() { f.Field("Text").Type("a") }},
		{"number", func() { f.Field("Number").Type("") }, func() { f.Field("Number").Type("1") }},
		{"date", func() { f.Field("Date").Type("") }, func() { f.Field("Date").Type("2026-01-02") }},
		{"textarea", func() { f.Field("Notes").Type("") }, func() { f.Field("Notes").Type("n") }},
		{"select", func() { f.Field("Boxes").Unchoose("B") }, func() { f.Field("Boxes").Choose("B") }},
		{"search", func() { f.Field("Find").Find(`button[aria-label="Remove Kale"]`).Click() }, func() { covSubmitPick(f.Field("Find"), "ka", "Kale") }},
		{"checkbox", func() { f.Field("Agree").Uncheck() }, func() { f.Field("Agree").Check() }},
		{"list item", func() { f.List("Tags").Item(0).Control().Type("") }, func() { f.List("Tags").Item(0).Control().Type("t") }},
		{"group field", func() { f.Group("Address").Field("City").Type("") }, func() { f.Group("Address").Field("City").Type("Lima") }},
		{"series step", func() { f.Series("Pick").Step(1).Field("Why").Type("") }, func() { f.Series("Pick").Step(1).Field("Why").Type("y") }},
		{"file", func() { f.Field("Doc").RemoveFile("note.txt") }, func() { f.Field("Doc").SetFiles(note) }},
		{"series first step", func() { f.Series("Pick").Step(0).Field("Kind").Unchoose("X") }, func() {
			f.Series("Pick").Step(0).Field("Kind").Choose("X")
			f.Series("Pick").Step(1).Field("Why").Type("y")
		}},
	}
	for _, e := range empty {
		e.clear()
		f.ExpectSubmitDisabled(true)
		f.ExpectSubmitMessage("form is not completed or contains errors")
		e.fill()
		f.ExpectSubmitMessage("")
		f.ExpectSubmitDisabled(false)
	}
	f.Submit("Submit")
	var got struct {
		Data map[string]any
	}
	if err := json.Unmarshal(covSubmitted(a), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 11 {
		t.Fatalf("submitted %v, want all 11 fields", got.Data)
	}
}

const covSubmitErrorsSpec = `{fields: [
	{type: "text", label: "Code", name: "code", minLength: 3, maxLength: 5},
	{type: "text", label: "Zip", name: "zip", pattern: "[0-9]{5}"},
	{type: "email", label: "Email", name: "email"},
	{type: "url", label: "Site", name: "site"},
	{type: "text", label: "Nick", name: "nick", validate: "(s) => s === 'root' ? 'taken' : ''"},
	{type: "number", label: "Age", name: "age", min: 18, max: 99, step: 1, validate: "(n) => n === 50 ? 'not fifty' : ''"},
	{type: "date", label: "Start", name: "start", min: "2026-01-01", max: "2026-12-31", validate: "(s) => s === '2026-06-06' ? 'closed' : ''"},
	{type: "textarea", label: "Bio", name: "bio", minLength: 4, maxLength: 8, validate: "(s) => s === 'nope' ? 'no nope' : ''"},
	{type: "select", label: "Toppings", name: "toppings", min: 2, max: 3, value: [{value: "a", label: "A"}, {value: "b", label: "B"}], options: [{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}, {value: "d", label: "D"}]},
	{type: "search", label: "Veg", name: "veg", min: 2, max: 3, value: [{value: "kale", label: "Kale"}, {value: "leek", label: "Leek"}], search: "(q) => [{value: 'kale', label: 'Kale'}, {value: 'leek', label: 'Leek'}]"},
	{type: "file", label: "Docs", name: "docs", accept: ".txt", max: 10},
	{type: "list", min: 2, value: ["x", "y"], field: {type: "text", label: "Tags", name: "tags", maxLength: 3}},
	{type: "group", label: "Geo", name: "geo", fields: [{type: "number", label: "Lat", name: "lat", min: -90, max: 90}]},
	{type: "series", label: "Pick", name: "pick", fields: [
		"(p) => ({type: 'select', label: 'Kind', name: 'kind', options: [{value: 'x', label: 'X'}]})",
		"(p) => ({type: 'number', label: 'Qty', name: 'qty', min: 1})"
	]}
]}`

func TestSubmitDisabledWhileAnyFieldShowsAnError(t *testing.T) {
	src := t.TempDir()
	image := covSubmitFile(t, src, "image.png", "png")
	a, f := covSubmitApp(t, t.TempDir(), covSubmitErrorsSpec)
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)

	shown := []struct {
		what  string
		owner stack.Locator
		err   string
		spoil func()
		fix   func()
	}{
		{"text min length", f.Field("Code").Locator, "use at least 3 characters", func() { f.Field("Code").Type("ab") }, func() { f.Field("Code").Type("") }},
		{"text max length", f.Field("Code").Locator, "use at most 5 characters", func() { f.Field("Code").Type("abcdef") }, func() { f.Field("Code").Type("abc") }},
		{"text pattern", f.Field("Zip").Locator, "use the requested format", func() { f.Field("Zip").Type("1a") }, func() { f.Field("Zip").Type("12345") }},
		{"email", f.Field("Email").Locator, "enter an email address", func() { f.Field("Email").Type("ada") }, func() { f.Field("Email").Type("") }},
		{"url", f.Field("Site").Locator, "enter a full URL, like https://example.com", func() { f.Field("Site").Type("example.com") }, func() { f.Field("Site").Type("https://example.com") }},
		{"text validate", f.Field("Nick").Locator, "taken", func() { f.Field("Nick").Type("root") }, func() { f.Field("Nick").Type("ada") }},
		{"number min", f.Field("Age").Locator, "enter 18 or more", func() { f.Field("Age").Type("17") }, func() { f.Field("Age").Type("") }},
		{"number max", f.Field("Age").Locator, "enter 99 or less", func() { f.Field("Age").Type("100") }, func() { f.Field("Age").Type("30") }},
		{"number step", f.Field("Age").Locator, "use steps of 1", func() { f.Field("Age").Type("20.5") }, func() { f.Field("Age").Type("30") }},
		{"number validate", f.Field("Age").Locator, "not fifty", func() { f.Field("Age").Type("50") }, func() { f.Field("Age").Type("30") }},
		{"date min", f.Field("Start").Locator, "pick 2026-01-01 or later", func() { f.Field("Start").Type("2025-12-31") }, func() { f.Field("Start").Type("") }},
		{"date max", f.Field("Start").Locator, "pick 2026-12-31 or earlier", func() { f.Field("Start").Type("2027-01-01") }, func() { f.Field("Start").Type("2026-03-03") }},
		{"date validate", f.Field("Start").Locator, "closed", func() { f.Field("Start").Type("2026-06-06") }, func() { f.Field("Start").Type("2026-03-03") }},
		{"textarea min length", f.Field("Bio").Locator, "use at least 4 characters", func() { f.Field("Bio").Type("abc") }, func() { f.Field("Bio").Type("") }},
		{"textarea max length", f.Field("Bio").Locator, "use at most 8 characters", func() { f.Field("Bio").Type("abcdefghi") }, func() { f.Field("Bio").Type("abcd") }},
		{"textarea validate", f.Field("Bio").Locator, "no nope", func() { f.Field("Bio").Type("nope") }, func() { f.Field("Bio").Type("abcd") }},
		{"select below min", f.Field("Toppings").Locator, "choose at least 2", func() { f.Field("Toppings").Unchoose("B") }, func() { f.Field("Toppings").Choose("B") }},
		{"select above max", f.Field("Toppings").Locator, "choose at most 3", func() {
			f.Field("Toppings").Choose("C")
			f.Field("Toppings").Choose("D")
		}, func() { f.Field("Toppings").Unchoose("D") }},
		{"search below min", f.Field("Veg").Locator, "choose at least 2", func() { f.Field("Veg").Find(`button[aria-label="Remove Leek"]`).Click() }, func() { covSubmitPick(f.Field("Veg"), "l", "Leek") }},
		{"list below min", f.List("Tags").Locator, "add at least 2", func() { f.List("Tags").Remove(1) }, func() {
			f.List("Tags").Add()
			f.List("Tags").Item(1).Control().Type("y")
		}},
		{"list item", f.List("Tags").Item(1).Control().Locator, "use at most 3 characters", func() { f.List("Tags").Item(1).Control().Type("long") }, func() { f.List("Tags").Item(1).Control().Type("y") }},
		{"group field", f.Group("Geo").Field("Lat").Locator, "enter 90 or less", func() { f.Group("Geo").Field("Lat").Type("91") }, func() { f.Group("Geo").Field("Lat").Type("1") }},
		{"series step", f.Series("Pick").Step(1).Field("Qty").Locator, "enter 1 or more", func() {
			f.Series("Pick").Step(0).Field("Kind").Choose("X")
			f.Series("Pick").Step(1).Field("Qty").Type("0")
		}, func() { f.Series("Pick").Step(1).Field("Qty").Type("2") }},
	}
	f.Field("Docs").SetFiles(image)
	f.Field("Docs").ExpectFileFailed("image.png", "not .txt")
	f.Field("Docs").ExpectInvalid(false)
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
	for _, s := range shown {
		s.spoil()
		s.owner.ExpectError(s.err)
		f.ExpectSubmitDisabled(true)
		f.ExpectSubmitMessage("form is not completed or contains errors")
		s.fix()
		s.owner.ExpectNoError()
		f.ExpectSubmitMessage("")
		f.ExpectSubmitDisabled(false)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {
		"code": "abc", "zip": "12345", "site": "https://example.com", "nick": "ada", "age": 30, "start": "2026-03-03", "bio": "abcd",
		"toppings": [{"value": "a", "label": "A"}, {"value": "b", "label": "B"}, {"value": "c", "label": "C"}],
		"veg": [{"value": "kale", "label": "Kale"}, {"value": "leek", "label": "Leek"}],
		"tags": ["x", "y"], "geo": {"lat": 1}, "pick": {"kind": [{"value": "x", "label": "X"}], "qty": 2}
	}, "submit": ""}`)
}

func TestSubmitMessagePrefersErrorsOverMissingRequired(t *testing.T) {
	_, f := covSubmitApp(t, "", `{fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "number", label: "Count", name: "count", min: 1},
		{type: "group", label: "Inner", name: "inner", fields: [{type: "text", label: "Code", name: "code", minLength: 2}]}
	]}`)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Count").Type("0")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Group("Inner").Field("Code").Type("a")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Count").Type("1")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Group("Inner").Field("Code").Type("ab")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectSubmitDisabled(true)
	f.Field("Name").Type("Ada")
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
	f.Field("Name").Type("")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectSubmitDisabled(true)
}

func TestSubmitMessageNamesTheButtons(t *testing.T) {
	_, f := covSubmitApp(t, "", `{fields: [{type: "text", label: "Name", name: "name", required: true}], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}]}`)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	id := f.Find(".neisland-form-help").Element().MustAttribute("id")
	for _, b := range []string{"save", "publish"} {
		f.Find(`button[data-submit="`+b+`"]`).WaitAttr("aria-describedby", *id)
	}
}

func TestSubmitFromScriptWhileInvalidIsDropped(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "number", label: "Count", name: "count", min: 1}
	], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}]}`)
	f.ExpectSubmitDisabled(true)
	f.RequestSubmit()
	a.Settle()
	f.Field("Name").Type("Ada")
	f.Field("Count").Type("0")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.RequestSubmit()
	a.Settle()
	f.Field("Count").Type("2")
	f.ExpectSubmitDisabled(false)
	f.RequestSubmit()
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"name": "Ada", "count": 2}, "submit": "save"}`)
}

func TestSubmitButtonsKeepOrderValuesAndStyles(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [{type: "text", label: "Title", name: "title", value: "Hi"}], submits: [
		{text: "Draft"},
		{text: "Save", value: "save"},
		{text: "Delete", value: "delete", style: "danger"},
		{text: "Ship", value: "ship", style: "primary"},
		{text: "Keep", value: "keep", style: "success"}
	]}`)
	got := f.SubmitButtons()
	want := []stack.SubmitButton{
		{Text: "Draft", Value: "", Class: "neisland-button neisland-button-md neisland-button-primary"},
		{Text: "Save", Value: "save", Class: "neisland-button neisland-button-md neisland-button-secondary"},
		{Text: "Delete", Value: "delete", Class: "neisland-button neisland-button-md neisland-button-danger"},
		{Text: "Ship", Value: "ship", Class: "neisland-button neisland-button-md neisland-button-primary"},
		{Text: "Keep", Value: "keep", Class: "neisland-button neisland-button-md neisland-button-success"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the submit buttons are %+v, want %+v", got, want)
	}
	for _, b := range []struct{ text, value string }{{"Delete", "delete"}, {"Draft", ""}, {"Keep", "keep"}, {"Save", "save"}, {"Ship", "ship"}} {
		f.Submit(b.text)
		stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi"}, "submit": "`+b.value+`"}`)
	}
}

func TestSubmitOmittedGivesOnePrimarySubmit(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [{type: "text", label: "Title", name: "title", value: "Hi"}]}`)
	got := f.SubmitButtons()
	want := []stack.SubmitButton{{Text: "Submit", Value: "", Class: "neisland-button neisland-button-md neisland-button-primary"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the submit buttons are %+v, want %+v", got, want)
	}
	if texts := f.ActionTexts(); !reflect.DeepEqual(texts, []string{"Submit"}) {
		t.Fatalf("the actions are %q, want only Submit and no cancel", texts)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi"}, "submit": ""}`)
}

const covSubmitEnterSpec = `{fields: [
	{type: "text", label: "Title", name: "title"},
	{type: "search", label: "Veg", name: "veg", search: "(q) => [{value: 'kale', label: 'Kale'}]"}
], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}]}`

func TestSubmitEnterInTextPressesTheFirstButton(t *testing.T) {
	a, f := covSubmitApp(t, "", covSubmitEnterSpec)
	f.Field("Title").Type("Hi")
	f.Field("Title").PressEnter()
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi"}, "submit": "save"}`)
	f.Submit("Publish")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi"}, "submit": "publish"}`)
	f.Field("Title").Type("Bye")
	f.Field("Title").PressEnter()
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Bye"}, "submit": "save"}`)
}

func TestSubmitEnterDoesNothingWhileInvalid(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [
		{type: "text", label: "Title", name: "title", required: true},
		{type: "text", label: "Code", name: "code", minLength: 3}
	], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}]}`)
	f.Field("Code").Type("abc")
	f.Field("Code").PressEnter()
	f.Field("Title").Type("Hi")
	f.Field("Code").Type("a")
	f.Field("Code").ExpectError("use at least 3 characters")
	f.Field("Code").PressEnter()
	f.Field("Code").Type("abcd")
	f.Submit("Publish")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi", "code": "abcd"}, "submit": "publish"}`)
}

func TestSubmitEnterInSearchDoesNotSubmit(t *testing.T) {
	a, f := covSubmitApp(t, "", covSubmitEnterSpec)
	f.Field("Title").Type("Hi")
	veg := f.Field("Veg")
	veg.Type("ka")
	veg.Find(`button[aria-label="Choose Kale"]`).Element()
	veg.PressEnter()
	veg.Find(`button[aria-label="Choose Kale"]`).Click()
	veg.Find(`button[aria-label="Remove Kale"]`).Element()
	f.Submit("Publish")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Hi", "veg": [{"value": "kale", "label": "Kale"}]}, "submit": "publish"}`)
}

func TestCancelSkipsValidation(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "number", label: "Count", name: "count", min: 1}
	], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}], cancel: "Never mind"}`)
	if texts := f.ActionTexts(); !reflect.DeepEqual(texts, []string{"Save", "Publish", "Never mind"}) {
		t.Fatalf("the actions are %q, want the cancel after the submits", texts)
	}
	f.Find(".neisland-form-actions button[type=button]").WaitAttr("class", "neisland-form-cancel")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectCancelEnabled()
	f.Cancel()
	stack.ExpectJSON(t, covSubmitted(a), `{"canceled": true}`)
	f.Field("Count").Type("0")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectCancelEnabled()
	f.Cancel()
	stack.ExpectJSON(t, covSubmitted(a), `{"canceled": true}`)
	f.Field("Name").Type("Ada")
	f.Field("Count").Type("3")
	f.ExpectSubmitDisabled(false)
	f.Cancel()
	stack.ExpectJSON(t, covSubmitted(a), `{"canceled": true}`)
}

const covSubmitShapesSpec = `{fields: [
	{type: "text", label: "Text", name: "text"},
	{type: "email", label: "Email", name: "email"},
	{type: "tel", label: "Tel", name: "tel"},
	{type: "url", label: "Url", name: "url"},
	{type: "password", label: "Password", name: "password"},
	{type: "number", label: "Number", name: "number"},
	{type: "range", label: "Range", name: "range", min: 0, max: 10, step: 1},
	{type: "date", label: "Date", name: "date"},
	{type: "time", label: "Time", name: "time"},
	{type: "datetime-local", label: "Moment", name: "moment"},
	{type: "month", label: "Month", name: "month"},
	{type: "week", label: "Week", name: "week"},
	{type: "textarea", label: "Notes", name: "notes"},
	{type: "select", label: "Radio", name: "radio", options: [{value: 1, label: "One"}, {value: true, label: "Yes"}]},
	{type: "select", label: "Dropdown", name: "dropdown", options: [
		{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"},
		{value: "d", label: "D"}, {value: "e", label: "E"}, {value: {deep: [1, 2]}, label: "Deep"}
	]},
	{type: "select", label: "Boxes", name: "boxes", max: 3, options: [{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}, {value: "d", label: "D"}]},
	{type: "search", label: "Find", name: "find", max: 3, search: "(q) => [{value: 'kale', label: 'Kale'}, {value: 2, label: 'Leek'}, {value: {k: 'okra'}, label: 'Okra'}]"},
	{type: "checkbox", label: "Agree", name: "agree"},
	{type: "checkbox", label: "Spam", name: "spam", checked: true},
	{type: "file", label: "Doc", name: "doc"},
	{type: "file", label: "Pics", name: "pics", max: 10},
	{type: "group", label: "Address", name: "address", fields: [
		{type: "text", label: "City", name: "city"},
		{type: "group", label: "Geo", name: "geo", fields: [{type: "number", label: "Lat", name: "lat"}]}
	]},
	{type: "list", value: ["a"], field: {type: "text", label: "Tags", name: "tags"}},
	{type: "list", value: [[{value: "x", label: "X"}]], field: {type: "select", label: "Picks", name: "picks", options: [{value: "x", label: "X"}, {value: "y", label: "Y"}]}},
	{type: "list", value: [{name: "Ann"}], field: {type: "group", label: "People", name: "people", fields: [
		{type: "text", label: "Name", name: "name"},
		{type: "list", field: {type: "number", label: "Scores", name: "scores"}}
	]}},
	{type: "series", label: "Pick", name: "pick", fields: [
		"(p) => ({type: 'select', label: 'Kind', name: 'kind', options: [{value: 'fruit', label: 'Fruit'}]})",
		"(p) => ({type: 'number', label: 'Qty', name: 'qty'})"
	]}
]}`

func TestSubmitGivesEveryKindItsShape(t *testing.T) {
	dir := t.TempDir()
	src := t.TempDir()
	note := covSubmitFile(t, src, "note.txt", "hello")
	one := covSubmitFile(t, src, "one.png", "1")
	two := covSubmitFile(t, src, "two.png", "22")
	a, f := covSubmitApp(t, dir, covSubmitShapesSpec)
	f.Field("Text").Type("plain")
	f.Field("Email").Type("ada@example.com")
	f.Field("Tel").Type("+1 555")
	f.Field("Url").Type("https://example.com")
	f.Field("Password").Type("s3cret")
	f.Field("Number").Type("-2.5")
	f.Field("Range").Type("7")
	f.Field("Date").Type("2026-03-04")
	f.Field("Time").Type("05:06")
	f.Field("Moment").Type("2026-03-04T05:06")
	f.Field("Month").Type("2026-03")
	f.Field("Week").Type("2026-W10")
	f.Field("Notes").Type("one\ntwo")
	f.Field("Radio").Choose("Yes")
	f.Field("Dropdown").Choose("Deep")
	f.Field("Boxes").Choose("D")
	f.Field("Boxes").Choose("A")
	covSubmitPick(f.Field("Find"), "o", "Okra")
	covSubmitPick(f.Field("Find"), "k", "Kale")
	covSubmitPick(f.Field("Find"), "l", "Leek")
	f.Field("Agree").Check()
	f.Field("Spam").Uncheck()
	f.Field("Doc").SetFiles(note)
	f.Field("Pics").SetFiles(one, two)
	f.Group("Address").Field("City").Type("Lima")
	f.Group("Address").Group("Geo").Field("Lat").Type("12")
	f.List("Tags").Add()
	f.List("Tags").Item(1).Control().Type("b")
	f.List("Picks").Add()
	f.List("Picks").Item(1).Control().Choose("Y")
	f.List("People").Item(0).List("Scores").Add()
	f.List("People").Item(0).List("Scores").Item(0).Control().Type("9")
	f.List("People").Add()
	f.List("People").Item(1).Field("Name").Type("Bob")
	f.Series("Pick").Step(0).Field("Kind").Choose("Fruit")
	f.Series("Pick").Step(1).Field("Qty").Type("3")
	f.Series("Pick").ExpectCompleted(true)
	f.Submit("Submit")

	var got struct {
		Data   map[string]json.RawMessage `json:"data"`
		Submit *string                    `json:"submit"`
	}
	raw := covSubmitted(a)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Submit == nil || *got.Submit != "" {
		t.Fatalf("submitted %s, want submit \"\"", raw)
	}
	var doc []string
	var pics []string
	json.Unmarshal(got.Data["doc"], &doc)
	json.Unmarshal(got.Data["pics"], &pics)
	if len(doc) != 1 || path.Base(doc[0]) != "note.txt" || len(pics) != 2 || path.Base(pics[0]) != "one.png" || path.Base(pics[1]) != "two.png" {
		t.Fatalf("the files gave doc %s and pics %s, want arrays of paths to note.txt and to one.png and two.png", got.Data["doc"], got.Data["pics"])
	}
	delete(got.Data, "doc")
	delete(got.Data, "pics")
	rest, _ := json.Marshal(got.Data)
	stack.ExpectJSON(t, rest, `{
		"text": "plain", "email": "ada@example.com", "tel": "+1 555", "url": "https://example.com", "password": "s3cret",
		"number": -2.5, "range": 7,
		"date": "2026-03-04", "time": "05:06", "moment": "2026-03-04T05:06", "month": "2026-03", "week": "2026-W10",
		"notes": "one\ntwo",
		"radio": [{"value": true, "label": "Yes"}],
		"dropdown": [{"value": {"deep": [1, 2]}, "label": "Deep"}],
		"boxes": [{"value": "a", "label": "A"}, {"value": "d", "label": "D"}],
		"find": [{"value": {"k": "okra"}, "label": "Okra"}, {"value": "kale", "label": "Kale"}, {"value": 2, "label": "Leek"}],
		"agree": true, "spam": false,
		"address": {"city": "Lima", "geo": {"lat": 12}},
		"tags": ["a", "b"],
		"picks": [[{"value": "x", "label": "X"}], [{"value": "y", "label": "Y"}]],
		"people": [{"name": "Ann", "scores": [9]}, {"name": "Bob", "scores": []}],
		"pick": {"kind": [{"value": "fruit", "label": "Fruit"}], "qty": 3}
	}`)
}

const covSubmitEmptySpec = `{fields: [
	{type: "text", label: "Text", name: "text", value: "pre"},
	{type: "password", label: "Password", name: "password"},
	{type: "number", label: "Number", name: "number", value: 4},
	{type: "date", label: "Date", name: "date", value: "2026-01-01"},
	{type: "time", label: "Time", name: "time"},
	{type: "textarea", label: "Notes", name: "notes", value: "pre"},
	{type: "select", label: "Radio", name: "radio", value: [{value: "a", label: "A"}], options: [{value: "a", label: "A"}]},
	{type: "select", label: "Dropdown", name: "dropdown", options: [
		{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"},
		{value: "d", label: "D"}, {value: "e", label: "E"}, {value: "f", label: "F"}
	]},
	{type: "select", label: "Boxes", name: "boxes", max: 2, value: [{value: "a", label: "A"}], options: [{value: "a", label: "A"}, {value: "b", label: "B"}]},
	{type: "search", label: "Find", name: "find", value: [{value: "kale", label: "Kale"}], search: "(q) => [{value: 'kale', label: 'Kale'}]"},
	{type: "file", label: "Doc", name: "doc"},
	{type: "checkbox", label: "Agree", name: "agree"}
]}`

func TestSubmitLeavesEmptyFieldsOut(t *testing.T) {
	a, f := covSubmitApp(t, t.TempDir(), covSubmitEmptySpec)
	f.Field("Text").Type("")
	f.Field("Number").Type("")
	f.Field("Date").Type("")
	f.Field("Notes").Type("")
	f.Field("Radio").Choose("None")
	f.Field("Boxes").Unchoose("A")
	f.Field("Find").Find(`button[aria-label="Remove Kale"]`).Click()
	f.Field("Find").WaitGone(`button[aria-label="Remove Kale"]`)
	f.Submit("Submit")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"agree": false}, "submit": ""}`)
}

func TestSubmitBlockedByEmptyListItemsUntilRemoved(t *testing.T) {
	a, f := covSubmitApp(t, "", `{fields: [
		{type: "list", value: ["a", "b", "c"], field: {type: "text", label: "Tags", name: "tags"}},
		{type: "list", value: [[{value: "x", label: "X"}], []], field: {type: "select", label: "Picks", name: "picks", options: [{value: "x", label: "X"}, {value: "y", label: "Y"}]}}
	]}`)
	picks := f.List("Picks")
	picks.Item(1).Control().ExpectRequiredEmpty(true)
	picks.Item(1).Control().WaitCount(`input[type="radio"]`, 2)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	picks.Item(1).Control().Choose("Y")
	picks.Item(1).Control().ExpectRequiredEmpty(false)
	f.ExpectSubmitDisabled(false)
	f.List("Tags").Item(1).Control().Type("")
	f.List("Tags").Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.List("Tags").Remove(1)
	f.List("Tags").ExpectCount(2)
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"tags": ["a", "c"], "picks": [[{"value": "x", "label": "X"}], [{"value": "y", "label": "Y"}]]}, "submit": ""}`)
}

func TestSubmitSavesFilesInTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	src := t.TempDir()
	note := covSubmitFile(t, src, "note.txt", "hello")
	one := covSubmitFile(t, src, "one.txt", "first")
	two := covSubmitFile(t, src, "two.txt", "second")
	gone := covSubmitFile(t, src, "gone.txt", "removed")
	kept := covSubmitFile(t, src, "kept.txt", "kept")
	a, f := covSubmitApp(t, dir, `{fields: [
		{type: "file", label: "Doc", name: "doc"},
		{type: "file", label: "Copy", name: "copy"},
		{type: "file", label: "Pics", name: "pics", max: 10},
		{type: "group", label: "Box", name: "box", fields: [{type: "file", label: "Inner", name: "inner"}]},
		{type: "list", field: {type: "file", label: "Files", name: "files"}}
	]}`)
	f.Field("Doc").SetFiles(note)
	f.Field("Copy").SetFiles(note)
	f.Field("Pics").SetFiles(one, two)
	f.Group("Box").Field("Inner").SetFiles(one)
	f.List("Files").Add()
	f.List("Files").Add()
	f.List("Files").ExpectCount(2)
	f.List("Files").Item(0).Control().SetFiles(gone)
	f.List("Files").Item(1).Control().SetFiles(kept)
	for _, name := range []string{"note.txt", "one.txt", "two.txt", "gone.txt", "kept.txt"} {
		if matches, _ := filepath.Glob(filepath.Join(dir, "board.island", "*", name)); len(matches) == 0 {
			t.Fatalf("%s is not in the workspace before the submit", name)
		}
	}
	f.List("Files").Remove(0)
	f.List("Files").ExpectCount(1)
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		matches, _ := filepath.Glob(filepath.Join(dir, "board.island", "*", "gone.txt"))
		return len(matches) == 0, strings.Join(matches, ", ")
	}, "the removed item's file is deleted")
	f.Submit("Submit")
	var got struct {
		Data struct {
			Doc   []string
			Copy  []string
			Pics  []string
			Box   struct{ Inner []string }
			Files [][]string
		}
	}
	raw := covSubmitted(a)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	d := got.Data
	if len(d.Doc) != 1 || len(d.Copy) != 1 || len(d.Box.Inner) != 1 || len(d.Pics) != 2 || len(d.Files) != 1 || len(d.Files[0]) != 1 {
		t.Fatalf("submitted %s, want one path for each single file, two pics and one file in the list", raw)
	}
	want := map[string]string{d.Doc[0]: "hello", d.Copy[0]: "hello", d.Box.Inner[0]: "first"}
	want[d.Pics[0]] = "first"
	want[d.Pics[1]] = "second"
	want[d.Files[0][0]] = "kept"
	dirs := map[string]bool{}
	for _, p := range []string{d.Doc[0], d.Copy[0], d.Pics[0], d.Box.Inner[0], d.Files[0][0]} {
		dirs[path.Dir(p)] = true
	}
	if len(dirs) != 5 || path.Dir(d.Pics[0]) != path.Dir(d.Pics[1]) {
		t.Fatalf("submitted %s, want one directory per file field", raw)
	}
	for p, content := range want {
		parts := strings.Split(p, "/")
		if len(parts) != 3 || parts[0] != "board.island" || parts[1] == "" {
			t.Fatalf("the path %q is not board.island/<field id>/<file name>", p)
		}
		saved, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil || string(saved) != content {
			t.Fatalf("the workspace has %q (%v) at %s, want %q", saved, err, p, content)
		}
	}
	if path.Base(d.Doc[0]) != "note.txt" || path.Base(d.Pics[0]) != "one.txt" || path.Base(d.Pics[1]) != "two.txt" || path.Base(d.Files[0][0]) != "kept.txt" {
		t.Fatalf("submitted %s, want the chosen file names", raw)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "board.island", "*", "gone.txt"))
	if len(matches) != 0 {
		t.Fatalf("the removed item's file was saved at %v", matches)
	}
}

func TestSubmitAgainAfterSubmit(t *testing.T) {
	dir := t.TempDir()
	src := t.TempDir()
	note := covSubmitFile(t, src, "note.txt", "v1")
	a, f := covSubmitApp(t, dir, `{fields: [
		{type: "text", label: "Title", name: "title", required: true},
		{type: "number", label: "Count", name: "count"},
		{type: "file", label: "Doc", name: "doc"}
	], submits: [{text: "Save", value: "save"}, {text: "Publish", value: "publish"}]}`)
	f.Field("Title").Type("One")
	f.Field("Count").Type("1")
	f.Field("Doc").SetFiles(note)
	f.Submit("Save")
	var first struct{ Data struct{ Doc []string } }
	raw := covSubmitted(a)
	json.Unmarshal(raw, &first)
	if len(first.Data.Doc) != 1 {
		t.Fatalf("submitted %s, want the file", raw)
	}
	doc := first.Data.Doc[0]
	stack.ExpectJSON(t, raw, `{"data": {"title": "One", "count": 1, "doc": ["`+doc+`"]}, "submit": "save"}`)

	f.Field("Title").ExpectValue("One")
	f.Field("Count").Type("")
	if err := os.WriteFile(note, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.Field("Doc").RemoveFile("note.txt")
	f.Field("Doc").SetFiles(note)
	f.Submit("Publish")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "One", "doc": ["`+doc+`"]}, "submit": "publish"}`)
	saved, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(doc)))
	if err != nil || string(saved) != "v2" {
		t.Fatalf("the workspace has %q (%v), want the file chosen last", saved, err)
	}

	f.Field("Title").Type("")
	f.ExpectSubmitDisabled(true)
	f.Field("Title").Type("Two")
	f.Submit("Save")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"title": "Two", "doc": ["`+doc+`"]}, "submit": "save"}`)
}
