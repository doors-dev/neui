package main

import (
	"context"

	"github.com/doors-dev/doors"
)

type ListView int

const (
	ListIndex ListView = iota
	ListNew
)

type ItemView int

const (
	ItemShow ItemView = iota
	ItemEdit
)

type HomePath struct {
	Section int `/:""`
}

type OrdersPath struct {
	View     ListView `/orders:" | new"`
	Status   string   `query:"status,omitempty"`
	From     string   `query:"from,omitempty"`
	To       string   `query:"to,omitempty"`
	Search   string   `query:"q,omitempty"`
	Sort     string   `query:"sort,omitempty"`
	Page     int      `query:"page,omitempty"`
	Customer int      `query:"customer,omitempty"`
}

type OrderPath struct {
	View ItemView `/:"orders/:ID | orders/:ID/edit"`
	ID   int
}

type CustomersPath struct {
	View   ListView `/customers:" | new"`
	Search string   `query:"q,omitempty"`
	Sort   string   `query:"sort,omitempty"`
	Page   int      `query:"page,omitempty"`
}

type CustomerPath struct {
	View ItemView `/:"customers/:ID | customers/:ID/edit"`
	ID   int
	Page int `query:"page,omitempty"`
}

type ProductsPath struct {
	View   ListView `/products:" | new"`
	Search string   `query:"q,omitempty"`
	Sort   string   `query:"sort,omitempty"`
	Page   int      `query:"page,omitempty"`
}

type ProductPath struct {
	Section int `/:"products/:ID"`
	ID      int
}

func navigate(ctx context.Context, model any) {
	loc, err := doors.NewLocation(model)
	if err != nil {
		doors.Logger(ctx).Error("navigate", "err", err)
		return
	}
	doors.Router(ctx).Update(ctx, loc)
}
