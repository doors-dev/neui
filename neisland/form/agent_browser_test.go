//go:build browser

package form_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"github.com/go-rod/rod/lib/proto"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
)

const formPath = "board.screen.form_1"

type input struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Type   string `json:"type"`
	Desc   string `json:"desc"`
	Rights string `json:"rights"`
}

type data struct {
	Text   string          `json:"text"`
	Object json.RawMessage `json:"object"`
}

type child struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type state struct {
	Desc     string  `json:"desc"`
	Inputs   []input `json:"inputs"`
	Data     []data  `json:"data"`
	Children []child `json:"children"`
}

type report struct {
	Handled bool `json:"handled"`
	Diff    *struct {
		Removed bool `json:"removed"`
		Inputs  *struct {
			Added   []input  `json:"added"`
			Removed []string `json:"removed"`
		} `json:"inputs"`
		Data     *[]data `json:"data"`
		Children *struct {
			Added   []child  `json:"added"`
			Removed []string `json:"removed"`
		} `json:"children"`
	} `json:"diff"`
	Changes []change `json:"changes"`
}

type change struct {
	Path    string   `json:"path"`
	Changed []string `json:"changed"`
	Added   bool     `json:"added"`
}

type call struct {
	name string
	args map[string]any
}

type agent struct {
	t *testing.T
	c *stack.Chat
	p *testkit.Provider
	n int
}

func board() gox.Elem {
	return func(cur gox.Cursor) error {
		return cur.Comp(neui.Island{
			Name:        "board",
			Desc:        "a board",
			Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
			Template: func(slot gox.Comp) gox.Elem {
				return func(cur gox.Cursor) error { return cur.Comp(slot) }
			},
			Library: neisland.RegisterForm,
		})
	}
}

func open(t *testing.T, spec string) *agent {
	t.Helper()
	p := testkit.NewProvider(api.Model{})
	a := &agent{t: t, c: stack.Browser(t, stack.Host{Provider: p, Page: board()}), p: p}
	r := a.run(call{"page__island_spec", map[string]any{"path": "board"}})[0]
	line, _, _ := strings.Cut(text(r), "\n")
	revision, ok := strings.CutPrefix(line, "revision ")
	if !ok {
		t.Fatalf("island_spec answered %q, want a revision line first", text(r))
	}
	r = a.run(call{"page__island_begin", map[string]any{
		"path":     "board",
		"revision": revision,
		"code":     `ui.root.set(ui.form(` + spec + `)); ui.notify(ui.draw()); ui.draw(1)`,
		"comment":  "a form",
	}})[0]
	if r.Error != api.ToolOk {
		t.Fatalf("island_begin failed: %s", text(r))
	}
	a.c.Page.Timeout(10 * time.Second).MustElement(".neisland-form")
	return a
}

func text(r api.ContentToolResult) string {
	for _, c := range r.Content {
		if t, ok := c.(api.ContentText); ok {
			return t.Text
		}
	}
	return ""
}

func (a *agent) run(calls ...call) []api.ContentToolResult {
	a.t.Helper()
	return a.round(nil, calls...)
}

func (a *agent) round(after []testkit.Turn, calls ...call) []api.ContentToolResult {
	a.t.Helper()
	a.n++
	events := []api.Event{api.EventStart{ID: fmt.Sprintf("turn-%d", a.n), Model: "fake"}}
	for i, c := range calls {
		args, err := json.Marshal(c.args)
		if err != nil {
			a.t.Fatal(err)
		}
		events = append(events, api.EventTool{Index: i, ID: fmt.Sprintf("call-%d-%d", a.n, i), Name: c.name, Args: string(args)}, api.EventClose{Index: i})
	}
	events = append(events, api.EventUsage{Input: 1, Output: 1}, api.EventOk{Code: api.CodeTurn})
	before := len(a.p.Requests())
	a.p.Push(append([]testkit.Turn{testkit.Events(events...), testkit.Say("done")}, after...)...)
	a.c.Send("go on")
	a.c.Wait()
	results := testkit.Results(a.p.Request(a.t, before+1))
	if len(results) != len(calls) {
		a.t.Fatalf("%d calls answered with %d results", len(calls), len(results))
	}
	return results
}

