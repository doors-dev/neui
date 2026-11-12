//go:build browser

package devkit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neui.dev/internal/testkit/stack"
)

const reqEmptyTypedSpec = `{fields: [
	{type: "text", label: "Text", name: "text", required: true, minLength: 3},
	{type: "email", label: "Email", name: "email", required: true},
	{type: "tel", label: "Tel", name: "tel", required: true, pattern: "[0-9]+"},
	{type: "url", label: "Url", name: "url", required: true},
	{type: "password", label: "Password", name: "password", required: true, minLength: 3},
	{type: "number", label: "Number", name: "number", required: true, max: 5},
	{type: "date", label: "Date", name: "date", required: true, max: "2026-12-31"},
	{type: "textarea", label: "Notes", name: "notes", required: true, minLength: 3},
	{type: "range", label: "Range", name: "range", required: true, min: 0, max: 10},
	{type: "text", label: "Optional", name: "optional"}
]}`

func TestRequiredEmptyTypedFields(t *testing.T) {
	_, f := formApp(t, reqEmptyTypedSpec)
	for _, c := range []struct{ label, bad, good string }{
		{"Text", "ab", "abc"},
		{"Email", "a@", "a@b.example"},
		{"Tel", "x", "123"},
		{"Url", "nope", "https://example.com"},
		{"Password", "ab", "abc"},
		{"Number", "9", "3"},
		{"Date", "2027-01-01", "2026-06-15"},
		{"Notes", "ab", "abc"},
	} {
		field := f.Field(c.label)
		field.ExpectRequiredEmpty(true)
		field.ExpectInvalid(false)
		field.ExpectNoError()
		field.Type(c.bad)
		field.ExpectInvalid(true)
		field.ExpectRequiredEmpty(false)
		field.Type(c.good)
		field.ExpectInvalid(false)
		field.ExpectRequiredEmpty(false)
		field.Type("")
		field.ExpectRequiredEmpty(true)
		field.ExpectInvalid(false)
		field.ExpectNoError()
		field.Type(c.good)
		field.ExpectRequiredEmpty(false)
	}
	f.Field("Range").ExpectRequiredEmpty(false)
	f.Field("Optional").ExpectRequiredEmpty(false)
	f.ExpectSubmitDisabled(false)
}

const reqEmptyChoiceSpec = `{fields: [
	{type: "select", label: "Radio", name: "radio", min: 1, options: [{value: "a", label: "A"}, {value: "b", label: "B"}]},
	{type: "select", label: "Dropdown", name: "dropdown", min: 1, options: [
		{value: 1, label: "One"}, {value: 2, label: "Two"}, {value: 3, label: "Three"},
		{value: 4, label: "Four"}, {value: 5, label: "Five"}, {value: 6, label: "Six"}
	]},
	{type: "select", label: "Boxes", name: "boxes", min: 1, max: 3, options: [{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}]},
	{type: "select", label: "Pair", name: "pair", min: 2, max: 3, options: [{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}]},
	{type: "search", label: "Find", name: "find", min: 1, search: "(q) => [{value: 'kale', label: 'Kale'}]"},
	{type: "checkbox", label: "Agree", name: "agree", required: true},
	{type: "select", label: "Loose", name: "loose", options: [{value: "a", label: "A"}]},
	{type: "checkbox", label: "News", name: "news"}
]}`

