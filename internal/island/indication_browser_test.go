//go:build browser

package island_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit"
)

func twoIslands() gox.Elem {
	return func(cur gox.Cursor) error {
		for _, name := range []string{"board", "other"} {
			if err := cur.Comp(neui.Island{
				Name:        name,
				Desc:        "a board",
				Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
				Template: func(slot gox.Comp) gox.Elem {
					return func(cur gox.Cursor) error { return cur.Comp(slot) }
				},
				Library: nekit.IslandRegister,
			}); err != nil {
				return err
			}
		}
		return nil
	}
}

type pending struct {
	t *testing.T
	p *testkit.Provider
	c *stack.Chat
}

func openPending(t *testing.T, apps map[string]string) *pending {
	t.Helper()
	a := &pending{t: t, p: testkit.NewProvider(api.Model{})}
	a.c = stack.Browser(t, stack.Host{Provider: a.p, Page: twoIslands()})
	for path, code := range apps {
		spec := a.result("page__island_spec", map[string]any{"path": path})
		text, _ := spec.Content[0].(api.ContentText)
		revision, _, _ := strings.Cut(strings.TrimPrefix(text.Text, "revision "), "\n")
		if r := a.result("page__island_begin", map[string]any{"path": path, "revision": revision, "code": code, "comment": "the " + path + " app"}); r.Error != api.ToolOk {
			t.Fatalf("island_begin %s failed: %#v", path, r.Content)
		}
	}
	return a
}

func (a *pending) result(name string, args any) api.ContentToolResult {
	a.t.Helper()
	a.p.Push(testkit.Call(name, args), testkit.Say("done"))
	a.c.Send("go on")
	a.c.Wait()
	requests := a.p.Requests()
	return testkit.Results(requests[len(requests)-1])[0]
}

func (a *pending) states(want map[string]string) {
	a.t.Helper()
	stack.Eventually(a.t, 10*time.Second, func() (bool, string) {
		got, err := a.c.Page.Eval(`() => Object.fromEntries([...document.querySelectorAll("button, form")].map((e) => [
			e.matches("form") ? "form" : e.textContent.trim(),
			(e.getAttribute("aria-busy") ?? "-") + " " + (e.getAttribute("aria-disabled") ?? "-"),
		]))`)
		if err != nil {
			return false, err.Error()
		}
		m := got.Value.Map()
		for name, state := range want {
			if m[name].Str() != state {
				return false, fmt.Sprint(m)
			}
		}
		return true, ""
	}, fmt.Sprint("aria-busy and aria-disabled ", want))
}

func (a *pending) still(want map[string]string) {
	a.t.Helper()
	time.Sleep(time.Second)
	a.states(want)
}

func (a *pending) draw() {
	a.t.Helper()
	if r := a.result("page__island_append", map[string]any{"path": "board", "code": `ui.draw(1)`}); r.Error != api.ToolOk {
		a.t.Fatalf("island_append failed: %#v", r.Content)
	}
}

func (a *pending) read(path string) (disabled bool) {
	a.t.Helper()
	var s inputs
	testkit.Decode(a.t, a.result("page__sync", map[string]any{"path": path}), &s)
	if len(s.Inputs) != 1 {
		a.t.Fatalf("%s synced as %+v, want one input", path, s)
	}
	var read struct{ Disabled bool }
	testkit.Decode(a.t, a.result("page__read", map[string]any{"path": path, "id": s.Inputs[0].ID}), &read)
	return read.Disabled
}

const goAndPlain = `ui.root.set(ui.column({}, ui.button({text: "Go", value: "v1"}), ui.button({text: "Plain", value: "v2"}))); ui.draw()`

func TestIslandSubmitIndicationHoldsUntilTheNextDraw(t *testing.T) {
	a := openPending(t, map[string]string{"board": goAndPlain})
	a.states(map[string]string{"Go": "- -", "Plain": "- -"})
	userClick(t, a.c, "Go")
	a.states(map[string]string{"Go": "true -", "Plain": "- true"})
	a.still(map[string]string{"Go": "true -", "Plain": "- true"})
	focused, err := a.c.Page.Eval(`() => document.activeElement.textContent.trim()`)
	if err != nil {
		t.Fatal(err)
	}
	if focused.Value.Str() != "Go" {
		t.Fatalf("focus moved to %q while the submit is pending, want it kept on Go", focused.Value.Str())
	}
	a.draw()
	a.states(map[string]string{"Go": "- -", "Plain": "- -"})
}