func (a *agent) sync(paths ...string) []state {
	a.t.Helper()
	calls := make([]call, len(paths))
	for i, path := range paths {
		calls[i] = call{"page__sync", map[string]any{"path": path}}
	}
	out := make([]state, len(paths))
	for i, r := range a.run(calls...) {
		if r.Error != api.ToolOk {
			a.t.Fatalf("sync %s failed: %s", paths[i], text(r))
		}
		testkit.Decode(a.t, r, &out[i])
	}
	return out
}

func (a *agent) act(name string, args map[string]any) report {
	a.t.Helper()
	r := a.run(call{"page__" + name, args})[0]
	if r.Error != api.ToolOk {
		a.t.Fatalf("%s %v failed: %s", name, args, text(r))
	}
	var rep report
	testkit.Decode(a.t, r, &rep)
	return rep
}

func (a *agent) write(path string, id string, value any) report {
	a.t.Helper()
	return a.act("write", map[string]any{"path": path, "id": id, "value": value})
}

func (a *agent) click(path string, id string) report {
	a.t.Helper()
	return a.act("click", map[string]any{"path": path, "id": id})
}

func (a *agent) notified(want string) string {
	a.t.Helper()
	for range 5 {
		for _, r := range a.p.Requests() {
			for _, c := range r.Messages[len(r.Messages)-1].Content {
				if t, ok := c.(api.ContentText); ok && strings.Contains(t.Text, want) {
					return t.Text
				}
			}
		}
		a.c.Wait()
	}
	a.t.Fatalf("no notification carries %s", want)
	return ""
}

func inputs(ins []input) []string {
	out := make([]string, len(ins))
	for i, in := range ins {
		out[i] = in.Kind + " " + in.Type + " " + in.Desc + " " + in.Rights
	}
	return out
}

func datas(ds []data) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Text + ": " + string(d.Object)
	}
	return out
}

func children(cs []child) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name + " (" + c.Desc + ")"
	}
	return out
}

func expect(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	got, want = slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Fatalf("%s:\n got  %q\n want %q", what, got, want)
	}
}

func id(t *testing.T, ins []input, desc string) string {
	t.Helper()
	for _, in := range ins {
		if in.Desc == desc {
			return in.ID
		}
	}
	t.Fatalf("no input %q in %q", desc, inputs(ins))
	return ""
}

func dataOf(t *testing.T, rep report) []string {
	t.Helper()
	if rep.Diff == nil || rep.Diff.Data == nil {
		t.Fatalf("the call's diff carries no data list: %+v", rep.Diff)
	}
	return datas(*rep.Diff.Data)
}

func changed(rep report, path string, part string) bool {
	return slices.ContainsFunc(rep.Changes, func(c change) bool {
		return c.Path == path && (part == "" && c.Added || slices.Contains(c.Changed, part))
	})
}

const agentSpec = `{fields: [
	{type: "text", label: "Name", name: "name", required: true},
	{type: "password", label: "Secret", name: "secret"},
	{type: "number", label: "Age", name: "age"},
	{type: "range", label: "Volume", name: "volume"},
	{type: "date", label: "Born", name: "born"},
	{type: "textarea", label: "Bio", name: "bio"},
	{type: "checkbox", label: "Agree", name: "agree"},
	{type: "select", label: "Size", name: "size", options: [{value: "s", label: "S"}, {value: "m", label: "M"}]},
	{type: "select", label: "Color", name: "color", options: [{value: 1, label: "Red"}, {value: 2, label: "Green"}, {value: 3, label: "Blue"}, {value: 4, label: "Cyan"}, {value: 5, label: "Pink"}, {value: 6, label: "Gray"}]},
	{type: "select", label: "Toppings", name: "toppings", max: 2, options: [{value: "h", label: "Ham"}, {value: "e", label: "Egg"}]},
	{type: "group", label: "Address", name: "address", fields: [{type: "text", label: "City", name: "city", minLength: 3}]},
	{type: "list", field: {type: "text", label: "Tags", name: "tags"}, value: ["a", "b"]},
	{type: "list", field: {type: "group", label: "Contact", name: "contacts", fields: [
		{type: "email", label: "Email", name: "email"},
		{type: "search", label: "Owner", name: "owner", search: "(q) => [{value: 7, label: 'Max'}]"}
	]}, value: [{email: "a@b.co"}]},
	{type: "search", label: "Person", name: "person", search: "(q) => [{value: 1, label: 'Ada'}, {value: 2, label: 'Bob'}]"},
	{type: "file", label: "Docs", name: "docs"},
	{type: "series", label: "Trip", name: "trip", fields: [
		"(v) => ({type: 'text', label: 'From', name: 'from'})",
		"(v) => ({type: 'text', label: 'To', name: 'to'})"
	]}
], submits: [{text: "Save", value: "save"}, {text: "Save", value: "copy"}, {text: "Skip", value: "skip", novalidate: true}]}`

