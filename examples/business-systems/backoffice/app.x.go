// Managed by GoX v0.4.0

//line app.gox:1
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

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/examples/business-systems/backoffice/css"
	"neui.dev/nekit"
)

type App struct {
	Threads nekit.SQLiteMessages
}

type openInput struct {
	ID int `json:"id" required:"true" description:"id, like 1042 for an order or 7 for a customer"`
}

func openOrder(ctx context.Context, in openInput) (order, error) {
	o, ok := shop.Order(in.ID)
	if !ok {
		return order{}, fmt.Errorf("no order %d", in.ID)
	}
	navigate(ctx, OrderPath{ID: in.ID})
	return o, nil
}

func openCustomer(ctx context.Context, in openInput) (customerRow, error) {
	c, ok := shop.Customer(in.ID)
	if !ok {
		return customerRow{}, fmt.Errorf("no customer %d", in.ID)
	}
	navigate(ctx, CustomerPath{ID: in.ID})
	return c, nil
}

//line app.gox:54
func (a App) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:56
		if err := neui.Enable(ctx, neui.OptionMessageStorage(a.Threads.User("demo"))); err != nil {
		doors.Logger(ctx).Error("neui enable", "err", err)
	}

		__e = __c.Raw("<!doctype html>"); if __e != nil { return }
		__e = __c.Init("html"); if __e != nil { return }
		{
//line app.gox:61
			__e = __c.Set("lang", "en"); if __e != nil { return }
//line app.gox:61
			__e = __c.Set("data-theme", "light"); if __e != nil { return }
//line app.gox:61
			__e = __c.Set("class", "bg-stone-50"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("head"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("meta"); if __e != nil { return }
				{
//line app.gox:63
					__e = __c.Set("charset", "utf-8"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("meta"); if __e != nil { return }
				{
//line app.gox:64
					__e = __c.Set("name", "viewport"); if __e != nil { return }
//line app.gox:64
					__e = __c.Set("content", "width=device-width, initial-scale=1"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("title"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Kettle & Burr"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.InitVoid("link"); if __e != nil { return }
				{
//line app.gox:66
					__e = __c.Set("rel", "stylesheet"); if __e != nil { return }
//line app.gox:66
					__e = __c.Set("href", css.Style); if __e != nil { return }
//line app.gox:66
					__e = __c.Set("name", "backoffice.css"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("body"); if __e != nil { return }
			{
//line app.gox:68
				__e = __c.Set("class", "bg-stone-50"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line app.gox:69
					__e = __c.Set("id", "app"); if __e != nil { return }
//line app.gox:69
					__e = __c.Set("class", "min-h-dvh text-stone-900 antialiased"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line app.gox:70
						__e = __c.Set("id", "top"); if __e != nil { return }
//line app.gox:70
						__e = __c.Set("class", "side:pl-64"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line app.gox:71
						__e = __c.Any(sidebar()); if __e != nil { return }
//line app.gox:72
						__e = __c.Any(operations()); if __e != nil { return }
//line app.gox:73
						__e = __c.Any(neui.Tool[openInput, order]{
						Name: "open_order",
						Desc: "Open the page of an order for the user",
						Handler: openOrder,
					}); if __e != nil { return }
//line app.gox:78
						__e = __c.Any(neui.Tool[openInput, customerRow]{
						Name: "open_customer",
						Desc: "Open the page of a customer for the user",
						Handler: openCustomer,
					}); if __e != nil { return }
						__e = __c.Init("main"); if __e != nil { return }
						{
//line app.gox:83
							__e = __c.Set("class", "@container mx-auto max-w-6xl px-4 pt-6 pb-24 sm:px-6 side:px-10 side:pt-10"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line app.gox:84
							__e = __c.Any(routes()); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line app.gox:87
					__e = __c.Any(workbenchPanel()); if __e != nil { return }
//line app.gox:88
					__e = __c.Any(toast()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line app.gox:90
				__e = __c.Any(nekit.Chat{ThemeLight: chatTheme}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line app.gox:93
}

//line app.gox:95
func routes() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:96
		__e = __c.Any(doors.Route(
		doors.RouteModel[HomePath](doors.RouteComp[HomePath](dashboardPage())),
		doors.RouteModel[OrdersPath](
			doors.RouteMatch(func(p OrdersPath) bool {
				return p.View == ListNew
			}, doors.RouteBind(func(p OrdersPath) gox.Elem {
				return orderCreatePage(p.Customer)
			})),
			doors.RouteSource(ordersPage),
		),
		doors.RouteModel[OrderPath](
			doors.RouteMatch(func(p OrderPath) bool {
				return p.View == ItemEdit
			}, doors.RouteBind(func(p OrderPath) gox.Elem {
				return orderEditPage(p.ID)
			})),
			doors.RouteBind(func(p OrderPath) gox.Elem {
				return orderPage(p.ID)
			}),
		),
		doors.RouteModel[CustomersPath](
			doors.RouteMatch(func(p CustomersPath) bool {
				return p.View == ListNew
			}, doors.RouteComp[CustomersPath](customerFormPage(0))),
			doors.RouteSource(customersPage),
		),
		doors.RouteModel[CustomerPath](
			doors.RouteMatch(func(p CustomerPath) bool {
				return p.View == ItemEdit
			}, doors.RouteBind(func(p CustomerPath) gox.Elem {
				return customerFormPage(p.ID)
			})),
			doors.RouteSource(customerPage),
		),
		doors.RouteModel[ProductsPath](
			doors.RouteMatch(func(p ProductsPath) bool {
				return p.View == ListNew
			}, doors.RouteComp[ProductsPath](productFormPage(0))),
			doors.RouteSource(productsPage),
		),
		doors.RouteModel[ProductPath](doors.RouteBind(func(p ProductPath) gox.Elem {
			return productFormPage(p.ID)
		})),
		doors.RouteComp[doors.Location](notFound("Page", crumb{Label: "Back to the dashboard", Model: HomePath{}})),
	)); if __e != nil { return }
	return })
//line app.gox:141
}

//line app.gox:143
func operations() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:144
		__e = (neui.Space{
		Name: "operations",
		Desc: "Every change the agent can make in the store, by id, available on every page.",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line app.gox:148
				__e = __c.Any(neui.Tool[createOrderInput, created]{
			Name: "create_order",
			Desc: "Create an order, checked like the New order form; stock is taken when it is created. Returns its id; open_order shows it to the user",
			Handler: createOrder,
		}); if __e != nil { return }
//line app.gox:153
				__e = __c.Any(neui.Tool[orderItemInput, struct{}]{
			Name: "set_item_quantity",
			Desc: "Set the quantity of a product already on an order; 0 takes it off, and an order keeps at least one item. Checked against stock like the edit form",
			Handler: setItemQuantity,
		}); if __e != nil { return }
//line app.gox:158
				__e = __c.Any(neui.Tool[orderItemInput, struct{}]{
			Name: "add_item",
			Desc: "Add a product to an order; when it is already on the order, its quantity grows by qty. Checked against stock like the edit form",
			Handler: addItem,
		}); if __e != nil { return }
//line app.gox:163
				__e = __c.Any(neui.Tool[removeItemInput, struct{}]{
			Name: "remove_item",
			Desc: "Take a product off an order and put its units back in stock; an order keeps at least one item",
			Handler: removeItem,
		}); if __e != nil { return }
//line app.gox:168
				__e = __c.Any(neui.Tool[addressInput, struct{}]{
			Name: "update_shipping_address",
			Desc: "Change where an order ships",
			Handler: updateShippingAddress,
		}); if __e != nil { return }
//line app.gox:173
				__e = __c.Any(neui.Tool[statusInput, struct{}]{
			Name: "set_status",
			Desc: "Mark an order paid or unpaid; refund_order and cancel_order close it",
			Handler: setStatus,
		}); if __e != nil { return }
//line app.gox:178
				__e = __c.Any(neui.Tool[orderInput, struct{}]{
			Name: "refund_order",
			Desc: "Refund a paid order: the money goes back to the customer and the order closes. Can't be undone",
			Handler: refundOrder,
		}); if __e != nil { return }
//line app.gox:183
				__e = __c.Any(neui.Tool[orderInput, struct{}]{
			Name: "cancel_order",
			Desc: "Cancel an unpaid order: its items go back into stock and the order closes. Can't be undone",
			Handler: cancelOrder,
		}); if __e != nil { return }
//line app.gox:188
				__e = __c.Any(neui.Tool[orderInput, struct{}]{
			Name: "remind_customer",
			Desc: "Send the customer of an unpaid order a payment reminder; the order page shows when it was sent",
			Handler: remindCustomer,
		}); if __e != nil { return }
//line app.gox:193
				__e = __c.Any(neui.Tool[extendDueInput, dueOutput]{
			Name: "extend_due",
			Desc: "Move the due date of an unpaid order later by 1 to 60 days",
			Handler: extendDue,
		}); if __e != nil { return }
//line app.gox:198
				__e = __c.Any(neui.Tool[customerInput, created]{
			Name: "create_customer",
			Desc: "Add a customer, checked like the New customer form. Returns their id; open_customer shows them to the user",
			Handler: createCustomer,
		}); if __e != nil { return }
//line app.gox:203
				__e = __c.Any(neui.Tool[customerPatch, struct{}]{
			Name: "update_customer",
			Desc: "Change a customer's contact details, checked like the edit form; omitted fields stay as they are",
			Handler: updateCustomer,
		}); if __e != nil { return }
//line app.gox:208
				__e = __c.Any(neui.Tool[productInput, created]{
			Name: "create_product",
			Desc: "Add a product, checked like the New product form. Returns its id",
			Handler: createProduct,
		}); if __e != nil { return }
//line app.gox:213
				__e = __c.Any(neui.Tool[productPatch, struct{}]{
			Name: "update_product",
			Desc: "Change a product's SKU, name, price or stock, checked like the product form; omitted fields stay as they are",
			Handler: updateProduct,
		}); if __e != nil { return }
//line app.gox:218
				__e = __c.Any(neui.Tool[adjustStockInput, stockOutput]{
			Name: "adjust_stock",
			Desc: "Add units to a product's stock, or take them away with a negative change; checked like the product form",
			Handler: adjustStock,
		}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line app.gox:224
}

//line app.gox:226
func sidebar() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:227
		__e = (neui.Space{
		Name: "navigation",
		Desc: "Main menu, on every page: links to the dashboard, orders, customers and products",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("aside"); if __e != nil { return }
			{
//line app.gox:230
				__e = __c.Set("class", "sticky top-0 z-30 border-b border-stone-200 bg-white/90 backdrop-blur side:fixed side:inset-y-0 side:left-0 side:w-64 side:border-r side:border-b-0 side:bg-white"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line app.gox:231
					__e = __c.Set("class", "flex h-full flex-col side:px-4 side:py-6"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line app.gox:232
					__e = (neui.Use{Desc: "the shop logo: goes to the dashboard"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line app.gox:233
							__e = __c.Modify(doors.ALink{Model: HomePath{}, Batch: true}); if __e != nil { return }
//line app.gox:234
							__e = __c.Set("class", "flex w-fit items-center gap-2.5 rounded-lg px-4 pt-3 transition-opacity hover:opacity-80 side:px-2 side:pt-0"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
//line app.gox:235
								__e = __c.Set("class", "flex size-8 items-center justify-center rounded-lg bg-amber-600 text-white shadow-xs"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line app.gox:236
								__e = __c.Any(icon(iconCup, "size-5")); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("div"); if __e != nil { return }
							{
//line app.gox:238
								__e = __c.Set("class", "leading-tight"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
//line app.gox:239
									__e = __c.Set("class", "text-sm font-semibold text-stone-900"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Text("Kettle & Burr"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
//line app.gox:240
									__e = __c.Set("class", "text-xs text-stone-500"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Text("Back office"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.Init("nav"); if __e != nil { return }
					{
//line app.gox:243
						__e = __c.Set("class", "mt-2 flex gap-1 overflow-x-auto px-3 pb-2 side:mt-8 side:flex-col side:overflow-visible side:px-0 side:pb-0"); if __e != nil { return }
//line app.gox:243
						__e = __c.Set("aria-label", "Main"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line app.gox:244
						__e = __c.Any(navItem("Dashboard", iconDashboard, HomePath{}, false, "go to the dashboard")); if __e != nil { return }
//line app.gox:245
						__e = __c.Any(navItem("Orders", iconOrders, OrdersPath{}, true, "go to the orders list")); if __e != nil { return }
//line app.gox:246
						__e = __c.Any(navItem("Customers", iconCustomers, CustomersPath{}, true, "go to the customers list")); if __e != nil { return }
//line app.gox:247
						__e = __c.Any(navItem("Products", iconProducts, ProductsPath{}, true, "go to the products list")); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line app.gox:249
						__e = __c.Set("class", "mt-auto hidden rounded-lg bg-stone-50 p-3 text-xs text-stone-500 ring-1 ring-stone-200 side:block"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("p"); if __e != nil { return }
						{
//line app.gox:250
							__e = __c.Set("class", "font-medium text-stone-700"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Assistant"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("p"); if __e != nil { return }
						{
//line app.gox:251
							__e = __c.Set("class", "mt-1"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Open the chat in the corner and ask it to filter, fill a form or build you an app."); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line app.gox:255
}

//line app.gox:257
func navItem(label string, d string, model any, starts bool, desc string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:259
		matcher := doors.PathMatcherFull()
	if starts {
		matcher = doors.PathMatcherStarts()
	}

//line app.gox:264
		__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("a"); if __e != nil { return }
			{
//line app.gox:265
				__e = __c.Modify(doors.ALink{
			Model: model,
			Batch: true,
			After: scrollTop,
			Active: doors.Active{
				Indicator: doors.IndicateAttr("aria-current", "page"),
				PathMatcher: matcher,
				QueryMatcher: doors.QueryMatcherIgnoreAll(),
			},
		}); if __e != nil { return }
//line app.gox:275
				__e = __c.Set("class", "flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium whitespace-nowrap text-stone-600 transition-colors hover:bg-stone-100 hover:text-stone-900 aria-[current=page]:bg-stone-900 aria-[current=page]:text-white"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line app.gox:276
				__e = __c.Any(icon(d, "hidden size-5 opacity-80 sm:block")); if __e != nil { return }
//line app.gox:277
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line app.gox:279
}

//line app.gox:281
func toast() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:282
		__e = __c.Any(notices(ctx).Bind(func(n notice) gox.Elem {
		return toastView(n)
	})); if __e != nil { return }
	return })
//line app.gox:285
}

//line app.gox:287
func toastView(n notice) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line app.gox:288
		if n.Text != "" {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line app.gox:290
				__e = __c.Set("class", "animate-toast pointer-events-none fixed inset-x-0 top-4 z-40 flex justify-center px-4 side:top-6 side:pl-64"); if __e != nil { return }
//line app.gox:291
				__e = __c.Set("role", "status"); if __e != nil { return }
//line app.gox:292
				__e = __c.Set("aria-live", "polite"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line app.gox:293
					__e = __c.Set("class", "pointer-events-auto flex items-center gap-2.5 rounded-xl bg-stone-900 py-2.5 pr-4 pl-3 text-sm text-white shadow-lg"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line app.gox:294
					__e = __c.Any(icon(iconCheck, "size-5 text-emerald-400")); if __e != nil { return }
//line app.gox:295
					__e = __c.Any(n.Text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line app.gox:299
}
