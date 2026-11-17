//go:build browser

package devkit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"github.com/go-rod/rod/lib/input"
	"neui.dev"
	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/neisland"
)

func TestMain(m *testing.M) { stack.Main(m) }

func widgets() neui.Island {
	return neui.Island{
		Name:        "board",
		Placeholder: func(cur gox.Cursor) error { return cur.Text("idle") },
		Template: func(slot gox.Comp) gox.Elem {
			return func(cur gox.Cursor) error { return cur.Comp(slot) }
		},
		Library: neisland.RegisterAll,
	}
}

func TestDevkitTextShows(t *testing.T) {
	a := stack.Island(t, devkit.App{
		Island: widgets(),
		Script: `ui.root.set(ui.markdown({text: "hello there"})); ui.draw(1); ui.notify("drawn")`,
	})
	a.Notified()
	if text := a.Page.MustElement(".neisland-text").MustText(); text != "hello there" {
		t.Fatalf("the text shows %q", text)
	}
}

func TestDevkitFormSubmitsTypedText(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets()})
	if _, _, err := a.Start(context.Background(), `ui.root.set(ui.form({fields: [{type: "text", label: "Name", name: "name", required: true}], submits: [{text: "Save", value: "save"}]})); ui.notify(ui.draw())`); err != nil {
		t.Fatal(err)
	}
	a.Page.MustElement(`input[name="name"]`).MustInput("Ada")
	button := a.Page.MustElement(`button[type=submit][value="save"]`)
	button.MustWaitEnabled().MustClick()
	var got []struct {
		ID     uint
		Output struct {
			Data   map[string]any
			Submit string
		}
	}
	if err := json.Unmarshal(a.Notified().Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Output.Data["name"] != "Ada" || got[0].Output.Submit != "save" {
		t.Fatalf("the form submitted %+v, want name Ada by save", got)
	}
}

func TestDevkitFormSubmitsSelectedOptions(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets()})
	if _, _, err := a.Start(context.Background(), `ui.root.set(ui.form({fields: [
		{type: "select", label: "Plan", name: "plan", min: 1, options: [{value: "free", label: "Free"}, {value: {tier: "pro", seats: 5}, label: "Pro"}, {value: 3, label: "Three"}]},
		{type: "select", label: "Extras", name: "extras", min: 2, max: 2, options: [{value: "a", label: "A"}, {value: true, label: "Yes"}, {value: [1, 2], label: "Pair"}]}
	], submits: [{text: "Save", value: "save"}]})); ui.notify(ui.draw())`); err != nil {
		t.Fatal(err)
	}
	a.Page.MustElement(`input.neisland-form-radio[value="1"]`).MustClick()
	a.Page.MustElement(`input.neisland-form-checkbox[value="2"]`).MustClick()
	a.Page.MustElement(`input.neisland-form-checkbox[value="1"]`).MustClick()
	a.Page.MustElement(`button[type=submit][value="save"]`).MustWaitEnabled().MustClick()
	var got []struct {
		Output struct {
			Data   map[string]any
			Submit string
		}
	}
	if err := json.Unmarshal(a.Notified().Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d notifications, want 1", len(got))
	}
	data, err := json.Marshal(got[0].Output.Data)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"extras":[{"label":"Yes","value":true},{"label":"Pair","value":[1,2]}],"plan":[{"label":"Pro","value":{"seats":5,"tier":"pro"}}]}`
	if string(data) != want || got[0].Output.Submit != "save" {
		t.Fatalf("the form submitted %s by %q, want %s by save", data, got[0].Output.Submit, want)
	}
}

func TestDevkitFormSearchPicksInOrder(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets()})
	if _, _, err := a.Start(context.Background(), `ui.root.set(ui.form({fields: [
		{type: "search", label: "Approvers", name: "approvers", min: 1, max: 2, search: "(q) => [{value: {id: 7, code: \"C-7\"}, label: \"Marta Ortiz\"}, {value: \"c2\", label: \"Mark de Vries\"}, {value: 3, label: \"Samantha Okafor\"}, {value: \"c4\", label: \"Thomas Mayer\"}, {value: \"c5\", label: \"Ada Lovelace\"}].filter(c => c.label.toLowerCase().includes(q.toLowerCase()))"}
	], submits: [{text: "Save", value: "save"}]})); ui.notify(ui.draw())`); err != nil {
		t.Fatal(err)
	}
	a.Page.MustElement(`input.neisland-form-search-input`).MustInput("ma")
	a.Page.MustElementR(`.neisland-form-search-status`, `^4 matches$`)
	a.Page.MustElement(`button[aria-label="Choose Samantha Okafor"]`).MustClick()
	a.Page.MustElement(`button[aria-label="Remove Samantha Okafor"]`)
	a.Page.MustWait(`() => document.activeElement?.matches("input.neisland-form-search-input") && document.activeElement.value === ""`)
	a.Page.MustElement(`input.neisland-form-search-input`).MustInput("ma")
	a.Page.MustElementR(`.neisland-form-search-status`, `^3 matches$`)
	a.Page.Keyboard.MustType(input.Enter)
	time.Sleep(500 * time.Millisecond)
	if value := a.Page.MustElement(`input.neisland-form-search-input`).MustProperty("value").String(); value != "ma" {
		t.Fatalf("enter changed the search to %q", value)
	}
	a.Page.MustElementR(`.neisland-form-search-status`, `^3 matches$`)
	if has, _, err := a.Page.Has(`button[aria-label="Choose Samantha Okafor"]`); err != nil || has {
		t.Fatalf("the chosen option is still offered: %v", err)
	}
	a.Page.MustElement(`button[aria-label="Choose Marta Ortiz"]`).MustClick()
	if text := a.Page.MustElement(`.neisland-form-search-max`).MustText(); text != "Maximum of 2 chosen." {
		t.Fatalf("the max line shows %q", text)
	}
	if has, _, err := a.Page.Has(`input.neisland-form-search-input`); err != nil || has {
		t.Fatalf("the search input shows at max: %v", err)
	}
	a.Page.MustElement(`button[aria-label="Remove Samantha Okafor"]`).MustClick()
	a.Page.MustElement(`input.neisland-form-search-input`).MustInput("ada")
	a.Page.MustElement(`button[aria-label="Choose Ada Lovelace"]`).MustClick()
	a.Page.MustElement(`button[aria-label="Remove Ada Lovelace"]`)
	a.Page.MustElement(`button[type=submit][value="save"]`).MustWaitEnabled().MustClick()
	var got []struct {
		Output struct {
			Data   map[string]any
			Submit string
		}
	}
	if err := json.Unmarshal(a.Notified().Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d notifications, want 1", len(got))
	}
	data, err := json.Marshal(got[0].Output.Data)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"approvers":[{"label":"Marta Ortiz","value":{"code":"C-7","id":7}},{"label":"Ada Lovelace","value":"c5"}]}`
	if string(data) != want || got[0].Output.Submit != "save" {
		t.Fatalf("the form submitted %s by %q, want %s by save", data, got[0].Output.Submit, want)
	}
}
