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

package schema

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	jsonvalid "github.com/santhosh-tekuri/jsonschema/v6"
	jsonreflect "github.com/swaggest/jsonschema-go"
)

func TestExternalRefRejected(t *testing.T) {
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"file:///etc/hosts"}}}`)); err == nil {
		t.Fatal("file:// ref compiled")
	}
	path := filepath.Join(t.TempDir(), "ref.json")
	if err := os.WriteFile(path, []byte(`{"type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"file://` + path + `"}}}`)); err == nil {
		t.Fatal("file:// ref to a readable schema compiled")
	}
	if _, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"http://example.com/s.json"}}}`)); err == nil {
		t.Fatal("http:// ref compiled")
	}
}

func TestLocalRefCompiles(t *testing.T) {
	s, err := NewRawSchema(json.RawMessage(`{"type":"object","properties":{"a":{"$ref":"#/$defs/name"}},"required":["a"],"$defs":{"name":{"type":"string"}}}`))
	if err != nil {
		t.Fatalf("local ref: %v", err)
	}
	if err := s.ValidateString(`{"a":"x"}`); err != nil {
		t.Fatalf("valid instance rejected: %v", err)
	}
	if err := s.ValidateString(`{"a":1}`); err == nil {
		t.Fatal("invalid instance accepted")
	}
}

var fieldsSchema = json.RawMessage(`{"type":"object","properties":{"fields":{"type":"array","items":{"$ref":"#/$defs/field"}}},"$defs":{` +
	`"field":{"anyOf":[{"$ref":"#/$defs/text"},{"$ref":"#/$defs/select"},{"$ref":"#/$defs/list"}]},` +
	`"text":{"type":"object","additionalProperties":false,"required":["type","name"],"properties":{"type":{"enum":["text","email"]},"name":{"type":"string"},"required":{"type":"boolean"}}},` +
	`"select":{"type":"object","additionalProperties":false,"required":["type","name","options"],"properties":{"type":{"const":"select"},"name":{"type":"string"},"options":{"type":"array","minItems":1}}},` +
	`"list":{"type":"object","additionalProperties":false,"required":["type","field"],"properties":{"type":{"const":"list"},"field":{"$ref":"#/$defs/field"}}}}}`)

func TestAnyOfReportsMatchingBranch(t *testing.T) {
	s, err := NewRawSchema(fieldsSchema)
	if err != nil {
		t.Fatal(err)
	}
	err = s.ValidateString(`{"fields":[{"type":"text","name":"a"},{"type":"select","name":"b","required":true,"options":[1]}]}`)
	want := "invalid input: argument does not match the schema:\n" +
		"- at '/fields/1': additional properties 'required' not allowed"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
}

func TestAnyOfReportsNestedBranch(t *testing.T) {
	s, err := NewRawSchema(fieldsSchema)
	if err != nil {
		t.Fatal(err)
	}
	err = s.ValidateString(`{"fields":[{"type":"list","field":{"type":"select","name":"b","options":[]}}]}`)
	want := "invalid input: argument does not match the schema:\n" +
		"- at '/fields/0/field/options': minItems: got 0, want 1"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
}

func TestAnyOfKeepsAllBranchesWithoutTypeMatch(t *testing.T) {
	s, err := NewRawSchema(fieldsSchema)
	if err != nil {
		t.Fatal(err)
	}
	err = s.ValidateString(`{"fields":[{"type":"bogus","name":"a"}]}`)
	if err == nil {
		t.Fatal("invalid instance accepted")
	}
	for _, line := range []string{"'anyOf' failed", "value must be one of 'text', 'email'", "value must be 'select'", "value must be 'list'"} {
		if !strings.Contains(err.Error(), line) {
			t.Fatalf("missing %q in %v", line, err)
		}
	}
}

func TestAnyOfKeepsAllBranchesOnWrongTypeBranch(t *testing.T) {
	s, err := NewRawSchema(json.RawMessage(`{"properties":{"filter":{"anyOf":[{"$ref":"#/$defs/F"},{"type":"null"}]}},"$defs":{"F":{"type":"object","properties":{"type":{"const":"eq"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	err = s.ValidateString(`{"filter":{"type":"ne"}}`)
	if err == nil {
		t.Fatal("invalid instance accepted")
	}
	for _, line := range []string{"'anyOf' failed", "at '/filter/type': value must be 'eq'", "at '/filter': got object, want null"} {
		if !strings.Contains(err.Error(), line) {
			t.Fatalf("missing %q in %v", line, err)
		}
	}
}

func TestAnyOfKeepsAllBranchesOnWrongValueBranch(t *testing.T) {
	for _, other := range []string{`{"const":null}`, `{"enum":["all","none"]}`} {
		s, err := NewRawSchema(json.RawMessage(`{"properties":{"filter":{"anyOf":[{"$ref":"#/$defs/F"},` + other + `]}},"$defs":{"F":{"type":"object","properties":{"type":{"const":"eq"}}}}}`))
		if err != nil {
			t.Fatal(err)
		}
		err = s.ValidateString(`{"filter":{"type":"ne"}}`)
		if err == nil {
			t.Fatal("invalid instance accepted")
		}
		for _, line := range []string{"'anyOf' failed", "at '/filter/type': value must be 'eq'"} {
			if !strings.Contains(err.Error(), line) {
				t.Fatalf("%s: missing %q in %v", other, line, err)
			}
		}
	}
}

type splitFlat struct {
	A string
	B int
}

type splitNested struct {
	In splitFlat
	L  []splitFlat
}

type node struct {
	Label    string
	Children []node
}

type pairA struct {
	Name string
	B    *pairB
}

type pairB struct {
	A *pairA
}

func checkSplit[T any](t *testing.T, defs []string, valid []string, invalid []string) Schema[T] {
	t.Helper()
	s, err := NewSchema[T]()
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("raw differs:\n got %s\nwant %s", s.Raw(), before)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(s.Root(), &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["definitions"]; ok {
		t.Fatalf("definitions in root: %s", s.Root())
	}
	if names := slices.Sorted(maps.Keys(s.Defs())); !slices.Equal(names, defs) {
		t.Fatalf("defs %v, want %v", names, defs)
	}
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(before))
	if err != nil {
		t.Fatal(err)
	}
	old, err := newSchema[any](before, doc, jsonvalid.Draft7)
	if err != nil {
		t.Fatal(err)
	}
	for _, inst := range valid {
		if err := s.ValidateString(inst); err != nil {
			t.Fatalf("%s rejected: %v", inst, err)
		}
		if err := old.ValidateString(inst); err != nil {
			t.Fatalf("%s rejected before: %v", inst, err)
		}
	}
	for _, inst := range invalid {
		if s.ValidateString(inst) == nil {
			t.Fatalf("%s accepted", inst)
		}
		if old.ValidateString(inst) == nil {
			t.Fatalf("%s accepted before", inst)
		}
	}
	return s
}

func TestSplitFlat(t *testing.T) {
	checkSplit[splitFlat](t, nil, []string{`{"A":"x","B":1}`, `{}`}, []string{`{"A":1}`, `{"B":"x"}`, `[]`})
}

func TestSplitNested(t *testing.T) {
	checkSplit[splitNested](t, []string{"SchemaSplitFlat"}, []string{`{"In":{"A":"x"},"L":[{"B":2}]}`, `{"L":null}`}, []string{`{"In":{"A":1}}`, `{"L":[{"B":"x"}]}`})
}

func TestSplitSelfReference(t *testing.T) {
	s := checkSplit[node](t, []string{"SchemaNode"}, []string{`{"Label":"a","Children":[{"Label":"b","Children":[{"Label":"c"}]}]}`}, []string{`{"Children":[{"Children":[{"Label":1}]}]}`, `{"Children":[1]}`})
	if !strings.Contains(string(s.Root()), `"$ref":"#/definitions/SchemaNode"`) || strings.Contains(string(s.Root()), `"$ref":"#"`) {
		t.Fatalf("root does not reference itself by name: %s", s.Root())
	}
	if !strings.Contains(string(s.Raw()), `"$ref":"#"`) || strings.Contains(string(s.Raw()), "SchemaNode") {
		t.Fatalf("raw keeps the root's name: %s", s.Raw())
	}
}

func TestSplitMutualReference(t *testing.T) {
	s := checkSplit[pairA](t, []string{"SchemaPairA", "SchemaPairB"}, []string{`{"Name":"a","B":{"A":{"Name":"b","B":{}}}}`}, []string{`{"B":{"A":{"Name":1}}}`, `{"B":{"A":{"B":{"A":[]}}}}`})
	if !strings.Contains(string(s.Defs()["SchemaPairB"]), `"$ref":"#/definitions/SchemaPairA"`) {
		t.Fatalf("pairB does not reference pairA by name: %s", s.Defs()["SchemaPairB"])
	}
}

func TestSplitPrimitive(t *testing.T) {
	s := checkSplit[string](t, nil, []string{`"x"`}, []string{`1`, `{}`})
	if string(s.Root()) != `{"type":"string"}` {
		t.Fatalf("root %s", s.Root())
	}
}

func TestSplitSlice(t *testing.T) {
	checkSplit[[]splitFlat](t, []string{"SchemaSplitFlat"}, []string{`[{"A":"x"}]`, `[]`}, []string{`[{"A":1}]`, `{}`})
}

func TestSplitMap(t *testing.T) {
	checkSplit[map[string]splitFlat](t, []string{"SchemaSplitFlat"}, []string{`{"k":{"B":1}}`}, []string{`{"k":{"B":"x"}}`, `[]`})
}

func TestSplitAny(t *testing.T) {
	s, err := NewSchema[any]()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.Raw(), anySchema) || !bytes.Equal(s.Root(), anySchema) || s.Defs() != nil {
		t.Fatalf("raw %s root %s defs %v", s.Raw(), s.Root(), s.Defs())
	}
}
