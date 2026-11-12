package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/doors-dev/doors"
	"neui.dev"
)

type textInput struct {
	_           struct{}                        `additionalProperties:"false" description:"single-line text input; its value is the typed string; left out of the result when empty"`
	Type        string                          `json:"type" required:"true" enum:"text,email,tel,url,password" description:"text for plain text; email, tel and url check the format and bring up the matching keyboard; password hides what is typed"`
	Label       string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                          `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required    bool                            `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value       string                          `json:"value,omitempty" maxLength:"1000" description:"prefilled text"`
	Placeholder string                          `json:"placeholder,omitempty" maxLength:"200" description:"hint shown while the field is empty, such as an example value"`
	MinLength   int                             `json:"minLength,omitempty" minimum:"0" description:"fewest characters a non-empty value can have"`
	MaxLength   int                             `json:"maxLength,omitempty" minimum:"1" description:"most characters the user can type"`
	Pattern     string                          `json:"pattern,omitempty" maxLength:"500" description:"regular expression the whole value must match, in HTML pattern attribute syntax; no lookahead, lookbehind or backreferences, a pattern using them is refused when the form is created"`
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; not called when the field is empty; returns an error message to show under the field, or \"\" when valid"` // returns error message when invalid
}

func (p textInput) name() string {
	return p.Name
}

func (p textInput) label() string {
	return p.Label
}

type numberInput struct {
	_        struct{}                          `additionalProperties:"false" description:"number input, or a slider with range; its value is a number; left out of the result when empty"`
	Type     string                            `json:"type" required:"true" enum:"number,range" description:"number for a typed number; range for a slider between min and max"`
	Label    string                            `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                            `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                              `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value    *float64                          `json:"value,omitempty" description:"prefilled number"`
	Min      *float64                          `json:"min,omitempty" description:"lowest allowed value"`
	Max      *float64                          `json:"max,omitempty" description:"highest allowed value"`
	Step     *float64                          `json:"step,omitempty" exclusiveMinimum:"0" description:"allowed increment, such as 1 for whole numbers or 0.01 for money"`
	Validate neui.IslandCall[float64, string] `json:"validate,omitempty" description:"gets the entered number; not called when the field is empty; returns an error message to show under the field, or \"\" when valid"`
}

func (p numberInput) name() string {
	return p.Name
}

func (p numberInput) label() string {
	return p.Label
}

type dateInput struct {
	_        struct{}                        `additionalProperties:"false" description:"date or time picker; its value is a string in the type's format; left out of the result when empty"`
	Type     string                          `json:"type" required:"true" enum:"date,time,datetime-local,month,week" description:"what is picked, and the format of value, min, max and the result: date YYYY-MM-DD, time HH:MM, datetime-local YYYY-MM-DDTHH:MM, month YYYY-MM, week YYYY-Www"`
	Label    string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                          `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                            `json:"required,omitempty" description:"true makes the user pick a value before submitting"`
	Value    string                          `json:"value,omitempty" maxLength:"20" description:"prefilled value in the type's format"`
	Min      string                          `json:"min,omitempty" maxLength:"20" description:"earliest allowed value in the type's format"`
	Max      string                          `json:"max,omitempty" maxLength:"20" description:"latest allowed value in the type's format"`
	Validate neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the picked value in the type's format; not called when the field is empty; returns an error message to show under the field, or \"\" when valid"`
}

func (p dateInput) name() string {
	return p.Name
}

func (p dateInput) label() string {
	return p.Label
}

