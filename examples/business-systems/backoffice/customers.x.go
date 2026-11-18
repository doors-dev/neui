// Managed by GoX v0.4.0

//line customers.gox:1
package main

import (
	"context"
	"fmt"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

type customersToolInput struct {
	Search string `json:"search,omitempty" description:"part of a name, email, company or phone; empty for all"`
	Sort string `json:"sort,omitempty" description:"name, orders or spent; prefix - for descending; empty for by name"`
	Page int `json:"page,omitempty" description:"page of the table, 1-based; 0 for the first"`
}

type customersToolOutput struct {
	Shown []customerRow `json:"shown" description:"customers now on the user's screen"`
	Page pageInfo `json:"page"`
}

type customerInput struct {
	Name string `json:"name" required:"true"`
	Email string `json:"email" required:"true"`
	Phone string `json:"phone,omitempty"`
	Company string `json:"company,omitempty"`
	Address string `json:"address" required:"true" description:"default shipping address"`
	Notes string `json:"notes,omitempty" description:"internal notes"`
}

func (p CustomersPath) withSort(sort string) any {
	p.Sort, p.Page = sort, 0
	return p
}

func (p CustomersPath) withPage(page int) any {
	p.Page = page
	if page == 1 {
		p.Page = 0
	}
	return p
}

//line customers.gox:45
func customersPage(path doors.Source[CustomersPath]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customers.gox:47
		search := doors.DeriveBeam(path, func(p CustomersPath) string {
		return p.Search
	})

//line customers.gox:51
		__e = (neui.Space{
		Name: "customers",
		Desc: "Customers list: a search box, a sortable table of customers and its pages; the search lives in the address",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line customers.gox:55
				__e = __c.Any(pageHeader{
			Title: "Customers",
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("People and companies who buy from the shop."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customers.gox:57
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line customers.gox:59
					__e = (neui.Use{Desc: "open the form to add a new customer"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line customers.gox:60
							__e = __c.Modify(doors.ALink{Model: CustomersPath{View: ListNew}, Batch: true, After: scrollTop}); if __e != nil { return }
//line customers.gox:61
							__e = __c.Set("class", btnPrimary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line customers.gox:62
							__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
							__e = __c.Text("New customer"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customers.gox:65
			return }),
		}); if __e != nil { return }
//line customers.gox:67
				__e = __c.Any(neui.Tool[customerInput, created]{
			Name: "create_customer",
			Desc: "Add a customer, checked like the New customer form, and open their page for the user",
			Handler: func(ctx context.Context, in customerInput) (created, error) {
				id, errs := shop.SaveCustomer(ctx, 0, customerForm(in))
				if err := errs.err(); err != nil {
					return created{}, err
				}
				notify(ctx, "Customer added")
				navigate(ctx, CustomerPath{ID: id})
				return created{ID: id}, nil
			},
		}); if __e != nil { return }
				__e = __c.Init("section"); if __e != nil { return }
				{
//line customers.gox:80
					__e = __c.Set("class", "overflow-hidden rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customers.gox:81
					__e = (neui.Space{Name: "search", Desc: "Search box of the customers table"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("div"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line customers.gox:82
							__e = __c.Any(neui.Tool[customersToolInput, customersToolOutput]{
					Name: "search_customers",
					Desc: "Set the search, sort and page of the customers table the user sees; an empty field clears it",
					Handler: func(ctx context.Context, in customersToolInput) (customersToolOutput, error) {
						next := CustomersPath{Search: in.Search, Sort: in.Sort, Page: in.Page}
						if err := checkSort(next.Sort, "name", "orders", "spent"); err != nil {
							return customersToolOutput{}, err
						}
						shown, info := paginate(shop.Customers(next.Search, next.Sort), next.Page, pageSize)
						next.Page = info.Page
						if next.Page == 1 {
							next.Page = 0
						}
						path.Update(ctx, next)
						return customersToolOutput{Shown: shown, Page: info}, nil
					},
				}); if __e != nil { return }
//line customers.gox:99
							__e = __c.Any(search.Bind(func(q string) gox.Elem {
					return searchBar("customers-search", "Search by name, email, company or phone", q, "search customers by name, email, company or phone; applies on Enter or when the field loses focus", func(ctx context.Context, v string) {
						path.Mutate(ctx, func(p CustomersPath) CustomersPath {
							p.Search, p.Page = v, 0
							return p
						})
					})
				})); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
//line customers.gox:108
					__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line customers.gox:108
						__e = __c.Any(func() any {
				_, _ = shop.version.Effect(ctx)
				p, ok := path.Effect(ctx)
				if !ok {
					return nil
				}
				return customersTable(p)
			}()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line customers.gox:118
}

//line customers.gox:120
func searchBar(id string, placeholder string, value string, desc string, change func(ctx context.Context, v string)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line customers.gox:121
			__e = __c.Set("class", "border-b border-stone-100 bg-stone-50/50 px-5 py-4"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line customers.gox:122
				__e = __c.Set("class", "relative max-w-md"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("label"); if __e != nil { return }
				{
//line customers.gox:123
					__e = __c.Set("for", id); if __e != nil { return }
//line customers.gox:123
					__e = __c.Set("class", "sr-only"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customers.gox:123
					__e = __c.Any(placeholder); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line customers.gox:124
					__e = __c.Set("class", "pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-stone-400"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customers.gox:125
					__e = __c.Any(icon(iconSearch, "size-4")); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customers.gox:127
				__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line customers.gox:128
						__e = __c.Modify(doors.AChange{
					Race: doors.RaceLatest,
					On: func(ctx context.Context, r doors.RequestChange) bool {
						change(ctx, r.Event().Value)
						return false
					},
				}); if __e != nil { return }
//line customers.gox:135
						__e = __c.Set("id", id); if __e != nil { return }
//line customers.gox:136
						__e = __c.Set("type", "search"); if __e != nil { return }
//line customers.gox:137
						__e = __c.Set("placeholder", placeholder); if __e != nil { return }
//line customers.gox:138
						__e = __c.Set("value", value); if __e != nil { return }
//line customers.gox:139
						__e = __c.Set("class", inputBase.Add("pl-9")); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line customers.gox:142
}

//line customers.gox:144
func customersTable(p CustomersPath) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customers.gox:146
		all := shop.Customers(p.Search, p.Sort)
	rows, info := paginate(all, p.Page, pageSize)
	sort := p.Sort
	if sort == "" {
		sort = "name"
	}

//line customers.gox:153
		__e = (neui.Space{Name: "table", Desc: "Customers matching the search, one page at a time; the column headers sort it"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line customers.gox:154
				__e = __c.Any(neui.Data{Text: "customers shown in the table, with their order count and paid total", Object: rows}); if __e != nil { return }
//line customers.gox:155
				__e = __c.Any(neui.Data{Text: fmt.Sprintf("page %d of %d; %d customers match", info.Page, info.Pages, info.Total)}); if __e != nil { return }
//line customers.gox:156
				if len(all) == 0 {
//line customers.gox:157
					__e = __c.Any(emptyState{
				Icon: iconCustomers,
				Title: "No customers found",
				Body: "Nobody matches this search. Check the spelling or add them as a new customer.",
				Action: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line customers.gox:162
						__e = (neui.Use{Desc: "clear the search"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line customers.gox:162
								__e = __c.Modify(doors.ALink{Model: CustomersPath{}, Batch: true}); if __e != nil { return }
//line customers.gox:162
								__e = __c.Set("class", btnSecondary); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Clear search"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line customers.gox:163
				return }),
			}); if __e != nil { return }
				} else  {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line customers.gox:166
						__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("table"); if __e != nil { return }
						{
//line customers.gox:167
							__e = __c.Set("class", tableClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("thead"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("tr"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line customers.gox:170
									__e = __c.Any(sortHeader{Label: "Name", Key: "name", First: "name", Current: sort, Model: p.withSort}); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line customers.gox:171
										__e = __c.Set("class", thClass.Add("hidden @2xl:table-cell")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Company"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line customers.gox:172
										__e = __c.Set("class", thClass.Add("hidden @4xl:table-cell")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Phone"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line customers.gox:173
									__e = __c.Any(sortHeader{Label: "Orders", Key: "orders", First: "-orders", Current: sort, Right: true, Class: "hidden @xl:table-cell", Model: p.withSort}); if __e != nil { return }
//line customers.gox:174
									__e = __c.Any(sortHeader{Label: "Spent", Key: "spent", First: "-spent", Current: sort, Right: true, Model: p.withSort}); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("tbody"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line customers.gox:178
								for _, c := range rows {
									__e = __c.Init("tr"); if __e != nil { return }
									{
//line customers.gox:179
										__e = __c.Set("class", rowClass); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line customers.gox:180
											__e = __c.Set("class", tdClass); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customers.gox:181
											__e = (neui.Use{Desc: "open customer " + c.Name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
												ctx := __c.Context(); _ = ctx
												__e = __c.Init("a"); if __e != nil { return }
												{
//line customers.gox:181
													__e = __c.Modify(doors.ALink{Model: CustomerPath{ID: c.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line customers.gox:181
													__e = __c.Set("class", "group flex items-center gap-3"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line customers.gox:182
													__e = __c.Any(avatar(c.Name)); if __e != nil { return }
													__e = __c.Init("span"); if __e != nil { return }
													{
//line customers.gox:183
														__e = __c.Set("class", "min-w-0"); if __e != nil { return }
														__e = __c.Submit(); if __e != nil { return }
														__e = __c.Init("span"); if __e != nil { return }
														{
//line customers.gox:184
															__e = __c.Set("class", "block font-medium text-stone-900 group-hover:underline"); if __e != nil { return }
															__e = __c.Submit(); if __e != nil { return }
//line customers.gox:184
															__e = __c.Any(c.Name); if __e != nil { return }
														}
														__e = __c.Close(); if __e != nil { return }
														__e = __c.Init("span"); if __e != nil { return }
														{
//line customers.gox:185
															__e = __c.Set("class", "block text-xs text-stone-500"); if __e != nil { return }
															__e = __c.Submit(); if __e != nil { return }
//line customers.gox:185
															__e = __c.Any(c.Email); if __e != nil { return }
														}
														__e = __c.Close(); if __e != nil { return }
													}
													__e = __c.Close(); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											return })); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line customers.gox:189
											__e = __c.Set("class", tdClass.Add("hidden text-stone-600 @2xl:table-cell")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customers.gox:189
											__e = __c.Any(dash(c.Company)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line customers.gox:190
											__e = __c.Set("class", tdClass.Add("hidden text-stone-600 tabular-nums @4xl:table-cell")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customers.gox:190
											__e = __c.Any(dash(c.Phone)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line customers.gox:191
											__e = __c.Set("class", tdClass.Add("hidden text-right tabular-nums @xl:table-cell")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customers.gox:191
											__e = __c.Any(c.Orders); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line customers.gox:192
											__e = __c.Set("class", tdClass.Add("text-right font-medium tabular-nums")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line customers.gox:192
											__e = __c.Any(money(c.Spent)); if __e != nil { return }
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
				}
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
//line customers.gox:200
		if len(all) > 0 {
//line customers.gox:201
			__e = __c.Any(pager{Info: info, Noun: "customers", Model: p.withPage}); if __e != nil { return }
		}
	return })
//line customers.gox:203
}

//line customers.gox:205
func avatar(name string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line customers.gox:206
			__e = __c.Set("class", "flex size-9 shrink-0 items-center justify-center rounded-full bg-amber-100 text-xs font-semibold text-amber-800"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line customers.gox:207
			__e = __c.Any(initials(name)); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line customers.gox:209
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
