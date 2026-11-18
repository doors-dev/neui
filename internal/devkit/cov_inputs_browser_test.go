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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

const covRequiredSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "password", label: "Secret", name: "secret", required: true},
	{type: "number", label: "Age", name: "age", required: true},
	{type: "date", label: "Day", name: "day", required: true},
	{type: "textarea", label: "Bio", name: "bio", required: true}
]}`

func TestCovRequiredMarkTogglesWithEmptiness(t *testing.T) {
	a, f := formApp(t, covRequiredSpec)
	for _, c := range []struct{ label, value string }{
		{"Name", "Ada"}, {"Secret", "s3cret"}, {"Age", "36"}, {"Day", "2026-06-15"}, {"Bio", "hi"},
	} {
		field := f.Field(c.label)
		field.ExpectRequiredMark(true)
		field.ExpectInvalid(false)
		field.ExpectNoError()
		field.Type(c.value)
		field.ExpectRequiredMark(false)
		field.Type("")
		field.ExpectRequiredMark(true)
		field.ExpectInvalid(false)
		field.ExpectNoError()
		field.Type(c.value)
		field.ExpectRequiredMark(false)
	}
	f.ExpectTextLacks("field is required")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ada", "secret": "s3cret", "age": 36, "day": "2026-06-15", "bio": "hi"}, "submit": ""}`)
}

func TestCovRequiredEmptyGatesSubmitWithoutErrorText(t *testing.T) {
	_, f := formApp(t, covRequiredSpec)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectTextLacks("field is required")
	f.Field("Name").Type("Ada")
	f.Field("Name").Type("")
	f.Field("Name").ExpectRequiredMark(true)
	f.ExpectTextLacks("field is required")
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestCovShownErrorOutranksRequiredInSubmitMessage(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name", required: true},
		{type: "text", label: "Code", name: "code", minLength: 3}
	]}`)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Code").Type("ab")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectSubmitDisabled(true)
	f.Field("Code").Type("abc")
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Name").Type("Ada")
	f.ExpectSubmitMessage("")
	f.ExpectSubmitDisabled(false)
}

func TestCovRequiredMarkStaysWhileAnyError(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "text", label: "Code", name: "code", required: true, minLength: 3}]}`)
	code := f.Field("Code")
	code.ExpectRequiredMark(true)
	code.Type("ab")
	code.ExpectError("use at least 3 characters")
	code.ExpectRequiredMark(true)
	code.ExpectInvalid(true)
	code.Type("")
	code.ExpectRequiredMark(true)
	code.ExpectNoError()
	code.ExpectInvalid(false)
	code.Type("abc")
	code.ExpectNoError()
	code.ExpectRequiredMark(false)
}

func TestCovOptionalNeverMarked(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name"},
		{type: "number", label: "Age", name: "age"},
		{type: "date", label: "Day", name: "day"},
		{type: "textarea", label: "Bio", name: "bio"},
		{type: "checkbox", label: "News", name: "news"},
		{type: "file", label: "Doc", name: "doc"}
	]}`)
	for _, label := range []string{"Name", "Age", "Day", "Bio", "News", "Doc"} {
		f.Field(label).ExpectRequiredMark(false)
		f.Field(label).ExpectInvalid(false)
	}
	f.ExpectSubmitDisabled(false)
	f.ExpectSubmitMessage("")
}

func TestCovEmptyValuesAreAbsent(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name", value: "Ada"},
		{type: "email", label: "Email", name: "email"},
		{type: "number", label: "Age", name: "age", value: 3},
		{type: "date", label: "Day", name: "day", value: "2026-01-01"},
		{type: "time", label: "At", name: "at"},
		{type: "textarea", label: "Bio", name: "bio", value: "x"}
	]}`)
	for _, label := range []string{"Name", "Age", "Day", "Bio"} {
		f.Field(label).Type("")
		f.Field(label).ExpectValue("")
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestCovTextTypesAndFormats(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "text", label: "Plain", name: "plain"},
		{type: "email", label: "Email", name: "email"},
		{type: "tel", label: "Phone", name: "phone"},
		{type: "url", label: "Site", name: "site"},
		{type: "password", label: "Secret", name: "secret"}
	]}`)
	for label, typ := range map[string]string{"Plain": "text", "Email": "email", "Phone": "tel", "Site": "url", "Secret": "password"} {
		f.Field(label).Find("input").WaitAttr("type", typ)
	}
	email := f.Field("Email")
	for _, bad := range []string{"ada", "ada@", "Ada <ada@example.com>", "ada@example.com x"} {
		email.Type(bad)
		email.ExpectError("enter an email address")
		email.ExpectInvalid(true)
	}
	email.Type("ada@example.com")
	email.ExpectNoError()
	email.ExpectInvalid(false)

	site := f.Field("Site")
	for _, bad := range []string{"example.com", "/path", "https://", "mailto:ada@example.com"} {
		site.Type(bad)
		site.ExpectError("enter a full URL, like https://example.com")
	}
	site.Type("https://example.com/a?b=c")
	site.ExpectNoError()

	phone := f.Field("Phone")
	phone.Type("not a number")
	phone.ExpectNoError()
	phone.Type("+1 555 0100")

	f.Field("Plain").Type("  spaced  ")
	f.Field("Secret").Type("hunter2")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"plain": "  spaced  ", "email": "ada@example.com", "phone": "+1 555 0100", "site": "https://example.com/a?b=c", "secret": "hunter2"}, "submit": ""}`)
}

