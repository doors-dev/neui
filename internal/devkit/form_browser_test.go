//go:build browser

package devkit_test

import (
	"encoding/json"
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
	f.ExpectSubmitMessage("fix the errors above")
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
	start.ExpectRequiredMark(false)
	start.Type("2027-01-01")
	start.ExpectError("pick 2026-12-31 or earlier")
	start.Type("2026-06-15")
	start.ExpectNoError()
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
	count := bio.Find(".kisland-form-count")
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
	f.ExpectSubmitMessage("fill the fields marked *")
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
	f.ExpectSubmitMessage("fill the fields marked *")
	address.Field("Street").Type("Main 1")
	address.Field("City").Type("Lima")
	lat := address.Group("Geo").Field("Lat")
	lat.Type("100")
	lat.ExpectError("enter 90 or less")
	f.ExpectSubmitMessage("fix the errors above")
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
	tags.ExpectCount(2)
	tags.WaitCount("button.kisland-form-item-remove", 2)
	tags.Find("button.kisland-form-item-remove").WaitAttr("aria-label", "Remove item 1 of Tags")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	f.ExpectSubmitMessage("fill the fields marked *")

	tags.Remove(0)
	tags.ExpectCount(1)
	tags.ExpectError("add at least 2")
	f.ExpectSubmitMessage("fix the errors above")
	tags.Add()
	tags.ExpectCount(2)
	tags.ExpectNoError()
	tags.ExpectRequiredMark(true)

	tags.Item(0).Field("Tags").Type("go")
	tags.Item(1).Field("Tags").Type("toolong")
	tags.Item(1).Field("Tags").ExpectError("use at most 4 characters")
	f.ExpectSubmitMessage("fix the errors above")
	tags.Item(1).Field("Tags").Type("js")
	tags.ExpectRequiredMark(false)
	tags.ExpectAddDisabled(false)

	tags.Add()
	tags.ExpectCount(3)
	tags.ExpectAddDisabled(true)
	tags.Find(".kisland-form-items-foot .kisland-form-help").WaitText("Maximum of 3 reached.")
	tags.Add()
	tags.ExpectCount(3)
	tags.WaitCount("button.kisland-form-item-remove", 3)
	tags.Item(2).Field("Tags").Type("rs")
	tags.Remove(0)
	tags.ExpectCount(2)
	tags.ExpectAddDisabled(false)
	tags.Find("button.kisland-form-item-remove").WaitAttr("aria-label", "Remove item 1 of Tags")
	tags.Item(0).Field("Tags").ExpectValue("js")

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
	people.ExpectCount(1)
	people.Find(".kisland-form-items-foot").WaitGone(".kisland-form-help")
	people.Item(0).Field("Name").ExpectRequiredMark(true)
	people.Item(0).Field("Name").Type("Ada")
	people.Item(0).Field("Age").Type("36")
	people.Add()
	people.ExpectCount(2)
	people.Item(1).Find(".kisland-form-item-body").WaitAttr("aria-label", "People, item 2")
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

func TestFormListDefaults(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: ["go", "js"], field: {type: "text", label: "Tags", name: "tags"}}]}`)
	tags := f.List("Tags")
	tags.ExpectCount(2)
	tags.Item(0).Field("Tags").ExpectValue("go")
	tags.Item(1).Field("Tags").ExpectValue("js")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": ["go", "js"]}, "submit": ""}`)
}

func TestFormNumberFraction(t *testing.T) {
	t.Skip("bug: a number field without step renders no step attribute, so the browser's own validation takes step 1, rejects 1.5 the widget accepts, and blocks the submit with no message")
	a, f := formApp(t, `{fields: [{type: "number", label: "Weight", name: "weight"}]}`)
	f.Field("Weight").Type("1.5")
	f.Field("Weight").ExpectNoError()
	f.WaitCount(":invalid", 0)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"weight": 1.5}, "submit": ""}`)
}

func TestFormListRemoveShowsCross(t *testing.T) {
	t.Skip("bug: the list item remove button has no content and no style, so it shows nothing where the × should be")
	_, f := formApp(t, listSpec)
	f.List("Tags").Find("button.kisland-form-item-remove").WaitText("×")
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
	f.ExpectSubmitMessage("fill the fields marked *")
	pick.Step(0).Field("Kind").ExpectRequiredMark(false)

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
	f.ExpectSubmitMessage("fill the fields marked *")
	f.Series("Extra").ExpectRequiredMark(true)
}

const gatingSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "number", label: "Count", name: "count", min: 1}
]}`

func TestFormSubmitGating(t *testing.T) {
	a, f := formApp(t, gatingSpec)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("fill the fields marked *")
	f.Field("Count").Type("0")
	f.ExpectSubmitMessage("fix the errors above")
	f.Field("Count").Type("2")
	f.ExpectSubmitMessage("fill the fields marked *")
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
	f.Find(".kisland-form-actions").WaitCount("button[type=submit]", 2)
	f.Field("Title").Type("Hi")
	f.Submit("Publish")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"title": "Hi"}, "submit": "publish"}`)
}

func TestFormCancel(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "text", label: "Name", name: "name", required: true}], cancel: "Never mind"}`)
	f.ExpectSubmitDisabled(true)
	f.Find(".kisland-form-actions button[type=button]").WaitText("Never mind")
	f.Cancel()
	stack.ExpectJSON(t, a.Submitted(), `{"canceled": true}`)
}

func TestFormFile(t *testing.T) {
	dir := t.TempDir()
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Script: `ui.root.set(ui.form({fields: [
		{type: "file", label: "Doc", name: "doc", required: true, accept: ".txt"}
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
	doc.ExpectValue("No file chosen")
	doc.SetFiles(image)
	doc.ExpectError("image.png isn't an accepted file type, choose .txt")
	f.ExpectSubmitDisabled(true)
	doc.SetFiles(note)
	doc.ExpectNoError()
	doc.ExpectValue("note.txt · 5 B")
	f.Submit("Submit")
	var got struct {
		Data struct{ Doc string }
	}
	if err := json.Unmarshal(a.Submitted(), &got); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got.Data.Doc) != "note.txt" || filepath.Dir(filepath.Dir(got.Data.Doc)) != "island" {
		t.Fatalf("the file field gave %q, want island/<id>/note.txt", got.Data.Doc)
	}
	saved, err := os.ReadFile(filepath.Join(dir, got.Data.Doc))
	if err != nil || string(saved) != "hello" {
		t.Fatalf("the workspace has %q (%v), want hello", saved, err)
	}
}
