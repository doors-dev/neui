//go:build browser

package devkit_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func fileApp(t *testing.T, files api.FilesConfig, spec string) (*stack.IslandApp, stack.Form, string, string) {
	dir := t.TempDir()
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Files: files, Script: `ui.root.set(ui.form({fields: [` + spec + `]})); while (true) ui.notify(ui.draw())`})
	src := t.TempDir()
	for name, body := range map[string]string{
		"a.txt":   "aaa",
		"b.txt":   "bbbb",
		"c.txt":   "c",
		"pic.png": "png",
		"big.txt": strings.Repeat("x", 300<<10),
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return a, a.Form(), dir, src
}

func fileFocus(a *stack.IslandApp) string {
	return a.Page.MustEval(`() => {
		const el = document.activeElement
		return el ? (el.getAttribute("aria-label") || el.id || el.tagName) : ""
	}`).String()
}

func expectFileFocus(t *testing.T, a *stack.IslandApp, want string) {
	t.Helper()
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		got := fileFocus(a)
		return got == want, got
	}, "focus on "+want)
}

func TestFileUploadsOnPickWithDownloadLink(t *testing.T) {
	a, f, dir, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	list := doc.Find("ul.kisland-form-search-chosen")
	list.WaitAttr("aria-live", "polite")
	list.WaitAttr("aria-relevant", "additions")
	id := *doc.Find("input").Element().MustAttribute("id")
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.ExpectFiles("a.txt")
	saved, err := os.ReadFile(filepath.Join(dir, "board.island", id, "a.txt"))
	if err != nil || string(saved) != "aaa" {
		t.Fatalf("before the submit the workspace has %q (%v) at board.island/%s/a.txt, want aaa", saved, err, id)
	}
	link := doc.Find(".kisland-form-file-link")
	link.WaitAttr("download", "a.txt")
	link.WaitAttr("title", "a.txt")
	doc.Find(".kisland-form-file-size").WaitText("3 B")
	doc.Find("li.kisland-form-file-chip > .kisland-form-remove").WaitAttr("aria-label", "Remove a.txt")
	if got := a.Page.MustEval(`() => fetch(document.querySelector(".kisland-form-file-link").href).then((r) => r.text())`).String(); got != "aaa" {
		t.Fatalf("the link downloads %q, want aaa", got)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"doc": "board.island/`+id+`/a.txt"}, "submit": ""}`)
}

func TestFileProgressBlocksSubmit(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	f.ExpectSubmitDisabled(false)
	a.ThrottleUploads(60 << 10)
	doc.PickFiles(filepath.Join(src, "big.txt"))
	doc.ExpectFileChips("big.txt uploading")
	doc.Find(`[role="progressbar"]`).WaitAttr("aria-label", "Uploading big.txt")
	doc.Find("li.kisland-form-file-chip > .kisland-form-remove").WaitAttr("aria-label", "Cancel upload of big.txt")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	doc.WaitGone("[aria-invalid]")
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		got := a.Page.MustEval(`() => {
			const bar = document.querySelector('[role="progressbar"]')
			const chip = bar && bar.closest("li")
			return bar ? bar.getAttribute("aria-valuenow") + " " + bar.textContent.trim() + " " + chip.style.getPropertyValue("--kisland-file-p") + " " + bar.getAttribute("aria-valuetext") : "none"
		}`).String()
		parts := strings.Fields(got)
		n, _ := strconv.Atoi(parts[0])
		return n > 0 && n < 100 && parts[1] == parts[0]+"%" && parts[2] == parts[0]+"%" && strings.Contains(got, "KB of 307 KB"), got
	}, "progress shown")
	a.Page.MustEval(`() => {
		const b = document.querySelector(".kisland-form button[type=submit]")
		b.disabled = false
		b.click()
	}`)
	a.ThrottleUploads(0)
	doc.ExpectFiles("big.txt")
	f.Submit("Submit")
	var got struct{ Data struct{ Doc string } }
	raw := a.Submitted()
	if err := json.Unmarshal(raw, &got); err != nil || path.Base(got.Data.Doc) != "big.txt" {
		t.Fatalf("the first submit gave %s, want the uploaded file: the forced one during the upload must be refused", raw)
	}
}

func TestFileCancelDeletesPartial(t *testing.T) {
	a, f, dir, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	id := *doc.Find("input").Element().MustAttribute("id")
	partial := filepath.Join(dir, "board.island", id, "big.txt")
	a.ThrottleUploads(20 << 10)
	defer a.ThrottleUploads(0)
	doc.PickFiles(filepath.Join(src, "big.txt"))
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		info, err := os.Stat(partial)
		return err == nil && info.Size() > 0, "no partial file yet"
	}, "the upload writes the partial file")
	doc.CancelFile("big.txt")
	doc.WaitGone("li.kisland-form-file-chip")
	stack.Eventually(t, stack.WaitTimeout, func() (bool, string) {
		_, err := os.Stat(partial)
		return os.IsNotExist(err), "still there"
	}, "the partial file is deleted")
	doc.ExpectValue("")
	expectFileFocus(t, a, id)
	f.ExpectSubmitDisabled(false)
}

func TestFileRemoveDeletesAndFocuses(t *testing.T) {
	a, f, dir, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Docs", name: "docs", multiple: true}`)
	docs := f.Field("Docs")
	id := *docs.Find("input").Element().MustAttribute("id")
	docs.SetFiles(filepath.Join(src, "a.txt"), filepath.Join(src, "b.txt"), filepath.Join(src, "pic.png"))
	docs.ExpectFiles("a.txt", "b.txt", "pic.png")
	docs.RemoveFile("a.txt")
	docs.ExpectValue("b.txt, pic.png")
	expectFileFocus(t, a, "Remove b.txt")
	if _, err := os.Stat(filepath.Join(dir, "board.island", id, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("the removed a.txt is still in the workspace (%v)", err)
	}
	docs.RemoveFile("pic.png")
	docs.ExpectValue("b.txt")
	expectFileFocus(t, a, "Remove b.txt")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"docs": ["board.island/`+id+`/b.txt"]}, "submit": ""}`)
}

func TestFileInterruptedThenRetry(t *testing.T) {
	a, f, dir, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Doc", name: "doc", required: true}`)
	doc := f.Field("Doc")
	id := *doc.Find("input").Element().MustAttribute("id")
	stop := a.FailUploads()
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.ExpectFileFailed("a.txt", "upload interrupted")
	doc.ExpectInvalid(false)
	doc.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	doc.Find("li.kisland-form-file-chip > .kisland-form-remove").WaitAttr("aria-label", "Dismiss a.txt")
	stop()
	doc.RetryFile("a.txt")
	doc.ExpectFiles("a.txt")
	saved, err := os.ReadFile(filepath.Join(dir, "board.island", id, "a.txt"))
	if err != nil || string(saved) != "aaa" {
		t.Fatalf("after the retry the workspace has %q (%v), want aaa", saved, err)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"doc": "board.island/`+id+`/a.txt"}, "submit": ""}`)
}

func TestFileRetryInListItem(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{}, `{type: "list", field: {type: "file", label: "Files", name: "files"}}`)
	files := f.List("Files")
	files.Add()
	item := files.Item(0).Control()
	stop := a.FailUploads()
	item.SetFiles(filepath.Join(src, "a.txt"))
	item.ExpectFileFailed("a.txt", "upload interrupted")
	stop()
	item.Find("button.kisland-form-file-retry").WaitNoAttr("hidden")
}

func TestFileRefusedChipsAreNotInTheValue(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{DocumentLimit: 100 << 10}, `{type: "file", label: "Docs", name: "docs", multiple: true, max: 2, accept: ".txt"}`)
	docs := f.Field("Docs")
	docs.SetFiles(filepath.Join(src, "big.txt"), filepath.Join(src, "pic.png"))
	docs.ExpectFileChips("big.txt failed: larger than 100 KB", "pic.png failed: not .txt")
	docs.ExpectValue("")
	docs.ExpectInvalid(false)
	f.ExpectSubmitDisabled(false)
	docs.DropFiles("a.txt", "aaa", "b.txt", "bbbb", "c.txt", "c")
	docs.ExpectFileChips("big.txt failed: larger than 100 KB", "pic.png failed: not .txt", "a.txt", "b.txt", "c.txt failed: at most 2 files")
	docs.ExpectValue("a.txt, b.txt")
	docs.WaitGone(".kisland-form-file-zone")
	docs.Find(".kisland-form-search-max").WaitText("Maximum of 2 chosen.")
	f.Submit("Submit")
	var got struct{ Data struct{ Docs []string } }
	raw := a.Submitted()
	if err := json.Unmarshal(raw, &got); err != nil || len(got.Data.Docs) != 2 || path.Base(got.Data.Docs[0]) != "a.txt" || path.Base(got.Data.Docs[1]) != "b.txt" {
		t.Fatalf("submitted %s, want the two uploaded files only", raw)
	}
}

func TestFileWorkspaceFull(t *testing.T) {
	_, f, _, src := fileApp(t, api.FilesConfig{MaxBytes: 64 << 10}, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	doc.SetFiles(filepath.Join(src, "big.txt"))
	doc.ExpectFileFailed("big.txt", "workspace is full")
	doc.ExpectValue("")
}

func TestFileMaxHidesBox(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Docs", name: "docs", multiple: true, max: 2}`)
	docs := f.Field("Docs")
	id := *docs.Find("input").Element().MustAttribute("id")
	list := docs.Find("ul.kisland-form-search-chosen")
	list.WaitNoAttr("aria-describedby")
	docs.SetFiles(filepath.Join(src, "a.txt"), filepath.Join(src, "b.txt"))
	docs.WaitGone(".kisland-form-file-zone")
	docs.WaitGone("input")
	max := docs.Find(".kisland-form-search-max")
	max.WaitText("Maximum of 2 chosen.")
	list.WaitAttr("aria-describedby", *max.Element().MustAttribute("id"))
	docs.RemoveFile("a.txt")
	docs.WaitCount(".kisland-form-file-zone", 1)
	docs.WaitGone(".kisland-form-search-max")
	list.WaitNoAttr("aria-describedby")
	expectFileFocus(t, a, id)
}

