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

package page_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type readOutput struct {
	Value any
	Label string
}

func order(changes chan<- string, clicks chan<- struct{}) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Space{Name: "order", Desc: "the order"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := (neui.Use{Desc: "how many"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				if err := cur.Set("name", "qty"); err != nil {
					return err
				}
				if err := cur.Modify(doors.AChange{On: func(ctx context.Context, r doors.RequestChange) bool {
					changes <- r.Event().Value
					return false
				}}); err != nil {
					return err
				}
				return cur.Submit()
			}); err != nil {
				return err
			}
			return (neui.Use{Desc: "places the order", Rights: neui.W}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init("button"); err != nil {
					return err
				}
				if err := cur.Set("name", "place"); err != nil {
					return err
				}
				if err := cur.Modify(doors.AClick{On: func(ctx context.Context, r doors.RequestPointer) bool {
					clicks <- struct{}{}
					return false
				}}); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text("Place"); err != nil {
					return err
				}
				return cur.Close()
			})
		})
	}
}

func TestInputDispatchReachesHandlers(t *testing.T) {
	changes := make(chan string, 1)
	clicks := make(chan struct{}, 1)
	inputs := map[string]string{}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "order"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			for _, in := range s.Inputs {
				inputs[in.Desc] = in.ID
			}
			return testkit.Call("page__write", map[string]any{"path": "order", "id": inputs["how many"], "value": "3"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__read", map[string]any{"path": "order", "id": inputs["how many"]})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__click", map[string]any{"path": "order", "id": inputs["places the order"]})
		}),
		testkit.Say("placed"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: order(changes, clicks)})
	c.Send("order three")
	c.Wait()
	var written json.RawMessage
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &written)
	if string(written) != `{"handled":true,"diff":{}}` {
		t.Fatalf("the write answered %s, want it handled and nothing changed", written)
	}
	select {
	case v := <-changes:
		if v != "3" {
			t.Fatalf("the change handler saw %q, want 3", v)
		}
	default:
		t.Fatal("the write returned before its change handler ran")
	}
	var read readOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &read)
	if read.Value != "3" {
		t.Fatalf("the read answered %+v, want the written 3", read)
	}
	var clicked json.RawMessage
	testkit.Decode(t, testkit.Results(p.Request(t, 4))[0], &clicked)
	if string(clicked) != `{"handled":true,"diff":{}}` {
		t.Fatalf("the click answered %s, want it handled and nothing changed", clicked)
	}
	select {
	case <-clicks:
	default:
		t.Fatal("the click returned before its handler ran")
	}
}

func doored(d *doors.Door, content gox.Elem) gox.Elem {
	return func(cur gox.Cursor) error {
		return d.Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("div"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Any(content); err != nil {
				return err
			}
			return cur.Close()
		})
	}
}

