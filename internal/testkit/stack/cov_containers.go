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

	"neui.dev/internal/devkit"
)

// ContainerScriptError waits for the next script_error report, as a script throwing
// on a failed ui.form sends it, and returns its message.
func (a *IslandApp) ContainerScriptError() string {
	a.t.Helper()
	return a.containerNote("script_error")
}

// ContainerWidgetError waits for the next widget_error report and returns its error.
func (a *IslandApp) ContainerWidgetError() string {
	a.t.Helper()
	var data struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(a.containerNote("widget_error")), &data); err != nil {
		a.t.Fatal(err)
	}
	return data.Error
}

func (a *IslandApp) containerNote(kind string) string {
	a.t.Helper()
	reports := a.Until(func(r devkit.Report) bool {
		return r.Kind == "notify" && r.Meta["kind"] == kind || r.Kind == "error"
	})
	r := reports[len(reports)-1]
	if r.Kind == "error" {
		a.t.Fatalf("the run ended with %v before a %s", r.Err, kind)
	}
	if kind == "script_error" {
		var msg string
		if err := json.Unmarshal(r.Data, &msg); err == nil {
			return msg
		}
	}
	return string(r.Data)
}

// ExpectItemName waits until item i's body is named want, as group and series
// items are.
func (l List) ExpectItemName(i int, want string) {
	l.a.t.Helper()
	l.Item(i).Find(".neisland-form-item-body").WaitAttr("aria-label", want)
}

// ExpectRemoveName waits until the remove button of item i is named want.
func (l List) ExpectRemoveName(i int, want string) {
	l.a.t.Helper()
	l.Item(i).Find(":scope > button.neisland-form-item-remove").WaitAttr("aria-label", want)
}

// ExpectHelp waits until the help line under the list reads text; "" expects
// none.
func (l List) ExpectHelp(text string) {
	l.a.t.Helper()
	foot := l.Find(":scope > .neisland-form-items-foot")
	if text == "" {
		foot.WaitGone(".neisland-form-help")
		return
	}
	foot.Find(".neisland-form-help").WaitText(text)
}

// ExpectCompletedText waits until the series' completed row reads text.
func (s Series) ExpectCompletedText(text string) {
	s.a.t.Helper()
	s.Find(":scope > ol.neisland-form-steps > li.neisland-form-step[data-complete]").WaitText(text)
}