func TestFileSingleChipTakesBoxPlace(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Doc", name: "doc"}`)
	doc := f.Field("Doc")
	id := *doc.Find("input").Element().MustAttribute("id")
	tops := func(sel string) string {
		return a.Page.MustEval(`(sel) => {
			const el = document.querySelector(sel)
			const actions = document.querySelector(".kisland-form-actions")
			return el ? Math.round(el.getBoundingClientRect().top) + " " + Math.round(actions.getBoundingClientRect().top) : "none"
		}`, sel).String()
	}
	doc.Find(".kisland-form-file-zone").Element()
	zone := tops(".kisland-form-file-zone")
	doc.SetFiles(filepath.Join(src, "a.txt"))
	doc.WaitGone(".kisland-form-file-zone")
	if chip := tops("li.kisland-form-file-chip"); chip != zone {
		t.Fatalf("the chip and the actions sit at %s, want the box's %s", chip, zone)
	}
	doc.RemoveFile("a.txt")
	doc.WaitCount(".kisland-form-file-zone", 1)
	expectFileFocus(t, a, id)
}

func TestFileDefaultsRenderAsLinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"reports/q3.csv": "a,b\n1,2\n", "reports/notes.md": "# notes\n"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := stack.Island(t, devkit.App{Island: widgets(), Dir: dir, Script: `ui.root.set(ui.form({fields: [
		{type: "file", label: "Reports", name: "reports", multiple: true, value: ["reports/q3.csv", "./reports/notes.md"]}
	]})); while (true) ui.notify(ui.draw())`})
	f := a.Form()
	reports := f.Field("Reports")
	reports.ExpectFiles("q3.csv", "notes.md")
	reports.Find(".kisland-form-file-link").WaitAttr("download", "q3.csv")
	if got := a.Page.MustEval(`() => fetch(document.querySelector(".kisland-form-file-link").href).then((r) => r.text())`).String(); got != "a,b\n1,2\n" {
		t.Fatalf("the default's link downloads %q, want its content", got)
	}
	reports.RemoveFile("q3.csv")
	reports.ExpectValue("notes.md")
	if _, err := os.Stat(filepath.Join(dir, "reports", "q3.csv")); err != nil {
		t.Fatalf("removing a default deleted the agent's file: %v", err)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"reports": ["reports/notes.md"]}, "submit": ""}`)
}

func TestFileDefaultMustExist(t *testing.T) {
	for _, c := range []struct{ kind, spec, want string }{
		{"missing", `{type: "file", label: "F", name: "f", value: ["nope.txt"]}`, `default file "nope.txt" not found`},
		{"too many", `{type: "file", label: "F", name: "f", value: ["a.txt", "b.txt"]}`, "2 default files, most is 1"},
		{"max without multiple", `{type: "file", label: "F", name: "f", max: 2}`, "max needs multiple"},
	} {
		t.Run(c.kind, func(t *testing.T) {
			a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form({fields: [` + c.spec + `]})); ui.notify(ui.draw())`})
			if msg := a.InputsScriptError(); !strings.Contains(msg, c.want) {
				t.Fatalf("the script failed with %q, want it to say %q", msg, c.want)
			}
		})
	}
}

