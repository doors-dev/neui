// Managed by GoX v0.4.0

//line form_textarea.gox:1
// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

//line form_textarea.gox:90
func (f *textareaWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_textarea.gox:92
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
//line form_textarea.gox:104
					__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_textarea.gox:104
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:104
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:104
			return })
		})),
	)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_textarea.gox:108
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:109
			if label == nil {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_textarea.gox:110
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_textarea.gox:110
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:111
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_textarea.gox:112
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_textarea.gox:115
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_textarea.gox:115
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:115
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_textarea.gox:119
			__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line form_textarea.gox:120
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_textarea.gox:121
					__e = __c.Modify(&f.label); if __e != nil { return }
//line form_textarea.gox:122
					__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_textarea.gox:130
					__e = __c.Set("class", "neisland-form-textarea"); if __e != nil { return }
//line form_textarea.gox:131
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_textarea.gox:132
					__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_textarea.gox:133
					__e = __c.Set("rows", func() any {
				if f.params.Rows > 0 {
					return f.params.Rows
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:139
					__e = __c.Set("maxlength", func() any {
				if f.params.MaxLength > 0 {
					return f.params.MaxLength
				}
				return nil
			}()); if __e != nil { return }
//line form_textarea.gox:145
					__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_textarea.gox:146
					__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_textarea.gox:147
					__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_textarea.gox:148
					__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_textarea.gox:149
					__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_textarea.gox:150
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:150
					__e = __c.Any(value); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line form_textarea.gox:151
			if f.params.MinLength > 0 || f.params.MaxLength > 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_textarea.gox:152
					__e = __c.Set("class", "neisland-form-foot"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:153
					__e = __c.Any(errLine); if __e != nil { return }
//line form_textarea.gox:154
					__e = __c.Any(f.count.Bind(func(n int) gox.Elem {
					if f.params.MinLength > 0 && n < f.params.MinLength {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:156
								__e = __c.Set("class", "neisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:156
								__e = __c.Many(n, " / ", f.params.MinLength, " min"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:156
						return })
					}
					if f.params.MaxLength > 0 {
						return gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_textarea.gox:159
								__e = __c.Set("class", "neisland-form-count"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_textarea.gox:159
								__e = __c.Many(n, " / ", f.params.MaxLength); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_textarea.gox:159
						return })
					}
					return nil
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line form_textarea.gox:165
				__e = __c.Any(errLine); if __e != nil { return }
			}
//line form_textarea.gox:167
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_textarea.gox:169
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