func TestCovTextLengthsCountCharacters(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "text", label: "Code", name: "code", minLength: 3, maxLength: 5},
		{type: "text", label: "Min", name: "min", minLength: 2},
		{type: "text", label: "Max", name: "max", maxLength: 2}
	]}`)
	code := f.Field("Code")
	code.Type("äö")
	code.ExpectError("use at least 3 characters")
	code.Type("äöü")
	code.ExpectNoError()
	code.Type("äöüßé")
	code.ExpectNoError()
	code.Type("äöüßéx")
	code.ExpectError("use at most 5 characters")
	code.Type("")
	code.ExpectNoError()
	code.Type("abc")

	f.Field("Min").Type("a")
	f.Field("Min").ExpectError("use at least 2 characters")
	f.Field("Min").Type("a very long value without a cap")
	f.Field("Min").ExpectNoError()
	f.Field("Max").Type("abc")
	f.Field("Max").ExpectError("use at most 2 characters")
	f.Field("Max").Type("ab")
	f.Field("Max").ExpectNoError()

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"code": "abc", "min": "a very long value without a cap", "max": "ab"}, "submit": ""}`)
}

func TestCovTextPatternMatchesWholeValue(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Zip", name: "zip", pattern: "[0-9]{3}"},
		{type: "text", label: "Pick", name: "pick", pattern: "a|b"}
	]}`)
	zip := f.Field("Zip")
	zip.Type("1234")
	zip.ExpectError("use the requested format")
	zip.Type("x123")
	zip.ExpectError("use the requested format")
	zip.Type("123")
	zip.ExpectNoError()
	pick := f.Field("Pick")
	pick.Type("ab")
	pick.ExpectError("use the requested format")
	pick.Type("b")
	pick.ExpectNoError()
	pick.Type("")
	pick.ExpectNoError()
}

func TestCovTextPatternLookaroundFailsCreation(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form({fields: [{type: "text", label: "Secret", name: "secret", pattern: "(?=.*[0-9]).{4,}"}]})); ui.notify(ui.draw())`})
	if msg := a.InputsScriptError(); !strings.Contains(msg, `field "secret" pattern`) {
		t.Fatalf("the script failed with %q, want it to name the field's pattern", msg)
	}
}

