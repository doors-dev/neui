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

package thread_test

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/internal/tools"
)

type simpleTree struct {
	Name string       `json:"name"`
	Kids []simpleTree `json:"kids"`
}

func TestSimpleSchemasReachTheProvider(t *testing.T) {
	m := tools.NewModule("probe", "")
	if _, err := m.AddRaw("run", "runs the probe", schema.ObjectRawSchema, schema.AnyRawSchema, func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add("tree", "walks", func(context.Context, simpleTree) (struct{}, error) { return struct{}{}, nil }); err != nil {
		t.Fatal(err)
	}
	module := common.OptionModule(func(context.Context) common.Module { return m })
	for on, want := range map[bool]map[string]string{
		false: {
			"probe__run":  `{"type":"object"}`,
			"probe__tree": `{"properties":{"kids":{"items":{"$ref":"#"},"type":["array","null"]},"name":{"type":"string"}},"type":"object"}`,
		},
		true: {
			"probe__run": `{"type":"object","properties":{}}`,
		},
	} {
		p := testkit.NewProvider(api.Model{}, testkit.Say("hi"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{module, common.OptionSimpleSchemas(on)}})
		c.Send("hello")
		c.Wait()
		got := map[string]string{}
		for _, tool := range p.Request(t, 0).Tools {
			sent := string(tool.Schema.(json.RawMessage))
			if strings.HasPrefix(tool.Name, "probe__") {
				got[tool.Name] = sent
			}
			if on && strings.Contains(sent, `"$ref"`) {
				t.Errorf("%s went out as %s with simple schemas on", tool.Name, sent)
			}
		}
		if !maps.Equal(got, want) {
			t.Errorf("simple schemas %v: sent %v, want %v", on, got, want)
		}
	}
}
