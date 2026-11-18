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

package schema_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	jsonreflect "github.com/swaggest/jsonschema-go"
	"neui.dev/internal/schema"
	"neui.dev/nekit"
)

func checkRaw[T any](t *testing.T) {
	t.Helper()
	s, err := schema.NewSchema[T]()
	if err != nil {
		t.Fatal(err)
	}
	var sample T
	reflected, err := (&jsonreflect.Reflector{}).Reflect(sample, jsonreflect.ProcessWithoutTags)
	if err != nil {
		t.Fatal(err)
	}
	before, err := reflected.JSONSchemaBytes()
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(s.Raw(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(before, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, dropCalls(want)) {
		t.Fatalf("%T: raw differs:\n got %s\nwant %s", sample, s.Raw(), before)
	}
	for _, key := range []string{`"x-neui-call"`, `"allOf"`, `"function"`} {
		if strings.Contains(string(s.Raw()), key) {
			t.Fatalf("%T: %s in raw: %s", sample, key, s.Raw())
		}
	}
	docs := []json.RawMessage{s.Root()}
	for _, def := range s.Defs() {
		docs = append(docs, def)
	}
	for _, doc := range docs {
		if strings.Contains(string(doc), `"x-neui-call"`) || strings.Contains(string(doc), `"allOf"`) {
			t.Fatalf("%T: call marker in spec: %s", sample, doc)
		}
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(s.Root(), &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["definitions"]; ok {
		t.Fatalf("%T: definitions in root: %s", sample, s.Root())
	}
}

func TestNeislandParamsRaw(t *testing.T) {
	checkRaw[nekit.IslandFormParams](t)
	checkRaw[nekit.IslandTableParams](t)
	checkRaw[nekit.IslandButtonParams](t)
	checkRaw[nekit.IslandMarkdownParams](t)
}

func dropCalls(v any) any {
	switch v := v.(type) {
	case []any:
		for i, item := range v {
			v[i] = dropCalls(item)
		}
	case map[string]any:
		if v["x-neui-call"] == true {
			call := map[string]any{"type": v["type"]}
			if desc, ok := v["description"]; ok {
				call["description"] = desc
			}
			return call
		}
		for key, item := range v {
			v[key] = dropCalls(item)
		}
	}
	return v
}