func TestAgentSeesTheFormAsSpacesOfInputsAndErrors(t *testing.T) {
	a := open(t, agentSpec)
	if a.c.Page.MustElement("form.neisland-form").MustAttribute("novalidate") == nil {
		t.Fatal("the form renders without novalidate")
	}
	s := a.sync(
		formPath,
		formPath+".address",
		formPath+".tags",
		formPath+".tags.item_0001",
		formPath+".contacts",
		formPath+".contacts.item_0001",
		formPath+".contacts.item_0001.owner",
		formPath+".person",
		formPath+".docs",
		formPath+".trip",
		formPath+".trip.from",
	)
	root, address, tags, tag, contacts, contact, owner, person, docs, trip, from := s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7], s[8], s[9], s[10]
	expect(t, "form inputs", inputs(root.Inputs),
		"input text Name RW",
		"input password Secret W",
		"input number Age RW",
		"input range Volume RW",
		"input date Born RW",
		"input textarea Bio RW",
		"input checkbox Agree RW",
		"radio radio Size RW",
		"input select Color RW",
		"input checkbox Toppings: Ham RW",
		"input checkbox Toppings: Egg RW",
		"button submit Save RW",
		"button submit Save RW",
		"button submit Skip RW",
	)
	expect(t, "form data", datas(root.Data),
		`name validation error: "field is required"`,
		`submit blocked: "form is not completed or contains errors"`,
	)
	expect(t, "form children", children(root.Children),
		"address (Address)", "contacts (Contact)", "docs (Docs)", "person (Person)", "tags (Tags)", "trip (Trip)")
	expect(t, "group inputs", inputs(address.Inputs), "input text City RW")
	expect(t, "list inputs", inputs(tags.Inputs), "button button add an item RW")
	expect(t, "list children", children(tags.Children), "item_0001 (an item of tags)", "item_0002 (an item of tags)")
	expect(t, "list item inputs", inputs(tag.Inputs), "input text Tags RW", "button button remove this item RW")
	expect(t, "group list children", children(contacts.Children), "item_0001 (an item of contacts)")
	expect(t, "group item inputs", inputs(contact.Inputs), "input email Email RW", "button button remove this item RW")
	expect(t, "group item children", children(contact.Children), "owner (Owner)")
	expect(t, "nested search inputs", inputs(owner.Inputs), "input text search RW")
	expect(t, "search inputs", inputs(person.Inputs), "input text search RW")
	expect(t, "file inputs", inputs(docs.Inputs), "input file file chooser RW")
	expect(t, "series children", children(trip.Children), "from (From)")
	expect(t, "series data", datas(trip.Data))
	expect(t, "series step inputs", inputs(from.Inputs), "input text From RW")
	if saves := slices.DeleteFunc(slices.Clone(root.Inputs), func(in input) bool { return in.Desc != "Save" }); saves[0].ID == saves[1].ID {
		t.Fatalf("the two Save buttons share id %s", saves[0].ID)
	}
	for _, b := range []struct {
		desc     string
		value    string
		disabled bool
	}{{"Save", "save", true}, {"Skip", "skip", false}} {
		var read struct {
			Value    string
			Disabled bool
		}
		r := a.run(call{"page__read", map[string]any{"path": formPath, "id": id(t, root.Inputs, b.desc)}})[0]
		testkit.Decode(t, r, &read)
		if read.Value != b.value || read.Disabled != b.disabled {
			t.Fatalf("the %s button reads %s while the name is missing, want value %s and disabled %t", b.desc, text(r), b.value, b.disabled)
		}
	}

	rep := a.write(formPath, id(t, root.Inputs, "Name"), "Ada")
	expect(t, "form data once the name is filled", dataOf(t, rep))

	rep = a.write(formPath+".address", id(t, address.Inputs, "City"), "ab")
	expect(t, "group data with a short city", dataOf(t, rep), `city validation error: "use at least 3 characters"`)
	if !changed(rep, formPath, "data") {
		t.Fatalf("the form's data did not change with the city error: %+v", rep.Changes)
	}
	rep = a.write(formPath+".address", id(t, address.Inputs, "City"), "abc")
	expect(t, "group data with a valid city", dataOf(t, rep))

	rep = a.write(formPath+".person", id(t, person.Inputs, "search"), "a")
	if rep.Diff == nil || rep.Diff.Inputs == nil {
		t.Fatalf("the search write brought no inputs: %+v", rep.Diff)
	}
	expect(t, "search results", inputs(rep.Diff.Inputs.Added), "button button choose Ada RW", "button button choose Bob RW")
	rep = a.click(formPath+".person", id(t, rep.Diff.Inputs.Added, "choose Ada"))
	expect(t, "search inputs after a pick", inputs(rep.Diff.Inputs.Added), "button button remove Ada RW")
	if len(rep.Diff.Inputs.Removed) != 3 {
		t.Fatalf("the pick removed %q, want the two results and the full field's search box", rep.Diff.Inputs.Removed)
	}

	rep = a.click(formPath+".tags", id(t, tags.Inputs, "add an item"))
	if rep.Diff == nil || rep.Diff.Children == nil {
		t.Fatalf("adding an item brought no child: %+v", rep.Diff)
	}
	expect(t, "list children added", children(rep.Diff.Children.Added), "item_0003 (an item of tags)")
	if !changed(rep, formPath, "data") {
		t.Fatalf("adding an empty item did not block submit or add its space: %+v", rep.Changes)
	}
	added := a.sync(formPath + ".tags.item_0003")[0]
	expect(t, "new item data", datas(added.Data), `tags validation error: "field is required"`)
	rep = a.click(formPath+".tags.item_0003", id(t, added.Inputs, "remove this item"))
	if rep.Diff == nil || !rep.Diff.Removed {
		t.Fatalf("removing the item left its space: %+v", rep.Diff)
	}
	expect(t, "list children after the remove", children(a.sync(formPath + ".tags")[0].Children), "item_0001 (an item of tags)", "item_0002 (an item of tags)")

	rep = a.write(formPath+".trip.from", id(t, from.Inputs, "From"), "x")
	if !changed(rep, formPath+".trip.to", "") {
		t.Fatalf("answering the first step added no next step: %+v", rep.Changes)
	}
	to := a.sync(formPath + ".trip.to")[0]
	expect(t, "next step data", datas(to.Data), `to validation error: "field is required"`)
	rep = a.write(formPath+".trip.to", id(t, to.Inputs, "To"), "y")
	expect(t, "next step data once answered", dataOf(t, rep))

	if err := (proto.NetworkEnable{}).Call(a.c.Page); err != nil {
		t.Fatal(err)
	}
	if err := (proto.NetworkEmulateNetworkConditions{DownloadThroughput: -1, UploadThroughput: 1000}).Call(a.c.Page); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(file, []byte(strings.Repeat("x", 3000)), 0o644); err != nil {
		t.Fatal(err)
	}
	a.c.Page.MustElement(".neisland-form-file-input").MustSetFiles(file)
	a.c.Page.Timeout(10 * time.Second).MustElement(".neisland-form-file-chip[data-uploading]")
	uploading := a.sync(formPath + ".docs")[0]
	expect(t, "file data while uploading", datas(uploading.Data), `docs validation error: "still uploading"`)
	if err := (proto.NetworkEmulateNetworkConditions{DownloadThroughput: -1, UploadThroughput: -1}).Call(a.c.Page); err != nil {
		t.Fatal(err)
	}
	a.c.Page.Timeout(10 * time.Second).MustElement(".neisland-form-file-link")
	docs = a.sync(formPath + ".docs")[0]
	if len(docs.Data) != 1 || docs.Data[0].Text != "uploaded file" {
		t.Fatalf("the uploaded file shows as %q", datas(docs.Data))
	}
	var uploaded struct {
		Name string
		Size int64
		Path string
	}
	if err := json.Unmarshal(docs.Data[0].Object, &uploaded); err != nil || uploaded.Name != "a.txt" || uploaded.Size != 3000 || !strings.HasSuffix(uploaded.Path, "/a.txt") {
		t.Fatalf("the uploaded file's data is %s (%v), want its name, size and workspace path", docs.Data[0].Object, err)
	}
	expect(t, "file inputs when full", inputs(docs.Inputs), "button button remove a.txt RW")
	rep = a.click(formPath+".docs", id(t, docs.Inputs, "remove a.txt"))
	expect(t, "file data after the remove", dataOf(t, rep))
	expect(t, "file inputs after the remove", inputs(rep.Diff.Inputs.Added), "input file file chooser RW")

	root = a.sync(formPath)[0]
	expect(t, "form data when complete", datas(root.Data))
	save := id(t, root.Inputs, "Save")
	r := a.run(call{"page__read", map[string]any{"path": formPath, "id": save}})[0]
	var read struct{ Value string }
	testkit.Decode(t, r, &read)
	if read.Value != "save" {
		t.Fatalf("the first Save button reads %s, want its value save", text(r))
	}
	if r := a.round([]testkit.Turn{testkit.Say("noted"), testkit.Say("ended")}, call{"page__click", map[string]any{"path": formPath, "id": save}})[0]; r.Error != api.ToolOk {
		t.Fatalf("the Save click failed: %s", text(r))
	}
	notice := a.notified(`"submit":"save"`)
	for _, want := range []string{`"name":"Ada"`, `"address":{"city":"abc"}`, `"tags":["a","b"]`, `"contacts":[{"email":"a@b.co"}]`, `"person":[{"value":1,"label":"Ada"}]`, `"trip":{"from":"x","to":"y"}`} {
		if !strings.Contains(notice, want) {
			t.Fatalf("the submit notification %q lacks %s", notice, want)
		}
	}
}

