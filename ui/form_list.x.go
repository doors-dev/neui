// Managed by GoX v0.3.4

//line form_list.gox:1
package ui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newFormWidgetList(ctx context.Context, group formGroup, params listInput) (*formWidgetList, error) {
	base, err := group.newGroup(params.name())
	if err != nil {
		return nil, err
	}
	base.collector = formListCollector{}
	items := doors.NewSourceEqual([]*formWidgetListField(nil), slices.Equal[[]*formWidgetListField])
	list := &formWidgetList{
		id: doors.IDRand(),
		group: base,
		params: params,
		append: new(doors.Door),
		items: items,
		count: doors.DeriveBeam(items, func(items []*formWidgetListField) int {
			return len(items)
		}),
	}
	list.system, err = base.newField("")
	if err != nil {
		return nil, err
	}
	return list, nil
}

type formListCollector struct{}

func (f formListCollector) collect(m map[string]any) any {
	slice := make([]any, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if k == "" {
			continue
		}
		value := m[k]
		slice = append(slice, defaultFormGroupCollector{}.value(value))
	}
	return slice
}

var _ formGroupCollector = formListCollector{}

type formWidgetList struct {
	id string
	index int
	group *formGroupBase
	system formGroupField
	params listInput
	fields []*formWidgetListField
	append *doors.Door
	items doors.Source[[]*formWidgetListField]
	count doors.Beam[int]
	add doors.Setter
}

func (f *formWidgetList) name() string {
	return f.params.name()
}

func (f *formWidgetList) init(ctx context.Context, value any) error {
	values := f.params.Value
	if value != nil {
		v, ok := value.([]any)
		if !ok {
			return fmt.Errorf("list %q: default value is %T, want an array", f.params.name(), value)
		}
		values = v
	}
	hi := 100
	if f.params.Max != 0 {
		hi = f.params.Max
	}
	if len(values) > hi {
		return fmt.Errorf("list %q: %d default items, most is %d", f.params.name(), len(values), hi)
	}
	for _, v := range values {
		if err := f.appendFieldLocked(ctx, v); err != nil {
			return err
		}
	}
	for len(f.fields) < f.params.Min {
		if err := f.appendFieldLocked(ctx, nil); err != nil {
			return err
		}
	}
	f.sync(ctx)
	return nil
}

func (f *formWidgetList) removeField(ctx context.Context, field *formWidgetListField) {
	f.group.mutex().Lock()
	defer f.group.mutex().Unlock()
	index := slices.Index(f.fields, field)
	if index == -1 {
		return
	}
	entry := f.fields[index]
	f.fields = slices.Delete(f.fields, index, index + 1)
	entry.field.destroy(ctx)
	f.items.Update(ctx, slices.Clone(f.fields))
	entry.door.Static(ctx, nil)
}

func (f *formWidgetList) appendField(ctx context.Context) {
	f.group.mutex().Lock()
	defer f.group.mutex().Unlock()
	if err := f.appendFieldLocked(ctx, nil); err != nil {
		f.onSystemError(ctx, err)
		return
	}
	f.sync(ctx)
}

func (f *formWidgetList) appendFieldLocked(ctx context.Context, value any) error {
	hi := 100
	if f.params.Max != 0 {
		hi = f.params.Max
	}
	if len(f.fields) >= hi {
		return nil
	}
	field, err := newFormWidgetField(ctx, f, f.params.Field.v)
	if err != nil {
		return err
	}
	if err := field.init(ctx, value); err != nil {
		field.destroy(ctx)
		return err
	}
	widgetField := &formWidgetListField{
		remove: f.removeField,
		field: field,
	}
	widgetField.state = doors.DeriveBeam(f.items, func(items []*formWidgetListField) formListItemState {
		return formListItemState{
			index: slices.Index(items, widgetField) + 1,
		}
	})
	f.fields = append(f.fields, widgetField)
	f.items.Update(ctx, slices.Clone(f.fields))
	widgetField.door.Outer(ctx, f.item(widgetField))
	new, old := new(doors.Door), f.append
	f.append = new
	old.Static(ctx, doors.Once(gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.InitContainer(); if __e != nil { return }
		{
//line form_list.gox:160
			__e = __c.Any(widgetField); if __e != nil { return }
//line form_list.gox:161
			__e = __c.Any(new); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_list.gox:162
	return })))
	return nil
}

