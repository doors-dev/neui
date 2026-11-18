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
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	jsonvalid "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	jsonreflect "github.com/swaggest/jsonschema-go"
	"github.com/zeebo/xxh3"
)

var (
	ErrInvalidInput     = errors.New("invalid input: argument is not valid JSON")
	ErrInvalidArguments = errors.New("invalid input: argument do not match the schema")
	errCallShape        = errors.New("schema: x-neui-call without allOf[0].properties arg and returns")
)

var schemas sync.Map
var rawSchemas = newCache(time.Hour)

func newSchema[T any](raw []byte, doc any, defaultDraft *jsonvalid.Draft) (Schema[T], error) {
	const loc = "urn:neui:input"
	c := jsonvalid.NewCompiler()
	c.UseLoader(jsonvalid.SchemeURLLoader{})
	c.DefaultDraft(defaultDraft)
	if err := c.AddResource(loc, doc); err != nil {
		return Schema[T]{}, err
	}
	validSchema, err := c.Compile(loc)
	if err != nil {
		return Schema[T]{}, err
	}
	schema := Schema[T]{
		raw:    raw,
		root:   raw,
		Schema: validSchema,
	}
	return schema, nil
}

type Schema[T any] struct {
	raw  []byte
	root []byte
	defs map[string]json.RawMessage
	*jsonvalid.Schema
}

func (s Schema[T]) DecodeString(str string) (T, error) {
	return s.Decode([]byte(str))
}

func (s Schema[T]) Validate(v any) error {
	if err := s.Schema.Validate(v); err != nil {
		if verr, ok := err.(*jsonvalid.ValidationError); ok {
			verr.Causes = pruneAnyOf(verr.Causes)
		}
		return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	return nil
}

func pruneAnyOf(causes []*jsonvalid.ValidationError) []*jsonvalid.ValidationError {
	var pruned []*jsonvalid.ValidationError
	for _, cause := range causes {
		if _, ok := cause.ErrorKind.(*kind.AnyOf); ok {
			typeLoc := append(slices.Clip(cause.InstanceLocation), "type")
			var match *jsonvalid.ValidationError
			matches := 0
			for _, branch := range cause.Causes {
				mismatched := false
				stack := []*jsonvalid.ValidationError{branch}
				for len(stack) > 0 && !mismatched {
					e := stack[len(stack)-1]
					stack = append(stack[:len(stack)-1], e.Causes...)
					wrongValue := false
					switch e.ErrorKind.(type) {
					case *kind.Type, *kind.Const, *kind.Enum:
						wrongValue = true
					}
					mismatched = slices.Equal(e.InstanceLocation, typeLoc) || wrongValue && slices.Equal(e.InstanceLocation, cause.InstanceLocation)
				}
				if !mismatched {
					match = branch
					matches++
				}
			}
			if matches == 1 {
				branch := []*jsonvalid.ValidationError{match}
				switch match.ErrorKind.(type) {
				case *kind.Group, *kind.Reference:
					branch = match.Causes
				}
				pruned = append(pruned, pruneAnyOf(branch)...)
				continue
			}
		}
		cause.Causes = pruneAnyOf(cause.Causes)
		pruned = append(pruned, cause)
	}
	return pruned
}

func (s Schema[T]) ValidateString(str string) error {
	return s.ValidateRaw([]byte(str))
}

func (s Schema[T]) ValidateRaw(raw []byte) error {
	inst, err := jsonvalid.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return s.Validate(inst)
}

func (s Schema[T]) Decode(raw []byte) (T, error) {
	var arg T
	if err := s.ValidateRaw(raw); err != nil {
		return arg, err
	}
	if err := json.Unmarshal(raw, &arg); err != nil {
		return arg, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return arg, nil
}

func (s Schema[T]) Raw() []byte {
	return s.raw
}

func (s Schema[T]) Root() []byte {
	return s.root
}

func (s Schema[T]) Defs() map[string]json.RawMessage {
	return s.defs
}

func (s Schema[T]) MarshalJSON() ([]byte, error) {
	return s.raw, nil
}

func (s Schema[T]) Params() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(s.raw, &m); err != nil {
		panic(fmt.Sprintf("schema: corrupt Raw: %v", err))
	}
	return m
}

var anySchema = []byte(`{"description":"any JSON value"}`)

var objectSchema = []byte(`{"type":"object","properties":{}}`)

var blocksSchema = []byte(`{"type":"array","description":"content blocks, in order","items":{"oneOf":[` +
	`{"type":"object","properties":{"type":{"const":"text"},"text":{"type":"string"}},"required":["type","text"]},` +
	`{"type":"object","properties":{"type":{"const":"image"},"data":{"type":"string","contentEncoding":"base64"},"mimeType":{"type":"string"}},"required":["type","data","mimeType"]},` +
	`{"type":"object","properties":{"type":{"const":"document"},"data":{"type":"string","contentEncoding":"base64"},"mimeType":{"type":"string"},"name":{"type":"string"}},"required":["type","data","mimeType"]}` +
	`]}}`)

var AnyRawSchema = mustRawSchema(anySchema)

var BlocksRawSchema = mustRawSchema(blocksSchema)

var ObjectRawSchema = mustRawSchema(objectSchema)

func mustRawSchema(raw []byte) Schema[any] {
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	schema, err := newSchema[any](raw, doc, jsonvalid.Draft2020)
	if err != nil {
		panic(err)
	}
	return schema
}

func NewRawSchema(s any) (Schema[any], error) {
	if s == nil {
		return AnyRawSchema, nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return Schema[any]{}, err
	}
	if bytes.Equal(raw, []byte("{}")) || bytes.Equal(raw, []byte("null")) {
		return AnyRawSchema, nil
	}
	hash := xxh3.Hash(raw)
	if schema, ok := rawSchemas.get(hash); ok {
		return schema, nil
	}
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return Schema[any]{}, err
	}
	schema, err := newSchema[any](raw, doc, jsonvalid.Draft2020)
	if err != nil {
		return Schema[any]{}, err
	}
	rawSchemas.put(hash, schema)
	return schema, nil
}

