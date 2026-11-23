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
	"strings"
	"testing"

	jsonvalid "github.com/santhosh-tekuri/jsonschema/v6"
)

type objectItem struct {
	Name string `json:"name"`
}

func TestObjectKeepsAnObjectSchema(t *testing.T) {
	s, err := NewSchema[objectItem]()
	if err != nil {
		t.Fatal(err)
	}
	o := s.Object()
	if !bytes.Equal(o.View().Raw, s.View().Raw) || !bytes.Equal(o.View().Root, s.View().Root) {
		t.Fatalf("an object schema changed in object mode: %s", o.View().Raw)
	}
	in := []byte(`{"name":"a"}`)
	got, err := o.Decode(in)
	if err != nil || got.Name != "a" {
		t.Fatalf("an object schema in object mode decoded %+v, %v, want the value as it came", got, err)
	}
}

func TestObjectWrapsAValue(t *testing.T) {
	s, err := NewSchema[string]()
	if err != nil {
		t.Fatal(err)
	}
	o := s.Object()
	var shown struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(o.View().Raw, &shown); err != nil {
		t.Fatal(err)
	}
	if shown.Type != "object" || len(shown.Required) != 1 || shown.Required[0] != "value" || !bytes.Equal(shown.Properties["value"], s.View().Raw) {
		t.Fatalf("a string schema in object mode shows %s, want it under a required value", o.View().Raw)
	}
	if !bytes.Equal(s.View().Raw, []byte(`{"type":"string"}`)) {
		t.Fatalf("object mode changed the original schema to %s", s.View().Raw)
	}
	got, err := o.Decode([]byte(`{"value":"hello"}`))
	if err != nil || got != "hello" {
		t.Fatalf("decoded %q, %v, want the inner value", got, err)
	}
	if _, err := o.Decode([]byte(`{"input":"hello"}`)); !errors.Is(err, ErrNoInput) {
		t.Fatalf("a call without input answered %v, want ErrNoInput", err)
	}
	if _, err := o.Decode([]byte(`{"value":1}`)); !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("a wrong type under value answered %v, want the schema error", err)
	}
	if _, err := s.Decode([]byte(`"hello"`)); err != nil {
		t.Fatalf("the original schema no longer decodes a bare value: %v", err)
	}
}

func TestObjectHoistsDefinitions(t *testing.T) {
	s, err := NewSchema[[]objectItem]()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(s.View().Raw, []byte(`"definitions"`)) {
		t.Skipf("the reflected schema has no definitions to hoist: %s", s.View().Raw)
	}
	o := s.Object()
	var shown map[string]json.RawMessage
	if err := json.Unmarshal(o.View().Raw, &shown); err != nil {
		t.Fatal(err)
	}
	if shown["definitions"] == nil || bytes.Contains(shown["properties"], []byte(`"definitions"`)) {
		t.Fatalf("definitions were not moved to the top: %s", o.View().Raw)
	}
	got, err := o.Decode([]byte(`{"value":[{"name":"a"}]}`))
	if err != nil || len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("decoded %+v, %v, want the list", got, err)
	}
}

func TestObjectOnARawSchema(t *testing.T) {
	s, err := NewRawSchema(map[string]any{"type": "array", "items": map[string]any{"type": "integer"}})
	if err != nil {
		t.Fatal(err)
	}
	o := s.Object()
	inner, err := o.Unwrap([]byte(`{"value":[1,2]}`))
	if err != nil || !bytes.Equal(inner, []byte(`[1,2]`)) {
		t.Fatalf("Unwrap gave %s, %v, want the inner bytes", inner, err)
	}
	if err := o.ValidateRaw([]byte(`{"value":[1,2]}`)); err != nil {
		t.Fatalf("the object form failed in object mode: %v", err)
	}
	if err := o.ValidateRaw(inner); !errors.Is(err, ErrNoInput) {
		t.Fatalf("the inner value alone answered %v in object mode, want ErrNoInput", err)
	}
	if err := s.ValidateRaw(inner); err != nil {
		t.Fatalf("the inner value failed the original schema: %v", err)
	}
	same, err := s.Unwrap([]byte(`[1,2]`))
	if err != nil || !bytes.Equal(same, []byte(`[1,2]`)) {
		t.Fatalf("Unwrap outside object mode changed the bytes to %s, %v", same, err)
	}
}

