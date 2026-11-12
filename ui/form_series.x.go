// Managed by GoX v0.3.4

//line form_series.gox:1
package ui

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newFormWidgetSeries(ctx context.Context, group formGroup, params seriesInput) (*formWidgetSeries, error) {
	base, err := group.newGroup(params.name())
	if err != nil {
		return nil, err
	}
	state := doors.NewSource(formSeriesState{})
	series := &formWidgetSeries{
		id: doors.IDRand(),
		params: params,
		group: base,
		fields: make([]*formWidgetSeriesField, len(params.Fields)),
		state: state,
		complete: doors.DeriveBeam(state, func(s formSeriesState) bool {
			return s.complete
		}),
	}
	for i := range params.Fields {
		series.fields[i] = &formWidgetSeriesField{}
	}
	series.system, err = base.newField("")
	if err != nil {
		return nil, err
	}
	series.system.parent = series
	return series, nil
}

type formSeriesState struct {
	steps int
	complete bool
}

type formWidgetSeries struct {
	id string
	params seriesInput
	group *formGroupBase
	system formGroupField
	fields []*formWidgetSeriesField
	defaults map[string]any
	initialized bool
	initError error
	state doors.Source[formSeriesState]
	complete doors.Beam[bool]
}

func (f *formWidgetSeries) name() string {
	return f.params.Name
}

func (f *formWidgetSeries) init(ctx context.Context, value any) error {
	if value != nil {
		defaults, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("series %q: default value is %T, want an object", f.params.Name, value)
		}
		f.defaults = defaults
	}
	f.system.set(ctx, nil, nil)
	f.initialized = true
	return f.initError
}

func (f *formWidgetSeries) getNextIndex(name string, err error) int {
	if name == "" {
		if f.fields[0].isSet() || err != nil {
			return -1
		}
		return 0
	}
	for i, field := range f.fields {
		if !field.isSet() {
			break
		}
		if name == field.name {
			return i + 1
		}
	}
	return -1
}

func (f *formWidgetSeries) setField(ctx context.Context, name string, value any, err error) {
	f.group.setField(ctx, name, value, err)
	nextIndex := f.getNextIndex(name, err)
	if nextIndex == -1 {
		return
	}
	for i := len(f.fields) - 1; i >= nextIndex; i-- {
		field := f.fields[i]
		if field.isSet() {
			field.unset(ctx)
		}
	}
	f.state.Update(ctx, formSeriesState{steps: nextIndex})
	if formShownErr(err) {
		return
	}
	if nextIndex != 0 && value == nil {
		if nextIndex == 1 && !f.params.Required {
			f.system.set(ctx, nil, nil)
			return
		}
		f.system.set(ctx, nil, errRequired)
		return
	}
	if nextIndex == len(f.params.Fields) {
		f.state.Update(ctx, formSeriesState{steps: nextIndex, complete: true})
		f.system.set(ctx, nil, nil)
		return
	}
	field, err := f.params.Fields[nextIndex].Call(ctx, value)
	if err != nil {
		if !f.initialized {
			f.initError = err
		}
		f.system.set(ctx, nil, errSystem)
		return
	}
	required := f.params.Required || nextIndex > 0
	switch v := field.v.(type) {
	case *textInput:
		v.Required = required
	case *numberInput:
		v.Required = required
	case *dateInput:
		v.Required = required
	case *textArea:
		v.Required = required
	case *selectInput:
		if required {
			v.Min = max(v.Min, 1)
		} else {
			v.Min = 0
		}
	case *searchInput:
		if required {
			v.Min = max(v.Min, 1)
		} else {
			v.Min = 0
		}
	case *fileInput:
		v.Required = required
	}
	widgetField, err := newFormWidgetField(ctx, f, field.v)
	if err != nil {
		f.onSystemError(ctx, err)
		return
	}
	step := f.fields[nextIndex]
	step.set(widgetField)
	f.state.Update(ctx, formSeriesState{steps: nextIndex + 1})
	initial := f.defaults[widgetField.name()]
	delete(f.defaults, widgetField.name())
	err = widgetField.init(ctx, initial)
	step.door.Outer(ctx, f.step(nextIndex, &step.current, widgetField))
	if err != nil {
		f.onSystemError(ctx, err)
		return
	}
}

func (f *formWidgetSeries) onSystemError(ctx context.Context, err error) {
	if f.initialized {
		neui.IslandReport(ctx, err)
	} else {
		f.initError = err
	}
	f.system.set(ctx, nil, errSystem)
}

