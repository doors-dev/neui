// Managed by GoX v0.4.0

//line form_textarea.gox:1
package form

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newTextareaWidget(ctx context.Context, group group, params textareaParams) (*textareaWidget, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &textareaWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type textareaWidget struct {
	id string
	field groupField
	params textareaParams
	count doors.Source[int]
	invalid doors.Setter
	label doors.Setter
}

func (f *textareaWidget) name() string {
	return f.params.Name
}

func (f *textareaWidget) init(ctx context.Context, value any) error {
	str := f.params.Value
	if value != nil {
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a string", f.params.Name, value)
		}
		str = v
	}
	f.count = doors.NewSource(utf8.RuneCountInString(str))
	f.setLocked(ctx, str, f.validate(ctx, str))
	return nil
}

func (f *textareaWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *textareaWidget) update(ctx context.Context, str string) {
	err := f.validate(ctx, str)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, str, err)
	f.count.Update(ctx, utf8.RuneCountInString(str))
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
}

func (f *textareaWidget) setLocked(ctx context.Context, str string, err error) {
	if str == "" {
		f.field.set(ctx, nil, err)
		return
	}
	f.field.set(ctx, str, err)
}

//line form_textarea.gox:76
func (f *textareaWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_textarea.gox:78
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
	errLine := f.field.error.RouteBeam(
		doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_textarea.gox:90
					__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_textarea.gox:90
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:90
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:90
			return })
		})),
	)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_textarea.gox:94
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:95
			if label == nil {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_textarea.gox:96
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_textarea.gox:96
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:97
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_textarea.gox:98
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_textarea.gox:101
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_textarea.gox:101
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:101
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_textarea.gox:105
			__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line form_textarea.gox:106
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_textarea.gox:107
					__e = __c.Modify(&f.label); if __e != nil { return }
//line form_textarea.gox:108
					__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_textarea.gox:116
					__e = __c.Set("class", "neisland-form-textarea"); if __e != nil { return }
//line form_textarea.gox:117
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_textarea.gox:118
					__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_textarea.gox:119
					__e = __c.Set("rows", func() any {
				if f.params.Rows > 0 {
					return f.params.Rows
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:125
					__e = __c.Set("maxlength", func() any {
				if f.params.MaxLength > 0 {
					return f.params.MaxLength
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:131
					__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_textarea.gox:132
					__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_textarea.gox:133
					__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_textarea.gox:134
					__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_textarea.gox:135
					__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_textarea.gox:136
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:136
					__e = __c.Any(value); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line form_textarea.gox:137
			if f.params.MinLength > 0 || f.params.MaxLength > 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_textarea.gox:138
					__e = __c.Set("class", "neisland-form-foot"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:139
					__e = __c.Any(errLine); if __e != nil { return }
//line form_textarea.gox:140
					__e = __c.Any(f.count.Bind(func(n int) gox.Elem {
					if f.params.MinLength > 0 && n < f.params.MinLength {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:142
								__e = __c.Set("class", "neisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:142
								__e = __c.Many(n, " / ", f.params.MinLength, " min"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:142
						return })
					}
					if f.params.MaxLength > 0 {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:145
								__e = __c.Set("class", "neisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:145
								__e = __c.Many(n, " / ", f.params.MaxLength); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:145
						return })
					}
					return nil
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line form_textarea.gox:151
				__e = __c.Any(errLine); if __e != nil { return }
			}
//line form_textarea.gox:153
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_textarea.gox:155
}

func (f *textareaWidget) validate(ctx context.Context, str string) error {
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
