//go:build browser

package devkit_test

import (
	"strings"
	"testing"

	"neui.dev/internal/devkit"
	"neui.dev/internal/testkit/stack"
)

func choiceCreate(t *testing.T, field string) string {
	t.Helper()
	return stack.ChoiceCreateError(t, widgets(), `ui.form({fields: [`+field+`]})`)
}

func expectCreateFails(t *testing.T, field string, want string) {
	t.Helper()
	got := choiceCreate(t, field)
	if got == "" || !strings.Contains(got, want) {
		t.Fatalf("creating %s failed with %q, want an error containing %q", field, got, want)
	}
}

func expectCreated(t *testing.T, field string) {
	t.Helper()
	if got := choiceCreate(t, field); got != "" {
		t.Fatalf("creating %s failed with %q, want it created", field, got)
	}
}

const abc = `[{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}]`

const abcdef = `[{value: "a", label: "A"}, {value: "b", label: "B"}, {value: "c", label: "C"}, {value: "d", label: "D"}, {value: "e", label: "E"}, {value: "f", label: "F"}]`

func TestSelectMinMaxCombinationsAtCreation(t *testing.T) {
	for _, c := range []struct {
		name  string
		field string
		fails string
	}{
		{"max omitted is 1", `{type: "select", label: "S", name: "s", min: 1, options: ` + abc + `}`, ""},
		{"min 2 above default max 1", `{type: "select", label: "S", name: "s", min: 2, options: ` + abc + `}`, "max 1 is below min 2"},
		{"min 3 above max 2", `{type: "select", label: "S", name: "s", min: 3, max: 2, options: ` + abc + `}`, "max 2 is below min 3"},
		{"min equal to max", `{type: "select", label: "S", name: "s", min: 3, max: 3, options: ` + abc + `}`, ""},
		{"min 0 max 2", `{type: "select", label: "S", name: "s", max: 2, options: ` + abc + `}`, ""},
		{"max above option count", `{type: "select", label: "S", name: "s", max: 5, options: ` + abc + `}`, ""},
		{"explicit max 0", `{type: "select", label: "S", name: "s", max: 0, options: ` + abc + `}`, "schema"},
		{"negative min", `{type: "select", label: "S", name: "s", min: -1, options: ` + abc + `}`, "schema"},
		{"no options", `{type: "select", label: "S", name: "s", options: []}`, "schema"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.fails == "" {
				expectCreated(t, c.field)
			} else {
				expectCreateFails(t, c.field, c.fails)
			}
		})
	}
}

func TestSelectOptionCapsAtCreation(t *testing.T) {
	for _, c := range []struct {
		name  string
		field string
		fails string
	}{
		{"duplicate string values", `{type: "select", label: "S", name: "s", options: [{value: "a", label: "A"}, {value: "a", label: "Again"}]}`, "already used by another option"},
		{"duplicate object values in another key order", `{type: "select", label: "S", name: "s", options: [{value: {x: 1, y: 2}, label: "A"}, {value: {y: 2, x: 1}, label: "Again"}]}`, "already used by another option"},
		{"null value", `{type: "select", label: "S", name: "s", options: [{value: null, label: "Nothing"}]}`, "has no value"},
		{"missing value", `{type: "select", label: "S", name: "s", options: [{label: "Nothing"}]}`, "schema"},
		{"empty label", `{type: "select", label: "S", name: "s", options: [{value: "a", label: ""}]}`, "schema"},
		{"label of 81 characters", `{type: "select", label: "S", name: "s", options: [{value: "a", label: "x".repeat(81)}]}`, "schema"},
		{"value of 4097 bytes", `{type: "select", label: "S", name: "s", options: [{value: "x".repeat(4095), label: "Big"}]}`, "most is 4096"},
		{"value of 4096 bytes", `{type: "select", label: "S", name: "s", options: [{value: "x".repeat(4094), label: "Big"}]}`, ""},
		{"200 options", `{type: "select", label: "S", name: "s", options: Array.from({length: 200}, (_, i) => ({value: i, label: "O" + i}))}`, ""},
		{"201 options", `{type: "select", label: "S", name: "s", options: Array.from({length: 201}, (_, i) => ({value: i, label: "O" + i}))}`, "schema"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.fails == "" {
				expectCreated(t, c.field)
			} else {
				expectCreateFails(t, c.field, c.fails)
			}
		})
	}
}

