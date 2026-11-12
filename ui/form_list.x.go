// Managed by GoX v0.3.4

//line form_list.gox:1
package ui

import (
	"context"
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
	switch v := params.Field.v.(type) {
	case *textInput:
		v.Required = true
	case *numberInput:
		v.Required = true
	case *dateInput:
		v.Required = true
	case *textArea:
		v.Required = true
	case *selectInput:
		v.Min = max(v.Min, 1)
	case *searchInput:
		v.Min = max(v.Min, 1)
	case *fileInput:
		v.Required = true
	case *seriesInput:
		v.Required = true
	}
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
	next string
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
	f.next = f.nextName()
	field, err := newFormWidgetField(ctx, f, f.params.Field.v)
	if err != nil {
		return err
	}
	if err := field.init(ctx, value); err != nil {
		field.destroy(ctx)
		return err
	}
	widgetField := &formWidgetListField{
		name: f.next,
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
//line form_list.gox:175
			__e = __c.Any(widgetField); if __e != nil { return }
//line form_list.gox:176
			__e = __c.Any(new); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_list.gox:177
	return })))
	return nil
}

func (f *formWidgetList) onSystemError(ctx context.Context, err error) {
	neui.IslandReport(ctx, err)
	f.system.set(ctx, nil, errSystem)
}

func (f *formWidgetList) sync(ctx context.Context) {
	if len(f.fields) == 0 && f.params.Min > 0 {
		f.system.set(ctx, nil, errRequired)
		return
	}
	if len(f.fields) < f.params.Min {
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
	group, err := f.group.newGroup(f.next)
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
	next := f.next
	field, err := f.group.newField(next)
	if err != nil {
		return formGroupField{}, err
	}
	field.parent = f
	field.item = doors.DeriveBeam(f.items, func(items []*formWidgetListField) string {
		return fmt.Sprintf("%s, item %d", f.params.label(), slices.IndexFunc(items, func(item *formWidgetListField) bool {
			return item.name == next
		}) + 1)
	})
	return field, nil
}

//line form_list.gox:277
func (f *formWidgetList) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_list.gox:279
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
//line form_list.gox:290
			__e = __c.Set("class", "kisland-form-box"); if __e != nil { return }
//line form_list.gox:290
			__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("legend"); if __e != nil { return }
			{
//line form_list.gox:291
				__e = __c.Set("class", "kisland-form-box-legend"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:293
					__e = __c.Any(f.params.label()); if __e != nil { return }
//line form_list.gox:294
					__e = __c.Any(doors.DeriveBeam(f.group.errors, func(m *map[string]any) bool {
					return f.params.Min > 0 && m != nil && len(*m) > 0
				}).RouteBeam(
					doors.RouteBeamValue(true, doors.RouteComp[bool](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_list.gox:297
							__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_list.gox:297
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_list.gox:297
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_list.gox:300
				__e = __c.Any(" "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_list.gox:301
					__e = __c.Set("class", "kisland-form-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:302
					__e = __c.Any(f.count.Bind(func(count int) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_list.gox:303
						if count == 0 {
//line form_list.gox:304
						} else if f.params.Max != 0 && f.params.Min != 0 {
//line form_list.gox:305
							__e = __c.Many(count, " of ", f.params.Min, "–", f.params.Max); if __e != nil { return }
//line form_list.gox:306
						} else if f.params.Max != 0 {
//line form_list.gox:307
							__e = __c.Many(count, " of ", f.params.Max); if __e != nil { return }
						} else  {
//line form_list.gox:309
							__e = __c.Any(count); if __e != nil { return }
						}
					return })
//line form_list.gox:311
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("ol"); if __e != nil { return }
			{
//line form_list.gox:314
				__e = __c.Set("class", "kisland-form-items"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:315
				for _, field := range f.fields {
//line form_list.gox:316
					__e = __c.Any(field); if __e != nil { return }
				}
//line form_list.gox:318
				__e = __c.Any(f.append); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_list.gox:320
				__e = __c.Set("class", "kisland-form-items-foot"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:322
					__e = __c.Modify(&f.add); if __e != nil { return }
//line form_list.gox:323
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						f.appendField(ctx)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:330
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:331
					__e = __c.Set("class", "kisland-form-item-add"); if __e != nil { return }
//line form_list.gox:332
					__e = __c.Set("aria-label", func() any {
					if f.params.label() == "" {
						return "Add item"
					}
					return "Add to " + f.params.label()
				}()); if __e != nil { return }
//line form_list.gox:338
					__e = __c.Set("aria-disabled", formAria(count >= hi)); if __e != nil { return }
//line form_list.gox:339
					__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Add"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_list.gox:342
				__e = __c.Any(f.system.error.RouteBeam(
				doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form_list.gox:344
							__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_list.gox:344
							__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:344
							__e = __c.Any(err.Error()); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_list.gox:344
					return })
				})),
				doors.RouteComp[error](f.count.Bind(func(count int) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_list.gox:347
						if count >= hi {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:348
								__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form_list.gox:348
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_list.gox:348
								__e = __c.Any(hi); if __e != nil { return }
								__e = __c.Text(" reached."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_list.gox:349
						} else if count == 0 {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:350
								__e = __c.Set("class", "kisland-form-help"); if __e != nil { return }
//line form_list.gox:350
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("None added."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					return })
//line form_list.gox:352
				})),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:356
}

//line form_list.gox:358
func (f *formWidgetList) item(field *formWidgetListField) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_list.gox:360
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
//line form_list.gox:371
			__e = __c.Set("class", "kisland-form-item"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_list.gox:373
				__e = __c.Modify(&field.body); if __e != nil { return }
//line form_list.gox:374
				__e = __c.Set("class", "kisland-form-item-body"); if __e != nil { return }
//line form_list.gox:375
				__e = __c.Set("role", func() any {
				if label == nil {
					return nil
				}
				return "group"
			}()); if __e != nil { return }
//line form_list.gox:381
				__e = __c.Set("aria-label", label); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:382
				__e = __c.Any(field.field); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_list.gox:384
			__e = __c.Any(field.state.Bind(func(state formListItemState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:386
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						field.remove(ctx, field)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:393
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:394
					__e = __c.Set("class", "kisland-form-item-remove"); if __e != nil { return }
//line form_list.gox:395
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
//line form_list.gox:402
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:404
}

var _ formGroup = (*formWidgetList)(nil)

type formListItemState struct {
	index int
}

type formWidgetListField struct {
	name string
	remove func(context.Context, *formWidgetListField)
	field formWidgetField
	door doors.Door
	state doors.Beam[formListItemState]
	body doors.Setter
}

func (f *formWidgetListField) Main() gox.Elem {
	return f.door.Main()
}
