package form

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

type textParams struct {
	_           struct{}                        `additionalProperties:"false" description:"single-line text input; its value is the typed string; left out of the result when empty"`
	Type        string                          `json:"type" required:"true" enum:"text,email,tel,url,password" description:"text for plain text, tel for a phone number; email and url also check the format; password hides what is typed"`
	Label       string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                          `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required    bool                            `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value       string                          `json:"value,omitempty" maxLength:"1000" description:"prefilled text"`
	Placeholder string                          `json:"placeholder,omitempty" maxLength:"200" description:"hint shown while the field is empty, such as an example value"`
	MinLength   int                             `json:"minLength,omitempty" minimum:"0" description:"fewest characters a non-empty value can have"`
	MaxLength   int                             `json:"maxLength,omitempty" minimum:"1" description:"most characters the user can type"`
	Pattern     string                          `json:"pattern,omitempty" maxLength:"500" description:"regular expression the whole value must match, in HTML pattern attribute syntax; no lookahead, lookbehind or backreferences, a pattern using them is refused when the form is created"`
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; not called when the field is empty; returns an error message for the user, or \"\" when valid"`
}

func (p textParams) name() string {
	return p.Name
}

func (p textParams) label() string {
	return p.Label
}

type numberParams struct {
	_        struct{}                         `additionalProperties:"false" description:"number input, or a slider with range; its value is a number; left out of the result when empty"`
	Type     string                           `json:"type" required:"true" enum:"number,range" description:"number for a typed number; range for a slider between min and max"`
	Label    string                           `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                           `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                             `json:"required,omitempty" description:"true makes the user fill it before submitting"`
	Value    *float64                         `json:"value,omitempty" description:"prefilled number"`
	Min      *float64                         `json:"min,omitempty" description:"lowest allowed value"`
	Max      *float64                         `json:"max,omitempty" description:"highest allowed value"`
	Step     *float64                         `json:"step,omitempty" exclusiveMinimum:"0" description:"allowed increment, such as 1 for whole numbers or 0.01 for money"`
	Validate neui.IslandCall[float64, string] `json:"validate,omitempty" description:"gets the entered number; not called when the field is empty; returns an error message for the user, or \"\" when valid"`
}

func (p numberParams) name() string {
	return p.Name
}

func (p numberParams) label() string {
	return p.Label
}

type dateParams struct {
	_        struct{}                        `additionalProperties:"false" description:"date or time picker; its value is a string in the type's format; left out of the result when empty"`
	Type     string                          `json:"type" required:"true" enum:"date,time,datetime-local,month,week" description:"what is picked, and the format of value, min, max and the result: date YYYY-MM-DD, time HH:MM, datetime-local YYYY-MM-DDTHH:MM, month YYYY-MM, week YYYY-Www"`
	Label    string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name     string                          `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                            `json:"required,omitempty" description:"true makes the user pick a value before submitting"`
	Value    string                          `json:"value,omitempty" maxLength:"20" description:"prefilled value in the type's format"`
	Min      string                          `json:"min,omitempty" maxLength:"20" description:"earliest allowed value in the type's format"`
	Max      string                          `json:"max,omitempty" maxLength:"20" description:"latest allowed value in the type's format"`
	Validate neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the picked value in the type's format; not called when the field is empty; returns an error message for the user, or \"\" when valid"`
}

func (p dateParams) name() string {
	return p.Name
}

func (p dateParams) label() string {
	return p.Label
}

type textareaParams struct {
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
	Validate    neui.IslandCall[string, string] `json:"validate,omitempty" description:"gets the entered text; not called when the field is empty; returns an error message for the user, or \"\" when valid"`
}

func (p textareaParams) name() string {
	return p.Name
}

func (p textareaParams) label() string {
	return p.Label
}

type option struct {
	_     struct{} `additionalProperties:"false" description:"one choice: value is what the result gets, label what the user sees; both are required"`
	Value any      `json:"value" required:"true" description:"any JSON value except null: a string, number, boolean, object or array; values must be unique within the field, each at most 4096 bytes as JSON"`
	Label string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text the user sees"`
}

type selectParams struct {
	_       struct{} `additionalProperties:"false" description:"choice from a fixed list of options; its value is an array of the chosen options as {value, label} objects in option order, a single item when max is 1, left out of the result when none is chosen"`
	Type    string   `json:"type" required:"true" const:"select"`
	Label   string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name    string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Options []option `json:"options" required:"true" nullable:"false" minItems:"1" maxItems:"200" description:"options in display order"`
	Value   []option `json:"value,omitempty" maxItems:"200" description:"options chosen when the form shows, each as {value, label} with the value of one of the options"`
	Min     int      `json:"min,omitempty" minimum:"0" description:"fewest options the user must choose; 1 or more makes the field required"`
	Max     int      `json:"max,omitempty" minimum:"1" description:"most options the user can choose, at least min; 1 when omitted, for a single choice"`
}