func NewSchema[T any]() (Schema[T], error) {
	t := reflect.TypeFor[T]()
	if entry, ok := schemas.Load(t); ok {
		return entry.(Schema[T]), nil
	}
	var sample T
	ref := jsonreflect.Reflector{}
	reflectedSchema, err := ref.Reflect(sample, jsonreflect.ProcessWithoutTags, jsonreflect.RootRef)
	if err != nil {
		return Schema[T]{}, err
	}
	reflected, err := reflectedSchema.JSONSchemaBytes()
	if err != nil {
		return Schema[T]{}, err
	}
	var top, defs map[string]json.RawMessage
	if err := json.Unmarshal(reflected, &top); err != nil {
		return Schema[T]{}, err
	}
	root := reflected
	if definitions, ok := top["definitions"]; ok {
		if err := json.Unmarshal(definitions, &defs); err != nil {
			return Schema[T]{}, err
		}
		delete(top, "definitions")
		if root, err = json.Marshal(top); err != nil {
			return Schema[T]{}, err
		}
	}
	raw, root, err := splitCalls(root)
	if err != nil {
		return Schema[T]{}, err
	}
	rawDefs := map[string]json.RawMessage{}
	for name, def := range defs {
		if rawDefs[name], defs[name], err = splitCalls(def); err != nil {
			return Schema[T]{}, err
		}
	}
	var target string
	if len(top) == 1 && top["$ref"] != nil {
		if err := json.Unmarshal(top["$ref"], &target); err != nil {
			return Schema[T]{}, err
		}
	}
	if name, ok := strings.CutPrefix(target, "#/definitions/"); ok && defs[name] != nil {
		root = defs[name]
		quoted, err := json.Marshal(target)
		if err != nil {
			return Schema[T]{}, err
		}
		raw = bytes.ReplaceAll(rawDefs[name], quoted, []byte(`"#"`))
		delete(rawDefs, name)
		referenced := false
		for n, def := range defs {
			referenced = referenced || bytes.Contains(def, quoted)
			if n != name {
				rawDefs[n] = bytes.ReplaceAll(rawDefs[n], quoted, []byte(`"#"`))
			}
		}
		if !referenced {
			delete(defs, name)
		}
	}
	if len(rawDefs) > 0 {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			return Schema[T]{}, err
		}
		if doc["definitions"], err = json.Marshal(rawDefs); err != nil {
			return Schema[T]{}, err
		}
		if raw, err = json.Marshal(doc); err != nil {
			return Schema[T]{}, err
		}
	}
	if bytes.Equal(raw, []byte("{}")) {
		root, raw, defs = anySchema, anySchema, nil
	}
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return Schema[T]{}, err
	}
	schema, err := newSchema[T](raw, doc, jsonvalid.Draft7)
	if err != nil {
		return Schema[T]{}, err
	}
	schema.root = root
	schema.defs = defs
	schemas.Store(t, schema)
	return schema, nil
}

func splitCalls(doc []byte) ([]byte, []byte, error) {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, err
	}
	rawValue, specValue, err := calls(v)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(rawValue)
	if err != nil {
		return nil, nil, err
	}
	spec, err := json.Marshal(specValue)
	if err != nil {
		return nil, nil, err
	}
	return raw, spec, nil
}

func calls(v any) (any, any, error) {
	switch v := v.(type) {
	case []any:
		raw, spec := make([]any, len(v)), make([]any, len(v))
		for i, item := range v {
			var err error
			if raw[i], spec[i], err = calls(item); err != nil {
				return nil, nil, err
			}
		}
		return raw, spec, nil
	case map[string]any:
		if _, ok := v["x-neui-call"]; !ok {
			raw, spec := make(map[string]any, len(v)), make(map[string]any, len(v))
			for key, item := range v {
				var err error
				if raw[key], spec[key], err = calls(item); err != nil {
					return nil, nil, err
				}
			}
			return raw, spec, nil
		}
		var props map[string]any
		if allOf, ok := v["allOf"].([]any); ok && len(allOf) > 0 {
			first, _ := allOf[0].(map[string]any)
			props, _ = first["properties"].(map[string]any)
		}
		arg, hasArg := props["arg"]
		returns, hasReturns := props["returns"]
		if !hasArg || !hasReturns {
			return nil, nil, errCallShape
		}
		raw := map[string]any{"type": v["type"]}
		if desc, ok := v["description"]; ok {
			raw["description"] = desc
		}
		function := map[string]any{}
		for key, schema := range map[string]any{"arg": arg, "returns": returns} {
			_, spec, err := calls(schema)
			if err != nil {
				return nil, nil, err
			}
			if m, ok := spec.(map[string]any); ok && len(m) == 0 {
				spec = map[string]any{"description": "any JSON value"}
			}
			function[key] = spec
		}
		spec := maps.Clone(raw)
		spec["function"] = function
		return raw, spec, nil
	}
	return v, v, nil
}
