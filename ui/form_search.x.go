// Managed by GoX v0.3.4

//line form_search.gox:1
package ui

import (
	"context"
	"encoding/json"
	"errors"
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
			neui.IslandReport(ctx, fmt.Errorf("field %q search: %w", f.params.Name, err))
			options, failed = nil, true
		}
	}
	var keys []string
	var valid []option
	for _, o := range options {
		if o.Value == nil || o.Label == "" {
			neui.IslandReport(ctx, fmt.Errorf("field %q search: an option has no value or no label", f.params.Name))
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
	f.found.Update(ctx, formSearchFound{})
	f.setLocked(ctx)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(f.validate(f.value)))))
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
	f.setLocked(ctx)
	doors.Call(ctx, f.invalid.Set("aria-invalid", formAria(formShownErr(f.validate(f.value)))))
}

func (f *formWidgetSearch) setLocked(ctx context.Context) {
	f.chosen.Update(ctx, f.value)
	if len(f.value) == 0 {
		f.field.set(ctx, nil, f.validate(nil))
		return
	}
	f.field.set(ctx, f.value, f.validate(f.value))
}

//line form_search.gox:222
func (f *formWidgetSearch) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:224
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
//line form_search.gox:235
					__e = __c.Set("data:id", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const input = document.getElementById($data(\"id\"))\n\t\t\t\tinput.value = \"\"\n\t\t\t\tinput.focus()"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_search.gox:239
			return }))
		} else {
			focus.Inner(ctx, nil)
		}
		return false
	})

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_search.gox:246
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_search.gox:247
				__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_search.gox:247
				__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:248
				__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_search.gox:249
				__e = __c.Any(f.field.error.RouteBeam(
				doors.RouteBeamMatch(func(err error) bool {
					return f.params.Min > 0 && errors.Is(err, errRequired)
				}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_search.gox:252
						__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_search.gox:252
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("*"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_search.gox:252
				return }))),
			)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_search.gox:255
			__e = __c.Any(f.chosen.Bind(f.picked)); if __e != nil { return }
//line form_search.gox:256
			__e = __c.Any(doors.DeriveBeam(f.chosen, func(chosen []option) bool {
			return len(chosen) >= f.params.Max
		}).Bind(f.entry)); if __e != nil { return }
//line form_search.gox:259
			__e = __c.Any(&focus); if __e != nil { return }
//line form_search.gox:260
			__e = __c.Any(f.found.Bind(f.results)); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:261
				__e = __c.Set("class", "kisland-form-search-status"); if __e != nil { return }
//line form_search.gox:261
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:262
				__e = __c.Any(f.found.Bind(func(found formSearchFound) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form_search.gox:263
					if found.failed {
						__e = __c.Text("Search failed."); if __e != nil { return }
//line form_search.gox:265
					} else if found.query != "" && len(found.results) == 0 {
						__e = __c.Text("No matches for “"); if __e != nil { return }
//line form_search.gox:266
						__e = __c.Any(found.query); if __e != nil { return }
						__e = __c.Text("”."); if __e != nil { return }
//line form_search.gox:267
					} else if len(found.results) == 1 {
						__e = __c.Text("1 match"); if __e != nil { return }
//line form_search.gox:269
					} else if len(found.results) > 1 {
//line form_search.gox:270
						__e = __c.Any(len(found.results)); if __e != nil { return }
						__e = __c.Text(" matches"); if __e != nil { return }
					}
				return })
//line form_search.gox:272
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_search.gox:274
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(formShownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_search.gox:276
						__e = __c.Set("class", "kisland-form-error"); if __e != nil { return }
//line form_search.gox:276
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:276
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_search.gox:276
				return })
			})),
		)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_search.gox:280
}

