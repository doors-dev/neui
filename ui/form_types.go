package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/swaggest/jsonschema-go"
	"neui.dev"
)

type textInput struct {
	_           struct{}                        `additionalProperties:"false" description:"single-line text input; its value is the typed string, \"\" when empty"`
	Type        string                          `json:"type" required:"true" enum:"text,email,tel,url,password,search" description:"text for plain text; email, tel and url check the format and bring up the matching keyboard; password hides what is typed; search shows a search box"`
	Label       string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                          `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required    bool                            `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value       string                          `json:"value,omitempty" maxLength:"1000" description:"prefilled text"`
	Placeholder string                          `json:"placeholder,omitempty" maxLength:"200" description:"hint shown while the field is empty, such as an example value"`
	MinLength   int                             `json:"minLength,omitempty" minimum:"0" description:"fewest characters a non-empty value can have"`
	MaxLength   int                             `json:"maxLength,omitempty" minimum:"1" description:"most characters the user can type"`
	Pattern     string                          `json:"pattern,omitempty" maxLength:"500" description:"JavaScript regular expression the whole value must match, as in the HTML pattern attribute"`
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; returns an error message to show under the field, or \"\" when valid"` // returns error message when invalid
}

func (p textInput) name() string {
	return p.Name
}

type numberInput struct {
	_        struct{}                          `additionalProperties:"false" description:"number input, or a slider with range; its value is a number, null when left empty"`
	Type     string                            `json:"type" required:"true" enum:"number,range" description:"number for a typed number; range for a slider between min and max"`
	Label    string                            `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                            `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                              `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value    *float64                          `json:"value,omitempty" description:"prefilled number"`
	Min      *float64                          `json:"min,omitempty" description:"lowest allowed value"`
	Max      *float64                          `json:"max,omitempty" description:"highest allowed value"`
	Step     *float64                          `json:"step,omitempty" exclusiveMinimum:"0" description:"allowed increment, such as 1 for whole numbers or 0.01 for money"`
	Validate neui.IslandCall[*float64, string] `json:"validate,omitempty" description:"gets the entered number, or null when empty; returns an error message to show under the field, or \"\" when valid"`
}

func (p numberInput) name() string {
	return p.Name
}

type dateInput struct {
	_        struct{}                        `additionalProperties:"false" description:"date or time picker; its value is a string in the type's format, \"\" when empty"`
	Type     string                          `json:"type" required:"true" enum:"date,time,datetime-local,month,week" description:"what is picked, and the format of value, min, max and the result: date YYYY-MM-DD, time HH:MM, datetime-local YYYY-MM-DDTHH:MM, month YYYY-MM, week YYYY-Www"`
	Label    string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                          `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                            `json:"required,omitempty" description:"true makes the user pick a value before submitting"`
	Value    string                          `json:"value,omitempty" maxLength:"20" description:"prefilled value in the type's format"`
	Min      string                          `json:"min,omitempty" maxLength:"20" description:"earliest allowed value in the type's format"`
	Max      string                          `json:"max,omitempty" maxLength:"20" description:"latest allowed value in the type's format"`
	Validate neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the picked value in the type's format, \"\" when empty; returns an error message to show under the field, or \"\" when valid"`
}

func (p dateInput) name() string {
	return p.Name
}

type textArea struct {
	_           struct{}                        `additionalProperties:"false" description:"multi-line text input for longer text; its value is the typed string with its line breaks, \"\" when empty"`
	Type        string                          `json:"type" required:"true" const:"textarea"`
	Label       string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                          `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required    bool                            `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value       string                          `json:"value,omitempty" maxLength:"4000" description:"prefilled text; line breaks are kept"`
	Placeholder string                          `json:"placeholder,omitempty" maxLength:"200" description:"hint shown while the field is empty"`
	Rows        int                             `json:"rows,omitempty" minimum:"1" maximum:"30" description:"visible height in lines"`
	MinLength   int                             `json:"minLength,omitempty" minimum:"0" description:"fewest characters a non-empty value can have"`
	MaxLength   int                             `json:"maxLength,omitempty" minimum:"1" description:"most characters the user can type"`
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; returns an error message to show under the field, or \"\" when valid"`
}

func (p textArea) name() string {
	return p.Name
}

type option struct {
	_     struct{} `additionalProperties:"false"`
	Value string   `json:"value" required:"true" minLength:"1" maxLength:"200" description:"what the result gets when this option is picked"`
	Label string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text the user sees"`
}