func TestAgentFixesAFieldErrorAndSubmitsToTheScript(t *testing.T) {
	a := open(t, `{fields: [
		{type: "text", label: "Name", name: "name"},
		{type: "email", label: "Email", name: "email"}
	], submits: [{text: "Send", value: "send"}]}`)
	root := a.sync(formPath)[0]
	expect(t, "form inputs", inputs(root.Inputs), "input text Name RW", "input email Email RW", "button submit Send RW")
	expect(t, "form data before any write", datas(root.Data))

	rep := a.write(formPath, id(t, root.Inputs, "Name"), "Ada")
	if !rep.Handled {
		t.Fatal("the name write ran no server handler")
	}
	rep = a.write(formPath, id(t, root.Inputs, "Email"), "ada")
	if !rep.Handled {
		t.Fatal("the bad email write ran no server handler")
	}
	expect(t, "form data with a bad email", dataOf(t, rep),
		`email validation error: "enter an email address"`,
		`submit blocked: "form is not completed or contains errors"`,
	)
	if r := a.run(call{"page__click", map[string]any{"path": formPath, "id": id(t, root.Inputs, "Send")}})[0]; r.Error == api.ToolOk || !strings.Contains(text(r), "target is disabled") {
		t.Fatalf("the Send click on a blocked form answered %s, want it refused as disabled", text(r))
	}
	rep = a.write(formPath, id(t, root.Inputs, "Email"), "ada@example.com")
	expect(t, "form data once the email is fixed", dataOf(t, rep))

	r := a.round([]testkit.Turn{testkit.Say("noted"), testkit.Say("ended")}, call{"page__click", map[string]any{"path": formPath, "id": id(t, root.Inputs, "Send")}})[0]
	if r.Error != api.ToolOk {
		t.Fatalf("the Send click failed: %s", text(r))
	}
	var click report
	testkit.Decode(t, r, &click)
	if !click.Handled {
		t.Fatalf("the Send click ran no server handler: %s", text(r))
	}
	notice := a.notified(`"submit":"send"`)
	for _, want := range []string{`"name":"Ada"`, `"email":"ada@example.com"`} {
		if !strings.Contains(notice, want) {
			t.Fatalf("the submit notification %q lacks %s", notice, want)
		}
	}
}

