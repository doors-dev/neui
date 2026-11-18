// Managed by GoX v0.4.0

//line dashboard.gox:1
package main

import (
	"fmt"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line dashboard.gox:12
func dashboardPage() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line dashboard.gox:13
		__e = (neui.Space{
		Name: "dashboard",
		Desc: "Dashboard: this month's numbers, recent orders, products running low and a report panel",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:17
				__e = __c.Any(pageHeader{
			Title: "Dashboard",
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:19
					__e = __c.Any(time.Now().Format("Monday, January 2")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:19
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line dashboard.gox:21
					__e = (neui.Use{Desc: "open the form to create a new order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line dashboard.gox:21
							__e = __c.Modify(doors.ALink{Model: OrdersPath{View: ListNew}, Batch: true, After: scrollTop}); if __e != nil { return }
//line dashboard.gox:21
							__e = __c.Set("class", btnPrimary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:22
							__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
							__e = __c.Text("New order"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:25
			return }),
		}); if __e != nil { return }
//line dashboard.gox:27
				__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line dashboard.gox:27
					__e = __c.Any(func() any {
			_, ok := shop.version.Effect(ctx)
			if !ok {
				return nil
			}
			return dashboardBody(shop.Stats(), shop.Orders(orderQuery{}))
		}()); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line dashboard.gox:34
					__e = __c.Set("class", "mt-6"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:34
					__e = __c.Any(reportPanel()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line dashboard.gox:36
}

//line dashboard.gox:38
func dashboardBody(st stats, orders []order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line dashboard.gox:40
		recent := orders[:min(6, len(orders))]
	change := ""
	if st.RevenuePrev > 0 {
		change = fmt.Sprintf("%+.0f%% vs last month", (st.Revenue - st.RevenuePrev) / st.RevenuePrev * 100)
	}

//line dashboard.gox:46
		__e = (neui.Space{
		Name: "numbers",
		Desc: "This month's numbers; the unpaid and low stock cards link to the matching lists",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line dashboard.gox:49
				__e = __c.Set("class", "grid grid-cols-2 gap-3 @xl:gap-4 @4xl:grid-cols-4"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:50
				__e = __c.Any(neui.Data{Text: "numbers shown on the dashboard", Object: st}); if __e != nil { return }
//line dashboard.gox:51
				__e = __c.Any(kpi("Revenue this month", money(st.Revenue), change, iconMoney, nil, "")); if __e != nil { return }
//line dashboard.gox:52
				__e = __c.Any(kpi("Orders this month", fmt.Sprint(st.Orders), "not counting cancelled", iconOrders, nil, "")); if __e != nil { return }
//line dashboard.gox:53
				__e = __c.Any(kpi("Unpaid orders", fmt.Sprint(st.Unpaid), money(st.UnpaidAmount) + " outstanding", iconClock, OrdersPath{Status: string(unpaid)}, "show the unpaid orders")); if __e != nil { return }
//line dashboard.gox:54
				__e = __c.Any(kpi("Low stock", fmt.Sprint(len(st.LowStock)), fmt.Sprintf("%d or fewer units left", lowStock), iconProducts, ProductsPath{Sort: "stock"}, "show the products by stock, lowest first")); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line dashboard.gox:56
			__e = __c.Set("class", "mt-6 grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:57
			__e = (neui.Space{Name: "recent_orders", Desc: "The six newest orders, linking to each order and its customer"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line dashboard.gox:57
					__e = __c.Set("class", "@4xl:col-span-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:58
					__e = __c.Any(card{
				Title: "Recent orders",
				Flush: true,
				Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line dashboard.gox:62
						__e = (neui.Use{Desc: "go to the full orders list"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line dashboard.gox:62
								__e = __c.Modify(doors.ALink{Model: OrdersPath{}, Batch: true, After: scrollTop}); if __e != nil { return }
//line dashboard.gox:62
								__e = __c.Set("class", "text-sm font-medium text-amber-700 hover:text-amber-900"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("View all"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:65
				return }),
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
//line dashboard.gox:66
						__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("table"); if __e != nil { return }
						{
//line dashboard.gox:67
							__e = __c.Set("class", tableClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("thead"); if __e != nil { return }
							{
//line dashboard.gox:68
								__e = __c.Set("class", "bg-stone-50/60"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("tr"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line dashboard.gox:70
										__e = __c.Set("class", thClass); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Order"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line dashboard.gox:71
										__e = __c.Set("class", thClass); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Customer"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line dashboard.gox:72
										__e = __c.Set("class", thClass.Add("hidden @xl:table-cell")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Date"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line dashboard.gox:73
										__e = __c.Set("class", thClass.Add("hidden @xl:table-cell")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Status"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line dashboard.gox:74
										__e = __c.Set("class", thClass.Add("text-right")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Total"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("tbody"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:78
								for _, o := range recent {
//line dashboard.gox:79
									__e = __c.Any(orderRow(o)); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:83
				return }),
			}); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line dashboard.gox:86
			__e = (neui.Space{Name: "low_stock", Desc: "Products with few or no units left, linking to each product"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line dashboard.gox:86
				__e = __c.Any(card{
			Title: "Low stock",
			Desc: "Reorder before these run out",
			Flush: true,
			Body: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line dashboard.gox:91
					if len(st.LowStock) == 0 {
						__e = __c.Init("p"); if __e != nil { return }
						{
//line dashboard.gox:92
							__e = __c.Set("class", "px-5 py-6 text-sm text-stone-500"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Everything is well stocked."); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
						__e = __c.Init("ul"); if __e != nil { return }
						{
//line dashboard.gox:94
							__e = __c.Set("class", "divide-y divide-stone-100"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:95
							for _, p := range st.LowStock {
								__e = __c.Init("li"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:97
									__e = (neui.Use{Desc: "open product " + p.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
										__e = __c.Init("a"); if __e != nil { return }
										{
//line dashboard.gox:98
											__e = __c.Modify(doors.ALink{Model: ProductPath{ID: p.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line dashboard.gox:99
											__e = __c.Set("class", "flex items-center justify-between gap-3 px-5 py-3 transition-colors hover:bg-amber-50/40"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
											__e = __c.Init("div"); if __e != nil { return }
											{
//line dashboard.gox:100
												__e = __c.Set("class", "min-w-0"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
												__e = __c.Init("p"); if __e != nil { return }
												{
//line dashboard.gox:101
													__e = __c.Set("class", "truncate text-sm font-medium text-stone-900"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:101
													__e = __c.Any(p.Name); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
												__e = __c.Init("p"); if __e != nil { return }
												{
//line dashboard.gox:102
													__e = __c.Set("class", "text-xs text-stone-500"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:102
													__e = __c.Any(p.SKU); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:104
											__e = __c.Any(stockBadge(p.Stock)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									return })); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:110
			return }),
		}); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line dashboard.gox:113
}

//line dashboard.gox:115
func kpi(label string, value string, note string, d string, model any, desc string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line dashboard.gox:117
		body := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.InitContainer(); if __e != nil { return }
		{
			__e = __c.Init("div"); if __e != nil { return }
			{
//line dashboard.gox:118
				__e = __c.Set("class", "flex items-center justify-between"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("p"); if __e != nil { return }
				{
//line dashboard.gox:119
					__e = __c.Set("class", "text-xs font-medium text-stone-500 @xl:text-sm"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:119
					__e = __c.Any(label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line dashboard.gox:120
					__e = __c.Set("class", "flex size-8 items-center justify-center rounded-lg bg-amber-50 text-amber-700"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:120
					__e = __c.Any(icon(d, "size-5")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line dashboard.gox:122
				__e = __c.Set("class", "mt-3 text-2xl font-semibold tracking-tight text-stone-900 tabular-nums @xl:text-3xl"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:122
				__e = __c.Any(value); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("p"); if __e != nil { return }
			{
//line dashboard.gox:123
				__e = __c.Set("class", "mt-1 text-xs text-stone-500 @xl:text-sm"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:123
				__e = __c.Any(note); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line dashboard.gox:124
	return })

//line dashboard.gox:126
		if model == nil {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line dashboard.gox:127
				__e = __c.Set("class", "rounded-xl bg-white p-4 shadow-xs ring-1 ring-stone-200 @xl:p-5"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:127
				__e = __c.Any(body); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
//line dashboard.gox:129
			__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line dashboard.gox:130
					__e = __c.Modify(doors.ALink{Model: model, Batch: true, After: scrollTop}); if __e != nil { return }
//line dashboard.gox:131
					__e = __c.Set("class", "block rounded-xl bg-white p-4 shadow-xs ring-1 ring-stone-200 transition hover:ring-amber-300 @xl:p-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line dashboard.gox:132
					__e = __c.Any(body); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line dashboard.gox:135
}
