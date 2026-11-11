// Managed by GoX v0.3.4

//line form.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"slices"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
)

const formUploadLimit = 64 << 20

const formUploadMemory = 8 << 20

// NewForm builds a form widget for Library.Control: the user fills the
// fields and presses one of the submit buttons, which sends
// {"data": values, "submit": button value} to the script waiting in
// ui.draw(), after saving the chosen files in the agent's workspace under
// island/<field id>/. The cancel button sends {"canceled": true}. A submit
// carries at most 64 MB of files.
func NewForm(ctx context.Context, p form, submit func(ctx context.Context, s map[string]any) bool) (api.Widget[struct{}], error) {
	if len(p.Submits) == 0 {
		p.Submits = []formSubmit{{Text: "Submit"}}
	}
	for i := range p.Submits {
		switch {
		case p.Submits[i].Style != "":
		case i == 0:
			p.Submits[i].Style = "primary"
		default:
			p.Submits[i].Style = "secondary"
		}
	}
	m := make(map[string]any)
	root := &root{
		values: make(map[string]any),
		scope: new(doors.ScopeFrame),
		files: make(map[string]*formWidgetFile),
	}
	group := &formGroupBase{
		root: root,
		collector: defaultFormGroupCollector{},
		errors: doors.NewSource(&m),
		reserved: make(map[string]struct{}),
		values: root.values,
	}
	fields := make([]formWidgetField, 0, len(p.Fields))
	for _, field := range p.Fields {
		widget, err := newFormWidgetField(ctx, group, field.v)
		if err != nil {
			return nil, err
		}
		if err := widget.init(ctx, nil); err != nil {
			return nil, err
		}
		fields = append(fields, widget)
	}
	return &formWidget{
		id: doors.IDRand(),
		params: p,
		root: root,
		group: group,
		fields: fields,
		errors: doors.DeriveBeam(group.errors, func(m *map[string]any) formErrors {
			return formErrorsOf(*m)
		}),
		submit: submit,
	}, nil
}

func newFormWidgetField(ctx context.Context, group formGroup, field formFieldParams) (formWidgetField, error) {
	switch field := field.(type) {
	case *textInput:
		return newFormWidgetText(ctx, group, *field)
	case *numberInput:
		return newFormWidgetNumber(ctx, group, *field)
	case *dateInput:
		return newFormWidgetDate(ctx, group, *field)
	case *textArea:
		return newFormWidgetTextArea(ctx, group, *field)
	case *selectInput:
		return newFormWidgetSelect(ctx, group, *field)
	case *searchInput:
		return newFormWidgetSearch(ctx, group, *field)
	case *checkboxInput:
		return newFormWidgetCheckbox(ctx, group, *field)
	case *fileInput:
		return newFormWidgetFile(ctx, group, *field)
	case *fieldGroup:
		return newFormWidgetGroup(ctx, group, *field)
	case *listInput:
		return newFormWidgetList(ctx, group, *field)
	case *seriesInput:
		return newFormWidgetSeries(ctx, group, *field)
	default:
		return nil, fmt.Errorf("unknown field %T", field)
	}
}

type formWidget struct {
	id string
	params form
	root *root
	group *formGroupBase
	fields []formWidgetField
	errors doors.Beam[formErrors]
	locked doors.Setter
	submit func(ctx context.Context, s map[string]any) bool
}

func (f *formWidget) Update(context.Context, struct{}) {
}

