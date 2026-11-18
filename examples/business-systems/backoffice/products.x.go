// Managed by GoX v0.4.0

//line products.gox:1
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
)

type productsToolInput struct {
	Search string `json:"search,omitempty" description:"part of a product name or SKU; empty for all"`
	Sort string `json:"sort,omitempty" description:"name, sku, price or stock; prefix - for descending; empty for by name"`
	Page int `json:"page,omitempty" description:"page of the table, 1-based; 0 for the first"`
}

type productsToolOutput struct {
	Shown []product `json:"shown" description:"products now on the user's screen"`
	Page pageInfo `json:"page"`
}

func (p ProductsPath) withSort(sort string) any {
	p.Sort, p.Page = sort, 0
	return p
}

func (p ProductsPath) withPage(page int) any {
	p.Page = page
	if page == 1 {
		p.Page = 0
	}
	return p
}

//line products.gox:50
func productsPage(path doors.Source[ProductsPath]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line products.gox:52
		search := doors.DeriveBeam(path, func(p ProductsPath) string {
		return p.Search
	})

//line products.gox:56
		__e = (neui.Space{
		Name: "products",
		Desc: "Products list: a search box, a sortable table of price and stock and its pages; sort by stock to see what runs low",
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line products.gox:60
				__e = __c.Any(pageHeader{
			Title: "Products",
			Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line products.gox:62
					__e = __c.Many("What the shop sells. Items with ", lowStock, " or fewer units are flagged."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line products.gox:62
			return }),
			Actions: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.InitContainer(); if __e != nil { return }
				{
//line products.gox:64
					__e = (neui.Use{Desc: "open the form to add a new product"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("a"); if __e != nil { return }
						{
//line products.gox:65
							__e = __c.Modify(doors.ALink{Model: ProductsPath{View: ListNew}, Batch: true, After: scrollTop}); if __e != nil { return }
//line products.gox:66
							__e = __c.Set("class", btnPrimary); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line products.gox:67
							__e = __c.Any(icon(iconPlus, "size-4")); if __e != nil { return }
							__e = __c.Text("New product"); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line products.gox:70
			return }),
		}); if __e != nil { return }
				__e = __c.Init("section"); if __e != nil { return }
				{
//line products.gox:72
					__e = __c.Set("class", "overflow-hidden rounded-xl bg-white shadow-xs ring-1 ring-stone-200"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line products.gox:73
					__e = (neui.Space{Name: "search", Desc: "Search box of the products table"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("div"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line products.gox:74
							__e = __c.Any(neui.Tool[productsToolInput, productsToolOutput]{
					Name: "search_products",
					Desc: "Set the search, sort and page of the products table the user sees; an empty field clears it",
					Handler: func(ctx context.Context, in productsToolInput) (productsToolOutput, error) {
						next := ProductsPath{Search: in.Search, Sort: in.Sort, Page: in.Page}
						if err := checkSort(next.Sort, "name", "sku", "price", "stock"); err != nil {
							return productsToolOutput{}, err
						}
						shown, info := paginate(shop.Products(next.Search, next.Sort), next.Page, pageSize)
						next.Page = info.Page
						if next.Page == 1 {
							next.Page = 0
						}
						path.Update(ctx, next)
						return productsToolOutput{Shown: shown, Page: info}, nil
					},
				}); if __e != nil { return }
//line products.gox:91
							__e = __c.Any(search.Bind(func(q string) gox.Elem {
					return searchBar("products-search", "Search by name or SKU", q, "search products by name or SKU; applies on Enter or when the field loses focus", func(ctx context.Context, v string) {
						path.Mutate(ctx, func(p ProductsPath) ProductsPath {
							p.Search, p.Page = v, 0
							return p
						})
					})
				})); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
//line products.gox:100
					__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line products.gox:100
						__e = __c.Any(func() any {
				_, _ = shop.version.Effect(ctx)
				p, ok := path.Effect(ctx)
				if !ok {
					return nil
				}
				return productsTable(p)
			}()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line products.gox:110
}

//line products.gox:112
func productsTable(p ProductsPath) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line products.gox:114
		all := shop.Products(p.Search, p.Sort)
	rows, info := paginate(all, p.Page, pageSize)
	sort := p.Sort
	if sort == "" {
		sort = "name"
	}

//line products.gox:121
		__e = (neui.Space{Name: "table", Desc: "Products matching the search with price and stock, one page at a time; the column headers sort it"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
//line products.gox:122
				__e = __c.Any(neui.Data{Text: "products shown in the table", Object: rows}); if __e != nil { return }
//line products.gox:123
				__e = __c.Any(neui.Data{Text: fmt.Sprintf("page %d of %d; %d products match", info.Page, info.Pages, info.Total)}); if __e != nil { return }
//line products.gox:124
				if len(all) == 0 {
//line products.gox:125
					__e = __c.Any(emptyState{
				Icon: iconProducts,
				Title: "No products found",
				Body: "Nothing matches this search.",
				Action: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line products.gox:130
						__e = (neui.Use{Desc: "clear the search"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("a"); if __e != nil { return }
							{
//line products.gox:130
								__e = __c.Modify(doors.ALink{Model: ProductsPath{}, Batch: true}); if __e != nil { return }
//line products.gox:130
								__e = __c.Set("class", btnSecondary); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Clear search"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line products.gox:131
				return }),
			}); if __e != nil { return }
				} else  {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line products.gox:134
						__e = __c.Set("class", "overflow-x-auto"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("table"); if __e != nil { return }
						{
//line products.gox:135
							__e = __c.Set("class", tableClass); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("thead"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("tr"); if __e != nil { return }
								{
									__e = __c.Submit(); if __e != nil { return }
//line products.gox:138
									__e = __c.Any(sortHeader{Label: "Product", Key: "name", First: "name", Current: sort, Model: p.withSort}); if __e != nil { return }
//line products.gox:139
									__e = __c.Any(sortHeader{Label: "SKU", Key: "sku", First: "sku", Current: sort, Class: "hidden @xl:table-cell", Model: p.withSort}); if __e != nil { return }
//line products.gox:140
									__e = __c.Any(sortHeader{Label: "Price", Key: "price", First: "-price", Current: sort, Right: true, Model: p.withSort}); if __e != nil { return }
//line products.gox:141
									__e = __c.Any(sortHeader{Label: "Stock", Key: "stock", First: "stock", Current: sort, Right: true, Model: p.withSort}); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
							__e = __c.Init("tbody"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line products.gox:145
								for _, it := range rows {
									__e = __c.Init("tr"); if __e != nil { return }
									{
//line products.gox:146
										__e = __c.Set("class", rowClass); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line products.gox:147
											__e = __c.Set("class", tdClass); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line products.gox:148
											__e = (neui.Use{Desc: "open product " + it.Name + " to edit it"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
												ctx := __c.Context(); _ = ctx
												__e = __c.Init("a"); if __e != nil { return }
												{
//line products.gox:148
													__e = __c.Modify(doors.ALink{Model: ProductPath{ID: it.ID}, Batch: true, After: scrollTop}); if __e != nil { return }
//line products.gox:148
													__e = __c.Set("class", "font-medium text-stone-900 hover:underline"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line products.gox:149
													__e = __c.Any(it.Name); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											return })); if __e != nil { return }
											__e = __c.Init("p"); if __e != nil { return }
											{
//line products.gox:151
												__e = __c.Set("class", "text-xs text-stone-500 @xl:hidden"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
//line products.gox:151
												__e = __c.Any(it.SKU); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line products.gox:153
											__e = __c.Set("class", tdClass.Add("hidden font-mono text-xs text-stone-500 @xl:table-cell")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line products.gox:153
											__e = __c.Any(it.SKU); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line products.gox:154
											__e = __c.Set("class", tdClass.Add("text-right tabular-nums")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line products.gox:154
											__e = __c.Any(money(it.Price)); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
										__e = __c.Init("td"); if __e != nil { return }
										{
//line products.gox:155
											__e = __c.Set("class", tdClass.Add("text-right")); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line products.gox:155
											__e = __c.Any(stockBadge(it.Stock)); if __e != nil { return }
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
//line products.gox:163
		if len(all) > 0 {
//line products.gox:164
			__e = __c.Any(pager{Info: info, Noun: "products", Model: p.withPage}); if __e != nil { return }
		}
	return })
//line products.gox:166
}