func TestRadioGroupStaysWritableAfterFirstMemberDoorIsRemoved(t *testing.T) {
	changes := make(chan string, 4)
	sizes := func(cur gox.Cursor) error {
		first := new(doors.Door)
		return (neui.Space{Name: "sizes", Desc: "the sizes"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Comp(neui.Tool[struct{}, string]{Name: "drop", Desc: "removes the small size", Handler: func(ctx context.Context, _ struct{}) (string, error) {
				for err := range first.Inner(ctx, nil) {
					if err != nil {
						return "", err
					}
				}
				return "dropped", nil
			}}); err != nil {
				return err
			}
			if err := cur.Any(doored(first, radio("size", "s", changes))); err != nil {
				return err
			}
			return cur.Any(doored(new(doors.Door), radio("size", "m", changes)))
		})
	}
	var group string
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "sizes"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var sizes state
			testkit.Decode(t, testkit.Results(r)[0], &sizes)
			if len(sizes.Inputs) != 1 {
				t.Errorf("the sizes synced as %+v, want one radio group", sizes.Inputs)
				return testkit.Say("lost")
			}
			group = sizes.Inputs[0].ID
			return testkit.Call("page__write", map[string]any{"path": "sizes", "id": group, "value": "s"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__tool", map[string]any{"path": "sizes", "tool": "drop", "arg": map[string]any{}})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__write", map[string]any{"path": "sizes", "id": group, "value": "m"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__read", map[string]any{"path": "sizes", "id": group})
		}),
		testkit.Say("picked"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: sizes})
	c.Send("pick medium")
	c.Wait()
	var before report
	testkit.Decode(t, testkit.Results(p.Request(t, 2))[0], &before)
	if !empty(before.Diff) || <-changes != "s" {
		t.Fatalf("the write before the drop answered %+v, want the small radio's change handler", before)
	}
	var dropped toolOutput
	testkit.Decode(t, testkit.Results(p.Request(t, 3))[0], &dropped)
	if dropped.Output != "dropped" || !empty(dropped.Diff) {
		t.Fatalf("the drop answered %+v, want the group kept", dropped)
	}
	if r := testkit.Results(p.Request(t, 4))[0]; r.Error != api.ToolOk {
		t.Fatalf("the write after the drop answered %#v, want the medium radio's change handler", r.Content)
	}
	select {
	case v := <-changes:
		if v != "m" {
			t.Fatalf("the write after the drop reached %q, want m", v)
		}
	default:
		t.Fatal("the write after the drop returned before its change handler ran")
	}
	var read struct {
		Value   any
		Options []struct{ Value string }
	}
	testkit.Decode(t, testkit.Results(p.Request(t, 5))[0], &read)
	if read.Value != "m" || len(read.Options) != 1 {
		t.Fatalf("the read after the drop answered %+v, want m as the only option", read)
	}
}