func TestRequiredEmptyChoiceFields(t *testing.T) {
	a, f := formApp(t, reqEmptyChoiceSpec)
	radio := f.Field("Radio")
	radio.ExpectRequiredEmpty(true)
	radio.Choose("A")
	radio.ExpectRequiredEmpty(false)

	dropdown := f.Field("Dropdown")
	dropdown.ExpectRequiredEmpty(true)
	dropdown.Choose("Two")
	dropdown.ExpectRequiredEmpty(false)
	dropdown.Choose("Choose…")
	dropdown.ExpectRequiredEmpty(true)
	dropdown.Choose("Six")

	boxes := f.Field("Boxes")
	boxes.ExpectRequiredEmpty(true)
	boxes.Choose("B")
	boxes.ExpectRequiredEmpty(false)
	boxes.Unchoose("B")
	boxes.ExpectRequiredEmpty(true)
	boxes.Choose("C")

	pair := f.Field("Pair")
	pair.ExpectRequiredEmpty(true)
	pair.Choose("A")
	pair.ExpectError("choose at least 2")
	pair.ExpectInvalid(true)
	pair.ExpectRequiredEmpty(false)
	pair.Unchoose("A")
	pair.ExpectInvalid(false)
	pair.ExpectRequiredEmpty(true)
	pair.Choose("A")
	pair.Choose("B")
	pair.ExpectRequiredEmpty(false)

	find := f.Field("Find")
	find.ExpectRequiredEmpty(true)
	covSubmitPick(find, "ka", "Kale")
	find.ExpectRequiredEmpty(false)
	find.Find(`button[aria-label="Remove Kale"]`).Click()
	find.ExpectRequiredEmpty(true)
	covSubmitPick(find, "ka", "Kale")

	agreeTint := `() => {
		const row = Array.from(document.querySelectorAll(".kisland-form-check-single")).find((l) => l.textContent.includes("Agree"))
		return getComputedStyle(row).backgroundColor
	}`
	agree := f.Field("Agree")
	agree.ExpectRequiredEmpty(true)
	tinted := a.Page.MustEval(agreeTint).String()
	agree.Check()
	agree.ExpectRequiredEmpty(false)
	expectEval(t, a, agreeTint, "rgba(0, 0, 0, 0)", "no tint once checked")
	agree.Uncheck()
	agree.ExpectRequiredEmpty(true)
	expectEval(t, a, agreeTint, tinted, "the tint back when unchecked")
	if tinted == "rgba(0, 0, 0, 0)" {
		t.Fatalf("the required empty checkbox row has no tint")
	}
	agree.ExpectInvalid(false)
	agree.Check()

	f.Field("Loose").ExpectRequiredEmpty(false)
	f.Field("News").ExpectRequiredEmpty(false)
	f.ExpectSubmitDisabled(false)
}