func TestCovTextChecksOrder(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "email", label: "Email", name: "email", minLength: 8, pattern: ".*@corp\\.com", validate: "(s) => s === 'boss@corp.com' ? 'reserved' : ''"}
	]}`)
	email := f.Field("Email")
	email.Type("a@b")
	email.ExpectError("use at least 8 characters")
	email.Type("ada-lovelace")
	email.ExpectError("enter an email address")
	email.Type("ada@example.com")
	email.ExpectError("use the requested format")
	email.Type("boss@corp.com")
	email.ExpectError("reserved")
	email.Type("ada@corp.com")
	email.ExpectNoError()
}

func TestCovValidateGetsValueAndClears(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "text", label: "Nick", name: "nick", validate: "(s) => s === s.toLowerCase() ? '' : 'lowercase only: ' + s"},
		{type: "number", label: "Lucky", name: "lucky", validate: "(n) => typeof n === 'number' && n % 2 === 1 ? 'even only' : ''"},
		{type: "date", label: "Day", name: "day", validate: "(s) => s.endsWith('-13') ? 'not the 13th' : ''"},
		{type: "textarea", label: "Bio", name: "bio", validate: "(s) => s.includes('\\n') ? 'one line' : ''"}
	]}`)
	nick := f.Field("Nick")
	nick.Type("Ada")
	nick.ExpectError("lowercase only: Ada")
	nick.ExpectInvalid(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	nick.Type("ada")
	nick.ExpectNoError()
	nick.ExpectInvalid(false)

	lucky := f.Field("Lucky")
	lucky.Type("3")
	lucky.ExpectError("even only")
	lucky.Type("4")
	lucky.ExpectNoError()

	day := f.Field("Day")
	day.Type("2026-03-13")
	day.ExpectError("not the 13th")
	day.Type("2026-03-14")
	day.ExpectNoError()

	bio := f.Field("Bio")
	bio.Type("a\nb")
	bio.ExpectError("one line")
	bio.Type("ab")
	bio.ExpectNoError()

	f.ExpectSubmitMessage("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"nick": "ada", "lucky": 4, "day": "2026-03-14", "bio": "ab"}, "submit": ""}`)
}

func TestCovFailingValidateIsSystemError(t *testing.T) {
	for _, c := range []struct{ kind, spec, value string }{
		{"text", `{type: "text", label: "Field", name: "field", validate: "(s) => { throw new Error('boom') }"}`, "x"},
		{"number", `{type: "number", label: "Field", name: "field", validate: "(n) => { if (n !== null) throw new Error('boom'); return '' }"}`, "1"},
		{"date", `{type: "date", label: "Field", name: "field", validate: "(s) => { if (s !== '') throw new Error('boom'); return '' }"}`, "2026-01-01"},
		{"textarea", `{type: "textarea", label: "Field", name: "field", validate: "(s) => { throw new Error('boom') }"}`, "x"},
		{"wrong result", `{type: "text", label: "Field", name: "field", validate: "(s) => 5"}`, "x"},
	} {
		t.Run(c.kind, func(t *testing.T) {
			a, f := formApp(t, `{fields: [`+c.spec+`]}`)
			field := f.Field("Field")
			field.ExpectNoError()
			field.Type(c.value)
			field.ExpectError("couldn't check this value")
			field.ExpectInvalid(true)
			field.ExpectRequiredMark(false)
			f.ExpectSubmitDisabled(true)
			f.ExpectSubmitMessage("form is not completed or contains errors")
			a.NextNotify("call_error")
			field.Type("")
			field.ExpectNoError()
			field.ExpectInvalid(false)
			f.ExpectSubmitDisabled(false)
		})
	}
}

func TestCovValidateSkippedWhileOtherChecksFail(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Code", name: "code", minLength: 3, validate: "(s) => 'never valid'"},
		{type: "number", label: "Count", name: "count", min: 1, validate: "(n) => 'never valid'"}
	]}`)
	f.Field("Code").Type("ab")
	f.Field("Code").ExpectError("use at least 3 characters")
	f.Field("Code").Type("abc")
	f.Field("Code").ExpectError("never valid")
	f.Field("Count").Type("0")
	f.Field("Count").ExpectError("enter 1 or more")
	f.Field("Count").Type("2")
	f.Field("Count").ExpectError("never valid")
}

func TestCovNumberValidateSkipsEmpty(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "number", label: "Count", name: "count", validate: "(n) => 'never ' + n"}]}`)
	count := f.Field("Count")
	count.Type("2")
	count.ExpectError("never 2")
	count.Type("")
	count.ExpectNoError()
	f.ExpectSubmitDisabled(false)
}

func TestCovDateValidateSkipsEmpty(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "date", label: "Day", name: "day", validate: "(s) => 'never ' + s"}]}`)
	day := f.Field("Day")
	day.Type("2026-01-02")
	day.ExpectError("never 2026-01-02")
	day.Type("")
	day.ExpectNoError()
	f.ExpectSubmitDisabled(false)
}

func TestCovNumberMinMax(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "number", label: "Temp", name: "temp", min: -40, max: -10},
		{type: "number", label: "Ratio", name: "ratio", min: 0.5, max: 1.5}
	]}`)
	temp := f.Field("Temp")
	temp.Find("input").WaitAttr("min", "-40")
	temp.Find("input").WaitAttr("max", "-10")
	temp.Type("-41")
	temp.ExpectError("enter -40 or more")
	temp.ExpectInvalid(true)
	temp.Type("-9.5")
	temp.ExpectError("enter -10 or less")
	temp.Type("-40")
	temp.ExpectNoError()
	temp.ExpectInvalid(false)
	temp.Type("-10")
	temp.ExpectNoError()

	ratio := f.Field("Ratio")
	ratio.Type("0.49")
	ratio.ExpectError("enter 0.5 or more")
	ratio.Type("1.51")
	ratio.ExpectError("enter 1.5 or less")
	ratio.Type("0.75")
	ratio.ExpectNoError()

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"temp": -10, "ratio": 0.75}, "submit": ""}`)
}

func TestCovNumberStep(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "number", label: "Odd", name: "odd", min: 1, step: 2},
		{type: "number", label: "Money", name: "money", step: 0.01},
		{type: "number", label: "Tenth", name: "tenth", step: 0.1},
		{type: "number", label: "Below", name: "below", min: -1, step: 0.5},
		{type: "number", label: "Free", name: "free"}
	]}`)
	odd := f.Field("Odd")
	odd.Type("4")
	odd.ExpectError("use steps of 2")
	odd.Type("5")
	odd.ExpectNoError()

	money := f.Field("Money")
	money.Type("1.155")
	money.ExpectError("use steps of 0.01")
	money.Type("1.15")
	money.ExpectNoError()

	tenth := f.Field("Tenth")
	tenth.Type("0.35")
	tenth.ExpectError("use steps of 0.1")
	tenth.Type("0.3")
	tenth.ExpectNoError()

	below := f.Field("Below")
	below.Type("-0.75")
	below.ExpectError("use steps of 0.5")
	below.Type("-0.5")
	below.ExpectNoError()

	free := f.Field("Free")
	free.Find("input").WaitAttr("step", "any")
	free.Type("3.14159")
	free.ExpectNoError()

	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"odd": 5, "money": 1.15, "tenth": 0.3, "below": -0.5, "free": 3.14159}, "submit": ""}`)
}

func TestCovNumberChecksOrder(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "number", label: "N", name: "n", min: 0, max: 10, step: 3}]}`)
	n := f.Field("N")
	n.Type("-1")
	n.ExpectError("enter 0 or more")
	n.Type("11")
	n.ExpectError("enter 10 or less")
	n.Type("4")
	n.ExpectError("use steps of 3")
	n.Type("9")
	n.ExpectNoError()
}

