// Managed by GoX v0.3.4

//line form_textarea.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func newFormWidgetTextArea(ctx context.Context, group formGroup, params textArea) (*formWidgetTextArea, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &formWidgetTextArea{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type formWidgetTextArea struct {
	id string
	field formGroupField
	params textArea
	value string
	count doors.Source[int]
	invalid doors.Setter
}

func (f *formWidgetTextArea) name() string {
	return f.params.Name
}

func (f *formWidgetTextArea) init(ctx context.Context, value any) error {
	str := f.params.Value
	if value != nil {
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a string", f.params.Name, value)
		}
		str = v
	}
	f.value = str
	f.count = doors.NewSource(utf8.RuneCountInString(str))
	f.setLocked(ctx, str, f.validate(ctx, str))
	return nil
}

func (f *formWidgetTextArea) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *formWidgetTextArea) update(ctx context.Context, str string) {
	err := f.validate(ctx, str)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, str, err)
	f.count.Update(ctx, utf8.RuneCountInString(str))
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(err))))
}

func (f *formWidgetTextArea) setLocked(ctx context.Context, str string, err error) {
	if str == "" {
		f.field.set(ctx, nil, err)
		return
	}
	f.field.set(ctx, str, err)
}

//line form_textarea.gox:75
func (f *formWidgetTextArea) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_textarea.gox:77
		errLine := f.field.error.RouteBeam(
		doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_textarea.gox:79
					__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_textarea.gox:79
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:79
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:79
			return })
		})),
	)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_textarea.gox:83
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_textarea.gox:84
				__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_textarea.gox:84
				__e = __c.Set("for", f.id); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:85
				__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_textarea.gox:86
				__e = __c.Any(f.field.error.RouteBeam(
				doors.RouteBeamMatch(func(err error) bool {
					return f.params.Required && errors.Is(err, errRequired)
				}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_textarea.gox:89
						__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_textarea.gox:89
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("*"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:89
				return }))),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("textarea"); if __e != nil { return }
			{
//line form_textarea.gox:93
				__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_textarea.gox:94
				__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeRate{Tick: 300 * time.Millisecond}),
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_textarea.gox:101
				__e = __c.Set("class", "kisland-form-textarea"); if __e != nil { return }
//line form_textarea.gox:102
				__e = __c.Set("id", f.id); if __e != nil { return }
//line form_textarea.gox:103
				__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_textarea.gox:104
				__e = __c.Set("rows", func() any {
				if f.params.Rows > 0 {
					return f.params.Rows
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:110
				__e = __c.Set("maxlength", func() any {
				if f.params.MaxLength > 0 {
					return f.params.MaxLength
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:116
				__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_textarea.gox:117
				__e = __c.Set("aria-required", formAria(f.params.Required)); if __e != nil { return }
//line form_textarea.gox:118
				__e = __c.Set("aria-invalid", formAria(formShownErr(f.field.error.Get()))); if __e != nil { return }
//line form_textarea.gox:119
				__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:119
				__e = __c.Any(f.value); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:120
			if f.params.MinLength > 0 || f.params.MaxLength > 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_textarea.gox:121
					__e = __c.Set("class", "kisland-form-foot"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:122
					__e = __c.Any(errLine); if __e != nil { return }
//line form_textarea.gox:123
					__e = __c.Any(f.count.Bind(func(n int) gox.Elem {
					if f.params.MinLength > 0 && n < f.params.MinLength {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:125
								__e = __c.Set("class", "kisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:125
								__e = __c.Many(n, " / ", f.params.MinLength, " min"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:125
						return })
					}
					if f.params.MaxLength > 0 {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:128
								__e = __c.Set("class", "kisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:128
								__e = __c.Many(n, " / ", f.params.MaxLength); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:128
						return })
					}
					return nil
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line form_textarea.gox:134
				__e = __c.Any(errLine); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_textarea.gox:137
}

func (f *formWidgetTextArea) validate(ctx context.Context, str string) error {
	if str == "" {
		if f.params.Required {
			return errRequired
		}
		return nil
	}
	length := utf8.RuneCountInString(str)
	if f.params.MinLength > 0 && length < f.params.MinLength {
		return fmt.Errorf("use at least %d characters", f.params.MinLength)
	}
	if f.params.MaxLength > 0 && length > f.params.MaxLength {
		return fmt.Errorf("use at most %d characters", f.params.MaxLength)
	}
	if f.params.Validate != "" {
		msg, err := f.params.Validate.Call(ctx, str)
		if err != nil {
			return errSystem
		}
		if msg != "" {
			return errors.New(msg)
		}
	}
	return nil
}