type textArea struct {
	_           struct{}                        `additionalProperties:"false" description:"multi-line text input for longer text; its value is the typed string with its line breaks; left out of the result when empty"`
	Type        string                          `json:"type" required:"true" const:"textarea"`
	Label       string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                          `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required    bool                            `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value       string                          `json:"value,omitempty" maxLength:"4000" description:"prefilled text; line breaks are kept"`
	Placeholder string                          `json:"placeholder,omitempty" maxLength:"200" description:"hint shown while the field is empty"`
	Rows        int                             `json:"rows,omitempty" minimum:"1" maximum:"30" description:"visible height in lines"`
	MinLength   int                             `json:"minLength,omitempty" minimum:"0" description:"fewest characters a non-empty value can have"`
	MaxLength   int                             `json:"maxLength,omitempty" minimum:"1" description:"most characters the user can type"`
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; not called when the field is empty; returns an error message to show under the field, or \"\" when valid"`
}

func (p textArea) name() string {
	return p.Name
}

func (p textArea) label() string {
	return p.Label
}

type option struct {
	_     struct{} `additionalProperties:"false" description:"one choice: value is what the result gets, label what the user sees; both are required"`
	Value any      `json:"value" required:"true" description:"any JSON value except null: a string, number, boolean, object or array; values must be unique within the field, each at most 4096 bytes as JSON"`
	Label string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text the user sees"`
}

type selectInput struct {
	_       struct{} `additionalProperties:"false" description:"choice from a fixed list of options; its value is an array of the chosen options as {value, label} objects in option order, a single item when max is 1, left out of the result when none is chosen; it shows as radio buttons, a dropdown or checkboxes depending on max and the number of options"`
	Type    string   `json:"type" required:"true" const:"select"`
	Label   string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name    string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Options []option `json:"options" required:"true" nullable:"false" minItems:"1" maxItems:"200" description:"options in display order"`
	Value   []option `json:"value,omitempty" maxItems:"200" description:"options chosen when the form shows, each as {value, label} with the value of one of the options"`
	Min     int      `json:"min,omitempty" minimum:"0" description:"fewest options the user must choose; 1 or more makes the field required"`
	Max     int      `json:"max,omitempty" minimum:"1" description:"most options the user can choose, at least min; 1 when omitted, for a single choice"`
}

func (p selectInput) name() string {
	return p.Name
}

func (p selectInput) label() string {
	return p.Label
}

type searchInput struct {
	_           struct{}                          `additionalProperties:"false" description:"choice from options the agent's search function finds for what the user types, for when the options are too many to list; its value is an array of the chosen options as {value, label} objects in the order they were picked, a single item when max is 1, left out of the result when none is chosen; chosen options show as removable chips, and the search box hides once max are chosen"`
	Type        string                            `json:"type" required:"true" const:"search"`
	Label       string                            `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                            `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Search      neui.IslandCall[string, []option] `json:"search" required:"true" description:"gets the typed text, trimmed and never empty; returns the matching options, best first; the first 8 the user hasn't chosen yet show, the rest are ignored; values must be unique among the returned options"`
	Value       []option                          `json:"value,omitempty" maxItems:"100" description:"options chosen when the form shows, each as {value, label}, with unique values and at most max of them"`
	Min         int                               `json:"min,omitempty" minimum:"0" description:"fewest options the user must choose; 1 or more makes the field required"`
	Max         int                               `json:"max,omitempty" minimum:"1" maximum:"100" description:"most options the user can choose, at least min; 1 when omitted, for a single choice"`
	Placeholder string                            `json:"placeholder,omitempty" maxLength:"200" description:"hint shown in the empty search box, such as what can be searched for; \"Search…\" when omitted"`
}

func (p searchInput) name() string {
	return p.Name
}

func (p searchInput) label() string {
	return p.Label
}

type checkboxInput struct {
	_        struct{} `additionalProperties:"false" description:"a single yes/no box with the label beside it; its value is true when checked, otherwise false"`
	Type     string   `json:"type" required:"true" const:"checkbox"`
	Label    string   `json:"label" required:"true" minLength:"1" maxLength:"200" description:"text beside the box, such as the statement the user agrees to"`
	Name     string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool     `json:"required,omitempty" description:"true makes the user check it before submitting, as for accepting terms"`
	Checked  bool     `json:"checked,omitempty" description:"true shows the box checked"`
}

func (p checkboxInput) name() string {
	return p.Name
}

func (p checkboxInput) label() string {
	return p.Label
}

