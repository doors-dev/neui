// Managed by GoX v0.4.0

//line orderform.gox:1
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
	"slices"
	"strconv"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

type formState[T any] struct {
	Values T
	Errs fieldErrors
}

func always[T any](T, T) bool {
	return false
}

//line orderform.gox:38
func orderCreatePage(customerID int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:40
		initial := orderForm{Status: string(unpaid), Lines: []lineForm{{Qty: "1"}}}
	if customerID != 0 {
		initial.Customer = strconv.Itoa(customerID)
	}

//line orderform.gox:45
		__e = __c.Any(orderEditor{Initial: initial}); if __e != nil { return }
	return })
//line orderform.gox:46
}

//line orderform.gox:48
func orderEditPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:50
		o, found := shop.Order(id)
	prices := map[int]float64{}
	for _, l := range o.Lines {
		prices[l.ProductID] = l.Price
	}

//line orderform.gox:56
		if !found {
//line orderform.gox:57
			__e = __c.Any(notFound(fmt.Sprintf("Order #%d", id), crumb{Label: "Back to orders", Model: OrdersPath{}})); if __e != nil { return }
//line orderform.gox:58
		} else if o.Status == refunded || o.Status == cancelled {
//line orderform.gox:59
			__e = __c.Any(pageHeader{
			Crumbs: []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: fmt.Sprintf("#%d", o.ID), Model: OrderPath{ID: o.ID}}},
			Title: fmt.Sprintf("Order #%d can't be edited", o.ID),
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("It is "); if __e != nil { return }
//line orderform.gox:62
					__e = __c.Any(string(o.Status)); if __e != nil { return }
					__e = __c.Text(", so it is closed."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orderform.gox:62
			return }),
		}); if __e != nil { return }
		} else  {
//line orderform.gox:65
			__e = __c.Any(orderEditor{ID: id, Initial: o.form(), Prices: prices}); if __e != nil { return }
		}
	return })
//line orderform.gox:67
}

type orderEditor struct {
	ID int
	Initial orderForm
	Prices map[int]float64
}

//line orderform.gox:75
func (e orderEditor) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:77
		state := doors.NewSourceEqual(formState[orderForm]{Values: e.Initial}, always)
	lines := doors.NewSourceEqual(e.Initial.Lines, always)
	customer := doors.NewSource(e.Initial.Customer)
	title := "New order"
	crumbs := []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: "New", Model: OrdersPath{View: ListNew}}}
	desc := "Form creating an order: pick the customer, add items with quantities, set the shipping address and payment status, then press Create order"
	if e.ID != 0 {
		title = fmt.Sprintf("Edit order #%d", e.ID)
		crumbs = []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: fmt.Sprintf("#%d", e.ID), Model: OrderPath{ID: e.ID}}, {Label: "Edit", Model: OrderPath{View: ItemEdit, ID: e.ID}}}
		desc = fmt.Sprintf("Form editing order #%d: customer, items, shipping address and payment status; press Save changes to keep them", e.ID)
	}