func TestSelectLookByMaxAndOptionCount(t *testing.T) {
	_, f := formApp(t, `{fields: [
		{type: "select", label: "Three optional", name: "a", options: `+abc+`},
		{type: "select", label: "Three required", name: "b", min: 1, options: `+abc+`},
		{type: "select", label: "Five", name: "c", options: [{value: 1, label: "1"}, {value: 2, label: "2"}, {value: 3, label: "3"}, {value: 4, label: "4"}, {value: 5, label: "5"}]},
		{type: "select", label: "Six optional", name: "d", options: `+abcdef+`},
		{type: "select", label: "Six required", name: "e", min: 1, max: 1, options: `+abcdef+`},
		{type: "select", label: "Two of three", name: "f", max: 2, options: `+abc+`},
		{type: "select", label: "Two of six", name: "g", min: 2, max: 2, options: `+abcdef+`},
		{type: "select", label: "Single option", name: "h", options: [{value: "only", label: "Only"}]}
	]}`)
	f.Field("Three optional").ExpectLook("radio")
	f.Field("Three optional").ExpectOptions("None", "A", "B", "C")
	f.Field("Three required").ExpectLook("radio")
	f.Field("Three required").ExpectOptions("A", "B", "C")
	f.Field("Five").ExpectLook("radio")
	f.Field("Five").ExpectOptions("None", "1", "2", "3", "4", "5")
	f.Field("Six optional").ExpectLook("dropdown")
	f.Field("Six optional").ExpectOptions("Choose…", "A", "B", "C", "D", "E", "F")
	f.Field("Six required").ExpectLook("dropdown")
	f.Field("Six required").ExpectOptions("Choose…", "A", "B", "C", "D", "E", "F")
	f.Field("Two of three").ExpectLook("checkbox")
	f.Field("Two of three").ExpectOptions("A", "B", "C")
	f.Field("Two of six").ExpectLook("checkbox")
	f.Field("Two of six").ExpectOptions("A", "B", "C", "D", "E", "F")
	f.Field("Single option").ExpectLook("radio")
	f.Field("Single option").ExpectOptions("None", "Only")
}

func TestSelectRadioOptionalNoneClearsValue(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "select", label: "Size", name: "size", options: `+abc+`},
		{type: "text", label: "Note", name: "note"}
	]}`)
	size := f.Field("Size")
	size.ExpectRequiredMark(false)
	size.ExpectValue("")
	size.Find(`input[value=""]`).WaitAttr("checked", "")
	f.ExpectSubmitDisabled(false)
	size.Choose("B")
	size.ExpectValue("B")
	size.Choose("C")
	size.ExpectValue("C")
	size.Choose("None")
	size.ExpectValue("")
	size.ExpectNoError()
	size.ExpectInvalid(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestSelectRadioSingleChoiceSubmitsOneItem(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Size", name: "size", options: `+abc+`}]}`)
	size := f.Field("Size")
	size.Choose("A")
	size.Choose("C")
	size.ExpectValue("C")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"size": [{"value": "c", "label": "C"}]}, "submit": ""}`)
}

func TestSelectRadioRequiredMarksUntilChosen(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Plan", name: "plan", min: 1, options: `+abc+`}]}`)
	plan := f.Field("Plan")
	plan.ExpectRequiredMark(true)
	plan.ExpectNoError()
	plan.ExpectInvalid(false)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	plan.Choose("B")
	plan.ExpectRequiredMark(false)
	plan.ExpectNoError()
	f.ExpectSubmitDisabled(false)
	f.ExpectSubmitMessage("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"plan": [{"value": "b", "label": "B"}]}, "submit": ""}`)
}