func (f *formWidget) onSubmit(ctx context.Context, r doors.RequestRawForm) {
	r.SetRequestBodyLimit(formUploadLimit)
	parsed, err := r.ParseForm(formUploadMemory)
	if err != nil {
		neui.IslandReport(ctx, fmt.Errorf("form submit: %w", err))
		return
	}
	choice := f.params.Submits[0].Value
	if value := parsed.FormValue(f.id); slices.ContainsFunc(f.params.Submits, func(s formSubmit) bool {
		return s.Value == value
	}) {
		choice = value
	}
	type upload struct {
		dir string
		input string
		files []formFileChosen
	}
	f.root.mu.Lock()
	if formErrorsOf(*f.group.errors.Get()) != formErrorsNone {
		f.root.mu.Unlock()
		return
	}
	uploads := make([]upload, 0, len(f.root.files))
	for _, file := range f.root.files {
		uploads = append(uploads, upload{dir: file.dir(), input: file.inputName(), files: *file.chosen.Get()})
	}
	data := f.group.collect()
	f.root.mu.Unlock()
	fs := neui.IslandFS(ctx)
	for _, upload := range uploads {
		if len(upload.files) == 0 {
			continue
		}
		if fs == nil {
			neui.IslandReport(ctx, errors.New("form submit: no workspace to save the files in"))
			return
		}
		var headers []*multipart.FileHeader
		if parsed.Form() != nil {
			headers = parsed.Form().File[upload.input]
		}
		for _, chosen := range upload.files {
			i := slices.IndexFunc(headers, func(h *multipart.FileHeader) bool {
				return h.Filename == chosen.Name
			})
			if i == -1 {
				neui.IslandReport(ctx, fmt.Errorf("form submit: file %q didn't arrive", chosen.Name))
				return
			}
			src, err := headers[i].Open()
			if err != nil {
				neui.IslandReport(ctx, fmt.Errorf("form submit: file %q: %w", chosen.Name, err))
				return
			}
			dst, err := fs.Create(path.Join(upload.dir, chosen.Name))
			if err != nil {
				src.Close()
				neui.IslandReport(ctx, fmt.Errorf("form submit: file %q: %w", chosen.Name, err))
				return
			}
			_, err = io.Copy(dst, src)
			src.Close()
			if closeErr := dst.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				neui.IslandReport(ctx, fmt.Errorf("form submit: file %q: %w", chosen.Name, err))
				return
			}
		}
	}
	f.submit(ctx, map[string]any{"data": data, "submit": choice})
}

