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

//go:build browser

package devkit_test

import (
	"encoding/json"
	"path"
	"path/filepath"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func formShownAgain(t *testing.T, spec string, fill func(f stack.Form)) (stack.Form, json.RawMessage) {
	t.Helper()
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var f = ui.form(` + spec + `)
		ui.root.set(f)
		var ev = ui.draw()
		ui.root.set(ui.column({}, ui.markdown({text: "Shown again"}), f))
		ui.notify(ev)
		ui.draw()`})
	f := a.Form()
	fill(f)
	f.Element().MustEval(`() => { this.dataset.old = "" }`)
	f.Submit("Submit")
	submitted := a.Submitted()
	a.Find(".neisland-text").WaitText("Shown again")
	a.Find("body").WaitCount(".neisland-form[data-old]", 0)
	return f, submitted
}

func TestFormShownAgainShowsTypedText(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "text", label: "Name", name: "name", value: "init"},
		{type: "email", label: "Email", name: "email"},
		{type: "tel", label: "Phone", name: "phone"},
		{type: "url", label: "Site", name: "site"},
		{type: "password", label: "Secret", name: "secret"},
		{type: "text", label: "Nick", name: "nick", value: "gone"}
	]}`, func(f stack.Form) {
		f.Field("Name").Type("Ada")
		f.Field("Email").Type("ada@example.com")
		f.Field("Phone").Type("555 0100")
		f.Field("Site").Type("https://example.com")
		f.Field("Secret").Type("s3cret")
		f.Field("Nick").Type("")
	})
	stack.ExpectJSON(t, submitted, `{"data": {"name": "Ada", "email": "ada@example.com", "phone": "555 0100", "site": "https://example.com", "secret": "s3cret"}, "submit": ""}`)
	f.Field("Name").ExpectValue("Ada")
	f.Field("Email").ExpectValue("ada@example.com")
	f.Field("Phone").ExpectValue("555 0100")
	f.Field("Site").ExpectValue("https://example.com")
	f.Field("Secret").ExpectValue("s3cret")
	f.Field("Nick").ExpectValue("")
}

func TestFormShownAgainShowsNumbers(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "number", label: "Age", name: "age", value: 1},
		{type: "range", label: "Volume", name: "volume", min: 0, max: 10, step: 2}
	]}`, func(f stack.Form) {
		f.Field("Volume").ExpectValue("6")
		f.Field("Age").Type("42")
		f.Field("Volume").Type("8")
	})
	stack.ExpectJSON(t, submitted, `{"data": {"age": 42, "volume": 8}, "submit": ""}`)
	f.Field("Age").ExpectValue("42")
	f.Field("Volume").ExpectValue("8")
	f.Field("Volume").Find("output").WaitText("8")
}

func TestFormShownAgainShowsDates(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "date", label: "Day", name: "day", value: "2026-01-01"},
		{type: "time", label: "Time", name: "time", value: "08:00"},
		{type: "datetime-local", label: "At", name: "at"},
		{type: "month", label: "Month", name: "month"},
		{type: "week", label: "Week", name: "week"}
	]}`, func(f stack.Form) {
		f.Field("Day").Type("2026-06-15")
		f.Field("Time").Type("09:30")
		f.Field("At").Type("2026-06-15T09:30")
		f.Field("Month").Type("2026-06")
		f.Field("Week").Type("2026-W24")
	})
	stack.ExpectJSON(t, submitted, `{"data": {"day": "2026-06-15", "time": "09:30", "at": "2026-06-15T09:30", "month": "2026-06", "week": "2026-W24"}, "submit": ""}`)
	f.Field("Day").ExpectValue("2026-06-15")
	f.Field("Time").ExpectValue("09:30")
	f.Field("At").ExpectValue("2026-06-15T09:30")
	f.Field("Month").ExpectValue("2026-06")
	f.Field("Week").ExpectValue("2026-W24")
}

func TestFormShownAgainShowsTextarea(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "textarea", label: "Notes", name: "notes", value: "line one", maxLength: 20}
	]}`, func(f stack.Form) {
		f.Field("Notes").Type("first\nsecond")
	})
	stack.ExpectJSON(t, submitted, `{"data": {"notes": "first\nsecond"}, "submit": ""}`)
	f.Field("Notes").ExpectValue("first\nsecond")
	f.Field("Notes").Find(".neisland-form-count").WaitText("12 / 20")
}

func TestFormShownAgainShowsCheckboxes(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "checkbox", label: "News", name: "news", checked: true},
		{type: "checkbox", label: "Terms", name: "terms"}
	]}`, func(f stack.Form) {
		f.Field("News").Uncheck()
		f.Field("Terms").Check()
	})
	stack.ExpectJSON(t, submitted, `{"data": {"news": false, "terms": true}, "submit": ""}`)
	f.Field("News").ExpectValue("false")
	f.Field("Terms").ExpectValue("true")
}