func TestSelectDropdownOptionalChooseClearsValue(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Country", name: "country", options: `+abcdef+`}]}`)
	country := f.Field("Country")
	country.ExpectRequiredMark(false)
	country.ExpectValue("")
	f.ExpectSubmitDisabled(false)
	country.Choose("E")
	country.ExpectValue("E")
	country.Choose("Choose…")
	country.ExpectValue("")
	country.ExpectNoError()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestSelectDropdownRequiredMarkReturnsOnChoose(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Country", name: "country", min: 1, options: `+abcdef+`}]}`)
	country := f.Field("Country")
	country.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	country.Choose("F")
	country.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	country.Choose("Choose…")
	country.ExpectRequiredMark(true)
	country.ExpectNoError()
	country.ExpectInvalid(false)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	country.Choose("A")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"country": [{"value": "a", "label": "A"}]}, "submit": ""}`)
}

func TestSelectCheckboxOptionalMaxError(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Tags", name: "tags", max: 2, options: `+abc+`}]}`)
	tags := f.Field("Tags")
	tags.ExpectRequiredMark(false)
	tags.Choose("A")
	tags.Choose("B")
	tags.ExpectNoError()
	tags.Choose("C")
	tags.ExpectError("choose at most 2")
	tags.ExpectInvalid(true)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Unchoose("A")
	tags.ExpectNoError()
	tags.ExpectInvalid(false)
	tags.Unchoose("B")
	tags.Unchoose("C")
	tags.ExpectValue("")
	tags.ExpectNoError()
	tags.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestSelectCheckboxMinErrorAndRequiredMark(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Tags", name: "tags", min: 2, max: 3, options: `+abcdef+`}]}`)
	tags := f.Field("Tags")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.ExpectInvalid(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Choose("B")
	tags.ExpectRequiredMark(true)
	tags.ExpectError("choose at least 2")
	tags.ExpectInvalid(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Unchoose("B")
	tags.ExpectRequiredMark(true)
	tags.ExpectNoError()
	tags.ExpectInvalid(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	tags.Choose("F")
	tags.Choose("A")
	tags.ExpectNoError()
	tags.ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	tags.Choose("D")
	tags.ExpectNoError()
	tags.Choose("C")
	tags.ExpectError("choose at most 3")
	tags.Unchoose("C")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": [{"value": "a", "label": "A"}, {"value": "d", "label": "D"}, {"value": "f", "label": "F"}]}, "submit": ""}`)
}

func TestSelectCheckboxRequiredOneIsEnough(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Tags", name: "tags", min: 1, max: 3, options: `+abc+`}]}`)
	tags := f.Field("Tags")
	tags.ExpectRequiredMark(true)
	tags.Choose("C")
	tags.ExpectRequiredMark(false)
	tags.ExpectNoError()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": [{"value": "c", "label": "C"}]}, "submit": ""}`)
}

func TestSelectOutputInOptionOrderNotClickOrder(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Tags", name: "tags", max: 6, options: `+abcdef+`}]}`)
	tags := f.Field("Tags")
	for _, o := range []string{"E", "A", "F", "C"} {
		tags.Choose(o)
	}
	tags.ExpectValue("A, C, E, F")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"tags": [{"value": "a", "label": "A"}, {"value": "c", "label": "C"}, {"value": "e", "label": "E"}, {"value": "f", "label": "F"}]}, "submit": ""}`)
}

const jsonTypeOptions = `[
	{value: "s", label: "String"},
	{value: "", label: "Empty string"},
	{value: 0, label: "Zero"},
	{value: -1.5, label: "Negative fraction"},
	{value: true, label: "True"},
	{value: false, label: "False"},
	{value: {a: [1, {b: null}]}, label: "Object"},
	{value: {}, label: "Empty object"},
	{value: [1, "x"], label: "Array"},
	{value: [], label: "Empty array"}
]`

const jsonTypeValues = `[
	{"value": "s", "label": "String"},
	{"value": "", "label": "Empty string"},
	{"value": 0, "label": "Zero"},
	{"value": -1.5, "label": "Negative fraction"},
	{"value": true, "label": "True"},
	{"value": false, "label": "False"},
	{"value": {"a": [1, {"b": null}]}, "label": "Object"},
	{"value": {}, "label": "Empty object"},
	{"value": [1, "x"], "label": "Array"},
	{"value": [], "label": "Empty array"}
]`

func TestSelectOptionValuesOfEveryJSONType(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "All", name: "all", max: 10, options: `+jsonTypeOptions+`}]}`)
	all := f.Field("All")
	for _, o := range []string{"Empty array", "Array", "Empty object", "Object", "False", "True", "Negative fraction", "Zero", "Empty string", "String"} {
		all.Choose(o)
	}
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"all": `+jsonTypeValues+`}, "submit": ""}`)
}

func TestSelectFalsyOptionValueIsAChoice(t *testing.T) {
	for _, c := range []struct{ label, want string }{
		{"Empty string", `""`},
		{"Zero", `0`},
		{"False", `false`},
		{"Empty array", `[]`},
		{"Empty object", `{}`},
	} {
		t.Run(c.label, func(t *testing.T) {
			a, f := formApp(t, `{fields: [{type: "select", label: "One", name: "one", min: 1, options: `+jsonTypeOptions+`}]}`)
			one := f.Field("One")
			one.ExpectLook("dropdown")
			one.Choose(c.label)
			one.ExpectRequiredMark(false)
			f.Submit("Submit")
			stack.ExpectJSON(t, a.Submitted(), `{"data": {"one": [{"value": `+c.want+`, "label": "`+c.label+`"}]}, "submit": ""}`)
		})
	}
}

func TestSelectDefaultsFromParamsShowInEveryLook(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "select", label: "Radio", name: "radio", value: [{value: "b", label: "B"}], options: `+abc+`},
		{type: "select", label: "Dropdown", name: "dropdown", value: [{value: "e", label: "E"}], options: `+abcdef+`},
		{type: "select", label: "Checks", name: "checks", max: 3, value: [{value: "c", label: "C"}, {value: "a", label: "A"}], options: `+abc+`},
		{type: "select", label: "Required", name: "required", min: 1, value: [{value: "c", label: "C"}], options: `+abc+`}
	]}`)
	f.Field("Radio").ExpectValue("B")
	f.Field("Dropdown").ExpectValue("E")
	f.Field("Checks").ExpectValue("A, C")
	f.Field("Required").ExpectValue("C")
	f.Field("Required").ExpectRequiredMark(false)
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {
		"radio": [{"value": "b", "label": "B"}],
		"dropdown": [{"value": "e", "label": "E"}],
		"checks": [{"value": "a", "label": "A"}, {"value": "c", "label": "C"}],
		"required": [{"value": "c", "label": "C"}]
	}, "submit": ""}`)
}

func TestSelectDefaultBelowMinShowsMinError(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "select", label: "Tags", name: "tags", min: 2, max: 3, value: [{value: "a", label: "A"}], options: `+abc+`}]}`)
	tags := f.Field("Tags")
	tags.ExpectValue("A")
	tags.ExpectRequiredMark(true)
	tags.ExpectError("choose at least 2")
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
}