func TestCovRangeAlwaysHasValue(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "range", label: "Plain", name: "plain"},
		{type: "range", label: "Neg", name: "neg", min: -10, max: -2, step: 4},
		{type: "range", label: "Fine", name: "fine", min: 0, max: 1, step: 0.1},
		{type: "range", label: "Set", name: "set", min: 0, max: 10, value: 7, required: true}
	]}`)
	f.Field("Plain").ExpectValue("50")
	f.Field("Plain").Find("output").WaitText("50")
	f.Field("Neg").ExpectValue("-6")
	f.Field("Fine").ExpectValue("0.5")
	set := f.Field("Set")
	set.ExpectValue("7")
	set.ExpectRequiredMark(false)
	set.Find("input").WaitAttr("min", "0")
	set.Find("input").WaitAttr("max", "10")
	set.Type("2")
	set.Find("output").WaitText("2")
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"plain": 50, "neg": -6, "fine": 0.5, "set": 2}, "submit": ""}`)
}

func TestCovDefaultsFromParams(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name", value: "Ada", required: true},
		{type: "number", label: "Age", name: "age", value: 36, required: true},
		{type: "number", label: "Zero", name: "zero", value: 0},
		{type: "datetime-local", label: "At", name: "at", value: "2026-06-15T10:30", required: true},
		{type: "textarea", label: "Bio", name: "bio", value: "one\ntwo", required: true},
		{type: "checkbox", label: "Terms", name: "terms", checked: true, required: true}
	]}`)
	f.Field("Name").ExpectValue("Ada")
	f.Field("Age").ExpectValue("36")
	f.Field("Zero").ExpectValue("0")
	f.Field("At").ExpectValue("2026-06-15T10:30")
	f.Field("Bio").ExpectValue("one\ntwo")
	f.Field("Terms").ExpectValue("true")
	for _, label := range []string{"Name", "Age", "At", "Bio", "Terms"} {
		f.Field(label).ExpectRequiredMark(false)
	}
	f.ExpectSubmitDisabled(false)
	f.ExpectSubmitMessage("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"name": "Ada", "age": 36, "zero": 0, "at": "2026-06-15T10:30", "bio": "one\ntwo", "terms": true}, "submit": ""}`)
}

