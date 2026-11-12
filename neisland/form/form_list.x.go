// Managed by GoX v0.3.4

//line form_list.gox:1
package form

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

func newListWidget(ctx context.Context, group group, params listParams) (*listWidget, error) {
	switch v := params.Field.v.(type) {
	case *textParams:
		v.Required = true
	case *numberParams:
		v.Required = true
	case *dateParams:
		v.Required = true
	case *textareaParams:
		v.Required = true
	case *selectParams:
		v.Min = max(v.Min, 1)
	case *searchParams:
		v.Min = max(v.Min, 1)
	case *fileParams:
		v.Min = max(v.Min, 1)
	case *seriesParams:
		v.Required = true
	}
	base, err := group.newGroup(params.name())
	if err != nil {
		return nil, err
	}
	base.collector = listCollector{}
	items := doors.NewSourceEqual([]*listItemWidget(nil), slices.Equal[[]*listItemWidget])
	list := &listWidget{
		id: doors.IDRand(),
		group: base,
		params: params,
		append: new(doors.Door),
		items: items,
		count: doors.DeriveBeam(items, func(items []*listItemWidget) int {
			return len(items)
		}),
	}
	list.system, err = base.newField("")
	if err != nil {
		return nil, err
	}
	return list, nil
}

type listCollector struct{}

func (f listCollector) collect(m map[string]any) any {
	slice := make([]any, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if k == "" {
			continue
		}
		value := m[k]
		slice = append(slice, defaultCollector{}.value(value))
	}
	return slice
}

var _ groupCollector = listCollector{}

type listWidget struct {
	id string
	index int
	next string
	group *groupBase
	system groupField
	params listParams
	fields []*listItemWidget
	append *doors.Door
	items doors.Source[[]*listItemWidget]
	count doors.Beam[int]
	add doors.Setter
}

func (f *listWidget) name() string {
	return f.params.name()
}

