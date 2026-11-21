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

//go:build browser

package main

import (
	"encoding/json"
	"testing"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

func TestMain(m *testing.M) { stack.Main(m) }

type step struct {
	tool  string
	path  string
	desc  string
	value any
}

func syncStep(path string) step {
	return step{tool: "page__sync", path: path}
}

func writeStep(path string, desc string, value any) step {
	return step{tool: "page__write", path: path, desc: desc, value: value}
}

func clickStep(path string, desc string) step {
	return step{tool: "page__click", path: path, desc: desc}
}

func drive(t *testing.T, page gox.Elem, steps ...step) []string {
	t.Helper()
	ids := map[string]string{}
	var results []string
	learn := func(r api.Request) {
		got := testkit.Results(r)
		s := steps[len(results)]
		if len(got) != 1 || got[0].Error != api.ToolOk {
			t.Errorf("%s on %q in %s answered %#v, want it done", s.tool, s.desc, s.path, got)
			results = append(results, "")
			return
		}
		var raw json.RawMessage
		testkit.Decode(t, got[0], &raw)
		t.Logf("%s %s %q: %d bytes %s", s.tool, s.path, s.desc, len(raw), raw)
		results = append(results, string(raw))
		var seen struct {
			Inputs []struct{ ID, Desc string }
			Diff   struct {
				Inputs struct{ Added []struct{ ID, Desc string } }
			}
		}
		if err := json.Unmarshal(raw, &seen); err != nil {
			t.Error(err)
		}
		for _, in := range append(seen.Inputs, seen.Diff.Inputs.Added...) {
			ids[s.path+"\x00"+in.Desc] = in.ID
		}
	}
	call := func(s step) testkit.Turn {
		args := map[string]any{"path": s.path}
		if s.desc != "" {
			args["id"] = ids[s.path+"\x00"+s.desc]
		}
		if s.tool == "page__write" {
			args["value"] = s.value
		}
		return testkit.Call(s.tool, args)
	}
	turns := []testkit.Turn{call(steps[0])}
	for _, s := range steps[1:] {
		turns = append(turns, testkit.Then(func(r api.Request) testkit.Turn {
			learn(r)
			return call(s)
		}))
	}
	turns = append(turns, testkit.Then(func(r api.Request) testkit.Turn {
		learn(r)
		return testkit.Say("done")
	}))
	c := stack.Browser(t, stack.Host{Provider: testkit.NewProvider(api.Model{}, turns...), Page: page})
	c.Send("go")
	c.Wait()
	if len(results) != len(steps) {
		t.Fatalf("%d of %d page calls answered", len(results), len(steps))
	}
	return results
}

func TestOrderFormSavesAfterEditsWithoutSync(t *testing.T) {
	id := shop.nextID.order
	save := clickStep("order_form", "save the order with the values in the form")
	results := drive(t, orderCreatePage(0),
		syncStep("order_form"),
		writeStep("order_form", "customer who placed the order", "3"),
		writeStep("order_form", "product of item 1", "4"),
		writeStep("order_form", "quantity of item 1", "2"),
		clickStep("order_form", "add another item row to the order"),
		writeStep("order_form", "product of item 2", "7"),
		writeStep("order_form", "quantity of item 2", "99"),
		writeStep("order_form", "payment status: paid or unpaid", "paid"),
		save,
		writeStep("order_form", "quantity of item 2", "1"),
		save,
	)
	for i, r := range results[1:] {
		if len(r) > 400 {
			t.Errorf("edit %d answered %d bytes, want a short diff: %s", i+1, len(r), r)
		}
	}
	if want := `{"handled":true,"diff":{"data":[{"text":"order total for the items picked so far, USD","object":49.8}]},"changes":[{"path":"order_form.summary","changed":["data"]}]}`; results[3] != want {
		t.Errorf("the quantity write answered %s, want the total in the diff and the picked items as a change mark: %s", results[3], want)
	}
	o, ok := shop.Order(id)
	if !ok || o.CustomerID != 3 || o.Status != paid || len(o.Lines) != 2 || o.Lines[0].Qty != 2 || o.Lines[1].Qty != 1 {
		t.Fatalf("order %d saved as %+v, want Grace Hopper's paid order of 2 travel mugs and 1 cold brew jar", id, o)
	}
}

func TestFilterKeepsTheListPageRead(t *testing.T) {
	drive(t, customersPage(doors.NewSource(CustomersPath{})),
		syncStep("customers"),
		syncStep("customers.search"),
		writeStep("customers.search", "search customers by name, email, company or phone", "Grace"),
		clickStep("customers", "open the form to add a new customer"),
	)
	drive(t, ordersPage(doors.NewSource(OrdersPath{})),
		syncStep("orders"),
		syncStep("orders.filters"),
		writeStep("orders.filters", "show only orders with this status", "cancelled"),
		clickStep("orders", "open the form to create a new order"),
	)
}
