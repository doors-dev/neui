// Managed by GoX v0.4.0

//line orders.gox:1
package main

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

type ordersToolInput struct {
	Status string `json:"status,omitempty" description:"paid, unpaid, refunded or cancelled; empty for all"`
	From string `json:"from,omitempty" description:"first order date, YYYY-MM-DD; empty for no limit"`
	To string `json:"to,omitempty" description:"last order date, YYYY-MM-DD; empty for no limit"`
	Search string `json:"search,omitempty" description:"order id, customer name or email, or part of the address"`
	Sort string `json:"sort,omitempty" description:"date, total, customer or id; prefix - for descending; empty for newest first"`
	Page int `json:"page,omitempty" description:"page of the table, 1-based; 0 for the first"`
}

type ordersToolOutput struct {
	Shown []order `json:"shown" description:"orders now on the user's screen"`
	Page pageInfo `json:"page"`
}

type itemInput struct {
	Product int `json:"product" required:"true" description:"product id"`
	Qty int `json:"qty" required:"true" description:"units, at least 1"`
}

type createOrderInput struct {
	Customer int `json:"customer" required:"true" description:"customer id"`
	Items []itemInput `json:"items" required:"true" description:"products and quantities, each product once"`
	Status string `json:"status,omitempty" description:"paid or unpaid; empty for unpaid"`
	Address string `json:"address,omitempty" description:"shipping address; empty for the customer's address on file"`
}

type created struct {
	ID int `json:"id" description:"id of the new record"`
}

func (p OrdersPath) query() orderQuery {
	return orderQuery{Status: status(p.Status), From: p.From, To: p.To, Search: p.Search, Sort: p.Sort}
}

func (p OrdersPath) filters() OrdersPath {
	return OrdersPath{Status: p.Status, From: p.From, To: p.To, Search: p.Search}
}

func (p OrdersPath) withSort(sort string) any {
	p.Sort, p.Page = sort, 0
	return p
}

func (p OrdersPath) withPage(page int) any {
	p.Page = page
	if page == 1 {
		p.Page = 0
	}
	return p
}