type fileInput struct {
	_        struct{} `additionalProperties:"false" description:"file upload; each picked file uploads at once into the agent's workspace as <island dir>/<field id>/<file name>, a clashing name getting a (2) suffix; its value is that path, or an array of paths with multiple, left out of the result when no file is uploaded; failed uploads are left out and submit waits while any upload runs"`
	Type     string   `json:"type" required:"true" const:"file"`
	Label    string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool     `json:"required,omitempty" description:"true makes the user upload a file before submitting"`
	Accept   string   `json:"accept,omitempty" maxLength:"200" description:"file types the picker offers, as in the HTML accept attribute: comma-separated extensions or MIME types, such as .pdf,image/*; other types are refused, as are files over the app's size limit"`
	Multiple bool     `json:"multiple,omitempty" description:"true lets the user upload several files, and the value becomes an array of paths"`
	Max      int      `json:"max,omitempty" minimum:"1" maximum:"100" description:"most files with multiple, 10 when omitted"`
	Value    []string `json:"value,omitempty" maxItems:"100" description:"workspace paths of files to start with, shown like uploaded ones; each must exist; at most one without multiple; removing one drops it from the value without deleting it"`
}

func (p fileInput) name() string {
	return p.Name
}

func (p fileInput) label() string {
	return p.Label
}

type fieldGroup struct {
	_      struct{}    `additionalProperties:"false" description:"fields shown together under one label; its value is an object with each field's value under that field's name"`
	Type   string      `json:"type" required:"true" const:"group"`
	Label  string      `json:"label" required:"true" minLength:"1" maxLength:"80" description:"heading shown above the fields"`
	Name   string      `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the object in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Fields []formField `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the group"`
}

func (p fieldGroup) name() string {
	return p.Name
}

func (p fieldGroup) label() string {
	return p.Label
}

type listInput struct {
	_     struct{}  `additionalProperties:"false" description:"one field the user repeats, adding and removing copies within min and max; its value is an array with one item per copy, each the template's value"`
	Type  string    `json:"type" required:"true" const:"list"`
	Field formField `json:"field" required:"true" description:"template of each copy: any field except a checkbox, a group for several values per item, even another list; its name is the list's key in the result, and with a group the names of its fields become the keys of each item; each copy is required whatever the template's own required or min says, so an empty copy blocks submitting until it is filled or removed: a select or search needs at least one option chosen, a series every step answered; a group's fields keep their own required"`
	Value []any     `json:"value,omitempty" maxItems:"100" description:"initial copies, one per item, each shaped like the template's value: a string or number for a plain field, an array of {value, label} for select and search, an object for a group or series, an array for a list"`
	Min   int       `json:"min,omitempty" minimum:"0" maximum:"100" description:"fewest copies the user can submit"`
	Max   int       `json:"max,omitempty" minimum:"1" maximum:"100" description:"most copies the user can add"`
}

func (p listInput) name() string {
	return p.Field.v.name()
}

func (p listInput) label() string {
	return p.Field.v.label()
}

