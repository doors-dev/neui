// Managed by GoX v0.4.0

//line customerform.gox:1
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

//line customerform.gox:26
func customerFormPage(id int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customerform.gox:28
		c, found := shop.Customer(id)

//line customerform.gox:30
		if id != 0 && !found {
//line customerform.gox:31
			__e = __c.Any(notFound(fmt.Sprintf("Customer %d", id), crumb{Label: "Back to customers", Model: CustomersPath{}})); if __e != nil { return }
		} else  {
//line customerform.gox:33
			__e = __c.Any(customerEditor{ID: id, Initial: c.customer.form()}); if __e != nil { return }
		}
	return })
//line customerform.gox:35
}

type customerEditor struct {
	ID int
	Initial customerForm
}

//line customerform.gox:42
func (e customerEditor) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customerform.gox:44
		state := doors.NewSourceEqual(formState[customerForm]{Values: e.Initial}, always)
	title := "New customer"
	crumbs := []crumb{{Label: "Customers", Model: CustomersPath{}}, {Label: "New", Model: CustomersPath{View: ListNew}}}
	desc := "Form adding a customer: name, email, phone, company, shipping address and notes; press Create customer to save"
	if e.ID != 0 {
		title = "Edit " + e.Initial.Name
		crumbs = []crumb{{Label: "Customers", Model: CustomersPath{}}, {Label: e.Initial.Name, Model: CustomerPath{ID: e.ID}}, {Label: "Edit", Model: CustomerPath{View: ItemEdit, ID: e.ID}}}
		desc = fmt.Sprintf("Form editing customer %s; press Save changes to keep them", e.Initial.Name)
	}

		__e = __c.Init("div"); if __e != nil { return }
		{
//line customerform.gox:54
			__e = __c.Set("class", "mx-auto max-w-3xl"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:55
			__e = (neui.Space{Name: "customer_form", Desc: desc}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:56
					__e = __c.Any(pageHeader{Crumbs: crumbs, Title: title}); if __e != nil { return }
					__e = __c.Init("form"); if __e != nil { return }
					{
//line customerform.gox:58
						__e = __c.Modify(doors.ASubmit[customerForm]{
					Scope: &doors.ScopeBlocking{},
					Indicator: doors.IndicateAttrSubmitter("aria-busy", "true"),
					Batch: true,
					On: func(ctx context.Context, r doors.RequestForm[customerForm]) bool {
						id, errs := shop.SaveCustomer(ctx, e.ID, r.Data())
						if len(errs) > 0 {
							state.Update(ctx, formState[customerForm]{Values: r.Data(), Errs: errs})
							return false
						}
						if e.ID == 0 {
							notify(ctx, "Customer added")
						} else {
							notify(ctx, "Customer saved")
						}
						navigate(ctx, CustomerPath{ID: id})
						r.After(scrollTop)
						return false
					},
				}); if __e != nil { return }
						__e = __c.Set("novalidate", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:79
						__e = __c.Any(state.Bind(func(s formState[customerForm]) gox.Elem {
					return e.fields(s)
				})); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
//line customerform.gox:84
			if e.ID != 0 {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line customerform.gox:85
					__e = __c.Set("class", "mt-6"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:85
					__e = __c.Any(customerDanger(e.ID, e.Initial.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line customerform.gox:88
}

//line customerform.gox:90
func (e customerEditor) fields(s formState[customerForm]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customerform.gox:92
		v := s.Values
	submit := "Create customer"
	cancel := any(CustomersPath{})
	if e.ID != 0 {
		submit = "Save changes"
		cancel = CustomerPath{ID: e.ID}
	}

//line customerform.gox:100
		__e = __c.Any(formErrors(s.Errs)); if __e != nil { return }
		__e = __c.Init("div"); if __e != nil { return }
		{
//line customerform.gox:101
			__e = __c.Set("class", "space-y-6"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:102
			__e = __c.Any(card{
			Title: "Contact",
			Desc: "How to reach them about their orders.",
			Body: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line customerform.gox:105
					__e = __c.Set("class", "grid grid-cols-1 gap-5 @xl:grid-cols-2"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line customerform.gox:106
						__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:107
						__e = __c.Any(field{Label: "Full name", Name: "name", Value: v.Name, Required: true, Error: s.Errs["name"], Desc: "customer's full name", Placeholder: "Ada Lovelace"}); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line customerform.gox:109
					__e = __c.Any(field{Label: "Email", Name: "email", Type: "email", Value: v.Email, Required: true, Error: s.Errs["email"], Desc: "customer's email address", Placeholder: "name@example.com"}); if __e != nil { return }
//line customerform.gox:110
					__e = __c.Any(field{Label: "Phone", Name: "phone", Type: "tel", Value: v.Phone, Error: s.Errs["phone"], Help: "Optional.", Desc: "customer's phone number, optional"}); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line customerform.gox:111
						__e = __c.Set("class", "@xl:col-span-2"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:112
						__e = __c.Any(field{Label: "Company", Name: "company", Value: v.Company, Error: s.Errs["company"], Help: "Optional. Shown on invoices.", Desc: "customer's company, optional"}); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customerform.gox:114
			return }),
		}); if __e != nil { return }
//line customerform.gox:116
			__e = __c.Any(card{
			Title: "Shipping",
			Body: gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line customerform.gox:118
					__e = __c.Set("class", "space-y-5"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:119
					__e = __c.Any(textArea{Label: "Default shipping address", Name: "address", Value: v.Address, Rows: 3, Required: true, Error: s.Errs["address"], Help: "New orders ship here unless you enter another address.", Desc: "customer's default shipping address", Placeholder: "Street, city, postcode"}); if __e != nil { return }
//line customerform.gox:120
					__e = __c.Any(textArea{Label: "Notes", Name: "notes", Value: v.Notes, Rows: 3, Help: "Only your team sees these.", Desc: "internal notes about the customer, optional"}); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line customerform.gox:121
			return }),
		}); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line customerform.gox:123
				__e = __c.Set("class", "flex flex-col-reverse gap-2 @xl:flex-row @xl:justify-end"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:124
				__e = (neui.Use{Desc: "leave the form without saving"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("a"); if __e != nil { return }
					{
//line customerform.gox:124
						__e = __c.Modify(doors.ALink{Model: cancel, Batch: true}); if __e != nil { return }
//line customerform.gox:124
						__e = __c.Set("class", btnGhost); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Cancel"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
//line customerform.gox:125
				__e = (neui.Use{Desc: "save the customer with the values in the form"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line customerform.gox:125
						__e = __c.Set("type", "submit"); if __e != nil { return }
//line customerform.gox:125
						__e = __c.Set("class", btnPrimary); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:125
						__e = __c.Any(submit); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line customerform.gox:128
}

//line customerform.gox:130
func customerDanger(id int, name string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line customerform.gox:132
		n := len(shop.Orders(orderQuery{Customer: id}))

//line customerform.gox:134
		__e = (neui.Space{Name: "delete_customer", Desc: "Deleting this customer, possible only while they have no orders"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line customerform.gox:134
			__e = __c.Any(card{
		Title: "Delete customer",
		Desc: "Removes the customer for good.",
		Body: gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line customerform.gox:138
				if n > 0 {
//line customerform.gox:139
					__e = __c.Any(neui.Data{Text: fmt.Sprintf("This customer can't be deleted: they have %d orders, which are kept for the books.", n)}); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line customerform.gox:140
						__e = __c.Set("class", "text-sm text-stone-600"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line customerform.gox:141
						__e = __c.Many(name, " has ", n, " order", plural(n), ", so they can't be deleted: orders are kept for the books."); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
//line customerform.gox:144
					__e = __c.Any(confirmAction{
					Label: "Delete customer",
					Icon: iconTrash,
					Desc: "start deleting this customer; the user must then confirm it on the page",
					Question: "Delete " + name + "?",
					Detail: "Their contact details and notes are removed. This can't be undone.",
					Confirm: "Delete customer",
					Wait: "Deleting waits for the user: only they can press Delete customer in the confirmation.",
					Do: func(ctx context.Context) error {
						if err := shop.DeleteCustomer(ctx, id); err != nil {
							return err
						}
						notify(ctx, name + " deleted")
						navigate(ctx, CustomersPath{})
						return nil
					},
				}); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line customerform.gox:162
		return }),
	}); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line customerform.gox:164
}
