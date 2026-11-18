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
	"os"
	"path/filepath"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func markdownWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"reports/q3 final.csv": "region,orders\nNorth,1204\n",
		"notes.txt":            "carrier notes",
	} {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMarkdownText(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `
		ui.root.set(ui.markdown({text: "# Review\n\nLate fell to **9%**,\nsee [site](https://example.com) and [rel](notes.md).\n\n- [x] done\n\n| Region | Late |\n|:--|--:|\n| North | 8% |\n\n` + "```" + `\ncode <b>\n` + "```" + `\n\n<script>window.bad = 1</script><b>raw</b>"}))
		ui.notify(ui.draw())`})
	box := a.Find(".neisland-text.neisland-prose")
	box.Find("h1").WaitText("Review")
	box.Find("p strong").WaitText("9%")
	box.WaitCount("p br", 1)
	link := box.Find(`a[href="https://example.com"]`)
	link.WaitAttr("target", "_blank")
	link.WaitAttr("rel", "noopener noreferrer")
	box.WaitCount(`a[href="notes.md"]`, 0)
	box.WaitCount(`li > input[type=checkbox][disabled][checked]`, 1)
	region := box.Find(`div[role=region]`)
	region.WaitAttr("aria-label", "Table: Region, Late")
	region.WaitAttr("tabindex", "0")
	box.Find("td:last-child").WaitAttr("align", "right")
	box.Find("pre").WaitAttr("tabindex", "0")
	box.Find("pre code").WaitText("code <b>")
	box.WaitCount("script, p > b", 0)
	if bad := a.Page.MustEval(`() => String(window.bad)`).Str(); bad != "undefined" {
		t.Fatalf("raw script ran: %s", bad)
	}
}

func TestMarkdownFileLinks(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: markdownWorkspace(t), Script: `
		ui.root.set(ui.markdown({text: "Get [the export](file:///reports/q3%20final.csv), [file:///notes.txt](file:///notes.txt) or [old](file:///reports/q2.csv)."}))
		ui.notify(ui.draw())`})
	box := a.Find(".neisland-text")
	chip := box.Find("a.neisland-file")
	chip.WaitAttr("class", "neisland-file not-prose")
	chip.WaitAttr("download", "q3 final.csv")
	chip.WaitAttr("title", "reports/q3 final.csv")
	chip.Find(".neisland-file-name").WaitText("the export")
	chip.Find(".neisland-file-meta").WaitText("25 B")
	box.Find("a.neisland-file:nth-of-type(2) .neisland-file-name").WaitText("notes.txt")
	got := chip.Element().MustEval(`() => fetch(this.href).then(async (r) => r.status + "|" + r.headers.get("content-disposition") + "|" + r.headers.get("x-content-type-options") + "|" + await r.text())`).Str()
	if want := `200|attachment; filename="q3 final.csv"|nosniff|region,orders` + "\nNorth,1204\n"; got != want {
		t.Fatalf("download: got %q, want %q", got, want)
	}
	gone := box.Find("span.neisland-file")
	gone.WaitAttr("class", "neisland-file neisland-file-gone not-prose")
	gone.WaitAttr("title", "reports/q2.csv")
	gone.WaitText("old file is gone")
	box.WaitCount(`a[href^="file:"]`, 0)
}

func TestMarkdownTableCells(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: markdownWorkspace(t), Script: `
		ui.root.set(ui.table({columns: [{title: "File", sort: true}, {title: "Note"}], search: true, select: {}, rows: [
			{id: "a", cells: ["[zeta](file:///notes.txt)", "**b**old # not a heading"]},
			{id: "b", cells: ["*alpha*", "` + "`code`" + ` [site](https://example.com)"]},
			{id: "c", cells: ["**Mid**", "[gone](file:///missing.txt)"]}
		]}))
		ui.notify(ui.draw())`})
	grid := a.Find(".neisland-table-grid")
	first := grid.Find(".neisland-table-row:nth-child(1) .neisland-table-cell:nth-child(2)")
	first.WaitText("zeta 13 B")
	grid.Find(".neisland-table-row:nth-child(1) .neisland-table-cell:nth-child(3) .neisland-table-prose strong").WaitText("b")
	grid.Find(".neisland-table-row:nth-child(1) .neisland-table-cell:nth-child(3)").WaitText("bold # not a heading")
	grid.Find(".neisland-table-row:nth-child(2) .neisland-table-cell:nth-child(3) code").WaitText("code")
	grid.Find(".neisland-table-row:nth-child(2) .neisland-table-cell:nth-child(3) a").WaitAttr("target", "_blank")
	grid.Find(".neisland-table-row:nth-child(3) .neisland-file-gone").WaitText("gone file is gone")
	grid.WaitCount(`input[aria-label="Select alpha"]`, 1)
	grid.WaitCount(`h1, ul`, 0)

	grid.Find(".neisland-table-sort").Click()
	first.WaitText("alpha")
	grid.Find(".neisland-table-row:nth-child(2) .neisland-table-cell:nth-child(2)").WaitText("Mid")

	search := a.Find(".neisland-table-search")
	search.Element().MustInput("bold")
	grid.WaitCount(".neisland-table-row", 1)
	first.WaitText("zeta 13 B")
	search.Element().MustSelectAllText().MustInput("**")
	grid.WaitCount(".neisland-table-row", 0)
	search.Element().MustSelectAllText().MustInput("site")
	grid.WaitCount(".neisland-table-row", 1)
	first.WaitText("alpha")
}