type options struct {
	list   []option
	search neui.IslandCall[string, []option]
}

func (options) JSONSchemaAnyOf() []any {
	return []any{[]option{}, neui.IslandCall[string, []option]("")}
}

func (options) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.Type = nil
	return nil
}

func (o *options) UnmarshalJSON(data []byte) error {
	if data = bytes.TrimSpace(data); len(data) > 0 && data[0] == '[' {
		return json.Unmarshal(data, &o.list)
	}
	return json.Unmarshal(data, &o.search)
}

type selectInput struct {
	_        struct{}                        `additionalProperties:"false" description:"dropdown to pick one option; its value is the picked option's value, \"\" when none"`
	Type     string                          `json:"type" required:"true" const:"select"`
	Label    string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                          `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                            `json:"required,omitempty" description:"true makes the user pick an option before submitting"`
	Options  options                         `json:"options" required:"true" description:"either a fixed array of options, or a function that gets the text the user typed in the dropdown and returns the matching options; the function is called with \"\" when the dropdown opens, for the initial options; use it for long or looked-up lists"`
	Value    string                          `json:"value,omitempty" maxLength:"200" description:"value of the option picked when the form shows"`
	Validate neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the picked value, \"\" when none; returns an error message to show under the field, or \"\" when valid"`
}

func (p selectInput) name() string {
	return p.Name
}

type multiSelect struct {
	_        struct{}                          `additionalProperties:"false" description:"dropdown to pick any number of options; its value is an array of the picked options' values, [] when none"`
	Type     string                            `json:"type" required:"true" const:"multiselect"`
	Label    string                            `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                            `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                              `json:"required,omitempty" description:"true makes the user pick at least one option before submitting"`
	Options  options                           `json:"options" required:"true" description:"either a fixed array of options, or a function that gets the text the user typed in the dropdown and returns the matching options; the function is called with \"\" when the dropdown opens, for the initial options; use it for long or looked-up lists"`
	Value    []string                          `json:"value,omitempty" maxItems:"200" uniqueItems:"true" items.maxLength:"200" description:"values of the options picked when the form shows"`
	Validate neui.IslandCall[[]string, string] `json:"validate,omitempty" description:"gets the array of picked values; returns an error message to show under the field, or \"\" when valid"`
}

func (p multiSelect) name() string {
	return p.Name
}

type radioInput struct {
	_        struct{}                        `additionalProperties:"false" description:"a few options shown at once, one of them pickable; its value is the picked option's value, \"\" when none; prefer select for more than about 6 options"`
	Type     string                          `json:"type" required:"true" const:"radio"`
	Label    string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                          `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                            `json:"required,omitempty" description:"true makes the user pick an option before submitting"`
	Options  []option                        `json:"options" required:"true" nullable:"false" minItems:"2" maxItems:"20" description:"options in display order"`
	Value    string                          `json:"value,omitempty" maxLength:"200" description:"value of the option picked when the form shows"`
	Validate neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the picked value, \"\" when none; returns an error message to show under the field, or \"\" when valid"`
}

func (p radioInput) name() string {
	return p.Name
}

