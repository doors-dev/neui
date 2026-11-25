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
	"errors"
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
	if err := json.Unmarshal(s.View().Raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(before, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("raw differs:\n got %s\nwant %s", s.View().Raw, before)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(s.View().Root, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["definitions"]; ok {
		t.Fatalf("definitions in root: %s", s.View().Root)
	}
	if names := slices.Sorted(maps.Keys(s.View().Defs)); !slices.Equal(names, defs) {
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
	if !strings.Contains(string(s.View().Root), `"$ref":"#/definitions/SchemaNode"`) || strings.Contains(string(s.View().Root), `"$ref":"#"`) {
		t.Fatalf("root does not reference itself by name: %s", s.View().Root)
	}
	if !strings.Contains(string(s.View().Raw), `"$ref":"#"`) || strings.Contains(string(s.View().Raw), "SchemaNode") {
		t.Fatalf("raw keeps the root's name: %s", s.View().Raw)
	}
}

func TestSplitMutualReference(t *testing.T) {
	s := checkSplit[pairA](t, []string{"SchemaPairA", "SchemaPairB"}, []string{`{"Name":"a","B":{"A":{"Name":"b","B":{}}}}`}, []string{`{"B":{"A":{"Name":1}}}`, `{"B":{"A":{"B":{"A":[]}}}}`})
	if !strings.Contains(string(s.View().Defs["SchemaPairB"]), `"$ref":"#/definitions/SchemaPairA"`) {
		t.Fatalf("pairB does not reference pairA by name: %s", s.View().Defs["SchemaPairB"])
	}
}

func TestSplitPrimitive(t *testing.T) {
	s := checkSplit[string](t, nil, []string{`"x"`}, []string{`1`, `{}`})
	if string(s.View().Root) != `{"type":"string"}` {
		t.Fatalf("root %s", s.View().Root)
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
	if !bytes.Equal(s.View().Raw, anySchema) || !bytes.Equal(s.View().Root, anySchema) || s.View().Defs != nil {
		t.Fatalf("raw %s root %s defs %v", s.View().Raw, s.View().Root, s.View().Defs)
	}
}

type descTagged struct {
	Name string   `json:"name"`
	_    struct{} `description:"rows found"`
}

type descPlain struct {
	Name string `json:"name"`
}

type descInner struct {
	ID string   `json:"id"`
	_  struct{} `description:"inner"`
}

type descOuter struct {
	descInner
	Name string   `json:"name"`
	_    struct{} `description:"outer"`
}

type descText string

func (descText) Description() string {
	return "a greeting"
}

type briefRow struct {
	ID string   `json:"id" description:"the row's key"`
	_  struct{} `description:"one row"`
}

type briefRows struct {
	Rows      []briefRow `json:"rows" description:"in sheet order"`
	First     briefRow   `json:"first"`
	Last      briefRow   `json:"last" description:"the bottom row"`
	Truncated bool       `json:"truncated,omitempty" description:"more rows remain"`
	_         struct{}   `description:"rows found"`
}

type briefFields struct {
	Count int      `json:"count" description:"how many matched"`
	Paths []string `json:"paths"`
}

func TestViewBrief(t *testing.T) {
	tagged, err1 := NewSchema[descTagged]()
	plain, err2 := NewSchema[descPlain]()
	outer, err3 := NewSchema[descOuter]()
	text, err4 := NewSchema[descText]()
	raw, err5 := NewRawSchema(json.RawMessage(`{"type":"object","description":"orders","properties":{"id":{"type":"string"}}}`))
	rawText, err6 := NewRawSchema(json.RawMessage(`{"type":"string","description":"an order id"}`))
	nullable, err7 := NewRawSchema(json.RawMessage(`{"type":["object","null"],"description":"maybe orders"}`))
	accept, err8 := NewRawSchema(true)
	reject, err9 := NewRawSchema(false)
	anything, err10 := NewSchema[any]()
	rows, err11 := NewSchema[briefRows]()
	fields, err12 := NewSchema[briefFields]()
	list, err13 := NewSchema[[]briefRow]()
	pointers, err14 := NewSchema[[]*briefRow]()
	strs, err15 := NewSchema[[]string]()
	ordered, err16 := NewRawSchema(json.RawMessage(`{"type":"object","description":"orders","properties":{"total":{"description":"in cents"},"id":{"type":"string"},"due":{"description":"a date"}}}`))
	rawList, err17 := NewRawSchema(json.RawMessage(`{"type":"array","description":"orders","items":{"$ref":"#/$defs/order"},"$defs":{"order":{"type":"object","description":"an order","properties":{"id":{"description":"its key"},"lines":{"type":"array","items":{"type":"object","properties":{"sku":{"description":"nested"}}}}}}}}`))
	rawStrings, err18 := NewRawSchema(json.RawMessage(`{"type":"array","description":"order ids","items":{"type":"string","description":"an order id"}}`))
	if err := errors.Join(err1, err2, err3, err4, err5, err6, err7, err8, err9, err10, err11, err12, err13, err14, err15, err16, err17, err18); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		v    View
		want string
	}{
		{"tagged", tagged.View(), "Result: rows found"},
		{"tagged in object mode", tagged.Object().View(), "Result: rows found"},
		{"untagged", plain.View(), ""},
		{"embedded before the tag", outer.View(), "Result: outer"},
		{"non-object", text.View(), ""},
		{"non-object in object mode", text.Object().View(), ""},
		{"raw", raw.View(), "Result: orders"},
		{"raw non-object", rawText.View(), ""},
		{"raw nullable object", nullable.View(), ""},
		{"boolean true", accept.View(), ""},
		{"boolean false", reject.View(), ""},
		{"any raw", AnyRawSchema.View(), ""},
		{"any", anything.View(), ""},
		{"root and fields, nested fields left out", rows.View(), "Result: rows found\nlast: the bottom row\nrows: in sheet order\ntruncated: more rows remain"},
		{"fields without a root description", fields.View(), "Result:\ncount: how many matched"},
		{"fields in the schema's order", ordered.View(), "Result: orders\ntotal: in cents\ndue: a date"},
		{"a list", list.View(), "Result: one row\nid: the row's key"},
		{"a list of pointers", pointers.View(), "Result: one row\nid: the row's key"},
		{"a list in object mode", list.Object().View(), ""},
		{"a list of strings", strs.View(), ""},
		{"a raw list describing itself", rawList.View(), "Result: orders\nid: its key"},
		{"a raw list of described strings", rawStrings.View(), "Result: order ids"},
	} {
		if c.v.Brief != c.want {
			t.Fatalf("%s: Brief %q, want %q", c.name, c.v.Brief, c.want)
		}
	}
	for _, c := range []struct {
		name string
		v    View
		want string
	}{
		{"tagged", tagged.View(), "rows found"},
		{"tagged in object mode", tagged.Object().View(), "rows found"},
		{"embedded before the tag", outer.View(), "outer"},
		{"raw", raw.View(), "orders"},
	} {
		var root struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(c.v.Root, &root); err != nil {
			t.Fatal(err)
		}
		if root.Description != c.want {
			t.Fatalf("%s: root description %q, want %q", c.name, root.Description, c.want)
		}
	}
}

type viewRow struct {
	ID string `json:"id"`
}

type viewRows struct {
	Rows []viewRow `json:"rows"`
	_    struct{}  `description:"rows found"`
}

func TestView(t *testing.T) {
	rows, err1 := NewSchema[viewRows]()
	list, err2 := NewSchema[[]viewRow]()
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}
	const row = `{"properties":{"id":{"type":"string"}},"type":"object"}`
	const root = `{"description":"rows found","properties":{"rows":{"items":{"$ref":"#/definitions/SchemaViewRow"},"type":["array","null"]}},"type":"object"}`
	want := View{
		Raw:    []byte(`{"definitions":{"SchemaViewRow":` + row + `},` + root[1:]),
		Simple: []byte(`{"description":"rows found","properties":{"rows":{"items":` + row + `,"type":["array","null"]}},"type":"object"}`),
		Root:   []byte(root),
		Defs:   map[string]json.RawMessage{"SchemaViewRow": json.RawMessage(row)},
		Brief:  "Result: rows found",
	}
	if got := rows.View(); !reflect.DeepEqual(got, want) {
		t.Fatalf("view %+v, want %+v", got, want)
	}
	if got := rows.Object().View(); !reflect.DeepEqual(got, want) {
		t.Fatalf("an object schema in object mode shows %+v, want %+v", got, want)
	}
	want = View{
		Raw:    []byte(`{"definitions":{"SchemaViewRow":` + row + `},"properties":{"value":{"items":{"$ref":"#/definitions/SchemaViewRow"},"type":"array"}},"required":["value"],"type":"object"}`),
		Simple: []byte(`{"properties":{"value":{"items":` + row + `,"type":"array"}},"required":["value"],"type":"object"}`),
		Root:   []byte(`{"type":"object","properties":{"value":{"items":{"$ref":"#/definitions/SchemaViewRow"},"type":"array"}},"required":["value"]}`),
		Defs:   map[string]json.RawMessage{"SchemaViewRow": json.RawMessage(row)},
	}
	if got := list.Object().View(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a list in object mode shows %+v, want %+v", got, want)
	}
	if got := list.View(); string(got.Root) != `{"items":{"$ref":"#/definitions/SchemaViewRow"},"type":"array"}` || got.Brief != "" {
		t.Fatalf("object mode changed the list to %+v", got)
	}
}

func TestObjectSchemasKeepTheirShape(t *testing.T) {
	empty, err := NewSchema[struct{}]()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(empty.View().Raw); got != `{"type":"object"}` {
		t.Fatalf("an empty struct's schema is %s, want it as declared", got)
	}
	if got := string(empty.View().Simple); got != `{"type":"object","properties":{}}` {
		t.Fatalf("an empty struct's simple schema is %s, want an empty properties object", got)
	}
	if got := string(ObjectRawSchema.View().Simple); got != `{"type":"object","properties":{}}` {
		t.Fatalf("the object schema's simple form is %s, want an empty properties object", got)
	}
	for raw, simple := range map[string]string{
		`{"type":"object","description":"rows found"}`: `{"type":"object","description":"rows found","properties":{}}`,
		`{"type":"object","properties":{"a":{}}}`:      `{"type":"object","properties":{"a":{}}}`,
		`{"type":["object","null"]}`:                   `{"type":["object","null"],"properties":{}}`,
		`{"type":"string"}`:                            `{"type":"string"}`,
	} {
		s, err := NewRawSchema(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(s.View().Raw); got != raw {
			t.Fatalf("raw schema %s became %s, want it as declared", raw, got)
		}
		if got := string(s.View().Simple); got != simple {
			t.Fatalf("raw schema %s has the simple form %s, want %s", raw, got, simple)
		}
	}
}
