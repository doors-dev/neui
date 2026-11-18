// Managed by GoX v0.4.0

//line form_select.gox:1
package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newSelectWidget(ctx context.Context, group group, params selectParams) (*selectWidget, error) {
	if params.Max == 0 {
		params.Max = 1
	}
	if params.Max < params.Min {
		return nil, fmt.Errorf("field %q: max %d is below min %d", params.Name, params.Max, params.Min)
	}
	keys := make(map[string]int, len(params.Options))
	for i, o := range params.Options {
		if o.Value == nil {
			return nil, fmt.Errorf("field %q: option %q has no value", params.Name, o.Label)
		}
		key, err := json.Marshal(o.Value)
		if err != nil {
			return nil, fmt.Errorf("field %q: option %q: %w", params.Name, o.Label, err)
		}
		if len(key) > 4096 {
			return nil, fmt.Errorf("field %q: option %q: value is %d bytes as JSON, most is 4096", params.Name, o.Label, len(key))
		}
		if _, has := keys[string(key)]; has {
			return nil, fmt.Errorf("field %q: option %q: value %s is already used by another option", params.Name, o.Label, key)
		}
		keys[string(key)] = i
	}
	var defaults []int
	for _, o := range params.Value {
		key, err := json.Marshal(o.Value)
		if err != nil {
			return nil, fmt.Errorf("field %q: default %q: %w", params.Name, o.Label, err)
		}
		i, has := keys[string(key)]
		if !has {
			return nil, fmt.Errorf("field %q: default value %s is not one of the options", params.Name, key)
		}
		if !slices.Contains(defaults, i) {
			defaults = append(defaults, i)
		}
	}
	slices.Sort(defaults)
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &selectWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
		keys: keys,
		defaults: defaults,
	}, nil
}

type selectWidget struct {
	id string
	field groupField
	params selectParams
	keys map[string]int
	defaults []int
	invalid doors.Setter
	label doors.Setter
}

func (f *selectWidget) name() string {
	return f.params.Name
}

func (f *selectWidget) init(ctx context.Context, value any) error {
	selected := f.defaults
	if value != nil {
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want an array of {value, label} objects", f.params.Name, value)
		}
		selected = nil
		for _, item := range items {
			o, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("field %q: default item is %T, want a {value, label} object", f.params.Name, item)
			}
			v, hasValue := o["value"]
			_, hasLabel := o["label"].(string)
			if !hasValue || !hasLabel {
				return fmt.Errorf("field %q: default item needs a value and a string label", f.params.Name)
			}
			key, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("field %q: default item: %w", f.params.Name, err)
			}
			i, has := f.keys[string(key)]
			if !has {
				return fmt.Errorf("field %q: default value %s is not one of the options", f.params.Name, key)
			}
			if !slices.Contains(selected, i) {
				selected = append(selected, i)
			}
		}
		slices.Sort(selected)
	}
	f.setLocked(ctx, selected, f.validate(selected))
	return nil
}

func (f *selectWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *selectWidget) update(ctx context.Context, changed string, checked bool) {
	index := -1
	if changed != "" {
		i, err := strconv.Atoi(changed)
		if err != nil || i < 0 || i >= len(f.params.Options) {
			return
		}
		index = i
	}
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	var selected []int
	if f.params.Max == 1 {
		if index != -1 {
			selected = []int{index}
		}
	} else {
		current := f.selectedLocked()
		for i := range f.params.Options {
			if i == index && checked || i != index && slices.Contains(current, i) {
				selected = append(selected, i)
			}
		}
	}
	err := f.validate(selected)
	f.setLocked(ctx, selected, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(err))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(err, errRequired)))
}

func (f *selectWidget) setLocked(ctx context.Context, selected []int, err error) {
	if len(selected) == 0 {
		f.field.set(ctx, nil, err)
		return
	}
	options := make([]option, 0, len(selected))
	for _, i := range selected {
		options = append(options, f.params.Options[i])
	}
	f.field.set(ctx, options, err)
}