func (p selectParams) name() string {
	return p.Name
}

func (p selectParams) label() string {
	return p.Label
}

type searchParams struct {
	_           struct{}                          `additionalProperties:"false" description:"choice from options the agent's search function finds for what the user types, for when the options are too many to list; its value is an array of the chosen options as {value, label} objects in the order they were picked, a single item when max is 1, left out of the result when none is chosen"`
	Type        string                            `json:"type" required:"true" const:"search"`
	Label       string                            `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name        string                            `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Search      neui.IslandCall[string, []option] `json:"search" required:"true" description:"gets the typed text, trimmed and never empty; returns the matching options, best first; only the first 8 not already chosen are used; values must be unique among the returned options"`
	Value       []option                          `json:"value,omitempty" maxItems:"100" description:"options chosen when the form shows, each as {value, label}, with unique values and at most max of them"`
	Min         int                               `json:"min,omitempty" minimum:"0" description:"fewest options the user must choose; 1 or more makes the field required"`
	Max         int                               `json:"max,omitempty" minimum:"1" maximum:"100" description:"most options the user can choose, at least min; 1 when omitted, for a single choice"`
	Placeholder string                            `json:"placeholder,omitempty" maxLength:"200" description:"hint shown in the empty search box, such as what can be searched for; \"Search…\" when omitted"`
}

func (p searchParams) name() string {
	return p.Name
}

func (p searchParams) label() string {
	return p.Label
}

type checkboxParams struct {
	_        struct{} `additionalProperties:"false" description:"a single yes/no box; its value is true when checked, otherwise false"`
	Type     string   `json:"type" required:"true" const:"checkbox"`
	Label    string   `json:"label" required:"true" minLength:"1" maxLength:"200" description:"text beside the box, such as the statement the user agrees to"`
	Name     string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool     `json:"required,omitempty" description:"true makes the user check it before submitting, as for accepting terms"`
	Checked  bool     `json:"checked,omitempty" description:"true shows the box checked"`
}

func (p checkboxParams) name() string {
	return p.Name
}

func (p checkboxParams) label() string {
	return p.Label
}

type fileParams struct {
	_      struct{} `additionalProperties:"false" description:"file upload into the agent's workspace; its value is an array of the uploaded files' workspace paths, left out of the result when none is uploaded"`
	Type   string   `json:"type" required:"true" const:"file"`
	Label  string   `json:"label" required:"true" minLength:"1" maxLength:"80" description:"text shown with the field"`
	Name   string   `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the value in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Accept string   `json:"accept,omitempty" maxLength:"200" description:"allowed file types, as in the HTML accept attribute: comma-separated extensions or MIME types, such as .pdf,image/*"`
	Value  []string `json:"value,omitempty" maxItems:"100" description:"workspace paths of files to start with, counted toward max; each must exist; if the user removes one, the file is deleted from the workspace"`
	Min    int      `json:"min,omitempty" minimum:"0" maximum:"100" description:"fewest files the user must upload; 1 or more makes the field required"`
	Max    int      `json:"max,omitempty" minimum:"1" maximum:"100" description:"most files the user can upload, at least min; 1 when omitted, for a single file"`
}

func (p fileParams) name() string {
	return p.Name
}

func (p fileParams) label() string {
	return p.Label
}

type groupParams struct {
	_      struct{}      `additionalProperties:"false" description:"fields shown together under one label; its value is an object with each field's value under that field's name"`
	Type   string        `json:"type" required:"true" const:"group"`
	Label  string        `json:"label" required:"true" minLength:"1" maxLength:"80" description:"heading shown above the fields"`
	Name   string        `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the object in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Fields []fieldParams `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the group"`
}

func (p groupParams) name() string {
	return p.Name
}

func (p groupParams) label() string {
	return p.Label
}

type listParams struct {
	_     struct{}     `additionalProperties:"false" description:"one field the user repeats; its value is an array with one item per copy, each the template's value"`
	Type  string       `json:"type" required:"true" const:"list"`
	Field listTemplate `json:"field" required:"true" description:"template of each copy: any field except a checkbox, a group for several values per item, even another list; its name is the list's key in the result, and with a group the names of its fields become the keys of each item; each copy is required whatever the template's own required or min says: a select or search item has at least one option, a file item at least one file, a series item every step answered; a group's fields keep their own required"`
	Value []any        `json:"value,omitempty" maxItems:"100" description:"initial copies, one per item, each shaped like the template's value: a string or number for a plain field, an array of {value, label} for select and search, an array of workspace paths for a file, an object for a group or series, an array for a list"`
	Min   int          `json:"min,omitempty" minimum:"0" maximum:"100" description:"fewest copies the user can submit"`
	Max   int          `json:"max,omitempty" minimum:"1" maximum:"100" description:"most copies the user can add"`
}

