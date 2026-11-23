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
	"strings"
	"testing"
	"time"
)

type simpleInner struct {
	A string
}

type simpleOnce struct {
	In simpleInner
}

type simpleTwice struct {
	X simpleInner
	Y []simpleInner
	M map[string]simpleInner
}

type simplePair struct {
	P pairA
}

func checkSimple(t *testing.T, name string, v View, want string) {
	t.Helper()
	if want == "" {
		if v.Simple != nil {
			t.Fatalf("%s: simple form %s, want none", name, v.Simple)
		}
		return
	}
	if !json.Valid(v.Simple) {
		t.Fatalf("%s: simple form %q is not valid JSON", name, v.Simple)
	}
	if string(v.Simple) != want {
		t.Fatalf("%s: simple form\n got %s\nwant %s", name, v.Simple, want)
	}
}

func rawView(t *testing.T, raw string) View {
	t.Helper()
	s, err := NewRawSchema(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if string(s.View().Raw) != raw {
		t.Fatalf("raw schema %s became %s, want it as declared", raw, s.View().Raw)
	}
	return s.View()
}

func TestSimpleInlinesNamedTypes(t *testing.T) {
	const inner = `{"properties":{"A":{"type":"string"}},"type":"object"}`
	once, err1 := NewSchema[simpleOnce]()
	twice, err2 := NewSchema[simpleTwice]()
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}
	for _, s := range []View{once.View(), twice.View()} {
		if !bytes.Contains(s.Raw, []byte(`"$ref":"#/definitions/SchemaSimpleInner"`)) || !bytes.Contains(s.Raw, []byte(`"definitions"`)) {
			t.Fatalf("raw %s does not refer to its definitions", s.Raw)
		}
	}
	checkSimple(t, "used once", once.View(), `{"properties":{"In":`+inner+`},"type":"object"}`)
	checkSimple(t, "used twice", twice.View(), `{"properties":{`+
		`"M":{"additionalProperties":`+inner+`,"type":["object","null"],"properties":{}},`+
		`"X":`+inner+`,`+
		`"Y":{"items":`+inner+`,"type":["array","null"]}},"type":"object"}`)
}

func TestSimpleRefusesCycles(t *testing.T) {
	self, err1 := NewSchema[node]()
	pair, err2 := NewSchema[pairA]()
	wrapped, err3 := NewSchema[simplePair]()
	list, err4 := NewSchema[objectList]()
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(wrapped.View().Raw, []byte(`"$ref":"#/definitions/SchemaPairA"`)) {
		t.Fatalf("pairA is not referred to by name: %s", wrapped.View().Raw)
	}
	if !bytes.Contains(list.Object().View().Raw, []byte(`"$ref":"#/properties/value"`)) {
		t.Fatalf("the wrapped list does not refer to its input: %s", list.Object().View().Raw)
	}
	checkSimple(t, "a type that contains itself", self.View(), "")
	checkSimple(t, "a type that contains itself in object mode", self.Object().View(), "")
	checkSimple(t, "mutual recursion through the root", pair.View(), "")
	checkSimple(t, "mutual recursion through definitions", wrapped.View(), "")
	checkSimple(t, "a list that contains itself in object mode", list.Object().View(), "")
	checkSimple(t, "raw mutual recursion", rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/A"}},"$defs":{`+
		`"A":{"type":"object","properties":{"b":{"$ref":"#/$defs/B"}}},`+
		`"B":{"type":"object","properties":{"a":{"$ref":"#/$defs/A"}}}}}`), "")
	checkSimple(t, "a ref to an ancestor", rawView(t, `{"type":"object","properties":{"a":{"type":"array","items":{"$ref":"#/properties/a"}}}}`), "")
	checkSimple(t, "a ref to the root", rawView(t, `{"type":"object","properties":{"a":{"$ref":"#"}}}`), "")
}

