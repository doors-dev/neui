// Managed by GoX v0.4.0

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
	"neui.dev/neisland/internal/pending"
	"neui.dev/api"
)

// New builds a form widget for Library.Control: the user fills the
// fields and presses one of the submit buttons, which sends
// {"data": values, "submit": button value} to the script waiting in
// ui.draw(). Files upload into the island dir as they are picked; a file
// field's value is their workspace paths.
func New(ctx context.Context, p Params, submit func(ctx context.Context, s map[string]any) bool) (api.Widget[Patch], error) {
	if len(p.Submits) == 0 {
		p.Submits = []submitParams{{Text: "Submit"}}
	}
	for i := range p.Submits {
		if slices.ContainsFunc(p.Submits[:i], func(s submitParams) bool {
			return s.Text == p.Submits[i].Text && s.Value == p.Submits[i].Value
		}) {
			return nil, fmt.Errorf("submits: two buttons have text %q and value %q", p.Submits[i].Text, p.Submits[i].Value)
		}
		switch {
		case p.Submits[i].Style != "":
		case i == 0:
			p.Submits[i].Style = "primary"
		default:
			p.Submits[i].Style = "secondary"
		}
	}
	for _, s := range p.Submits {
		if s.Novalidate && slices.ContainsFunc(p.Submits, func(o submitParams) bool {
			return !o.Novalidate && o.Value == s.Value
		}) {
			return nil, fmt.Errorf("submit %q: %w", s.Text, errNovalidateValue)
		}
	}
	m := make(map[string]any)
	root := &root{
		values: make(map[string]any),
		scope: new(doors.ScopeCheckpoint),
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

func (f *widget) Update(context.Context, Patch) {
}

func (f *widget) onSubmit(ctx context.Context, r doors.RequestRawForm) {
	parsed, err := r.ParseForm(64 << 10)
	if err != nil {
		neui.IslandReport(ctx, fmt.Errorf("form submit: %w", err))
		return
	}
	choice := f.params.Submits[0]
	value := parsed.FormValue(f.id)
	if i := slices.IndexFunc(f.params.Submits, func(s submitParams) bool {
		return s.Value == value
	}); i >= 0 {
		choice = f.params.Submits[i]
	}
	f.root.mu.Lock()
	if !choice.Novalidate && errorsOf(*f.group.errors.Get()) != errorsNone {
		f.root.mu.Unlock()
		return
	}
	data := f.group.collect()
	f.root.mu.Unlock()
	f.submit(ctx, map[string]any{"data": data, "submit": choice.Value})
}

//line form.gox:148
func (f *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form.gox:150
		scope := doors.JoinScopes(neui.IslandScope(ctx), f.root.scope.Scope(true))
	errs, _ := f.errors.ReadAndSub(ctx, func(ctx context.Context, errs errorKind) bool {
		doors.Call(ctx, f.locked.Set("disabled", errs != errorsNone))
		return false
	})

		__e = __c.Init("form"); if __e != nil { return }
		{
//line form.gox:157
			__e = __c.Modify(doors.ARawSubmit{
			Scope: scope,
			Indicator: pending.FormIndicator(ctx),
			On: func(ctx context.Context, r doors.RequestRawForm) bool {
				f.onSubmit(ctx, r)
				return false
			},
		}); if __e != nil { return }
//line form.gox:165
			__e = __c.Set("class", "neisland-form neisland-form-framed"); if __e != nil { return }
			__e = __c.Set("novalidate", true); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form.gox:167
			for _, field := range f.fields {
//line form.gox:168
				__e = __c.Any(field); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form.gox:170
				__e = __c.Set("class", "neisland-form-actions neisland-form-actions-band"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form.gox:171
				for _, s := range f.params.Submits {
//line form.gox:172
					if s.Novalidate {
//line form.gox:173
						__e = (neui.Input{Desc: s.Text}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line form.gox:174
								__e = __c.Set("type", "submit"); if __e != nil { return }
//line form.gox:175
								__e = __c.Set("class", "neisland-button neisland-button-md neisland-button-" + s.Style); if __e != nil { return }
//line form.gox:176
								__e = __c.Set("name", f.id); if __e != nil { return }
//line form.gox:177
								__e = __c.Set("value", s.Value); if __e != nil { return }
//line form.gox:178
								__e = __c.Set("data-neui", "submit"); if __e != nil { return }
								__e = __c.Set("formnovalidate", true); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form.gox:180
								__e = __c.Any(s.Text); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					} else  {
//line form.gox:183
						__e = (neui.Input{Desc: s.Text}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line form.gox:184
								__e = __c.Modify(&f.locked); if __e != nil { return }
//line form.gox:185
								__e = __c.Set("type", "submit"); if __e != nil { return }
//line form.gox:186
								__e = __c.Set("class", "neisland-button neisland-button-md neisland-button-" + s.Style); if __e != nil { return }
//line form.gox:187
								__e = __c.Set("name", f.id); if __e != nil { return }
//line form.gox:188
								__e = __c.Set("value", s.Value); if __e != nil { return }
//line form.gox:189
								__e = __c.Set("data-neui", "submit"); if __e != nil { return }
//line form.gox:190
								__e = __c.Set("disabled", errs != errorsNone); if __e != nil { return }
//line form.gox:191
								__e = __c.Set("aria-describedby", f.id + "-s"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form.gox:192
								__e = __c.Any(s.Text); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
				}
//line form.gox:196
				__e = __c.Any(f.errors.Bind(func(errs errorKind) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form.gox:197
					if errs != errorsNone {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form.gox:198
							__e = __c.Set("class", "neisland-form-help"); if __e != nil { return }
//line form.gox:198
							__e = __c.Set("id", f.id + "-s"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("form is not completed or contains errors"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form.gox:199
						__e = __c.Any(neui.Data{Text: "submit blocked", Object: "form is not completed or contains errors"}); if __e != nil { return }
					}
				return })
//line form.gox:201
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form.gox:204
}

func shownErr(err error) bool {
	return err != nil && !errors.Is(err, errRequired) && !errors.Is(err, errUploading)
}

//line form.gox:210
func errorData(name string, err doors.Beam[error]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form.gox:211
		__e = __c.Any(err.Bind(func(err error) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line form.gox:212
			if err != nil {
//line form.gox:213
				__e = __c.Any(neui.Data{Text: name + " validation error", Object: err.Error()}); if __e != nil { return }
			}
		return })
//line form.gox:215
	})); if __e != nil { return }
	return })
//line form.gox:216
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