type objectNode struct {
	Name string     `json:"name"`
	Kids objectList `json:"kids,omitempty"`
}

type objectList []objectNode

func checkObjectMatchesRaw[T any](t *testing.T, name string, s Schema[T], draft *jsonvalid.Draft, inputs map[string]bool) {
	t.Helper()
	o := s.Object()
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(o.View().Raw))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(o.View().Raw, &top); err != nil || !bytes.Equal(top["type"], []byte(`"object"`)) {
		t.Fatalf("%s: object mode declares %s, want an object schema", name, o.View().Raw)
	}
	if err := json.Unmarshal(o.View().Root, &top); err != nil || !bytes.Equal(top["type"], []byte(`"object"`)) {
		t.Fatalf("%s: object mode root is %s, want an object schema", name, o.View().Root)
	}
	c := jsonvalid.NewCompiler()
	c.DefaultDraft(draft)
	if err := c.AddResource("urn:declared", doc); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	declared, err := c.Compile("urn:declared")
	if err != nil {
		t.Fatalf("%s: the declared schema %s does not compile: %v", name, o.View().Raw, err)
	}
	for in, want := range inputs {
		inst, err := jsonvalid.UnmarshalJSON(strings.NewReader(in))
		if err != nil {
			t.Fatalf("%s: %s: %v", name, in, err)
		}
		if got := declared.Validate(inst) == nil; got != want {
			t.Errorf("%s: the declared schema accepts %s: %v, want %v", name, in, got, want)
		}
		_, decodeErr := o.Decode([]byte(in))
		_, decodeStringErr := o.DecodeString(in)
		for method, err := range map[string]error{
			"Decode":         decodeErr,
			"DecodeString":   decodeStringErr,
			"ValidateRaw":    o.ValidateRaw([]byte(in)),
			"ValidateString": o.ValidateString(in),
			"Validate":       o.Validate(inst),
		} {
			if got := err == nil; got != want {
				t.Errorf("%s: %s accepts %s: %v (%v), want %v", name, method, in, got, err, want)
			}
		}
	}
}

func TestObjectMatchesDeclaredSchema(t *testing.T) {
	text, err1 := NewSchema[string]()
	items, err2 := NewSchema[[]objectItem]()
	list, err3 := NewSchema[objectList]()
	anyValue, err4 := NewSchema[any]()
	message, err5 := NewSchema[json.RawMessage]()
	var defsDoc any
	err6 := json.Unmarshal([]byte(`{"type":"array","items":{"$ref":"#/$defs/n"},"$defs":{"n":{"type":"object","properties":{"kids":{"$ref":"#"},"v":{"type":"integer"}}}}}`), &defsDoc)
	defs, err7 := NewRawSchema(defsDoc)
	if err := errors.Join(err1, err2, err3, err4, err5, err6, err7); err != nil {
		t.Fatal(err)
	}
	checkObjectMatchesRaw(t, "string", text, jsonvalid.Draft7, map[string]bool{
		`{"value":"a"}`:       true,
		`{"value":"a","x":1}`: true,
		`{"value":1}`:         false,
		`{"value":null}`:      false,
		`{"VALUE":"a"}`:       false,
		`{}`:                  false,
		`"a"`:                 false,
		`null`:                false,
	})
	checkObjectMatchesRaw(t, "slice of structs", items, jsonvalid.Draft7, map[string]bool{
		`{"value":[]}`:             true,
		`{"value":[{"name":"a"}]}`: true,
		`{"value":[{"name":1}]}`:   false,
		`{"value":{"name":"a"}}`:   false,
		`[{"name":"a"}]`:           false,
	})
	checkObjectMatchesRaw(t, "recursive list", list, jsonvalid.Draft7, map[string]bool{
		`{"value":[{"name":"a","kids":[{"name":"b","kids":[{"name":"c"}]}]}]}`: true,
		`{"value":[{"name":"a","kids":[{"name":1}]}]}`:                         false,
		`{"value":[{"name":"a","kids":[{"name":"b","kids":[{"name":1}]}]}]}`:   false,
		`{"value":[{"name":"a","kids":{"value":[]}}]}`:                         false,
		`[{"name":"a"}]`: false,
	})
	for name, s := range map[string]Schema[any]{"any raw": AnyRawSchema, "any": anyValue} {
		checkObjectMatchesRaw(t, name, s, jsonvalid.Draft2020, map[string]bool{
			`{"value":1}`:       true,
			`{"value":null}`:    true,
			`{"value":{"a":1}}`: true,
			`{}`:                false,
			`{"a":1}`:           false,
			`"a"`:               false,
			`[1]`:               false,
			`null`:              false,
		})
	}
	checkObjectMatchesRaw(t, "raw message", message, jsonvalid.Draft7, map[string]bool{
		`{"value":1}`:  true,
		`{"value":{}}`: true,
		`{}`:           false,
		`1`:            false,
	})
	checkObjectMatchesRaw(t, "raw $defs", defs, jsonvalid.Draft2020, map[string]bool{
		`{"value":[]}`:                           true,
		`{"value":[{"v":1,"kids":[{"v":2}]}]}`:   true,
		`{"value":[{"v":1,"kids":[{"v":"x"}]}]}`: false,
		`{"value":[{"v":1,"kids":{"v":2}}]}`:     false,
		`{"value":[{"v":"x"}]}`:                  false,
		`[{"v":1}]`:                              false,
	})
}