//line orders.gox:68
func ordersPage(path doors.Source[OrdersPath]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orders.gox:69
		__e = (neui.Space{
		Name: "orders",
		Desc: "Orders list: filters, a sortable table of orders and its pages. The filters live in the address, so the user can share or reload them",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:73
				__e = __c.Any(pageHeader{
			Title: "Orders",
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Every order, newest first. Click a column to sort."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orders.gox:75
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line orders.gox:77
					__e = (neui.Use{Desc: "open the form to create a new order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line orders.gox:78
							__e = __c.Modify(doors.ALink{Model: OrdersPath{View: ListNew}, Batch: true, After: scrollTop}); if __e != nil { return }
//line orders.gox:79
							__e = __c.Set("class", btnPrimary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orders.gox:80
							__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
							__e = __c.Text("New order"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orders.gox:83
			return }),
		}); if __e != nil { return }
//line orders.gox:85
				__e = __c.Any(neui.Tool[createOrderInput, created]{
			Name: "create_order",
			Desc: "Create an order, checked like the New order form, and open its page for the user. Stock is taken when it is created",
			Handler: func(ctx context.Context, in createOrderInput) (created, error) {
				f := orderForm{Customer: strconv.Itoa(in.Customer), Address: in.Address, Status: cmp.Or(in.Status, string(unpaid))}
				for _, it := range in.Items {
					f.Lines = append(f.Lines, lineForm{Product: strconv.Itoa(it.Product), Qty: strconv.Itoa(it.Qty)})
				}
				id, errs := shop.SaveOrder(ctx, 0, f)
				if err := errs.err(); err != nil {
					return created{}, err
				}
				notify(ctx, fmt.Sprintf("Order #%d created", id))
				navigate(ctx, OrderPath{ID: id})
				return created{ID: id}, nil
			},
		}); if __e != nil { return }
				__e = __c.Init("section"); if __e != nil { return }
				{
//line orders.gox:102
					__e = __c.Set("class", "overflow-hidden rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line orders.gox:103
					__e = __c.Any(orderFilters(path)); if __e != nil { return }
//line orders.gox:104
					__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line orders.gox:104
						__e = __c.Any(func() any {
				_, _ = shop.version.Effect(ctx)
				p, ok := path.Effect(ctx)
				if !ok {
					return nil
				}
				return ordersTable(p)
			}()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line orders.gox:114
}

func checkDate(name string, v string) error {
	if v == "" {
		return nil
	}
	if _, err := time.Parse(time.DateOnly, v); err != nil {
		return fmt.Errorf("%s %q is not a date in the form YYYY-MM-DD", name, v)
	}
	return nil
}

func validStatus(s string) bool {
	for _, v := range statuses {
		if string(v) == s {
			return true
		}
	}
	return false
}

//line orders.gox:135
func orderFilters(path doors.Source[OrdersPath]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orders.gox:137
		set := func(ctx context.Context, change func(p *OrdersPath)) {
		path.Mutate(ctx, func(p OrdersPath) OrdersPath {
			change(&p)
			p.Page = 0
			return p
		})
	}
	options := []option{}
	for _, s := range statuses {
		options = append(options, option{Value: string(s), Label: "Status: " + strings.ToUpper(string(s[:1])) + string(s[1:])})
	}
	active := doors.DeriveBeam(path, func(p OrdersPath) bool {
		return p.filters() != OrdersPath{}
	})

//line orders.gox:152
		__e = (neui.Space{
		Name: "filters",
		Desc: "Filters of the orders table: search, status and date range; each applies as soon as it changes",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line orders.gox:155
				__e = __c.Set("class", "flex flex-col gap-3 border-b border-stone-100 bg-stone-50/50 px-5 py-4 @4xl:flex-row @4xl:items-center"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:156
				__e = __c.Any(neui.Tool[ordersToolInput, ordersToolOutput]{
			Name: "filter_orders",
			Desc: "Set the filters, sort and page of the orders table the user sees, replacing all of them; an empty field clears it",
			Handler: func(ctx context.Context, in ordersToolInput) (ordersToolOutput, error) {
				next := OrdersPath{Status: in.Status, From: in.From, To: in.To, Search: in.Search, Sort: in.Sort, Page: in.Page}
				if next.Status != "" && !validStatus(next.Status) {
					return ordersToolOutput{}, fmt.Errorf("unknown status %q", in.Status)
				}
				if err := cmp.Or(checkDate("from", next.From), checkDate("to", next.To), checkSort(next.Sort, "date", "total", "customer", "id")); err != nil {
					return ordersToolOutput{}, err
				}
				if next.From != "" && next.To != "" && next.From > next.To {
					return ordersToolOutput{}, fmt.Errorf("from %s is after to %s", next.From, next.To)
				}
				shown, info := paginate(shop.Orders(next.query()), next.Page, pageSize)
				next.Page = info.Page
				if next.Page == 1 {
					next.Page = 0
				}
				path.Update(ctx, next)
				return ordersToolOutput{Shown: shown, Page: info}, nil
			},
		}); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line orders.gox:179
					__e = __c.Set("class", "relative flex-1 @4xl:min-w-56"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("label"); if __e != nil { return }
					{
//line orders.gox:180
						__e = __c.Set("for", "orders-search"); if __e != nil { return }
//line orders.gox:180
						__e = __c.Set("class", "sr-only"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Search orders"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line orders.gox:181
						__e = __c.Set("class", "pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-stone-400"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orders.gox:182
						__e = __c.Any(icon(iconSearch, "size-4")); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orders.gox:184
					__e = __c.Any(keepTyped(doors.DeriveBeam(path, func(p OrdersPath) string {
				return p.Search
			}), func(v string, typed func(v string)) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line orders.gox:188
						__e = (neui.Use{Desc: "search orders by id, customer name, email or address; applies on Enter or when the field loses focus"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.InitVoid("input"); if __e != nil { return }
							{
//line orders.gox:189
								__e = __c.Modify(doors.AChange{
							Race: doors.RaceLatest,
							On: func(ctx context.Context, r doors.RequestChange) bool {
								typed(r.Event().Value)
								set(ctx, func(p *OrdersPath) {
									p.Search = r.Event().Value
								})
								return false
							},
						}); if __e != nil { return }
//line orders.gox:199
								__e = __c.Set("id", "orders-search"); if __e != nil { return }
//line orders.gox:200
								__e = __c.Set("type", "search"); if __e != nil { return }
//line orders.gox:201
								__e = __c.Set("placeholder", "Search by order, customer, email or address"); if __e != nil { return }
//line orders.gox:202
								__e = __c.Set("value", v); if __e != nil { return }
//line orders.gox:203
								__e = __c.Set("class", inputBase.Add("pl-9")); if __e != nil { return }
							}
							__e = __c.Submit(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orders.gox:204
				return })
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line orders.gox:207
					__e = __c.Set("class", "grid grid-cols-1 gap-3 @xl:grid-cols-3 @4xl:flex @4xl:items-center"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line orders.gox:208
						__e = __c.Set("class", "@4xl:w-44"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orders.gox:209
						__e = __c.Any(keepTyped(doors.DeriveBeam(path, func(p OrdersPath) string {
					return p.Status
				}), func(v string, typed func(v string)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.InitContainer(); if __e != nil { return }
						{
//line orders.gox:213
							__e = __c.Any(selectField{
							Label: "Status",
							Hide: true,
							Name: "status",
							Value: v,
							Placeholder: "All statuses",
							Options: options,
							Desc: "show only orders with this status",
							Change: func(ctx context.Context, v string) {
								typed(v)
								set(ctx, func(p *OrdersPath) {
									p.Status = v
								})
							},
						}); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line orders.gox:228
					return })
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orders.gox:231
					__e = __c.Any(keepTyped(doors.DeriveBeam(path, func(p OrdersPath) string {
				return p.From
			}), func(v string, typed func(v string)) gox.Elem {
				return dateFilter("from", "From", v, "show orders on or after this date", func(ctx context.Context, v string) {
					typed(v)
					set(ctx, func(p *OrdersPath) {
						p.From = v
					})
				})
			})); if __e != nil { return }
//line orders.gox:241
					__e = __c.Any(keepTyped(doors.DeriveBeam(path, func(p OrdersPath) string {
				return p.To
			}), func(v string, typed func(v string)) gox.Elem {
				return dateFilter("to", "To", v, "show orders on or before this date", func(ctx context.Context, v string) {
					typed(v)
					set(ctx, func(p *OrdersPath) {
						p.To = v
					})
				})
			})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orders.gox:252
				__e = __c.Any(active.Bind(func(on bool) gox.Elem {
			if !on {
				return nil
			}
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line orders.gox:257
					__e = (neui.Use{Desc: "clear every filter and the search"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line orders.gox:258
							__e = __c.Modify(doors.ALink{Model: OrdersPath{}, Batch: true}); if __e != nil { return }
//line orders.gox:259
							__e = __c.Set("class", btnGhost); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line orders.gox:260
							__e = __c.Any(icon(iconClose, "size-4")); if __e != nil { return }
							__e = __c.Text("Clear"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line orders.gox:263
			return })
		})); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line orders.gox:266
}

//line orders.gox:268
func keepTyped(value doors.Beam[string], field func(v string, typed func(v string)) gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orders.gox:270
		door := new(doors.Door)
	var mu sync.Mutex
	shown, _ := value.Read(ctx)
	first := shown
	typed := func(v string) {
		mu.Lock()
		shown = v
		mu.Unlock()
	}
	value.Sub(ctx, func(ctx context.Context, v string) bool {
		mu.Lock()
		changed := v != shown
		shown = v
		mu.Unlock()
		if changed {
			door.Inner(ctx, field(v, typed))
		}
		return false
	})

//line orders.gox:290
		__e = (door).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line orders.gox:290
				__e = __c.Any(field(first, typed)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line orders.gox:291
}

//line orders.gox:293
func dateFilter(name string, label string, value string, desc string, change func(ctx context.Context, v string)) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("label"); if __e != nil { return }
		{
//line orders.gox:294
			__e = __c.Set("class", "flex items-center gap-2 text-sm text-stone-500"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line orders.gox:295
				__e = __c.Set("class", "w-10 shrink-0 @4xl:w-auto"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:295
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line orders.gox:296
			__e = (neui.Use{Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line orders.gox:297
					__e = __c.Modify(doors.AChange{
				Scope: &doors.ScopeDebounce{Duration: 400 * time.Millisecond},
				Race: doors.RaceLatest,
				On: func(ctx context.Context, r doors.RequestChange) bool {
					change(ctx, r.Event().Value)
					return false
				},
			}); if __e != nil { return }
//line orders.gox:305
					__e = __c.Set("type", "date"); if __e != nil { return }
//line orders.gox:306
					__e = __c.Set("name", name); if __e != nil { return }
//line orders.gox:307
					__e = __c.Set("value", value); if __e != nil { return }
//line orders.gox:308
					__e = __c.Set("class", inputBase.Add("@4xl:w-40")); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orders.gox:310
}

//line orders.gox:312
func ordersTable(p OrdersPath) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line orders.gox:314
		all := shop.Orders(p.query())
	rows, info := paginate(all, p.Page, pageSize)
	sort := p.Sort
	if sort == "" {
		sort = "-date"
	}

//line orders.gox:321
		__e = (neui.Space{
		Name: "table",
		Desc: "Orders matching the filters, one page at a time; the column headers sort it",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:325
				__e = __c.Any(neui.Data{Text: "orders shown in the table", Object: rows}); if __e != nil { return }
//line orders.gox:326
				__e = __c.Any(neui.Data{Text: fmt.Sprintf("page %d of %d; %d orders match the filters", info.Page, info.Pages, info.Total)}); if __e != nil { return }
//line orders.gox:327
				if len(all) == 0 {
//line orders.gox:328
					__e = __c.Any(emptyState{
				Icon: iconSearch,
				Title: "No orders match",
				Body: "Try another status, a wider date range or a shorter search.",
				Action: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line orders.gox:333
						__e = (neui.Use{Desc: "clear every filter and the search"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line orders.gox:333
								__e = __c.Modify(doors.ALink{Model: OrdersPath{}, Batch: true}); if __e != nil { return }
//line orders.gox:333
								__e = __c.Set("class", btnSecondary); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Clear filters"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line orders.gox:334
				return }),
			}); if __e != nil { return }
				} else  {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line orders.gox:337
						__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("table"); if __e != nil { return }
						{
//line orders.gox:338
							__e = __c.Set("class", tableClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("thead"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("tr"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line orders.gox:341
									__e = __c.Any(sortHeader{Label: "Order", Key: "id", First: "-id", Current: sort, Model: p.withSort}); if __e != nil { return }
//line orders.gox:342
									__e = __c.Any(sortHeader{Label: "Customer", Key: "customer", First: "customer", Current: sort, Model: p.withSort}); if __e != nil { return }
//line orders.gox:343
									__e = __c.Any(sortHeader{Label: "Date", Key: "date", First: "-date", Current: sort, Class: "hidden @xl:table-cell", Model: p.withSort}); if __e != nil { return }
									__e = __c.Init("th"); if __e != nil { return }
									{
//line orders.gox:344
										__e = __c.Set("class", thClass.Add("hidden @xl:table-cell")); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Text("Status"); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line orders.gox:345
									__e = __c.Any(sortHeader{Label: "Total", Key: "total", First: "-total", Current: sort, Right: true, Model: p.withSort}); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("tbody"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line orders.gox:349
								for _, o := range rows {
//line orders.gox:350
									__e = __c.Any(orderRow(o)); if __e != nil { return }
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
//line orders.gox:357
		if len(all) > 0 {
//line orders.gox:358
			__e = __c.Any(pager{Info: info, Noun: "orders", Model: p.withPage}); if __e != nil { return }
		}
	return })
//line orders.gox:360
}

//line orders.gox:362
func orderRow(o order) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("tr"); if __e != nil { return }
		{
//line orders.gox:363
			__e = __c.Set("class", rowClass); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("td"); if __e != nil { return }
			{
//line orders.gox:364
				__e = __c.Set("class", tdClass); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:365
				__e = (neui.Use{Desc: fmt.Sprintf("open order #%d", o.ID)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("a"); if __e != nil { return }
					{
//line orders.gox:365
						__e = __c.Modify(doors.ALink{Model: OrderPath{ID: o.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line orders.gox:365
						__e = __c.Set("class", linkText); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orders.gox:366
						__e = __c.Many("#", o.ID); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line orders.gox:368
					__e = __c.Set("class", "mt-1 @xl:hidden"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line orders.gox:368
					__e = __c.Any(badge(o.Status)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("td"); if __e != nil { return }
			{
//line orders.gox:370
				__e = __c.Set("class", tdClass); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:371
				__e = (neui.Use{Desc: fmt.Sprintf("open customer %s of order #%d", o.Customer, o.ID)}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("a"); if __e != nil { return }
					{
//line orders.gox:372
						__e = __c.Modify(doors.ALink{Model: CustomerPath{ID: o.CustomerID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line orders.gox:373
						__e = __c.Set("class", "text-stone-700 hover:text-stone-900 hover:underline"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line orders.gox:374
						__e = __c.Any(o.Customer); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("p"); if __e != nil { return }
				{
//line orders.gox:376
					__e = __c.Set("class", "mt-0.5 text-xs text-stone-500 tabular-nums @xl:hidden"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line orders.gox:376
					__e = __c.Any(o.Date); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("td"); if __e != nil { return }
			{
//line orders.gox:378
				__e = __c.Set("class", tdClass.Add("hidden text-stone-500 tabular-nums @xl:table-cell")); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:378
				__e = __c.Any(o.Date); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("td"); if __e != nil { return }
			{
//line orders.gox:379
				__e = __c.Set("class", tdClass.Add("hidden @xl:table-cell")); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:379
				__e = __c.Any(badge(o.Status)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("td"); if __e != nil { return }
			{
//line orders.gox:380
				__e = __c.Set("class", tdClass.Add("text-right font-medium tabular-nums")); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line orders.gox:380
				__e = __c.Any(money(o.Total)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line orders.gox:382
}