const nestedSpec = `{fields: [
	{type: "list", field: {type: "search", label: "Pick", name: "picks", max: 2, search: "(q) => [{value: 2, label: 'Two'}]"}, value: [[{value: 1, label: "One"}]]},
	{type: "list", field: {type: "file", label: "Scan", name: "scans"}},
	{type: "series", label: "Route", name: "route", required: true, fields: [
		"(v) => ({type: 'search', label: 'City', name: 'city', search: \"(q) => [{value: 'rome', label: 'Rome'}]\"})",
		"(v) => ({type: 'text', label: 'Note', name: 'note'})"
	]}
]}`

func TestAgentSeesFieldsInListItemsAndSeriesStepsWithoutExtraSpaces(t *testing.T) {
	a := open(t, nestedSpec)
	s := a.sync(formPath, formPath+".picks", formPath+".picks.item_0001", formPath+".scans", formPath+".route", formPath+".route.city")
	root, picks, pick, scans, route, city := s[0], s[1], s[2], s[3], s[4], s[5]
	expect(t, "form children", children(root.Children), "picks (Pick)", "route (Route)", "scans (Scan)")
	expect(t, "search list children", children(picks.Children), "item_0001 (an item of picks)")
	expect(t, "search item inputs", inputs(pick.Inputs), "input text search RW", "button button remove One RW", "button button remove this item RW")
	expect(t, "search item children", children(pick.Children))
	expect(t, "series children", children(route.Children), "city (City)")
	expect(t, "search step inputs", inputs(city.Inputs), "input text search RW")
	expect(t, "search step data", datas(city.Data), `city validation error: "field is required"`)
	expect(t, "search step children", children(city.Children))

	rep := a.click(formPath+".scans", id(t, scans.Inputs, "add an item"))
	if rep.Diff == nil || rep.Diff.Children == nil {
		t.Fatalf("adding a file item brought no child: %+v", rep.Diff)
	}
	expect(t, "file list children added", children(rep.Diff.Children.Added), "item_0001 (an item of scans)")
	scan := a.sync(formPath + ".scans.item_0001")[0]
	expect(t, "file item inputs", inputs(scan.Inputs), "input file file chooser RW", "button button remove this item RW")
	expect(t, "file item data", datas(scan.Data), `scans validation error: "field is required"`)
	expect(t, "file item children", children(scan.Children))
}