func TestObjectRewritesSelfReferencesInDefinitions(t *testing.T) {
	s, err := NewSchema[objectList]()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(s.View().Raw, []byte(`"$ref":"#"`)) {
		t.Skipf("the reflected schema has no self reference: %s", s.View().Raw)
	}
	o := s.Object()
	if bytes.Contains(o.View().Raw, []byte(`"$ref":"#"`)) || !bytes.Contains(o.View().Raw, []byte(`"$ref":"#/properties/value"`)) {
		t.Fatalf("a self reference still points at the wrapper: %s", o.View().Raw)
	}
}

func TestObjectAnswersNoInput(t *testing.T) {
	s, err := NewSchema[string]()
	if err != nil {
		t.Fatal(err)
	}
	o := s.Object()
	for _, in := range []string{`"x"`, `null`, `{}`, `[1]`, `1`, `{"input":"x"}`, `{"VALUE":"x"}`, `{"Value":"x"}`} {
		_, decodeErr := o.Decode([]byte(in))
		_, decodeStringErr := o.DecodeString(in)
		_, unwrapErr := o.Unwrap([]byte(in))
		for method, err := range map[string]error{
			"Decode":         decodeErr,
			"DecodeString":   decodeStringErr,
			"Unwrap":         unwrapErr,
			"ValidateRaw":    o.ValidateRaw([]byte(in)),
			"ValidateString": o.ValidateString(in),
		} {
			if err != ErrNoInput {
				t.Errorf("%s of %s answered %v, want exactly ErrNoInput", method, in, err)
			}
		}
	}
	for _, in := range []string{``, `{`, `{"value":`, `{"value":"x"} x`} {
		_, decodeErr := o.Decode([]byte(in))
		_, unwrapErr := o.Unwrap([]byte(in))
		for method, err := range map[string]error{"Decode": decodeErr, "Unwrap": unwrapErr, "ValidateRaw": o.ValidateRaw([]byte(in))} {
			if !errors.Is(err, ErrInvalidInput) || strings.Contains(err.Error(), "Go value") || strings.Contains(err.Error(), "json.RawMessage") || strings.Contains(err.Error(), "jsontext") {
				t.Errorf("%s of %q answered %v, want ErrInvalidInput without Go type names", method, in, err)
			}
		}
	}
}

func TestObjectOnASchemaWithoutAType(t *testing.T) {
	o := AnyRawSchema.Object()
	if string(o.View().Raw) != `{"properties":{"value":{"description":"any JSON value"}},"required":["value"],"type":"object"}` {
		t.Fatalf("an any schema in object mode shows %s, want it under a required value", o.View().Raw)
	}
	if string(AnyRawSchema.View().Raw) != `{"description":"any JSON value"}` {
		t.Fatalf("object mode changed the any schema to %s", AnyRawSchema.View().Raw)
	}
	inner, err := o.Unwrap([]byte(`{"value":{"a":1}}`))
	if err != nil || !bytes.Equal(inner, []byte(`{"a":1}`)) {
		t.Fatalf("Unwrap gave %s, %v, want the inner value", inner, err)
	}
	for _, in := range []string{`"a"`, `{}`, `{"a":1}`} {
		if err := o.ValidateRaw([]byte(in)); err != ErrNoInput {
			t.Fatalf("%s answered %v, want ErrNoInput", in, err)
		}
	}
}