func TestSelectDefaultMatchesObjectValueByContent(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "select", label: "Plan", name: "plan", value: [{value: {seats: 5, tier: "pro"}, label: "Pro"}], options: [
		{value: {tier: "free"}, label: "Free"}, {value: {tier: "pro", seats: 5}, label: "Pro"}
	]}]}`)
	f.Field("Plan").ExpectValue("Pro")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"plan": [{"value": {"tier": "pro", "seats": 5}, "label": "Pro"}]}, "submit": ""}`)
}

func TestSelectDefaultNotAmongOptionsFailsCreation(t *testing.T) {
	expectCreateFails(t, `{type: "select", label: "S", name: "s", value: [{value: "z", label: "Z"}], options: `+abc+`}`, "not one of the options")
	expectCreateFails(t, `{type: "select", label: "S", name: "s", value: [{value: 1, label: "One"}], options: [{value: "1", label: "One"}]}`, "not one of the options")
}

func TestSelectDefaultsFromListValue(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: [[{value: "b", label: "B"}], [{value: "c", label: "C"}, {value: "a", label: "A"}]], field: {type: "select", label: "Picks", name: "picks", max: 2, options: `+abc+`}}]}`)
	picks := f.List("Picks")
	picks.ExpectCount(2)
	picks.Item(0).Control().ExpectValue("B")
	picks.Item(1).Control().ExpectValue("A, C")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"picks": [[{"value": "b", "label": "B"}], [{"value": "a", "label": "A"}, {"value": "c", "label": "C"}]]}, "submit": ""}`)
}

