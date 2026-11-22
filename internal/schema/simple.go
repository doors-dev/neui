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
	"net/url"
	"strconv"
	"strings"
)

const simpleLimit = 1 << 20

type jsonNode struct {
	raw     []byte
	parent  *jsonNode
	pre     int
	end     int
	members []jsonMember
	items   []*jsonNode
	index   map[string]*jsonNode
}

type jsonMember struct {
	raw   []byte
	name  string
	value *jsonNode
}

type jsonParser struct {
	data      []byte
	pos       int
	count     int
	resources []*jsonNode
}

func (p *jsonParser) value(parent *jsonNode) *jsonNode {
	start := p.pos
	n := &jsonNode{parent: parent, pre: p.count}
	p.count++
	switch p.data[p.pos] {
	case '{':
		p.pos++
		for p.data[p.pos] != '}' {
			keyStart := p.pos
			p.str()
			key := p.data[keyStart:p.pos]
			p.pos++
			m := jsonMember{raw: key, name: decodeString(key), value: p.value(n)}
			n.members = append(n.members, m)
			if parent != nil && m.name == "$id" && m.value.raw[0] == '"' {
				p.resources = append(p.resources, n)
			}
			if p.data[p.pos] == ',' {
				p.pos++
			}
		}
		p.pos++
	case '[':
		p.pos++
		for p.data[p.pos] != ']' {
			n.items = append(n.items, p.value(n))
			if p.data[p.pos] == ',' {
				p.pos++
			}
		}
		p.pos++
	case '"':
		p.str()
	default:
		for p.pos < len(p.data) && strings.IndexByte(",]}", p.data[p.pos]) < 0 {
			p.pos++
		}
	}
	n.raw = p.data[start:p.pos]
	n.end = p.count
	return n
}

func (p *jsonParser) str() {
	p.pos++
	for p.data[p.pos] != '"' {
		if p.data[p.pos] == '\\' {
			p.pos++
		}
		p.pos++
	}
	p.pos++
}