//line form.gox:197
func (f *formWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form.gox:199
		scope := f.root.scope.Scope(true).And(neui.IslandScope(ctx))
	errs, _ := f.errors.ReadAndSub(ctx, func(ctx context.Context, errs formErrors) bool {
		doors.Call(ctx, f.locked.Set("disabled", errs != formErrorsNone))
		return false
	})

		__e = __c.Init("form"); if __e != nil { return }
		{
//line form.gox:206
			__e = __c.Modify(doors.ARawSubmit{
			Scope: scope,
			RequestTimeout: 5 * time.Minute,
			On: func(ctx context.Context, r doors.RequestRawForm) bool {
				f.onSubmit(ctx, r)
				return false
			},
		}); if __e != nil { return }
//line form.gox:214
			__e = __c.Set("class", "kisland-form"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form.gox:215
			for _, field := range f.fields {
//line form.gox:216
				__e = __c.Any(field); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form.gox:218
				__e = __c.Set("class", "kisland-form-actions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form.gox:219
				if f.params.Cancel != "" {
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form.gox:221
						__e = __c.Modify(doors.AClick{
						Scope: scope,
						On: func(ctx context.Context, r doors.RequestPointer) bool {
							f.submit(ctx, map[string]any{"canceled": true})
							return false
						},
					}); if __e != nil { return }
//line form.gox:228
						__e = __c.Set("type", "button"); if __e != nil { return }
//line form.gox:229
						__e = __c.Set("class", "kisland-button kisland-button-md kisland-button-secondary"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form.gox:230
						__e = __c.Any(f.params.Cancel); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line form.gox:233
				for _, s := range f.params.Submits {
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form.gox:235
						__e = __c.Modify(&f.locked); if __e != nil { return }
//line form.gox:236
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line form.gox:237
						__e = __c.Set("class", "kisland-button kisland-button-md kisland-button-" + s.Style); if __e != nil { return }
//line form.gox:238
						__e = __c.Set("data-submit", s.Value); if __e != nil { return }
//line form.gox:239
						__e = __c.Set("disabled", errs != formErrorsNone); if __e != nil { return }
//line form.gox:240
						__e = __c.Set("aria-describedby", f.id + "-s"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form.gox:241
						__e = __c.Any(s.Text); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line form.gox:244
				__e = __c.Any(f.errors.Bind(func(errs formErrors) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form.gox:245
					if errs == formErrorsShown {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form.gox:246
							__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form.gox:246
							__e = __c.Set("id", f.id + "-s"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("fix the errors above"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form.gox:247
					} else if errs == formErrorsRequired {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form.gox:248
							__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form.gox:248
							__e = __c.Set("id", f.id + "-s"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("fill the fields marked *"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line form.gox:250
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form.gox:252
				__e = __c.Set("type", "hidden"); if __e != nil { return }
//line form.gox:252
				__e = __c.Set("name", f.id); if __e != nil { return }
//line form.gox:252
				__e = __c.Set("value", f.params.Submits[0].Value); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
//line form.gox:253
				__e = __c.Set("data:name", f.id); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const form = document.currentScript.parentElement\n\t\t\tconst choice = form.querySelector(\n\t\t\t\t`input[type=\"hidden\"][name=\"${$data(\"name\")}\"]`,\n\t\t\t)\n\t\t\tform.addEventListener(\"click\", (event) => {\n\t\t\t\tconst button = event.target.closest(\"button[type=submit][data-submit]\")\n\t\t\t\tif (button && button.form === form) {\n\t\t\t\t\tchoice.value = button.dataset.submit\n\t\t\t\t}\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form.gox:266
}

func formShownErr(err error) bool {
	return err != nil && !errors.Is(err, errRequired)
}

func formAria(b bool) any {
	if b {
		return "true"
	}
	return nil
}

type formWidgetField interface {
	gox.Comp
	init(ctx context.Context, value any) error
	destroy(ctx context.Context)
	name() string
}

/*
func NewForm(ctx context.Context, p form, submit func(ctx context.Context, s map[string]any) bool) (api.Widget[struct{}], error) {
	state := formWidgetGroupState{
		state: doors.NewSourceEqual(formWidgetGroupValue{
			values: make(map[string]any),
			errors: make(map[string]string),
		}, nil),
	}
	fields := make([]formWidgetField, 0, len(p.Fields))
	for _, field := range p.Fields {
		comp, err := newFormWidgetField(ctx, state, field.v)
		if err != nil {
			return nil, err
		}
		fields = append(fields, comp)
	}
	return &formWidget{
		parms: p,
		submit: submit,
		state: state,
		fields: fields,
	}, nil
}

type formWidget struct {
	parms form
	state formWidgetGroupState
	submit func(ctx context.Context, s map[string]any) bool
	fields []formWidgetField
}

func (f *formWidget) Update(context.Context, struct{}) {
}

elem (f *formWidget) Main() {
}

type formWidgetField interface {
	gox.Comp
	name() string
}

func newFormWidgetField(ctx context.Context, ctrl formGroupController, hook hook, field formFieldParams) (formWidgetField, error) {
	switch field := field.(type) {
	case *textInput:
		return newFormWidgetText(ctx, state, *field)
	case *numberInput:
		return newFormWidgetNumber(ctx, state, *field)
	case *dateInput:
		return newFormWidgetDate(ctx, state, *field)
	case *textArea:
		return newFormWidgetTextArea(ctx, state, *field)
	case *selectInput:
		return newFormWidgetSelect(ctx, state, *field)
	case *multiSelect:
		return newFormWidgetMultiSelect(ctx, state, *field)
	case *radioInput:
		return newFormWidgetRadio(ctx, state, *field)
	case *checkboxInput:
		return newFormWidgetCheckbox(ctx, state, *field)
	case *fileInput:
		return newFormWidgetFile(ctx, state, *field)
	case *fieldList:
		return newFormWidgetList(ctx, state, *field)
	case *seriesInput:
		return newFormWidgetSeries(ctx, state, *field)
	case *fieldGroup:
		return newFormWidgetGroup(ctx, state, *field)
	default:
		return nil, fmt.Errorf("unknown field %T", field)
	}
}

func newFormWidgetGroup(ctx context.Context, parent formWidgetGroupState, params fieldGroup) (formWidgetGroup, error) {
	state, err := parent.newGroup(ctx, params.Name)
	if err != nil {
		return formWidgetGroup{}, err
	}
	fields := make([]formWidgetField, 0, len(params.Fields))
	for _, field := range params.Fields {
		comp, err := newFormWidgetField(ctx, state, field.v)
		if err != nil {
			return formWidgetGroup{}, err
		}
		fields = append(fields, comp)
	}
	return formWidgetGroup{
		params: params,
		state: state,
		fields: fields,
	}, nil
}

func (f formWidgetGroup) name() string {
	return f.params.Name
}

type formWidgetGroup struct {
	params fieldGroup
	state formWidgetGroupState
	fields []formWidgetField
}

elem (f formWidgetGroup) Main() {
	~(for _, f := range f.fields {
		~(f)
	})
}

func newFormWidgetText(ctx context.Context, parent formWidgetGroupState, params textInput) (formWidgetText, error) {
	var value *string
	if params.Value != "" {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetText{}, err
	}
	return formWidgetText{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetText) name() string {
	return f.params.Name
}

type formWidgetText struct {
	params textInput
	state formWidgetFieldState[string]
}

elem (f formWidgetText) Main() {
}

func newFormWidgetNumber(ctx context.Context, parent formWidgetGroupState, params numberInput) (formWidgetNumber, error) {
	state, err := parent.newField(ctx, params.Name, params.Value, "")
	if err != nil {
		return formWidgetNumber{}, err
	}
	return formWidgetNumber{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetNumber) name() string {
	return f.params.Name
}

type formWidgetNumber struct {
	params numberInput
	state formWidgetFieldState[float64]
}

elem (f formWidgetNumber) Main() {
}

func newFormWidgetDate(ctx context.Context, parent formWidgetGroupState, params dateInput) (formWidgetDate, error) {
	var value *string
	if params.Value != "" {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetDate{}, err
	}
	return formWidgetDate{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetDate) name() string {
	return f.params.Name
}

type formWidgetDate struct {
	params dateInput
	state formWidgetFieldState[string]
}

elem (f formWidgetDate) Main() {
}

func newFormWidgetTextArea(ctx context.Context, parent formWidgetGroupState, params textArea) (formWidgetTextArea, error) {
	var value *string
	if params.Value != "" {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetTextArea{}, err
	}
	return formWidgetTextArea{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetTextArea) name() string {
	return f.params.Name
}

type formWidgetTextArea struct {
	params textArea
	state formWidgetFieldState[string]
}

elem (f formWidgetTextArea) Main() {
}

func newFormWidgetSelect(ctx context.Context, parent formWidgetGroupState, params selectInput) (formWidgetSelect, error) {
	var value *string
	if params.Value != "" {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetSelect{}, err
	}
	return formWidgetSelect{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetSelect) name() string {
	return f.params.Name
}

type formWidgetSelect struct {
	params selectInput
	state formWidgetFieldState[string]
}

elem (f formWidgetSelect) Main() {
}

func newFormWidgetMultiSelect(ctx context.Context, parent formWidgetGroupState, params multiSelect) (formWidgetMultiSelect, error) {
	var value *[]string
	if len(params.Value) > 0 {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetMultiSelect{}, err
	}
	return formWidgetMultiSelect{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetMultiSelect) name() string {
	return f.params.Name
}

type formWidgetMultiSelect struct {
	params multiSelect
	state formWidgetFieldState[[]string]
}

elem (f formWidgetMultiSelect) Main() {
}

func newFormWidgetRadio(ctx context.Context, parent formWidgetGroupState, params radioInput) (formWidgetRadio, error) {
	var value *string
	if params.Value != "" {
		value = &params.Value
	}
	state, err := parent.newField(ctx, params.Name, value, "")
	if err != nil {
		return formWidgetRadio{}, err
	}
	return formWidgetRadio{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetRadio) name() string {
	return f.params.Name
}

type formWidgetRadio struct {
	params radioInput
	state formWidgetFieldState[string]
}

elem (f formWidgetRadio) Main() {
}

func newFormWidgetCheckbox(ctx context.Context, parent formWidgetGroupState, params checkboxInput) (formWidgetCheckbox, error) {
	state, err := parent.newField(ctx, params.Name, &params.Checked, "")
	if err != nil {
		return formWidgetCheckbox{}, err
	}
	return formWidgetCheckbox{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetCheckbox) name() string {
	return f.params.Name
}

type formWidgetCheckbox struct {
	params checkboxInput
	state formWidgetFieldState[bool]
}

elem (f formWidgetCheckbox) Main() {
}

func newFormWidgetFile(ctx context.Context, parent formWidgetGroupState, params fileInput) (formWidgetFile, error) {
	state, err := parent.newField[[]string](ctx, params.Name, nil, "")
	if err != nil {
		return formWidgetFile{}, err
	}
	return formWidgetFile{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetFile) name() string {
	return f.params.Name
}

type formWidgetFile struct {
	params fileInput
	state formWidgetFieldState[[]string]
}

elem (f formWidgetFile) Main() {
}

func newFormWidgetList(ctx context.Context, parent formWidgetGroupState, params fieldList) (formWidgetList, error) {
	state, err := parent.newField[[]any](ctx, params.Name, nil, "")
	if err != nil {
		return formWidgetList{}, err
	}
	return formWidgetList{
		params: params,
		state: state,
	}, nil
}

func (f formWidgetList) name() string {
	return f.params.Name
}

type formWidgetList struct {
	params fieldList
	state formWidgetFieldState[[]any]
}

elem (f formWidgetList) Main() {
}

func newFormWidgetSeries(ctx context.Context, parent formGroupController, params seriesInput) (*formWidgetSeries, error) {
	group, err := parent.newGroup(params.Name)
	if err != nil {
		return nil, err
	}
	/*
	fields := make([]formWidgetSeriesField, 0)
	count := 0
	var prevValue any = nil
	for _, call := range params.Fields {
		seriesField, err := call.Call(ctx, prevValue)
		if err != nil {
			return nil, err
		}
		field, err := newFormWidgetField(ctx, ctrl, seriesField.v)
		if err != nil {
			return nil, err
		}
		fields = append(fields, formWidgetSeriesField{
			slot: new(doors.Door),
			field: field,
		})
		value, err := ctrl.get(field.name())
		if err != nil || value == nil {
			break
		}
		prevValue = value
		count += 1
	}
	errMsg := ""
	if count != len(params.Fields) {
		errMsg = errRequired.Error()
	}
	if err != nil {
		panic(err)
	} 
	f := &formWidgetSeries{
		group: group,
		params: params,
	}
	system, _ := group.newField("", f)
	f.systemField = system
	errMsg := ""
	if f.params.Required {
		errMsg = errRequired.Error()
	}
	system.update(ctx, nil, errMsg)
	return f, nil
}

func (f *formWidgetSeries) name() string {
	return f.params.Name
}

type formWidgetSeries struct {
	mu sync.Mutex
	params seriesInput
	systemField formFieldController
	group *formGroupController
	fields []formWidgetSeriesField
	slot *doors.Door
}

func (f *formWidgetSeries) initialize() {
	if len(f.fields) != 0 {
		return
	}
}

func (f *formWidgetSeries) on(ctx context.Context, name string, value any, error string) {
	if name == "" {
		f.initialize()
		return
	}
}

elem (f *formWidgetSeries) Main() {
	~~
	~~
}

type formWidgetSeriesField struct {
	slot *doors.Door
	field formWidgetField
} */
