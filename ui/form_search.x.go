// Managed by GoX v0.3.4

//line form_search.gox:1
package ui

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

func newFormWidgetSearch(ctx context.Context, group formGroup, params searchInput) (*formWidgetSearch, error) {
	if params.Search == "" {
		return nil, fmt.Errorf("field %q: search is required", params.Name)
	}
	if params.Max == 0 {
		params.Max = 1
	}
	if params.Max < params.Min {
		return nil, fmt.Errorf("field %q: max %d is below min %d", params.Name, params.Max, params.Min)
	}
	if params.Placeholder == "" {
		params.Placeholder = "Search…"
	}
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	return &formWidgetSearch{
		id: doors.IDRand(),
		field: field,
		params: params,
		chosen: doors.NewSourceEqual[[]option](nil, nil),
		found: doors.NewSourceEqual(formSearchFound{}, nil),
	}, nil
}

type formSearchFound struct {
	query string
	results []option
	failed bool
}

type formWidgetSearch struct {
	id string
	field formGroupField
	params searchInput
	value []option
	keys []string
	query string
	chosen doors.Source[[]option]
	found doors.Source[formSearchFound]
	controls doors.Setter
	invalid doors.Setter
	label doors.Setter
}

func (f *formWidgetSearch) name() string {
	return f.params.Name
}

func (f *formWidgetSearch) init(ctx context.Context, value any) error {
	options := f.params.Value
	if value != nil {
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want an array of {value, label} objects", f.params.Name, value)
		}
		options = make([]option, 0, len(items))
		for _, item := range items {
			o, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("field %q: default item is %T, want a {value, label} object", f.params.Name, item)
			}
			label, _ := o["label"].(string)
			options = append(options, option{Value: o["value"], Label: label})
		}
	}
	if len(options) > f.params.Max {
		return fmt.Errorf("field %q: %d default options, most is %d", f.params.Name, len(options), f.params.Max)
	}
	keys := make([]string, 0, len(options))
	for _, o := range options {
		if o.Value == nil || o.Label == "" {
			return fmt.Errorf("field %q: default item needs a value and a label", f.params.Name)
		}
		key, err := json.Marshal(o.Value)
		if err != nil {
			return fmt.Errorf("field %q: default %q: %w", f.params.Name, o.Label, err)
		}
		if len(key) > 4096 {
			return fmt.Errorf("field %q: default %q: value is %d bytes as JSON, most is 4096", f.params.Name, o.Label, len(key))
		}
		if slices.Contains(keys, string(key)) {
			return fmt.Errorf("field %q: default value %s is chosen twice", f.params.Name, key)
		}
		keys = append(keys, string(key))
	}
	if len(options) == 0 {
		options, keys = nil, nil
	}
	f.value = options
	f.keys = keys
	f.setLocked(ctx)
	return nil
}

func (f *formWidgetSearch) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *formWidgetSearch) search(ctx context.Context, query string) {
	query = strings.TrimSpace(query)
	f.field.mutex().Lock()
	f.query = query
	f.field.mutex().Unlock()
	var options []option
	failed := false
	if query != "" {
		var err error
		options, err = f.params.Search.Call(ctx, query)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			options, failed = nil, true
		}
	}
	var keys []string
	var valid []option
	for _, o := range options {
		if o.Value == nil {
			neui.IslandReport(ctx, fmt.Errorf("field %q search: an option has no value", f.params.Name))
			continue
		}
		key, err := json.Marshal(o.Value)
		if err != nil {
			neui.IslandReport(ctx, fmt.Errorf("field %q search: option %q: %w", f.params.Name, o.Label, err))
			continue
		}
		if len(key) > 4096 {
			neui.IslandReport(ctx, fmt.Errorf("field %q search: option %q: value is %d bytes as JSON, most is 4096", f.params.Name, o.Label, len(key)))
			continue
		}
		if slices.Contains(keys, string(key)) {
			neui.IslandReport(ctx, fmt.Errorf("field %q search: option %q: value %s is already used by another option", f.params.Name, o.Label, key))
			continue
		}
		keys = append(keys, string(key))
		valid = append(valid, o)
	}
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	if f.query != query {
		return
	}
	var results []option
	for i, o := range valid {
		if len(results) == 8 {
			break
		}
		if !slices.Contains(f.keys, keys[i]) {
			results = append(results, o)
		}
	}
	f.found.Update(ctx, formSearchFound{query: query, results: results, failed: failed})
	controls := any(nil)
	if len(results) > 0 {
		controls = f.id + "-r"
	}
	doors.Call(ctx, f.controls.Set("aria-controls", controls))
}

