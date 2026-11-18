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
	"strconv"
	"testing"

	"neui.dev/internal/testkit/stack"
)

const novalidateSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "email", label: "Email", name: "email"}
], submits: [{text: "Save", value: "save"}, {text: "Skip", value: "skip", novalidate: true}]}`

func novalidateExpectDisabled(t *testing.T, button stack.Locator, want bool) {
	t.Helper()
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		got := button.Element().MustEval(`() => this.disabled`).Bool()
		return got == want, strconv.FormatBool(got)
	}, "button disabled is "+strconv.FormatBool(want))
}

func TestNovalidateSubmitGivesTheDataAsEntered(t *testing.T) {
	a, f := covSubmitApp(t, "", novalidateSpec)
	save := f.Find(`button[type=submit][value="save"]`)
	skip := f.Find(`button[type=submit][value="skip"]`)
	novalidateExpectDisabled(t, save, true)
	novalidateExpectDisabled(t, skip, false)
	if !skip.Element().MustEval(`() => this.formNoValidate`).Bool() || save.Element().MustEval(`() => this.formNoValidate`).Bool() {
		t.Fatal("formnovalidate is not on Skip alone")
	}
	id := f.Find(".neisland-form-help").Element().MustAttribute("id")
	save.WaitAttr("aria-describedby", *id)
	skip.WaitNoAttr("aria-describedby")

	f.Submit("Skip")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {}, "submit": "skip"}`)

	f.Field("Email").Type("not-an-email")
	f.Field("Email").ExpectError("enter an email address")
	novalidateExpectDisabled(t, save, true)
	novalidateExpectDisabled(t, skip, false)
	f.Submit("Skip")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"email": "not-an-email"}, "submit": "skip"}`)

	f.Field("Name").Type("Ada")
	f.Field("Name").ExpectRequiredMark(false)
	novalidateExpectDisabled(t, save, true)
	f.Submit("Skip")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"name": "Ada", "email": "not-an-email"}, "submit": "skip"}`)

	f.Field("Email").Type("ada@example.com")
	f.ExpectSubmitMessage("")
	novalidateExpectDisabled(t, save, false)
	f.Submit("Save")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"name": "Ada", "email": "ada@example.com"}, "submit": "save"}`)
}

func TestNovalidateLeavesTheValidatedButtonLocked(t *testing.T) {
	a, f := covSubmitApp(t, "", novalidateSpec)
	save := f.Find(`button[type=submit][value="save"]`)
	f.Element().MustEval(`() => {
		window.novalidateSubmits = 0
		this.addEventListener("submit", () => window.novalidateSubmits++)
	}`)
	force := func() {
		t.Helper()
		save.Element().MustEval(`() => {
			this.disabled = false
			this.formNoValidate = true
			this.click()
		}`)
		a.Settle()
	}
	submits := func() int {
		return f.Element().MustEval(`() => window.novalidateSubmits`).Int()
	}

	novalidateExpectDisabled(t, save, true)
	force()

	f.Field("Email").Type("not-an-email")
	f.Field("Email").ExpectError("enter an email address")
	f.Field("Name").Type("Ada")
	f.Field("Name").ExpectRequiredMark(false)
	novalidateExpectDisabled(t, save, true)
	f.Field("Name").PressEnter()
	a.Settle()
	force()

	if n := submits(); n != 2 {
		t.Fatalf("the page sent %d submits, want the 2 forced through Save and none from Enter", n)
	}
	f.Field("Name").Type("Bea")
	f.Submit("Skip")
	stack.ExpectJSON(t, covSubmitted(a), `{"data": {"name": "Bea", "email": "not-an-email"}, "submit": "skip"}`)
}
