// Managed by GoX v0.4.0

//line ui.gox:1
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line ui.gox:13
func icon(d string, class string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line ui.gox:15
			__e = __c.Set("class", doors.Class("shrink-0", class)); if __e != nil { return }
//line ui.gox:16
			__e = __c.Set("viewBox", "0 0 24 24"); if __e != nil { return }
//line ui.gox:17
			__e = __c.Set("fill", "none"); if __e != nil { return }
//line ui.gox:18
			__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line ui.gox:19
			__e = __c.Set("stroke-width", "1.6"); if __e != nil { return }
//line ui.gox:20
			__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line ui.gox:21
				__e = __c.Set("stroke-linecap", "round"); if __e != nil { return }
//line ui.gox:21
				__e = __c.Set("stroke-linejoin", "round"); if __e != nil { return }
//line ui.gox:21
				__e = __c.Set("d", d); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:23
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

//line ui.gox:37
func (h pageHeader) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("title"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:38
			__e = __c.Many(h.Title, " · Kettle & Burr"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("header"); if __e != nil { return }
		{
//line ui.gox:39
			__e = __c.Set("class", "mb-6 flex flex-col gap-4 @xl:flex-row @xl:items-end @xl:justify-between"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:40
				__e = __c.Set("class", "min-w-0"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:41
				if len(h.Crumbs) > 0 {
					__e = __c.Init("nav"); if __e != nil { return }
					{
//line ui.gox:42
						__e = __c.Set("class", "mb-2 flex items-center gap-1 text-sm text-stone-500"); if __e != nil { return }
//line ui.gox:42
						__e = __c.Set("aria-label", "Breadcrumb"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:43
						for i, c := range h.Crumbs {
//line ui.gox:44
							if i > 0 {
//line ui.gox:45
								__e = __c.Any(icon(iconRight, "size-3.5 text-stone-400")); if __e != nil { return }
							}
//line ui.gox:47
							__e = (neui.Use{Desc: crumbDesc(i, len(h.Crumbs), c.Label)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line ui.gox:48
									__e = __c.Modify(doors.ALink{Model: c.Model, Batch: true, After: scrollTop}); if __e != nil { return }
//line ui.gox:49
									__e = __c.Set("class", "rounded hover:text-stone-900 hover:underline"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ui.gox:50
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
//line ui.gox:55
					__e = __c.Set("class", "truncate text-2xl font-semibold tracking-tight text-stone-900"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:55
					__e = __c.Any(h.Title); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line ui.gox:56
				if h.Meta != nil {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line ui.gox:57
						__e = __c.Set("class", "mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-stone-500"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:57
						__e = __c.Any(h.Meta); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:60
			if h.Actions != nil {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:61
					__e = __c.Set("class", "flex flex-wrap items-center gap-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:61
					__e = __c.Any(h.Actions); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:64
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

//line ui.gox:81
func (c card) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("section"); if __e != nil { return }
		{
//line ui.gox:82
			__e = __c.Set("class", "overflow-hidden rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:83
			if c.Title != "" {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:84
					__e = __c.Set("class", "flex flex-wrap items-center justify-between gap-3 border-b border-stone-100 px-5 py-4"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("h2"); if __e != nil { return }
						{
//line ui.gox:86
							__e = __c.Set("class", "text-base font-semibold text-stone-900"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line ui.gox:86
							__e = __c.Any(c.Title); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line ui.gox:87
						if c.Desc != "" {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line ui.gox:88
								__e = __c.Set("class", "mt-0.5 text-sm text-stone-500"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:88
								__e = __c.Any(c.Desc); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:91
					__e = __c.Any(c.Actions); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line ui.gox:94
			if c.Flush {
//line ui.gox:95
				__e = __c.Any(c.Body); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:97
					__e = __c.Set("class", "px-5 py-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:97
					__e = __c.Any(c.Body); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:100
}

//line ui.gox:102
func badge(s status) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:104
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
//line ui.gox:117
			__e = __c.Set("class", doors.Class("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium capitalize ring-1 ring-inset", tone)); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:118
				__e = __c.Set("class", doors.Class("size-1.5 rounded-full", dot)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:119
			__e = __c.Any(string(s)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:121
}

//line ui.gox:123
func stockBadge(n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:124
		if n == 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:125
				__e = __c.Set("class", "inline-flex items-center rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700 ring-1 ring-red-600/20 ring-inset"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Out of stock"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:128
		} else if n <= lowStock {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:129
				__e = __c.Set("class", "inline-flex items-center rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-800 ring-1 ring-amber-600/25 ring-inset"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Low · "); if __e != nil { return }
//line ui.gox:130
				__e = __c.Any(n); if __e != nil { return }
				__e = __c.Text(" left"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:133
				__e = __c.Set("class", "text-stone-700 tabular-nums"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:133
				__e = __c.Any(n); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line ui.gox:135
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

//line ui.gox:155
func (f field) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:157
		id := fieldID(f.Name)
	kind := f.Type
	if kind == "" {
		kind = "text"
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:164
			__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:165
				__e = __c.Set("class", "relative mt-1.5"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:166
				if f.Prefix != "" {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:167
						__e = __c.Set("class", "pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-sm text-stone-400"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:167
						__e = __c.Any(f.Prefix); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line ui.gox:169
				__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line ui.gox:170
						__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:171
						__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:172
						__e = __c.Set("type", kind); if __e != nil { return }
//line ui.gox:173
						__e = __c.Set("value", f.Value); if __e != nil { return }
//line ui.gox:174
						__e = __c.Set("placeholder", opt(f.Placeholder)); if __e != nil { return }
//line ui.gox:175
						__e = __c.Set("inputmode", opt(f.Mode)); if __e != nil { return }
//line ui.gox:176
						__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:177
						__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:178
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:179
						__e = __c.Set("class", inputBase.Add(when(f.Prefix != "", "pl-7"))); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:181
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:183
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

//line ui.gox:197
func (f textArea) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:199
		id := fieldID(f.Name)

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:202
			__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
//line ui.gox:203
			__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("textarea"); if __e != nil { return }
				{
//line ui.gox:204
					__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:205
					__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:206
					__e = __c.Set("rows", max(f.Rows, 2)); if __e != nil { return }
//line ui.gox:207
					__e = __c.Set("placeholder", opt(f.Placeholder)); if __e != nil { return }
//line ui.gox:208
					__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:209
					__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:210
					__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:211
					__e = __c.Set("class", inputBase.Add("mt-1.5")); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:212
					__e = __c.Any(f.Value); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line ui.gox:214
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:216
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

//line ui.gox:237
func (f selectField) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:239
		id := fieldID(f.Name)

		__e = __c.Init("div"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:242
			if f.Hide {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line ui.gox:243
					__e = __c.Set("for", id); if __e != nil { return }
//line ui.gox:243
					__e = __c.Set("class", "sr-only"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:243
					__e = __c.Any(f.Label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line ui.gox:245
				__e = __c.Any(fieldLabel(id, f.Label, f.Required)); if __e != nil { return }
			}
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:247
				__e = __c.Set("class", doors.Class("relative", when(!f.Hide, "mt-1.5"))); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:248
				__e = (neui.Use{Desc: f.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("select"); if __e != nil { return }
					{
//line ui.gox:249
						__e = __c.Set("id", id); if __e != nil { return }
//line ui.gox:250
						__e = __c.Set("name", f.Name); if __e != nil { return }
//line ui.gox:251
						__e = __c.Set("aria-required", ariaTrue(f.Required)); if __e != nil { return }
//line ui.gox:252
						__e = __c.Set("aria-invalid", ariaTrue(f.Error != "")); if __e != nil { return }
//line ui.gox:253
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line ui.gox:254
						__e = __c.Set("class", inputBase.Add("cursor-pointer appearance-none truncate pr-9")); if __e != nil { return }
//line ui.gox:255
						__e = __c.Modify(f.onChange()); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:256
						if f.Placeholder != "" {
							__e = __c.Init("option"); if __e != nil { return }
							{
//line ui.gox:257
								__e = __c.Set("value", ""); if __e != nil { return }
//line ui.gox:257
								__e = __c.Set("selected", f.Value == ""); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:257
								__e = __c.Any(f.Placeholder); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
//line ui.gox:259
						for _, o := range f.Options {
							__e = __c.Init("option"); if __e != nil { return }
							{
//line ui.gox:260
								__e = __c.Set("value", o.Value); if __e != nil { return }
//line ui.gox:260
								__e = __c.Set("selected", o.Value == f.Value); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:260
								__e = __c.Any(o.Label); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line ui.gox:263
					__e = __c.Set("class", "pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-stone-400"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:264
					__e = __c.Any(icon(iconUpDown, "size-4")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:267
			__e = __c.Any(fieldNote(id, f.Help, f.Error)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:269
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

//line ui.gox:286
func fieldLabel(id string, label string, required bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("label"); if __e != nil { return }
		{
//line ui.gox:287
			__e = __c.Set("for", id); if __e != nil { return }
//line ui.gox:287
			__e = __c.Set("class", "block text-sm font-medium text-stone-800"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:288
			__e = __c.Any(label); if __e != nil { return }
//line ui.gox:289
			if required {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line ui.gox:290
					__e = __c.Set("class", "text-amber-600"); if __e != nil { return }
//line ui.gox:290
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:290
					__e = __c.Any(" *"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:293
}

//line ui.gox:295
func fieldNote(id string, help string, err string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:296
		if err != "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:297
				__e = __c.Set("id", id + "-note"); if __e != nil { return }
//line ui.gox:297
				__e = __c.Set("class", "mt-1.5 flex items-start gap-1 text-sm text-red-600"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:298
				__e = __c.Any(icon(iconWarning, "mt-0.5 size-4")); if __e != nil { return }
//line ui.gox:299
				__e = __c.Any(err); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:301
		} else if help != "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:302
				__e = __c.Set("id", id + "-note"); if __e != nil { return }
//line ui.gox:302
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:302
				__e = __c.Any(help); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line ui.gox:304
}

//line ui.gox:306
func formErrors(errs fieldErrors) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:307
		if len(errs) > 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:308
				__e = __c.Set("class", "mb-6 flex gap-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-800 ring-1 ring-red-200"); if __e != nil { return }
//line ui.gox:308
				__e = __c.Set("role", "alert"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:309
				__e = __c.Any(icon(iconWarning, "size-5 text-red-500")); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:311
						__e = __c.Set("class", "font-medium"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:312
						if len(errs) == 1 {
							__e = __c.Text("Please fix the highlighted field."); if __e != nil { return }
						} else  {
//line ui.gox:315
							__e = __c.Any(fmt.Sprintf("Please fix the %d highlighted fields.", len(errs))); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:318
					if errs[""] != "" {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line ui.gox:319
							__e = __c.Set("class", "mt-1"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line ui.gox:319
							__e = __c.Any(errs[""]); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:323
			__e = __c.Any(neui.Data{Text: "The last save was refused; fix these fields (keys are field names) and save again", Object: errs}); if __e != nil { return }
		}
	return })
//line ui.gox:325
}

type pager struct {
	Info pageInfo
	Noun string
	Model func(page int) any
}

//line ui.gox:333
func (p pager) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:334
		if p.Info.Pages > 1 {
//line ui.gox:335
			__e = (neui.Space{Name: "pager", Desc: "Pages of the " + p.Noun + " table: which rows are shown and links to the other pages"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line ui.gox:335
				__e = __c.Any(p.bar()); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
//line ui.gox:337
			__e = __c.Any(p.bar()); if __e != nil { return }
		}
	return })
//line ui.gox:339
}

//line ui.gox:341
func (p pager) bar() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:342
			__e = __c.Set("class", "flex flex-col items-center justify-between gap-3 border-t border-stone-100 px-5 py-3 text-sm text-stone-600 @xl:flex-row"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:344
				if p.Info.Total == 0 {
					__e = __c.Text("No "); if __e != nil { return }
//line ui.gox:345
					__e = __c.Any(p.Noun); if __e != nil { return }
				} else  {
					__e = __c.Text("Showing"); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:348
						__e = __c.Set("class", "font-medium text-stone-900"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:348
						__e = __c.Many(" ", p.Info.From, "–", p.Info.To, " "); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Text("of"); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line ui.gox:350
						__e = __c.Set("class", "font-medium text-stone-900"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:350
						__e = __c.Many(" ", p.Info.Total, " "); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:351
					__e = __c.Any(p.Noun); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:354
			if p.Info.Pages > 1 {
				__e = __c.Init("nav"); if __e != nil { return }
				{
//line ui.gox:355
					__e = __c.Set("class", "flex items-center gap-1"); if __e != nil { return }
//line ui.gox:355
					__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:356
					__e = __c.Any(p.step(p.Info.Page - 1, iconLeft, "Previous")); if __e != nil { return }
//line ui.gox:357
					for n := 1; n <= p.Info.Pages; n++ {
//line ui.gox:358
						if n == p.Info.Page {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line ui.gox:360
								__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg bg-stone-900 text-sm font-medium text-white tabular-nums"); if __e != nil { return }
//line ui.gox:361
								__e = __c.Set("aria-current", "page"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line ui.gox:362
								__e = __c.Any(n); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						} else  {
//line ui.gox:365
							__e = (neui.Use{Desc: fmt.Sprintf("show page %d of the %s", n, p.Noun)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line ui.gox:366
									__e = __c.Modify(doors.ALink{Model: p.Model(n), Batch: true}); if __e != nil { return }
//line ui.gox:367
									__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-sm font-medium text-stone-700 tabular-nums hover:bg-stone-100"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line ui.gox:368
									__e = __c.Any(n); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
						}
					}
//line ui.gox:372
					__e = __c.Any(p.step(p.Info.Page + 1, iconRight, "Next")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:376
}

//line ui.gox:378
func (p pager) step(n int, d string, label string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:379
		if n < 1 || n > p.Info.Pages {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line ui.gox:380
				__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-stone-300"); if __e != nil { return }
//line ui.gox:380
				__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:380
				__e = __c.Any(icon(d, "size-4")); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line ui.gox:382
			__e = (neui.Use{Desc: "show the " + strings.ToLower(label) + " page of the " + p.Noun}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line ui.gox:383
					__e = __c.Modify(doors.ALink{Model: p.Model(n), Batch: true}); if __e != nil { return }
//line ui.gox:384
					__e = __c.Set("class", "inline-flex size-8 items-center justify-center rounded-lg text-stone-600 hover:bg-stone-100"); if __e != nil { return }
//line ui.gox:385
					__e = __c.Set("aria-label", label + " page"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:386
					__e = __c.Any(icon(d, "size-4")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line ui.gox:389
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

//line ui.gox:401
func (s sortHeader) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:403
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
//line ui.gox:417
			__e = __c.Set("class", thClass.Add(when(s.Right, "text-right"), s.Class)); if __e != nil { return }
//line ui.gox:418
			__e = __c.Set("aria-sort", order); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:419
			__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line ui.gox:420
					__e = __c.Modify(doors.ALink{Model: s.Model(nextSort(s.Current, s.Key, s.First)), Batch: true}); if __e != nil { return }
//line ui.gox:421
					__e = __c.Set("class", doors.Class("group inline-flex items-center gap-1 hover:text-stone-900", when(active, "text-stone-900"))); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:422
					__e = __c.Any(s.Label); if __e != nil { return }
//line ui.gox:423
					__e = __c.Any(icon(arrow, when(!active, "text-stone-300 group-hover:text-stone-500") + " size-3.5")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:426
}

type emptyState struct {
	Icon string
	Title string
	Body string
	Action gox.Elem
}

//line ui.gox:435
func (e emptyState) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:436
			__e = __c.Set("class", "flex flex-col items-center px-6 py-14 text-center"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:437
				__e = __c.Set("class", "flex size-12 items-center justify-center rounded-full bg-stone-100 text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:438
				__e = __c.Any(icon(e.Icon, "size-6")); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("h3"); if __e != nil { return }
			{
//line ui.gox:440
				__e = __c.Set("class", "mt-4 text-sm font-semibold text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:440
				__e = __c.Any(e.Title); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:441
				__e = __c.Set("class", "mt-1 max-w-sm text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:441
				__e = __c.Any(e.Body); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:442
			if e.Action != nil {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:443
					__e = __c.Set("class", "mt-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:443
					__e = __c.Any(e.Action); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:446
}

//line ui.gox:448
func notFound(what string, back crumb) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:449
		__e = __c.Any(doors.Status(404)); if __e != nil { return }
		__e = __c.Init("title"); if __e != nil { return }
		{
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Text("Not found · Kettle & Burr"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:451
			__e = __c.Set("class", "mx-auto max-w-md py-20 text-center"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:452
				__e = __c.Set("class", "text-sm font-semibold text-amber-700"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("404"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("h1"); if __e != nil { return }
			{
//line ui.gox:453
				__e = __c.Set("class", "mt-2 text-2xl font-semibold tracking-tight text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:453
				__e = __c.Any(what); if __e != nil { return }
				__e = __c.Text(" not found"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line ui.gox:454
				__e = __c.Set("class", "mt-2 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("It may have been deleted, or the link is wrong."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line ui.gox:455
			__e = __c.Any(neui.Data{Text: what + " not found: the id in the address does not exist"}); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:456
				__e = __c.Set("class", "mt-6"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:457
				__e = (neui.Use{Desc: strings.ToLower(back.Label)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("a"); if __e != nil { return }
					{
//line ui.gox:457
						__e = __c.Modify(doors.ALink{Model: back.Model, Batch: true}); if __e != nil { return }
//line ui.gox:457
						__e = __c.Set("class", btnSecondary); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:458
						__e = __c.Any(icon(iconBack, "size-4")); if __e != nil { return }
//line ui.gox:459
						__e = __c.Any(back.Label); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:463
}

//line ui.gox:465
func dl(items ...gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("dl"); if __e != nil { return }
		{
//line ui.gox:466
			__e = __c.Set("class", "divide-y divide-stone-100"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line ui.gox:467
			for _, it := range items {
//line ui.gox:468
				__e = __c.Any(it); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:471
}

//line ui.gox:473
func dlRow(label string, value any) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line ui.gox:474
			__e = __c.Set("class", "grid grid-cols-3 gap-4 py-3 first:pt-0 last:pb-0"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("dt"); if __e != nil { return }
			{
//line ui.gox:475
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:475
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("dd"); if __e != nil { return }
			{
//line ui.gox:476
				__e = __c.Set("class", "col-span-2 text-sm text-stone-900"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:476
				__e = __c.Any(value); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line ui.gox:478
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

//line ui.gox:497
func (c confirmAction) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:499
		state := doors.NewSource(confirmState{})

//line ui.gox:501
		__e = __c.Any(state.Bind(func(s confirmState) gox.Elem {
		return c.view(state, s)
	})); if __e != nil { return }
	return })
//line ui.gox:504
}

//line ui.gox:506
func (c confirmAction) view(state doors.Source[confirmState], s confirmState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line ui.gox:507
		if !s.Open {
//line ui.gox:508
			__e = (neui.Use{Desc: c.Desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line ui.gox:509
					__e = __c.Modify(doors.AClick{
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					state.Update(ctx, confirmState{Open: true})
					return false
				},
			}); if __e != nil { return }
//line ui.gox:515
					__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:516
					__e = __c.Set("class", btnDangerSoft.Add(when(c.Full, "w-full"))); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:517
					__e = __c.Any(icon(c.Icon, "size-4")); if __e != nil { return }
//line ui.gox:518
					__e = __c.Any(c.Label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line ui.gox:521
				__e = __c.Set("class", "rounded-lg bg-red-50 p-4 ring-1 ring-red-200"); if __e != nil { return }
//line ui.gox:521
				__e = __c.Set("role", "alertdialog"); if __e != nil { return }
//line ui.gox:521
				__e = __c.Set("aria-label", c.Question); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line ui.gox:522
				__e = __c.Any(neui.Data{Text: c.Wait}); if __e != nil { return }
				__e = __c.Init("p"); if __e != nil { return }
				{
//line ui.gox:523
					__e = __c.Set("class", "text-sm font-semibold text-red-900"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line ui.gox:523
					__e = __c.Any(c.Question); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line ui.gox:524
				if c.Detail != "" {
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:525
						__e = __c.Set("class", "mt-1 text-sm text-red-800"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:525
						__e = __c.Any(c.Detail); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line ui.gox:527
				if s.Err != "" {
					__e = __c.Init("p"); if __e != nil { return }
					{
//line ui.gox:528
						__e = __c.Set("class", "mt-2 text-sm font-medium text-red-700"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:528
						__e = __c.Any(s.Err); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line ui.gox:530
					__e = __c.Set("class", "mt-3 flex flex-wrap gap-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line ui.gox:532
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
//line ui.gox:543
						__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:544
						__e = __c.Set("class", btnDanger); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line ui.gox:545
						__e = __c.Any(c.Confirm); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line ui.gox:547
					__e = (neui.Use{Desc: "close the confirmation without doing it"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line ui.gox:548
							__e = __c.Modify(doors.AClick{
						On: func(ctx context.Context, _ doors.RequestPointer) bool {
							state.Update(ctx, confirmState{})
							return false
						},
					}); if __e != nil { return }
//line ui.gox:554
							__e = __c.Set("type", "button"); if __e != nil { return }
//line ui.gox:555
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
//line ui.gox:561
}
