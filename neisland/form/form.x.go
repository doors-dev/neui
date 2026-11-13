// Managed by GoX v0.3.6-0.20261113082228-02db5fed593a

//line form.gox:1
package form

import (
	"context"
	"errors"
	"fmt"
	"slices"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
)

// New builds a form widget for Library.Control: the user fills the
// fields and presses one of the submit buttons, which sends
// {"data": values, "submit": button value} to the script waiting in
// ui.draw(). Files upload into the island dir as they are picked, each under
// <field id>/. The cancel button sends {"canceled": true}.
func New(ctx context.Context, p Params, submit func(ctx context.Context, s map[string]any) bool) (api.Widget[struct{}], error) {
	if len(p.Submits) == 0 {
		p.Submits = []submitParams{{Text: "Submit"}}
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
	}
	group := &groupBase{
		root: root,
		collector: defaultCollector{},
		errors: doors.NewSource(&m),
		reserved: make(map[string]struct{}),
		values: root.values,
	}
	fields := make([]fieldWidget, 0, len(p.Fields))
	for _, field := range p.Fields {
		widget, err := newFieldWidget(ctx, group, field.v)
		if err != nil {
			return nil, err
		}
		if err := widget.init(ctx, nil); err != nil {
			return nil, err
		}
		fields = append(fields, widget)
	}
	return &widget{
		id: doors.IDRand(),
		params: p,
		root: root,
		group: group,
		fields: fields,
		errors: doors.DeriveBeam(group.errors, func(m *map[string]any) errorKind {
			return errorsOf(*m)
		}),
		submit: submit,
	}, nil
}

func newFieldWidget(ctx context.Context, group group, field anyParams) (fieldWidget, error) {
	switch field := field.(type) {
	case *textParams:
		return newTextWidget(ctx, group, *field)
	case *numberParams:
		return newNumberWidget(ctx, group, *field)
	case *dateParams:
		return newDateWidget(ctx, group, *field)
	case *textareaParams:
		return newTextareaWidget(ctx, group, *field)
	case *selectParams:
		return newSelectWidget(ctx, group, *field)
	case *searchParams:
		return newSearchWidget(ctx, group, *field)
	case *checkboxParams:
		return newCheckboxWidget(ctx, group, *field)
	case *fileParams:
		return newFileWidget(ctx, group, *field)
	case *groupParams:
		return newGroupWidget(ctx, group, *field)
	case *listParams:
		return newListWidget(ctx, group, *field)
	case *seriesParams:
		return newSeriesWidget(ctx, group, *field)
	default:
		return nil, fmt.Errorf("unknown field %T", field)
	}
}

type widget struct {
	id string
	params Params
	root *root
	group *groupBase
	fields []fieldWidget
	errors doors.Beam[errorKind]
	locked doors.Setter
	submit func(ctx context.Context, s map[string]any) bool
}

func (f *widget) Update(context.Context, struct{}) {
}

func (f *widget) onSubmit(ctx context.Context, r doors.RequestRawForm) {
	parsed, err := r.ParseForm(64 << 10)
	if err != nil {
		neui.IslandReport(ctx, fmt.Errorf("form submit: %w", err))
		return
	}
	choice := f.params.Submits[0].Value
	if value := parsed.FormValue(f.id); slices.ContainsFunc(f.params.Submits, func(s submitParams) bool {
		return s.Value == value
	}) {
		choice = value
	}
	f.root.mu.Lock()
	if errorsOf(*f.group.errors.Get()) != errorsNone {
		f.root.mu.Unlock()
		return
	}
	data := f.group.collect()
	f.root.mu.Unlock()
	f.submit(ctx, map[string]any{"data": data, "submit": choice})
}

//line form.gox:134
func (f *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form.gox:136
		scope := f.root.scope.Scope(true).And(neui.IslandScope(ctx))
	errs, _ := f.errors.ReadAndSub(ctx, func(ctx context.Context, errs errorKind) bool {
		doors.Call(ctx, f.locked.Set("disabled", errs != errorsNone))
		return false
	})

		__e = __c.Init("form"); if __e != nil { return }
		{
//line form.gox:143
			__e = __c.Modify(doors.ARawSubmit{
			Scope: scope,
			On: func(ctx context.Context, r doors.RequestRawForm) bool {
				f.onSubmit(ctx, r)
				return false
			},
		}); if __e != nil { return }
//line form.gox:150
			__e = __c.Set("class", "neisland-form neisland-form-framed"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form.gox:151
			for _, field := range f.fields {
//line form.gox:152
				__e = __c.Any(field); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form.gox:154
				__e = __c.Set("class", "neisland-form-actions neisland-form-actions-band"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form.gox:155
				for _, s := range f.params.Submits {
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form.gox:157
						__e = __c.Modify(&f.locked); if __e != nil { return }
//line form.gox:158
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line form.gox:159
						__e = __c.Set("class", "neisland-button neisland-button-md neisland-button-" + s.Style); if __e != nil { return }
//line form.gox:160
						__e = __c.Set("data-submit", s.Value); if __e != nil { return }
//line form.gox:161
						__e = __c.Set("disabled", errs != errorsNone); if __e != nil { return }
//line form.gox:162
						__e = __c.Set("aria-describedby", f.id + "-s"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form.gox:163
						__e = __c.Any(s.Text); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line form.gox:166
				if f.params.Cancel != "" {
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form.gox:168
						__e = __c.Modify(doors.AClick{
						Scope: scope,
						On: func(ctx context.Context, r doors.RequestPointer) bool {
							f.submit(ctx, map[string]any{"canceled": true})
							return false
						},
					}); if __e != nil { return }
//line form.gox:175
						__e = __c.Set("type", "button"); if __e != nil { return }
//line form.gox:176
						__e = __c.Set("class", "neisland-form-cancel"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form.gox:177
						__e = __c.Any(f.params.Cancel); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line form.gox:180
				__e = __c.Any(f.errors.Bind(func(errs errorKind) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form.gox:181
					if errs != errorsNone {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form.gox:182
							__e = __c.Set("class", "neisland-form-help"); if __e != nil { return }
//line form.gox:182
							__e = __c.Set("id", f.id + "-s"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("form is not completed or contains errors"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				return })
//line form.gox:184
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form.gox:186
				__e = __c.Set("type", "hidden"); if __e != nil { return }
//line form.gox:186
				__e = __c.Set("name", f.id); if __e != nil { return }
//line form.gox:186
				__e = __c.Set("value", f.params.Submits[0].Value); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
//line form.gox:187
				__e = __c.Set("data:name", f.id); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const form = document.currentScript.parentElement\n\t\t\tconst choice = form.querySelector(\n\t\t\t\t`input[type=\"hidden\"][name=\"${$data(\"name\")}\"]`,\n\t\t\t)\n\t\t\tform.addEventListener(\"click\", (event) => {\n\t\t\t\tconst button = event.target.closest(\"button[type=submit][data-submit]\")\n\t\t\t\tif (button && button.form === form) {\n\t\t\t\t\tchoice.value = button.dataset.submit\n\t\t\t\t}\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form.gox:200
}

func shownErr(err error) bool {
	return err != nil && !errors.Is(err, errRequired) && !errors.Is(err, errUploading)
}

func aria(b bool) any {
	if b {
		return "true"
	}
	return nil
}

type fieldWidget interface {
	gox.Comp
	init(ctx context.Context, value any) error
	destroy(ctx context.Context)
	name() string
}
