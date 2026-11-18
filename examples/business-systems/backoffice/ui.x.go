// Managed by GoX v0.4.0

//line ui.gox:1
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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line ui.gox:27
func icon(d string, class string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line ui.gox:29
			__e = __c.Set("class", doors.Class("shrink-0", class)); if __e != nil { return }
//line ui.gox:30
			__e = __c.Set("viewBox", "0 0 24 24"); if __e != nil { return }
//line ui.gox:31
			__e = __c.Set("fill", "none"); if __e != nil { return }
//line ui.gox:32
			__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line ui.gox:33
			__e = __c.Set("stroke-width", "1.6"); if __e != nil { return }
//line ui.gox:34
			__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line ui.gox:35
				__e = __c.Set("stroke-linecap", "round"); if __e != nil { return }
//line ui.gox:35
				__e = __c.Set("stroke-linejoin", "round"); if __e != nil { return }
//line ui.gox:35
				__e = __c.Set("d", d); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:37
}

type crumb struct {
	Label string
	Model any
}

type pageHeader struct {
	Crumbs []crumb
	Title string
	Meta gox.Elem
	Actions gox.Elem
}

//line ui.gox:51
func (h pageHeader) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("title"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:52
			__e = __c.Many(h.Title, " · Kettle & Burr"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("header"); if __e != nil { return }
		{
//line ui.gox:53
			__e = __c.Set("class", "mb-6 flex flex-col gap-4 @xl:flex-row @xl:items-end @xl:justify-between"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:54
				__e = __c.Set("class", "min-w-0"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:55
				if len(h.Crumbs) > 0 {
					__e = __c.Init("nav"); if __e != nil { return }
					{
//line ui.gox:56
						__e = __c.Set("class", "mb-2 flex items-center gap-1 text-sm text-stone-500"); if __e != nil { return }
//line ui.gox:56
						__e = __c.Set("aria-label", "Breadcrumb"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:57
						for i, c := range h.Crumbs {
//line ui.gox:58
							if i > 0 {
//line ui.gox:59
								__e = __c.Any(icon(iconRight, "size-3.5 text-stone-400")); if __e != nil { return }
							}
//line ui.gox:61
							__e = (neui.Use{Desc: crumbDesc(i, len(h.Crumbs), c.Label)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line ui.gox:62
									__e = __c.Modify(doors.ALink{Model: c.Model, Batch: true, After: scrollTop}); if __e != nil { return }
//line ui.gox:63
									__e = __c.Set("class", "rounded hover:text-stone-900 hover:underline"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ui.gox:64
									__e = __c.Any(c.Label); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("h1"); if __e != nil { return }
				{
//line ui.gox:69
					__e = __c.Set("class", "truncate text-2xl font-semibold tracking-tight text-stone-900"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:69
					__e = __c.Any(h.Title); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line ui.gox:70
				if h.Meta != nil {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line ui.gox:71
						__e = __c.Set("class", "mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-stone-500"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:71
						__e = __c.Any(h.Meta); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:74
			if h.Actions != nil {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:75
					__e = __c.Set("class", "flex flex-wrap items-center gap-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:75
					__e = __c.Any(h.Actions); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:78
}

func crumbDesc(i int, n int, label string) string {
	if i == n - 1 {
		return "breadcrumb to this page, " + label
	}
	return "breadcrumb back to " + label
}

type card struct {
	Title string
	Desc string
	Actions gox.Elem
	Body gox.Elem
	Flush bool
}

//line ui.gox:95
func (c card) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("section"); if __e != nil { return }
		{
//line ui.gox:96
			__e = __c.Set("class", "overflow-hidden rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:97
			if c.Title != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:98
					__e = __c.Set("class", "flex flex-wrap items-center justify-between gap-3 border-b border-stone-100 px-5 py-4"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("h2"); if __e != nil { return }
						{
//line ui.gox:100
							__e = __c.Set("class", "text-base font-semibold text-stone-900"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line ui.gox:100
							__e = __c.Any(c.Title); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line ui.gox:101
						if c.Desc != "" {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line ui.gox:102
								__e = __c.Set("class", "mt-0.5 text-sm text-stone-500"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:102
								__e = __c.Any(c.Desc); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:105
					__e = __c.Any(c.Actions); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line ui.gox:108
			if c.Flush {
//line ui.gox:109
				__e = __c.Any(c.Body); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:111
					__e = __c.Set("class", "px-5 py-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:111
					__e = __c.Any(c.Body); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:114
}

//line ui.gox:116
func badge(s status) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:118
		tone := map[status]string{
		paid: "bg-emerald-50 text-emerald-700 ring-emerald-600/20",
		unpaid: "bg-amber-50 text-amber-800 ring-amber-600/25",
		refunded: "bg-sky-50 text-sky-700 ring-sky-600/20",
		cancelled: "bg-stone-100 text-stone-600 ring-stone-500/20",
	}[s]
	dot := map[status]string{
		paid: "bg-emerald-500",
		unpaid: "bg-amber-500",
		refunded: "bg-sky-500",
		cancelled: "bg-stone-400",
	}[s]

		__e = __c.Init("span"); if __e != nil { return }
		{
//line ui.gox:131
			__e = __c.Set("class", doors.Class("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium capitalize ring-1 ring-inset", tone)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:132
				__e = __c.Set("class", doors.Class("size-1.5 rounded-full", dot)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:133
			__e = __c.Any(string(s)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:135
}

//line ui.gox:137
func stockBadge(n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:138
		if n == 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:139
				__e = __c.Set("class", "inline-flex items-center rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700 ring-1 ring-red-600/20 ring-inset"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Out of stock"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:142
		} else if n <= lowStock {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:143
				__e = __c.Set("class", "inline-flex items-center rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-800 ring-1 ring-amber-600/25 ring-inset"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Low · "); if __e != nil { return }
//line ui.gox:144
				__e = __c.Any(n); if __e != nil { return }
				__e = __c.Text(" left"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:147
				__e = __c.Set("class", "text-stone-700 tabular-nums"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:147
				__e = __c.Any(n); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line ui.gox:149
}

type field struct {
	Label string
	Name string
	Value string
	Type string
	Placeholder string
	Help string
	Error string
	Desc string
	Required bool
	Prefix string
	Mode string
}

func fieldID(name string) string {
	return "f-" + strings.NewReplacer("[", "-", "]", "", ".", "-").Replace(name)
}

//line ui.gox:169
func (f field) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:171
		id := fieldID(f.Name)
	kind := f.Type
	if kind == "" {
		kind = "text"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:178
			__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:179
				__e = __c.Set("class", "relative mt-1.5"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:180
				if f.Prefix != "" {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:181
						__e = __c.Set("class", "pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-sm text-stone-400"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:181
						__e = __c.Any(f.Prefix); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line ui.gox:183
				__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line ui.gox:184
						__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:185
						__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:186
						__e = __c.Set("type", kind); if __e != nil { return }
//line ui.gox:187
						__e = __c.Set("value", f.Value); if __e != nil { return }
//line ui.gox:188
						__e = __c.Set("placeholder", opt(f.Placeholder)); if __e != nil { return }
//line ui.gox:189
						__e = __c.Set("inputmode", opt(f.Mode)); if __e != nil { return }
//line ui.gox:190
						__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:191
						__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:192
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:193
						__e = __c.Set("class", inputBase.Add(when(f.Prefix != "", "pl-7"))); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:195
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:197
}

type textArea struct {
	Label string
	Name string
	Value string
	Placeholder string
	Help string
	Error string
	Desc string
	Required bool
	Rows int
}

//line ui.gox:211
func (f textArea) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:213
		id := fieldID(f.Name)

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:216
			__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
//line ui.gox:217
			__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line ui.gox:218
					__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:219
					__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:220
					__e = __c.Set("rows", max(f.Rows, 2)); if __e != nil { return }
//line ui.gox:221
					__e = __c.Set("placeholder", opt(f.Placeholder)); if __e != nil { return }
//line ui.gox:222
					__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:223
					__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:224
					__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:225
					__e = __c.Set("class", inputBase.Add("mt-1.5")); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:226
					__e = __c.Any(f.Value); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line ui.gox:228
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:230
}

type option struct {
	Value string
	Label string
}

type selectField struct {
	Label string
	Name string
	Value string
	Options []option
	Placeholder string
	Help string
	Error string
	Desc string
	Required bool
	Hide bool
	Change func(ctx context.Context, value string)
}

//line ui.gox:251
func (f selectField) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:253
		id := fieldID(f.Name)

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:256
			if f.Hide {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line ui.gox:257
					__e = __c.Set("for", id); if __e != nil { return }
//line ui.gox:257
					__e = __c.Set("class", "sr-only"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:257
					__e = __c.Any(f.Label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line ui.gox:259
				__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:261
				__e = __c.Set("class", doors.Class("relative", when(!f.Hide, "mt-1.5"))); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:262
				__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("select"); if __e != nil { return }
					{
//line ui.gox:263
						__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:264
						__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:265
						__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:266
						__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:267
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:268
						__e = __c.Set("class", inputBase.Add("cursor-pointer appearance-none truncate pr-9")); if __e != nil { return }
//line ui.gox:269
						__e = __c.Modify(f.onChange()); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:270
						if f.Placeholder != "" {
							__e = __c.Init("option"); if __e != nil { return }
							{
//line ui.gox:271
								__e = __c.Set("value", ""); if __e != nil { return }
//line ui.gox:271
								__e = __c.Set("selected", f.Value == ""); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:271
								__e = __c.Any(f.Placeholder); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
//line ui.gox:273
						for _, o := range f.Options {
							__e = __c.Init("option"); if __e != nil { return }
							{
//line ui.gox:274
								__e = __c.Set("value", o.Value); if __e != nil { return }
//line ui.gox:274
								__e = __c.Set("selected", o.Value == f.Value); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:274
								__e = __c.Any(o.Label); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line ui.gox:277
					__e = __c.Set("class", "pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-stone-400"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:278
					__e = __c.Any(icon(iconUpDown, "size-4")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:281
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:283
}

func (f selectField) onChange() gox.Modify {
	if f.Change == nil {
		return gox.ModifyFunc(func(context.Context, string, gox.Attrs) error {
			return nil
		})
	}
	return doors.AChange{
		Race: doors.RaceLatest,
		On: func(ctx context.Context, r doors.RequestChange) bool {
			f.Change(ctx, r.Event().Value)
			return false
		},
	}
}

//line ui.gox:300
func fieldLabel(id string, label string, required bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("label"); if __e != nil { return }
		{
//line ui.gox:301
			__e = __c.Set("for", id); if __e != nil { return }
//line ui.gox:301
			__e = __c.Set("class", "block text-sm font-medium text-stone-800"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:302
			__e = __c.Any(label); if __e != nil { return }
//line ui.gox:303
			if required {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line ui.gox:304
					__e = __c.Set("class", "text-amber-600"); if __e != nil { return }
//line ui.gox:304
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:304
					__e = __c.Any(" *"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:307
}

//line ui.gox:309
func fieldNote(id string, help string, err string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:310
		if err != "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:311
				__e = __c.Set("id", id + "-note"); if __e != nil { return }
//line ui.gox:311
				__e = __c.Set("class", "mt-1.5 flex items-start gap-1 text-sm text-red-600"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:312
				__e = __c.Any(icon(iconWarning, "mt-0.5 size-4")); if __e != nil { return }
//line ui.gox:313
				__e = __c.Any(err); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:315
		} else if help != "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:316
				__e = __c.Set("id", id + "-note"); if __e != nil { return }
//line ui.gox:316
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:316
				__e = __c.Any(help); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line ui.gox:318
}

//line ui.gox:320
func formErrors(errs fieldErrors) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:321
		if len(errs) > 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:322
				__e = __c.Set("class", "mb-6 flex gap-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-800 ring-1 ring-red-200"); if __e != nil { return }
//line ui.gox:322
				__e = __c.Set("role", "alert"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:323
				__e = __c.Any(icon(iconWarning, "size-5 text-red-500")); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:325
						__e = __c.Set("class", "font-medium"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:326
						if len(errs) == 1 {
							__e = __c.Text("Please fix the highlighted field."); if __e != nil { return }
						} else  {
//line ui.gox:329
							__e = __c.Any(fmt.Sprintf("Please fix the %d highlighted fields.", len(errs))); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:332
					if errs[""] != "" {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line ui.gox:333
							__e = __c.Set("class", "mt-1"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line ui.gox:333
							__e = __c.Any(errs[""]); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:337
			__e = __c.Any(neui.Data{Text: "The last save was refused; fix these fields (keys are field names) and save again", Object: errs}); if __e != nil { return }
		}
	return })
//line ui.gox:339
}

type pager struct {
	Info pageInfo
	Noun string
	Model func(page int) any
}

//line ui.gox:347
func (p pager) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:348
		if p.Info.Pages > 1 {
//line ui.gox:349
			__e = (neui.Space{Name: "pager", Desc: "Pages of the " + p.Noun + " table: which rows are shown and links to the other pages"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line ui.gox:349
				__e = __c.Any(p.bar()); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
//line ui.gox:351
			__e = __c.Any(p.bar()); if __e != nil { return }
		}
	return })
//line ui.gox:353
}

//line ui.gox:355
func (p pager) bar() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:356
			__e = __c.Set("class", "flex flex-col items-center justify-between gap-3 border-t border-stone-100 px-5 py-3 text-sm text-stone-600 @xl:flex-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:358
				if p.Info.Total == 0 {
					__e = __c.Text("No "); if __e != nil { return }
//line ui.gox:359
					__e = __c.Any(p.Noun); if __e != nil { return }
				} else  {
					__e = __c.Text("Showing"); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:362
						__e = __c.Set("class", "font-medium text-stone-900"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:362
						__e = __c.Many(" ", p.Info.From, "–", p.Info.To, " "); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Text("of"); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:364
						__e = __c.Set("class", "font-medium text-stone-900"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:364
						__e = __c.Many(" ", p.Info.Total, " "); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:365
					__e = __c.Any(p.Noun); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:368
			if p.Info.Pages > 1 {
				__e = __c.Init("nav"); if __e != nil { return }
				{
//line ui.gox:369
					__e = __c.Set("class", "flex items-center gap-1"); if __e != nil { return }
//line ui.gox:369
					__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:370
					__e = __c.Any(p.step(p.Info.Page - 1, iconLeft, "Previous")); if __e != nil { return }
//line ui.gox:371
					for n := 1; n <= p.Info.Pages; n++ {
//line ui.gox:372
						if n == p.Info.Page {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line ui.gox:374
								__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg bg-stone-900 text-sm font-medium text-white tabular-nums"); if __e != nil { return }
//line ui.gox:375
								__e = __c.Set("aria-current", "page"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:376
								__e = __c.Any(n); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						} else  {
//line ui.gox:379
							__e = (neui.Use{Desc: fmt.Sprintf("show page %d of the %s", n, p.Noun)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line ui.gox:380
									__e = __c.Modify(doors.ALink{Model: p.Model(n), Batch: true}); if __e != nil { return }
//line ui.gox:381
									__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-sm font-medium text-stone-700 tabular-nums hover:bg-stone-100"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ui.gox:382
									__e = __c.Any(n); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
					}
//line ui.gox:386
					__e = __c.Any(p.step(p.Info.Page + 1, iconRight, "Next")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:390
}

//line ui.gox:392
func (p pager) step(n int, d string, label string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:393
		if n < 1 || n > p.Info.Pages {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:394
				__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-stone-300"); if __e != nil { return }
//line ui.gox:394
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:394
				__e = __c.Any(icon(d, "size-4")); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line ui.gox:396
			__e = (neui.Use{Desc: "show the " + strings.ToLower(label) + " page of the " + p.Noun}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line ui.gox:397
					__e = __c.Modify(doors.ALink{Model: p.Model(n), Batch: true}); if __e != nil { return }
//line ui.gox:398
					__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-stone-600 hover:bg-stone-100"); if __e != nil { return }
//line ui.gox:399
					__e = __c.Set("aria-label", label + " page"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:400
					__e = __c.Any(icon(d, "size-4")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line ui.gox:403
}

type sortHeader struct {
	Label string
	Key string
	First string
	Current string
	Right bool
	Class string
	Model func(sort string) any
}

//line ui.gox:415
func (s sortHeader) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:417
		key := strings.TrimPrefix(s.Current, "-")
	active := key == s.Key
	arrow := iconUpDown
	var order any
	desc := "sort the table by " + strings.ToLower(s.Label)
	if active && strings.HasPrefix(s.Current, "-") {
		arrow, order = iconDown, "descending"
		desc += ", now descending; press again for ascending"
	} else if active {
		arrow, order = iconUp, "ascending"
		desc += ", now ascending; press again for descending"
	}

		__e = __c.Init("th"); if __e != nil { return }
		{
//line ui.gox:431
			__e = __c.Set("class", thClass.Add(when(s.Right, "text-right"), s.Class)); if __e != nil { return }
//line ui.gox:432
			__e = __c.Set("aria-sort", order); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:433
			__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line ui.gox:434
					__e = __c.Modify(doors.ALink{Model: s.Model(nextSort(s.Current, s.Key, s.First)), Batch: true}); if __e != nil { return }
//line ui.gox:435
					__e = __c.Set("class", doors.Class("group inline-flex items-center gap-1 hover:text-stone-900", when(active, "text-stone-900"))); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:436
					__e = __c.Any(s.Label); if __e != nil { return }
//line ui.gox:437
					__e = __c.Any(icon(arrow, when(!active, "text-stone-300 group-hover:text-stone-500") + " size-3.5")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:440
}

type emptyState struct {
	Icon string
	Title string
	Body string
	Action gox.Elem
}

//line ui.gox:449
func (e emptyState) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:450
			__e = __c.Set("class", "flex flex-col items-center px-6 py-14 text-center"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:451
				__e = __c.Set("class", "flex size-12 items-center justify-center rounded-full bg-stone-100 text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:452
				__e = __c.Any(icon(e.Icon, "size-6")); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("h3"); if __e != nil { return }
			{
//line ui.gox:454
				__e = __c.Set("class", "mt-4 text-sm font-semibold text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:454
				__e = __c.Any(e.Title); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:455
				__e = __c.Set("class", "mt-1 max-w-sm text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:455
				__e = __c.Any(e.Body); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:456
			if e.Action != nil {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:457
					__e = __c.Set("class", "mt-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:457
					__e = __c.Any(e.Action); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:460
}

//line ui.gox:462
func notFound(what string, back crumb) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:463
		__e = __c.Any(doors.Status(404)); if __e != nil { return }
		__e = __c.Init("title"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Text("Not found · Kettle & Burr"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:465
			__e = __c.Set("class", "mx-auto max-w-md py-20 text-center"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:466
				__e = __c.Set("class", "text-sm font-semibold text-amber-700"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("404"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("h1"); if __e != nil { return }
			{
//line ui.gox:467
				__e = __c.Set("class", "mt-2 text-2xl font-semibold tracking-tight text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:467
				__e = __c.Any(what); if __e != nil { return }
				__e = __c.Text(" not found"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:468
				__e = __c.Set("class", "mt-2 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("It may have been deleted, or the link is wrong."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:469
			__e = __c.Any(neui.Data{Text: what + " not found: the id in the address does not exist"}); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:470
				__e = __c.Set("class", "mt-6"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:471
				__e = (neui.Use{Desc: strings.ToLower(back.Label)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("a"); if __e != nil { return }
					{
//line ui.gox:471
						__e = __c.Modify(doors.ALink{Model: back.Model, Batch: true}); if __e != nil { return }
//line ui.gox:471
						__e = __c.Set("class", btnSecondary); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:472
						__e = __c.Any(icon(iconBack, "size-4")); if __e != nil { return }
//line ui.gox:473
						__e = __c.Any(back.Label); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:477
}

//line ui.gox:479
func dl(items ...gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("dl"); if __e != nil { return }
		{
//line ui.gox:480
			__e = __c.Set("class", "divide-y divide-stone-100"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:481
			for _, it := range items {
//line ui.gox:482
				__e = __c.Any(it); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:485
}

//line ui.gox:487
func dlRow(label string, value any) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:488
			__e = __c.Set("class", "grid grid-cols-3 gap-4 py-3 first:pt-0 last:pb-0"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("dt"); if __e != nil { return }
			{
//line ui.gox:489
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:489
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("dd"); if __e != nil { return }
			{
//line ui.gox:490
				__e = __c.Set("class", "col-span-2 text-sm text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:490
				__e = __c.Any(value); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:492
}

type confirmAction struct {
	Label string
	Icon string
	Desc string
	Question string
	Detail string
	Confirm string
	Wait string
	Full bool
	Do func(ctx context.Context) error
}

type confirmState struct {
	Open bool
	Err string
}

//line ui.gox:511
func (c confirmAction) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:513
		state := doors.NewSource(confirmState{})

//line ui.gox:515
		__e = __c.Any(state.Bind(func(s confirmState) gox.Elem {
		return c.view(state, s)
	})); if __e != nil { return }
	return })
//line ui.gox:518
}

//line ui.gox:520
func (c confirmAction) view(state doors.Source[confirmState], s confirmState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:521
		if !s.Open {
//line ui.gox:522
			__e = (neui.Use{Desc: c.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line ui.gox:523
					__e = __c.Modify(doors.AClick{
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					state.Update(ctx, confirmState{Open: true})
					return false
				},
			}); if __e != nil { return }
//line ui.gox:529
					__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:530
					__e = __c.Set("class", btnDangerSoft.Add(when(c.Full, "w-full"))); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:531
					__e = __c.Any(icon(c.Icon, "size-4")); if __e != nil { return }
//line ui.gox:532
					__e = __c.Any(c.Label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:535
				__e = __c.Set("class", "rounded-lg bg-red-50 p-4 ring-1 ring-red-200"); if __e != nil { return }
//line ui.gox:535
				__e = __c.Set("role", "alertdialog"); if __e != nil { return }
//line ui.gox:535
				__e = __c.Set("aria-label", c.Question); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:536
				__e = __c.Any(neui.Data{Text: c.Wait}); if __e != nil { return }
				__e = __c.Init("p"); if __e != nil { return }
				{
//line ui.gox:537
					__e = __c.Set("class", "text-sm font-semibold text-red-900"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:537
					__e = __c.Any(c.Question); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line ui.gox:538
				if c.Detail != "" {
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:539
						__e = __c.Set("class", "mt-1 text-sm text-red-800"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:539
						__e = __c.Any(c.Detail); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line ui.gox:541
				if s.Err != "" {
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:542
						__e = __c.Set("class", "mt-2 text-sm font-medium text-red-700"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:542
						__e = __c.Any(s.Err); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:544
					__e = __c.Set("class", "mt-3 flex flex-wrap gap-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line ui.gox:546
						__e = __c.Modify(doors.AClick{
						Scope: &doors.ScopeBlocking{},
						Indicator: doors.IndicateAttr("aria-busy", "true"),
						Batch: true,
						On: func(ctx context.Context, _ doors.RequestPointer) bool {
							if err := c.Do(ctx); err != nil {
								state.Update(ctx, confirmState{Open: true, Err: err.Error()})
							}
							return false
						},
					}); if __e != nil { return }
//line ui.gox:557
						__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:558
						__e = __c.Set("class", btnDanger); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:559
						__e = __c.Any(c.Confirm); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:561
					__e = (neui.Use{Desc: "close the confirmation without doing it"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line ui.gox:562
							__e = __c.Modify(doors.AClick{
						On: func(ctx context.Context, _ doors.RequestPointer) bool {
							state.Update(ctx, confirmState{})
							return false
						},
					}); if __e != nil { return }
//line ui.gox:568
							__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:569
							__e = __c.Set("class", btnSecondary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Keep it"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line ui.gox:575
}