func TestSelectListValueOverridesTemplateDefault(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: [[{value: "c", label: "C"}]], field: {type: "select", label: "Picks", name: "picks", value: [{value: "a", label: "A"}], options: `+abc+`}}]}`)
	picks := f.List("Picks")
	picks.ExpectCount(1)
	picks.Item(0).Control().ExpectValue("C")
	picks.Add()
	picks.Item(1).Control().ExpectValue("A")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"picks": [[{"value": "c", "label": "C"}], [{"value": "a", "label": "A"}]]}, "submit": ""}`)
}

func TestSelectWrongListValueShapesFailCreation(t *testing.T) {
	for _, c := range []struct {
		name  string
		value string
		fails string
	}{
		{"item not an array", `["a"]`, "want an array of {value, label} objects"},
		{"item entry not an object", `[["a"]]`, "want a {value, label} object"},
		{"entry without label", `[[{value: "a"}]]`, "needs a value and a string label"},
		{"entry with a number label", `[[{value: "a", label: 1}]]`, "needs a value and a string label"},
		{"entry without value", `[[{label: "A"}]]`, "needs a value and a string label"},
		{"entry not among options", `[[{value: "z", label: "Z"}]]`, "not one of the options"},
	} {
		t.Run(c.name, func(t *testing.T) {
			expectCreateFails(t, `{type: "list", value: `+c.value+`, field: {type: "select", label: "Picks", name: "picks", options: `+abc+`}}`, c.fails)
		})
	}
}

const people = `"(q) => [{value: {id: 7}, label: 'Marta Ortiz'}, {value: 'c2', label: 'Mark de Vries'}, {value: 3, label: 'Samantha Okafor'}, {value: 'c4', label: 'Thomas Mayer'}, {value: 'c5', label: 'Ada Lovelace'}].filter((c) => c.label.toLowerCase().includes(q.toLowerCase()))"`

func TestSearchMinMaxCombinationsAtCreation(t *testing.T) {
	for _, c := range []struct {
		name  string
		field string
		fails string
	}{
		{"max omitted is 1", `{type: "search", label: "S", name: "s", min: 1, search: ` + people + `}`, ""},
		{"min 2 above default max 1", `{type: "search", label: "S", name: "s", min: 2, search: ` + people + `}`, "max 1 is below min 2"},
		{"min 3 above max 2", `{type: "search", label: "S", name: "s", min: 3, max: 2, search: ` + people + `}`, "max 2 is below min 3"},
		{"min equal to max", `{type: "search", label: "S", name: "s", min: 2, max: 2, search: ` + people + `}`, ""},
		{"max 100", `{type: "search", label: "S", name: "s", max: 100, search: ` + people + `}`, ""},
		{"max 101", `{type: "search", label: "S", name: "s", max: 101, search: ` + people + `}`, "schema"},
		{"explicit max 0", `{type: "search", label: "S", name: "s", max: 0, search: ` + people + `}`, "schema"},
		{"no search function", `{type: "search", label: "S", name: "s"}`, "schema"},
		{"validate is not a search key", `{type: "search", label: "S", name: "s", validate: "(v) => ''", search: ` + people + `}`, "schema"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.fails == "" {
				expectCreated(t, c.field)
			} else {
				expectCreateFails(t, c.field, c.fails)
			}
		})
	}
}

func TestSearchSingleSwapsAfterRemove(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", search: `+people+`}]}`)
	owner := f.Field("Owner")
	owner.ExpectRequiredMark(false)
	owner.Find("input.neisland-form-search-input").WaitAttr("placeholder", "Search…")
	owner.SearchFor("mar")
	owner.ExpectResults("Marta Ortiz", "Mark de Vries")
	owner.PickResult("Mark de Vries")
	owner.ExpectChips("Mark de Vries")
	owner.ExpectSearchInput(false)
	owner.DropChip("Mark de Vries")
	owner.ExpectChips()
	owner.ExpectSearchInput(true)
	owner.SearchFor("ada")
	owner.PickResult("Ada Lovelace")
	owner.ExpectChips("Ada Lovelace")
	owner.ExpectSearchInput(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"owner": [{"value": "c5", "label": "Ada Lovelace"}]}, "submit": ""}`)
}

func TestSearchCustomPlaceholder(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", placeholder: "Name or email", search: `+people+`}]}`)
	f.Field("Owner").Find("input.neisland-form-search-input").WaitAttr("placeholder", "Name or email")
}

