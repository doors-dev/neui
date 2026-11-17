// Managed by GoX v0.4.0

//line form_series.gox:1
package form

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newSeriesWidget(ctx context.Context, group group, params seriesParams) (*seriesWidget, error) {
	base, err := group.newGroup(params.name())
	if err != nil {
		return nil, err
	}
	state := doors.NewSource(seriesState{})
	_, item := group.(*listWidget)
	series := &seriesWidget{
		id: doors.IDRand(),
		item: item,
		params: params,
		group: base,
		fields: make([]*seriesStepWidget, len(params.Fields)),
		state: state,
		complete: doors.DeriveBeam(state, func(s seriesState) bool {
			return s.complete
		}),
	}
	for i := range params.Fields {
		series.fields[i] = &seriesStepWidget{}
	}
	series.system, err = base.newField("")
	if err != nil {
		return nil, err
	}
	series.system.parent = series
	return series, nil
}

type seriesState struct {
	steps int
	complete bool
}

type seriesWidget struct {
	id string
	item bool
	params seriesParams
	group *groupBase
	system groupField
	fields []*seriesStepWidget
	defaults map[string]any
	initialized bool
	initError error
	state doors.Source[seriesState]
	complete doors.Beam[bool]
}

func (f *seriesWidget) name() string {
	return f.params.Name
}

