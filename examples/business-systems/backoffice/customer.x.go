// Managed by GoX v0.4.0

//line customer.gox:1
package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line customer.gox:14
func customerPage(path doors.Source[CustomerPath]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customer.gox:16
		id := doors.DeriveBeam(path, func(p CustomerPath) int {
		return p.ID
	})
	page := doors.DeriveBeam(path, func(p CustomerPath) int {
		return p.Page
	})

//line customer.gox:23
		__e = __c.Any(id.Bind(func(id int) gox.Elem {
		return customerView(id, page)
	})); if __e != nil { return }
	return })
//line customer.gox:26
}

//line customer.gox:28
func customerView(id int, page doors.Beam[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customer.gox:29
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line customer.gox:29
			__e = __c.Any(func() any {
		_, ok := shop.version.Effect(ctx)
		if !ok {
			return nil
		}
		c, found := shop.Customer(id)
		if !found {
			return notFound(fmt.Sprintf("Customer %d", id), crumb{Label: "Back to customers", Model: CustomersPath{}})
		}
		return customerDetail(c, page)
	}()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line customer.gox:40
}

type customerPatch struct {
	Name *string `json:"name,omitempty" description:"omit to keep"`
	Email *string `json:"email,omitempty" description:"omit to keep"`
	Phone *string `json:"phone,omitempty" description:"omit to keep; empty to clear"`
	Company *string `json:"company,omitempty" description:"omit to keep; empty to clear"`
	Address *string `json:"address,omitempty" description:"default shipping address; omit to keep"`
	Notes *string `json:"notes,omitempty" description:"internal notes; omit to keep, empty to clear"`
}

func patch(to *string, from *string) {
	if from != nil {
		*to = *from
	}
}

//line customer.gox:57
func customerDetail(c customerRow, page doors.Beam[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customer.gox:59
		all := shop.Orders(orderQuery{Customer: c.ID})
	last := "—"
	if len(all) > 0 {
		last = shortDate(all[0].Date)
	}

//line customer.gox:65
		__e = (neui.Space{
		Name: "customer",
		Desc: fmt.Sprintf("Page of customer %s: totals, contact details and their orders", c.Name),
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line customer.gox:69
				__e = __c.Any(neui.Data{Text: "the customer on this page", Object: c.customer}); if __e != nil { return }
//line customer.gox:70
				__e = __c.Any(pageHeader{
			Crumbs: []crumb{{Label: "Customers", Model: CustomersPath{}}, {Label: c.Name, Model: CustomerPath{ID: c.ID}}},
			Title: c.Name,
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line customer.gox:74
					if c.Company != "" {
						__e = __c.Init("span"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line customer.gox:75
							__e = __c.Any(c.Company); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Init("span"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line customer.gox:77
						__e = __c.Any(c.Email); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customer.gox:78
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line customer.gox:80
					__e = (neui.Use{Desc: "open the edit form of this customer"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line customer.gox:81
							__e = __c.Modify(doors.ALink{Model: CustomerPath{View: ItemEdit, ID: c.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line customer.gox:82
							__e = __c.Set("class", btnSecondary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line customer.gox:83
							__e = __c.Any(icon(iconEdit, "size-4")); if __e != nil { return }
							__e = __c.Text("Edit"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
//line customer.gox:86
					__e = (neui.Use{Desc: "start a new order for this customer"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line customer.gox:87
							__e = __c.Modify(doors.ALink{Model: OrdersPath{View: ListNew, Customer: c.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line customer.gox:88
							__e = __c.Set("class", btnPrimary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line customer.gox:89
							__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
							__e = __c.Text("New order"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customer.gox:92
			return }),
		}); if __e != nil { return }
				__e = __c.Init("dl"); if __e != nil { return }
				{
//line customer.gox:94
					__e = __c.Set("class", "mb-6 grid grid-cols-3 divide-x divide-stone-100 rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customer.gox:95
					__e = __c.Any(stat("Orders", strconv.Itoa(c.Orders))); if __e != nil { return }
//line customer.gox:96
					__e = __c.Any(stat("Paid", money(c.Spent))); if __e != nil { return }
//line customer.gox:97
					__e = __c.Any(stat("Last order", last)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line customer.gox:99
					__e = __c.Set("class", "grid grid-cols-1 gap-6 @4xl:grid-cols-3"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customer.gox:100
					__e = (neui.Space{Name: "orders", Desc: "Orders of " + c.Name + ", newest first, with pages"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("div"); if __e != nil { return }
						{
//line customer.gox:100
							__e = __c.Set("class", "@4xl:col-span-2"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line customer.gox:101
							__e = __c.Any(card{
					Title: "Orders",
					Flush: true,
					Body: page.Bind(func(n int) gox.Elem {
						return customerOrders(c, all, n)
					}),
				}); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
//line customer.gox:109
					__e = (neui.Space{Name: "contact", Desc: "Contact details of " + c.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("div"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line customer.gox:110
							__e = __c.Any(neui.Tool[customerPatch, struct{}]{
					Name: "update_customer",
					Desc: "Change the contact details of this customer, checked like the edit form; omitted fields stay as they are",
					Handler: func(ctx context.Context, in customerPatch) (struct{}, error) {
						f := c.customer.form()
						patch(&f.Name, in.Name)
						patch(&f.Email, in.Email)
						patch(&f.Phone, in.Phone)
						patch(&f.Company, in.Company)
						patch(&f.Address, in.Address)
						patch(&f.Notes, in.Notes)
						if _, errs := shop.SaveCustomer(ctx, c.ID, f); len(errs) > 0 {
							return struct{}{}, errs.err()
						}
						notify(ctx, "Customer saved")
						return struct{}{}, nil
					},
				}); if __e != nil { return }
//line customer.gox:128
							__e = __c.Any(card{
					Title: "Contact",
					Body: dl(
						dlRow("Email", c.Email),
						dlRow("Phone", dash(c.Phone)),
						dlRow("Company", dash(c.Company)),
						dlRow("Address", c.Address),
						dlRow("Notes", dash(c.Notes)),
					),
				}); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line customer.gox:141
}

//line customer.gox:143
func customerOrders(c customerRow, all []order, n int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customer.gox:145
		rows, info := paginate(all, n, 8)

//line customer.gox:147
		__e = __c.Any(neui.Data{Text: fmt.Sprintf("orders of %s shown on this page of the table (page %d of %d)", c.Name, info.Page, info.Pages), Object: rows}); if __e != nil { return }
//line customer.gox:148
		if len(all) == 0 {
//line customer.gox:149
			__e = __c.Any(emptyState{
			Icon: iconOrders,
			Title: "No orders yet",
			Body: c.Name + " hasn't ordered anything yet.",
			Action: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line customer.gox:154
					__e = (neui.Use{Desc: "start the first order for " + c.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line customer.gox:154
							__e = __c.Modify(doors.ALink{Model: OrdersPath{View: ListNew, Customer: c.ID}, Batch: true}); if __e != nil { return }
//line customer.gox:154
							__e = __c.Set("class", btnSecondary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Text("Create their first order"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customer.gox:155
			return }),
		}); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line customer.gox:158
				__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("table"); if __e != nil { return }
				{
//line customer.gox:159
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
//line customer.gox:162
								__e = __c.Set("class", thClass); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Order"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("th"); if __e != nil { return }
							{
//line customer.gox:163
								__e = __c.Set("class", thClass.Add("hidden @xl:table-cell")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Date"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("th"); if __e != nil { return }
							{
//line customer.gox:164
								__e = __c.Set("class", thClass.Add("hidden @2xl:table-cell")); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Items"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("th"); if __e != nil { return }
							{
//line customer.gox:165
								__e = __c.Set("class", thClass); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Status"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("th"); if __e != nil { return }
							{
//line customer.gox:166
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
//line customer.gox:170
						for _, o := range rows {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line customer.gox:171
								__e = __c.Set("class", rowClass); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line customer.gox:172
									__e = __c.Set("class", tdClass); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line customer.gox:173
									__e = (neui.Use{Desc: fmt.Sprintf("open order #%d", o.ID)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
										ctx := __c.Context(); _ = ctx
										__e = __c.Init("a"); if __e != nil { return }
										{
//line customer.gox:173
											__e = __c.Modify(doors.ALink{Model: OrderPath{ID: o.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line customer.gox:173
											__e = __c.Set("class", linkText); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customer.gox:173
											__e = __c.Many("#", o.ID); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									return })); if __e != nil { return }
									__e = __c.Init("p"); if __e != nil { return }
									{
//line customer.gox:174
										__e = __c.Set("class", "mt-0.5 text-xs text-stone-500 tabular-nums @xl:hidden"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line customer.gox:174
										__e = __c.Any(o.Date); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line customer.gox:176
									__e = __c.Set("class", tdClass.Add("hidden text-stone-500 tabular-nums @xl:table-cell")); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line customer.gox:176
									__e = __c.Any(o.Date); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line customer.gox:177
									__e = __c.Set("class", tdClass.Add("hidden text-stone-500 @2xl:table-cell")); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line customer.gox:177
									__e = __c.Many(len(o.Lines), " item", plural(len(o.Lines))); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line customer.gox:178
									__e = __c.Set("class", tdClass); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line customer.gox:178
									__e = __c.Any(badge(o.Status)); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line customer.gox:179
									__e = __c.Set("class", tdClass.Add("text-right font-medium tabular-nums")); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line customer.gox:179
									__e = __c.Any(money(o.Total)); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line customer.gox:185
			__e = __c.Any(pager{
			Info: info,
			Noun: "orders",
			Model: func(n int) any {
				if n == 1 {
					n = 0
				}
				return CustomerPath{ID: c.ID, Page: n}
			},
		}); if __e != nil { return }
		}
	return })
//line customer.gox:196
}

//line customer.gox:198
func stat(label string, value string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line customer.gox:199
			__e = __c.Set("class", "px-4 py-3 @xl:px-5 @xl:py-4"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("dt"); if __e != nil { return }
			{
//line customer.gox:200
				__e = __c.Set("class", "text-xs font-medium text-stone-500"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line customer.gox:200
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("dd"); if __e != nil { return }
			{
//line customer.gox:201
				__e = __c.Set("class", "mt-1 truncate text-base font-semibold text-stone-900 tabular-nums @xl:text-xl"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line customer.gox:201
				__e = __c.Any(value); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line customer.gox:203
}

func shortDate(date string) string {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	if t.Year() == time.Now().Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}
