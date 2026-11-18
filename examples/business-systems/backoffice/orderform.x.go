// Managed by GoX v0.4.0

//line orderform.gox:1
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

//line orderform.gox:24
func orderCreatePage(customerID int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:26
		initial := orderForm{Status: string(unpaid), Lines: []lineForm{{Qty: "1"}}}
	if customerID != 0 {
		initial.Customer = strconv.Itoa(customerID)
	}

//line orderform.gox:31
		__e = __c.Any(orderEditor{Initial: initial}); if __e != nil { return }
	return })
//line orderform.gox:32
}

//line orderform.gox:34
func orderEditPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:36
		o, found := shop.Order(id)
	prices := map[int]float64{}
	for _, l := range o.Lines {
		prices[l.ProductID] = l.Price
	}

//line orderform.gox:42
		if !found {
//line orderform.gox:43
			__e = __c.Any(notFound(fmt.Sprintf("Order #%d", id), crumb{Label: "Back to orders", Model: OrdersPath{}})); if __e != nil { return }
//line orderform.gox:44
		} else if o.Status == refunded || o.Status == cancelled {
//line orderform.gox:45
			__e = __c.Any(pageHeader{
			Crumbs: []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: fmt.Sprintf("#%d", o.ID), Model: OrderPath{ID: o.ID}}},
			Title: fmt.Sprintf("Order #%d can't be edited", o.ID),
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("It is "); if __e != nil { return }
//line orderform.gox:48
					__e = __c.Any(string(o.Status)); if __e != nil { return }
					__e = __c.Text(", so it is closed."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orderform.gox:48
			return }),
		}); if __e != nil { return }
		} else  {
//line orderform.gox:51
			__e = __c.Any(orderEditor{ID: id, Initial: o.form(), Prices: prices}); if __e != nil { return }
		}
	return })
//line orderform.gox:53
}

type orderEditor struct {
	ID int
	Initial orderForm
	Prices map[int]float64
}

//line orderform.gox:61
func (e orderEditor) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:63
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

