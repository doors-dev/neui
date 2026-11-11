// Managed by GoX v0.3.4

//line form_checkbox.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func newFormWidgetCheckbox(ctx context.Context, group formGroup, params checkboxInput) (*formWidgetCheckbox, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &formWidgetCheckbox{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type formWidgetCheckbox struct {
	id string
	field formGroupField
	params checkboxInput
	value bool
	invalid doors.Setter
}

func (f *formWidgetCheckbox) name() string {
	return f.params.Name
}

func (f *formWidgetCheckbox) init(ctx context.Context, value any) error {
	checked := f.params.Checked
	if value != nil {
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a boolean", f.params.Name, value)
		}
		checked = v
	}
	f.value = checked
	f.setLocked(ctx, checked, f.validate(ctx, checked))
	return nil
}

func (f *formWidgetCheckbox) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *formWidgetCheckbox) update(ctx context.Context, checked bool) {
	err := f.validate(ctx, checked)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, checked, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(err))))
}

func (f *formWidgetCheckbox) setLocked(ctx context.Context, checked bool, err error) {
	f.field.set(ctx, checked, err)
}

//line form_checkbox.gox:66
func (f *formWidgetCheckbox) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_checkbox.gox:67
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_checkbox.gox:68
				__e = __c.Set("class", "kisland-form-check"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line form_checkbox.gox:70
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_checkbox.gox:71
					__e = __c.Modify(doors.AChange{
					Scope: f.field.scope(),
					On: func(ctx context.Context, r doors.RequestChange) bool {
						f.update(ctx, r.Event().Checked)
						return false
					},
				}); if __e != nil { return }
//line form_checkbox.gox:78
					__e = __c.Set("class", "kisland-form-checkbox"); if __e != nil { return }
//line form_checkbox.gox:79
					__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line form_checkbox.gox:80
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_checkbox.gox:81
					__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_checkbox.gox:82
					__e = __c.Set("checked", f.value); if __e != nil { return }
//line form_checkbox.gox:83
					__e = __c.Set("aria-required", formAria(f.params.Required)); if __e != nil { return }
//line form_checkbox.gox:84
					__e = __c.Set("aria-invalid", formAria(formShownErr(f.field.error.Get()))); if __e != nil { return }
//line form_checkbox.gox:85
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line form_checkbox.gox:87
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_checkbox.gox:88
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && errors.Is(err, errRequired)
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_checkbox.gox:91
							__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_checkbox.gox:91
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:91
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:95
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_checkbox.gox:97
						__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_checkbox.gox:97
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_checkbox.gox:97
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:97
				return })
			})),
		)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_checkbox.gox:101
}

func (f *formWidgetCheckbox) validate(ctx context.Context, checked bool) error {
	if !checked && f.params.Required {
		return errRequired
	}
	return nil
}
