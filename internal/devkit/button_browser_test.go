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
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func TestButtonDisabledIgnoresAForcedClick(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var b = ui.button({text: "Go", value: "go", disabled: true})
		ui.root.set(b)
		ui.notify({first: ui.draw(3000)})
		b.update({disabled: false})
		ui.notify({second: ui.draw()})`})
	button := a.Find(".neisland-button")
	button.WaitAttr("disabled", "")
	button.Element().MustEval(`() => { this.disabled = false; this.click(); this.disabled = true }`)
	stack.ExpectJSON(t, a.Notified().Data, `[{"first": null}]`)
	button.WaitNoAttr("disabled")
	button.Click()
	if got := string(a.Notified().Data); !strings.Contains(got, `"output":"go"`) {
		t.Fatalf("the enabled button notified %s, want its value go", got)
	}
}
