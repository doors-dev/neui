// Managed by GoX v0.4.0

//line form_number.gox:1
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
	"math"
	"strconv"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newNumberWidget(ctx context.Context, group group, params numberParams) (*numberWidget, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &numberWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type numberWidget struct {
	id string
	field groupField
	params numberParams
	invalid doors.Setter
	label doors.Setter
	output doors.Door
}

func (f *numberWidget) name() string {
	return f.params.Name
}

func (f *numberWidget) init(ctx context.Context, value any) error {
	number := f.params.Value
	if value != nil {
		v, ok := value.(float64)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want a number", f.params.Name, value)
		}
		number = &v
	}
	if number == nil && f.params.Type == "range" {
		lo, hi := 0.0, 100.0
		if f.params.Min != nil {
			lo = *f.params.Min
		}
		if f.params.Max != nil {
			hi = *f.params.Max
		}
		step := 1.0
		if f.params.Step != nil {
			step = *f.params.Step
		}
		v := lo
		if hi > lo {
			v = lo + (hi - lo) / 2
			v = lo + math.Round((v - lo) / step) * step
			for v > hi {
				v -= step
			}
		}
		number = &v
	}
	f.setLocked(ctx, number, f.validate(ctx, number))
	return nil
}

func (f *numberWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *numberWidget) update(ctx context.Context, str string) {
	number := (*float64)(nil)
	err := error(nil)
	if str != "" {
		v, parseErr := strconv.ParseFloat(str, 64)
		if parseErr != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			err = errors.New("enter a number")
		} else {
			number = &v
		}
	}
	if err == nil {
		err = f.validate(ctx, number)
	}
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, number, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
	if f.params.Type == "range" {
		f.output.Inner(ctx, str)
	}
}

func (f *numberWidget) setLocked(ctx context.Context, number *float64, err error) {
	if number == nil {
		f.field.set(ctx, nil, err)
		return
	}
	f.field.set(ctx, *number, err)
}

//line form_number.gox:126
func (f *numberWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_number.gox:128
		f.field.mutex().Lock()
	value := ""
	if number, ok := f.field.value().(float64); ok {
		value = strconv.FormatFloat(number, 'f', -1, 64)
	}
	f.field.mutex().Unlock()
	label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}
	min, max, step := any(nil), any(nil), any(nil)
	if f.params.Min != nil {
		min = strconv.FormatFloat(*f.params.Min, 'f', -1, 64)
	}
	if f.params.Max != nil {
		max = strconv.FormatFloat(*f.params.Max, 'f', -1, 64)
	}
	if f.params.Step != nil {
		step = strconv.FormatFloat(*f.params.Step, 'f', -1, 64)
	} else if f.params.Type == "number" {
		step = "any"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_number.gox:154
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_number.gox:155
			if label == nil {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_number.gox:156
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_number.gox:156
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_number.gox:157
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_number.gox:158
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_number.gox:161
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_number.gox:161
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_number.gox:161
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_number.gox:165
			if f.params.Type == "range" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_number.gox:166
					__e = __c.Set("class", "neisland-form-slider"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_number.gox:167
					__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitVoid("input"); if __e != nil { return }
						{
//line form_number.gox:168
							__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_number.gox:169
							__e = __c.Modify(&f.label); if __e != nil { return }
//line form_number.gox:170
							__e = __c.Modify(doors.AInput{
						Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
						Batch: true,
						On: func(ctx context.Context, r doors.RequestInput) bool {
							f.update(ctx, r.Event().Value)
							return false
						},
					}); if __e != nil { return }
//line form_number.gox:178
							__e = __c.Set("class", "neisland-form-range"); if __e != nil { return }
//line form_number.gox:179
							__e = __c.Set("id", f.id); if __e != nil { return }
//line form_number.gox:180
							__e = __c.Set("type", "range"); if __e != nil { return }
//line form_number.gox:181
							__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_number.gox:182
							__e = __c.Set("value", value); if __e != nil { return }
//line form_number.gox:183
							__e = __c.Set("min", min); if __e != nil { return }
//line form_number.gox:184
							__e = __c.Set("max", max); if __e != nil { return }
//line form_number.gox:185
							__e = __c.Set("step", step); if __e != nil { return }
//line form_number.gox:186
							__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_number.gox:187
							__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_number.gox:188
							__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_number.gox:189
							__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_number.gox:190
							__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
						}
						__e = __c.Submit(); if __e != nil { return }
					return })); if __e != nil { return }
//line form_number.gox:191
					__e = (&f.output).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("output"); if __e != nil { return }
						{
//line form_number.gox:191
							__e = __c.Set("class", "neisland-form-output"); if __e != nil { return }
//line form_number.gox:191
							__e = __c.Set("for", f.id); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_number.gox:191
							__e = __c.Any(value); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line form_number.gox:194
				__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line form_number.gox:195
						__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_number.gox:196
						__e = __c.Modify(&f.label); if __e != nil { return }
//line form_number.gox:197
						__e = __c.Modify(doors.AInput{
					Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
					Batch: true,
					On: func(ctx context.Context, r doors.RequestInput) bool {
						f.update(ctx, r.Event().Value)
						return false
					},
				}); if __e != nil { return }
//line form_number.gox:205
						__e = __c.Set("class", "neisland-form-input"); if __e != nil { return }
//line form_number.gox:206
						__e = __c.Set("id", f.id); if __e != nil { return }
//line form_number.gox:207
						__e = __c.Set("type", "number"); if __e != nil { return }
//line form_number.gox:208
						__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_number.gox:209
						__e = __c.Set("value", value); if __e != nil { return }
//line form_number.gox:210
						__e = __c.Set("min", min); if __e != nil { return }
//line form_number.gox:211
						__e = __c.Set("max", max); if __e != nil { return }
//line form_number.gox:212
						__e = __c.Set("step", step); if __e != nil { return }
//line form_number.gox:213
						__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_number.gox:214
						__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_number.gox:215
						__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_number.gox:216
						__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_number.gox:217
						__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
			}
//line form_number.gox:219
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_number.gox:221
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_number.gox:221
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_number.gox:221
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_number.gox:221
				return })
			})),
		)); if __e != nil { return }
//line form_number.gox:224
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_number.gox:226
}

func (f *numberWidget) validate(ctx context.Context, number *float64) error {
	if number == nil {
		if f.params.Required {
			return errRequired
		}
		return nil
	}
	if f.params.Min != nil && *number < *f.params.Min {
		return fmt.Errorf("enter %s or more", strconv.FormatFloat(*f.params.Min, 'f', -1, 64))
	}
	if f.params.Max != nil && *number > *f.params.Max {
		return fmt.Errorf("enter %s or less", strconv.FormatFloat(*f.params.Max, 'f', -1, 64))
	}
	if f.params.Step != nil && *f.params.Step > 0 {
		base := 0.0
		if f.params.Min != nil {
			base = *f.params.Min
		}
		steps := (*number - base) / *f.params.Step
		if math.Abs(steps - math.Round(steps)) > 1e-9 {
			return fmt.Errorf("use steps of %s", strconv.FormatFloat(*f.params.Step, 'f', -1, 64))
		}
	}
	if f.params.Validate != "" {
		msg, err := f.params.Validate.Call(ctx, *number)
		if err != nil {
			return errSystem
		}
		if msg != "" {
			return errors.New(msg)
		}
	}
	return nil
}
