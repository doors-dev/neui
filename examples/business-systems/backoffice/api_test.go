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
	"slices"
	"strings"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type treeNode struct {
	Name     string
	Desc     string
	Island   bool
	Tools    int
	Children []treeNode
}

func (n treeNode) paths(prefix string) map[string]treeNode {
	out := map[string]treeNode{}
	for _, c := range n.Children {
		out[prefix+c.Name] = c
		for path, d := range c.paths(prefix + c.Name + ".") {
			out[path] = d
		}
	}
	return out
}

func renderedAt(tier neui.ModelTier) gox.Elem {
	return func(cur gox.Cursor) error {
		return doors.Ctx(context.WithValue(cur.Context(), common.TierKey{}, tier)).Proxy(cur, func(cur gox.Cursor) error {
			return cur.Many(dashboardPage(), workbenchPanel(), apiSpace())
		})
	}
}

func TestAPISpaceServesScriptsAtTierAgent(t *testing.T) {
	price := func() float64 {
		p, _ := shop.Product(8)
		return p.Price
	}
	before := price()
	id := shop.Orders(orderQuery{Status: unpaid})[0].ID
	p := testkit.NewProvider(api.Model{},
		testkit.Call("page__tree", map[string]any{}),
		testkit.Call("js__run", map[string]any{"comment": "raises a price", "code": `page__sync({path: "api.products"})
const pitcher = () => page__tool({path: "api.products", tool: "products"}).output.products.find((p) => p.sku === "SKU-108")
const p = pitcher()
page__tool({path: "api.products", tool: "update_product", arg: {product: p.id, price: p.price + 2}})
return pitcher().price`}),
		testkit.Call("page__sync", map[string]any{"path": "api.orders"}),
		testkit.Call("page__tool", map[string]any{"path": "api.orders", "tool": "set_status", "arg": map[string]any{"order": id, "status": "paid"}}),
		testkit.Say("done"),
	)
	c := stack.Page(t, stack.Host{Provider: p, Page: renderedAt(neui.TierAgent), Options: []neui.Option{neui.OptionTier(neui.TierAgent), neui.OptionModules(neui.ModuleJS)}})
	c.Send("raise the pitcher's price by 2 and mark the order paid")
	c.Wait()
	result := func(i int) api.ContentToolResult {
		r := testkit.Results(p.Request(t, i))[0]
		if r.Error != api.ToolOk {
			t.Fatalf("call %d answered %#v", i, r.Content)
		}
		return r
	}
	var root treeNode
	testkit.Decode(t, result(1), &root)
	spaces := root.paths("")
	for path, want := range map[string]int{"api.orders": len(orderOperations) + 1, "api.customers": len(customerOperations) + 1, "api.products": len(productOperations) + 1} {
		if got, ok := spaces[path]; !ok || got.Tools != want {
			t.Errorf("tree shows %s with %d tools, want %d", path, got.Tools, want)
		}
	}
	var script struct{ Result float64 }
	testkit.Decode(t, result(2), &script)
	if script.Result != before+2 || price() != before+2 {
		t.Errorf("the script read back %v and the shop has %v, want the price raised from %v by 2", script.Result, price(), before)
	}
	result(4)
	if o, _ := shop.Order(id); o.Status != paid {
		t.Errorf("order %d is %s after set_status, want paid", id, o.Status)
	}
}

func TestToolkitFollowsTheTier(t *testing.T) {
	for _, tc := range []struct {
		name string
		tier neui.ModelTier
		want []string
		desc string
	}{
		{"developer", neui.TierDeveloper, []string{"workbench*"}, ""},
		{"agent", neui.TierAgent, []string{"api", "api.customers", "api.orders", "api.products"}, "The shop's data and operations as tools"},
		{"assistant", neui.TierAssistant, []string{"api", "api.customers", "api.orders", "api.products"}, "The shop's data and operations as tools"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testkit.NewProvider(api.Model{}, testkit.Call("page__tree", map[string]any{}), testkit.Say("done"))
			c := stack.Page(t, stack.Host{Provider: p, Page: renderedAt(tc.tier), Options: []neui.Option{neui.OptionTier(neui.TierDeveloper)}})
			c.Send("look")
			c.Wait()
			var root treeNode
			testkit.Decode(t, testkit.Results(p.Request(t, 1))[0], &root)
			var got []string
			for path, n := range root.paths("") {
				if n.Island {
					path += "*"
				}
				if path == "workbench*" || path == "api" || strings.HasPrefix(path, "api.") {
					got = append(got, path)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("rendered at %s, a TierDeveloper session sees %v on the page, want %v", tc.name, got, tc.want)
			}
			if desc := root.paths("")["api"].Desc; desc != tc.desc {
				t.Fatalf("rendered at %s, api is described %q, want %q", tc.name, desc, tc.desc)
			}
		})
	}
}