type checkboxInput struct {
	_        struct{}                      `additionalProperties:"false" description:"a single yes/no box with the label beside it; its value is true when checked, otherwise false"`
	Type     string                        `json:"type" required:"true" const:"checkbox"`
	Label    string                        `json:"label" required:"true" minLength:"1" maxLength:"200" description:"text beside the box, such as the statement the user agrees to"`
	Name     string                        `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool                          `json:"required,omitempty" description:"true makes the user check it before submitting, as for accepting terms"`
	Checked  bool                          `json:"checked,omitempty" description:"true shows the box checked"`
	Validate neui.IslandCall[bool, string] `json:"validate,omitempty" description:"gets true when checked, otherwise false; returns an error message to show under the field, or \"\" when valid"`
}

func (p checkboxInput) name() string {
	return p.Name
}

type fileInput struct {
	_        struct{} `additionalProperties:"false" description:"file upload; the file is saved in the agent's workdir and its value is that file's path, or an array of paths with multiple"`
	Type     string   `json:"type" required:"true" const:"file"`
	Label    string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string   `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the value in the result; set it on every field except a list's template, where it is unused"`
	Required bool     `json:"required,omitempty" description:"true makes the user upload a file before submitting"`
	Accept   string   `json:"accept,omitempty" maxLength:"200" description:"file types the picker offers, as in the HTML accept attribute: comma-separated extensions or MIME types, such as .pdf,image/*"`
	Multiple bool     `json:"multiple,omitempty" description:"true lets the user upload several files, and the value becomes an array of paths"`
}

func (p fileInput) name() string {
	return p.Name
}

type fieldGroup struct {
	_      struct{}    `additionalProperties:"false" description:"fields shown together under one label; its value is an object with each field's value under that field's name"`
	Type   string      `json:"type" required:"true" const:"group"`
	Label  string      `json:"label,omitempty" maxLength:"80" description:"heading shown above the fields"`
	Name   string      `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the object in the result; set it unless this is a list's template, where it is unused"`
	Fields []formField `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the group"`
}

func (p fieldGroup) name() string {
	return p.Name
}

type listInput struct {
	_     struct{}  `additionalProperties:"false" description:"one field the user repeats, adding and removing copies within min and max; its value is an array with one item per copy, each the template's value"`
	Type  string    `json:"type" required:"true" const:"list"`
	Label string    `json:"label,omitempty" maxLength:"80" description:"heading shown above the copies"`
	Field formField `json:"field" required:"true" description:"template of each copy: any field, a group for several values per item, even another list; its own name is unused, and with a group the names of its fields become the keys of each item"`
	Value []any
	Min   int `json:"min,omitempty" minimum:"0" maximum:"100" description:"fewest copies the user can submit"`
	Max   int `json:"max,omitempty" minimum:"1" maximum:"100" description:"most copies the user can add"`
}

func (p listInput) name() string {
	return p.Field.v.name()
}

type seriesInput struct {
	_        struct{}                            `additionalProperties:"false" description:"fields shown one at a time, each chosen from the answer to the previous one, as in a wizard; its value is an object with each shown field's value under that field's name, like a group"`
	Type     string                              `json:"type" required:"true" const:"series"`
	Label    string                              `json:"label,omitempty" maxLength:"80" description:"heading shown above the fields"`
	Name     string                              `json:"name,omitempty" minLength:"1" maxLength:"80" description:"key of the object in the result; set it unless this is a list's template, where it is unused"`
	Required bool                                `json:"required,omitempty" description:"true makes the user answer every step before submitting; false lets the user leave the first step empty to skip the series, but once it is answered every step must be"`
	Fields   []neui.IslandCall[any, seriesField] `json:"fields" required:"true" nullable:"false" minItems:"2" maxItems:"20" description:"one step function per field, in order: each gets the value of the field the previous step returned, null for the first step, and returns the field to show; a series always shows one field per step, so to let a path end early return a field whose only option is a skip value, preselected; a step returns a plain field only, not a group, list or series; a step field's own required is ignored, the series' required applies"` // first one accepts "" as prev value, null returns null there is no more fields
}

func (p seriesInput) name() string {
	return p.Name
}

type formFieldParams interface {
	name() string
}

type formField struct {
	v formFieldParams
}

func (formField) JSONSchemaAnyOf() []any {
	return []any{textInput{}, numberInput{}, dateInput{}, textArea{}, selectInput{}, multiSelect{}, radioInput{}, checkboxInput{}, fileInput{}, fieldGroup{}, listInput{}, seriesInput{}}
}

