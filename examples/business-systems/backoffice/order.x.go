// Managed by GoX v0.4.0

//line order.gox:1
package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line order.gox:16
func orderPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:17
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line order.gox:17
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
//line order.gox:29
}

type statusInput struct {
	Status string `json:"status" required:"true" description:"paid or unpaid"`
}

type addressInput struct {
	Address string `json:"address" description:"new shipping address; empty for the customer's address on file"`
}

type removeItemInput struct {
	Product int `json:"product" required:"true" description:"product id of the item to take off"`
}

//line order.gox:43
func orderView(o order, c customerRow) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:45
		open := o.Status == paid || o.Status == unpaid

//line order.gox:47
		__e = (neui.Space{
		Name: "order",
		Desc: fmt.Sprintf("Page of order #%d: its items, shipping address, customer, payment status and refund or cancel", o.ID),
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line order.gox:51
				__e = __c.Any(neui.Data{Text: "the order on this page", Object: o}); if __e != nil { return }
//line order.gox:52
				__e = __c.Any(pageHeader{
			Crumbs: []crumb{{Label: "Orders", Model: OrdersPath{}}, {Label: fmt.Sprintf("#%d", o.ID), Model: OrderPath{ID: o.ID}}},
			Title: fmt.Sprintf("Order #%d", o.ID),
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line order.gox:56
					__e = __c.Any(badge(o.Status)); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:57
						__e = __c.Many("Placed ", o.Date); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:58
						__e = __c.Any(money(o.Total)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:59
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line order.gox:61
					if open {
//line order.gox:62
						__e = (neui.Use{Desc: "open the edit form of this order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line order.gox:63
								__e = __c.Modify(doors.ALink{Model: OrderPath{View: ItemEdit, ID: o.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:64
								__e = __c.Set("class", btnSecondary); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:65
								__e = __c.Any(icon(iconEdit, "size-4")); if __e != nil { return }
								__e = __c.Text("Edit order"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:69
			return }),
		}); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line order.gox:71
					__e = __c.Set("class", "grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line order.gox:72
						__e = __c.Set("class", "space-y-6 @4xl:col-span-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:73
						__e = (neui.Space{Name: "items", Desc: "Products on the order with quantities and amounts"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:74
								if open {
//line order.gox:75
									__e = __c.Any(neui.Tool[itemInput, struct{}]{
							Name: "add_item",
							Desc: "Add a product to this order; when it is already on the order, its quantity grows by qty. Checked against stock like the edit form",
							Handler: func(ctx context.Context, in itemInput) (struct{}, error) {
								line := 0
								err := shop.ChangeOrder(ctx, o.ID, func(f *orderForm) error {
									if in.Qty < 1 {
										return fmt.Errorf("qty must be at least 1")
									}
									product := strconv.Itoa(in.Product)
									i := slices.IndexFunc(f.Lines, func(l lineForm) bool {
										return l.Product == product
									})
									if i < 0 {
										line = len(f.Lines)
										f.Lines = append(f.Lines, lineForm{Product: product, Qty: strconv.Itoa(in.Qty)})
										return nil
									}
									line = i
									qty, _ := strconv.Atoi(f.Lines[i].Qty)
									f.Lines[i].Qty = strconv.Itoa(qty + in.Qty)
									return nil
								})
								if err != nil {
									return struct{}{}, errors.New(strings.ReplaceAll(err.Error(), "lines."+strconv.Itoa(line)+".", ""))
								}
								notify(ctx, fmt.Sprintf("Order #%d: item added", o.ID))
								return struct{}{}, nil
							},
						}); if __e != nil { return }
//line order.gox:105
									__e = __c.Any(neui.Tool[removeItemInput, struct{}]{
							Name: "remove_item",
							Desc: "Take a product off this order and put its units back in stock; an order keeps at least one item",
							Handler: func(ctx context.Context, in removeItemInput) (struct{}, error) {
								err := shop.ChangeOrder(ctx, o.ID, func(f *orderForm) error {
									product := strconv.Itoa(in.Product)
									if !slices.ContainsFunc(f.Lines, func(l lineForm) bool {
										return l.Product == product
									}) {
										return fmt.Errorf("product %d is not on order #%d", in.Product, o.ID)
									}
									f.Lines = slices.DeleteFunc(f.Lines, func(l lineForm) bool {
										return l.Product == product
									})
									return nil
								})
								if err != nil {
									return struct{}{}, err
								}
								notify(ctx, fmt.Sprintf("Order #%d: item removed", o.ID))
								return struct{}{}, nil
							},
						}); if __e != nil { return }
								}
//line order.gox:129
								__e = __c.Any(card{
						Title: "Items",
						Flush: true,
						Body: orderLines(o),
					}); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
//line order.gox:135
						__e = (neui.Space{Name: "shipping", Desc: "Where the order ships"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:136
								if open {
//line order.gox:137
									__e = __c.Any(neui.Tool[addressInput, struct{}]{
							Name: "update_shipping_address",
							Desc: "Change where this order ships",
							Handler: func(ctx context.Context, in addressInput) (struct{}, error) {
								err := shop.ChangeOrder(ctx, o.ID, func(f *orderForm) error {
									f.Address = in.Address
									return nil
								})
								if err != nil {
									return struct{}{}, err
								}
								notify(ctx, fmt.Sprintf("Order #%d: shipping address saved", o.ID))
								return struct{}{}, nil
							},
						}); if __e != nil { return }
								}
//line order.gox:153
								__e = __c.Any(card{
						Title: "Shipping",
						Body: gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.InitContainer(); if __e != nil { return }
							{
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:156
									__e = __c.Set("class", "text-sm whitespace-pre-line text-stone-900"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:156
									__e = __c.Any(o.Address); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line order.gox:157
								if open {
									__e = __c.Init("p"); if __e != nil { return }
									{
//line order.gox:158
										__e = __c.Set("class", "mt-2 text-sm text-stone-500"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Wrong address? Fix it with Edit order."); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
//line order.gox:160
						return }),
					}); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line order.gox:164
						__e = __c.Set("class", "space-y-6"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line order.gox:165
						__e = (neui.Space{Name: "customer", Desc: "Who placed the order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line order.gox:165
							__e = __c.Any(card{
					Title: "Customer",
					Body: gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitContainer(); if __e != nil { return }
						{
//line order.gox:168
							__e = (neui.Use{Desc: "open the page of customer " + c.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("a"); if __e != nil { return }
								{
//line order.gox:169
									__e = __c.Modify(doors.ALink{Model: CustomerPath{ID: c.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:170
									__e = __c.Set("class", linkText); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:171
									__e = __c.Any(c.Name); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							return })); if __e != nil { return }
//line order.gox:173
							if c.Company != "" {
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:174
									__e = __c.Set("class", "mt-0.5 text-sm text-stone-500"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:174
									__e = __c.Any(c.Company); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Init("div"); if __e != nil { return }
							{
//line order.gox:176
								__e = __c.Set("class", "mt-3 space-y-1 text-sm text-stone-600"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:177
									__e = __c.Any(c.Email); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line order.gox:178
								if c.Phone != "" {
									__e = __c.Init("p"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line order.gox:179
										__e = __c.Any(c.Phone); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("p"); if __e != nil { return }
							{
//line order.gox:182
								__e = __c.Set("class", "mt-3 text-xs text-stone-500"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:182
								__e = __c.Many(c.Orders, " order", plural(c.Orders), " · ", money(c.Spent), " paid"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line order.gox:183
					return }),
				}); if __e != nil { return }
						return })); if __e != nil { return }
//line order.gox:185
						__e = (neui.Space{Name: "actions", Desc: "Payment status and what can still be done with the order: refund a paid one, cancel an unpaid one"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("div"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:186
								if open {
//line order.gox:187
									__e = __c.Any(neui.Tool[statusInput, struct{}]{
							Name: "set_status",
							Desc: "Mark this order paid or unpaid. Refunded and cancelled are not statuses to set: the refund and cancel buttons here open a confirmation only the user can press",
							Handler: func(ctx context.Context, in statusInput) (struct{}, error) {
								err := shop.ChangeOrder(ctx, o.ID, func(f *orderForm) error {
									f.Status = in.Status
									return nil
								})
								if err != nil {
									return struct{}{}, err
								}
								notify(ctx, fmt.Sprintf("Order #%d marked %s", o.ID, in.Status))
								return struct{}{}, nil
							},
						}); if __e != nil { return }
								}
//line order.gox:203
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
//line order.gox:208
}

//line order.gox:210
func orderLines(o order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line order.gox:211
			__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("table"); if __e != nil { return }
			{
//line order.gox:212
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
//line order.gox:215
							__e = __c.Set("class", thClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Product"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:216
							__e = __c.Set("class", thClass.Add("text-right")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Qty"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:217
							__e = __c.Set("class", thClass.Add("hidden text-right @xl:table-cell")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Unit price"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("th"); if __e != nil { return }
						{
//line order.gox:218
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
//line order.gox:222
					for _, l := range o.Lines {
						__e = __c.Init("tr"); if __e != nil { return }
						{
//line order.gox:223
							__e = __c.Set("class", "border-t border-stone-100"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:224
								__e = __c.Set("class", tdClass); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:225
								__e = (neui.Use{Desc: "open product " + l.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
									ctx := __c.Context(); _ = ctx
									__e = __c.Init("a"); if __e != nil { return }
									{
//line order.gox:225
										__e = __c.Modify(doors.ALink{Model: ProductPath{ID: l.ProductID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line order.gox:225
										__e = __c.Set("class", "font-medium text-stone-900 hover:underline"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line order.gox:226
										__e = __c.Any(l.Name); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								return })); if __e != nil { return }
								__e = __c.Init("p"); if __e != nil { return }
								{
//line order.gox:228
									__e = __c.Set("class", "text-xs text-stone-500"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line order.gox:228
									__e = __c.Any(l.SKU); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:230
								__e = __c.Set("class", tdClass.Add("text-right tabular-nums")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:230
								__e = __c.Any(l.Qty); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:231
								__e = __c.Set("class", tdClass.Add("hidden text-right text-stone-500 tabular-nums @xl:table-cell")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:231
								__e = __c.Any(money(l.Price)); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line order.gox:232
								__e = __c.Set("class", tdClass.Add("text-right tabular-nums")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line order.gox:232
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
//line order.gox:237
						__e = __c.Set("class", "border-t border-stone-200 bg-stone-50/60"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:238
							__e = __c.Set("class", tdClass.Add("font-medium")); if __e != nil { return }
//line order.gox:238
							__e = __c.Set("colspan", "2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Total"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:239
							__e = __c.Set("class", tdClass.Add("hidden @xl:table-cell")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("td"); if __e != nil { return }
						{
//line order.gox:240
							__e = __c.Set("class", tdClass.Add("text-right text-base font-semibold tabular-nums")); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line order.gox:240
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
//line order.gox:245
}

//line order.gox:247
func orderActions(o order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line order.gox:248
		if o.Status == paid {
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Init("p"); if __e != nil { return }
				{
//line order.gox:250
					__e = __c.Set("class", "mb-3 text-sm text-stone-500"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Paid in full. A refund returns "); if __e != nil { return }
//line order.gox:250
					__e = __c.Any(money(o.Total)); if __e != nil { return }
					__e = __c.Text(" to the customer."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:251
				__e = __c.Any(confirmAction{
				Label: "Refund order",
				Icon: iconMoney,
				Desc: "start a refund of this order; the user must then confirm it on the page",
				Question: fmt.Sprintf("Refund %s to %s?", money(o.Total), o.Customer),
				Detail: "The money goes back to the customer's card. This can't be undone.",
				Confirm: "Confirm refund",
				Full: true,
				Wait: "The refund waits for the user: only they can press Confirm refund. Ask them to confirm it on the page.",
				Do: func(ctx context.Context) error {
					if err := shop.Refund(ctx, o.ID); err != nil {
						return err
					}
					notify(ctx, fmt.Sprintf("Order #%d refunded", o.ID))
					return nil
				},
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line order.gox:269
		} else if o.Status == unpaid {
			__e = __c.InitContainer(); if __e != nil { return }
			{
				__e = __c.Init("p"); if __e != nil { return }
				{
//line order.gox:271
					__e = __c.Set("class", "mb-3 text-sm text-stone-500"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Not paid yet. Mark it paid in Edit order, or cancel it to put the items back in stock."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line order.gox:272
				__e = __c.Any(confirmAction{
				Label: "Cancel order",
				Icon: iconClose,
				Desc: "start cancelling this order; the user must then confirm it on the page",
				Question: fmt.Sprintf("Cancel order #%d?", o.ID),
				Detail: "Its items go back into stock. A cancelled order can't be reopened.",
				Confirm: "Cancel order",
				Full: true,
				Wait: "Cancelling waits for the user: only they can press Cancel order in the confirmation. Ask them to confirm it on the page.",
				Do: func(ctx context.Context) error {
					if err := shop.Cancel(ctx, o.ID); err != nil {
						return err
					}
					notify(ctx, fmt.Sprintf("Order #%d cancelled", o.ID))
					return nil
				},
			}); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line order.gox:291
				__e = __c.Set("class", "text-sm text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("This order is "); if __e != nil { return }
//line order.gox:292
				__e = __c.Any(string(o.Status)); if __e != nil { return }
				__e = __c.Text(" and closed. Nothing more can be done with it."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line order.gox:295
}
