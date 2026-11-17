//go:build browser

package markdown_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
)

func TestGetTextReadsTheCurrentText(t *testing.T) {
	p := testkit.NewProvider(api.Model{})
	c := stack.Browser(t, stack.Host{Provider: p, Page: func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: func(l neui.Library) error {
				if err := neisland.RegisterMarkdown(l); err != nil {
					return err
				}
				return neisland.RegisterButton(l)
			},
		})
	}})
	result := func(name string, args any) api.ContentToolResult {
		t.Helper()
		p.Push(testkit.Call(name, args), testkit.Say("done"))
		c.Send("go on")
		c.Wait()
		requests := p.Requests()
		return testkit.Results(requests[len(requests)-1])[0]
	}
	spec := result("page__island_spec", map[string]any{"path": "board"})
	text, _ := spec.Content[0].(api.ContentText)
	revision, _, _ := strings.Cut(strings.TrimPrefix(text.Text, "revision "), "\n")
	script := `var m = ui.markdown({ text: "one" })
ui.root.set(m, ui.button({ text: "OK" }))
ui.draw(50)
m.update({ text: "**two**", style: "warning" })
ui.draw()`
	if r := result("page__island_begin", map[string]any{"path": "board", "revision": revision, "code": script, "comment": "the markdown app"}); r.Error != api.ToolOk {
		t.Fatalf("island_begin failed: %#v", r.Content)
	}
	stack.Eventually(t, 10*time.Second, func() (bool, string) {
		if r := result("page__sync", map[string]any{"path": "board.screen.markdown_1"}); r.Error != api.ToolOk {
			return false, fmt.Sprint(r.Content)
		}
		r := result("page__tool", map[string]any{"path": "board.screen.markdown_1", "tool": "get_text", "arg": map[string]any{}})
		var out struct{ Output json.RawMessage }
		if r.Error == api.ToolOk {
			testkit.Decode(t, r, &out)
		}
		return string(out.Output) == `{"text":"**two**","style":"warning"}`, fmt.Sprint(r.Content)
	}, "get_text reads the updated text and style")
}
