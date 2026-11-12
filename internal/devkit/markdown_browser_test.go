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
		ui.root.set(ui.text({text: "# Review\n\nLate fell to **9%**,\nsee [site](https://example.com) and [rel](notes.md).\n\n- [x] done\n\n| Region | Late |\n|:--|--:|\n| North | 8% |\n\n` + "```" + `\ncode <b>\n` + "```" + `\n\n<script>window.bad = 1</script><b>raw</b>"}))
		ui.notify(ui.draw())`})
	box := a.Find(".kisland-text.kisland-prose")
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
		ui.root.set(ui.text({text: "Get [the export](file:///reports/q3%20final.csv), [file:///notes.txt](file:///notes.txt) or [old](file:///reports/q2.csv)."}))
		ui.notify(ui.draw())`})
	box := a.Find(".kisland-text")
	chip := box.Find("a.kisland-file")
	chip.WaitAttr("class", "kisland-file not-prose")
	chip.WaitAttr("download", "q3 final.csv")
	chip.WaitAttr("title", "reports/q3 final.csv")
	chip.Find(".kisland-file-name").WaitText("the export")
	chip.Find(".kisland-file-meta").WaitText("25 B")
	box.Find("a.kisland-file:nth-of-type(2) .kisland-file-name").WaitText("notes.txt")
	got := chip.Element().MustEval(`() => fetch(this.href).then(async (r) => r.status + "|" + r.headers.get("content-disposition") + "|" + r.headers.get("x-content-type-options") + "|" + await r.text())`).Str()
	if want := `200|attachment; filename="q3 final.csv"|nosniff|region,orders` + "\nNorth,1204\n"; got != want {
		t.Fatalf("download: got %q, want %q", got, want)
	}
	gone := box.Find("span.kisland-file")
	gone.WaitAttr("class", "kisland-file kisland-file-gone not-prose")
	gone.WaitAttr("title", "reports/q2.csv")
	gone.WaitText("old file is gone")
	box.WaitCount(`a[href^="file:"]`, 0)
}

func TestMarkdownTableCells(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: markdownWorkspace(t), Script: `
		ui.root.set(ui.table({columns: [{title: "File", sort: true}, {title: "Note"}], search: true, selectable: true, rows: [
			{id: "a", cells: ["[zeta](file:///notes.txt)", "**b**old # not a heading"]},
			{id: "b", cells: ["*alpha*", "` + "`code`" + ` [site](https://example.com)"]},
			{id: "c", cells: ["**Mid**", "[gone](file:///missing.txt)"]}
		]}))
		ui.notify(ui.draw())`})
	grid := a.Find(".kisland-table-grid")
	first := grid.Find(".kisland-table-row:nth-child(1) .kisland-table-cell:nth-child(2)")
	first.WaitText("zeta 13 B")
	grid.Find(".kisland-table-row:nth-child(1) .kisland-table-cell:nth-child(3) .kisland-table-prose strong").WaitText("b")
	grid.Find(".kisland-table-row:nth-child(1) .kisland-table-cell:nth-child(3)").WaitText("bold # not a heading")
	grid.Find(".kisland-table-row:nth-child(2) .kisland-table-cell:nth-child(3) code").WaitText("code")
	grid.Find(".kisland-table-row:nth-child(2) .kisland-table-cell:nth-child(3) a").WaitAttr("target", "_blank")
	grid.Find(".kisland-table-row:nth-child(3) .kisland-file-gone").WaitText("gone file is gone")
	grid.WaitCount(`input[aria-label="Select alpha"]`, 1)
	grid.WaitCount(`h1, ul`, 0)

	grid.Find(".kisland-table-sort").Click()
	first.WaitText("alpha")
	grid.Find(".kisland-table-row:nth-child(2) .kisland-table-cell:nth-child(2)").WaitText("Mid")

	search := a.Find(".kisland-table-search")
	search.Element().MustInput("bold")
	grid.WaitCount(".kisland-table-row", 1)
	first.WaitText("zeta 13 B")
	search.Element().MustSelectAllText().MustInput("**")
	grid.WaitCount(".kisland-table-row", 0)
	search.Element().MustSelectAllText().MustInput("site")
	grid.WaitCount(".kisland-table-row", 1)
	first.WaitText("alpha")
}