func TestSimpleInlinesObjectModeDefinitions(t *testing.T) {
	s, err := NewSchema[[]objectItem]()
	if err != nil {
		t.Fatal(err)
	}
	o := s.Object().View()
	if !bytes.Contains(o.Raw, []byte(`"definitions"`)) {
		t.Fatalf("object mode has no definitions to inline: %s", o.Raw)
	}
	checkSimple(t, "object mode of a list", o, `{"properties":{"value":{"items":{"properties":{"name":{"type":"string"}},"type":"object"},"type":"array"}},"required":["value"],"type":"object"}`)
	checkSimple(t, "the list itself", s.View(), `{"items":{"properties":{"name":{"type":"string"}},"type":"object"},"type":"array"}`)
	str, err := NewSchema[string]()
	if err != nil {
		t.Fatal(err)
	}
	checkSimple(t, "object mode of a string", str.Object().View(), `{"properties":{"value":{"type":"string"}},"required":["value"],"type":"object"}`)
}

func TestSimpleMergesRefSiblings(t *testing.T) {
	v := rawView(t, `{"type":"object","properties":{"a":{"description":"own","$ref":"#/$defs/x","title":"A"}},"$defs":{"x":{"description":"shared","type":"string","minLength":1}}}`)
	checkSimple(t, "a ref with siblings", v, `{"type":"object","properties":{"a":{"description":"own","title":"A","type":"string","minLength":1}}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/x","type":"object"}},"$defs":{"x":{"description":"x","$ref":"#/$defs/y"},"y":{"type":"string","description":"y","properties":{"b":{"type":"object"}}}}}`)
	checkSimple(t, "a chain of refs", v, `{"type":"object","properties":{"a":{"type":"object","description":"x","properties":{"b":{"type":"object","properties":{}}}}}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/x","type":"object"}},"$defs":{"x":{"description":"x"}}}`)
	checkSimple(t, "a sibling type without properties", v, `{"type":"object","properties":{"a":{"type":"object","description":"x","properties":{}}}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/o","type":"string"}},"$defs":{"o":{"type":"object"}}}`)
	checkSimple(t, "a sibling type over an object", v, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/x","items":true}},"$defs":{"x":{"items":{"$ref":"#/$defs/x"}}}}`)
	checkSimple(t, "a cycle under an overridden key", v, `{"type":"object","properties":{"a":{"items":true}}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/t","description":"any"},"b":{"$ref":"#/$defs/t"},"c":{"$ref":"#/$defs/f"}},"$defs":{"t":true,"f":false}}`)
	checkSimple(t, "boolean targets", v, `{"type":"object","properties":{"a":{"description":"any"},"b":{},"c":false}}`)
	v = rawView(t, `{"type":"object","properties":{"a":{"$ref":"#/$defs/f","description":"never"}},"$defs":{"f":false}}`)
	checkSimple(t, "a false target with siblings", v, "")
	v = rawView(t, `{"type":"object","properties":{"a~/b":{"type":"integer"},"c d":{"type":"string"},"e":{"$ref":"#/properties/a~0~1b"},"f":{"$ref":"#/properties/c%20d"}}}`)
	checkSimple(t, "escaped pointers", v, `{"type":"object","properties":{"a~/b":{"type":"integer"},"c d":{"type":"string"},"e":{"type":"integer"},"f":{"type":"string"}}}`)
	if got := simplify([]byte(`{"type":"object","properties":{"a":{"$ref":"#/$defs/l/1"}},"$defs":{"l":[{"type":"string"},{"type":"integer"}]}}`)); string(got) != `{"type":"object","properties":{"a":{"type":"integer"}}}` {
		t.Fatalf("a pointer into an array gave %s", got)
	}
}

func TestSimpleKeepsUntouchedBytes(t *testing.T) {
	v := rawView(t, `{"$schema":"https://json-schema.org/draft/2020-12/schema","required":["b","a"],"properties":{"b":{"$ref":"#/$defs/n"},"a":{"type":"string","pattern":"^\\d+é\/$","default":"x\"y"}},"type":"object","$defs":{"n":{"maximum":1.50,"minimum":-2E3,"type":"number","enum":[1.0,1e2]}},"x-tag":{"z":1,"a":[2,1]}}`)
	checkSimple(t, "a draft 2020 schema", v, `{"$schema":"https://json-schema.org/draft/2020-12/schema","required":["b","a"],"properties":{"b":{"maximum":1.50,"minimum":-2E3,"type":"number","enum":[1.0,1e2]},"a":{"type":"string","pattern":"^\\d+é\/$","default":"x\"y"}},"type":"object","x-tag":{"z":1,"a":[2,1]}}`)
	if got := simplify([]byte("{ \"type\" : \"object\" ,\n\t\"enum\" : [ {} , 1.0 ] }")); string(got) != `{"type":"object","enum":[{},1.0],"properties":{}}` {
		t.Fatalf("whitespace gave %s", got)
	}
	plain := []byte(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	if got := simplify(plain); &got[0] != &plain[0] {
		t.Fatalf("a schema that needs no change was copied to %s", got)
	}
}

func TestSimpleLeavesValuesAlone(t *testing.T) {
	const raw = `{"type":"object","properties":{"a":{"type":"object","properties":{},` +
		`"default":{"$ref":"#/nowhere","type":"object"},"enum":[{"type":"object"},{"$ref":"#/nowhere"}],` +
		`"const":{"$ref":"#/nowhere"},"examples":[{"type":"object"},{"$defs":{}}],"x-ref":{"$ref":"#/nowhere"}}}}`
	checkSimple(t, "values", rawView(t, raw), raw)
}

func TestSimpleWalksPropertyNamesAsSchemas(t *testing.T) {
	v := rawView(t, `{"type":"object","properties":{"definitions":{"$ref":"#/$defs/s"},"default":{"type":"object"},"$ref":{"$ref":"#/$defs/s"},"type":{"type":"object"},"$defs":{"type":"object","properties":{"x":{"type":"integer"}}}},"$defs":{"s":{"type":"string"}}}`)
	checkSimple(t, "keyword-like property names", v, `{"type":"object","properties":{"definitions":{"type":"string"},"default":{"type":"object","properties":{}},"$ref":{"type":"string"},"type":{"type":"object","properties":{}},"$defs":{"type":"object","properties":{"x":{"type":"integer"}}}}}`)
	v = rawView(t, `{"type":"object","patternProperties":{"^a":{"$ref":"#/$defs/o"}},"dependentSchemas":{"b":{"$ref":"#/$defs/o"}},"propertyNames":{"$ref":"#/$defs/s"},"$defs":{"o":{"type":"object"},"s":{"type":"string"}}}`)
	checkSimple(t, "maps of schemas", v, `{"type":"object","patternProperties":{"^a":{"type":"object","properties":{}}},"dependentSchemas":{"b":{"type":"object","properties":{}}},"propertyNames":{"type":"string"},"properties":{}}`)
	v = rawView(t, `{"anyOf":[{"$ref":"#/$defs/o"},{"type":"null"}],"not":{"$ref":"#/$defs/o"},"if":{"$ref":"#/$defs/o"},"then":true,"$defs":{"o":{"type":"object"}}}`)
	checkSimple(t, "applicators", v, `{"anyOf":[{"type":"object","properties":{}},{"type":"null"}],"not":{"type":"object","properties":{}},"if":{"type":"object","properties":{}},"then":true}`)
	got := simplify([]byte(`{"type":"array","items":[{"$ref":"#/definitions/o"},{"type":"object"}],"additionalItems":{"$ref":"#/definitions/o"},"dependencies":{"a":["b"],"c":{"$ref":"#/definitions/o"}},"definitions":{"o":{"type":"object"}}}`))
	if want := `{"type":"array","items":[{"type":"object","properties":{}},{"type":"object","properties":{}}],"additionalItems":{"type":"object","properties":{}},"dependencies":{"a":["b"],"c":{"type":"object","properties":{}}}}`; string(got) != want {
		t.Fatalf("draft 7 keywords gave\n got %s\nwant %s", got, want)
	}
}

func TestSimpleAddsProperties(t *testing.T) {
	v := rawView(t, `{"type":"object","properties":{"m":{"type":["object","null"],"additionalProperties":{"type":"object"}},"n":{"type":["null","string"]}}}`)
	checkSimple(t, "a map and a nullable object", v, `{"type":"object","properties":{"m":{"type":["object","null"],"additionalProperties":{"type":"object","properties":{}},"properties":{}},"n":{"type":["null","string"]}}}`)
	m, err := NewSchema[map[string]int]()
	if err != nil {
		t.Fatal(err)
	}
	checkSimple(t, "a Go map", m.View(), `{"additionalProperties":{"type":"integer"},"type":"object","properties":{}}`)
	if string(m.View().Raw) != `{"additionalProperties":{"type":"integer"},"type":"object"}` {
		t.Fatalf("a Go map's raw schema is %s", m.View().Raw)
	}
}

func TestSimpleRefusesUnresolvableRefs(t *testing.T) {
	for name, raw := range map[string]string{
		"external":        `{"type":"object","properties":{"a":{"$ref":"https://example.com/s.json"}}}`,
		"relative":        `{"type":"object","properties":{"a":{"$ref":"s.json#/a"}}}`,
		"anchor":          `{"type":"object","properties":{"a":{"$ref":"#name"}},"$defs":{"n":{"$anchor":"name","type":"string"}}}`,
		"missing":         `{"type":"object","properties":{"a":{"$ref":"#/$defs/missing"}}}`,
		"past an array":   `{"type":"object","properties":{"a":{"$ref":"#/$defs/l/2"}},"$defs":{"l":[{},{}]}}`,
		"leading zero":    `{"type":"object","properties":{"a":{"$ref":"#/$defs/l/01"}},"$defs":{"l":[{},{}]}}`,
		"bad escape":      `{"type":"object","properties":{"a":{"$ref":"#/$defs/%zz"}}}`,
		"into a scalar":   `{"type":"object","properties":{"a":{"$ref":"#/$defs/s/x"}},"$defs":{"s":{"type":"string"}}}`,
		"dynamic":         `{"$dynamicAnchor":"n","type":"object","properties":{"a":{"$dynamicRef":"#n"}}}`,
		"recursive":       `{"type":"object","properties":{"a":{"$recursiveRef":"#"}}}`,
		"to a non-schema": `{"type":"object","properties":{"a":{"$ref":"#/required"}},"required":["a"]}`,
		"in an embedded resource": `{"type":"object","properties":{"a":{"$ref":"#/$defs/A"}},"$defs":{"B":{"type":"integer"},` +
			`"A":{"$id":"urn:example:a","type":"object","properties":{"b":{"$ref":"#/$defs/B"}},"$defs":{"B":{"type":"string"}}}}}`,
		"into an embedded resource": `{"type":"object","properties":{"a":{"$ref":"#/$defs/A/properties/b"}},"$defs":{"B":{"type":"integer"},` +
			`"A":{"$id":"urn:example:a","type":"object","properties":{"b":{"$ref":"#/$defs/B"}},"$defs":{"B":{"type":"string"}}}}}`,
	} {
		if got := simplify([]byte(raw)); got != nil {
			t.Errorf("%s: simple form %s, want none", name, got)
		}
	}
	checkSimple(t, "anchor", rawView(t, `{"type":"object","properties":{"a":{"$ref":"#name"}},"$defs":{"n":{"$anchor":"name","type":"string"}}}`), "")
	plain := `{"$id":"urn:example:root","type":"object","properties":{"a":{"$id":"urn:example:a","type":"string"},"b":{"$ref":"#/$defs/B"}},"$defs":{"B":{"type":"integer"}}}`
	checkSimple(t, "an embedded resource without refs", rawView(t, plain), `{"$id":"urn:example:root","type":"object","properties":{"a":{"$id":"urn:example:a","type":"string"},"b":{"type":"integer"}}}`)
}

func TestSimpleSizeCap(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"type":"object","properties":{"a":{"$ref":"#/$defs/d0"}},"$defs":{`)
	for i := range 40 {
		fmt.Fprintf(&b, `"d%d":{"type":"array","items":[{"$ref":"#/$defs/d%d"},{"$ref":"#/$defs/d%d"}]},`, i, i+1, i+1)
	}
	b.WriteString(`"d40":{"type":"string","description":"a leaf"}}}`)
	start := time.Now()
	if got := simplify([]byte(b.String())); got != nil {
		t.Fatalf("a schema that expands to 2^40 leaves has a simple form of %d bytes", len(got))
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("the size cap took %v", d)
	}
	b.Reset()
	b.WriteString(`{"type":"object","properties":{"a":{"$ref":"#/$defs/d0"}},"$defs":{`)
	for i := range 10 {
		fmt.Fprintf(&b, `"d%d":{"type":"array","items":[{"$ref":"#/$defs/d%d"},{"$ref":"#/$defs/d%d"}]},`, i, i+1, i+1)
	}
	b.WriteString(`"d10":{"type":"string","description":"a leaf"}}}`)
	got := simplify([]byte(b.String()))
	if got == nil || !json.Valid(got) || bytes.Contains(got, []byte("$ref")) || bytes.Count(got, []byte("a leaf")) != 1024 {
		t.Fatalf("a diamond of 1024 leaves gave %.200s", got)
	}
}

func diamond(levels int, sibling, leaf string, defs ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"type":"object","properties":{"a":{%s"$ref":"#/$defs/g0"}},"$defs":{`, sibling)
	for i := range levels {
		fmt.Fprintf(&b, `"g%d":{"allOf":[{%[2]s"$ref":"#/$defs/g%[3]d"},{%[2]s"$ref":"#/$defs/g%[3]d"}]},`, i, sibling, i+1)
	}
	fmt.Fprintf(&b, `"g%d":%s`, levels, leaf)
	for _, d := range defs {
		b.WriteString("," + d)
	}
	b.WriteString("}}")
	return b.String()
}