func (f *seriesWidget) init(ctx context.Context, value any) error {
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

func (f *seriesWidget) getNextIndex(name string, err error) int {
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

func (f *seriesWidget) setField(ctx context.Context, name string, value any, err error) {
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
	f.state.Update(ctx, seriesState{steps: nextIndex})
	if shownErr(err) {
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
		f.state.Update(ctx, seriesState{steps: nextIndex, complete: true})
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
	if field.v == nil {
		if nextIndex == 0 {
			f.onSystemError(ctx, fmt.Errorf("series %q: the first step returned null, want a field", f.params.Name))
			return
		}
		f.state.Update(ctx, seriesState{steps: nextIndex, complete: true})
		f.system.set(ctx, nil, nil)
		return
	}
	required := f.params.Required || nextIndex > 0
	switch v := field.v.(type) {
	case *textParams:
		v.Required = required
	case *numberParams:
		v.Required = required
	case *dateParams:
		v.Required = required
	case *textareaParams:
		v.Required = required
	case *selectParams:
		if required {
			v.Min = max(v.Min, 1)
		} else {
			v.Min = 0
		}
	case *searchParams:
		if required {
			v.Min = max(v.Min, 1)
		} else {
			v.Min = 0
		}
	}
	widgetField, err := newFieldWidget(ctx, f, field.v)
	if err != nil {
		f.onSystemError(ctx, err)
		return
	}
	step := f.fields[nextIndex]
	step.set(widgetField)
	f.state.Update(ctx, seriesState{steps: nextIndex + 1})
	initial := f.defaults[widgetField.name()]
	delete(f.defaults, widgetField.name())
	err = widgetField.init(ctx, initial)
	step.door.Outer(ctx, f.step(nextIndex, &step.current, widgetField, field.v.label()))
	if err != nil {
		f.onSystemError(ctx, err)
		return
	}
}

func (f *seriesWidget) onSystemError(ctx context.Context, err error) {
	if f.initialized {
		neui.IslandReport(ctx, err)
	} else {
		f.initError = err
	}
	f.system.set(ctx, nil, errSystem)
}

func (f *seriesWidget) destroy(ctx context.Context) {
	for _, field := range f.fields {
		if field.isSet() {
			field.unset(ctx)
		}
	}
	f.group.destroy(ctx)
}

func (f *seriesWidget) form() *root {
	return f.group.form()
}

func (f *seriesWidget) mutex() *sync.Mutex {
	return f.group.mutex()
}

func (f *seriesWidget) scope() doors.Scopes {
	return f.group.scope()
}

func (f *seriesWidget) newField(name string) (groupField, error) {
	field, err := f.group.newField(name)
	if err != nil {
		return groupField{}, err
	}
	field.parent = f
	return field, nil
}

func (f *seriesWidget) newGroup(name string) (*groupBase, error) {
	return f.group.newGroup(name)
}

func (f *seriesWidget) value(name string) any {
	return f.group.value(name)
}

func (f *seriesWidget) deleteField(ctx context.Context, name string) {
	f.group.deleteField(ctx, name)
}

func (f *seriesWidget) deleteGroup(ctx context.Context, name string) {
	f.group.deleteGroup(ctx, name)
}

//line form_series.gox:238
func (f *seriesWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_series.gox:240
		steps := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("ol"); if __e != nil { return }
		{
//line form_series.gox:240
			__e = __c.Set("class", "neisland-form-steps"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:241
			for _, field := range f.fields {
//line form_series.gox:242
				__e = __c.Any(field); if __e != nil { return }
			}
//line form_series.gox:244
			__e = __c.Any(f.complete.Bind(func(complete bool) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_series.gox:245
				if complete {
					__e = __c.Init("li"); if __e != nil { return }
					{
//line form_series.gox:246
						__e = __c.Set("class", "neisland-form-step"); if __e != nil { return }
						__e = __c.Set("data-complete", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:247
							__e = __c.Set("class", "neisland-form-step-mark"); if __e != nil { return }
//line form_series.gox:247
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form_series.gox:248
							__e = __c.Set("class", "neisland-form-help"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Completed."); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("li"); if __e != nil { return }
					{
//line form_series.gox:251
						__e = __c.Set("class", "neisland-form-step"); if __e != nil { return }
						__e = __c.Set("data-incomplete", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:252
							__e = __c.Set("class", "neisland-form-step-mark"); if __e != nil { return }
//line form_series.gox:252
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:253
							__e = __c.Set("class", "neisland-form-step-status"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Answer to continue."); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			return })
//line form_series.gox:256
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_series.gox:257
	return })
	errLine := f.system.error.RouteBeam(
		doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_series.gox:260
					__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_series.gox:260
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:261
					__e = __c.Any(err.Error()); if __e != nil { return }
//line form_series.gox:262
					__e = __c.Any(neui.Data{Text: f.params.Name + " validation error", Object: err.Error()}); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_series.gox:263
			return })
		})),
	)
	body := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("fieldset"); if __e != nil { return }
		{
//line form_series.gox:266
			__e = __c.Set("class", "neisland-form-box"); if __e != nil { return }
//line form_series.gox:266
			__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("legend"); if __e != nil { return }
			{
//line form_series.gox:267
				__e = __c.Set("class", "neisland-form-box-legend"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line form_series.gox:269
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_series.gox:270
					__e = __c.Any(f.system.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Required && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_series.gox:273
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_series.gox:273
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_series.gox:273
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_series.gox:277
			__e = __c.Any(steps); if __e != nil { return }
//line form_series.gox:278
			__e = __c.Any(errLine); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_series.gox:279
	return })

//line form_series.gox:281
		if f.item {
//line form_series.gox:282
			__e = __c.Any(body); if __e != nil { return }
		} else  {
//line form_series.gox:284
			__e = (neui.Space{Name: f.params.Name, Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_series.gox:284
				__e = __c.Any(body); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line form_series.gox:286
}

//line form_series.gox:288
func (f *seriesWidget) step(index int, current *doors.Setter, field fieldWidget, label string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_series.gox:290
		isCurrent, _ := doors.DeriveBeam(f.state, func(s seriesState) bool {
		return !s.complete && s.steps == index + 1
	}).ReadAndSub(ctx, func(ctx context.Context, isCurrent bool) bool {
		doors.Call(ctx, current.Set("aria-current", stepAttr(isCurrent)))
		return false
	})
	_, search := field.(*searchWidget)

		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_series.gox:298
			__e = __c.Set("class", "neisland-form-step"); if __e != nil { return }
//line form_series.gox:298
			__e = __c.Modify(current); if __e != nil { return }
//line form_series.gox:298
			__e = __c.Set("aria-current", stepAttr(isCurrent)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_series.gox:299
				__e = __c.Set("class", "neisland-form-step-mark"); if __e != nil { return }
//line form_series.gox:299
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_series.gox:300
			if search {
//line form_series.gox:301
				__e = __c.Any(field); if __e != nil { return }
			} else  {
//line form_series.gox:303
				__e = (neui.Space{Name: field.name(), Desc: label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form_series.gox:303
					__e = __c.Any(field); if __e != nil { return }
				return })); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_series.gox:306
}

func stepAttr(current bool) any {
	if current {
		return "step"
	}
	return nil
}

var _ group = (*seriesWidget)(nil)

type seriesStepWidget struct {
	name string
	field fieldWidget
	door doors.Door
	current doors.Setter
}

func (f *seriesStepWidget) Main() gox.Elem {
	return f.door.Main()
}

func (f *seriesStepWidget) set(field fieldWidget) {
	f.name = field.name()
	f.field = field
}

func (f *seriesStepWidget) unset(ctx context.Context) {
	f.field.destroy(ctx)
	f.door.Outer(ctx, nil)
	f.name = ""
	f.field = nil
}

func (f *seriesStepWidget) isSet() bool {
	return f.name != ""
}