func (f *formWidgetSeries) destroy(ctx context.Context) {
	for _, field := range f.fields {
		if field.isSet() {
			field.unset(ctx)
		}
	}
	f.group.destroy(ctx)
}

func (f *formWidgetSeries) form() *root {
	return f.group.form()
}

func (f *formWidgetSeries) mutex() *sync.Mutex {
	return f.group.mutex()
}

func (f *formWidgetSeries) scope() doors.Scopes {
	return f.group.scope()
}

func (f *formWidgetSeries) newField(name string) (formGroupField, error) {
	field, err := f.group.newField(name)
	if err != nil {
		return formGroupField{}, err
	}
	field.parent = f
	return field, nil
}

func (f *formWidgetSeries) newGroup(name string) (*formGroupBase, error) {
	return f.group.newGroup(name)
}

func (f *formWidgetSeries) deleteField(ctx context.Context, name string) {
	f.group.deleteField(ctx, name)
}

func (f *formWidgetSeries) deleteGroup(ctx context.Context, name string) {
	f.group.deleteGroup(ctx, name)
}

//line form_series.gox:224
func (f *formWidgetSeries) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_series.gox:226
		steps := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("ol"); if __e != nil { return }
		{
//line form_series.gox:226
			__e = __c.Set("class", "kisland-form-steps"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:227
			for _, field := range f.fields {
//line form_series.gox:228
				__e = __c.Any(field); if __e != nil { return }
			}
//line form_series.gox:230
			__e = __c.Any(f.complete.Bind(func(complete bool) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_series.gox:231
				if complete {
					__e = __c.Init("li"); if __e != nil { return }
					{
//line form_series.gox:232
						__e = __c.Set("class", "kisland-form-step"); if __e != nil { return }
						__e = __c.Set("data-complete", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:233
							__e = __c.Set("class", "kisland-form-step-mark"); if __e != nil { return }
//line form_series.gox:233
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form_series.gox:234
							__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Completed."); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			return })
//line form_series.gox:237
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_series.gox:238
	return })
	errLine := f.system.error.RouteBeam(
		doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_series.gox:241
					__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_series.gox:241
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:241
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_series.gox:241
			return })
		})),
	)

		__e = __c.Init("fieldset"); if __e != nil { return }
		{
//line form_series.gox:245
			__e = __c.Set("class", "kisland-form-box"); if __e != nil { return }
//line form_series.gox:245
			__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("legend"); if __e != nil { return }
			{
//line form_series.gox:246
				__e = __c.Set("class", "kisland-form-box-legend"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:248
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_series.gox:249
					__e = __c.Any(f.system.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:252
							__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_series.gox:252
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_series.gox:252
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_series.gox:256
			__e = __c.Any(steps); if __e != nil { return }
//line form_series.gox:257
			__e = __c.Any(errLine); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_series.gox:259
}

//line form_series.gox:261
func (f *formWidgetSeries) step(index int, current *doors.Setter, field formWidgetField) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_series.gox:263
		isCurrent, _ := doors.DeriveBeam(f.state, func(s formSeriesState) bool {
		return !s.complete && s.steps == index + 1
	}).ReadAndSub(ctx, func(ctx context.Context, isCurrent bool) bool {
		doors.Call(ctx, current.Set("aria-current", formStep(isCurrent)))
		return false
	})

		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_series.gox:270
			__e = __c.Set("class", "kisland-form-step"); if __e != nil { return }
//line form_series.gox:270
			__e = __c.Modify(current); if __e != nil { return }
//line form_series.gox:270
			__e = __c.Set("aria-current", formStep(isCurrent)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_series.gox:271
				__e = __c.Set("class", "kisland-form-step-mark"); if __e != nil { return }
//line form_series.gox:271
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_series.gox:272
			__e = __c.Any(field); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_series.gox:274
}

func formStep(current bool) any {
	if current {
		return "step"
	}
	return nil
}

var _ formGroup = (*formWidgetSeries)(nil)

type formWidgetSeriesField struct {
	name string
	field formWidgetField
	door doors.Door
	current doors.Setter
}

func (f *formWidgetSeriesField) Main() gox.Elem {
	return f.door.Main()
}

func (f *formWidgetSeriesField) set(field formWidgetField) {
	f.name = field.name()
	f.field = field
}

func (f *formWidgetSeriesField) unset(ctx context.Context) {
	f.field.destroy(ctx)
	f.door.Outer(ctx, nil)
	f.name = ""
	f.field = nil
}

func (f *formWidgetSeriesField) isSet() bool {
	return f.name != ""
}