func TestSearchMultiChipsInPickOrderHideInputAtMax(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Team", name: "team", max: 3, search: `+people+`}]}`)
	team := f.Field("Team")
	team.SearchFor("sam")
	team.PickResult("Samantha Okafor")
	team.ExpectSearchInput(true)
	team.SearchFor("ada")
	team.PickResult("Ada Lovelace")
	team.ExpectChips("Samantha Okafor", "Ada Lovelace")
	team.ExpectSearchInput(true)
	team.ExpectMaxLine("")
	team.SearchFor("mar")
	team.PickResult("Marta Ortiz")
	team.ExpectChips("Samantha Okafor", "Ada Lovelace", "Marta Ortiz")
	team.ExpectSearchInput(false)
	team.ExpectMaxLine("Maximum of 3 chosen.")
	team.DropChip("Ada Lovelace")
	team.ExpectChips("Samantha Okafor", "Marta Ortiz")
	team.ExpectSearchInput(true)
	team.ExpectMaxLine("")
	team.SearchFor("tho")
	team.PickResult("Thomas Mayer")
	team.ExpectChips("Samantha Okafor", "Marta Ortiz", "Thomas Mayer")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"team": [
		{"value": 3, "label": "Samantha Okafor"},
		{"value": {"id": 7}, "label": "Marta Ortiz"},
		{"value": "c4", "label": "Thomas Mayer"}
	]}, "submit": ""}`)
}

func TestSearchResultsHideChosen(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Team", name: "team", max: 3, search: `+people+`}]}`)
	team := f.Field("Team")
	team.SearchFor("ma")
	team.ExpectResults("Marta Ortiz", "Mark de Vries", "Samantha Okafor", "Thomas Mayer")
	team.ExpectSearchStatus("4 matches")
	team.PickResult("Mark de Vries")
	team.SearchFor("ma")
	team.ExpectResults("Marta Ortiz", "Samantha Okafor", "Thomas Mayer")
	team.ExpectSearchStatus("3 matches")
	team.DropChip("Mark de Vries")
	team.SearchFor("mar")
	team.ExpectResults("Marta Ortiz", "Mark de Vries")
}

func TestSearchShowsFirstEightUnchosen(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Num", name: "num", max: 2, search: "(q) => Array.from({length: 10}, (_, i) => ({value: i + 1, label: 'N' + (i + 1)}))"}]}`)
	num := f.Field("Num")
	num.SearchFor("n")
	num.ExpectResults("N1", "N2", "N3", "N4", "N5", "N6", "N7", "N8")
	num.ExpectSearchStatus("8 matches")
	num.PickResult("N3")
	num.SearchFor("n")
	num.ExpectResults("N1", "N2", "N4", "N5", "N6", "N7", "N8", "N9")
}

func TestSearchStatusLines(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", search: `+people+`}]}`)
	owner := f.Field("Owner")
	owner.ExpectSearchStatus("")
	owner.SearchFor("zz")
	owner.ExpectResults()
	owner.ExpectSearchStatus("No matches for “zz”.")
	owner.SearchFor("ada")
	owner.ExpectSearchStatus("1 match")
	owner.SearchFor("")
	owner.ExpectResults()
	owner.ExpectSearchStatus("")
}

func TestSearchGetsTrimmedNonEmptyText(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Echo", name: "echo", search: "(q) => { if (q === '' || q !== q.trim()) throw new Error('got ' + JSON.stringify(q)); return [{value: q, label: 'got ' + q}] }"}]}`)
	echo := f.Field("Echo")
	echo.SearchFor("  ab  ")
	echo.ExpectResults("got ab")
	echo.SearchFor("   ")
	echo.ExpectResults()
	echo.ExpectSearchStatus("")
}

