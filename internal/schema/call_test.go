package schema_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	jsonreflect "github.com/swaggest/jsonschema-go"
	"neui.dev/internal/island"
	"neui.dev/internal/schema"
	"neui.dev/neisland/form"
)

type callSelf struct {
	Name string                          `json:"name"`
	Kids island.Call[string, []callSelf] `json:"kids" description:"finds <kids>"`
}

type callA struct {
	B island.Call[int, callB] `json:"b"`
}

type callB struct {
	C island.Call[callC, int] `json:"c"`
}

type callC struct {
	A []island.Call[callA, callA] `json:"a"`
}

type callBroken string

func (callBroken) InlineJSONSchema() {}

func (callBroken) PrepareJSONSchema(s *jsonreflect.Schema) error {
	s.WithExtraPropertiesItem("x-neui-call", true)
	return nil
}

type callBrokenParams struct {
	F callBroken `json:"f"`
}

func checkCalls[T any](t *testing.T) schema.Schema[T] {
	t.Helper()
	type result struct {
		s   schema.Schema[T]
		err error
	}
	done := make(chan result, 1)
	go func() {
		s, err := schema.NewSchema[T]()
		done <- result{s, err}
	}()
	var s schema.Schema[T]
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		s = r.s
	case <-time.After(5 * time.Second):
		t.Fatal("NewSchema did not return")
	}
	for _, key := range []string{`"x-neui-call"`, `"allOf"`, `"function"`} {
		if strings.Contains(string(s.Raw()), key) {
			t.Fatalf("%s in raw: %s", key, s.Raw())
		}
	}
	docs := []json.RawMessage{s.Root()}
	for _, def := range s.Defs() {
		docs = append(docs, def)
	}
	for _, doc := range docs {
		if strings.Contains(string(doc), `"x-neui-call"`) || strings.Contains(string(doc), `"allOf"`) {
			t.Fatalf("call marker in spec: %s", doc)
		}
	}
	return s
}

func TestCallSelfReference(t *testing.T) {
	s := checkCalls[callSelf](t)
	want := `{"properties":{"kids":{"description":"finds \u003ckids\u003e","function":{"arg":{"type":"string"},"returns":{"items":{"$ref":"#/definitions/SchemaTestCallSelf"},"type":"array"}},"type":"string"},"name":{"type":"string"}},"type":"object"}`
	if string(s.Root()) != want {
		t.Fatalf("root %s", s.Root())
	}
	if names := slices.Sorted(maps.Keys(s.Defs())); !slices.Equal(names, []string{"SchemaTestCallSelf"}) || string(s.Defs()["SchemaTestCallSelf"]) != want {
		t.Fatalf("defs %v", s.Defs())
	}
	if string(s.Raw()) != `{"properties":{"kids":{"description":"finds \u003ckids\u003e","type":"string"},"name":{"type":"string"}},"type":"object"}` {
		t.Fatalf("raw %s", s.Raw())
	}
	if _, err := s.DecodeString(`{"name":"a","kids":"(name) => []"}`); err != nil {
		t.Fatalf("source rejected: %v", err)
	}
	for _, inst := range []string{`{"kids":{}}`, `{"kids":{"arg":"x","returns":[]}}`, `{"kids":1}`} {
		if s.ValidateString(inst) == nil {
			t.Fatalf("%s accepted", inst)
		}
	}
}

func TestCallChain(t *testing.T) {
	s := checkCalls[callA](t)
	if string(s.Root()) != `{"properties":{"b":{"function":{"arg":{"type":"integer"},"returns":{"$ref":"#/definitions/SchemaTestCallB"}},"type":"string"}},"type":"object"}` {
		t.Fatalf("root %s", s.Root())
	}
	if names := slices.Sorted(maps.Keys(s.Defs())); !slices.Equal(names, []string{"SchemaTestCallA", "SchemaTestCallB", "SchemaTestCallC"}) {
		t.Fatalf("defs %v", names)
	}
	if string(s.Defs()["SchemaTestCallB"]) != `{"properties":{"c":{"function":{"arg":{"$ref":"#/definitions/SchemaTestCallC"},"returns":{"type":"integer"}},"type":"string"}},"type":"object"}` {
		t.Fatalf("callB %s", s.Defs()["SchemaTestCallB"])
	}
	if string(s.Defs()["SchemaTestCallC"]) != `{"properties":{"a":{"items":{"function":{"arg":{"$ref":"#/definitions/SchemaTestCallA"},"returns":{"$ref":"#/definitions/SchemaTestCallA"}},"type":"string"},"type":["array","null"]}},"type":"object"}` {
		t.Fatalf("callC %s", s.Defs()["SchemaTestCallC"])
	}
	if _, err := s.DecodeString(`{"b":"(n) => ({c: '(c) => 1'})"}`); err != nil {
		t.Fatalf("source rejected: %v", err)
	}
	if s.ValidateString(`{"b":{"c":"(c) => 1"}}`) == nil {
		t.Fatal("object accepted as source")
	}
	checkCalls[callB](t)
	checkCalls[callC](t)
}

func TestCallFormSearch(t *testing.T) {
	s := checkCalls[form.Params](t)
	var search struct {
		Properties struct {
			Search struct {
				Type     string `json:"type"`
				Function struct {
					Arg     json.RawMessage `json:"arg"`
					Returns struct {
						Type  string `json:"type"`
						Items struct {
							Ref string `json:"$ref"`
						} `json:"items"`
					} `json:"returns"`
				} `json:"function"`
			} `json:"search"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(s.Defs()["FormSearchParams"], &search); err != nil {
		t.Fatal(err)
	}
	call := search.Properties.Search
	if call.Type != "string" || string(call.Function.Arg) != `{"type":"string"}` || call.Function.Returns.Type != "array" || call.Function.Returns.Items.Ref != "#/definitions/FormOption" || s.Defs()["FormOption"] == nil {
		t.Fatalf("search: %s", s.Defs()["FormSearchParams"])
	}
	if err := s.ValidateString(`{"fields":[{"type":"search","label":"City","name":"city","search":"(q) => app.cities({q})"}]}`); err != nil {
		t.Fatalf("source rejected: %v", err)
	}
	if s.ValidateString(`{"fields":[{"type":"search","label":"City","name":"city","search":{}}]}`) == nil {
		t.Fatal("object accepted as source")
	}
}

func TestCallMarkerWithoutFunction(t *testing.T) {
	if _, err := schema.NewSchema[callBrokenParams](); err == nil {
		t.Fatal("marker without allOf accepted")
	}
}