func decodeString(raw []byte) string {
	if !bytes.ContainsRune(raw, '\\') {
		return string(raw[1 : len(raw)-1])
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

type simplifier struct {
	root      *jsonNode
	resources []*jsonNode
	out       bytes.Buffer
	spent     int
	targets   map[*jsonNode]*jsonNode
	expanding map[*jsonNode]int
}

type schemaShape struct {
	typed  bool
	object bool
	props  bool
}

func simplify(raw []byte) []byte {
	var compact bytes.Buffer
	if json.Compact(&compact, raw) != nil {
		return nil
	}
	data := raw
	if !bytes.Equal(compact.Bytes(), raw) {
		data = compact.Bytes()
	}
	p := jsonParser{data: data}
	root := p.value(nil)
	s := &simplifier{root: root, resources: p.resources, targets: map[*jsonNode]*jsonNode{}, expanding: map[*jsonNode]int{}}
	if !s.schema(s.root) {
		return nil
	}
	if bytes.Equal(s.out.Bytes(), raw) {
		return raw
	}
	return bytes.Clone(s.out.Bytes())
}

func (s *simplifier) verbatim(n *jsonNode) bool {
	s.out.Write(n.raw)
	return s.out.Len()+s.spent <= simpleLimit
}

func (s *simplifier) comma() {
	if b := s.out.Bytes(); b[len(b)-1] != '{' && b[len(b)-1] != '[' {
		s.out.WriteByte(',')
	}
}

func (s *simplifier) schema(n *jsonNode) bool {
	if n.raw[0] != '{' {
		return s.verbatim(n)
	}
	start := s.out.Len()
	s.out.WriteByte('{')
	_, never, ok := s.members(n, nil)
	if !ok {
		return false
	}
	if never {
		s.spent += s.out.Len() - start
		s.out.Truncate(start)
		s.out.WriteString("false")
	} else {
		s.out.WriteByte('}')
	}
	return s.out.Len()+s.spent <= simpleLimit
}

func (s *simplifier) members(n *jsonNode, exclude map[string]int) (schemaShape, bool, bool) {
	var shape schemaShape
	var ref *jsonNode
	var siblings []string
	for _, m := range n.members {
		if exclude[m.name] > 0 {
			s.spent += len(m.raw)
			continue
		}
		switch m.name {
		case "$ref":
			if m.value.raw[0] == '"' {
				ref = m.value
				s.spent += len(m.raw) + len(m.value.raw)
				continue
			}
		case "$dynamicRef", "$recursiveRef":
			return shape, false, false
		case "definitions", "$defs":
			s.spent += len(m.raw)
			continue
		case "type":
			shape.typed, shape.object = true, objectType(m.value)
		case "properties":
			shape.props = true
		}
		siblings = append(siblings, m.name)
		s.comma()
		s.out.Write(m.raw)
		s.out.WriteByte(':')
		if !s.keyword(m) {
			return shape, false, false
		}
	}
	if s.out.Len()+s.spent > simpleLimit {
		return shape, false, false
	}
	if ref != nil {
		for _, r := range s.resources {
			if r.pre <= n.pre && n.pre < r.end {
				return shape, false, false
			}
		}
		target, seen := s.targets[ref]
		if !seen {
			target = s.resolve(decodeString(ref.raw))
			s.targets[ref] = target
		}
		if target == nil || s.expanding[target] > 0 || target.pre <= n.pre && n.pre < target.end {
			return shape, false, false
		}
		for p := target; p != nil; p = p.parent {
			s.expanding[p]++
		}
		never := false
		switch {
		case bytes.Equal(target.raw, []byte("true")):
		case bytes.Equal(target.raw, []byte("false")):
			if len(siblings) > 0 {
				return shape, false, false
			}
			never = true
		case target.raw[0] == '{':
			if exclude == nil && len(siblings) > 0 {
				exclude = map[string]int{}
			}
			for _, name := range siblings {
				exclude[name]++
			}
			targetShape, targetNever, ok := s.members(target, exclude)
			if !ok || targetNever && len(siblings) > 0 {
				return shape, false, false
			}
			for _, name := range siblings {
				exclude[name]--
			}
			never = targetNever
			if !shape.typed {
				shape.typed, shape.object = targetShape.typed, targetShape.object
			}
			shape.props = shape.props || targetShape.props
		default:
			return shape, false, false
		}
		for p := target; p != nil; p = p.parent {
			s.expanding[p]--
		}
		if never {
			return shape, true, true
		}
	}
	if shape.object && !shape.props {
		shape.props = true
		if exclude["properties"] == 0 {
			s.comma()
			s.out.WriteString(`"properties":{}`)
		}
	}
	return shape, false, s.out.Len()+s.spent <= simpleLimit
}

func (s *simplifier) keyword(m jsonMember) bool {
	v := m.value
	switch m.name {
	case "properties", "patternProperties", "dependentSchemas", "dependencies":
		if v.raw[0] != '{' {
			return s.verbatim(v)
		}
		s.out.WriteByte('{')
		for _, p := range v.members {
			s.comma()
			s.out.Write(p.raw)
			s.out.WriteByte(':')
			if m.name == "dependencies" && p.value.raw[0] == '[' {
				if !s.verbatim(p.value) {
					return false
				}
			} else if !s.schema(p.value) {
				return false
			}
		}
		s.out.WriteByte('}')
	case "items", "prefixItems", "allOf", "anyOf", "oneOf":
		if v.raw[0] != '[' {
			return s.schema(v)
		}
		s.out.WriteByte('[')
		for _, item := range v.items {
			s.comma()
			if !s.schema(item) {
				return false
			}
		}
		s.out.WriteByte(']')
	case "additionalProperties", "additionalItems", "unevaluatedProperties", "unevaluatedItems",
		"contains", "propertyNames", "not", "if", "then", "else", "contentSchema":
		return s.schema(v)
	default:
		return s.verbatim(v)
	}
	return s.out.Len()+s.spent <= simpleLimit
}

func (s *simplifier) resolve(ref string) *jsonNode {
	if ref == "#" {
		return s.root
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	pointer, err := url.PathUnescape(ref[1:])
	if err != nil || !strings.HasPrefix(pointer, "/") {
		return nil
	}
	n := s.root
	for _, token := range strings.Split(pointer[1:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch n.raw[0] {
		case '{':
			if n.index == nil {
				n.index = make(map[string]*jsonNode, len(n.members))
				for _, m := range n.members {
					n.index[m.name] = m.value
				}
			}
			n = n.index[token]
		case '[':
			j, err := strconv.Atoi(token)
			if err != nil || j < 0 || j >= len(n.items) || strconv.Itoa(j) != token {
				return nil
			}
			n = n.items[j]
		default:
			return nil
		}
		if n == nil {
			return nil
		}
	}
	return n
}

func objectType(n *jsonNode) bool {
	switch n.raw[0] {
	case '"':
		return decodeString(n.raw) == "object"
	case '[':
		for _, item := range n.items {
			if item.raw[0] == '"' && decodeString(item.raw) == "object" {
				return true
			}
		}
	}
	return false
}