//line orderform.gox:75
		__e = (neui.Space{Name: "order_form", Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:76
				__e = __c.Any(pageHeader{Crumbs: crumbs, Title: title}); if __e != nil { return }
				__e = __c.Init("form"); if __e != nil { return }
				{
//line orderform.gox:78
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
//line orderform.gox:88
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:88
						__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:88
						__e = __c.Set("value", "save"); if __e != nil { return }
//line orderform.gox:88
						__e = __c.Set("class", "sr-only"); if __e != nil { return }
//line orderform.gox:88
						__e = __c.Set("tabindex", "-1"); if __e != nil { return }
//line orderform.gox:88
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Save"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:89
					__e = __c.Any(state.Bind(func(s formState[orderForm]) gox.Elem {
				return e.fields(s, lines, customer)
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line orderform.gox:94
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

//line orderform.gox:130
func (e orderEditor) fields(s formState[orderForm], lines doors.Source[[]lineForm], customer doors.Source[string]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:132
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

//line orderform.gox:150
		__e = __c.Any(formErrors(s.Errs)); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line orderform.gox:151
			__e = __c.Set("class", "grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:152
				__e = __c.Set("class", "space-y-6 @4xl:col-span-2"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:153
				__e = __c.Any(card{
				Title: "Customer and shipping",
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
//line orderform.gox:155
						__e = __c.Set("class", "grid grid-cols-1 gap-5 @xl:grid-cols-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:156
							__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:157
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
//line orderform.gox:171
							__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:172
							__e = __c.Any(textArea{
							Label: "Shipping address",
							Name: "address",
							Value: v.Address,
							Rows: 2,
							Placeholder: "Street, city, postcode",
							Error: s.Errs["address"],
							Desc: "shipping address; leave empty to use the customer's address",
						}); if __e != nil { return }
//line orderform.gox:181
							__e = __c.Any(customer.Bind(func(id string) gox.Elem {
							return addressHint(id, v.Address)
						})); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line orderform.gox:185
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
//line orderform.gox:194
				return }),
			}); if __e != nil { return }
//line orderform.gox:196
				__e = (neui.Space{Name: "items", Desc: "Item rows of the form: a product and a quantity each, with buttons to add and remove rows"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line orderform.gox:196
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
//line orderform.gox:201
							__e = __c.Set("class", "divide-y divide-stone-100"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:202
							for i, l := range v.Lines {
//line orderform.gox:203
								__e = __c.Any(e.lineRow(i, l, s.Errs, products, lines)); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:206
							__e = __c.Set("class", "flex flex-wrap items-center gap-3 border-t border-stone-100 px-5 py-4"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:207
							__e = (neui.Use{Desc: "add another item row to the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line orderform.gox:207
									__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:207
									__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:207
									__e = __c.Set("value", "add"); if __e != nil { return }
									__e = __c.Set("formnovalidate", true); if __e != nil { return }
//line orderform.gox:207
									__e = __c.Set("class", btnSecondary); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:208
									__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
									__e = __c.Text("Add item"); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line orderform.gox:211
							if s.Errs["lines"] != "" {
								__e = __c.Init("p"); if __e != nil { return }
								{
//line orderform.gox:212
									__e = __c.Set("class", "flex items-center gap-1 text-sm text-red-600"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:213
									__e = __c.Any(icon(iconWarning, "size-4")); if __e != nil { return }
//line orderform.gox:214
									__e = __c.Any(s.Errs["lines"]); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orderform.gox:218
				return }),
			}); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:221
				__e = __c.Set("class", "@4xl:sticky @4xl:top-6 @4xl:self-start"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:222
				__e = __c.Any(card{
				Title: "Summary",
				Body: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line orderform.gox:225
						__e = __c.Any(lines.Bind(func(ls []lineForm) gox.Elem {
						return e.summary(ls)
					})); if __e != nil { return }
						__e = __c.Init("div"); if __e != nil { return }
						{
//line orderform.gox:228
							__e = __c.Set("class", "mt-5 flex flex-col gap-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:229
							__e = (neui.Use{Desc: "save the order with the values in the form"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("button"); if __e != nil { return }
								{
//line orderform.gox:229
									__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:229
									__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:229
									__e = __c.Set("value", "save"); if __e != nil { return }
//line orderform.gox:229
									__e = __c.Set("class", btnPrimary.Add("w-full")); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:230
									__e = __c.Any(submit); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line orderform.gox:232
							if e.ID == 0 {
//line orderform.gox:233
								__e = (neui.Use{Desc: "leave the form without creating the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line orderform.gox:233
										__e = __c.Modify(doors.ALink{Model: OrdersPath{}, Batch: true}); if __e != nil { return }
//line orderform.gox:233
										__e = __c.Set("class", btnGhost.Add("w-full")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Cancel"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								return })); if __e != nil { return }
							} else  {
//line orderform.gox:235
								__e = (neui.Use{Desc: "leave the form without saving the changes"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line orderform.gox:235
										__e = __c.Modify(doors.ALink{Model: OrderPath{ID: e.ID}, Batch: true}); if __e != nil { return }
//line orderform.gox:235
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
//line orderform.gox:238
				return }),
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:242
}

//line orderform.gox:244
func addressHint(id string, typed string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:246
		cid, _ := strconv.Atoi(id)
	c, ok := shop.Customer(cid)

//line orderform.gox:249
		if ok && typed == "" {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:250
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:250
				__e = __c.Many("Leave empty to ship to ", c.Address, "."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line orderform.gox:251
		} else if ok && typed != c.Address {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:252
				__e = __c.Set("class", "mt-1.5 text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:252
				__e = __c.Many("Customer's address on file: ", c.Address); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line orderform.gox:254
}

//line orderform.gox:256
func (e orderEditor) lineRow(i int, l lineForm, errs fieldErrors, products []option, lines doors.Source[[]lineForm]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:258
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
//line orderform.gox:269
			__e = __c.Set("class", "grid grid-cols-[1fr_auto] items-start gap-3 px-5 py-4 @xl:grid-cols-[1fr_8rem_auto]"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:270
				__e = __c.Set("class", "col-span-2 @xl:col-span-1"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:271
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
//line orderform.gox:289
				id := fieldID(fmt.Sprintf("lines[%d].qty", i))

//line orderform.gox:291
				if i == 0 {
//line orderform.gox:292
					__e = __c.Any(fieldLabel(id, "Qty", false)); if __e != nil { return }
				}
//line orderform.gox:294
				__e = (neui.Use{Desc: fmt.Sprintf("quantity of item %d", i + 1)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line orderform.gox:295
						__e = __c.Modify(doors.AChange{
					Race: doors.RaceLatest,
					On: func(ctx context.Context, r doors.RequestChange) bool {
						set(ctx, func(l *lineForm) {
							l.Qty = r.Event().Value
						})
						return false
					},
				}); if __e != nil { return }
//line orderform.gox:304
						__e = __c.Set("id", id); if __e != nil { return }
//line orderform.gox:305
						__e = __c.Set("name", fmt.Sprintf("lines[%d].qty", i)); if __e != nil { return }
//line orderform.gox:306
						__e = __c.Set("type", "number"); if __e != nil { return }
//line orderform.gox:307
						__e = __c.Set("min", "1"); if __e != nil { return }
//line orderform.gox:308
						__e = __c.Set("inputmode", "numeric"); if __e != nil { return }
//line orderform.gox:309
						__e = __c.Set("value", l.Qty); if __e != nil { return }
//line orderform.gox:310
						__e = __c.Set("aria-label", fmt.Sprintf("Quantity of item %d", i + 1)); if __e != nil { return }
//line orderform.gox:311
						__e = __c.Set("aria-invalid", ariaTrue(errs[key + ".qty"] != "")); if __e != nil { return }
//line orderform.gox:312
						__e = __c.Set("aria-describedby", id + "-note"); if __e != nil { return }
//line orderform.gox:313
						__e = __c.Set("class", inputBase.Add("tabular-nums", when(i == 0, "mt-1.5"))); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
//line orderform.gox:314
				__e = __c.Any(fieldNote(id, "", errs[key + ".qty"])); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orderform.gox:316
				__e = __c.Set("class", when(i == 0, "pt-7")); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:317
				__e = (neui.Use{Desc: fmt.Sprintf("remove item %d from the order", i + 1)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line orderform.gox:318
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line orderform.gox:319
						__e = __c.Set("name", "intent"); if __e != nil { return }
//line orderform.gox:320
						__e = __c.Set("value", fmt.Sprintf("remove-%d", i)); if __e != nil { return }
						__e = __c.Set("formnovalidate", true); if __e != nil { return }
//line orderform.gox:322
						__e = __c.Set("class", btnGhost.Add("px-2.5")); if __e != nil { return }
//line orderform.gox:323
						__e = __c.Set("aria-label", fmt.Sprintf("Remove item %d", i + 1)); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:324
						__e = __c.Any(icon(iconTrash, "size-4")); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:328
}

//line orderform.gox:330
func (e orderEditor) summary(ls []lineForm) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orderform.gox:332
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

//line orderform.gox:354
		__e = __c.Any(neui.Data{Text: "order total for the items picked so far, USD", Object: round(total)}); if __e != nil { return }
//line orderform.gox:355
		if len(rows) == 0 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line orderform.gox:356
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Pick products to see the total."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line orderform.gox:358
				__e = __c.Set("class", "space-y-2 text-sm"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:359
				for _, r := range rows {
					__e = __c.Init("li"); if __e != nil { return }
					{
//line orderform.gox:360
						__e = __c.Set("class", "flex justify-between gap-3"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line orderform.gox:361
							__e = __c.Set("class", "text-stone-600"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:361
							__e = __c.Many(r.Qty, " × ", r.Name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line orderform.gox:362
							__e = __c.Set("class", "text-stone-900 tabular-nums"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:362
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
//line orderform.gox:367
			__e = __c.Set("class", "mt-4 flex items-baseline justify-between border-t border-stone-100 pt-4"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line orderform.gox:368
				__e = __c.Set("class", "text-sm font-medium text-stone-600"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Total"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line orderform.gox:369
				__e = __c.Set("class", "text-xl font-semibold text-stone-900 tabular-nums"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orderform.gox:369
				__e = __c.Any(money(total)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orderform.gox:371
}