func TestSearchMinErrorsAndRequiredMark(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Team", name: "team", min: 2, max: 3, search: `+people+`}]}`)
	team := f.Field("Team")
	team.ExpectRequiredMark(true)
	team.ExpectNoError()
	team.ExpectInvalid(false)
	f.ExpectSubmitDisabled(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	team.SearchFor("ada")
	team.PickResult("Ada Lovelace")
	team.ExpectRequiredMark(true)
	team.ExpectError("choose at least 2")
	team.ExpectInvalid(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	team.DropChip("Ada Lovelace")
	team.ExpectRequiredMark(true)
	team.ExpectNoError()
	team.ExpectInvalid(false)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	team.SearchFor("ada")
	team.PickResult("Ada Lovelace")
	team.SearchFor("sam")
	team.PickResult("Samantha Okafor")
	team.ExpectNoError()
	team.ExpectRequiredMark(false)
	team.ExpectInvalid(false)
	f.ExpectSubmitDisabled(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"team": [{"value": "c5", "label": "Ada Lovelace"}, {"value": 3, "label": "Samantha Okafor"}]}, "submit": ""}`)
}

func TestSearchMinErrorAfterRemoveAtMax(t *testing.T) {
	_, f := formApp(t, `{fields: [{type: "search", label: "Pair", name: "pair", min: 2, max: 2, search: `+people+`}]}`)
	pair := f.Field("Pair")
	pair.SearchFor("ada")
	pair.PickResult("Ada Lovelace")
	pair.SearchFor("sam")
	pair.PickResult("Samantha Okafor")
	pair.ExpectSearchInput(false)
	pair.WaitGone(".neisland-form-error")
	pair.DropChip("Samantha Okafor")
	pair.ExpectSearchInput(true)
	pair.ExpectError("choose at least 2")
	pair.ExpectInvalid(true)
}

func TestSearchOptionalEmptyIsAbsent(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", search: `+people+`}]}`)
	owner := f.Field("Owner")
	owner.SearchFor("ada")
	owner.PickResult("Ada Lovelace")
	owner.DropChip("Ada Lovelace")
	owner.ExpectChips()
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}