func (f *formWidgetList) onSystemError(ctx context.Context, err error) {
	neui.IslandReport(ctx, err)
	f.system.set(ctx, nil, errSystem)
}

func (f *formWidgetList) sync(ctx context.Context) {
	if f.group.hasErrors() {
		f.system.set(ctx, nil, nil)
		return
	}
	if f.group.len() < f.params.Min && len(f.fields) >= f.params.Min {
		f.system.set(ctx, nil, errRequired)
		return
	}
	if f.group.len() < f.params.Min {
		f.system.set(ctx, nil, fmt.Errorf("add at least %d", f.params.Min))
		return
	}
	hi := 100
	if f.params.Max != 0 {
		hi = f.params.Max
	}
	if f.group.len() > hi {
		f.system.set(ctx, nil, fmt.Errorf("remove some, at most %d", hi))
		return
	}
	f.system.set(ctx, nil, nil)
}

func (f *formWidgetList) destroy(ctx context.Context) {
	for _, field := range f.fields {
		field.field.destroy(ctx)
	}
	f.group.destroy(ctx)
}

func (f *formWidgetList) form() *root {
	return f.group.form()
}

func (f *formWidgetList) mutex() *sync.Mutex {
	return f.group.mutex()
}

func (f *formWidgetList) scope() doors.Scopes {
	return f.group.scope()
}

func (f *formWidgetList) deleteField(ctx context.Context, name string) {
	f.group.deleteField(ctx, name)
	f.sync(ctx)
}

func (f *formWidgetList) deleteGroup(ctx context.Context, name string) {
	f.group.deleteGroup(ctx, name)
	f.sync(ctx)
}

func (f *formWidgetList) setField(ctx context.Context, name string, value any, err error) {
	f.group.setField(ctx, name, value, err)
	f.sync(ctx)
}

var maxIntWidth = len(strconv.Itoa(math.MaxInt))

func (f *formWidgetList) nextName() string {
	f.index += 1
	return fmt.Sprintf("%0*d", maxIntWidth, f.index)
}

func (f *formWidgetList) newGroup(name string) (*formGroupBase, error) {
	if f.params.name() != name {
		panic("wrong list group")
	}
	name = f.nextName()
	group, err := f.group.newGroup(name)
	if err != nil {
		return nil, err
	}
	group.parent = f
	return group, nil
}

func (f *formWidgetList) newField(name string) (formGroupField, error) {
	if f.params.name() != name {
		panic("wrong list group")
	}
	name = f.nextName()
	field, err := f.group.newField(name)
	if err != nil {
		return formGroupField{}, err
	}
	field.parent = f
	return field, nil
}

