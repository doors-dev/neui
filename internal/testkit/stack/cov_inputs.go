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

package stack

import (
	"encoding/json"
	"fmt"
	"strings"

	"neui.dev/internal/devkit"
)

// NextNotify waits for the next notification of kind, such as
// "call_error" or "widget_error", passing over the reports before it.
func (a *IslandApp) NextNotify(kind string) devkit.Report {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool { return r.Kind == "notify" && r.Meta["kind"] == kind })
	return reports[len(reports)-1]
}

// InputsScriptError waits until the script fails, as it does when ui.form
// throws, and returns its error.
func (a *IslandApp) InputsScriptError() string {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool {
		return r.Kind == "error" || r.Kind == "notify" && r.Meta["kind"] == "script_error"
	})
	r := reports[len(reports)-1]
	if r.Kind == "error" {
		return r.Err.Error()
	}
	var msg string
	if err := json.Unmarshal(r.Data, &msg); err != nil {
		a.t.Fatalf("script_error %s: %v", r.Data, err)
	}
	return msg
}

// SubmittedPastErrors is Submitted that passes over the call and widget
// errors reported before the submit.
func (a *IslandApp) SubmittedPastErrors() json.RawMessage {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool {
		return r.Kind != "ok" && !(r.Kind == "notify" && (r.Meta["kind"] == "call_error" || r.Meta["kind"] == "widget_error"))
	})
	r := reports[len(reports)-1]
	if r.Kind != "notify" || r.Meta["kind"] != "notifications_list" {
		a.t.Fatalf("got %v, want a notifications_list", r)
	}
	var list []struct {
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(r.Data, &list); err != nil {
		a.t.Fatalf("notification %s: %v", r.Data, err)
	}
	if len(list) != 1 {
		a.t.Fatalf("got %d notifications %s, want 1", len(list), r.Data)
	}
	return list[0].Output
}

// ExpectTextLacks waits until the element's text doesn't contain s.
func (l Locator) ExpectTextLacks(s string) {
	l.a.t.Helper()
	Eventually(l.a.t, WaitTimeout, func() (bool, string) {
		got, err := l.eval("text", nil)
		if err != nil {
			return false, evalErr(err)
		}
		return !strings.Contains(got, s), fmt.Sprintf("%q", got)
	}, fmt.Sprintf("%s: text lacks %q", l.desc, s))
}
