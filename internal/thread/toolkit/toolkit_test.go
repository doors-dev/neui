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

package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
)

type toolCustomer struct {
	ID string `json:"id"`
}

type toolOrder struct {
	_        struct{}     `description:"the order"`
	Customer toolCustomer `json:"customer"`
}

type toolParts struct{}

func (toolParts) Contents() []api.UserContent { return nil }

func TestToolsAssembleTheTierText(t *testing.T) {
	const customer = `{"properties":{"id":{"type":"string"}},"type":"object"}`
	const order = `{"description":"the order","properties":{"customer":{"$ref":"#/definitions/ToolkitToolCustomer"}},"type":"object"}`
	orderRaw := `{"definitions":{"ToolkitToolCustomer":` + customer + `},` + order[1:]
	const image = `{"properties":{"data":{"format":"base64","type":"string"},"mime":{"type":"string"}},"type":"object"}`
	rows, err := schema.NewRawSchema(json.RawMessage(`{"type":"object","description":"rows found"}`))
	if err != nil {
		t.Fatal(err)
	}
	module := func(name, desc string) common.ModuleFactory {
		m := tools.NewModule(name, desc)
		_, err1 := m.Add("order", "reads an order", func(context.Context, struct{}) (toolOrder, error) { return toolOrder{}, nil })
		_, err2 := m.Add("image", "draws", func(context.Context, struct{}) (api.Image, error) { return api.Image{}, nil })
		_, err3 := m.Add("parts", "splits", func(context.Context, struct{}) (toolParts, error) { return toolParts{}, nil })
		_, err4 := m.AddRaw("rows", "lists rows", schema.ObjectRawSchema, rows, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
		_, err5 := m.AddRaw("blocks", "fetches", schema.ObjectRawSchema, schema.BlocksRawSchema, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
		if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
			t.Fatal(err)
		}
		return func(context.Context) common.Module { return m }
	}
	blocks := string(schema.BlocksRawSchema.View().Raw)
	full := map[string]string{
		"bare__blocks": "fetches\nOutput schema: " + blocks + "\nOn a direct tool call the result arrives as content blocks",
		"bare__image":  "draws\nOutput schema: " + image + "\nOn a direct tool call the result arrives as an image block",
		"bare__order":  "reads an order\nOutput schema: " + orderRaw,
		"bare__parts":  "splits\nOutput schema: " + blocks + "\nOn a direct tool call the result arrives as content blocks",
		"bare__rows":   "lists rows\nOutput schema: {\"type\":\"object\",\"description\":\"rows found\",\"properties\":{}}",
		"crm__blocks":  "fetches\nOutput schema: " + blocks + "\nOn a direct tool call the result arrives as content blocks",
		"crm__image":   "draws\nOutput schema: " + image + "\nOn a direct tool call the result arrives as an image block",
		"crm__order":   "reads an order\nOutput schema: " + order,
		"crm__parts":   "splits\nOutput schema: " + blocks + "\nOn a direct tool call the result arrives as content blocks",
		"crm__rows":    "lists rows\nOutput schema: {\"type\":\"object\",\"description\":\"rows found\",\"properties\":{}}",
	}
	short := map[string]string{
		"bare__blocks": "fetches",
		"bare__image":  "draws",
		"bare__order":  "reads an order\nResult: the order",
		"bare__parts":  "splits",
		"bare__rows":   "lists rows\nResult: rows found",
		"crm__blocks":  "fetches",
		"crm__image":   "draws",
		"crm__order":   "reads an order\nResult: the order",
		"crm__parts":   "splits",
		"crm__rows":    "lists rows\nResult: rows found",
	}
	system := `<module name="crm">` + "\ncrm tools" + outputDefsHead + "\nToolkitToolCustomer " + customer + "\n</module>"
	for _, tier := range []common.ModelTier{common.TierDeveloper, common.TierAgent, common.TierAssistant} {
		k := NewToolkit(context.WithValue(context.Background(), common.TierKey{}, tier), module("crm", "crm tools"), module("bare", ""))
		want, wantSystem := full, system
		if tier == common.TierAssistant {
			want, wantSystem = short, `<module name="crm">`+"\ncrm tools\n</module>"
		}
		var names []string
		for tool := range k.Tools() {
			names = append(names, tool.Name)
			if tool.Desc != want[tool.Name] {
				t.Errorf("tier %d: %s reads %q, want %q", tier, tool.Name, tool.Desc, want[tool.Name])
			}
			input := `{"type":"object","properties":{}}`
			if string(tool.Schema.(json.RawMessage)) != input {
				t.Errorf("tier %d: %s takes %s, want %s", tier, tool.Name, tool.Schema, input)
			}
		}
		if len(names) != len(want) {
			t.Errorf("tier %d: tools %v", tier, names)
		}
		if got := slices.Collect(k.System()); len(got) != 1 || got[0] != wantSystem {
			t.Errorf("tier %d: system %q, want %q", tier, got, wantSystem)
		}
	}
}