func TestReadReportsDisabled(t *testing.T) {
	field := func(desc string, attrs ...any) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Use{Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				for i := 0; i < len(attrs); i += 2 {
					if err := cur.Set(attrs[i].(string), attrs[i+1]); err != nil {
						return err
					}
				}
				return cur.Submit()
			})
		}
	}
	size := func(cur gox.Cursor) error {
		return (neui.Use{Desc: "the size"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("select"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			for _, o := range []string{"s", "m"} {
				if err := cur.Init("option"); err != nil {
					return err
				}
				if err := cur.Set("value", o); err != nil {
					return err
				}
				if err := cur.Set("disabled", o == "m"); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text(o); err != nil {
					return err
				}
				if err := cur.Close(); err != nil {
					return err
				}
			}
			return cur.Close()
		})
	}
	page := func(cur gox.Cursor) error {
		return (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, el := range []gox.Elem{
				field("the name", "disabled", true),
				size,
				field("the color", "type", "radio", "name", "color", "value", "red"),
				field("the color", "type", "radio", "name", "color", "value", "blue", "disabled", true),
			} {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		})
	}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			ids := map[string]string{}
			for _, in := range s.Inputs {
				ids[in.Desc] = in.ID
			}
			return round(
				call("name", "page__read", map[string]any{"path": "form", "id": ids["the name"]}),
				call("size", "page__read", map[string]any{"path": "form", "id": ids["the size"]}),
				call("color", "page__read", map[string]any{"path": "form", "id": ids["the color"]}),
			)
		}),
		testkit.Say("read"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: page})
	c.Send("read the form")
	c.Wait()
	var got []string
	for _, r := range testkit.Results(p.Request(t, 2)) {
		var raw json.RawMessage
		testkit.Decode(t, r, &raw)
		got = append(got, string(raw))
	}
	want := []string{
		`{"value":"","disabled":true}`,
		`{"value":"s","options":[{"value":"s","label":"s","selected":true},{"value":"m","label":"m","selected":false,"disabled":true}]}`,
		`{"options":[{"value":"red","label":"","selected":false},{"value":"blue","label":"","selected":false,"disabled":true}]}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the reads answered\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInputCallsReportChanges(t *testing.T) {
	edits := doors.NewSource(0)
	bump := func(ctx context.Context) { edits.Mutate(ctx, func(n int) int { return n + 1 }) }
	count := func() gox.Elem {
		return edits.Bind(func(n int) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(neui.Data{Text: "edits", Object: n}) }
		})
	}
	field := func(desc string, tag string, hook gox.Modify) gox.Elem {
		return func(cur gox.Cursor) error {
			return (neui.Use{Desc: desc}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init(tag); err != nil {
					return err
				}
				if err := cur.Modify(hook); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				return cur.Close()
			})
		}
	}
	form := func(cur gox.Cursor) error {
		if err := (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, el := range []gox.Elem{
				field("the name", "input", doors.AChange{On: func(ctx context.Context, _ doors.RequestChange) bool { bump(ctx); return false }}),
				field("saves", "button", doors.AClick{On: func(ctx context.Context, _ doors.RequestPointer) bool { bump(ctx); return false }}),
				field("the note", "textarea", doors.AFocus{On: func(ctx context.Context, _ doors.RequestFocus) bool { bump(ctx); return false }}),
				count(),
			} {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return (neui.Space{Name: "summary", Desc: "the summary"}).Proxy(cur, func(cur gox.Cursor) error { return cur.Any(count()) })
	}
	ids := map[string]string{}
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			for _, in := range s.Inputs {
				ids[in.Desc] = in.ID
			}
			return testkit.Call("page__write", map[string]any{"path": "form", "id": ids["the name"], "value": "Ann"})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__click", map[string]any{"path": "form", "id": ids["saves"]})
		}),
		testkit.Then(func(api.Request) testkit.Turn {
			return testkit.Call("page__focus", map[string]any{"path": "form", "id": ids["the note"]})
		}),
		testkit.Say("done"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: form})
	c.Send("fill the form")
	c.Wait()
	for i, call := range []string{"write", "click", "focus"} {
		var out report
		r := testkit.Results(p.Request(t, i+2))[0]
		if r.Error != api.ToolOk {
			t.Fatalf("the %s answered %#v", call, r.Content)
		}
		testkit.Decode(t, r, &out)
		if out.Diff == nil || len(out.Diff.Data) != 1 || out.Diff.Data[0].Object != float64(i+1) || string(out.Changes) != `[{"path":"summary","changed":["data"]}]` {
			t.Fatalf("the %s answered diff %+v and changes %s, want edits at %d in the form and the summary listed", call, out.Diff, out.Changes, i+1)
		}
	}
}

func TestFailedInputCallsReport(t *testing.T) {
	ghost := func(cur gox.Cursor) error {
		return (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := cur.Init("template"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := (neui.Use{Desc: "the ghost"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				return cur.Submit()
			}); err != nil {
				return err
			}
			return cur.Close()
		})
	}
	var id string
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__sync", map[string]any{"path": "form"}),
		testkit.Then(func(r api.Request) testkit.Turn {
			var s state
			testkit.Decode(t, testkit.Results(r)[0], &s)
			if len(s.Inputs) == 1 {
				id = s.Inputs[0].ID
			}
			return round(
				call("write", "page__write", map[string]any{"path": "form", "id": id, "value": "x"}),
				call("click", "page__click", map[string]any{"path": "form", "id": id}),
				call("focus", "page__focus", map[string]any{"path": "form", "id": id}),
			)
		}),
		testkit.Say("done"),
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: ghost})
	c.Send("fill the ghost")
	c.Wait()
	results := testkit.Results(p.Request(t, 2))
	for i, want := range []string{`{"diff":{}}`, `{"diff":{}}`, `{"diff":{}}`} {
		r := results[i]
		if r.Error == api.ToolOk || len(r.Content) != 2 {
			t.Fatalf("the %s answered %#v, want its error and its result", r.ID, r.Content)
		}
		message, _ := r.Content[0].(api.ContentText)
		value, _ := r.Content[1].(api.ContentText)
		if !strings.HasSuffix(message.Text, "not found") || value.Text != want {
			t.Fatalf("the %s failed with %#v, want the message and then %s", r.ID, r.Content, want)
		}
	}
}

func html(tag string, text string, parts ...any) gox.Elem {
	return func(cur gox.Cursor) error {
		void := tag == "input"
		if void {
			if err := cur.InitVoid(tag); err != nil {
				return err
			}
		} else if err := cur.Init(tag); err != nil {
			return err
		}
		var children []gox.Elem
		for i := 0; i < len(parts); i++ {
			switch part := parts[i].(type) {
			case gox.Elem:
				children = append(children, part)
			case gox.Modify:
				if err := cur.Modify(part); err != nil {
					return err
				}
			case string:
				if err := cur.Set(part, parts[i+1]); err != nil {
					return err
				}
				i++
			}
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if void {
			return nil
		}
		for _, child := range children {
			if err := cur.Any(child); err != nil {
				return err
			}
		}
		if text != "" {
			if err := cur.Text(text); err != nil {
				return err
			}
		}
		return cur.Close()
	}
}

func exposed(desc string, el gox.Elem) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Use{Desc: desc}).Proxy(cur, el)
	}
}

func space(els ...gox.Elem) gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Space{Name: "form", Desc: "the form"}).Proxy(cur, func(cur gox.Cursor) error {
			for _, el := range els {
				if err := cur.Any(el); err != nil {
					return err
				}
			}
			return nil
		})
	}
}

type step struct {
	tool   string
	input  string
	value  any
	before func()
}

func steps(t *testing.T, list ...step) *testkit.Provider {
	ids := map[string]string{}
	turns := []testkit.Turn{testkit.Call("page__sync", map[string]any{"path": "form"})}
	for i, s := range list {
		turns = append(turns, testkit.Then(func(r api.Request) testkit.Turn {
			if i == 0 {
				var synced state
				testkit.Decode(t, testkit.Results(r)[0], &synced)
				for _, in := range synced.Inputs {
					ids[in.Desc] = in.ID
				}
			}
			if s.before != nil {
				s.before()
			}
			args := map[string]any{"path": "form", "id": ids[s.input]}
			if s.tool == "page__write" {
				args["value"] = s.value
			}
			return testkit.Call(s.tool, args)
		}))
	}
	return testkit.NewProvider(api.Model{}, append(turns, testkit.Say("done"))...)
}

func answers(t *testing.T, p *testkit.Provider, n int) []string {
	t.Helper()
	var out []string
	for i := range n {
		r := testkit.Results(p.Request(t, i+2))[0]
		text, _ := r.Content[0].(api.ContentText)
		if r.Error != api.ToolOk {
			out = append(out, "error: "+text.Text)
			continue
		}
		out = append(out, text.Text)
	}
	return out
}

func failed(message string) string {
	return "error: client execution failed: " + message
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the calls answered\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDisabledTargetsRefuseActions(t *testing.T) {
	hooks := make(chan string, 8)
	click := func(name string) gox.Modify {
		return doors.AClick{On: func(context.Context, doors.RequestPointer) bool { hooks <- name; return false }}
	}
	change := func(name string) gox.Modify {
		return doors.AChange{On: func(context.Context, doors.RequestChange) bool { hooks <- name; return false }}
	}
	p := steps(t,
		step{tool: "page__write", input: "the name", value: "Bo"},
		step{tool: "page__read", input: "the name"},
		step{tool: "page__click", input: "saves"},
		step{tool: "page__click", input: "sends"},
		step{tool: "page__write", input: "the size", value: "m"},
		step{tool: "page__read", input: "the size"},
		step{tool: "page__click", input: "adds"},
		step{tool: "page__read", input: "adds"},
		step{tool: "page__read", input: "sends"},
		step{tool: "page__write", input: "the terms", value: true},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the name", html("input", "", "value", "Ann", "disabled", true, change("name"))),
		exposed("the terms", html("input", "", "type", "checkbox", "disabled", true, change("terms"))),
		exposed("saves", html("button", "Save", "disabled", true, click("save"))),
		html("div", "", "inert", true, exposed("sends", html("button", "Send", click("send")))),
		html("fieldset", "", "disabled", true, exposed("the size", html("select", "", change("size"), html("option", "s", "value", "s"), html("option", "m", "value", "m")))),
		exposed("adds", html("button", "Add", "aria-disabled", "true", click("add"))),
	)})
	c.Send("fill the form")
	c.Wait()
	expect(t, answers(t, p, 10),
		failed("target is disabled"),
		`{"value":"Ann","disabled":true}`,
		failed("target is disabled"),
		failed("target is disabled"),
		failed("target is disabled"),
		`{"value":"s","options":[{"value":"s","label":"s","selected":true},{"value":"m","label":"m","selected":false}],"disabled":true}`,
		`{"handled":true,"diff":{}}`,
		`{"label":"Add","disabled":true}`,
		`{"label":"Send","disabled":true}`,
		failed("target is disabled"),
	)
	if got := <-hooks; got != "add" || len(hooks) != 0 {
		t.Fatalf("the hooks ran %q and %d more, want only the aria-disabled button's", got, len(hooks))
	}
}

func TestCheckboxWriteClicksLikeAUser(t *testing.T) {
	hooks := make(chan string, 8)
	p := steps(t,
		step{tool: "page__write", input: "the row", value: "true"},
		step{tool: "page__write", input: "the row", value: true},
		step{tool: "page__write", input: "the row", value: true},
		step{tool: "page__write", input: "the terms", value: true},
		step{tool: "page__write", input: "the row", value: false},
		step{tool: "page__read", input: "the row"},
		step{tool: "page__write", input: "the note", value: "hi"},
		step{tool: "page__click", input: "does nothing"},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the row", html("input", "", "type", "checkbox", doors.AClick{On: func(_ context.Context, r doors.RequestPointer) bool { hooks <- "row"; return false }})),
		exposed("the terms", html("input", "", "type", "checkbox", doors.AChange{On: func(_ context.Context, r doors.RequestChange) bool { hooks <- "terms"; return false }})),
		exposed("the note", html("input", "")),
		exposed("does nothing", html("button", "Nothing")),
	)})
	c.Send("tick the row")
	c.Wait()
	expect(t, answers(t, p, 8),
		failed("a checkbox takes true or false"),
		`{"handled":true,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
		`{"handled":true,"diff":{}}`,
		`{"handled":true,"diff":{}}`,
		`{"value":false}`,
		`{"handled":false,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
	)
	var got []string
	for len(hooks) > 0 {
		got = append(got, <-hooks)
	}
	if strings.Join(got, " ") != "row terms row" {
		t.Fatalf("the hooks ran %q, want the row's click, the terms' change, the row's click", got)
	}
}

func TestRadioWriteClicksTheOption(t *testing.T) {
	changes := make(chan string, 4)
	p := steps(t,
		step{tool: "page__write", input: "size s", value: "m"},
		step{tool: "page__write", input: "size s", value: "m"},
		step{tool: "page__write", input: "size s", value: "xl"},
		step{tool: "page__read", input: "size s"},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		radio("size", "s", changes),
		radio("size", "m", changes),
	)})
	c.Send("pick medium")
	c.Wait()
	expect(t, answers(t, p, 4),
		`{"handled":true,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
		failed(`no option with value "xl"`),
		`{"value":"m","options":[{"value":"s","label":"","selected":false},{"value":"m","label":"","selected":true}]}`,
	)
	if got := <-changes; got != "m" || len(changes) != 0 {
		t.Fatalf("the change handlers saw %q and %d more, want m once", got, len(changes))
	}
}

func TestWriteFailsWhenTheFieldDropsTheValue(t *testing.T) {
	p := steps(t,
		step{tool: "page__write", input: "the count", value: "abc"},
		step{tool: "page__read", input: "the count"},
		step{tool: "page__write", input: "the count", value: "7"},
		step{tool: "page__write", input: "the day", value: "2024-02-30"},
		step{tool: "page__write", input: "the day", value: "2024-02-03"},
		step{tool: "page__write", input: "the size", value: "x"},
		step{tool: "page__read", input: "the size"},
		step{tool: "page__write", input: "the tags", value: []string{"a", "x"}},
		step{tool: "page__write", input: "the tags", value: []string{"a", "c"}},
		step{tool: "page__read", input: "the tags"},
	)
	options := func(values ...string) []any {
		var out []any
		for _, v := range values {
			out = append(out, html("option", v, "value", v))
		}
		return out
	}
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the count", html("input", "", "type", "number", "value", "5")),
		exposed("the day", html("input", "", "type", "date")),
		exposed("the size", html("select", "", options("s", "m")...)),
		exposed("the tags", html("select", "", append([]any{"multiple", true, "aria-label", "Tags"}, options("a", "b", "c")...)...)),
	)})
	c.Send("fill the form")
	c.Wait()
	expect(t, answers(t, p, 10),
		failed("the field did not accept this value"),
		`{"value":"5"}`,
		`{"handled":false,"diff":{}}`,
		failed("the field did not accept this value"),
		`{"handled":false,"diff":{}}`,
		failed(`no option with value "x"`),
		`{"value":"s","options":[{"value":"s","label":"s","selected":true},{"value":"m","label":"m","selected":false}]}`,
		failed(`no option with value "x"`),
		`{"handled":false,"diff":{}}`,
		`{"value":["a","c"],"label":"Tags","options":[{"value":"a","label":"a","selected":true},{"value":"b","label":"b","selected":false},{"value":"c","label":"c","selected":true}]}`,
	)
}