//line form_list.gox:262
func (f *formWidgetList) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_list.gox:264
		f.group.mutex().Lock()
	defer f.group.mutex().Unlock()
	hi := 100
	if f.params.Max != 0 {
		hi = f.params.Max
	}
	count, _ := f.count.ReadAndSub(ctx, func(ctx context.Context, count int) bool {
		doors.Call(ctx, f.add.Set("aria-disabled", formAria(count >= hi)))
		return false
	})

		__e = __c.Init("fieldset"); if __e != nil { return }
		{
//line form_list.gox:275
			__e = __c.Set("class", "kisland-form-box"); if __e != nil { return }
//line form_list.gox:275
			__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("legend"); if __e != nil { return }
			{
//line form_list.gox:276
				__e = __c.Set("class", "kisland-form-box-legend"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:277
				__e = __c.Any(f.params.label()); if __e != nil { return }
//line form_list.gox:278
				__e = __c.Any(f.system.error.RouteBeam(
				doors.RouteBeamMatch(func(err error) bool {
					return errors.Is(err, errRequired)
				}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_list.gox:281
						__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_list.gox:281
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("*"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_list.gox:281
				return }))),
			)); if __e != nil { return }
//line form_list.gox:283
				__e = __c.Any(" "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_list.gox:284
					__e = __c.Set("class", "kisland-form-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:285
					__e = __c.Any(f.count.Bind(func(count int) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_list.gox:286
						if f.params.Max != 0 && f.params.Min != 0 {
//line form_list.gox:287
							__e = __c.Many(count, " of ", f.params.Min, "–", f.params.Max); if __e != nil { return }
//line form_list.gox:288
						} else if f.params.Max != 0 {
//line form_list.gox:289
							__e = __c.Many(count, " of ", f.params.Max); if __e != nil { return }
						} else  {
//line form_list.gox:291
							__e = __c.Any(count); if __e != nil { return }
						}
					return })
//line form_list.gox:293
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("ol"); if __e != nil { return }
			{
//line form_list.gox:296
				__e = __c.Set("class", "kisland-form-items"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:297
				for _, field := range f.fields {
//line form_list.gox:298
					__e = __c.Any(field); if __e != nil { return }
				}
//line form_list.gox:300
				__e = __c.Any(f.append); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_list.gox:302
				__e = __c.Set("class", "kisland-form-items-foot"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:304
					__e = __c.Modify(&f.add); if __e != nil { return }
//line form_list.gox:305
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						f.appendField(ctx)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:312
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:313
					__e = __c.Set("class", "kisland-form-item-add"); if __e != nil { return }
//line form_list.gox:314
					__e = __c.Set("aria-label", func() any {
					if f.params.label() == "" {
						return "Add item"
					}
					return "Add to " + f.params.label()
				}()); if __e != nil { return }
//line form_list.gox:320
					__e = __c.Set("aria-disabled", formAria(count >= hi)); if __e != nil { return }
//line form_list.gox:321
					__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Add"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_list.gox:324
				__e = __c.Any(f.system.error.RouteBeam(
				doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form_list.gox:326
							__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_list.gox:326
							__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:326
							__e = __c.Any(err.Error()); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_list.gox:326
					return })
				})),
				doors.RouteComp[error](f.count.Bind(func(count int) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_list.gox:329
						if count >= hi {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:330
								__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form_list.gox:330
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_list.gox:330
								__e = __c.Any(hi); if __e != nil { return }
								__e = __c.Text(" reached."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_list.gox:331
						} else if count == 0 {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:332
								__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form_list.gox:332
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("None added."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					return })
//line form_list.gox:334
				})),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:338
}

//line form_list.gox:340
func (f *formWidgetList) item(field *formWidgetListField) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_list.gox:342
		_, group := field.field.(*formWidgetGroup)
	_, series := field.field.(*formWidgetSeries)
	label := any(nil)
	if group || series {
		state, _ := field.state.ReadAndSub(ctx, func(ctx context.Context, state formListItemState) bool {
			doors.Call(ctx, field.body.Set("aria-label", fmt.Sprintf("%s, item %d", f.params.label(), state.index)))
			return false
		})
		label = fmt.Sprintf("%s, item %d", f.params.label(), state.index)
	}

		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_list.gox:353
			__e = __c.Set("class", "kisland-form-item"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_list.gox:355
				__e = __c.Modify(&field.body); if __e != nil { return }
//line form_list.gox:356
				__e = __c.Set("class", "kisland-form-item-body"); if __e != nil { return }
//line form_list.gox:357
				__e = __c.Set("role", func() any {
				if label == nil {
					return nil
				}
				return "group"
			}()); if __e != nil { return }
//line form_list.gox:363
				__e = __c.Set("aria-label", label); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:364
				__e = __c.Any(field.field); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_list.gox:366
			__e = __c.Any(field.state.Bind(func(state formListItemState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:368
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						field.remove(ctx, field)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:375
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:376
					__e = __c.Set("class", "kisland-form-item-remove"); if __e != nil { return }
//line form_list.gox:377
					__e = __c.Set("aria-label", func() any {
					if f.params.label() == "" {
						return fmt.Sprintf("Remove item %d", state.index)
					}
					return fmt.Sprintf("Remove item %d of %s", state.index, f.params.label())
				}()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })
//line form_list.gox:384
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:386
}

var _ formGroup = (*formWidgetList)(nil)

type formListItemState struct {
	index int
}

type formWidgetListField struct {
	remove func(context.Context, *formWidgetListField)
	field formWidgetField
	door doors.Door
	state doors.Beam[formListItemState]
	body doors.Setter
}

func (f *formWidgetListField) Main() gox.Elem {
	return f.door.Main()
}