func (f *formField) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	switch head.Type {
	case "text", "email", "tel", "url", "password", "search":
		f.v = new(textInput)
	case "number", "range":
		f.v = new(numberInput)
	case "date", "time", "datetime-local", "month", "week":
		f.v = new(dateInput)
	case "textarea":
		f.v = new(textArea)
	case "select":
		f.v = new(selectInput)
	case "multiselect":
		f.v = new(multiSelect)
	case "radio":
		f.v = new(radioInput)
	case "checkbox":
		f.v = new(checkboxInput)
	case "file":
		f.v = new(fileInput)
	case "group":
		f.v = new(fieldGroup)
	case "list":
		f.v = new(listInput)
	case "series":
		f.v = new(seriesInput)
	default:
		return fmt.Errorf("unknown field type %q", head.Type)
	}
	return json.Unmarshal(data, f.v)
}

type seriesField struct {
	v formFieldParams
}

func (seriesField) JSONSchemaAnyOf() []any {
	return []any{textInput{}, numberInput{}, dateInput{}, textArea{}, selectInput{}, multiSelect{}, radioInput{}, checkboxInput{}, fileInput{}}
}

func (f *seriesField) UnmarshalJSON(data []byte) error {
	var field formField
	if err := json.Unmarshal(data, &field); err != nil {
		return err
	}
	switch field.v.(type) {
	case *fieldGroup, *listInput, *seriesInput:
		return fmt.Errorf("a series field can't be a group, a list or a series")
	}
	f.v = field.v
	return nil
}

type form struct {
	_      struct{}    `additionalProperties:"false" description:"form the user fills and submits; the submit is one JSON object with each field's value under its name: text, textarea, date, select and radio give a string, number and range a number or null when empty, checkbox a boolean, multiselect an array of strings, file the path of the uploaded file in the agent's workdir or an array of paths with multiple; a group nests its fields' values as an object, a list gives an array with one item per copy, a series an object like a group"`
	Fields []formField `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the form"`
}

var errDuplicateName = errors.New("duplicate field name")
var errRequired = errors.New("field is required")
var errSystem = errors.New("couldn't check this value")
var errMarker = errors.New("system error marker, not rendered")

func formErrEqual(a, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Error() == b.Error()
}

type formController struct {
	errors doors.Source[map[string]error]
}

func (f *formController) deriveError(name string) doors.Source[error] {
	return doors.DeriveSourceEqual[map[string]error, error](f.errors, func(m map[string]error) error {
		return m[name]
	}, func(m map[string]error, err error) map[string]error {
		current, ok := m[name]
		switch {
		case err == nil && !ok:
			return m
		case err == nil:
			m = maps.Clone(m)
			delete(m, name)
			return m
		case !ok || !formErrEqual(current, err):
			m = maps.Clone(m)
			m[name] = err
			return m
		default:
			return m
		}
	}, formErrEqual)
}

type formGroup interface {
	mutex() *sync.Mutex
	scope() doors.Scopes
	setField(ctx context.Context, name string, value any, err error)
	newField(name string) (formGroupField, error)
	newGroup(name string) (*formGroupBase, error)
	deleteField(ctx context.Context, name string)
	deleteGroup(ctx context.Context, name string)
}

type root struct {
	mu     sync.Mutex
	values map[string]any
	scope  *doors.ScopeFrame
}

type formGroupBase struct {
	root      *root
	name      string
	parent    formGroup
	collector formGroupCollector
	errors    doors.Source[*map[string]any]
	reserved  map[string]struct{}
	values    map[string]any
}

func (f *formGroupBase) mutex() *sync.Mutex {
	return &f.root.mu
}

func (f *formGroupBase) scope() doors.Scopes {
	if f.parent == nil {
		return f.root.scope.Scope(false)
	}
	return f.parent.scope()
}

var _ formGroup = (*formGroupBase)(nil)

func (f *formGroupBase) hasErrors() bool {
	for name := range *f.errors.Get() {
		if name != "" {
			return true
		}
	}
	return false
}

func (f *formGroupBase) len() int {
	return len(f.values)
}

func (f *formGroupBase) destroy(ctx context.Context) {
	f.parent.deleteGroup(ctx, f.name)
}