func (p listParams) name() string {
	return p.Field.v.name()
}

func (p listParams) label() string {
	return p.Field.v.label()
}

type seriesParams struct {
	_        struct{}                           `additionalProperties:"false" description:"fields shown one at a time, each chosen from the answer to the previous one, as in a wizard; its value is an object with each shown field's value under that field's name, like a group"`
	Type     string                             `json:"type" required:"true" const:"series"`
	Label    string                             `json:"label" required:"true" minLength:"1" maxLength:"80" description:"heading shown above the fields"`
	Name     string                             `json:"name" required:"true" minLength:"1" maxLength:"80" description:"key of the object in the result, unique among the fields beside it; in a list's template it is the key of the whole list"`
	Required bool                               `json:"required,omitempty" description:"true makes the user answer every step before submitting; false lets the user leave the first step empty to skip the series, but once it is answered every step must be"`
	Fields   []neui.IslandCall[any, seriesStep] `json:"fields" required:"true" nullable:"false" minItems:"2" maxItems:"20" description:"one step function per field, in order: each gets the value of the field the previous step returned, null for the first step, and returns the field to show; a series always shows one field per step, so to let a path end early return a field whose only option is a skip value, preselected; a step returns a plain field only, not a checkbox, file, group, list or series; a step field's own required is ignored: the first step is required when the series is, every later step always is, as the series is started by then, and a required select or search step needs at least one option chosen"`
}

func (p seriesParams) name() string {
	return p.Name
}

func (p seriesParams) label() string {
	return p.Label
}

type anyParams interface {
	name() string
	label() string
}

type fieldParams struct {
	v anyParams
}

func (fieldParams) JSONSchemaAnyOf() []any {
	return []any{textParams{}, numberParams{}, dateParams{}, textareaParams{}, selectParams{}, searchParams{}, checkboxParams{}, fileParams{}, groupParams{}, listParams{}, seriesParams{}}
}

func (f *fieldParams) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	switch head.Type {
	case "text", "email", "tel", "url", "password":
		f.v = new(textParams)
	case "number", "range":
		f.v = new(numberParams)
	case "date", "time", "datetime-local", "month", "week":
		f.v = new(dateParams)
	case "textarea":
		f.v = new(textareaParams)
	case "select":
		f.v = new(selectParams)
	case "search":
		f.v = new(searchParams)
	case "checkbox":
		f.v = new(checkboxParams)
	case "file":
		f.v = new(fileParams)
	case "group":
		f.v = new(groupParams)
	case "list":
		f.v = new(listParams)
	case "series":
		f.v = new(seriesParams)
	default:
		return fmt.Errorf("unknown field type %q", head.Type)
	}
	return json.Unmarshal(data, f.v)
}

type listTemplate struct {
	v anyParams
}

func (listTemplate) JSONSchemaAnyOf() []any {
	return []any{textParams{}, numberParams{}, dateParams{}, textareaParams{}, selectParams{}, searchParams{}, fileParams{}, groupParams{}, listParams{}, seriesParams{}}
}

func (f *listTemplate) UnmarshalJSON(data []byte) error {
	var field fieldParams
	if err := json.Unmarshal(data, &field); err != nil {
		return err
	}
	f.v = field.v
	return nil
}

type seriesStep struct {
	v anyParams
}

func (seriesStep) JSONSchemaAnyOf() []any {
	return []any{textParams{}, numberParams{}, dateParams{}, textareaParams{}, selectParams{}, searchParams{}}
}

func (f *seriesStep) UnmarshalJSON(data []byte) error {
	var field fieldParams
	if err := json.Unmarshal(data, &field); err != nil {
		return err
	}
	f.v = field.v
	return nil
}

// Params is what a script passes to ui.form.
type Params struct {
	_       struct{}       `additionalProperties:"false"`
	Fields  []fieldParams  `json:"fields" required:"true" nullable:"false" minItems:"1" maxItems:"50" description:"fields in display order, each with a name unique within the form"`
	Submits []submitParams `json:"submits,omitempty" maxItems:"5" description:"submit buttons in display order; omitted gives one Submit button"`
}

type submitParams struct {
	_          struct{} `additionalProperties:"false"`
	Text       string   `json:"text" required:"true" minLength:"1" maxLength:"80" description:"label of the button"`
	Value      string   `json:"value,omitempty" maxLength:"200" description:"what the result's submit gets when this button is pressed; \"\" when omitted"`
	Style      string   `json:"style,omitempty" enum:"primary,secondary,success,danger" description:"visual weight; primary for the first button and secondary for the others when omitted"`
	Novalidate bool     `json:"novalidate,omitempty" description:"true lets this button work even while rules fail; its submit gives the data as entered; use it for skip/reject actions"`
}