func TestSimpleSizeCapCountsWorkOutsideOutput(t *testing.T) {
	var overridden, aliases, siblings strings.Builder
	for i := range 40 {
		fmt.Fprintf(&overridden, `"w%d":{"$ref":"#/$defs/x%[1]d","not":true},"x%[1]d":{"not":{"allOf":[{"$ref":"#/$defs/w%[2]d"},{"$ref":"#/$defs/w%[2]d"}]}},`, i, i+1)
	}
	for i := range 5000 {
		fmt.Fprintf(&aliases, `"c%d":{"$ref":"#/$defs/c%d"},`, i, i+1)
		fmt.Fprintf(&siblings, `"s%d":{"$ref":"#/$defs/s%d","k%[1]d":0},`, i, i+1)
	}
	for name, c := range map[string]struct{ raw, want string }{
		"overridden keys":     {`{"type":"object","properties":{"a":{"$ref":"#/$defs/w0"}},"$defs":{` + overridden.String() + `"w40":{"type":"string"}}}`, `{"type":"object","properties":{"a":{"not":true}}}`},
		"overridden diamonds": {diamond(40, `"allOf":[true],`, `{"type":"string"}`), `{"type":"object","properties":{"a":{"allOf":[true]}}}`},
	} {
		start := time.Now()
		if got := simplify([]byte(c.raw)); string(got) != c.want {
			t.Errorf("%s: simple form %.200s, want %s", name, got, c.want)
		}
		if d := time.Since(start); d > time.Second {
			t.Errorf("%s: took %v", name, d)
		}
	}
	for name, raw := range map[string]string{
		"a deep pointer": diamond(20, "", `{"$ref":"#/$defs/deep`+strings.Repeat("/not", 2000)+`"}`,
			`"deep":`+strings.Repeat(`{"not":`, 2000)+"true"+strings.Repeat("}", 2000)),
		"a chain of aliases":  diamond(20, "", `{"$ref":"#/$defs/c0"}`, aliases.String()+`"c5000":true`),
		"a chain of siblings": diamond(6, "", `{"$ref":"#/$defs/s0"}`, siblings.String()+`"s5000":true`),
		"padded values":       diamond(20, "", `{"enum":[1`+strings.Repeat(" ", 1<<20)+`]}`),
		"repeated keys":       diamond(20, "", `{`+strings.Repeat(`"$defs":{},`, 100000)+`"type":"string"}`),
	} {
		if !json.Valid([]byte(raw)) {
			t.Fatalf("%s: invalid JSON", name)
		}
		start := time.Now()
		if got := simplify([]byte(raw)); got != nil {
			t.Errorf("%s: simple form of %d bytes, want none", name, len(got))
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%s: the size cap took %v", name, d)
		}
	}
}
