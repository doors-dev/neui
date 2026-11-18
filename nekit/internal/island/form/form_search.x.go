// Managed by GoX v0.4.0

//line form_search.gox:1
// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package form

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

func newSearchWidget(ctx context.Context, group group, params searchParams) (*searchWidget, error) {
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
	return &searchWidget{
		id: doors.IDRand(),
		field: field,
		params: params,
		chosen: doors.NewSourceEqual[[]option](nil, nil),
		found: doors.NewSourceEqual(searchFound{}, nil),
	}, nil
}

type searchFound struct {
	query string
	results []option
	failed bool
}

type searchWidget struct {
	id string
	field groupField
	params searchParams
	value []option
	keys []string
	query string
	chosen doors.Source[[]option]
	found doors.Source[searchFound]
	controls doors.Setter
	invalid doors.Setter
	label doors.Setter
}

func (f *searchWidget) name() string {
	return f.params.Name
}

func (f *searchWidget) init(ctx context.Context, value any) error {
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

func (f *searchWidget) destroy(ctx context.Context) {
	f.field.destroy(ctx)
}

func (f *searchWidget) search(ctx context.Context, query string) {
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
	f.found.Update(ctx, searchFound{query: query, results: results, failed: failed})
	controls := any(nil)
	if len(results) > 0 {
		controls = f.id + "-r"
	}
	doors.Call(ctx, f.controls.Set("aria-controls", controls))
}

func (f *searchWidget) pick(ctx context.Context, o option) {
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
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(f.validate(f.value)))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(f.validate(f.value), errRequired)))
	f.setLocked(ctx)
	f.found.Update(ctx, searchFound{})
}

func (f *searchWidget) remove(ctx context.Context, key string) {
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
	doors.Call(ctx, f.invalid.Set("aria-invalid", aria(shownErr(f.validate(f.value)))))
	doors.Call(ctx, f.invalid.Set("data-required-empty", errors.Is(f.validate(f.value), errRequired)))
	f.setLocked(ctx)
}

func (f *searchWidget) setLocked(ctx context.Context) {
	if len(f.value) == 0 {
		f.field.set(ctx, nil, f.validate(nil))
	} else {
		f.field.set(ctx, f.value, f.validate(f.value))
	}
	f.chosen.Update(ctx, f.value)
}

//line form_search.gox:238
func (f *searchWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:240
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
//line form_search.gox:258
					__e = __c.Set("data:id", f.id); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const input = document.getElementById($data(\"id\"))\n\t\t\t\tinput.value = \"\"\n\t\t\t\tinput.focus()"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line form_search.gox:262
			return }))
		} else {
			focus.Inner(ctx, nil)
		}
		return false
	})
	body := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_search.gox:268
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:269
			if label == nil {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_search.gox:270
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_search.gox:270
					__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:271
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_search.gox:272
					__e = __c.Any(f.field.error.RouteBeam(
					doors.RouteBeamMatch(func(err error) bool {
						return f.params.Min > 0 && err != nil
					}, doors.RouteComp[error](gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("span"); if __e != nil { return }
						{
//line form_search.gox:275
							__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_search.gox:275
							__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("*"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line form_search.gox:275
					return }))),
				)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_search.gox:279
			__e = __c.Any(f.chosen.Bind(f.picked)); if __e != nil { return }
//line form_search.gox:280
			__e = __c.Any(doors.DeriveBeam(f.chosen, func(chosen []option) bool {
			return len(chosen) >= f.params.Max
		}).Bind(f.entry)); if __e != nil { return }
//line form_search.gox:283
			__e = __c.Any(&focus); if __e != nil { return }
//line form_search.gox:284
			__e = __c.Any(f.found.Bind(f.results)); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:285
				__e = __c.Set("class", "neisland-form-search-status"); if __e != nil { return }
//line form_search.gox:285
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:286
				__e = __c.Any(f.found.Bind(func(found searchFound) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line form_search.gox:287
					if found.failed {
						__e = __c.Text("Search failed."); if __e != nil { return }
//line form_search.gox:289
					} else if found.query != "" && len(found.results) == 0 {
						__e = __c.Text("No matches for “"); if __e != nil { return }
//line form_search.gox:290
						__e = __c.Any(found.query); if __e != nil { return }
						__e = __c.Text("”."); if __e != nil { return }
//line form_search.gox:291
					} else if len(found.results) == 1 {
						__e = __c.Text("1 match"); if __e != nil { return }
//line form_search.gox:293
					} else if len(found.results) > 1 {
//line form_search.gox:294
						__e = __c.Any(len(found.results)); if __e != nil { return }
						__e = __c.Text(" matches"); if __e != nil { return }
					}
				return })
//line form_search.gox:296
			})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_search.gox:298
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_search.gox:300
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_search.gox:300
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:300
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_search.gox:300
				return })
			})),
		)); if __e != nil { return }
//line form_search.gox:303
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_search.gox:304
	return })

