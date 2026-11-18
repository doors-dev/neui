// Managed by GoX v0.4.0

//line order.gox:1
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
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line order.gox:27
func orderPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:28
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line order.gox:28
			__e = __c.Any(func() any {
		_, ok := shop.version.Effect(ctx)
		if !ok {
			return nil
		}
		o, found := shop.Order(id)
		if !found {
			return notFound(fmt.Sprintf("Order #%d", id), crumb{Label: "Back to orders", Model: OrdersPath{}})
		}
		c, _ := shop.Customer(o.CustomerID)
		return orderView(o, c)
	}()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line order.gox:40
}

//line order.gox:42
func orderView(o order, c customerRow) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:44
		open := o.Status == paid || o.Status == unpaid

//line order.gox:46
		__e = (neui.Space{
		Name: "order",
		Desc: fmt.Sprintf("Page of order #%d: its items, shipping address, customer, payment status and refund or cancel", o.ID),
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line order.gox:50
				__e = __c.Any(neui.Data{Text: "the order on this page", Object: o}); if __e != nil { return }
//line order.gox:51
				__e = __c.Any(pageHeader{
			Crumbs: []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: fmt.Sprintf("#%d", o.ID), Model: OrderPath{ID: o.ID}}},
			Title: fmt.Sprintf("Order #%d", o.ID),
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line order.gox:55
					__e = __c.Any(badge(o.Status)); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:56
						__e = __c.Many("Placed ", o.Date); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line order.gox:57
					if o.Status == unpaid {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line order.gox:58
							__e = __c.Set("class", opt(when(overdue(o), "font-medium text-red-700"))); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line order.gox:58
							__e = __c.Many("Due ", o.Due); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:60
						__e = __c.Any(money(o.Total)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:61
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line order.gox:63
					if open {
//line order.gox:64
						__e = (neui.Use{Desc: "open the edit form of this order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line order.gox:65
								__e = __c.Modify(doors.ALink{Model: OrderPath{View: ItemEdit, ID: o.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:66
								__e = __c.Set("class", btnSecondary); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:67
								__e = __c.Any(icon(iconEdit, "size-4")); if __e != nil { return }
								__e = __c.Text("Edit order"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:71
			return }),
		}); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line order.gox:73
					__e = __c.Set("class", "grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line order.gox:74
						__e = __c.Set("class", "space-y-6 @4xl:col-span-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:75
						__e = (neui.Space{Name: "items", Desc: "Products on the order with quantities and amounts"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:76
								__e = __c.Any(card{
						Title: "Items",
						Flush: true,
						Body: orderLines(o),
					}); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
//line order.gox:82
						__e = (neui.Space{Name: "shipping", Desc: "Where the order ships"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:83
								__e = __c.Any(card{
						Title: "Shipping",
						Body: gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.InitContainer(); if __e != nil { return }
							{
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:86
									__e = __c.Set("class", "text-sm whitespace-pre-line text-stone-900"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:86
									__e = __c.Any(o.Address); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line order.gox:87
								if open {
									__e = __c.Init("p"); if __e != nil { return }
									{
//line order.gox:88
										__e = __c.Set("class", "mt-2 text-sm text-stone-500"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Wrong address? Fix it with Edit order."); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
//line order.gox:90
						return }),
					}); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line order.gox:94
						__e = __c.Set("class", "space-y-6"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:95
						__e = (neui.Space{Name: "customer", Desc: "Who placed the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line order.gox:95
							__e = __c.Any(card{
					Title: "Customer",
					Body: gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitContainer(); if __e != nil { return }
						{
//line order.gox:98
							__e = (neui.Use{Desc: "open the page of customer " + c.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line order.gox:99
									__e = __c.Modify(doors.ALink{Model: CustomerPath{ID: c.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:100
									__e = __c.Set("class", linkText); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:101
									__e = __c.Any(c.Name); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line order.gox:103
							if c.Company != "" {
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:104
									__e = __c.Set("class", "mt-0.5 text-sm text-stone-500"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:104
									__e = __c.Any(c.Company); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Init("div"); if __e != nil { return }
							{
//line order.gox:106
								__e = __c.Set("class", "mt-3 space-y-1 text-sm text-stone-600"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:107
									__e = __c.Any(c.Email); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line order.gox:108
								if c.Phone != "" {
									__e = __c.Init("p"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line order.gox:109
										__e = __c.Any(c.Phone); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("p"); if __e != nil { return }
							{
//line order.gox:112
								__e = __c.Set("class", "mt-3 text-xs text-stone-500"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:112
								__e = __c.Many(c.Orders, " order", plural(c.Orders), " · ", money(c.Spent), " paid"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line order.gox:113
					return }),
				}); if __e != nil { return }
						return })); if __e != nil { return }
//line order.gox:115
						__e = (neui.Space{Name: "actions", Desc: "Payment status and what can still be done with the order: refund a paid one, cancel an unpaid one"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:116
								__e = __c.Any(card{Title: "Actions", Body: orderActions(o)}); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line order.gox:121
}

//line order.gox:123
func orderLines(o order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line order.gox:124
			__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("table"); if __e != nil { return }
			{
//line order.gox:125
				__e = __c.Set("class", tableClass); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("thead"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("tr"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:128
							__e = __c.Set("class", thClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Product"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:129
							__e = __c.Set("class", thClass.Add("text-right")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Qty"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:130
							__e = __c.Set("class", thClass.Add("hidden text-right @xl:table-cell")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Unit price"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:131
							__e = __c.Set("class", thClass.Add("text-right")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Amount"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("tbody"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line order.gox:135
					for _, l := range o.Lines {
						__e = __c.Init("tr"); if __e != nil { return }
						{
//line order.gox:136
							__e = __c.Set("class", "border-t border-stone-100"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:137
								__e = __c.Set("class", tdClass); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:138
								__e = (neui.Use{Desc: "open product " + l.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line order.gox:138
										__e = __c.Modify(doors.ALink{Model: ProductPath{ID: l.ProductID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:138
										__e = __c.Set("class", "font-medium text-stone-900 hover:underline"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line order.gox:139
										__e = __c.Any(l.Name); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								return })); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:141
									__e = __c.Set("class", "text-xs text-stone-500"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:141
									__e = __c.Any(l.SKU); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:143
								__e = __c.Set("class", tdClass.Add("text-right tabular-nums")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:143
								__e = __c.Any(l.Qty); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:144
								__e = __c.Set("class", tdClass.Add("hidden text-right text-stone-500 tabular-nums @xl:table-cell")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:144
								__e = __c.Any(money(l.Price)); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:145
								__e = __c.Set("class", tdClass.Add("text-right tabular-nums")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:145
								__e = __c.Any(money(float64(l.Qty) * l.Price)); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("tfoot"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("tr"); if __e != nil { return }
					{
//line order.gox:150
						__e = __c.Set("class", "border-t border-stone-200 bg-stone-50/60"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:151
							__e = __c.Set("class", tdClass.Add("font-medium")); if __e != nil { return }
//line order.gox:151
							__e = __c.Set("colspan", "2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Total"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:152
							__e = __c.Set("class", tdClass.Add("hidden @xl:table-cell")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:153
							__e = __c.Set("class", tdClass.Add("text-right text-base font-semibold tabular-nums")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line order.gox:153
							__e = __c.Any(money(o.Total)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line order.gox:158
}

//line order.gox:160
func orderActions(o order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:161
		if o.Status == paid {
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Init("p"); if __e != nil { return }
				{
//line order.gox:163
					__e = __c.Set("class", "mb-3 text-sm text-stone-500"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Paid in full. A refund returns "); if __e != nil { return }
//line order.gox:163
					__e = __c.Any(money(o.Total)); if __e != nil { return }
					__e = __c.Text(" to the customer."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:164
				__e = __c.Any(confirmAction{
				Label: "Refund order",
				Icon: iconMoney,
				Desc: "start a refund of this order; Confirm refund in the confirmation completes it",
				Question: fmt.Sprintf("Refund %s to %s?", money(o.Total), o.Customer),
				Detail: "The money goes back to the customer's card. This can't be undone.",
				Confirm: "Confirm refund",
				Full: true,
				ConfirmDesc: "refund this order now; it can't be undone",
				Do: func(ctx context.Context) error {
					_, err := refundOrder(ctx, orderInput{Order: o.ID})
					return err
				},
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line order.gox:179
		} else if o.Status == unpaid {
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Init("p"); if __e != nil { return }
				{
//line order.gox:181
					__e = __c.Set("class", "mb-3 text-sm text-stone-500"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line order.gox:182
					__e = __c.Many("Not paid yet, due ", shortDate(o.Due), ". Mark it paid in Edit order, or cancel it to put the items back in stock."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:184
				if o.Reminded != "" {
					__e = __c.Init("p"); if __e != nil { return }
					{
//line order.gox:185
						__e = __c.Set("class", "mb-3 flex items-center gap-1.5 text-sm text-stone-600"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:186
						__e = __c.Any(icon(iconClock, "size-4 text-stone-400")); if __e != nil { return }
//line order.gox:187
						__e = __c.Many("Reminder sent ", shortDate(o.Reminded)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
//line order.gox:190
				__e = __c.Any(confirmAction{
				Label: "Cancel order",
				Icon: iconClose,
				Desc: "start cancelling this order; Cancel order in the confirmation completes it",
				Question: fmt.Sprintf("Cancel order #%d?", o.ID),
				Detail: "Its items go back into stock. A cancelled order can't be reopened.",
				Confirm: "Cancel order",
				Full: true,
				ConfirmDesc: "cancel this order now; it can't be undone",
				Do: func(ctx context.Context) error {
					_, err := cancelOrder(ctx, orderInput{Order: o.ID})
					return err
				},
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line order.gox:206
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("This order is "); if __e != nil { return }
//line order.gox:207
				__e = __c.Any(string(o.Status)); if __e != nil { return }
				__e = __c.Text(" and closed. Nothing more can be done with it."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line order.gox:210
}

func overdue(o order) bool {
	return o.Status == unpaid && o.Due < time.Now().Format(time.DateOnly)
}
