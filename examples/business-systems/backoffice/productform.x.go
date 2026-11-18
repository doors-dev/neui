// Managed by GoX v0.4.0

//line productform.gox:1
package main

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
)

//line productform.gox:14
func productFormPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line productform.gox:16
		p, found := shop.Product(id)
	initial := productForm{}
	if found {
		initial = p.form()
	}

//line productform.gox:22
		if id != 0 && !found {
//line productform.gox:23
			__e = __c.Any(notFound(fmt.Sprintf("Product %d", id), crumb{Label: "Back to products", Model: ProductsPath{}})); if __e != nil { return }
		} else  {
//line productform.gox:25
			__e = __c.Any(productEditor{ID: id, Initial: initial, Stock: p.Stock}); if __e != nil { return }
		}
	return })
//line productform.gox:27
}

type productEditor struct {
	ID int
	Initial productForm
	Stock int
}

//line productform.gox:35
func (e productEditor) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line productform.gox:37
		state := doors.NewSourceEqual(formState[productForm]{Values: e.Initial}, always)
	live := doors.NewSource(e.Stock)
	if e.ID != 0 {
		shop.version.Sub(ctx, func(ctx context.Context, _ int) bool {
			if p, ok := shop.Product(e.ID); ok {
				live.Update(ctx, p.Stock)
			}
			return false
		})
	}
	title := "New product"
	crumbs := []crumb{{Label: "Products", Model: ProductsPath{}}, {Label: "New", Model: ProductsPath{View: ListNew}}}
	desc := "Form adding a product: SKU, name, price and units in stock; press Create product to save"
	if e.ID != 0 {
		title = e.Initial.Name
		crumbs = []crumb{{Label: "Products", Model: ProductsPath{}}, {Label: e.Initial.Name, Model: ProductPath{ID: e.ID}}}
		desc = fmt.Sprintf("Form editing product %s (%s): price and stock among others; press Save changes to keep them", e.Initial.Name, e.Initial.SKU)
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line productform.gox:56
			__e = __c.Set("class", "mx-auto max-w-3xl"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line productform.gox:57
			__e = (neui.Space{Name: "product_form", Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line productform.gox:58
					__e = __c.Any(pageHeader{
				Crumbs: crumbs,
				Title: title,
				Meta: gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitContainer(); if __e != nil { return }
					{
//line productform.gox:62
						if e.ID != 0 {
							__e = __c.InitContainer(); if __e != nil { return }
							{
								__e = __c.Init("span"); if __e != nil { return }
								{
//line productform.gox:64
									__e = __c.Set("class", "font-mono text-xs"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line productform.gox:64
									__e = __c.Any(e.Initial.SKU); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line productform.gox:65
								__e = __c.Any(live.Bind(stockBadge)); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
//line productform.gox:68
				return }),
			}); if __e != nil { return }
					__e = __c.Init("form"); if __e != nil { return }
					{
//line productform.gox:71
						__e = __c.Modify(doors.ASubmit[productForm]{
					Scope: &doors.ScopeBlocking{},
					Indicator: doors.IndicateAttrSubmitter("aria-busy", "true"),
					Batch: true,
					On: func(ctx context.Context, r doors.RequestForm[productForm]) bool {
						id, errs := shop.SaveProduct(ctx, e.ID, r.Data())
						if len(errs) > 0 {
							state.Update(ctx, formState[productForm]{Values: r.Data(), Errs: errs})
							return false
						}
						if e.ID == 0 {
							notify(ctx, "Product added")
							navigate(ctx, ProductPath{ID: id})
						} else {
							notify(ctx, "Product saved")
							navigate(ctx, ProductsPath{})
						}
						r.After(scrollTop)
						return false
					},
				}); if __e != nil { return }
						__e = __c.Set("novalidate", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line productform.gox:93
						__e = __c.Any(state.Bind(func(s formState[productForm]) gox.Elem {
					return e.fields(s, live)
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line productform.gox:98
			if e.ID != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line productform.gox:99
					__e = __c.Set("class", "mt-6"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line productform.gox:99
					__e = __c.Any(productDanger(e.ID, e.Initial.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line productform.gox:102
}

//line productform.gox:104
func (e productEditor) fields(s formState[productForm], live doors.Beam[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line productform.gox:106
		v := s.Values
	stock := new(doors.Door)
	live.ReadAndSub(ctx, func(ctx context.Context, n int) bool {
		stock.Inner(ctx, stockField(strconv.Itoa(n), ""))
		return false
	})
	submit := "Create product"
	if e.ID != 0 {
		submit = "Save changes"
	}

//line productform.gox:117
		__e = __c.Any(formErrors(s.Errs)); if __e != nil { return }
//line productform.gox:118
		__e = __c.Any(card{
		Title: "Product",
		Body: gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line productform.gox:120
				__e = __c.Set("class", "grid grid-cols-1 gap-5 @xl:grid-cols-2"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line productform.gox:121
					__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line productform.gox:122
					__e = __c.Any(field{Label: "Name", Name: "name", Value: v.Name, Required: true, Error: s.Errs["name"], Desc: "product name", Placeholder: "Pour-over kettle"}); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line productform.gox:124
				__e = __c.Any(field{Label: "SKU", Name: "sku", Value: v.SKU, Required: true, Error: s.Errs["sku"], Help: "Unique code on the shelf label.", Desc: "stock keeping unit, unique, like SKU-109", Placeholder: "SKU-109"}); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line productform.gox:126
				__e = __c.Any(field{Label: "Price", Name: "price", Value: v.Price, Prefix: "$", Mode: "decimal", Required: true, Error: s.Errs["price"], Help: "Per unit, in US dollars.", Desc: "price per unit in USD, like 24.90", Placeholder: "0.00"}); if __e != nil { return }
//line productform.gox:127
				__e = (stock).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line productform.gox:127
					__e = __c.Any(stockField(v.Stock, s.Errs["stock"])); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line productform.gox:128
		return }),
	}); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line productform.gox:130
			__e = __c.Set("class", "mt-6 flex flex-col-reverse gap-2 @xl:flex-row @xl:justify-end"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line productform.gox:131
			__e = (neui.Use{Desc: "leave the form without saving"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("a"); if __e != nil { return }
				{
//line productform.gox:131
					__e = __c.Modify(doors.ALink{Model: ProductsPath{}, Batch: true}); if __e != nil { return }
//line productform.gox:131
					__e = __c.Set("class", btnGhost); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Cancel"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line productform.gox:132
			__e = (neui.Use{Desc: "save the product with the values in the form"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line productform.gox:132
					__e = __c.Set("type", "submit"); if __e != nil { return }
//line productform.gox:132
					__e = __c.Set("class", btnPrimary); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line productform.gox:132
					__e = __c.Any(submit); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line productform.gox:134
}

//line productform.gox:136
func stockField(value string, err string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line productform.gox:137
		__e = __c.Any(field{Label: "In stock", Name: "stock", Type: "number", Value: value, Mode: "numeric", Required: true, Error: err, Help: "Units on the shelf now.", Desc: "units in stock, a whole number", Placeholder: "0"}); if __e != nil { return }
	return })
//line productform.gox:138
}

//line productform.gox:140
func productDanger(id int, name string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line productform.gox:142
		n := 0
	for _, o := range shop.Orders(orderQuery{}) {
		if slices.ContainsFunc(o.Lines, func(l line) bool {
			return l.ProductID == id
		}) {
			n++
		}
	}

//line productform.gox:151
		__e = (neui.Space{Name: "delete_product", Desc: "Deleting this product, possible only while it is on no order"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line productform.gox:151
			__e = __c.Any(card{
		Title: "Delete product",
		Desc: "Removes the product from the catalogue for good.",
		Body: gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line productform.gox:155
				if n > 0 {
//line productform.gox:156
					__e = __c.Any(neui.Data{Text: fmt.Sprintf("This product can't be deleted: it is on %d orders. Setting its stock to 0 stops selling it.", n)}); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line productform.gox:157
						__e = __c.Set("class", "text-sm text-stone-600"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line productform.gox:158
						__e = __c.Many(name, " is on ", n, " order", plural(n), ", so it can't be deleted. Set its stock to 0 to stop selling it."); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
//line productform.gox:161
					__e = __c.Any(confirmAction{
					Label: "Delete product",
					Icon: iconTrash,
					Desc: "start deleting this product; the user must then confirm it on the page",
					Question: "Delete " + name + "?",
					Detail: "This can't be undone.",
					Confirm: "Delete product",
					Wait: "Deleting waits for the user: only they can press Delete product in the confirmation.",
					Do: func(ctx context.Context) error {
						if err := shop.DeleteProduct(ctx, id); err != nil {
							return err
						}
						notify(ctx, name + " deleted")
						navigate(ctx, ProductsPath{})
						return nil
					},
				}); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line productform.gox:179
		return }),
	}); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line productform.gox:181
}
