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
	"context"
	"strconv"
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func TestTextStyles(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		var styles = ["plain", "card", "info", "success", "warning", "danger"]
		var texts = styles.map((s) => ui.markdown({text: "Import " + s + "\n\nRows were read.", style: s}))
		texts.push(ui.markdown({text: "Default style"}))
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
		ui.root.set(ui.markdown({text: "# One\n\n## Two\n\n### Three\n\nBody"}))
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
		var tx = ui.markdown({text: "Saving"})
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
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.markdown(Object({text: "Title", size: "lg"}))); ui.notify(ui.draw())`})
	if got := a.ContainerScriptError(); !strings.Contains(got, "size") {
		t.Fatalf("got %q, want an error naming size", got)
	}
}

const emptyTextJS = `() => {
	const boxes = Array.from(document.querySelectorAll(".neisland-text"))
	const column = document.querySelector(".neisland-column")
	const first = boxes[0], last = boxes[boxes.length - 1]
	const oneGap = !!column && boxes.length > 1 && Math.abs(last.getBoundingClientRect().top - first.getBoundingClientRect().bottom - parseFloat(getComputedStyle(column).rowGap)) < 0.5
	return boxes.map((n) => n.className.replace("neisland-text neisland-prose", "").trim() + ":" + (n.querySelector("p")?.textContent ?? "")).join("|") + " " + oneGap
}`

func TestTextEmptyShowsNothing(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets()})
	for i, step := range []struct{ code, want, what string }{
		{`var styles = ["plain", "card", "info", "success", "warning", "danger"]
			var m = ui.markdown({text: "", style: "card"})
			ui.root.set(ui.column({}, ui.markdown({text: "Top"}), ...styles.map((s) => ui.markdown({text: "", style: s})), m, ui.markdown({text: "Bottom"})))`,
			":Top|:Bottom true", "empty texts of every style show no box and take no gap"},
		{`m.update({text: "Back"})`, ":Top|neisland-text-card:Back|:Bottom false", "a text patch shows the text"},
		{`m.update({style: "info"})`, ":Top|neisland-text-card neisland-tone-info:Back|:Bottom false", "a patch without text keeps it"},
		{`m.update({text: ""})`, ":Top|:Bottom true", "an empty text patch clears it"},
	} {
		code := step.code + "\nui.draw(1)\nui.notify(\"step " + strconv.Itoa(i) + "\")"
		var err error
		if i == 0 {
			_, _, err = a.Start(context.Background(), code)
		} else {
			_, _, err = a.Continue(code, false)
		}
		if err != nil {
			t.Fatal(err)
		}
		a.Until(func(r devkit.Report) bool { return strings.Contains(string(r.Data), `"step `+strconv.Itoa(i)+`"`) })
		expectEval(t, a, emptyTextJS, step.want, step.what)
	}
}