func TestIslandSubmitIndicationStaysInItsIsland(t *testing.T) {
	a := openPending(t, map[string]string{
		"board": goAndPlain,
		"other": `ui.root.set(ui.button({text: "Elsewhere", value: "e"})); ui.draw()`,
	})
	a.states(map[string]string{"Go": "- -", "Plain": "- -", "Elsewhere": "- -"})
	ids, err := a.c.Page.Eval(`() => [...document.querySelectorAll("[data-neui-island]")].map((e) => e.getAttribute("data-neui-island") + ":" + e.querySelectorAll("[data-neui=submit]").length).join(" ")`)
	if err != nil {
		t.Fatal(err)
	}
	if parts := strings.Fields(ids.Value.Str()); len(parts) != 2 || parts[0] == parts[1] || !strings.HasSuffix(parts[0], ":2") || !strings.HasSuffix(parts[1], ":1") {
		t.Fatalf("the islands are marked %q, want two ids holding two and one submits", ids.Value.Str())
	}
	userClick(t, a.c, "Go")
	a.states(map[string]string{"Go": "true -", "Plain": "- true", "Elsewhere": "- -"})
	a.still(map[string]string{"Go": "true -", "Plain": "- true", "Elsewhere": "- -"})
}

func TestIslandFormSubmitMarksItsButtonAndDisablesTheIslandSubmits(t *testing.T) {
	a := openPending(t, map[string]string{"board": `ui.root.set(ui.column({},
		ui.form({fields: [{type: "text", label: "Name", name: "name"}], submits: [{text: "Send", value: "send"}, {text: "Skip", value: "skip", novalidate: true}]}),
		ui.button({text: "Plain", value: "v2"})
	)); ui.draw()`})
	a.states(map[string]string{"form": "- -", "Send": "- -", "Skip": "- -", "Plain": "- -"})
	userClick(t, a.c, "Send")
	a.states(map[string]string{"Send": "true -", "Skip": "- true", "Plain": "- true"})
	a.still(map[string]string{"Send": "true -", "Skip": "- true", "Plain": "- true"})
	a.draw()
	a.states(map[string]string{"form": "- -", "Send": "- -", "Skip": "- -", "Plain": "- -"})
}

func TestIslandAgentReadsAnotherSubmitAsDisabledWhilePending(t *testing.T) {
	a := openPending(t, map[string]string{"board": goAndPlain})
	a.states(map[string]string{"Go": "- -", "Plain": "- -"})
	if a.read("board.screen.button_2") {
		t.Fatal("Plain read as disabled before any submit")
	}
	userClick(t, a.c, "Go")
	a.states(map[string]string{"Go": "true -", "Plain": "- true"})
	if !a.read("board.screen.button_2") {
		t.Fatal("Plain read as enabled while Go's submit is pending, want disabled")
	}
	a.draw()
	a.states(map[string]string{"Plain": "- -"})
	if a.read("board.screen.button_2") {
		t.Fatal("Plain read as disabled after the next draw")
	}
}

func TestIslandAgentClickOnASubmitBlockedByAPendingOneIsCanceled(t *testing.T) {
	a := openPending(t, map[string]string{"board": goAndPlain})
	userClick(t, a.c, "Go")
	a.states(map[string]string{"Go": "true -", "Plain": "- true"})
	var s inputs
	testkit.Decode(t, a.result("page__sync", map[string]any{"path": "board.screen.button_2"}), &s)
	r := a.result("page__click", map[string]any{"path": "board.screen.button_2", "id": s.Inputs[0].ID})
	if want := "the page canceled its server handlers, as it does while an earlier action is still being handled"; r.Error == api.ToolOk || !strings.Contains(fmt.Sprint(r.Content), want) {
		t.Fatalf("the click on Plain answered %v, want an error saying %q", r.Content, want)
	}
}