func (f *listWidget) init(ctx context.Context, value any) error {
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

func (f *listWidget) removeField(ctx context.Context, field *listItemWidget) {
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

func (f *listWidget) appendField(ctx context.Context) {
	f.group.mutex().Lock()
	defer f.group.mutex().Unlock()
	if err := f.appendFieldLocked(ctx, nil); err != nil {
		f.onSystemError(ctx, err)
		return
	}
	f.sync(ctx)
}

func (f *listWidget) appendFieldLocked(ctx context.Context, value any) error {
	hi := 100
	if f.params.Max != 0 {
		hi = f.params.Max
	}
	if len(f.fields) >= hi {
		return nil
	}
	f.next = f.nextName()
	field, err := newFieldWidget(ctx, f, f.params.Field.v)
	if err != nil {
		return err
	}
	if err := field.init(ctx, value); err != nil {
		field.destroy(ctx)
		return err
	}
	widgetField := &listItemWidget{
		name: f.next,
		remove: f.removeField,
		field: field,
	}
	widgetField.state = doors.DeriveBeam(f.items, func(items []*listItemWidget) listItemState {
		return listItemState{
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

func (f *listWidget) onSystemError(ctx context.Context, err error) {
	neui.IslandReport(ctx, err)
	f.system.set(ctx, nil, errSystem)
}

func (f *listWidget) sync(ctx context.Context) {
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

func (f *listWidget) destroy(ctx context.Context) {
	for _, field := range f.fields {
		field.field.destroy(ctx)
	}
	f.group.destroy(ctx)
}

func (f *listWidget) form() *root {
	return f.group.form()
}

func (f *listWidget) mutex() *sync.Mutex {
	return f.group.mutex()
}

func (f *listWidget) scope() doors.Scopes {
	return f.group.scope()
}

func (f *listWidget) deleteField(ctx context.Context, name string) {
	f.group.deleteField(ctx, name)
	f.sync(ctx)
}

func (f *listWidget) deleteGroup(ctx context.Context, name string) {
	f.group.deleteGroup(ctx, name)
	f.sync(ctx)
}

func (f *listWidget) setField(ctx context.Context, name string, value any, err error) {
	f.group.setField(ctx, name, value, err)
	f.sync(ctx)
}

var maxIntWidth = len(strconv.Itoa(math.MaxInt))

func (f *listWidget) nextName() string {
	f.index += 1
	return fmt.Sprintf("%0*d", maxIntWidth, f.index)
}

func (f *listWidget) newGroup(name string) (*groupBase, error) {
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

func (f *listWidget) newField(name string) (groupField, error) {
	if f.params.name() != name {
		panic("wrong list group")
	}
	next := f.next
	field, err := f.group.newField(next)
	if err != nil {
		return groupField{}, err
	}
	field.parent = f
	field.item = doors.DeriveBeam(f.items, func(items []*listItemWidget) string {
		return fmt.Sprintf("%s, item %d", f.params.label(), slices.IndexFunc(items, func(item *listItemWidget) bool {
			return item.name == next
		}) + 1)
	})
	return field, nil
}

//line form_list.gox:277
func (f *listWidget) Main() gox.Elem {
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
		doors.Call(ctx, f.add.Set("aria-disabled", aria(count >= hi)))
		return false
	})

		__e = __c.Init("fieldset"); if __e != nil { return }
		{
//line form_list.gox:290
			__e = __c.Set("class", "neisland-form-box"); if __e != nil { return }
//line form_list.gox:290
			__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("legend"); if __e != nil { return }
			{
//line form_list.gox:291
				__e = __c.Set("class", "neisland-form-box-legend"); if __e != nil { return }
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
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
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
					__e = __c.Set("class", "neisland-form-count"); if __e != nil { return }
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
				__e = __c.Set("class", "neisland-form-items"); if __e != nil { return }
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
				__e = __c.Set("class", "neisland-form-items-foot"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:322
					__e = __c.Modify(&f.add); if __e != nil { return }
//line form_list.gox:323
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					Batch: true,
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						f.appendField(ctx)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:331
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:332
					__e = __c.Set("class", "neisland-form-item-add"); if __e != nil { return }
//line form_list.gox:333
					__e = __c.Set("aria-label", func() any {
					if f.params.label() == "" {
						return "Add item"
					}
					return "Add to " + f.params.label()
				}()); if __e != nil { return }
//line form_list.gox:339
					__e = __c.Set("aria-disabled", aria(count >= hi)); if __e != nil { return }
//line form_list.gox:340
					__e = __c.Set("aria-describedby", f.id + "-m"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Add"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_list.gox:343
				__e = __c.Any(f.system.error.RouteBeam(
				doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("p"); if __e != nil { return }
						{
//line form_list.gox:345
							__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_list.gox:345
							__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:345
							__e = __c.Any(err.Error()); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_list.gox:345
					return })
				})),
				doors.RouteComp[error](f.count.Bind(func(count int) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_list.gox:348
						if count >= hi {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:349
								__e = __c.Set("class", "neisland-form-help"); if __e != nil { return }
//line form_list.gox:349
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_list.gox:349
								__e = __c.Any(hi); if __e != nil { return }
								__e = __c.Text(" reached."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line form_list.gox:350
						} else if count == 0 {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line form_list.gox:351
								__e = __c.Set("class", "neisland-form-help"); if __e != nil { return }
//line form_list.gox:351
								__e = __c.Set("id", f.id + "-m"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("None added."); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					return })
//line form_list.gox:353
				})),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:357
}

//line form_list.gox:359
func (f *listWidget) item(field *listItemWidget) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_list.gox:361
		_, group := field.field.(*groupWidget)
	_, series := field.field.(*seriesWidget)
	label := any(nil)
	if group || series {
		state, _ := field.state.ReadAndSub(ctx, func(ctx context.Context, state listItemState) bool {
			doors.Call(ctx, field.body.Set("aria-label", fmt.Sprintf("%s, item %d", f.params.label(), state.index)))
			return false
		})
		label = fmt.Sprintf("%s, item %d", f.params.label(), state.index)
	}

		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_list.gox:372
			__e = __c.Set("class", "neisland-form-item"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_list.gox:374
				__e = __c.Modify(&field.body); if __e != nil { return }
//line form_list.gox:375
				__e = __c.Set("class", "neisland-form-item-body"); if __e != nil { return }
//line form_list.gox:376
				__e = __c.Set("role", func() any {
				if label == nil {
					return nil
				}
				return "group"
			}()); if __e != nil { return }
//line form_list.gox:382
				__e = __c.Set("aria-label", label); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_list.gox:383
				__e = __c.Any(field.field); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_list.gox:385
			__e = __c.Any(field.state.Bind(func(state listItemState) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_list.gox:387
					__e = __c.Modify(doors.AClick{
					Scope: f.group.scope(),
					Batch: true,
					On: func(ctx context.Context, r doors.RequestPointer) bool {
						field.remove(ctx, field)
						return false
					},
				}); if __e != nil { return }
//line form_list.gox:395
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_list.gox:396
					__e = __c.Set("class", "neisland-form-item-remove"); if __e != nil { return }
//line form_list.gox:397
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
//line form_list.gox:404
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_list.gox:406
}

var _ group = (*listWidget)(nil)

type listItemState struct {
	index int
}

type listItemWidget struct {
	name string
	remove func(context.Context, *listItemWidget)
	field fieldWidget
	door doors.Door
	state doors.Beam[listItemState]
	body doors.Setter
}

func (f *listItemWidget) Main() gox.Elem {
	return f.door.Main()
}