func TestFormShownAgainShowsChoices(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "select", label: "Size", name: "size", value: [{value: "s", label: "S"}], options: [{value: "s", label: "S"}, {value: "m", label: "M"}, {value: "l", label: "L"}]},
		{type: "select", label: "Country", name: "country", value: [{value: "ar", label: "Argentina"}], options: [
			{value: "ar", label: "Argentina"}, {value: "br", label: "Brazil"}, {value: "cl", label: "Chile"},
			{value: "co", label: "Colombia"}, {value: "pe", label: "Peru"}, {value: "uy", label: "Uruguay"}
		]},
		{type: "select", label: "Toppings", name: "toppings", max: 3, value: [{value: "a", label: "A"}], options: [{value: "a", label: "A"}, {value: "b", label: "B"}, {value: {k: "c"}, label: "C"}]}
	]}`, func(f stack.Form) {
		f.Field("Size").Choose("L")
		f.Field("Country").Choose("Peru")
		f.Field("Toppings").Unchoose("A")
		f.Field("Toppings").Choose("B")
		f.Field("Toppings").Choose("C")
		f.Field("Toppings").ExpectValue("B, C")
	})
	stack.ExpectJSON(t, submitted, `{"data": {
		"size": [{"value": "l", "label": "L"}],
		"country": [{"value": "pe", "label": "Peru"}],
		"toppings": [{"value": "b", "label": "B"}, {"value": {"k": "c"}, "label": "C"}]
	}, "submit": ""}`)
	f.Field("Size").ExpectValue("L")
	f.Field("Country").ExpectValue("Peru")
	f.Field("Toppings").ExpectValue("B, C")
}

func TestFormShownAgainShowsSearchChips(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "search", label: "Veg", name: "veg", max: 2, value: [{value: "leek", label: "Leek"}], search: "(q) => [{value: 'kale', label: 'Kale'}, {value: 'leek', label: 'Leek'}]"}
	]}`, func(f stack.Form) {
		f.Field("Veg").Find(`button[aria-label="Remove Leek"]`).Click()
		f.Field("Veg").WaitCount(".neisland-form-search-chip", 0)
		covSubmitPick(f.Field("Veg"), "k", "Kale")
	})
	stack.ExpectJSON(t, submitted, `{"data": {"veg": [{"value": "kale", "label": "Kale"}]}, "submit": ""}`)
	f.Field("Veg").WaitCount(".neisland-form-search-chip", 1)
	f.Field("Veg").WaitCount(`button[aria-label="Remove Kale"]`, 1)
}

func TestFormShownAgainShowsFiles(t *testing.T) {
	src := t.TempDir()
	a := covSubmitFile(t, src, "a.txt", "aaa")
	b := covSubmitFile(t, src, "b.txt", "bbbb")
	f, submitted := formShownAgain(t, `{fields: [{type: "file", label: "Docs", name: "docs", max: 2}]}`, func(f stack.Form) {
		f.Field("Docs").SetFiles(a, b)
	})
	var got struct {
		Data struct {
			Docs []string
		}
	}
	if err := json.Unmarshal(submitted, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Docs) != 2 || path.Base(got.Data.Docs[0]) != filepath.Base(a) || path.Base(got.Data.Docs[1]) != filepath.Base(b) {
		t.Fatalf("submitted %s, want a.txt and b.txt", submitted)
	}
	f.Field("Docs").ExpectFiles("a.txt", "b.txt")
}

func TestFormShownAgainShowsContainers(t *testing.T) {
	f, submitted := formShownAgain(t, `{fields: [
		{type: "group", label: "Address", name: "address", fields: [{type: "text", label: "City", name: "city", value: "Rome"}]},
		{type: "list", value: ["x"], field: {type: "text", label: "Tags", name: "tags"}},
		{type: "list", value: [{name: "Ann"}], field: {type: "group", label: "People", name: "people", fields: [{type: "text", label: "Name", name: "name"}]}},
		{type: "series", label: "Pick", name: "pick", fields: [
			"(p) => ({type: 'select', label: 'Kind', name: 'kind', options: [{value: 'x', label: 'X'}, {value: 'z', label: 'Z'}]})",
			"(p) => ({type: 'text', label: 'Why', name: 'why', value: 'init'})"
		]}
	]}`, func(f stack.Form) {
		f.Group("Address").Field("City").Type("Lima")
		f.List("Tags").Item(0).Control().Type("x2")
		f.List("Tags").Add()
		f.List("Tags").Item(1).Control().Type("y")
		f.List("People").Item(0).Field("Name").Type("Bea")
		f.Series("Pick").Step(0).Field("Kind").Choose("Z")
		f.Series("Pick").Step(1).Field("Why").Type("because")
	})
	stack.ExpectJSON(t, submitted, `{"data": {
		"address": {"city": "Lima"},
		"tags": ["x2", "y"],
		"people": [{"name": "Bea"}],
		"pick": {"kind": [{"value": "z", "label": "Z"}], "why": "because"}
	}, "submit": ""}`)
	f.Group("Address").Field("City").ExpectValue("Lima")
	f.List("Tags").ExpectCount(2)
	f.List("Tags").Item(0).Control().ExpectValue("x2")
	f.List("Tags").Item(1).Control().ExpectValue("y")
	f.List("People").Item(0).Field("Name").ExpectValue("Bea")
	f.Series("Pick").Step(0).Field("Kind").ExpectValue("Z")
	f.Series("Pick").Step(1).Field("Why").ExpectValue("because")
}