//line form_search.gox:282
func (f *formWidgetSearch) picked(chosen []option) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:284
		full := len(chosen) >= f.params.Max
	describedby := any(nil)
	if full && f.params.Max > 1 {
		describedby = f.id + "-h"
	}

		__e = __c.Init("ul"); if __e != nil { return }
		{
//line form_search.gox:290
			__e = __c.Set("class", "kisland-form-search-chosen"); if __e != nil { return }
//line form_search.gox:290
			__e = __c.Set("aria-labelledby", f.id + "-l"); if __e != nil { return }
//line form_search.gox:290
			__e = __c.Set("aria-describedby", describedby); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:291
			for _, o := range chosen {
//line form_search.gox:293
				key, _ := json.Marshal(o.Value)

				__e = __c.Init("li"); if __e != nil { return }
				{
//line form_search.gox:295
					__e = __c.Set("class", "kisland-form-search-chip"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:296
					__e = __c.Any(o.Label); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line form_search.gox:298
						__e = __c.Modify(doors.AClick{
						Scope: f.field.scope(),
						On: func(ctx context.Context, r doors.RequestPointer) bool {
							f.remove(ctx, string(key))
							return false
						},
					}); if __e != nil { return }
//line form_search.gox:305
						__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:306
						__e = __c.Set("class", "kisland-form-remove"); if __e != nil { return }
//line form_search.gox:307
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
//line form_search.gox:313
}

//line form_search.gox:315
func (f *formWidgetSearch) entry(full bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:316
		if !full {
//line form_search.gox:318
			chosen, _ := f.chosen.Read(ctx)

			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line form_search.gox:321
				__e = __c.Modify(&f.controls); if __e != nil { return }
//line form_search.gox:322
				__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_search.gox:323
				__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Race: doors.RaceStrict,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.search(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_search.gox:331
				__e = __c.Modify(doors.AKeyDown{
				Key: doors.Key{Key: "Enter"},
				PreventDefault: true,
			}); if __e != nil { return }
//line form_search.gox:335
				__e = __c.Set("class", "kisland-form-search-input"); if __e != nil { return }
//line form_search.gox:336
				__e = __c.Set("id", f.id); if __e != nil { return }
//line form_search.gox:337
				__e = __c.Set("type", "text"); if __e != nil { return }
//line form_search.gox:338
				__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line form_search.gox:339
				__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_search.gox:340
				__e = __c.Set("aria-labelledby", f.id + "-l"); if __e != nil { return }
//line form_search.gox:341
				__e = __c.Set("aria-required", formAria(f.params.Min > 0)); if __e != nil { return }
//line form_search.gox:342
				__e = __c.Set("aria-invalid", formAria(formShownErr(f.validate(chosen)))); if __e != nil { return }
//line form_search.gox:343
				__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:344
		} else if f.params.Max > 1 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:345
				__e = __c.Set("class", "kisland-form-search-max"); if __e != nil { return }
//line form_search.gox:345
				__e = __c.Set("id", f.id + "-h"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_search.gox:345
				__e = __c.Any(f.params.Max); if __e != nil { return }
				__e = __c.Text(" chosen."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_search.gox:347
}

//line form_search.gox:349
func (f *formWidgetSearch) results(found formSearchFound) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:350
		if len(found.results) > 0 {
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line form_search.gox:351
				__e = __c.Set("class", "kisland-form-search-results"); if __e != nil { return }
//line form_search.gox:351
				__e = __c.Set("id", f.id + "-r"); if __e != nil { return }
//line form_search.gox:351
				__e = __c.Set("aria-labelledby", f.id + "-l"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:352
				for i := range found.results {
//line form_search.gox:354
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
//line form_search.gox:365
							__e = __c.Modify(doors.AClick{
							Scope: f.field.scope(),
							On: func(ctx context.Context, r doors.RequestPointer) bool {
								f.pick(ctx, found.results[i])
								return false
							},
						}); if __e != nil { return }
//line form_search.gox:372
							__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:373
							__e = __c.Set("class", "kisland-form-search-result"); if __e != nil { return }
//line form_search.gox:374
							__e = __c.Set("aria-label", "Choose " + o.Label); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:376
								__e = __c.Any(before); if __e != nil { return }
//line form_search.gox:377
								if match != "" {
									__e = __c.Init("mark"); if __e != nil { return }
									{
//line form_search.gox:378
										__e = __c.Set("class", "kisland-form-match"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:378
										__e = __c.Any(match); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
//line form_search.gox:380
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
//line form_search.gox:387
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