func (f *formWidgetSearch) pick(ctx context.Context, o option) {
	key, err := json.Marshal(o.Value)
	if err != nil {
		return
	}
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	if len(f.value) >= f.params.Max || slices.Contains(f.keys, string(key)) {
		return
	}
	f.value = append(slices.Clone(f.value), o)
	f.keys = append(slices.Clone(f.keys), string(key))
	f.query = ""
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(f.validate(f.value)))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(f.validate(f.value), errRequired)))
	f.setLocked(ctx)
	f.found.Update(ctx, formSearchFound{})
}

func (f *formWidgetSearch) remove(ctx context.Context, key string) {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.Index(f.keys, key)
	if i == -1 {
		return
	}
	f.value = slices.Delete(slices.Clone(f.value), i, i + 1)
	f.keys = slices.Delete(slices.Clone(f.keys), i, i + 1)
	if len(f.value) == 0 {
		f.value, f.keys = nil, nil
	}
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(f.validate(f.value)))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(f.validate(f.value), errRequired)))
	f.setLocked(ctx)
}

func (f *formWidgetSearch) setLocked(ctx context.Context) {
	if len(f.value) == 0 {
		f.field.set(ctx, nil, f.validate(nil))
	} else {
		f.field.set(ctx, f.value, f.validate(f.value))
	}
	f.chosen.Update(ctx, f.value)
}

//line form_search.gox:224
func (f *formWidgetSearch) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:226
		label := any(nil)
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
	}
	var focus doors.Door
	first := true
	var prev []option
	f.chosen.Sub(ctx, func(ctx context.Context, chosen []option) bool {
		grew := len(chosen) > len(prev) && len(chosen) < f.params.Max
		if first {
			first, prev = false, chosen
			return false
		}
		prev = chosen
		if grew {
			focus.Inner(ctx, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("script"); if __e != nil { return }
				{
//line form_search.gox:244
					__e = __c.Set("data:id", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const input = document.getElementById($data(\"id\"))\n\t\t\t\tinput.value = \"\"\n\t\t\t\tinput.focus()"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_search.gox:248
			return }))
		} else {
			focus.Inner(ctx, nil)
		}
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_search.gox:255
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:256
			if label == nil {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_search.gox:257
					__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_search.gox:257
					__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:258
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_search.gox:259
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Min > 0 && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_search.gox:262
							__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_search.gox:262
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_search.gox:262
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_search.gox:266
			__e = __c.Any(f.chosen.Bind(f.picked)); if __e != nil { return }
//line form_search.gox:267
			__e = __c.Any(doors.DeriveBeam(f.chosen, func(chosen []option) bool {
			return len(chosen) >= f.params.Max
		}).Bind(f.entry)); if __e != nil { return }
//line form_search.gox:270
			__e = __c.Any(&focus); if __e != nil { return }
//line form_search.gox:271
			__e = __c.Any(f.found.Bind(f.results)); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:272
				__e = __c.Set("class", "kisland-form-search-status"); if __e != nil { return }
//line form_search.gox:272
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:273
				__e = __c.Any(f.found.Bind(func(found formSearchFound) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form_search.gox:274
					if found.failed {
						__e = __c.Text("Search failed."); if __e != nil { return }
//line form_search.gox:276
					} else if found.query != "" && len(found.results) == 0 {
						__e = __c.Text("No matches for “"); if __e != nil { return }
//line form_search.gox:277
						__e = __c.Any(found.query); if __e != nil { return }
						__e = __c.Text("”."); if __e != nil { return }
//line form_search.gox:278
					} else if len(found.results) == 1 {
						__e = __c.Text("1 match"); if __e != nil { return }
//line form_search.gox:280
					} else if len(found.results) > 1 {
//line form_search.gox:281
						__e = __c.Any(len(found.results)); if __e != nil { return }
						__e = __c.Text(" matches"); if __e != nil { return }
					}
				return })
//line form_search.gox:283
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_search.gox:285
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_search.gox:287
						__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_search.gox:287
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:287
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_search.gox:287
				return })
			})),
		)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_search.gox:291
}