func TestWriteFailsWhenTheFieldSubstitutesTheValue(t *testing.T) {
	p := steps(t,
		step{tool: "page__write", input: "the volume", value: "abc"},
		step{tool: "page__read", input: "the volume"},
		step{tool: "page__write", input: "the volume", value: "150"},
		step{tool: "page__write", input: "the volume", value: "33"},
		step{tool: "page__write", input: "the volume", value: "50.0"},
		step{tool: "page__write", input: "the shade", value: "nope"},
		step{tool: "page__write", input: "the shade", value: "#ABCDEF"},
		step{tool: "page__read", input: "the shade"},
		step{tool: "page__write", input: "the name", value: "a\nb"},
		step{tool: "page__write", input: "the name", value: "Ada\n"},
		step{tool: "page__write", input: "the email", value: " a@b.co "},
		step{tool: "page__write", input: "the start", value: "2026-01-01 10:00"},
		step{tool: "page__write", input: "the note", value: "a\r\nb"},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the volume", html("input", "", "type", "range", "min", "0", "max", "100", "step", "10", "value", "20")),
		exposed("the shade", html("input", "", "type", "color")),
		exposed("the name", html("input", "")),
		exposed("the email", html("input", "", "type", "email")),
		exposed("the start", html("input", "", "type", "datetime-local")),
		exposed("the note", html("textarea", "")),
	)})
	c.Send("fill the form")
	c.Wait()
	expect(t, answers(t, p, 13),
		failed("the field did not accept this value"),
		`{"value":"20"}`,
		failed("the field did not accept this value"),
		failed("the field did not accept this value"),
		`{"handled":false,"diff":{}}`,
		failed("the field did not accept this value"),
		`{"handled":false,"diff":{}}`,
		`{"value":"#abcdef"}`,
		failed("the field did not accept this value"),
		`{"handled":false,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
		`{"handled":false,"diff":{}}`,
	)
}

func TestAgentClicksCanBeCanceled(t *testing.T) {
	hooks := make(chan string, 8)
	declined := func(name string) doors.AClick {
		return doors.AClick{PreventDefault: true, On: func(context.Context, doors.RequestPointer) bool {
			hooks <- name
			return false
		}}
	}
	p := steps(t,
		step{tool: "page__write", input: "the box", value: true},
		step{tool: "page__read", input: "the box"},
		step{tool: "page__click", input: "the box"},
		step{tool: "page__read", input: "the box"},
		step{tool: "page__write", input: "pick a", value: "b"},
		step{tool: "page__read", input: "pick a"},
		step{tool: "page__click", input: "the link"},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the box", html("input", "", "type", "checkbox", declined("box"))),
		exposed("pick a", html("input", "", "type", "radio", "name", "pick", "value", "a", declined("a"))),
		exposed("pick b", html("input", "", "type", "radio", "name", "pick", "value", "b", declined("b"))),
		exposed("the link", html("a", "Away", "href", "#away", declined("link"))),
	)})
	c.Send("try the controls")
	c.Wait()
	expect(t, answers(t, p, 7),
		`{"handled":true,"diff":{}}`,
		`{"value":false}`,
		`{"handled":true,"diff":{}}`,
		`{"value":false}`,
		`{"handled":true,"diff":{}}`,
		`{"options":[{"value":"a","label":"","selected":false},{"value":"b","label":"","selected":false}]}`,
		`{"handled":true,"diff":{}}`,
	)
	if hash := c.Page.MustEval(`() => location.hash`).Str(); hash != "" {
		t.Fatalf("the link went to %q, want its handler's preventDefault to keep the page", hash)
	}
	var got []string
	for len(hooks) > 0 {
		got = append(got, <-hooks)
	}
	if strings.Join(got, " ") != "box box b link" {
		t.Fatalf("the hooks ran %q, want box twice, then b, then link", got)
	}
}

func TestReadFallsBackToAriaLabel(t *testing.T) {
	p := steps(t,
		step{tool: "page__read", input: "the query"},
		step{tool: "page__read", input: "the search"},
		step{tool: "page__read", input: "removes a"},
		step{tool: "page__read", input: "the color"},
	)
	c := stack.Browser(t, stack.Host{Provider: p, Page: space(
		html("label", "Query", "for", "q"),
		exposed("the query", html("input", "", "id", "q", "aria-label", "ignored")),
		exposed("the search", html("input", "", "type", "search", "aria-label", "Search the rows")),
		exposed("removes a", html("button", "", "aria-label", "Remove a.txt")),
		exposed("the color", html("input", "", "type", "radio", "name", "color", "value", "red", "aria-label", "Red")),
	)})
	c.Send("read the form")
	c.Wait()
	expect(t, answers(t, p, 4),
		`{"value":"","label":"Query"}`,
		`{"value":"","label":"Search the rows"}`,
		`{"label":"Remove a.txt"}`,
		`{"options":[{"value":"red","label":"Red","selected":false}]}`,
	)
}

func TestFileInputIsReadAndFocusedOnly(t *testing.T) {
	var c *stack.Chat
	choose := func() {
		_, err := c.Page.Eval(`() => {
			const files = new DataTransfer()
			files.items.add(new File(["a"], "a.txt"))
			files.items.add(new File(["b"], "b.pdf"))
			document.querySelector("input[type=file]").files = files.files
		}`)
		if err != nil {
			t.Error(err)
		}
	}
	p := steps(t,
		step{tool: "page__read", input: "the attachment"},
		step{tool: "page__read", input: "the attachment", before: choose},
		step{tool: "page__write", input: "the attachment", value: "c.txt"},
		step{tool: "page__click", input: "the attachment"},
		step{tool: "page__focus", input: "the attachment"},
	)
	c = stack.Browser(t, stack.Host{Provider: p, Page: space(
		exposed("the attachment", html("input", "", "type", "file", "multiple", true, "aria-label", "Attachment")),
	)})
	c.Send("attach")
	c.Wait()
	refusal := "a file input can't be set or opened by you: focus it and ask the user to choose a file"
	expect(t, answers(t, p, 5),
		`{"label":"Attachment","files":[]}`,
		`{"label":"Attachment","files":["a.txt","b.pdf"]}`,
		failed(refusal),
		failed(refusal),
		`{"diff":{}}`,
	)
}