func TestCovInvalidDefaultShowsError(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Code", name: "code", value: "ab", minLength: 3},
		{type: "number", label: "Age", name: "age", value: 5, min: 18},
		{type: "date", label: "Day", name: "day", value: "2025-01-01", min: "2026-01-01"},
		{type: "textarea", label: "Bio", name: "bio", value: "toolong", maxLength: 3}
	]}`)
	f.Field("Code").ExpectError("use at least 3 characters")
	f.Field("Code").ExpectInvalid(true)
	f.Field("Age").ExpectError("enter 18 or more")
	f.Field("Age").ExpectInvalid(true)
	f.Field("Day").ExpectError("pick 2026-01-01 or later")
	f.Field("Day").ExpectInvalid(true)
	f.Field("Bio").ExpectError("use at most 3 characters")
	f.Field("Bio").ExpectInvalid(true)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.Field("Code").Type("abc")
	f.Field("Code").ExpectInvalid(false)
	f.Field("Age").Type("18")
	f.Field("Day").Type("2026-01-01")
	f.Field("Bio").Type("ok")
	f.ExpectSubmitDisabled(false)
}

func TestCovDefaultsFromParentOverrideParams(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: [
		{name: "Grace", age: 85, day: "1906-12-09", bio: "navy", ok: false},
		{age: 1},
		{}
	], field: {type: "group", label: "People", name: "people", fields: [
		{type: "text", label: "Name", name: "name", value: "Ada"},
		{type: "number", label: "Age", name: "age", value: 36},
		{type: "date", label: "Day", name: "day", value: "1815-12-10"},
		{type: "textarea", label: "Bio", name: "bio", value: "math"},
		{type: "checkbox", label: "Ok", name: "ok", checked: true}
	]}}]}`)
	people := f.List("People")
	people.ExpectCount(3)
	first := people.Item(0)
	first.Field("Name").ExpectValue("Grace")
	first.Field("Age").ExpectValue("85")
	first.Field("Day").ExpectValue("1906-12-09")
	first.Field("Bio").ExpectValue("navy")
	first.Field("Ok").ExpectValue("false")
	people.Item(1).Field("Name").ExpectValue("Ada")
	people.Item(1).Field("Age").ExpectValue("1")
	people.Item(2).Field("Ok").ExpectValue("true")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"people": [
		{"name": "Grace", "age": 85, "day": "1906-12-09", "bio": "navy", "ok": false},
		{"name": "Ada", "age": 1, "day": "1815-12-10", "bio": "math", "ok": true},
		{"name": "Ada", "age": 36, "day": "1815-12-10", "bio": "math", "ok": true}
	]}, "submit": ""}`)
}

func TestCovParentDefaultsPerKind(t *testing.T) {
	for _, c := range []struct{ kind, spec, value, shown, want string }{
		{"text", `{type: "text", label: "Item", name: "items", value: "p"}`, `"x"`, "x", `"x"`},
		{"number", `{type: "number", label: "Item", name: "items", value: 1}`, `-2.5`, "-2.5", `-2.5`},
		{"range", `{type: "range", label: "Item", name: "items", min: 0, max: 10}`, `3`, "3", `3`},
		{"date", `{type: "month", label: "Item", name: "items"}`, `"2026-02"`, "2026-02", `"2026-02"`},
		{"textarea", `{type: "textarea", label: "Item", name: "items"}`, `"a\nb"`, "a\nb", `"a\nb"`},
	} {
		t.Run(c.kind, func(t *testing.T) {
			a, f := formApp(t, `{fields: [{type: "list", value: [`+c.value+`], field: `+c.spec+`}]}`)
			items := f.List("Item")
			items.ExpectCount(1)
			items.Item(0).Control().ExpectValue(c.shown)
			f.Submit("Submit")
			stack.ExpectJSON(t, a.Submitted(), `{"data": {"items": [`+c.want+`]}, "submit": ""}`)
		})
	}
}

func TestCovWrongDefaultTypeFailsCreation(t *testing.T) {
	for _, c := range []struct{ kind, spec, value, want string }{
		{"text", `{type: "text", label: "Item", name: "items"}`, `5`, "want a string"},
		{"number", `{type: "number", label: "Item", name: "items"}`, `"5"`, "want a number"},
		{"range", `{type: "range", label: "Item", name: "items"}`, `true`, "want a number"},
		{"date", `{type: "date", label: "Item", name: "items"}`, `20260101`, "want a string"},
		{"textarea", `{type: "textarea", label: "Item", name: "items"}`, `["a"]`, "want a string"},
		{"checkbox in group", `{type: "group", label: "Item", name: "items", fields: [{type: "checkbox", label: "C", name: "c"}]}`, `{c: "yes"}`, "want a boolean"},
		{"file", `{type: "file", label: "Item", name: "items"}`, `5`, "want an array of paths"},
		{"file path string", `{type: "file", label: "Item", name: "items"}`, `"island/x/a.txt"`, "want an array of paths"},
		{"file item", `{type: "file", label: "Item", name: "items"}`, `[5]`, "want a path"},
		{"group member", `{type: "group", label: "Item", name: "items", fields: [{type: "number", label: "N", name: "n"}]}`, `{n: "one"}`, "want a number"},
		{"group", `{type: "group", label: "Item", name: "items", fields: [{type: "number", label: "N", name: "n"}]}`, `"one"`, "want an object"},
	} {
		t.Run(c.kind, func(t *testing.T) {
			a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form({fields: [{type: "list", value: [` + c.value + `], field: ` + c.spec + `}]})); ui.notify(ui.draw())`})
			if msg := a.InputsScriptError(); !strings.Contains(msg, c.want) {
				t.Fatalf("the script failed with %q, want it to say %q", msg, c.want)
			}
		})
	}
}