func (f *selectWidget) selectedLocked() []int {
	options, _ := f.field.value().([]option)
	selected := make([]int, 0, len(options))
	for _, o := range options {
		key, _ := json.Marshal(o.Value)
		selected = append(selected, f.keys[string(key)])
	}
	return selected
}

//line form_select.gox:174
func (f *selectWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_select.gox:176
		f.field.mutex().Lock()
	value := f.selectedLocked()
	f.field.mutex().Unlock()
	change := doors.A(ctx, doors.AChange{
		Scope: f.field.scope(),
		Batch: true,
		On: func(ctx context.Context, r doors.RequestChange) bool {
			f.update(ctx, r.Event().Value, r.Event().Checked)
			return false
		},
	})
	invalid := aria(shownErr(f.field.error.Get()))
	requiredEmpty := errors.Is(f.field.error.Get(), errRequired)
	label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}
	required := f.field.error.RouteBeam(
		doors.RouteBeamMatch(func(err error) bool {
			return f.params.Min > 0 && err != nil
		}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_select.gox:199
				__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_select.gox:199
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("*"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_select.gox:199
		return }))),
	)
	errorLine := f.field.error.RouteBeam(
		doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_select.gox:203
					__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_select.gox:203
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:203
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:203
			return })
		})),
	)