func (f *formGroupBase) setField(ctx context.Context, name string, value any, err error) {
	if _, has := f.reserved[name]; !has {
		return
	}
	if value == nil {
		delete(f.values, name)
	} else {
		f.values[name] = value
	}
	f.errors.Mutate(ctx, func(m *map[string]any) *map[string]any {
		current, _ := (*m)[name].(error)
		switch {
		case formErrEqual(current, err):
			return m
		case err == nil:
			clone := maps.Clone(*m)
			delete(clone, name)
			return &clone
		default:
			clone := maps.Clone(*m)
			clone[name] = err
			return &clone
		}
	})
}

type formGroupField struct {
	parent formGroup
	name   string
	error  doors.Beam[error]
}

func (f formGroupField) mutex() *sync.Mutex {
	return f.parent.mutex()
}

func (f formGroupField) scope() doors.Scopes {
	return f.parent.scope()
}

func (f formGroupField) destroy(ctx context.Context) {
	f.parent.deleteField(ctx, f.name)
}

func (f formGroupField) set(ctx context.Context, value any, err error) {
	f.parent.setField(ctx, f.name, value, err)
}

func (f *formGroupBase) newField(name string) (formGroupField, error) {
	if _, has := f.reserved[name]; has {
		return formGroupField{}, errDuplicateName
	}
	return formGroupField{
		parent: f,
		name:   name,
		error: doors.DeriveBeamEqual[*map[string]any, error](
			f.errors,
			func(m *map[string]any) error {
				if m == nil {
					return nil
				}
				err, _ := (*m)[name].(error)
				return err
			},
			formErrEqual),
	}, nil
}

type formGroupCollector interface {
	collect(map[string]any) any
}

type defaultFormGroupCollector struct{}

func (defaultFormGroupCollector) value(v any) any {
	switch a := v.(type) {
	case *formGroupBase:
		return a.collect()
	default:
		return a
	}
}

func (d defaultFormGroupCollector) collect(v map[string]any) any {
	m := make(map[string]any, len(v))
	for key, value := range v {
		if key == "" {
			continue
		}
		m[key] = d.value(value)
	}
	return m
}

func (f *formGroupBase) newGroup(name string) (*formGroupBase, error) {
	if _, has := f.reserved[name]; has {
		return nil, errDuplicateName
	}
	errors := doors.DeriveSource[*map[string]any, *map[string]any](
		f.errors,
		func(v *map[string]any) *map[string]any {
			m, _ := (*v)[name].(*map[string]any)
			if m == nil {
				new := make(map[string]any)
				return &new
			}
			return m
		},
		func(v1, v2 *map[string]any) *map[string]any {
			stored, ok := (*v1)[name].(*map[string]any)
			switch {
			case !ok && len(*v2) == 0:
				return v1
			case len(*v2) == 0:
				clone := maps.Clone(*v1)
				delete(clone, name)
				return &clone
			case stored != v2:
				clone := maps.Clone(*v1)
				clone[name] = v2
				return &clone
			default:
				return v1
			}
		},
	)
	fg := &formGroupBase{
		root:      f.root,
		parent:    f,
		name:      name,
		errors:    errors,
		collector: defaultFormGroupCollector{},
		reserved:  make(map[string]struct{}),
		values:    make(map[string]any),
	}
	f.values[name] = fg
	return fg, nil
}

func (f *formGroupBase) deleteField(ctx context.Context, name string) {
	delete(f.reserved, name)
	delete(f.values, name)
	f.errors.Mutate(ctx, func(m *map[string]any) *map[string]any {
		_, ok := (*m)[name].(error)
		if !ok {
			return m
		}
		clone := maps.Clone(*m)
		delete(clone, name)
		return &clone
	})
}

func (f *formGroupBase) collect() any {
	return f.collector.collect(f.values)
}

func (f *formGroupBase) deleteGroup(ctx context.Context, name string) {
	delete(f.reserved, name)
	delete(f.values, name)
	f.errors.Mutate(ctx, func(v1 *map[string]any) *map[string]any {
		m, _ := (*v1)[name].(*map[string]any)
		if m == nil {
			return m
		}
		clone := maps.Clone(*v1)
		delete(clone, name)
		return &clone
	})
}
