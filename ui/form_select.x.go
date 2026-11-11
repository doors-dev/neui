// Managed by GoX v0.3.4

//line form_select.gox:1
package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func newFormWidgetSelect(ctx context.Context, group formGroup, params selectInput) (*formWidgetSelect, error) {
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
	return &formWidgetSelect{
		id: doors.IDRand(),
		field: field,
		params: params,
		keys: keys,
		defaults: defaults,
	}, nil
}

type formWidgetSelect struct {
	id string
	field formGroupField
	params selectInput
	keys map[string]int
	defaults []int
	value []int
	invalid doors.Setter
}

func (f *formWidgetSelect) name() string {
	return f.params.Name
}

func (f *formWidgetSelect) init(ctx context.Context, value any) error {
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
	f.value = selected
	f.setLocked(ctx, selected, f.validate(selected))
	return nil
}

func (f *formWidgetSelect) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *formWidgetSelect) update(ctx context.Context, changed string, checked bool) {
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
		for i := range f.params.Options {
			if i == index && checked || i != index && slices.Contains(f.value, i) {
				selected = append(selected, i)
			}
		}
	}
	err := f.validate(selected)
	f.value = selected
	f.setLocked(ctx, selected, err)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(err))))
}

func (f *formWidgetSelect) setLocked(ctx context.Context, selected []int, err error) {
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

//line form_select.gox:163
func (f *formWidgetSelect) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_select.gox:165
		f.field.mutex().Lock()
	value := f.value
	f.field.mutex().Unlock()
	change := doors.A(ctx, doors.AChange{
		Scope: f.field.scope(),
		On: func(ctx context.Context, r doors.RequestChange) bool {
			f.update(ctx, r.Event().Value, r.Event().Checked)
			return false
		},
	})
	invalid := formAria(formShownErr(f.field.error.Get()))
	required := f.field.error.RouteBeam(
		doors.RouteBeamMatch(func(err error) bool {
			return f.params.Min > 0 && errors.Is(err, errRequired)
		}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_select.gox:179
				__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_select.gox:179
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("*"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_select.gox:179
		return }))),
	)
	errorLine := f.field.error.RouteBeam(
		doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("p"); if __e != nil { return }
				{
//line form_select.gox:183
					__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_select.gox:183
					__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:183
					__e = __c.Any(err.Error()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:183
			return })
		})),
	)

//line form_select.gox:187
		if f.params.Max == 1 && len(f.params.Options) > 5 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line form_select.gox:188
				__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("label"); if __e != nil { return }
				{
//line form_select.gox:189
					__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_select.gox:189
					__e = __c.Set("for", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:190
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:191
					__e = __c.Any(required); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("select"); if __e != nil { return }
				{
//line form_select.gox:194
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:195
					__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:196
					__e = __c.Set("class", "kisland-form-select"); if __e != nil { return }
//line form_select.gox:197
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_select.gox:198
					__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:199
					__e = __c.Set("aria-required", formAria(f.params.Min > 0)); if __e != nil { return }
//line form_select.gox:200
					__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:201
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("option"); if __e != nil { return }
					{
//line form_select.gox:202
						__e = __c.Set("class", "kisland-form-option"); if __e != nil { return }
//line form_select.gox:202
						__e = __c.Set("value", ""); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Choose…"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_select.gox:203
					for i, o := range f.params.Options {
						__e = __c.Init("option"); if __e != nil { return }
						{
//line form_select.gox:204
							__e = __c.Set("class", "kisland-form-option"); if __e != nil { return }
//line form_select.gox:204
							__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:204
							__e = __c.Set("selected", slices.Contains(value, i)); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:204
							__e = __c.Any(o.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:207
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_select.gox:209
		} else if f.params.Max == 1 {
			__e = __c.Init("fieldset"); if __e != nil { return }
			{
//line form_select.gox:210
				__e = __c.Set("class", "kisland-form-group"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("legend"); if __e != nil { return }
				{
//line form_select.gox:211
					__e = __c.Set("class", "kisland-form-label kisland-form-legend"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:212
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:213
					__e = __c.Any(required); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_select.gox:216
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:217
					__e = __c.Set("class", "kisland-form-options"); if __e != nil { return }
//line form_select.gox:218
					__e = __c.Set("role", "radiogroup"); if __e != nil { return }
//line form_select.gox:219
					__e = __c.Set("aria-required", formAria(f.params.Min > 0)); if __e != nil { return }
//line form_select.gox:220
					__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:221
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:222
					if f.params.Min == 0 {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:223
							__e = __c.Set("class", "kisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line form_select.gox:225
								__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:226
								__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:227
								__e = __c.Set("class", "kisland-form-radio"); if __e != nil { return }
//line form_select.gox:228
								__e = __c.Set("type", "radio"); if __e != nil { return }
//line form_select.gox:229
								__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:230
								__e = __c.Set("value", ""); if __e != nil { return }
//line form_select.gox:231
								__e = __c.Set("checked", len(value) == 0); if __e != nil { return }
//line form_select.gox:232
								__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("None"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
//line form_select.gox:236
					for i, o := range f.params.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:237
							__e = __c.Set("class", "kisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line form_select.gox:239
								__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:240
								__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:241
								__e = __c.Set("class", "kisland-form-radio"); if __e != nil { return }
//line form_select.gox:242
								__e = __c.Set("type", "radio"); if __e != nil { return }
//line form_select.gox:243
								__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:244
								__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:245
								__e = __c.Set("checked", slices.Contains(value, i)); if __e != nil { return }
//line form_select.gox:246
								__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:247
							__e = __c.Any(o.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:251
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("fieldset"); if __e != nil { return }
			{
//line form_select.gox:254
				__e = __c.Set("class", "kisland-form-group"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("legend"); if __e != nil { return }
				{
//line form_select.gox:255
					__e = __c.Set("class", "kisland-form-label kisland-form-legend"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:256
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_select.gox:257
					__e = __c.Any(required); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line form_select.gox:260
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:261
					__e = __c.Set("class", "kisland-form-options"); if __e != nil { return }
//line form_select.gox:262
					__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
//line form_select.gox:263
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:264
					for i, o := range f.params.Options {
						__e = __c.Init("label"); if __e != nil { return }
						{
//line form_select.gox:265
							__e = __c.Set("class", "kisland-form-check"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line form_select.gox:267
								__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_select.gox:268
								__e = __c.Modify(change); if __e != nil { return }
//line form_select.gox:269
								__e = __c.Set("class", "kisland-form-checkbox"); if __e != nil { return }
//line form_select.gox:270
								__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line form_select.gox:271
								__e = __c.Set("name", f.id); if __e != nil { return }
//line form_select.gox:272
								__e = __c.Set("value", strconv.Itoa(i)); if __e != nil { return }
//line form_select.gox:273
								__e = __c.Set("checked", slices.Contains(value, i)); if __e != nil { return }
//line form_select.gox:274
								__e = __c.Set("aria-invalid", invalid); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
//line form_select.gox:275
							__e = __c.Any(o.Label); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line form_select.gox:279
				__e = __c.Any(errorLine); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_select.gox:282
}

func (f *formWidgetSelect) validate(selected []int) error {
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