func TestRequiredEmptyFileFields(t *testing.T) {
	a, f, _, src := covFileApp(t, `
		{type: "file", label: "Doc", name: "doc", required: true, accept: ".txt"},
		{type: "file", label: "Docs", name: "docs", required: true, multiple: true},
		{type: "file", label: "Spare", name: "spare"}`)
	big := filepath.Join(src, "big.txt")
	if err := os.WriteFile(big, bytes.Repeat([]byte("x"), 200<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := f.Field("Doc")
	doc.ExpectRequiredEmpty(true)
	doc.SetFiles(filepath.Join(src, "pic.png"))
	doc.ExpectFileFailed("pic.png", "not .txt")
	doc.ExpectInvalid(false)
	doc.ExpectRequiredEmpty(true)
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.WaitGone(".kisland-form-file-zone")
	doc.ExpectRequiredMark(false)

	docs := f.Field("Docs")
	docs.ExpectRequiredEmpty(true)
	a.ThrottleUploads(20 << 10)
	docs.PickFiles(big)
	docs.ExpectFileChips("big.txt uploading")
	docs.ExpectRequiredEmpty(true)
	docs.ExpectRequiredMark(true)
	a.ThrottleUploads(0)
	docs.ExpectFiles("big.txt")
	docs.ExpectRequiredEmpty(false)
	docs.ExpectRequiredMark(false)
	docs.RemoveFile("big.txt")
	docs.ExpectRequiredEmpty(true)

	f.Field("Spare").ExpectRequiredEmpty(false)
}

func TestRequiredEmptyListItemsOfEveryKind(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "list", field: {type: "text", label: "Texts", name: "texts", required: false}},
		{type: "list", field: {type: "number", label: "Numbers", name: "numbers"}},
		{type: "list", field: {type: "date", label: "Dates", name: "dates"}},
		{type: "list", field: {type: "textarea", label: "Notes", name: "notes"}},
		{type: "list", field: {type: "select", label: "Radios", name: "radios", min: 0, options: [{value: "a", label: "A"}, {value: "b", label: "B"}]}},
		{type: "list", field: {type: "select", label: "Dropdowns", name: "dropdowns", options: [
			{value: 1, label: "One"}, {value: 2, label: "Two"}, {value: 3, label: "Three"}, {value: 4, label: "Four"}, {value: 5, label: "Five"}, {value: 6, label: "Six"}
		]}},
		{type: "list", field: {type: "select", label: "Boxes", name: "boxes", max: 2, options: [{value: "x", label: "X"}, {value: "y", label: "Y"}]}},
		{type: "list", field: {type: "search", label: "Finds", name: "finds", search: "(q) => [{value: 'kale', label: 'Kale'}]"}},
		{type: "list", field: {type: "file", label: "Files", name: "files"}}
	]}`)
	f.ExpectSubmitDisabled(false)
	for _, label := range []string{"Texts", "Numbers", "Dates", "Notes", "Radios", "Dropdowns", "Boxes", "Finds", "Files"} {
		list := f.List(label)
		list.Add()
		list.ExpectCount(1)
		list.Item(0).Control().ExpectRequiredEmpty(true)
		list.Item(0).Control().ExpectInvalid(false)
		list.ExpectNoError()
		list.ExpectRequiredMark(false)
		f.ExpectSubmitMessage("form is not completed or contains errors")
		list.Remove(0)
		list.ExpectCount(0)
		f.ExpectSubmitMessage("")
	}
	radios := f.List("Radios")
	radios.Add()
	radios.Item(0).Control().WaitCount(`input[type="radio"]`, 2)
	radios.Item(0).Control().Choose("B")
	radios.Item(0).Control().ExpectRequiredEmpty(false)
	boxes := f.List("Boxes")
	boxes.Add()
	boxes.Item(0).Control().Choose("X")
	boxes.Item(0).Control().ExpectRequiredEmpty(false)
	boxes.Item(0).Control().Unchoose("X")
	boxes.Item(0).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitDisabled(true)
}

func TestRequiredEmptyListRowsBesideCountRules(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", min: 2, max: 3, field: {type: "text", label: "Tags", name: "tags", maxLength: 3}}]}`)
	tags := f.List("Tags")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Add()
	tags.ExpectError("add at least 2")
	tags.ExpectRequiredMark(true)
	tags.Item(0).Control().ExpectRequiredEmpty(true)
	tags.Item(0).Control().Type("a")
	tags.Item(0).Control().ExpectRequiredEmpty(false)
	tags.Add()
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)
	tags.Item(0).Control().ExpectRequiredEmpty(false)
	tags.Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(1).Control().Type("long")
	tags.Item(1).Control().ExpectInvalid(true)
	tags.Item(1).Control().ExpectRequiredEmpty(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Item(1).Control().Type("b")
	tags.Item(1).Control().ExpectRequiredEmpty(false)
	tags.Add()
	tags.ExpectAddDisabled(true)
	tags.Item(2).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitDisabled(true)
	tags.Remove(2)
	tags.ExpectCount(2)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["a", "b"]}, "submit": ""}`)
}

func TestRequiredEmptyInsideListOfGroups(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", min: 1, field: {type: "group", label: "People", name: "people", fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "text", label: "Nick", name: "nick"},
		{type: "select", label: "Role", name: "role", min: 1, options: [{value: "a", label: "Admin"}, {value: "u", label: "User"}]}
	]}}]}`)
	people := f.List("People")
	people.Add()
	people.ExpectCount(1)
	item := people.Item(0)
	item.Field("Name").ExpectRequiredEmpty(true)
	item.Field("Name").ExpectRequiredMark(true)
	item.Field("Nick").ExpectRequiredEmpty(false)
	item.Field("Role").ExpectRequiredEmpty(true)
	people.ExpectNoError()
	people.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	item.Field("Name").Type("Ada")
	item.Field("Name").ExpectRequiredEmpty(false)
	item.Field("Role").Choose("User")
	item.Field("Role").ExpectRequiredEmpty(false)
	item.Field("Nick").ExpectRequiredEmpty(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"people": [{"name": "Ada", "role": [{"value": "u", "label": "User"}]}]}, "submit": ""}`)
}

func TestRequiredEmptyRequiredSeriesSteps(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "series", label: "Order", name: "order", required: true, fields: [
		"(p) => ({type: 'text', label: 'Item', name: 'item', required: false})",
		"(p) => ({type: 'select', label: 'Size', name: 'size', min: 0, options: [{value: 's', label: 'S'}, {value: 'l', label: 'L'}]})"
	]}]}`)
	order := f.Series("Order")
	item := order.Step(0).Field("Item")
	order.ExpectRequiredMark(true)
	item.ExpectRequiredMark(true)
	item.ExpectRequiredEmpty(true)
	item.Find("input").WaitAttr("aria-required", "true")
	item.Type("tea")
	item.ExpectRequiredEmpty(false)
	size := order.Step(1).Field("Size")
	size.ExpectRequiredEmpty(true)
	size.WaitCount(`input[type="radio"]`, 2)
	size.Choose("L")
	size.ExpectRequiredEmpty(false)
	order.ExpectCompleted(true)
	order.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	item.Type("")
	item.ExpectRequiredEmpty(true)
	order.ExpectSteps(1)
	order.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestRequiredEmptyOptionalSeriesSteps(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "series", label: "Extra", name: "extra", fields: [
		"(p) => ({type: 'select', label: 'Want', name: 'want', min: 1, options: [{value: 'yes', label: 'Yes'}]})",
		"(p) => ({type: 'text', label: 'What', name: 'what'})",
		"(p) => ({type: 'search', label: 'Who', name: 'who', search: \"(q) => [{value: 'ann', label: 'Ann'}]\"})"
	]}]}`)
	extra := f.Series("Extra")
	want := extra.Step(0).Field("Want")
	want.ExpectRequiredEmpty(false)
	want.ExpectRequiredMark(false)
	want.Find(".kisland-form-options").WaitNoAttr("aria-required")
	want.WaitCount(`input[type="radio"]`, 2)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	want.Choose("Yes")
	what := extra.Step(1).Field("What")
	what.ExpectRequiredEmpty(true)
	what.ExpectRequiredMark(true)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	what.Type("cake")
	what.ExpectRequiredEmpty(false)
	who := extra.Step(2).Field("Who")
	who.ExpectRequiredEmpty(true)
	covSubmitPick(who, "a", "Ann")
	who.ExpectRequiredEmpty(false)
	extra.ExpectCompleted(true)
	extra.ExpectRequiredMark(false)
	what.Type("")
	what.ExpectRequiredEmpty(true)
	extra.ExpectSteps(2)
	extra.ExpectRequiredMark(false)
	want.Choose("None")
	extra.ExpectSteps(1)
	want.ExpectRequiredEmpty(false)
	extra.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
}

func TestRequiredEmptyCheckboxRejectedAsListTemplateOrSeriesStep(t *testing.T) {
	for _, c := range []struct{ name, spec, want string }{
		{"list template", `{type: "list", field: {type: "checkbox", label: "Flags", name: "flags"}}`, `list "flags": a list's field can't be a checkbox`},
		{"list template with defaults", `{type: "list", value: [true], field: {type: "checkbox", label: "Flags", name: "flags"}}`, `list "flags": a list's field can't be a checkbox`},
		{"list template in a list's group", `{type: "list", field: {type: "group", label: "Rows", name: "rows", fields: [{type: "list", field: {type: "checkbox", label: "Flags", name: "flags"}}]}}`, `list "flags": a list's field can't be a checkbox`},
		{"first series step", `{type: "series", label: "S", name: "s", fields: [
			"(p) => ({type: 'checkbox', label: 'Sure', name: 'sure'})",
			"(p) => ({type: 'text', label: 'B', name: 'b'})"
		]}`, "a series field can't be a checkbox"},
		{"series step replayed from defaults", `{type: "list", value: [{a: "x"}], field: {type: "series", label: "S", name: "s", fields: [
			"(p) => ({type: 'text', label: 'A', name: 'a'})",
			"(p) => ({type: 'checkbox', label: 'Sure', name: 'sure'})"
		]}}`, "a series field can't be a checkbox"},
	} {
		t.Run(c.name, func(t *testing.T) {
			contFails(t, `{fields: [`+c.spec+`]}`, c.want)
		})
	}
	a, f := formApp(t, `{fields: [{type: "series", label: "S", name: "s", required: true, fields: [
		"(p) => ({type: 'text', label: 'A', name: 'a'})",
		"(p) => ({type: 'checkbox', label: 'Sure', name: 'sure'})"
	]}]}`)
	s := f.Series("S")
	s.Step(0).Field("A").Type("x")
	if got := string(a.ChoiceReport("call_error")); !strings.Contains(got, "a series field can't be a checkbox") {
		t.Fatalf("the call_error reads %s, want the checkbox step refused", got)
	}
	s.ExpectSteps(1)
	f.ExpectSubmitDisabled(true)
}