func TestCovWrongParamTypeFailsCreation(t *testing.T) {
	for _, c := range []struct{ kind, spec string }{
		{"text value", `{type: "text", label: "F", name: "f", value: 5}`},
		{"number value", `{type: "number", label: "F", name: "f", value: "5"}`},
		{"date value", `{type: "date", label: "F", name: "f", value: 20260101}`},
		{"textarea value", `{type: "textarea", label: "F", name: "f", value: false}`},
		{"checkbox checked", `{type: "checkbox", label: "F", name: "f", checked: "yes"}`},
		{"file value", `{type: "file", label: "F", name: "f", value: "a.txt"}`},
	} {
		t.Run(c.kind, func(t *testing.T) {
			a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form({fields: [` + c.spec + `]})); ui.notify(ui.draw())`})
			if msg := a.InputsScriptError(); msg == "" {
				t.Fatal("the script failed with no message")
			}
		})
	}
}

func TestCovDateTypes(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "date", label: "Date", name: "date"},
		{type: "time", label: "Time", name: "time"},
		{type: "datetime-local", label: "Local", name: "local"},
		{type: "month", label: "Month", name: "month"},
		{type: "week", label: "Week", name: "week"}
	]}`)
	values := map[string]string{"Date": "2026-02-28", "Time": "23:59", "Local": "2026-06-15T10:30", "Month": "2026-11", "Week": "2026-W53"}
	types := map[string]string{"Date": "date", "Time": "time", "Local": "datetime-local", "Month": "month", "Week": "week"}
	for label, value := range values {
		field := f.Field(label)
		field.Find("input").WaitAttr("type", types[label])
		field.Type(value)
		field.ExpectValue(value)
		field.ExpectNoError()
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"date": "2026-02-28", "time": "23:59", "local": "2026-06-15T10:30", "month": "2026-11", "week": "2026-W53"}, "submit": ""}`)
}

func TestCovDateMinMaxPerType(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "date", label: "Date", name: "date", min: "2026-01-10", max: "2026-01-20"},
		{type: "time", label: "Time", name: "time", min: "09:00", max: "17:30"},
		{type: "datetime-local", label: "Local", name: "local", min: "2026-06-15T09:00", max: "2026-06-15T18:00"},
		{type: "month", label: "Month", name: "month", min: "2026-03", max: "2026-09"},
		{type: "week", label: "Week", name: "week", min: "2026-W10", max: "2026-W20"}
	]}`)
	for _, c := range []struct{ label, min, max, before, after, inside string }{
		{"Date", "2026-01-10", "2026-01-20", "2026-01-09", "2026-01-21", "2026-01-10"},
		{"Time", "09:00", "17:30", "08:59", "17:31", "17:30"},
		{"Local", "2026-06-15T09:00", "2026-06-15T18:00", "2026-06-15T08:59", "2026-06-15T18:01", "2026-06-15T12:00"},
		{"Month", "2026-03", "2026-09", "2026-02", "2026-10", "2026-09"},
		{"Week", "2026-W10", "2026-W20", "2026-W09", "2026-W21", "2026-W15"},
	} {
		field := f.Field(c.label)
		field.Find("input").WaitAttr("min", c.min)
		field.Find("input").WaitAttr("max", c.max)
		field.Type(c.before)
		field.ExpectError("pick " + c.min + " or later")
		field.ExpectInvalid(true)
		field.Type(c.after)
		field.ExpectError("pick " + c.max + " or earlier")
		field.Type(c.inside)
		field.ExpectNoError()
		field.ExpectInvalid(false)
	}
	f.ExpectSubmitDisabled(false)
}