var errDuplicateName = errors.New("duplicate field name")
var errNovalidateValue = errors.New("a novalidate button needs a value that no button without novalidate has")
var errRequired = errors.New("field is required")
var errUploading = errors.New("still uploading")
var errSystem = errors.New("couldn't check this value")
var errMarker = errors.New("system error marker, not rendered")

func errEqual(a, b error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Error() == b.Error()
}

type controller struct {
	errors doors.Source[map[string]error]
}

func (f *controller) deriveError(name string) doors.Source[error] {
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
		case !ok || !errEqual(current, err):
			m = maps.Clone(m)
			m[name] = err
			return m
		default:
			return m
		}
	}, errEqual)
}

type group interface {
	form() *root
	mutex() *sync.Mutex
	scope() doors.Scopes
	setField(ctx context.Context, name string, value any, err error)
	value(name string) any
	newField(name string) (groupField, error)
	newGroup(name string) (*groupBase, error)
	deleteField(ctx context.Context, name string)
	deleteGroup(ctx context.Context, name string)
}

type root struct {
	mu     sync.Mutex
	values map[string]any
	scope  *doors.ScopeCheckpoint
}

type groupBase struct {
	root      *root
	name      string
	parent    group
	collector groupCollector
	errors    doors.Source[*map[string]any]
	reserved  map[string]struct{}
	values    map[string]any
}

func (f *groupBase) form() *root {
	return f.root
}

func (f *groupBase) mutex() *sync.Mutex {
	return &f.root.mu
}

func (f *groupBase) scope() doors.Scopes {
	if f.parent == nil {
		return f.root.scope.Scope(false)
	}
	return f.parent.scope()
}

var _ group = (*groupBase)(nil)

type errorKind int

const (
	errorsNone errorKind = iota
	errorsRequired
	errorsShown
)

func errorsOf(m map[string]any) errorKind {
	kind := errorsNone
	for _, v := range m {
		switch v := v.(type) {
		case error:
			if shownErr(v) {
				return errorsShown
			}
			kind = errorsRequired
		case *map[string]any:
			kind = max(kind, errorsOf(*v))
			if kind == errorsShown {
				return kind
			}
		}
	}
	return kind
}

func (f *groupBase) len() int {
	return len(f.values)
}

func (f *groupBase) destroy(ctx context.Context) {
	f.parent.deleteGroup(ctx, f.name)
}

func (f *groupBase) value(name string) any {
	return f.values[name]
}

func (f *groupBase) setField(ctx context.Context, name string, value any, err error) {
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
		case errEqual(current, err):
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

type groupField struct {
	parent group
	name   string
	error  doors.Beam[error]
	item   doors.Beam[string]
}

func (f groupField) mutex() *sync.Mutex {
	return f.parent.mutex()
}

func (f groupField) scope() doors.Scopes {
	return f.parent.scope()
}

func (f groupField) destroy(ctx context.Context) {
	f.parent.deleteField(ctx, f.name)
}

func (f groupField) set(ctx context.Context, value any, err error) {
	f.parent.setField(ctx, f.name, value, err)
}

func (f groupField) value() any {
	return f.parent.value(f.name)
}

func (f *groupBase) newField(name string) (groupField, error) {
	if _, has := f.reserved[name]; has {
		return groupField{}, errDuplicateName
	}
	f.reserved[name] = struct{}{}
	return groupField{
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
			errEqual),
	}, nil
}

type groupCollector interface {
	collect(map[string]any) any
}

type defaultCollector struct{}

func (defaultCollector) value(v any) any {
	switch a := v.(type) {
	case *groupBase:
		return a.collect()
	default:
		return a
	}
}

func (d defaultCollector) collect(v map[string]any) any {
	m := make(map[string]any, len(v))
	for key, value := range v {
		if key == "" {
			continue
		}
		m[key] = d.value(value)
	}
	return m
}

func (f *groupBase) newGroup(name string) (*groupBase, error) {
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
	fg := &groupBase{
		root:      f.root,
		parent:    f,
		name:      name,
		errors:    errors,
		collector: defaultCollector{},
		reserved:  make(map[string]struct{}),
		values:    make(map[string]any),
	}
	f.values[name] = fg
	return fg, nil
}

func (f *groupBase) deleteField(ctx context.Context, name string) {
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

func (f *groupBase) collect() any {
	return f.collector.collect(f.values)
}

func (f *groupBase) deleteGroup(ctx context.Context, name string) {
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
