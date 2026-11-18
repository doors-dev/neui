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
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"

	"neui.dev"
)

type created struct {
	ID int `json:"id" description:"id of the new record"`
}

type itemInput struct {
	Product int `json:"product" required:"true" description:"product id"`
	Qty     int `json:"qty" required:"true" description:"units, at least 1"`
}

type createOrderInput struct {
	Customer int         `json:"customer" required:"true" description:"customer id"`
	Items    []itemInput `json:"items" required:"true" description:"products and quantities, each product once"`
	Status   string      `json:"status,omitempty" description:"paid or unpaid; empty for unpaid"`
	Address  string      `json:"address,omitempty" description:"shipping address; empty for the customer's address on file"`
}

type orderInput struct {
	Order int `json:"order" required:"true" description:"order id"`
}

type orderItemInput struct {
	Order   int `json:"order" required:"true" description:"order id"`
	Product int `json:"product" required:"true" description:"product id"`
	Qty     int `json:"qty" required:"true" description:"units"`
}

type removeItemInput struct {
	Order   int `json:"order" required:"true" description:"order id"`
	Product int `json:"product" required:"true" description:"product id of the item to take off"`
}

type addressInput struct {
	Order   int    `json:"order" required:"true" description:"order id"`
	Address string `json:"address" description:"new shipping address; empty for the customer's address on file"`
}

type statusInput struct {
	Order  int    `json:"order" required:"true" description:"order id"`
	Status string `json:"status" required:"true" description:"paid or unpaid"`
}

type extendDueInput struct {
	Order int `json:"order" required:"true" description:"order id"`
	Days  int `json:"days" required:"true" description:"days to add to the due date, 1 to 60"`
}

type dueOutput struct {
	Due string `json:"due" description:"new due date, YYYY-MM-DD"`
}

type customerInput struct {
	Name    string `json:"name" required:"true"`
	Email   string `json:"email" required:"true"`
	Phone   string `json:"phone,omitempty"`
	Company string `json:"company,omitempty"`
	Address string `json:"address" required:"true" description:"default shipping address"`
	Notes   string `json:"notes,omitempty" description:"internal notes"`
}

type customerPatch struct {
	Customer int     `json:"customer" required:"true" description:"customer id"`
	Name     *string `json:"name,omitempty" description:"omit to keep"`
	Email    *string `json:"email,omitempty" description:"omit to keep"`
	Phone    *string `json:"phone,omitempty" description:"omit to keep; empty to clear"`
	Company  *string `json:"company,omitempty" description:"omit to keep; empty to clear"`
	Address  *string `json:"address,omitempty" description:"default shipping address; omit to keep"`
	Notes    *string `json:"notes,omitempty" description:"internal notes; omit to keep, empty to clear"`
}

type productInput struct {
	SKU   string  `json:"sku" required:"true" description:"unique code without spaces, like SKU-109"`
	Name  string  `json:"name" required:"true"`
	Price float64 `json:"price" required:"true" description:"USD per unit"`
	Stock int     `json:"stock" description:"units in stock"`
}

type productPatch struct {
	Product int      `json:"product" required:"true" description:"product id"`
	SKU     *string  `json:"sku,omitempty" description:"omit to keep"`
	Name    *string  `json:"name,omitempty" description:"omit to keep"`
	Price   *float64 `json:"price,omitempty" description:"USD per unit; omit to keep"`
	Stock   *int     `json:"stock,omitempty" description:"units in stock; omit to keep, or use adjust_stock to add or take away"`
}

type adjustStockInput struct {
	Product int `json:"product" required:"true" description:"product id"`
	Change  int `json:"change" required:"true" description:"units to add; negative to take away"`
}

type stockOutput struct {
	Stock int `json:"stock" description:"units in stock after the change"`
}

func patch(to *string, from *string) {
	if from != nil {
		*to = *from
	}
}