type seriesInput struct {
	_        struct{}                            `additionalProperties:"false" description:"fields shown one at a time, each chosen from the answer to the previous one, as in a wizard; its value is an object with each shown field's value under that field's name, like a group"`
	Type     string                              `json:"type" required:"true" const:"series"`
	Label    string                              `json:"label" required:"true" minLength:"1" maxLength:"80" description:"heading shown above the fields"`
	Name     string                              `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the object in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                                `json:"required,omitempty" description:"true makes the user answer every step before submitting; false lets the user leave the first step empty to skip the series, but once it is answered every step must be"`
	Fields   []neui.IslandCall[any, seriesField] `json:"fields" required:"true" nullable:"false" minItems:"2" maxItems:"20" description:"one step function per field, in order: each gets the value of the field the previous step returned, null for the first step, and returns the field to show; a series always shows one field per step, so to let a path end early return a field whose only option is a skip value, preselected; a step returns a plain field only, not a checkbox, group, list or series; a step field's own required is ignored: the first step is required when the series is, every later step always is, as the series is started by then, and a required select or search step needs at least one option chosen"` // first one accepts "" as prev value, null returns null there is no more fields
}

func (p seriesInput) name() string {
	return p.Name
}

func (p seriesInput) label() string {
	return p.Label
}

type formFieldParams interface {
	name() string
	label() string
}

type formField struct {
	v formFieldParams
}

func (formField) JSONSchemaAnyOf() []any {
	return []any{textInput{}, numberInput{}, dateInput{}, textArea{}, selectInput{}, searchInput{}, checkboxInput{}, fileInput{}, fieldGroup{}, listInput{}, seriesInput{}}
}

func (f *formField) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	switch head.Type {
	case "text", "email", "tel", "url", "password":
		f.v = new(textInput)
	case "number", "range":
		f.v = new(numberInput)
	case "date", "time", "datetime-local", "month", "week":
		f.v = new(dateInput)
	case "textarea":
		f.v = new(textArea)
	case "select":
		f.v = new(selectInput)
	case "search":
		f.v = new(searchInput)
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
	if err := json.Unmarshal(data, f.v); err != nil {
		return err
	}
	if list, ok := f.v.(*listInput); ok {
		if _, ok := list.Field.v.(*checkboxInput); ok {
			return fmt.Errorf("list %q: a list's field can't be a checkbox", list.name())
		}
	}
	return nil
}

type seriesField struct {
	v formFieldParams
}

func (seriesField) JSONSchemaAnyOf() []any {
	return []any{textInput{}, numberInput{}, dateInput{}, textArea{}, selectInput{}, searchInput{}, checkboxInput{}, fileInput{}}
}

func (f *seriesField) UnmarshalJSON(data []byte) error {
	var field formField
	if err := json.Unmarshal(data, &field); err != nil {
		return err
	}
	switch field.v.(type) {
	case *checkboxInput:
		return fmt.Errorf("a series field can't be a checkbox")
	case *fieldGroup, *listInput, *seriesInput:
		return fmt.Errorf("a series field can't be a group, a list or a series")
	}
	f.v = field.v
	return nil
}

type form struct {
	_       struct{}     `additionalProperties:"false" description:"form the user fills and submits; a submit gives {\"data\": object, \"submit\": the pressed button's value}, where data has each filled field's value under its name and leaves empty fields out: text, textarea and date give a string, number and range a number, checkbox a boolean, select and search an array of the chosen options as {\"value\": the option's value, \"label\": its label} objects, file the path of the uploaded file in the agent's workspace or an array of paths with multiple; a group nests its fields' values as an object, a list gives an array with one item per filled copy, a series an object like a group; cancel gives {\"canceled\": true}"`
	Fields  []formField  `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the form"`
	Submits []formSubmit `json:"submits,omitempty" maxItems:"5" description:"submit buttons in display order; the first is the default, pressed by Enter; omitted gives one Submit button; buttons stay disabled while a field has an error or a required field is empty"`
	Cancel  string       `json:"cancel,omitempty" minLength:"1" maxLength:"80" description:"label of a cancel button, shown after the submit buttons; it skips the checks and gives {\"canceled\": true}; omit for no cancel button"`
}

type formSubmit struct {
	_     struct{} `additionalProperties:"false"`
	Text  string   `json:"text" required:"true" minLength:"1" maxLength:"80" description:"label of the button"`
	Value string   `json:"value,omitempty" maxLength:"200" description:"what the result's submit gets when this button is pressed; \"\" when omitted"`
	Style string   `json:"style,omitempty" enum:"primary,secondary,success,danger" description:"visual weight; primary for the first button and secondary for the others when omitted"`
}

var errDuplicateName = errors.New("duplicate field name")
var errRequired = errors.New("field is required")
var errUploading = errors.New("files are still uploading")
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
	form() *root
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

func (f *formGroupBase) form() *root {
	return f.root
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

type formErrors int

const (
	formErrorsNone formErrors = iota
	formErrorsRequired
	formErrorsShown
)

func formErrorsOf(m map[string]any) formErrors {
	kind := formErrorsNone
	for _, v := range m {
		switch v := v.(type) {
		case error:
			if formShownErr(v) {
				return formErrorsShown
			}
			kind = formErrorsRequired
		case *map[string]any:
			kind = max(kind, formErrorsOf(*v))
			if kind == formErrorsShown {
				return kind
			}
		}
	}
	return kind
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
	item   doors.Beam[string]
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
	f.reserved[name] = struct{}{}
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
	f.reserved[name] = struct{}{}
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
			return v1
		}
		clone := maps.Clone(*v1)
		delete(clone, name)
		return &clone
	})
}
