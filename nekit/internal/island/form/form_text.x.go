// Managed by GoX v0.4.0

//line form_text.gox:1
package form

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

func newTextWidget(ctx context.Context, group group, params textParams) (*textWidget, error) {
	var pattern *regexp.Regexp
	if params.Pattern != "" {
		re, err := regexp.Compile("^(?:" + params.Pattern + ")$")
		if err != nil {
			return nil, fmt.Errorf("field %q pattern: %w", params.Name, err)
		}
		pattern = re
	}
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &textWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
		pattern: pattern,
	}, nil
}

type textWidget struct {
	id string
	field groupField
	params textParams
	pattern *regexp.Regexp
	invalid doors.Setter
	label doors.Setter
}

func (f *textWidget) name() string {
	return f.params.Name
}

func (f *textWidget) init(ctx context.Context, value any) error {
	str := f.params.Value
	if value != nil {
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a string", f.params.Name, value)
		}
		str = v
	}
	f.setLocked(ctx, str, f.validate(ctx, str))
	return nil
}

func (f *textWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *textWidget) update(ctx context.Context, str string) {
	err := f.validate(ctx, str)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, str, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
}

func (f *textWidget) setLocked(ctx context.Context, str string, err error) {
	if str == "" {
		f.field.set(ctx, nil, err)
		return
	}
	f.field.set(ctx, str, err)
}

//line form_text.gox:86
func (f *textWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_text.gox:88
		f.field.mutex().Lock()
	value, _ := f.field.value().(string)
	f.field.mutex().Unlock()
	label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}
	rights := neui.RW
	if f.params.Type == "password" {
		rights = neui.W
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_text.gox:103
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:104
			if label == nil {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_text.gox:105
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_text.gox:105
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:106
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_text.gox:107
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_text.gox:110
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_text.gox:110
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_text.gox:110
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_text.gox:114
			__e = (neui.Input{Desc: f.params.Label, Rights: rights}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line form_text.gox:115
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_text.gox:116
					__e = __c.Modify(&f.label); if __e != nil { return }
//line form_text.gox:117
					__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_text.gox:125
					__e = __c.Set("class", "neisland-form-input"); if __e != nil { return }
//line form_text.gox:126
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_text.gox:127
					__e = __c.Set("type", f.params.Type); if __e != nil { return }
//line form_text.gox:128
					__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_text.gox:129
					__e = __c.Set("value", value); if __e != nil { return }
//line form_text.gox:130
					__e = __c.Set("maxlength", func() any {
				if f.params.MaxLength > 0 {
					return f.params.MaxLength
				}
				return nil
			}()); if __e != nil { return }
//line form_text.gox:136
					__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_text.gox:137
					__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_text.gox:138
					__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_text.gox:139
					__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_text.gox:140
					__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_text.gox:141
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
			return })); if __e != nil { return }
//line form_text.gox:142
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_text.gox:144
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_text.gox:144
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_text.gox:144
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_text.gox:144
				return })
			})),
		)); if __e != nil { return }
//line form_text.gox:147
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_text.gox:149
}

func (f *textWidget) validate(ctx context.Context, str string) error {
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
	if f.pattern != nil && !f.pattern.MatchString(str) {
		return errors.New("use the requested format")
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
