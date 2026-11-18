// Managed by GoX v0.4.0

//line form_checkbox.gox:1
package form

import (
	"context"
	"errors"
	"fmt"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newCheckboxWidget(ctx context.Context, group group, params checkboxParams) (*checkboxWidget, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &checkboxWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type checkboxWidget struct {
	id string
	field groupField
	params checkboxParams
	invalid doors.Setter
	label doors.Setter
}

func (f *checkboxWidget) name() string {
	return f.params.Name
}

func (f *checkboxWidget) init(ctx context.Context, value any) error {
	checked := f.params.Checked
	if value != nil {
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a boolean", f.params.Name, value)
		}
		checked = v
	}
	f.setLocked(ctx, checked, f.validate(ctx, checked))
	return nil
}

func (f *checkboxWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *checkboxWidget) update(ctx context.Context, checked bool) {
	err := f.validate(ctx, checked)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, checked, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
}

func (f *checkboxWidget) setLocked(ctx context.Context, checked bool, err error) {
	f.field.set(ctx, checked, err)
}

//line form_checkbox.gox:67
func (f *checkboxWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_checkbox.gox:69
		f.field.mutex().Lock()
	checked, _ := f.field.value().(bool)
	f.field.mutex().Unlock()
	label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_checkbox.gox:80
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_checkbox.gox:81
				__e = __c.Set("class", "neisland-form-check neisland-form-check-single"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_checkbox.gox:82
				__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line form_checkbox.gox:83
						__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_checkbox.gox:84
						__e = __c.Modify(&f.label); if __e != nil { return }
//line form_checkbox.gox:85
						__e = __c.Modify(doors.AChange{
					Scope: f.field.scope(),
					On: func(ctx context.Context, r doors.RequestChange) bool {
						f.update(ctx, r.Event().Checked)
						return false
					},
				}); if __e != nil { return }
//line form_checkbox.gox:92
						__e = __c.Set("class", "neisland-form-checkbox"); if __e != nil { return }
//line form_checkbox.gox:93
						__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line form_checkbox.gox:94
						__e = __c.Set("id", f.id); if __e != nil { return }
//line form_checkbox.gox:95
						__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_checkbox.gox:96
						__e = __c.Set("checked", checked); if __e != nil { return }
//line form_checkbox.gox:97
						__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_checkbox.gox:98
						__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_checkbox.gox:99
						__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_checkbox.gox:100
						__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_checkbox.gox:101
						__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
//line form_checkbox.gox:102
				if label == nil {
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line form_checkbox.gox:104
						__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_checkbox.gox:105
						__e = __c.Any(f.field.error.RouteBeam(
						doors.RouteBeamMatch(func(err error) bool {
							return f.params.Required && err != nil
						}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_checkbox.gox:108
								__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_checkbox.gox:108
								__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("*"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:108
						return }))),
					)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:113
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_checkbox.gox:115
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_checkbox.gox:115
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_checkbox.gox:115
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_checkbox.gox:115
				return })
			})),
		)); if __e != nil { return }
//line form_checkbox.gox:118
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_checkbox.gox:120
}

func (f *checkboxWidget) validate(ctx context.Context, checked bool) error {
	if !checked && f.params.Required {
		return errRequired
	}
	return nil
}