func TestListItemSpacesKeepScreenOrderAfterRemovals(t *testing.T) {
	a := open(t, `{fields: [{type: "list", field: {type: "text", label: "Tags", name: "tags"}, value: ["a", "b"]}]}`)
	first := a.sync(formPath + ".tags.item_0001")[0]
	a.click(formPath+".tags.item_0001", id(t, first.Inputs, "remove this item"))
	tags := a.sync(formPath + ".tags")[0]
	a.click(formPath+".tags", id(t, tags.Inputs, "add an item"))
	expect(t, "list children after removing the first and adding one", children(a.sync(formPath + ".tags")[0].Children), "item_0002 (an item of tags)", "item_0003 (an item of tags)")
}

func TestAgentSeesARejectedFileWithItsReason(t *testing.T) {
	a := open(t, `{fields: [{type: "file", label: "Docs", name: "docs", accept: ".txt"}]}`)
	file := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(file, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.c.Page.MustElement(".neisland-form-file-input").MustSetFiles(file)
	reason := a.c.Page.Timeout(10 * time.Second).MustElement(".neisland-form-file-reason").MustText()
	docs := a.sync(formPath + ".docs")[0]
	if len(docs.Data) != 1 || docs.Data[0].Text != "file not uploaded" {
		t.Fatalf("the rejected file shows as %q, want one file not uploaded", datas(docs.Data))
	}
	var rejected struct {
		Name  string
		Error string
	}
	if err := json.Unmarshal(docs.Data[0].Object, &rejected); err != nil || rejected.Name != "image.png" || rejected.Error != reason {
		t.Fatalf("the rejected file's data is %s (%v), want its name and the reason the user sees, %q", docs.Data[0].Object, err, reason)
	}
}