//line form_search.gox:306
		if f.field.item != nil {
//line form_search.gox:307
			__e = __c.Any(body); if __e != nil { return }
		} else  {
//line form_search.gox:309
			__e = (neui.Space{Name: f.params.Name, Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_search.gox:309
				__e = __c.Any(body); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line form_search.gox:311
}

//line form_search.gox:313
func (f *searchWidget) picked(chosen []option) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:315
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
//line form_search.gox:327
			__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:328
			__e = __c.Set("class", "neisland-form-search-chosen"); if __e != nil { return }
//line form_search.gox:329
			__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:330
			__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_search.gox:331
			__e = __c.Set("aria-describedby", describedby); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:332
			for _, o := range chosen {
//line form_search.gox:334
				key, _ := json.Marshal(o.Value)

				__e = __c.Init("li"); if __e != nil { return }
				{
//line form_search.gox:336
					__e = __c.Set("class", "neisland-form-search-chip"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:337
					__e = __c.Any(o.Label); if __e != nil { return }
//line form_search.gox:338
					__e = (neui.Use{Desc: "remove " + o.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line form_search.gox:339
							__e = __c.Modify(doors.AClick{
						Scope: f.field.scope(),
						Batch: true,
						On: func(ctx context.Context, r doors.RequestPointer) bool {
							f.remove(ctx, string(key))
							return false
						},
					}); if __e != nil { return }
//line form_search.gox:347
							__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:348
							__e = __c.Set("class", "neisland-form-remove"); if __e != nil { return }
//line form_search.gox:349
							__e = __c.Set("value", string(key)); if __e != nil { return }
//line form_search.gox:350
							__e = __c.Set("aria-label", "Remove " + o.Label); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("×"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_search.gox:356
}

//line form_search.gox:358
func (f *searchWidget) entry(full bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:359
		if !full {
//line form_search.gox:361
			chosen, _ := f.chosen.Read(ctx)
		label, labelledby := any(nil), any(f.id + "-l")
		if f.field.item != nil {
			label, _ = f.field.item.Read(ctx)
			labelledby = nil
		}

//line form_search.gox:368
			__e = (neui.Use{Desc: "search"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line form_search.gox:369
					__e = __c.Modify(&f.controls); if __e != nil { return }
//line form_search.gox:370
					__e = __c.Modify(&f.invalid); if __e != nil { return }
//line form_search.gox:371
					__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:372
					__e = __c.Modify(doors.AInput{
				Scope: f.field.scope().And(&doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second}),
				Race: doors.RaceStrict,
				On: func(ctx context.Context, r doors.RequestInput) bool {
					f.search(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line form_search.gox:380
					__e = __c.Modify(doors.AKeyDown{
				Key: doors.Key{Key: "Enter"},
				PreventDefault: true,
			}); if __e != nil { return }
//line form_search.gox:384
					__e = __c.Set("class", "neisland-form-search-input"); if __e != nil { return }
//line form_search.gox:385
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_search.gox:386
					__e = __c.Set("type", "text"); if __e != nil { return }
//line form_search.gox:387
					__e = __c.Set("autocomplete", "off"); if __e != nil { return }
//line form_search.gox:388
					__e = __c.Set("placeholder", f.params.Placeholder); if __e != nil { return }
//line form_search.gox:389
					__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:390
					__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_search.gox:391
					__e = __c.Set("aria-required", aria(f.params.Min > 0)); if __e != nil { return }
//line form_search.gox:392
					__e = __c.Set("aria-invalid", aria(shownErr(f.validate(chosen)))); if __e != nil { return }
//line form_search.gox:393
					__e = __c.Set("data-required-empty", errors.Is(f.validate(chosen), errRequired)); if __e != nil { return }
//line form_search.gox:394
					__e = __c.Set("aria-describedby", f.id + "-e"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
			return })); if __e != nil { return }
//line form_search.gox:395
		} else if f.params.Max > 1 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_search.gox:396
				__e = __c.Set("class", "neisland-form-search-max"); if __e != nil { return }
//line form_search.gox:396
				__e = __c.Set("id", f.id + "-h"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_search.gox:396
				__e = __c.Any(f.params.Max); if __e != nil { return }
				__e = __c.Text(" chosen."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_search.gox:398
}

//line form_search.gox:400
func (f *searchWidget) results(found searchFound) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_search.gox:402
		label, labelledby := any(nil), any(f.id + "-l")
	if f.field.item != nil {
		label, _ = f.field.item.Read(ctx)
		labelledby = nil
	}

//line form_search.gox:408
		if len(found.results) > 0 {
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line form_search.gox:410
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_search.gox:411
				__e = __c.Set("class", "neisland-form-search-results"); if __e != nil { return }
//line form_search.gox:412
				__e = __c.Set("id", f.id + "-r"); if __e != nil { return }
//line form_search.gox:413
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_search.gox:414
				__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:415
				for i := range found.results {
//line form_search.gox:417
					o := found.results[i]
				key, _ := json.Marshal(o.Value)
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
//line form_search.gox:428
						__e = (neui.Use{Desc: "choose " + o.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line form_search.gox:429
								__e = __c.Modify(doors.AClick{
							Scope: f.field.scope(),
							Batch: true,
							On: func(ctx context.Context, r doors.RequestPointer) bool {
								f.pick(ctx, found.results[i])
								return false
							},
						}); if __e != nil { return }
//line form_search.gox:437
								__e = __c.Set("type", "button"); if __e != nil { return }
//line form_search.gox:438
								__e = __c.Set("class", "neisland-form-search-result"); if __e != nil { return }
//line form_search.gox:439
								__e = __c.Set("value", string(key)); if __e != nil { return }
//line form_search.gox:440
								__e = __c.Set("aria-label", "Choose " + o.Label); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:442
									__e = __c.Any(before); if __e != nil { return }
//line form_search.gox:443
									if match != "" {
										__e = __c.Init("mark"); if __e != nil { return }
										{
//line form_search.gox:444
											__e = __c.Set("class", "neisland-form-match"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line form_search.gox:444
											__e = __c.Any(match); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line form_search.gox:446
									__e = __c.Any(after); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_search.gox:453
}

func (f *searchWidget) validate(chosen []option) error {
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