//line form_search.gox:293
func (f *formWidgetSearch) picked(chosen []option) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:295
		full := len(chosen) >= f.params.Max
	label, labelledby := any(nil), any(f.id + "-l")
	if f.field.item != nil {
		label, _ = f.field.item.Read(ctx)
		labelledby = nil
	}
	describedby := any(nil)
	if full && f.params.Max > 1 {
		describedby = f.id + "-h"
	}

		__e = __c.Init("ul"); if __e != nil { return }
		{
//line form_search.gox:307
			__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:308
			__e = __c.Set("class", "kisland-form-search-chosen"); if __e != nil { return }
//line form_search.gox:309
			__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:310
			__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_search.gox:311
			__e = __c.Set("aria-describedby", describedby); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:312
			for _, o := range chosen {
//line form_search.gox:314
				key, _ := json.Marshal(o.Value)

				__e = __c.Init("li"); if __e != nil { return }
				{
//line form_search.gox:316
					__e = __c.Set("class", "kisland-form-search-chip"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:317
					__e = __c.Any(o.Label); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form_search.gox:319
						__e = __c.Modify(doors.AClick{
						Scope: f.field.scope(),
						Batch: true,
						On: func(ctx context.Context, r doors.RequestPointer) bool {
							f.remove(ctx, string(key))
							return false
						},
					}); if __e != nil { return }
//line form_search.gox:327
						__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:328
						__e = __c.Set("class", "kisland-form-remove"); if __e != nil { return }
//line form_search.gox:329
						__e = __c.Set("aria-label", "Remove " + o.Label); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("×"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_search.gox:335
}

//line form_search.gox:337
func (f *formWidgetSearch) entry(full bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:338
		if !full {
//line form_search.gox:340
			chosen, _ := f.chosen.Read(ctx)
		label, labelledby := any(nil), any(f.id + "-l")
		if f.field.item != nil {
			label, _ = f.field.item.Read(ctx)
			labelledby = nil
		}

			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form_search.gox:348
				__e = __c.Modify(&f.controls); if __e != nil { return }
//line form_search.gox:349
				__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_search.gox:350
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:351
				__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Race: doors.RaceStrict,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.search(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_search.gox:359
				__e = __c.Modify(doors.AKeyDown{
				Key: doors.Key{Key: "Enter"},
				PreventDefault: true,
			}); if __e != nil { return }
//line form_search.gox:363
				__e = __c.Set("class", "kisland-form-search-input"); if __e != nil { return }
//line form_search.gox:364
				__e = __c.Set("id", f.id); if __e != nil { return }
//line form_search.gox:365
				__e = __c.Set("type", "text"); if __e != nil { return }
//line form_search.gox:366
				__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line form_search.gox:367
				__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_search.gox:368
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:369
				__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_search.gox:370
				__e = __c.Set("aria-required", formAria(f.params.Min > 0)); if __e != nil { return }
//line form_search.gox:371
				__e = __c.Set("aria-invalid", formAria(formShownErr(f.validate(chosen)))); if __e != nil { return }
//line form_search.gox:372
				__e = __c.Set("data-required-empty", errors.Is(f.validate(chosen), errRequired)); if __e != nil { return }
//line form_search.gox:373
				__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:374
		} else if f.params.Max > 1 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:375
				__e = __c.Set("class", "kisland-form-search-max"); if __e != nil { return }
//line form_search.gox:375
				__e = __c.Set("id", f.id + "-h"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_search.gox:375
				__e = __c.Any(f.params.Max); if __e != nil { return }
				__e = __c.Text(" chosen."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_search.gox:377
}

//line form_search.gox:379
func (f *formWidgetSearch) results(found formSearchFound) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:381
		label, labelledby := any(nil), any(f.id + "-l")
	if f.field.item != nil {
		label, _ = f.field.item.Read(ctx)
		labelledby = nil
	}

//line form_search.gox:387
		if len(found.results) > 0 {
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line form_search.gox:389
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:390
				__e = __c.Set("class", "kisland-form-search-results"); if __e != nil { return }
//line form_search.gox:391
				__e = __c.Set("id", f.id + "-r"); if __e != nil { return }
//line form_search.gox:392
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:393
				__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:394
				for i := range found.results {
//line form_search.gox:396
					o := found.results[i]
				before, match, after := o.Label, "", ""
				for at := range o.Label {
					if end := at + len(found.query); end <= len(o.Label) && strings.EqualFold(o.Label[at:end], found.query) {
						before, match, after = o.Label[:at], o.Label[at:end], o.Label[end:]
						break
					}
				}

					__e = __c.Init("li"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("button"); if __e != nil { return }
						{
//line form_search.gox:407
							__e = __c.Modify(doors.AClick{
							Scope: f.field.scope(),
							Batch: true,
							On: func(ctx context.Context, r doors.RequestPointer) bool {
								f.pick(ctx, found.results[i])
								return false
							},
						}); if __e != nil { return }
//line form_search.gox:415
							__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:416
							__e = __c.Set("class", "kisland-form-search-result"); if __e != nil { return }
//line form_search.gox:417
							__e = __c.Set("aria-label", "Choose " + o.Label); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:419
								__e = __c.Any(before); if __e != nil { return }
//line form_search.gox:420
								if match != "" {
									__e = __c.Init("mark"); if __e != nil { return }
									{
//line form_search.gox:421
										__e = __c.Set("class", "kisland-form-match"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:421
										__e = __c.Any(match); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
//line form_search.gox:423
								__e = __c.Any(after); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_search.gox:430
}

func (f *formWidgetSearch) validate(chosen []option) error {
	if len(chosen) == 0 {
		if f.params.Min > 0 {
			return errRequired
		}
		return nil
	}
	if len(chosen) < f.params.Min {
		return fmt.Errorf("choose at least %d", f.params.Min)
	}
	return nil
}
