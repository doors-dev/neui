// Managed by GoX v0.3.4

//line form_date.gox:1
package form

import (
	"context"
	"errors"
	"fmt"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func newDateWidget(ctx context.Context, group group, params dateParams) (*dateWidget, error) {
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &dateWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
	}, nil
}

type dateWidget struct {
	id string
	field groupField
	params dateParams
	value string
	invalid doors.Setter
	label doors.Setter
}

func (f *dateWidget) name() string {
	return f.params.Name
}

func (f *dateWidget) init(ctx context.Context, value any) error {
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

func (f *dateWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *dateWidget) update(ctx context.Context, str string) {
	err := f.validate(ctx, str)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	f.setLocked(ctx, str, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
}

func (f *dateWidget) setLocked(ctx context.Context, str string, err error) {
	if str == "" {
		f.field.set(ctx, nil, err)
		return
	}
	f.field.set(ctx, str, err)
}

//line form_date.gox:73
func (f *dateWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_date.gox:75
		label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_date.gox:83
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_date.gox:84
			if label == nil {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_date.gox:85
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_date.gox:85
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_date.gox:86
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_date.gox:87
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_date.gox:90
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_date.gox:90
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_date.gox:90
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form_date.gox:95
				__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_date.gox:96
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_date.gox:97
				__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.update(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_date.gox:105
				__e = __c.Set("class", "neisland-form-input"); if __e != nil { return }
//line form_date.gox:106
				__e = __c.Set("id", f.id); if __e != nil { return }
//line form_date.gox:107
				__e = __c.Set("type", f.params.Type); if __e != nil { return }
//line form_date.gox:108
				__e = __c.Set("name", f.params.Name); if __e != nil { return }
//line form_date.gox:109
				__e = __c.Set("value", f.value); if __e != nil { return }
//line form_date.gox:110
				__e = __c.Set("min", f.params.Min); if __e != nil { return }
//line form_date.gox:111
				__e = __c.Set("max", f.params.Max); if __e != nil { return }
//line form_date.gox:112
				__e = __c.Set("aria-required", aria(f.params.Required)); if __e != nil { return }
//line form_date.gox:113
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_date.gox:114
				__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_date.gox:115
				__e = __c.Set("data-required-empty", errors.Is(f.field.error.Get(), errRequired)); if __e != nil { return }
//line form_date.gox:116
				__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
//line form_date.gox:117
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_date.gox:119
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_date.gox:119
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_date.gox:119
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_date.gox:119
				return })
			})),
		)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_date.gox:123
}

func (f *dateWidget) validate(ctx context.Context, str string) error {
	if str == "" {
		if f.params.Required {
			return errRequired
		}
		return nil
	}
	switch f.params.Type {
	case "date":
		if _, err := time.Parse("2006-01-02", str); err != nil {
			return errors.New("enter a date as YYYY-MM-DD")
		}
	case "time":
		_, err := time.Parse("15:04", str)
		if err != nil {
			_, err = time.Parse("15:04:05", str)
		}
		if err != nil {
			return errors.New("enter a time as HH:MM")
		}
	case "datetime-local":
		_, err := time.Parse("2006-01-02T15:04", str)
		if err != nil {
			_, err = time.Parse("2006-01-02T15:04:05", str)
		}
		if err != nil {
			return errors.New("enter a date and time as YYYY-MM-DDTHH:MM")
		}
	case "month":
		if _, err := time.Parse("2006-01", str); err != nil {
			return errors.New("enter a month as YYYY-MM")
		}
	case "week":
		var year,week int
		var rest string
		n, _ := fmt.Sscanf(str, "%4d-W%2d%s", &year, &week, &rest)
		_, last := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
		if n != 2 || len(str) != 8 || week < 1 || week > last {
			return errors.New("enter a week as YYYY-Www")
		}
	}
	if f.params.Min != "" && str < f.params.Min {
		return fmt.Errorf("pick %s or later", f.params.Min)
	}
	if f.params.Max != "" && str > f.params.Max {
		return fmt.Errorf("pick %s or earlier", f.params.Max)
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
