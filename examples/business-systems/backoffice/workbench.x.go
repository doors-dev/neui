// Managed by GoX v0.4.0

//line workbench.gox:1
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

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line workbench.gox:25
func workbenchPanel() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line workbench.gox:26
		__e = __c.Any(neui.Island{
		Name: "workbench",
		Desc: "Apps over the shop's data. Their scripts read orders, customers and products, and by id can create orders, customers and products, change an order's items and quantities, status, shipping address and due date, send payment reminders, refund or cancel orders, and update customers, products and stock. Build here what the user needs beyond the app UI, such as reviewing and changing many records, comparisons or calculators. It shows as a panel over the page only while an app runs; the user can hide it and bring it back. Its app space has the same changes as tools you can call directly.",
		Placeholder: gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
			}
			__e = __c.Close(); if __e != nil { return }
//line workbench.gox:29
		return }),
		Template: func(slot gox.Comp) gox.Elem {
			return workbenchFrame(slot)
		},
		Library: workbenchLibrary,
	}); if __e != nil { return }
	return })
//line workbench.gox:35
}

//line workbench.gox:37
func workbenchFrame(slot gox.Comp) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line workbench.gox:39
		shown := doors.NewSource(true)
	panel := new(doors.Setter)
	pill := new(doors.Setter)
	open, _ := shown.ReadAndSub(ctx, func(ctx context.Context, v bool) bool {
		doors.Call(ctx, panel.Set("data-open", v))
		doors.Call(ctx, pill.Set("data-open", v))
		return false
	})
	show := func(v bool) doors.AClick {
		return doors.AClick{
			On: func(ctx context.Context, _ doors.RequestPointer) bool {
				shown.Update(ctx, v)
				return false
			},
		}
	}

		__e = __c.Init("section"); if __e != nil { return }
		{
//line workbench.gox:57
			__e = __c.Modify(panel); if __e != nil { return }
//line workbench.gox:58
			__e = __c.Set("data-open", open); if __e != nil { return }
//line workbench.gox:59
			__e = __c.Set("id", "workbench"); if __e != nil { return }
//line workbench.gox:60
			__e = __c.Set("aria-labelledby", "workbench-title"); if __e != nil { return }
//line workbench.gox:61
			__e = __c.Set("class", "fixed inset-x-4 top-16 z-40 hidden max-h-[calc(100dvh-5rem)] flex-col overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-stone-200 data-open:flex side:top-6 side:left-72 side:max-h-[calc(100dvh-3rem)] docked:right-[29rem]"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("header"); if __e != nil { return }
			{
//line workbench.gox:62
				__e = __c.Set("class", "flex items-start justify-between gap-4 border-b border-stone-100 px-5 py-4"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("h2"); if __e != nil { return }
					{
//line workbench.gox:64
						__e = __c.Set("id", "workbench-title"); if __e != nil { return }
//line workbench.gox:64
						__e = __c.Set("class", "flex items-center gap-2 text-base font-semibold text-stone-900"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line workbench.gox:65
						__e = __c.Any(icon(iconSparkles, "size-5 text-amber-600")); if __e != nil { return }
						__e = __c.Text("Workbench"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line workbench.gox:68
						__e = __c.Set("class", "mt-0.5 text-sm text-stone-500"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("An app the assistant built for you, on live shop data."); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line workbench.gox:70
				__e = (neui.Use{Desc: "hide the Workbench panel so the page is visible; the app keeps running"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line workbench.gox:71
						__e = __c.Modify(show(false)); if __e != nil { return }
//line workbench.gox:72
						__e = __c.Set("type", "button"); if __e != nil { return }
//line workbench.gox:73
						__e = __c.Set("aria-label", "Hide the Workbench"); if __e != nil { return }
//line workbench.gox:74
						__e = __c.Set("class", btnGhost.Add("-mr-2 px-2")); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line workbench.gox:75
						__e = __c.Any(icon(iconMinus, "size-5")); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line workbench.gox:78
				__e = __c.Set("class", "overflow-y-auto px-5 py-5"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line workbench.gox:78
				__e = __c.Any(slot); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line workbench.gox:81
			__e = __c.Modify(pill); if __e != nil { return }
//line workbench.gox:82
			__e = __c.Set("data-open", open); if __e != nil { return }
//line workbench.gox:83
			__e = __c.Set("class", "pointer-events-none fixed inset-x-4 top-16 z-40 flex justify-center data-open:hidden side:top-6 side:left-72 docked:right-[29rem]"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line workbench.gox:84
			__e = (neui.Use{Desc: "show the hidden Workbench panel with the running app"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line workbench.gox:85
					__e = __c.Modify(show(true)); if __e != nil { return }
//line workbench.gox:86
					__e = __c.Set("type", "button"); if __e != nil { return }
//line workbench.gox:87
					__e = __c.Set("class", "pointer-events-auto inline-flex cursor-pointer items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-medium text-stone-800 shadow-lg ring-1 ring-stone-200 transition-colors hover:bg-amber-50 hover:text-stone-900 hover:ring-amber-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-500 active:bg-amber-100"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line workbench.gox:88
					__e = __c.Any(icon(iconSparkles, "size-4 text-amber-600")); if __e != nil { return }
					__e = __c.Text("Show Workbench"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line workbench.gox:92
}
