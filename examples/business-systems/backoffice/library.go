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

	"neui.dev"
	"neui.dev/nekit"
)

type customersOutput struct {
	Customers []customerRow `json:"customers" description:"every customer with order count and paid total"`
}

type productsOutput struct {
	Products []product `json:"products"`
}

func reportLibrary(l neui.Library) error {
	if err := nekit.IslandRegisterLayout(l); err != nil {
		return err
	}
	if err := nekit.IslandRegisterMarkdown(l); err != nil {
		return err
	}
	if err := nekit.IslandRegisterTable(l); err != nil {
		return err
	}
	return addShopData(l)
}

func workbenchLibrary(l neui.Library) error {
	if err := nekit.IslandRegisterAll(l); err != nil {
		return err
	}
	for _, t := range operations {
		if err := l.Add(t); err != nil {
			return err
		}
	}
	return addShopData(l)
}

func addShopData(l neui.Library) error {
	if err := l.Tool("orders", "Orders matching a filter, with lines and totals", func(ctx context.Context, in orderQuery) (ordersOutput, error) {
		return ordersOutput{Orders: shop.Orders(in)}, nil
	}); err != nil {
		return err
	}
	if err := l.Tool("customers", "All customers with their order count and paid total", func(ctx context.Context, _ struct{}) (customersOutput, error) {
		return customersOutput{Customers: shop.Customers("", "")}, nil
	}); err != nil {
		return err
	}
	return l.Tool("products", "All products with price and stock", func(ctx context.Context, _ struct{}) (productsOutput, error) {
		return productsOutput{Products: shop.Products("", "")}, nil
	})
}

type ordersOutput struct {
	Orders []order `json:"orders" description:"matching orders"`
}
