//go:build browser

package devkit_test

import (
	"strconv"
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func TestTextStyles(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var styles = ["plain", "card", "info", "success", "warning", "danger"]
		var texts = styles.map((s) => ui.text({text: "Import " + s + "\n\nRows were read.", style: s}))
		texts.push(ui.text({text: "Default style"}))
		ui.root.set(ui.grid({}, ...texts))
		ui.notify(ui.draw())
	`})
	for i, c := range []struct{ style, class, label, role, rule string }{
		{"plain", "", "", "", ""},
		{"card", " neisland-text-card", "", "", ""},
		{"info", " neisland-text-card neisland-tone-info", "Info:", "status", "rgb(3, 105, 161)"},
		{"success", " neisland-text-card neisland-tone-success", "Success:", "status", "rgb(21, 128, 61)"},
		{"warning", " neisland-text-card neisland-tone-warning", "Warning:", "alert", "rgb(180, 83, 9)"},
		{"danger", " neisland-text-card neisland-tone-danger", "Error:", "alert", "rgb(220, 38, 38)"},
	} {
		box := a.Find(".neisland-grid-cell:nth-child(" + strconv.Itoa(i+1) + ") .neisland-text")
		box.Find("p").WaitText("Import " + c.style)
		box.WaitAttr("class", "neisland-text neisland-prose"+c.class)
		if c.role == "" {
			box.WaitNoAttr("role")
			box.WaitCount(".neisland-tone-label", 0)
		} else {
			box.WaitAttr("role", c.role)
			box.WaitCount(".neisland-tone-label", 1)
			box.Find(".neisland-tone-label:first-child").WaitText(c.label)
		}
		got := box.Element().MustEval(`() => {
			const s = getComputedStyle(this), b = getComputedStyle(this, "::before"), a = getComputedStyle(this, "::after")
			const label = this.querySelector(".neisland-tone-label"), first = this.querySelector("p")
			const l = label && getComputedStyle(label)
			const shadow = s.boxShadow.split(/,(?![^(]*\))/).map((x) => x.trim()).filter((x) => !x.startsWith("rgba(0, 0, 0, 0)")).join(",") || "none"
			return [s.borderTopWidth + " " + s.borderRightWidth + " " + s.borderBottomWidth + " " + s.borderLeftWidth, s.borderTopLeftRadius, s.padding, s.backgroundColor, shadow, b.content, a.content, getComputedStyle(first).marginTop, l ? l.position + " " + l.width + " " + l.height + " " + l.overflow : "", this.textContent.replace(/\s+/g, " ").trim()].join("|")
		}`).Str()
		frame, shadow, hidden := "0px 0px 0px 0px|0px|0px", "none", ""
		if c.class != "" {
			frame = "1px 1px 1px 1px|6px|12px"
		}
		if c.rule != "" {
			shadow, hidden = c.rule+" 0px 3px 0px 0px inset", "absolute 1px 1px hidden"
		}
		if want := frame + "|rgba(0, 0, 0, 0)|" + shadow + "|none|none|0px|" + hidden + "|" + c.label + "Import " + c.style + " Rows were read."; got != want {
			t.Fatalf("%s: got %q, want %q", c.style, got, want)
		}
	}
	a.Find(".neisland-grid-cell:nth-child(7) .neisland-text").WaitAttr("class", "neisland-text neisland-prose")
}

func TestTextHeadings(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		ui.root.set(ui.text({text: "# One\n\n## Two\n\n### Three\n\nBody"}))
		ui.notify(ui.draw())
	`})
	box := a.Find(".neisland-text")
	box.Find("h1").WaitText("One")
	box.Find("h2").WaitText("Two")
	box.Find("h3").WaitText("Three")
	got := box.Element().MustEval(`() => ["h1", "h2", "h3", "p"].map((s) => getComputedStyle(this.querySelector(s)).fontSize).join(" ")`).Str()
	if got != "18px 16px 14px 14px" {
		t.Fatalf("heading sizes: %s", got)
	}
}

func TestTextStyleUpdate(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var tx = ui.text({text: "Saving"})
		ui.root.set(ui.column({}, tx, ui.button({text: "Next", value: "next"})))
		var styles = ["warning", "plain"]
		for (var s of styles) {
			var ev = ui.draw()
			tx.update({text: "Saved with " + s, style: s})
			ui.notify(ev)
		}
		while (true) ui.draw()
	`})
	box := a.Find(".neisland-text")
	box.WaitAttr("class", "neisland-text neisland-prose")
	a.Find(".neisland-button").Click()
	box.WaitAttr("class", "neisland-text neisland-prose neisland-text-card neisland-tone-warning")
	box.WaitAttr("role", "alert")
	box.Find(".neisland-tone-label").WaitText("Warning:")
	box.Find("p").WaitText("Saved with warning")
	a.Notified()
	a.Find(".neisland-button").Click()
	box.Find("p").WaitText("Saved with plain")
	box.WaitAttr("class", "neisland-text neisland-prose")
	box.WaitNoAttr("role")
	box.WaitCount(".neisland-tone-label", 0)
}

func TestTextSizeRejected(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.text({text: "Title", size: "lg"})); ui.notify(ui.draw())`})
	if got := a.ContainerScriptError(); !strings.Contains(got, "size") {
		t.Fatalf("got %q, want an error naming size", got)
	}
}