func TestFileLongNameTruncates(t *testing.T) {
	a, f, _, src := fileApp(t, api.FilesConfig{}, `{type: "file", label: "Docs", name: "docs", multiple: true}`)
	long := filepath.Join(src, "a very long file name that keeps going well past the chip width limit.txt")
	if err := os.WriteFile(long, bytes.Repeat([]byte("z"), 1500), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := f.Field("Docs")
	docs.SetFiles(long)
	docs.Find(".kisland-form-file-link").WaitAttr("title", filepath.Base(long))
	got := a.Page.MustEval(`() => {
		const chip = document.querySelector("li.kisland-form-file-chip")
		const field = chip.closest(".kisland-form-field").getBoundingClientRect()
		const rem = parseFloat(getComputedStyle(document.documentElement).fontSize)
		const box = chip.getBoundingClientRect()
		const link = chip.querySelector(".kisland-form-file-link")
		const inside = (el) => {
			const r = el.getBoundingClientRect()
			return r.width > 0 && r.left >= box.left - 0.5 && r.right <= box.right + 0.5
		}
		return JSON.stringify({
			narrow: box.width <= Math.min(16 * rem, field.width) + 0.5,
			ellipsis: getComputedStyle(link).textOverflow === "ellipsis" && link.scrollWidth > link.clientWidth,
			size: inside(chip.querySelector(".kisland-form-file-size")),
			remove: inside(chip.querySelector(".kisland-form-remove")),
			oneLine: box.height <= 2.25 * rem,
		})
	}`).String()
	if got != `{"narrow":true,"ellipsis":true,"size":true,"remove":true,"oneLine":true}` {
		t.Fatalf("the long name's chip measures %s, want it capped, the name cut with an ellipsis and size and × in view", got)
	}
	a.Page.MustSetViewport(240, 800, 1, false)
	docs.Find(".kisland-form-file-link").Element()
	narrow := a.Page.MustEval(`() => {
		const chip = document.querySelector("li.kisland-form-file-chip")
		const field = chip.closest(".kisland-form-field").getBoundingClientRect()
		return chip.getBoundingClientRect().right <= field.right + 0.5
	}`).Bool()
	if !narrow {
		t.Fatal("at 240px the chip overflows the field")
	}
}