func TestCovTextareaRowsAndCount(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "textarea", label: "Rows", name: "rows", rows: 7},
		{type: "textarea", label: "Plain", name: "plain"},
		{type: "textarea", label: "MinOnly", name: "minonly", minLength: 3},
		{type: "textarea", label: "MaxOnly", name: "maxonly", maxLength: 4}
	]}`)
	f.Field("Rows").Find("textarea").WaitAttr("rows", "7")
	f.Field("Plain").Find("textarea").WaitNoAttr("rows")
	f.Field("Plain").Find("textarea").WaitNoAttr("maxlength")
	f.Field("Plain").WaitGone(".neisland-form-count")
	f.Field("Plain").Type("anything at all")
	f.Field("Plain").WaitGone(".neisland-form-count")

	minOnly := f.Field("MinOnly")
	minOnly.Find(".neisland-form-count").WaitText("0 / 3 min")
	minOnly.Type("ab")
	minOnly.Find(".neisland-form-count").WaitText("2 / 3 min")
	minOnly.Type("abc")
	minOnly.WaitGone(".neisland-form-count")
	minOnly.ExpectNoError()

	maxOnly := f.Field("MaxOnly")
	maxOnly.Find("textarea").WaitAttr("maxlength", "4")
	maxOnly.Find(".neisland-form-count").WaitText("0 / 4")
	maxOnly.Type("äö\nü")
	maxOnly.Find(".neisland-form-count").WaitText("4 / 4")
	maxOnly.ExpectNoError()
	maxOnly.Type("abcde")
	maxOnly.Find(".neisland-form-count").WaitText("5 / 4")
	maxOnly.ExpectError("use at most 4 characters")
}

func TestCovTextareaRequiredAndLengths(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "textarea", label: "Bio", name: "bio", required: true, minLength: 3, maxLength: 10}]}`)
	bio := f.Field("Bio")
	bio.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	bio.Type("ab")
	bio.ExpectRequiredMark(true)
	bio.ExpectError("use at least 3 characters")
	bio.ExpectInvalid(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	bio.Type("")
	bio.ExpectRequiredMark(true)
	bio.ExpectNoError()
	bio.ExpectInvalid(false)
	bio.Type("line\nline")
	bio.ExpectNoError()
	bio.ExpectRequiredMark(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"bio": "line\nline"}, "submit": ""}`)
}

func TestCovCheckboxFalseIsAValue(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "checkbox", label: "Untouched", name: "untouched"},
		{type: "checkbox", label: "Toggled", name: "toggled"}
	]}`)
	f.Field("Untouched").ExpectValue("false")
	f.Field("Toggled").Check()
	f.Field("Toggled").ExpectValue("true")
	f.Field("Toggled").Uncheck()
	f.Field("Toggled").ExpectValue("false")
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"untouched": false, "toggled": false}, "submit": ""}`)
}

func TestCovCheckboxRequired(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "checkbox", label: "I agree", name: "agree", required: true}]}`)
	agree := f.Field("I agree")
	agree.ExpectRequiredMark(true)
	agree.ExpectInvalid(false)
	agree.ExpectNoError()
	agree.Find("input").WaitAttr("aria-required", "true")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	agree.Check()
	agree.ExpectRequiredMark(false)
	f.ExpectSubmitMessage("")
	agree.Uncheck()
	agree.ExpectRequiredMark(true)
	agree.ExpectInvalid(false)
	agree.ExpectNoError()
	f.ExpectTextLacks("field is required")
	f.ExpectSubmitDisabled(true)
	agree.Check()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"agree": true}, "submit": ""}`)
}

func covFileApp(t *testing.T, spec string) (*stack.IslandApp, stack.Form, string, string) {
	dir := t.TempDir()
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Script: `ui.root.set(ui.form({fields: [` + spec + `]})); ui.notify(ui.draw())`})
	src := t.TempDir()
	for name, body := range map[string]string{"a.txt": "aaa", "b.txt": "bbbb", "c.TXT": "c", "pic.png": "png", "doc.pdf": "pdf"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return a, a.Form(), dir, src
}

func TestCovFileAccept(t *testing.T) {
	_, f, _, src := covFileApp(t, `
		{type: "file", label: "Ext", name: "ext", accept: ".txt"},
		{type: "file", label: "Wild", name: "wild", accept: "image/*"},
		{type: "file", label: "Mime", name: "mime", accept: "application/pdf"},
		{type: "file", label: "Mixed", name: "mixed", accept: " .pdf , image/png "}`)
	ext := f.Field("Ext")
	ext.Find("input").WaitAttr("accept", ".txt")
	ext.SetFiles(filepath.Join(src, "pic.png"))
	ext.ExpectFileFailed("pic.png", "not .txt")
	ext.ExpectInvalid(false)
	ext.ExpectNoError()
	ext.ExpectValue("")
	ext.SetFiles(filepath.Join(src, "c.TXT"))
	ext.ExpectFileChips("pic.png failed: not .txt", "c.TXT")
	ext.ExpectValue("c.TXT")
	ext.WaitGone(".neisland-form-file-zone")

	wild := f.Field("Wild")
	wild.SetFiles(filepath.Join(src, "a.txt"))
	wild.ExpectFileFailed("a.txt", "not image/*")
	wild.SetFiles(filepath.Join(src, "pic.png"))
	wild.ExpectFiles("pic.png")

	mime := f.Field("Mime")
	mime.SetFiles(filepath.Join(src, "pic.png"))
	mime.ExpectFileFailed("pic.png", "not application/pdf")
	mime.SetFiles(filepath.Join(src, "doc.pdf"))
	mime.ExpectFiles("doc.pdf")

	mixed := f.Field("Mixed")
	mixed.SetFiles(filepath.Join(src, "a.txt"))
	mixed.ExpectFileFailed("a.txt", "not .pdf or image/png")
	mixed.DismissFile("a.txt")
	mixed.WaitGone("li.neisland-form-file-chip")
	mixed.SetFiles(filepath.Join(src, "pic.png"))
	mixed.ExpectFiles("pic.png")
	f.ExpectSubmitDisabled(false)
}

func TestCovFileRequired(t *testing.T) {
	_, f, _, src := covFileApp(t, `
		{type: "file", label: "Doc", name: "doc", min: 1},
		{type: "file", label: "Docs", name: "docs", min: 1, max: 10}`)
	doc := f.Field("Doc")
	docs := f.Field("Docs")
	doc.ExpectRequiredMark(true)
	docs.ExpectRequiredMark(true)
	doc.ExpectInvalid(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	f.ExpectTextLacks("field is required")
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.ExpectRequiredMark(false)
	doc.ExpectValue("a.txt")
	docs.SetFiles(filepath.Join(src, "a.txt"))
	docs.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	docs.RemoveFile("a.txt")
	docs.ExpectValue("")
	docs.ExpectRequiredMark(true)
	docs.ExpectInvalid(false)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestCovFileOptionalEmptyIsAbsent(t *testing.T) {
	a, f, _, _ := covFileApp(t, `
		{type: "file", label: "Doc", name: "doc"},
		{type: "file", label: "Docs", name: "docs", max: 10}`)
	f.Field("Doc").ExpectValue("")
	f.Field("Docs").ExpectValue("")
	f.Field("Doc").Find(".neisland-form-file-summary").WaitText("Drop a file or click to choose")
	f.Field("Docs").Find(".neisland-form-file-summary").WaitText("Drop files or click to choose")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestCovFileManyPaths(t *testing.T) {
	a, f, dir, src := covFileApp(t, `{type: "file", label: "Docs", name: "docs", max: 10, accept: ".txt"}`)
	docs := f.Field("Docs")
	docs.Find("input").WaitAttr("multiple", "")
	docs.SetFiles(filepath.Join(src, "a.txt"))
	docs.ExpectValue("a.txt")
	docs.SetFiles(filepath.Join(src, "b.txt"), filepath.Join(src, "pic.png"))
	docs.ExpectFileChips("a.txt", "b.txt", "pic.png failed: not .txt")
	docs.ExpectValue("a.txt, b.txt")
	docs.DismissFile("pic.png")
	docs.ExpectFileChips("a.txt", "b.txt")
	docs.SetFiles(filepath.Join(src, "a.txt"))
	docs.ExpectValue("a.txt, b.txt, a (2).txt")
	f.Submit("Submit")
	var got struct {
		Data struct{ Docs []string }
	}
	if err := json.Unmarshal(a.Submitted(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Docs) != 3 {
		t.Fatalf("the file field gave %q, want three paths", got.Data.Docs)
	}
	want := []struct{ base, content string }{{"a.txt", "aaa"}, {"b.txt", "bbbb"}, {"a (2).txt", "aaa"}}
	for i, p := range got.Data.Docs {
		if filepath.Base(p) != want[i].base || filepath.Dir(filepath.Dir(p)) != "board.island" || filepath.Dir(p) != filepath.Dir(got.Data.Docs[0]) {
			t.Fatalf("the file field gave %q, want island/<id>/ a.txt, b.txt and a (2).txt", got.Data.Docs)
		}
		saved, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil || string(saved) != want[i].content {
			t.Fatalf("the workspace has %q (%v) at %s, want %q", saved, err, p, want[i].content)
		}
	}
}

func TestCovFileSingleRemoveThenPick(t *testing.T) {
	a, f, dir, src := covFileApp(t, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	doc.Find("input").WaitNoAttr("multiple")
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.ExpectValue("a.txt")
	doc.WaitGone(".neisland-form-file-zone")
	doc.WaitGone(".neisland-form-search-max")
	doc.Find(".neisland-form-file-link").WaitAttr("download", "a.txt")
	doc.RemoveFile("a.txt")
	doc.WaitCount(".neisland-form-file-zone", 1)
	doc.ExpectValue("")
	doc.SetFiles(filepath.Join(src, "b.txt"))
	doc.ExpectValue("b.txt")
	f.Submit("Submit")
	var got struct {
		Data struct{ Doc []string }
	}
	if err := json.Unmarshal(a.Submitted(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Doc) != 1 || filepath.Base(got.Data.Doc[0]) != "b.txt" || filepath.Dir(filepath.Dir(got.Data.Doc[0])) != "board.island" {
		t.Fatalf("the file field gave %q, want [island/<id>/b.txt]", got.Data.Doc)
	}
	saved, err := os.ReadFile(filepath.Join(dir, got.Data.Doc[0]))
	if err != nil || string(saved) != "bbbb" {
		t.Fatalf("the workspace has %q (%v), want bbbb", saved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Dir(got.Data.Doc[0]), "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("the removed a.txt is still in the workspace (%v)", err)
	}
}

func TestCovAriaInvalidOnlyForShownErrors(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "text", label: "Name", name: "name", required: true, maxLength: 2},
		{type: "number", label: "Age", name: "age", required: true, max: 5},
		{type: "date", label: "Day", name: "day", required: true, max: "2026-01-01"},
		{type: "textarea", label: "Bio", name: "bio", required: true, maxLength: 2}
	]}`)
	for _, c := range []struct{ label, bad, good string }{
		{"Name", "abc", "ab"}, {"Age", "6", "5"}, {"Day", "2026-01-02", "2026-01-01"}, {"Bio", "abc", "ab"},
	} {
		field := f.Field(c.label)
		field.ExpectInvalid(false)
		field.Find("[aria-required]").WaitAttr("aria-required", "true")
		field.Type(c.bad)
		field.ExpectInvalid(true)
		field.Type("")
		field.ExpectInvalid(false)
		field.Type(c.bad)
		field.ExpectInvalid(true)
		field.Type(c.good)
		field.ExpectInvalid(false)
	}
}
