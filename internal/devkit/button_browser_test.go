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
