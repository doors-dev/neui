// Managed by GoX v0.3.4

//line form_text.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newFormWidgetText(ctx context.Context, group formGroup, params textInput) (*formWidgetText, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &formWidgetText{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type formWidgetText struct {
	id string
	field formGroupField
	params textInput
	value string
	invalid doors.Setter
}

func (f *formWidgetText) name() string {
	return f.params.Name
}

func (f *formWidgetText) init(ctx context.Context, value any) error {
	str := f.params.Value
	if value != nil {
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a string", f.params.Name, value)
		}
		str = v
	}
	f.value = str
	f.setLocked(ctx, str, f.validate(ctx, str))
	return nil
}

func (f *formWidgetText) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *formWidgetText) update(ctx context.Context, str string) {
	err := f.validate(ctx, str)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, str, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(err))))
}

func (f *formWidgetText) setLocked(ctx context.Context, str string, err error) {
	value := (*string)(nil)
	if str != "" {
		value = &str
	}
	f.field.set(ctx, value, err)
}

//line form_text.gox:76
func (f *formWidgetText) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_text.gox:77
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_text.gox:78
				__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_text.gox:78
				__e = __c.Set("for", f.id); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:79
				__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_text.gox:80
				__e = __c.Any(f.field.error.RouteBeam(
				doors.RouteBeamMatch(func(err error) bool {
					return f.params.Required && errors.Is(err, errRequired)
				}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_text.gox:83
						__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_text.gox:83
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("*"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_text.gox:83
				return }))),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form_text.gox:87
				__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_text.gox:88
				__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeRate{Tick: 300 * time.Millisecond}),
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_text.gox:95
				__e = __c.Set("class", "kisland-form-input"); if __e != nil { return }
//line form_text.gox:96
				__e = __c.Set("id", f.id); if __e != nil { return }
//line form_text.gox:97
				__e = __c.Set("type", f.params.Type); if __e != nil { return }
//line form_text.gox:98
				__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_text.gox:99
				__e = __c.Set("value", f.value); if __e != nil { return }
//line form_text.gox:100
				__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_text.gox:101
				__e = __c.Set("aria-required", formAria(f.params.Required)); if __e != nil { return }
//line form_text.gox:102
				__e = __c.Set("aria-invalid", formAria(formShownErr(f.field.error.Get()))); if __e != nil { return }
//line form_text.gox:103
				__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:104
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_text.gox:106
						__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_text.gox:106
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:106
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_text.gox:106
				return })
			})),
		)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_text.gox:110
}

func (f *formWidgetText) validate(ctx context.Context, str string) error {
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
	switch f.params.Type {
	case "email":
		if addr, err := mail.ParseAddress(str); err != nil || addr.Address != str {
			return errors.New("enter an email address")
		}
	case "url":
		if u, err := url.ParseRequestURI(str); err != nil || u.Scheme == "" || u.Host == "" {
			return errors.New("enter a full URL, like https://example.com")
		}
	}
	if f.params.Pattern != "" {
		re, err := regexp.Compile("^(?:" + f.params.Pattern + ")$")
		if err != nil {
			neui.IslandReport(ctx, fmt.Errorf("field %q pattern: %w", f.params.Name, err))
		} else if !re.MatchString(str) {
			return errors.New("use the requested format")
		}
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