func createOrder(ctx context.Context, in createOrderInput) (created, error) {
	f := orderForm{Customer: strconv.Itoa(in.Customer), Address: in.Address, Status: cmp.Or(in.Status, string(unpaid))}
	for _, it := range in.Items {
		f.Lines = append(f.Lines, lineForm{Product: strconv.Itoa(it.Product), Qty: strconv.Itoa(it.Qty)})
	}
	id, errs := shop.SaveOrder(ctx, 0, f)
	if err := errs.items(f); err != nil {
		return created{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d created", id))
	return created{ID: id}, nil
}

func setItemQuantity(ctx context.Context, in orderItemInput) (struct{}, error) {
	product := strconv.Itoa(in.Product)
	err := shop.ChangeOrder(ctx, in.Order, func(f *orderForm) error {
		i := slices.IndexFunc(f.Lines, func(l lineForm) bool {
			return l.Product == product
		})
		switch {
		case i < 0:
			return fmt.Errorf("product %d is not on order #%d; add it with add_item", in.Product, in.Order)
		case in.Qty < 0:
			return fmt.Errorf("qty: 0 or more; 0 takes the item off")
		case in.Qty == 0:
			f.Lines = slices.Delete(f.Lines, i, i+1)
		default:
			f.Lines[i].Qty = strconv.Itoa(in.Qty)
		}
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}
	p, _ := shop.Product(in.Product)
	if in.Qty == 0 {
		notify(ctx, fmt.Sprintf("Order #%d: %s removed", in.Order, p.Name))
	} else {
		notify(ctx, fmt.Sprintf("Order #%d: %d × %s", in.Order, in.Qty, p.Name))
	}
	return struct{}{}, nil
}

func addItem(ctx context.Context, in orderItemInput) (struct{}, error) {
	product := strconv.Itoa(in.Product)
	err := shop.ChangeOrder(ctx, in.Order, func(f *orderForm) error {
		if in.Qty < 1 {
			return fmt.Errorf("qty: at least 1")
		}
		i := slices.IndexFunc(f.Lines, func(l lineForm) bool {
			return l.Product == product
		})
		if i < 0 {
			f.Lines = append(f.Lines, lineForm{Product: product, Qty: strconv.Itoa(in.Qty)})
			return nil
		}
		qty, _ := strconv.Atoi(f.Lines[i].Qty)
		f.Lines[i].Qty = strconv.Itoa(qty + in.Qty)
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}
	p, _ := shop.Product(in.Product)
	notify(ctx, fmt.Sprintf("Order #%d: %s added", in.Order, p.Name))
	return struct{}{}, nil
}

func removeItem(ctx context.Context, in removeItemInput) (struct{}, error) {
	return setItemQuantity(ctx, orderItemInput{Order: in.Order, Product: in.Product})
}

func updateShippingAddress(ctx context.Context, in addressInput) (struct{}, error) {
	err := shop.ChangeOrder(ctx, in.Order, func(f *orderForm) error {
		f.Address = in.Address
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d: shipping address saved", in.Order))
	return struct{}{}, nil
}

func setStatus(ctx context.Context, in statusInput) (struct{}, error) {
	err := shop.ChangeOrder(ctx, in.Order, func(f *orderForm) error {
		f.Status = in.Status
		return nil
	})
	if err != nil {
		return struct{}{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d marked %s", in.Order, in.Status))
	return struct{}{}, nil
}

func refundOrder(ctx context.Context, in orderInput) (struct{}, error) {
	if err := shop.Refund(ctx, in.Order); err != nil {
		return struct{}{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d refunded", in.Order))
	return struct{}{}, nil
}

func cancelOrder(ctx context.Context, in orderInput) (struct{}, error) {
	if err := shop.Cancel(ctx, in.Order); err != nil {
		return struct{}{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d cancelled", in.Order))
	return struct{}{}, nil
}

func remindCustomer(ctx context.Context, in orderInput) (struct{}, error) {
	name, err := shop.Remind(ctx, in.Order)
	if err != nil {
		return struct{}{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d: payment reminder sent to %s", in.Order, name))
	return struct{}{}, nil
}

func extendDue(ctx context.Context, in extendDueInput) (dueOutput, error) {
	due, err := shop.ExtendDue(ctx, in.Order, in.Days)
	if err != nil {
		return dueOutput{}, err
	}
	notify(ctx, fmt.Sprintf("Order #%d: due %s", in.Order, shortDate(due)))
	return dueOutput{Due: due}, nil
}

func createCustomer(ctx context.Context, in customerInput) (created, error) {
	id, errs := shop.SaveCustomer(ctx, 0, customerForm(in))
	if err := errs.err(); err != nil {
		return created{}, err
	}
	notify(ctx, in.Name+" added")
	return created{ID: id}, nil
}

func updateCustomer(ctx context.Context, in customerPatch) (struct{}, error) {
	c, ok := shop.Customer(in.Customer)
	if !ok {
		return struct{}{}, fmt.Errorf("no customer %d", in.Customer)
	}
	f := c.customer.form()
	patch(&f.Name, in.Name)
	patch(&f.Email, in.Email)
	patch(&f.Phone, in.Phone)
	patch(&f.Company, in.Company)
	patch(&f.Address, in.Address)
	patch(&f.Notes, in.Notes)
	if _, errs := shop.SaveCustomer(ctx, in.Customer, f); len(errs) > 0 {
		return struct{}{}, errs.err()
	}
	notify(ctx, f.Name+" saved")
	return struct{}{}, nil
}

func createProduct(ctx context.Context, in productInput) (created, error) {
	f := productForm{SKU: in.SKU, Name: in.Name, Price: strconv.FormatFloat(in.Price, 'f', -1, 64), Stock: strconv.Itoa(in.Stock)}
	id, errs := shop.SaveProduct(ctx, 0, f)
	if err := errs.err(); err != nil {
		return created{}, err
	}
	notify(ctx, in.Name+" added")
	return created{ID: id}, nil
}

func updateProduct(ctx context.Context, in productPatch) (struct{}, error) {
	p, ok := shop.Product(in.Product)
	if !ok {
		return struct{}{}, fmt.Errorf("no product %d", in.Product)
	}
	f := p.form()
	patch(&f.SKU, in.SKU)
	patch(&f.Name, in.Name)
	if in.Price != nil {
		f.Price = strconv.FormatFloat(*in.Price, 'f', -1, 64)
	}
	if in.Stock != nil {
		f.Stock = strconv.Itoa(*in.Stock)
	}
	if _, errs := shop.SaveProduct(ctx, in.Product, f); len(errs) > 0 {
		return struct{}{}, errs.err()
	}
	notify(ctx, f.Name+" saved")
	return struct{}{}, nil
}

func adjustStock(ctx context.Context, in adjustStockInput) (stockOutput, error) {
	p, err := shop.AdjustStock(ctx, in.Product, in.Change)
	if err != nil {
		return stockOutput{}, err
	}
	notify(ctx, fmt.Sprintf("%s: %d in stock", p.Name, p.Stock))
	return stockOutput{Stock: p.Stock}, nil
}

var operations = []neui.Tooler{
	neui.Tool[createOrderInput, created]{
		Name:    "create_order",
		Desc:    "Create an order, checked like the New order form; stock is taken when it is created. Returns its id; open_order shows it to the user",
		Handler: createOrder,
	},
	neui.Tool[orderItemInput, struct{}]{
		Name:    "set_item_quantity",
		Desc:    "Set the quantity of a product already on an order; 0 takes it off, and an order keeps at least one item. Checked against stock like the edit form",
		Handler: setItemQuantity,
	},
	neui.Tool[orderItemInput, struct{}]{
		Name:    "add_item",
		Desc:    "Add a product to an order; when it is already on the order, its quantity grows by qty. Checked against stock like the edit form",
		Handler: addItem,
	},
	neui.Tool[removeItemInput, struct{}]{
		Name:    "remove_item",
		Desc:    "Take a product off an order and put its units back in stock; an order keeps at least one item",
		Handler: removeItem,
	},
	neui.Tool[addressInput, struct{}]{
		Name:    "update_shipping_address",
		Desc:    "Change where an order ships",
		Handler: updateShippingAddress,
	},
	neui.Tool[statusInput, struct{}]{
		Name:    "set_status",
		Desc:    "Mark an order paid or unpaid; refund_order and cancel_order close it",
		Handler: setStatus,
	},
	neui.Tool[orderInput, struct{}]{
		Name:    "refund_order",
		Desc:    "Refund a paid order: the money goes back to the customer and the order closes. Can't be undone",
		Handler: refundOrder,
	},
	neui.Tool[orderInput, struct{}]{
		Name:    "cancel_order",
		Desc:    "Cancel an unpaid order: its items go back into stock and the order closes. Can't be undone",
		Handler: cancelOrder,
	},
	neui.Tool[orderInput, struct{}]{
		Name:    "remind_customer",
		Desc:    "Send the customer of an unpaid order a payment reminder; the order page shows when it was sent",
		Handler: remindCustomer,
	},
	neui.Tool[extendDueInput, dueOutput]{
		Name:    "extend_due",
		Desc:    "Move the due date of an unpaid order later by 1 to 60 days",
		Handler: extendDue,
	},
	neui.Tool[customerInput, created]{
		Name:    "create_customer",
		Desc:    "Add a customer, checked like the New customer form. Returns their id; open_customer shows them to the user",
		Handler: createCustomer,
	},
	neui.Tool[customerPatch, struct{}]{
		Name:    "update_customer",
		Desc:    "Change a customer's contact details, checked like the edit form; omitted fields stay as they are",
		Handler: updateCustomer,
	},
	neui.Tool[productInput, created]{
		Name:    "create_product",
		Desc:    "Add a product, checked like the New product form. Returns its id",
		Handler: createProduct,
	},
	neui.Tool[productPatch, struct{}]{
		Name:    "update_product",
		Desc:    "Change a product's SKU, name, price or stock, checked like the product form; omitted fields stay as they are",
		Handler: updateProduct,
	},
	neui.Tool[adjustStockInput, stockOutput]{
		Name:    "adjust_stock",
		Desc:    "Add units to a product's stock, or take them away with a negative change; checked like the product form",
		Handler: adjustStock,
	},
}
