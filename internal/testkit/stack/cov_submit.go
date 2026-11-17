//go:build browser

package stack

import (
	"encoding/json"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// PressEnter focuses the field's text control and presses Enter, as the
// user does to submit from the keyboard.
func (f Field) PressEnter() {
	f.a.t.Helper()
	f.Find("input:not([type=hidden]), textarea").Element().MustFocus()
	f.a.Page.Keyboard.MustType(input.Enter)
}

// RequestSubmit submits the form from script with no submitter, the way
// a submit that skips the disabled buttons reaches the server.
func (f Form) RequestSubmit() {
	f.a.t.Helper()
	f.Element().MustEval(`() => this.requestSubmit()`)
}

// SubmitButton is a submit button as shown.
type SubmitButton struct {
	Text  string `json:"text"`
	Value string `json:"value"`
	Class string `json:"class"`
}

// SubmitButtons lists the submit buttons in display order.
func (f Form) SubmitButtons() []SubmitButton {
	f.a.t.Helper()
	raw := f.Find(".neisland-form-actions").Element().MustEval(`() => JSON.stringify(Array.from(this.querySelectorAll("button[type=submit]")).map((b) => ({text: b.textContent.replace(/\s+/g, " ").trim(), value: b.value, class: b.className})))`).Str()
	var buttons []SubmitButton
	if err := json.Unmarshal([]byte(raw), &buttons); err != nil {
		f.a.t.Fatal(err)
	}
	return buttons
}

// ActionTexts lists the texts of the form's action buttons in display order.
func (f Form) ActionTexts() []string {
	f.a.t.Helper()
	raw := f.Find(".neisland-form-actions").Element().MustEval(`() => JSON.stringify(Array.from(this.querySelectorAll("button")).map((b) => b.textContent.replace(/\s+/g, " ").trim()))`).Str()
	var texts []string
	if err := json.Unmarshal([]byte(raw), &texts); err != nil {
		f.a.t.Fatal(err)
	}
	return texts
}

// Settle waits out the moment after a submit, while the app
// still rejects submits until the script draws again.
func (a *IslandApp) Settle() {
	time.Sleep(500 * time.Millisecond)
}
