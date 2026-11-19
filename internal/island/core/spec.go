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

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/zeebo/xxh3"
)

var identifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

var reserved = []string{"slot", "draw", "notify", "end", "root"}

func (l *library) checkName(name string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("%q: %w", name, errName)
	}
	if slices.Contains(reserved, name) {
		return fmt.Errorf("%q: %w", name, errNameReserved)
	}
	if _, ok := l.widgets[name]; ok {
		return fmt.Errorf("%q: %w", name, errNameTaken)
	}
	if _, ok := l.layouts[name]; ok {
		return fmt.Errorf("%q: %w", name, errNameTaken)
	}
	return nil
}

func writeItem(b *strings.Builder, name string, desc string) {
	b.WriteString(name)
	if desc != "" {
		b.WriteString("  ")
		b.WriteString(desc)
	}
	b.WriteString("\n")
}

func writeSchema(b *strings.Builder, label string, raw []byte) {
	fmt.Fprintf(b, "  %-6s %s\n", label, raw)
}

type specSchema struct {
	label string
	raw   []byte
	defs  map[string]json.RawMessage
}

var definitionRef = regexp.MustCompile(`"#/definitions/([^"]+)"`)

func (l Library) Build() error {
	b := strings.Builder{}
	var ends []int
	var items []string
	schemas := map[string]string{}
	defs := map[string][]byte{}
	owners := map[string]string{}
	write := func(item string, desc string, specs []specSchema) error {
		writeItem(&b, item, desc)
		sb := strings.Builder{}
		writeItem(&sb, item, "")
		for _, s := range specs {
			renames := map[string]string{}
			rename := func(raw []byte) []byte {
				for from, to := range renames {
					raw = bytes.ReplaceAll(raw, []byte(`"#/definitions/`+from+`"`), []byte(`"#/definitions/`+to+`"`))
				}
				return raw
			}
			for changed := true; changed; {
				changed = false
				for _, name := range slices.Sorted(maps.Keys(s.defs)) {
					if _, ok := renames[name]; ok {
						continue
					}
					if seen, ok := defs[name]; ok && !bytes.Equal(seen, rename(s.defs[name])) {
						renames[name] = fmt.Sprintf("%sType_%06x", name, xxh3.Hash(s.defs[name])&0xffffff)
						changed = true
					}
				}
			}
			writeSchema(&sb, s.label, rename(s.raw))
			for name, raw := range s.defs {
				if to, ok := renames[name]; ok {
					name = to
				}
				raw = rename(raw)
				seen, ok := defs[name]
				if !ok {
					defs[name] = raw
					owners[name] = item
					continue
				}
				if !bytes.Equal(seen, raw) {
					return fmt.Errorf("%q of %s and %s: %w", name, owners[name], item, errDefinitionClash)
				}
			}
		}
		ends = append(ends, b.Len())
		items = append(items, item)
		schemas[item] = sb.String()
		return nil
	}
	names := slices.Sorted(maps.Keys(l.widgets))
	for _, control := range []bool{false, true} {
		first := true
		for _, name := range names {
			f := l.widgets[name]
			if f.control() != control {
				continue
			}
			if first {
				first = false
				if control {
					b.WriteString("\nControls: c = ui.<name>(params); c.update(patch)\n")
				} else {
					b.WriteString("\nViews: v = ui.<name>(params); v.update(patch)\n")
				}
			}
			desc, specs := f.spec()
			if err := write("ui."+name, desc, specs); err != nil {
				return err
			}
		}
	}
	if len(l.layouts) > 0 {
		b.WriteString("\nLayouts: ui.<name>(params, ...children)\n")
		for _, name := range slices.Sorted(maps.Keys(l.layouts)) {
			desc, specs := l.layouts[name].spec()
			if err := write("ui."+name, desc, specs); err != nil {
				return err
			}
		}
	}
	fetches := l.tools.List()
	if len(fetches) > 0 {
		b.WriteString("\nApp functions: app.<name>(input) -> result\n")
		for _, t := range fetches {
			specs := []specSchema{{"input", t.Params(), nil}, {"result", t.Output(), nil}}
			if t, ok := t.(interface {
				ParamsSpec() ([]byte, map[string]json.RawMessage)
				OutputSpec() ([]byte, map[string]json.RawMessage)
			}); ok {
				input, inputDefs := t.ParamsSpec()
				result, resultDefs := t.OutputSpec()
				specs = []specSchema{{"input", input, inputDefs}, {"result", result, resultDefs}}
			}
			if err := write("app."+t.Name(), t.Desc(), specs); err != nil {
				return err
			}
		}
	}
	full := strings.Builder{}
	for _, item := range items {
		full.WriteString(schemas[item])
	}
	for _, name := range slices.Sorted(maps.Keys(defs)) {
		writeSchema(&full, name, defs[name])
	}
	l.spec = b.String()
	l.ends = ends
	l.items = items
	l.schemas = schemas
	l.defs = defs
	l.full = full.String()
	return nil
}

func (l Library) Spec() (string, []int, []string) {
	return l.spec, l.ends, l.items
}

// Schemas returns every item's schemas and every definition of l, for a
// revision that covers what Schema can return.
func Schemas(l Library) string {
	return l.full
}

// Schema returns the schemas of the named items, then the definitions they
// refer to, directly or through other definitions.
func Schema(l Library, items []string) (string, error) {
	b := strings.Builder{}
	picked := map[string]bool{}
	refs := map[string]bool{}
	var queue []string
	follow := func(text string) {
		for _, m := range definitionRef.FindAllStringSubmatch(text, -1) {
			if !refs[m[1]] {
				refs[m[1]] = true
				queue = append(queue, m[1])
			}
		}
	}
	for _, item := range items {
		text, ok := l.schemas[item]
		if !ok {
			return "", fmt.Errorf("%q: %w", item, errUnknownItem)
		}
		if picked[item] {
			continue
		}
		picked[item] = true
		b.WriteString(text)
		follow(text)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		follow(string(l.defs[name]))
	}
	if len(refs) > 0 {
		b.WriteString("\nDefinitions\n")
		for _, name := range slices.Sorted(maps.Keys(refs)) {
			if raw, ok := l.defs[name]; ok {
				writeSchema(&b, name, raw)
			}
		}
	}
	return b.String(), nil
}