//line orderform.gox:89
		__e = (neui.Space{Name: "order_form", Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:90
				__e = __c.Any(pageHeader{Crumbs: crumbs, Title: title}); if __e != nil { return }
				__e = __c.Init("form"); if __e != nil { return }
				{
//line orderform.gox:92
					__e = __c.Modify(doors.ASubmit[orderForm]{
				Scope: &doors.ScopeBlocking{},
				Indicator: doors.IndicateAttrSubmitter("aria-busy", "true"),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestForm[orderForm]) bool {
					e.submit(ctx, r, state, lines)
					return false
				},
			}); if __e != nil { return }
					__e = __c.Set("novalidate", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("button"); if __e != nil { return }
					{
//line orderform.gox:102
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:102
						__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:102
						__e = __c.Set("value", "save"); if __e != nil { return }
//line orderform.gox:102
						__e = __c.Set("class", "sr-only"); if __e != nil { return }
//line orderform.gox:102
						__e = __c.Set("tabindex", "-1"); if __e != nil { return }
//line orderform.gox:102
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Save"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:103
					__e = __c.Any(state.Bind(func(s formState[orderForm]) gox.Elem {
				return e.fields(s, lines, customer)
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line orderform.gox:108
}

func (e orderEditor) submit(ctx context.Context, r doors.RequestForm[orderForm], state doors.Source[formState[orderForm]], lines doors.Source[[]lineForm]) {
	v := r.Data()
	intent := v.Intent
	v.Intent = ""
	switch {
	case intent == "add":
		v.Lines = append(slices.Clone(v.Lines), lineForm{Qty: "1"})
		state.Update(ctx, formState[orderForm]{Values: v})
		lines.Update(ctx, v.Lines)
		return
	case strings.HasPrefix(intent, "remove-"):
		i, _ := strconv.Atoi(strings.TrimPrefix(intent, "remove-"))
		if i >= 0 && i < len(v.Lines) {
			v.Lines = slices.Delete(slices.Clone(v.Lines), i, i + 1)
		}
		state.Update(ctx, formState[orderForm]{Values: v})
		lines.Update(ctx, v.Lines)
		return
	}
	id, errs := shop.SaveOrder(ctx, e.ID, v)
	if len(errs) > 0 {
		state.Update(ctx, formState[orderForm]{Values: v, Errs: errs})
		lines.Update(ctx, v.Lines)
		return
	}
	if e.ID == 0 {
		notify(ctx, fmt.Sprintf("Order #%d created", id))
	} else {
		notify(ctx, fmt.Sprintf("Order #%d saved", id))
	}
	navigate(ctx, OrderPath{ID: id})
	r.After(scrollTop)
}

//line orderform.gox:144
func (e orderEditor) fields(s formState[orderForm], lines doors.Source[[]lineForm], customer doors.Source[string]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:146
		var customers []option
	for _, c := range shop.AllCustomers() {
		label := c.Name
		if c.Company != "" {
			label += " · " + c.Company
		}
		customers = append(customers, option{Value: strconv.Itoa(c.ID), Label: label})
	}
	var products []option
	for _, p := range shop.Products("", "name") {
		products = append(products, option{Value: strconv.Itoa(p.ID), Label: fmt.Sprintf("%s · %s · %d in stock", p.Name, money(p.Price), p.Stock)})
	}
	v := s.Values
	submit := "Create order"
	if e.ID != 0 {
		submit = "Save changes"
	}

//line orderform.gox:164
		__e = __c.Any(formErrors(s.Errs)); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line orderform.gox:165
			__e = __c.Set("class", "grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:166
				__e = __c.Set("class", "space-y-6 @4xl:col-span-2"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:167
				__e = __c.Any(card{
				Title: "Customer and shipping",
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
//line orderform.gox:169
						__e = __c.Set("class", "grid grid-cols-1 gap-5 @xl:grid-cols-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:170
							__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:171
							__e = __c.Any(selectField{
							Label: "Customer",
							Name: "customer",
							Value: v.Customer,
							Placeholder: "Choose a customer…",
							Options: customers,
							Required: true,
							Error: s.Errs["customer"],
							Desc: "customer who placed the order",
							Change: func(ctx context.Context, value string) {
								customer.Update(ctx, value)
							},
						}); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:185
							__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:186
							__e = __c.Any(textArea{
							Label: "Shipping address",
							Name: "address",
							Value: v.Address,
							Rows: 2,
							Placeholder: "Street, city, postcode",
							Error: s.Errs["address"],
							Desc: "shipping address; leave empty to use the customer's address",
						}); if __e != nil { return }
//line orderform.gox:195
							__e = __c.Any(customer.Bind(func(id string) gox.Elem {
							return addressHint(id, v.Address)
						})); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line orderform.gox:199
						__e = __c.Any(selectField{
						Label: "Payment",
						Name: "status",
						Value: v.Status,
						Options: []option{{Value: string(unpaid), Label: "Unpaid"}, {Value: string(paid), Label: "Paid"}},
						Required: true,
						Error: s.Errs["status"],
						Desc: "payment status: paid or unpaid",
					}); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:208
				return }),
			}); if __e != nil { return }
//line orderform.gox:210
				__e = (neui.Space{Name: "items", Desc: "Item rows of the form: a product and a quantity each, with buttons to add and remove rows"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line orderform.gox:210
					__e = __c.Any(card{
				Title: "Items",
				Desc: "Products and quantities. Stock is checked when you save.",
				Flush: true,
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("ul"); if __e != nil { return }
						{
//line orderform.gox:215
							__e = __c.Set("class", "divide-y divide-stone-100"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:216
							for i, l := range v.Lines {
//line orderform.gox:217
								__e = __c.Any(e.lineRow(i, l, s.Errs, products, lines)); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:220
							__e = __c.Set("class", "flex flex-wrap items-center gap-3 border-t border-stone-100 px-5 py-4"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:221
							__e = (neui.Use{Desc: "add another item row to the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line orderform.gox:221
									__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:221
									__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:221
									__e = __c.Set("value", "add"); if __e != nil { return }
									__e = __c.Set("formnovalidate", true); if __e != nil { return }
//line orderform.gox:221
									__e = __c.Set("class", btnSecondary); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:222
									__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
									__e = __c.Text("Add item"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line orderform.gox:225
							if s.Errs["lines"] != "" {
								__e = __c.Init("p"); if __e != nil { return }
								{
//line orderform.gox:226
									__e = __c.Set("class", "flex items-center gap-1 text-sm text-red-600"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:227
									__e = __c.Any(icon(iconWarning, "size-4")); if __e != nil { return }
//line orderform.gox:228
									__e = __c.Any(s.Errs["lines"]); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:232
				return }),
			}); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:235
				__e = __c.Set("class", "@4xl:sticky @4xl:top-6 @4xl:self-start"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:236
				__e = __c.Any(card{
				Title: "Summary",
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line orderform.gox:239
						__e = __c.Any(lines.Bind(func(ls []lineForm) gox.Elem {
						return e.summary(ls)
					})); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:242
							__e = __c.Set("class", "mt-5 flex flex-col gap-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:243
							__e = (neui.Use{Desc: "save the order with the values in the form"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line orderform.gox:243
									__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:243
									__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:243
									__e = __c.Set("value", "save"); if __e != nil { return }
//line orderform.gox:243
									__e = __c.Set("class", btnPrimary.Add("w-full")); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:244
									__e = __c.Any(submit); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line orderform.gox:246
							if e.ID == 0 {
//line orderform.gox:247
								__e = (neui.Use{Desc: "leave the form without creating the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line orderform.gox:247
										__e = __c.Modify(doors.ALink{Model: OrdersPath{}, Batch: true}); if __e != nil { return }
//line orderform.gox:247
										__e = __c.Set("class", btnGhost.Add("w-full")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Cancel"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								return })); if __e != nil { return }
							} else  {
//line orderform.gox:249
								__e = (neui.Use{Desc: "leave the form without saving the changes"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line orderform.gox:249
										__e = __c.Modify(doors.ALink{Model: OrderPath{ID: e.ID}, Batch: true}); if __e != nil { return }
//line orderform.gox:249
										__e = __c.Set("class", btnGhost.Add("w-full")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Cancel"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								return })); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:252
				return }),
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:256
}

//line orderform.gox:258
func addressHint(id string, typed string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:260
		cid, _ := strconv.Atoi(id)
	c, ok := shop.Customer(cid)

//line orderform.gox:263
		if ok && typed == "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:264
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:264
				__e = __c.Many("Leave empty to ship to ", c.Address, "."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line orderform.gox:265
		} else if ok && typed != c.Address {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:266
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:266
				__e = __c.Many("Customer's address on file: ", c.Address); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line orderform.gox:268
}

//line orderform.gox:270
func (e orderEditor) lineRow(i int, l lineForm, errs fieldErrors, products []option, lines doors.Source[[]lineForm]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:272
		key := fmt.Sprintf("lines.%d", i)
	set := func(ctx context.Context, change func(l *lineForm)) {
		lines.Mutate(ctx, func(ls []lineForm) []lineForm {
			ls = slices.Clone(ls)
			if i < len(ls) {
				change(&ls[i])
			}
			return ls
		})
	}

		__e = __c.Init("li"); if __e != nil { return }
		{
//line orderform.gox:283
			__e = __c.Set("class", "grid grid-cols-[1fr_auto] items-start gap-3 px-5 py-4 @xl:grid-cols-[1fr_8rem_auto]"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:284
				__e = __c.Set("class", "col-span-2 @xl:col-span-1"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:285
				__e = __c.Any(selectField{
				Label: "Product",
				Name: fmt.Sprintf("lines[%d].product", i),
				Value: l.Product,
				Placeholder: "Choose a product…",
				Options: products,
				Hide: i > 0,
				Error: errs[key + ".product"],
				Desc: fmt.Sprintf("product of item %d", i + 1),
				Change: func(ctx context.Context, value string) {
					set(ctx, func(l *lineForm) {
						l.Product = value
					})
				},
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:303
				id := fieldID(fmt.Sprintf("lines[%d].qty", i))

//line orderform.gox:305
				if i == 0 {
//line orderform.gox:306
					__e = __c.Any(fieldLabel(id, "Qty", false)); if __e != nil { return }
				}
//line orderform.gox:308
				__e = (neui.Use{Desc: fmt.Sprintf("quantity of item %d", i + 1)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line orderform.gox:309
						__e = __c.Modify(doors.AChange{
					Race: doors.RaceLatest,
					On: func(ctx context.Context, r doors.RequestChange) bool {
						set(ctx, func(l *lineForm) {
							l.Qty = r.Event().Value
						})
						return false
					},
				}); if __e != nil { return }
//line orderform.gox:318
						__e = __c.Set("id", id); if __e != nil { return }
//line orderform.gox:319
						__e = __c.Set("name", fmt.Sprintf("lines[%d].qty", i)); if __e != nil { return }
//line orderform.gox:320
						__e = __c.Set("type", "number"); if __e != nil { return }
//line orderform.gox:321
						__e = __c.Set("min", "1"); if __e != nil { return }
//line orderform.gox:322
						__e = __c.Set("inputmode", "numeric"); if __e != nil { return }
//line orderform.gox:323
						__e = __c.Set("value", l.Qty); if __e != nil { return }
//line orderform.gox:324
						__e = __c.Set("aria-label", fmt.Sprintf("Quantity of item %d", i + 1)); if __e != nil { return }
//line orderform.gox:325
						__e = __c.Set("aria-invalid", ariaTrue(errs[key + ".qty"] != "")); if __e != nil { return }
//line orderform.gox:326
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line orderform.gox:327
						__e = __c.Set("class", inputBase.Add("tabular-nums", when(i == 0, "mt-1.5"))); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
//line orderform.gox:328
				__e = __c.Any(fieldNote(id, "", errs[key + ".qty"])); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:330
				__e = __c.Set("class", when(i == 0, "pt-7")); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:331
				__e = (neui.Use{Desc: fmt.Sprintf("remove item %d from the order", i + 1)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line orderform.gox:332
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:333
						__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:334
						__e = __c.Set("value", fmt.Sprintf("remove-%d", i)); if __e != nil { return }
						__e = __c.Set("formnovalidate", true); if __e != nil { return }
//line orderform.gox:336
						__e = __c.Set("class", btnGhost.Add("px-2.5")); if __e != nil { return }
//line orderform.gox:337
						__e = __c.Set("aria-label", fmt.Sprintf("Remove item %d", i + 1)); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:338
						__e = __c.Any(icon(iconTrash, "size-4")); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:342
}

//line orderform.gox:344
func (e orderEditor) summary(ls []lineForm) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:346
		type row struct {
		Name string
		Qty int
		Amount float64
	}
	var rows []row
	total := 0.0
	for _, l := range ls {
		pid, _ := strconv.Atoi(l.Product)
		p, ok := shop.Product(pid)
		qty, err := strconv.Atoi(strings.TrimSpace(l.Qty))
		if !ok || err != nil || qty < 1 {
			continue
		}
		price := p.Price
		if old, ok := e.Prices[pid]; ok {
			price = old
		}
		rows = append(rows, row{Name: p.Name, Qty: qty, Amount: round(float64(qty) * price)})
		total += float64(qty) * price
	}

//line orderform.gox:368
		__e = __c.Any(neui.Data{Text: "order total for the items picked so far, USD", Object: round(total)}); if __e != nil { return }
//line orderform.gox:369
		if len(rows) == 0 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:370
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Pick products to see the total."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line orderform.gox:372
				__e = __c.Set("class", "space-y-2 text-sm"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:373
				for _, r := range rows {
					__e = __c.Init("li"); if __e != nil { return }
					{
//line orderform.gox:374
						__e = __c.Set("class", "flex justify-between gap-3"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line orderform.gox:375
							__e = __c.Set("class", "text-stone-600"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:375
							__e = __c.Many(r.Qty, " × ", r.Name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line orderform.gox:376
							__e = __c.Set("class", "text-stone-900 tabular-nums"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:376
							__e = __c.Any(money(r.Amount)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Init("div"); if __e != nil { return }
		{
//line orderform.gox:381
			__e = __c.Set("class", "mt-4 flex items-baseline justify-between border-t border-stone-100 pt-4"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line orderform.gox:382
				__e = __c.Set("class", "text-sm font-medium text-stone-600"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Total"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line orderform.gox:383
				__e = __c.Set("class", "text-xl font-semibold text-stone-900 tabular-nums"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:383
				__e = __c.Any(money(total)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:385
}