//line form_select.gox:207
		if f.params.Max == 1 && len(f.params.Options) > 5 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_select.gox:208
				__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:209
				if label == nil {
					__e = __c.Init("label"); if __e != nil { return }
					{
//line form_select.gox:210
						__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_select.gox:210
						__e = __c.Set("for", f.id); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:211
						__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:212
						__e = __c.Any(required); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line form_select.gox:215
				__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("select"); if __e != nil { return }
					{
//line form_select.gox:216
						__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:217
						__e = __c.Modify(&f.label); if __e != nil { return }
//line form_select.gox:218
						__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:219
						__e = __c.Set("class", "neisland-form-select"); if __e != nil { return }
//line form_select.gox:220
						__e = __c.Set("id", f.id); if __e != nil { return }
//line form_select.gox:221
						__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:222
						__e = __c.Set("aria-required", aria(f.params.Min > 0)); if __e != nil { return }
//line form_select.gox:223
						__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_select.gox:224
						__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:225
						__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
//line form_select.gox:226
						__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("option"); if __e != nil { return }
						{
//line form_select.gox:227
							__e = __c.Set("class", "neisland-form-option"); if __e != nil { return }
//line form_select.gox:227
							__e = __c.Set("value", ""); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Choose…"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_select.gox:228
						for i, o := range f.params.Options {
							__e = __c.Init("option"); if __e != nil { return }
							{
//line form_select.gox:229
								__e = __c.Set("class", "neisland-form-option"); if __e != nil { return }
//line form_select.gox:229
								__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:229
								__e = __c.Set("selected", slices.Contains(value, i)); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:229
								__e = __c.Any(o.Label); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line form_select.gox:232
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_select.gox:234
		} else if f.params.Max == 1 {
			__e = __c.Init("fieldset"); if __e != nil { return }
			{
//line form_select.gox:235
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_select.gox:235
				__e = __c.Set("class", "neisland-form-group"); if __e != nil { return }
//line form_select.gox:235
				__e = __c.Set("aria-label", label); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:236
				if label == nil {
					__e = __c.Init("legend"); if __e != nil { return }
					{
//line form_select.gox:237
						__e = __c.Set("class", "neisland-form-label neisland-form-legend"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:238
						__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:239
						__e = __c.Any(required); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_select.gox:243
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:244
					__e = __c.Set("class", "neisland-form-options"); if __e != nil { return }
//line form_select.gox:245
					__e = __c.Set("role", "radiogroup"); if __e != nil { return }
//line form_select.gox:246
					__e = __c.Set("aria-required", aria(f.params.Min > 0)); if __e != nil { return }
//line form_select.gox:247
					__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:248
					__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
//line form_select.gox:249
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:250
					if f.params.Min == 0 {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:251
							__e = __c.Set("class", "neisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:252
							__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.InitVoid("input"); if __e != nil { return }
								{
//line form_select.gox:253
									__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:254
									__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:255
									__e = __c.Set("class", "neisland-form-radio"); if __e != nil { return }
//line form_select.gox:256
									__e = __c.Set("type", "radio"); if __e != nil { return }
//line form_select.gox:257
									__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:258
									__e = __c.Set("value", ""); if __e != nil { return }
//line form_select.gox:259
									__e = __c.Set("checked", len(value) == 0); if __e != nil { return }
//line form_select.gox:260
									__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:261
									__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
								}
								__e = __c.Submit(); if __e != nil { return }
							return })); if __e != nil { return }
							__e = __c.Text("None"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line form_select.gox:265
					for i, o := range f.params.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:266
							__e = __c.Set("class", "neisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:267
							__e = (neui.Use{Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.InitVoid("input"); if __e != nil { return }
								{
//line form_select.gox:268
									__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:269
									__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:270
									__e = __c.Set("class", "neisland-form-radio"); if __e != nil { return }
//line form_select.gox:271
									__e = __c.Set("type", "radio"); if __e != nil { return }
//line form_select.gox:272
									__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:273
									__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:274
									__e = __c.Set("checked", slices.Contains(value, i)); if __e != nil { return }
//line form_select.gox:275
									__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:276
									__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
								}
								__e = __c.Submit(); if __e != nil { return }
							return })); if __e != nil { return }
//line form_select.gox:277
							__e = __c.Any(o.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:281
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("fieldset"); if __e != nil { return }
			{
//line form_select.gox:284
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_select.gox:284
				__e = __c.Set("class", "neisland-form-group"); if __e != nil { return }
//line form_select.gox:284
				__e = __c.Set("aria-label", label); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:285
				if label == nil {
					__e = __c.Init("legend"); if __e != nil { return }
					{
//line form_select.gox:286
						__e = __c.Set("class", "neisland-form-label neisland-form-legend"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:287
						__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:288
						__e = __c.Any(required); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_select.gox:292
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:293
					__e = __c.Set("class", "neisland-form-options"); if __e != nil { return }
//line form_select.gox:294
					__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:295
					__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
//line form_select.gox:296
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:297
					for i, o := range f.params.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:298
							__e = __c.Set("class", "neisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:299
							__e = (neui.Use{Desc: f.params.Label + ": " + o.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.InitVoid("input"); if __e != nil { return }
								{
//line form_select.gox:300
									__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:301
									__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:302
									__e = __c.Set("class", "neisland-form-checkbox"); if __e != nil { return }
//line form_select.gox:303
									__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line form_select.gox:304
									__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:305
									__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:306
									__e = __c.Set("checked", slices.Contains(value, i)); if __e != nil { return }
//line form_select.gox:307
									__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:308
									__e = __c.Set("data-required-empty", requiredEmpty); if __e != nil { return }
								}
								__e = __c.Submit(); if __e != nil { return }
							return })); if __e != nil { return }
//line form_select.gox:309
							__e = __c.Any(o.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:313
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
//line form_select.gox:316
		__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
	return })
//line form_select.gox:317
}

func (f *selectWidget) validate(selected []int) error {
	if len(selected) == 0 {
		if f.params.Min > 0 {
			return errRequired
		}
		return nil
	}
	if len(selected) < f.params.Min {
		return fmt.Errorf("choose at least %d", f.params.Min)
	}
	if len(selected) > f.params.Max {
		return fmt.Errorf("choose at most %d", f.params.Max)
	}
	return nil
}
