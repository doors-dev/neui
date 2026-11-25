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
	ErrInvalidArguments = errors.New("invalid input: argument does not match the schema")
	ErrNoInput          = errors.New(`invalid input: argument must be {"value": <value>}`)
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
	objectRaw, err := wrap(raw)
	if err != nil {
		return Schema[T]{}, err
	}
	schema := Schema[T]{
		raw:       raw,
		simple:    simplify(raw),
		root:      raw,
		brief:     brief(raw),
		objectRaw: objectRaw,
		Schema:    validSchema,
	}
	if objectRaw != nil {
		schema.objectSimple = simplify(objectRaw)
		objectDoc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(objectRaw))
		if err != nil {
			return Schema[T]{}, err
		}
		c := jsonvalid.NewCompiler()
		c.UseLoader(jsonvalid.SchemeURLLoader{})
		c.DefaultDraft(defaultDraft)
		if err := c.AddResource(loc, objectDoc); err != nil {
			return Schema[T]{}, err
		}
		if schema.objectSchema, err = c.Compile(loc); err != nil {
			return Schema[T]{}, err
		}
	}
	return schema, nil
}

func brief(raw []byte) string {
	var compact bytes.Buffer
	if json.Compact(&compact, raw) != nil {
		return ""
	}
	p := jsonParser{data: compact.Bytes()}
	s := &simplifier{root: p.value(nil)}
	member := func(n *jsonNode, name string) *jsonNode {
		if n == nil || n.raw[0] != '{' {
			return nil
		}
		for _, m := range n.members {
			if m.name == name {
				return m.value
			}
		}
		return nil
	}
	text := func(n *jsonNode, name string) string {
		if v := member(n, name); v != nil && v.raw[0] == '"' {
			return decodeString(v.raw)
		}
		return ""
	}
	object := s.root
	desc := text(object, "description")
	switch text(object, "type") {
	case "object":
	case "array":
		object = member(object, "items")
		for hops := 0; hops < 16 && member(object, "$ref") != nil; hops++ {
			object = s.resolve(text(object, "$ref"))
		}
		if text(object, "type") != "object" {
			object = nil
		} else if desc == "" {
			desc = text(object, "description")
		}
	default:
		return ""
	}
	var lines []string
	if props := member(object, "properties"); props != nil {
		for _, m := range props.members {
			if d := text(m.value, "description"); d != "" {
				lines = append(lines, m.name+": "+d)
			}
		}
	}
	if desc == "" && len(lines) == 0 {
		return ""
	}
	head := "Result:"
	if desc != "" {
		head += " " + desc
	}
	return strings.Join(append([]string{head}, lines...), "\n")
}

func wrap(raw []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		top = nil
	}
	if bytes.Equal(top["type"], []byte(`"object"`)) {
		return nil, nil
	}
	doc := map[string]json.RawMessage{
		"type":     json.RawMessage(`"object"`),
		"required": json.RawMessage(`["value"]`),
	}
	inner := raw
	if top != nil {
		for _, key := range []string{"$schema", "definitions", "$defs"} {
			if v, ok := top[key]; ok {
				doc[key] = v
				delete(top, key)
			}
		}
		var err error
		if inner, err = json.Marshal(top); err != nil {
			return nil, err
		}
	}
	doc["properties"] = json.RawMessage(`{"value":` + string(inner) + `}`)
	object, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(object, []byte(`"$ref":"#"`), []byte(`"$ref":"#/properties/value"`)), nil
}

type Schema[T any] struct {
	raw          []byte
	simple       []byte
	root         []byte
	defs         map[string]json.RawMessage
	brief        string
	objectRaw    []byte
	objectSimple []byte
	objectSchema *jsonvalid.Schema
	object       bool
	*jsonvalid.Schema
}

func (s Schema[T]) Object() Schema[T] {
	if s.objectRaw != nil {
		s.object = true
		s.Schema = s.objectSchema
	}
	return s
}

func (s Schema[T]) Unwrap(raw []byte) ([]byte, error) {
	if !s.object {
		return raw, nil
	}
	var arg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arg); err != nil {
		if json.Valid(raw) {
			return nil, ErrNoInput
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	inner, ok := arg["value"]
	if !ok {
		return nil, ErrNoInput
	}
	return inner, nil
}

func (s Schema[T]) DecodeString(str string) (T, error) {
	return s.Decode([]byte(str))
}

func (s Schema[T]) Validate(v any) error {
	if s.object {
		arg, _ := v.(map[string]any)
		if _, ok := arg["value"]; !ok {
			return ErrNoInput
		}
	}
	if err := s.Schema.Validate(v); err != nil {
		verr, ok := err.(*jsonvalid.ValidationError)
		if !ok {
			return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
		}
		verr.Causes = pruneAnyOf(verr.Causes)
		_, causes, _ := strings.Cut(verr.Error(), "\n")
		return fmt.Errorf("%w:\n%s", ErrInvalidArguments, causes)
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
	raw, err := s.Unwrap(raw)
	if err != nil {
		return arg, err
	}
	if err := json.Unmarshal(raw, &arg); err != nil {
		return arg, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return arg, nil
}

// View is a JSON schema in the forms the agent sees.
type View struct {
	Raw    []byte                     // the whole schema, its definitions included
	Simple []byte                     // Raw with every $ref inlined and "properties":{} on every object schema without one; nil when a $ref cannot be inlined
	Root   []byte                     // Raw without Defs, referring to them as {"$ref":"#/definitions/X"}
	Defs   map[string]json.RawMessage // the definitions split out of Raw
	Brief  string                     // for an object or array schema, "Result:" with the root description (an array's own, else its items'), then a "name: description" line per described top-level property (of the items for an array); "" when nothing is described
}

func (s Schema[T]) View() View {
	if s.object {
		return View{
			Raw:    s.objectRaw,
			Simple: s.objectSimple,
			Root:   []byte(`{"type":"object","properties":{"value":` + string(s.root) + `},"required":["value"]}`),
			Defs:   s.defs,
		}
	}
	return View{Raw: s.raw, Simple: s.simple, Root: s.root, Defs: s.defs, Brief: s.brief}
}

var anySchema = []byte(`{"description":"any JSON value"}`)

var objectSchema = []byte(`{"type":"object"}`)

var blocksSchema = []byte(`{"type":"array","items":{"oneOf":[` +
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
