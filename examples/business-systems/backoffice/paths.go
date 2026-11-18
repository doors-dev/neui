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
