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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
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
	partsSchema, err := schema.NewSchema[toolParts]()
	if err != nil {
		t.Fatal(err)
	}
	parts := string(partsSchema.View().Raw)
	const native = "\nOn a direct tool call the result arrives as native content blocks."
	full := map[string]string{
		"bare__blocks": "fetches\nOutput schema, for scripts: " + blocks + native,
		"bare__image":  "draws\nOutput schema, for scripts: " + image + native,
		"bare__order":  "reads an order\nOutput schema: " + orderRaw,
		"bare__parts":  "splits\nOutput schema, for scripts: " + parts + native,
		"bare__rows":   "lists rows\nOutput schema: {\"type\":\"object\",\"description\":\"rows found\"}",
		"crm__blocks":  "fetches\nOutput schema, for scripts: " + blocks + native,
		"crm__image":   "draws\nOutput schema, for scripts: " + image + native,
		"crm__order":   "reads an order\nOutput schema: " + order,
		"crm__parts":   "splits\nOutput schema, for scripts: " + parts + native,
		"crm__rows":    "lists rows\nOutput schema: {\"type\":\"object\",\"description\":\"rows found\"}",
	}
	short := map[string]string{
		"bare__blocks": "fetches\nResult: native content blocks.",
		"bare__image":  "draws\nResult: native content blocks.",
		"bare__order":  "reads an order\nResult: the order",
		"bare__parts":  "splits\nResult: native content blocks.",
		"bare__rows":   "lists rows\nResult: rows found",
		"crm__blocks":  "fetches\nResult: native content blocks.",
		"crm__image":   "draws\nResult: native content blocks.",
		"crm__order":   "reads an order\nResult: the order",
		"crm__parts":   "splits\nResult: native content blocks.",
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
			input := `{"type":"object"}`
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

type toolTree struct {
	Name string     `json:"name"`
	Kids []toolTree `json:"kids"`
}

type toolQuery struct {
	Customer toolCustomer `json:"customer"`
}

func TestToolsSendSimpleSchemas(t *testing.T) {
	m := tools.NewModule("simple", "")
	_, err1 := m.Add("find", "finds", func(context.Context, toolQuery) (struct{}, error) { return struct{}{}, nil })
	_, err2 := m.Add("tree", "walks", func(context.Context, toolTree) (struct{}, error) { return struct{}{}, nil })
	_, err3 := m.AddRaw("rows", "lists rows", schema.ObjectRawSchema, schema.AnyRawSchema, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	if err := errors.Join(err1, err2, err3); err != nil {
		t.Fatal(err)
	}
	const customer = `{"properties":{"id":{"type":"string"}},"type":"object"}`
	raw := map[string]string{
		"simple__find": `{"definitions":{"ToolkitToolCustomer":` + customer + `},"properties":{"customer":{"$ref":"#/definitions/ToolkitToolCustomer"}},"type":"object"}`,
		"simple__rows": `{"type":"object"}`,
		"simple__tree": `{"properties":{"kids":{"items":{"$ref":"#"},"type":["array","null"]},"name":{"type":"string"}},"type":"object"}`,
	}
	simple := map[string]string{
		"simple__find": `{"properties":{"customer":` + customer + `},"type":"object"}`,
		"simple__rows": `{"type":"object","properties":{}}`,
	}
	unsimplified.Delete("simple__tree")
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, on := range []bool{false, true, true} {
		k := NewToolkit(context.Background(), func(context.Context) common.Module { return m })
		k.SimpleSchemas = on
		k = k.Wrap(context.Background())
		want := raw
		if on {
			want = simple
		}
		got := map[string]string{}
		for tool := range k.Tools() {
			got[tool.Name] = string(tool.Schema.(json.RawMessage))
		}
		if !maps.Equal(got, want) {
			t.Errorf("simple schemas %v: sent %v, want %v", on, got, want)
		}
	}
	if n := strings.Count(logs.String(), "tool skipped, its input schema cannot be simplified"); n != 1 || !strings.Contains(logs.String(), "tool=simple__tree") {
		t.Errorf("logged %d skips: %s", n, logs.String())
	}
}