func TestSearchDefaultsFromParams(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Team", name: "team", min: 1, max: 3, value: [{value: {id: 7}, label: "Marta Ortiz"}, {value: 3, label: "Samantha Okafor"}], search: `+people+`}]}`)
	team := f.Field("Team")
	team.ExpectChips("Marta Ortiz", "Samantha Okafor")
	team.ExpectRequiredMark(false)
	team.ExpectSearchInput(true)
	team.SearchFor("ma")
	team.ExpectResults("Mark de Vries", "Thomas Mayer")
	team.DropChip("Marta Ortiz")
	team.SearchFor("ada")
	team.PickResult("Ada Lovelace")
	team.ExpectChips("Samantha Okafor", "Ada Lovelace")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"team": [{"value": 3, "label": "Samantha Okafor"}, {"value": "c5", "label": "Ada Lovelace"}]}, "submit": ""}`)
}

func TestSearchDefaultsAtMaxHideInput(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", value: [{value: "anything", label: "Not in results"}], search: `+people+`}]}`)
	owner := f.Field("Owner")
	owner.ExpectChips("Not in results")
	owner.ExpectSearchInput(false)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"owner": [{"value": "anything", "label": "Not in results"}]}, "submit": ""}`)
}

func TestSearchDefaultsFromListValue(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "list", value: [[{value: 1, label: "One"}, {value: 2, label: "Two"}], []], field: {type: "search", label: "Finds", name: "finds", max: 2, value: [{value: "t", label: "Template"}], search: `+people+`}}]}`)
	finds := f.List("Finds")
	finds.ExpectCount(2)
	finds.Item(0).Control().ExpectChips("One", "Two")
	finds.Item(1).Control().ExpectChips()
	finds.Add()
	finds.Item(2).Control().ExpectChips("Template")
	finds.Item(1).Control().ExpectRequiredEmpty(true)
	f.ExpectSubmitDisabled(true)
	finds.Remove(1)
	finds.ExpectCount(2)
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {"finds": [[{"value": 1, "label": "One"}, {"value": 2, "label": "Two"}], [{"value": "t", "label": "Template"}]]}, "submit": ""}`)
}

func TestSearchWrongDefaultsFailCreation(t *testing.T) {
	for _, c := range []struct {
		name  string
		field string
		fails string
	}{
		{"more than max", `{type: "search", label: "S", name: "s", max: 1, value: [{value: 1, label: "One"}, {value: 2, label: "Two"}], search: ` + people + `}`, "2 default options, most is 1"},
		{"duplicate values", `{type: "search", label: "S", name: "s", max: 2, value: [{value: {a: 1, b: 2}, label: "One"}, {value: {b: 2, a: 1}, label: "Same"}], search: ` + people + `}`, "chosen twice"},
		{"null value", `{type: "search", label: "S", name: "s", value: [{value: null, label: "One"}], search: ` + people + `}`, "needs a value and a label"},
		{"value of 4097 bytes", `{type: "search", label: "S", name: "s", value: [{value: "x".repeat(4095), label: "Big"}], search: ` + people + `}`, "most is 4096"},
		{"list item not an array", `{type: "list", value: [{value: 1, label: "One"}], field: {type: "search", label: "S", name: "s", search: ` + people + `}}`, "want an array of {value, label} objects"},
		{"list entry not an object", `{type: "list", value: [[1]], field: {type: "search", label: "S", name: "s", search: ` + people + `}}`, "want a {value, label} object"},
		{"list entry without label", `{type: "list", value: [[{value: 1}]], field: {type: "search", label: "S", name: "s", search: ` + people + `}}`, "needs a value and a label"},
		{"list entry without value", `{type: "list", value: [[{label: "One"}]], field: {type: "search", label: "S", name: "s", search: ` + people + `}}`, "needs a value and a label"},
		{"list entry above max", `{type: "list", value: [[{value: 1, label: "One"}, {value: 2, label: "Two"}]], field: {type: "search", label: "S", name: "s", search: ` + people + `}}`, "most is 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			expectCreateFails(t, c.field, c.fails)
		})
	}
}

func TestSearchFailingFunctionIsReportedNotAFieldError(t *testing.T) {
	a, f := formApp(t, `{fields: [{type: "search", label: "Owner", name: "owner", min: 1, search: "(q) => { if (q === 'boom') throw new Error('boom'); return [{value: q, label: q}] }"}]}`)
	owner := f.Field("Owner")
	owner.SearchFor("boom")
	owner.ExpectSearchStatus("Search failed.")
	owner.ExpectResults()
	if data := string(a.ChoiceReport("call_error")); !strings.Contains(data, "boom") {
		t.Fatalf("the call_error reads %s, want the search failing with boom", data)
	}
	owner.ExpectNoError()
	owner.ExpectInvalid(false)
	owner.ExpectRequiredMark(true)
	f.ExpectSubmitMessage("form is not completed or contains errors")
	owner.SearchFor("fine")
	owner.ExpectResults("fine")
	owner.ExpectSearchStatus("1 match")
	owner.PickResult("fine")
	f.ExpectSubmitDisabled(false)
	f.ExpectSubmitMessage("")
}

func TestSearchSkipsBadOptions(t *testing.T) {
	a := stack.Island(t, devkit.App{Island: widgets(), Script: `ui.root.set(ui.form({fields: [{type: "search", label: "Pick", name: "pick", max: 3, search: "(q) => [{value: 'b', label: 'B'}, {value: null, label: 'Null'}, {value: 'b', label: 'B again'}, {value: 'x'.repeat(5000), label: 'Big'}, {value: 'c', label: 'C'}]"}]})); ui.notify(ui.draw())`})
	pick := a.Form().Field("Pick")
	pick.SearchFor("q")
	pick.ExpectResults("B", "C")
	pick.ExpectSearchStatus("2 matches")
	a.ChoiceReport("widget_error")
	pick.PickResult("C")
	pick.ExpectChips("C")
	pick.ExpectNoError()
}

func TestSelectNoneAndChooseClearDefaults(t *testing.T) {
	a, f := formApp(t, `{fields: [
		{type: "select", label: "Radio", name: "radio", value: [{value: "b", label: "B"}], options: `+abc+`},
		{type: "select", label: "Dropdown", name: "dropdown", value: [{value: "e", label: "E"}], options: `+abcdef+`}
	]}`)
	radio := f.Field("Radio")
	radio.ExpectValue("B")
	radio.Choose("None")
	radio.ExpectValue("")
	dropdown := f.Field("Dropdown")
	dropdown.ExpectValue("E")
	dropdown.Choose("Choose…")
	dropdown.ExpectValue("")
	f.Submit("Submit")
	stack.ExpectJSON(t, a.Submitted(), `{"data": {}, "submit": ""}`)
}
