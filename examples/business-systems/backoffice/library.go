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

type adjustStockInput struct {
	ID     int `json:"id" required:"true" description:"product id"`
	Change int `json:"change" required:"true" description:"units to add; negative to take away"`
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
	if err := addShopData(l); err != nil {
		return err
	}
	return l.Tool("adjust_stock", "Add units to a product's stock, or take them away with a negative change; checked like the product form. Returns the product after the change", func(ctx context.Context, in adjustStockInput) (product, error) {
		return shop.AdjustStock(ctx, in.ID, in.Change)
	})
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
