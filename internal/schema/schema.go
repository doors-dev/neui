package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	jsonvalid "github.com/santhosh-tekuri/jsonschema/v6"
	jsonreflect "github.com/swaggest/jsonschema-go"
	"github.com/zeebo/xxh3"
)

var (
	ErrInvalidInput     = errors.New("invalid input: argument is not valid JSON")
	ErrInvalidArguments = errors.New("invalid input: argument do not match the schema")
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
		Schema: validSchema,
	}
	return schema, nil
}

type Schema[T any] struct {
	raw []byte
	*jsonvalid.Schema
}

func (s Schema[T]) DecodeString(str string) (T, error) {
	return s.Decode([]byte(str))
}

func (s Schema[T]) Validate(v any) error {
	if err := s.Schema.Validate(v); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArguments, err)
	}
	return nil
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
	reflectedSchema, err := ref.Reflect(sample, jsonreflect.ProcessWithoutTags)
	if err != nil {
		return Schema[T]{}, err
	}
	raw, err := reflectedSchema.JSONSchemaBytes()
	if err != nil {
		return Schema[T]{}, err
	}
	if bytes.Equal(raw, []byte("{}")) {
		raw = anySchema
	}
	doc, err := jsonvalid.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return Schema[T]{}, err
	}
	schema, err := newSchema[T](raw, doc, jsonvalid.Draft7)
	if err != nil {
		return Schema[T]{}, err
	}
	schemas.Store(t, schema)
	return schema, nil
}
