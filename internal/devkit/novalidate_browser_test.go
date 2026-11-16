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
	save := f.Find(`button[data-submit="save"]`)
	skip := f.Find(`button[data-submit="skip"]`)
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
	save := f.Find(`button[data-submit="save"]`)
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
