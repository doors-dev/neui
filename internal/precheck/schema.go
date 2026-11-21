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

package precheck

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/token"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printer = message.NewPrinter(language.English)

const unknownValue = "\x00"

type literal struct {
	nodes   map[string]ast.Node
	keys    map[string]ast.Node
	unknown [][]string
	maybe   map[string]bool
}

func path(loc []string) string {
	return strings.Join(loc, "\x00")
}

func (l *literal) guess(loc []string, present bool) (any, bool) {
	l.unknown = append(l.unknown, loc)
	if !present {
		l.maybe[path(loc)] = true
	}
	return unknownValue, true
}

func (l *literal) value(e ast.Expression, loc []string) (any, bool) {
	l.nodes[path(loc)] = e
	switch e := e.(type) {
	case *ast.StringLiteral:
		return e.Value.String(), true
	case *ast.BooleanLiteral:
		return e.Value, true
	case *ast.NullLiteral:
		return nil, true
	case *ast.NumberLiteral:
		switch v := e.Value.(type) {
		case int64:
			return json.Number(strconv.FormatInt(v, 10)), true
		case float64:
			if !math.IsInf(v, 0) && !math.IsNaN(v) {
				return json.Number(strconv.FormatFloat(v, 'g', -1, 64)), true
			}
		}
		return l.guess(loc, true)
	case *ast.TemplateLiteral:
		if e.Tag == nil && len(e.Expressions) == 0 && len(e.Elements) == 1 {
			return e.Elements[0].Parsed.String(), true
		}
		return l.guess(loc, e.Tag == nil)
	case *ast.Identifier:
		if e.Name.String() == "undefined" {
			return nil, false
		}
	case *ast.UnaryExpression:
		switch e.Operator {
		case token.VOID:
			return nil, false
		case token.MINUS:
			if n, ok := e.Operand.(*ast.NumberLiteral); ok {
				if v, ok := l.value(n, loc); ok {
					if s, ok := v.(json.Number); ok {
						l.nodes[path(loc)] = e
						return json.Number("-" + string(s)), true
					}
				}
			}
		}
		return l.guess(loc, true)
	case *ast.BinaryExpression:
		switch e.Operator {
		case token.LOGICAL_AND, token.LOGICAL_OR, token.COALESCE:
			return l.guess(loc, false)
		}
		return l.guess(loc, true)
	case *ast.NewExpression, *ast.RegExpLiteral:
		return l.guess(loc, true)
	case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
		return l.guess(loc, false)
	case *ast.ObjectLiteral:
		obj := map[string]any{}
		for _, p := range e.Value {
			var name string
			switch p := p.(type) {
			case *ast.PropertyKeyed:
				named := !p.Computed && p.Kind != ast.PropertyKindGet && p.Kind != ast.PropertyKindSet
				switch key := p.Key.(type) {
				case *ast.Identifier:
					name = key.Name.String()
				case *ast.StringLiteral:
					name = key.Value.String()
				case *ast.NumberLiteral:
					n, ok := key.Value.(int64)
					named = named && ok
					name = strconv.FormatInt(n, 10)
				default:
					named = false
				}
				if name == "__proto__" || name == "toJSON" {
					return l.guess(loc, false)
				}
				if !named {
					return l.guess(loc, true)
				}
				at := append(slices.Clip(loc), name)
				l.keys[path(at)] = p
				if v, ok := l.value(p.Value, at); ok {
					obj[name] = v
				} else {
					delete(obj, name)
				}
			case *ast.PropertyShort:
				name = p.Name.Name.String()
				if name == "toJSON" {
					return l.guess(loc, false)
				}
				at := append(slices.Clip(loc), name)
				l.keys[path(at)] = p
				l.nodes[path(at)] = p
				obj[name], _ = l.guess(at, false)
			default:
				return l.guess(loc, true)
			}
		}
		return obj, true
	case *ast.ArrayLiteral:
		arr := make([]any, 0, len(e.Value))
		for i, x := range e.Value {
			if _, ok := x.(*ast.SpreadElement); ok {
				return l.guess(loc, true)
			}
			if x == nil {
				arr = append(arr, nil)
				continue
			}
			v, _ := l.value(x, append(slices.Clip(loc), strconv.Itoa(i)))
			arr = append(arr, v)
		}
		return arr, true
	}
	return l.guess(loc, false)
}

func (l *literal) known(loc []string) bool {
	return !slices.ContainsFunc(l.unknown, func(u []string) bool {
		return len(u) <= len(loc) && slices.Equal(u, loc[:len(u)])
	})
}

func (l *literal) complete(loc []string) bool {
	return l.known(loc) && !slices.ContainsFunc(l.unknown, func(u []string) bool {
		return len(u) > len(loc) && slices.Equal(u[:len(loc)], loc)
	})
}

func (l *literal) extra(e *jsonschema.ValidationError, k *kind.AdditionalProperties) []string {
	var extra []string
	for _, name := range k.Properties {
		if !l.maybe[path(append(slices.Clip(e.InstanceLocation), name))] {
			extra = append(extra, name)
		}
	}
	return extra
}

func (l *literal) real(e *jsonschema.ValidationError) bool {
	for _, keyword := range []string{"/then", "/else", "/dependentSchemas/", "/dependencies/"} {
		if strings.Contains(e.SchemaURL+"/", keyword) && len(l.unknown) > 0 {
			return false
		}
	}
	all := func() bool {
		return len(e.Causes) > 0 && !slices.ContainsFunc(e.Causes, func(c *jsonschema.ValidationError) bool { return !l.real(c) })
	}
	switch k := e.ErrorKind.(type) {
	case *kind.AnyOf:
		return all()
	case *kind.OneOf:
		if len(k.Subschemas) > 0 {
			return l.complete(e.InstanceLocation)
		}
		return all()
	case *kind.Not:
		return l.complete(e.InstanceLocation)
	case *kind.AdditionalProperties:
		return l.known(e.InstanceLocation) && len(l.extra(e, k)) > 0
	case *kind.Required, *kind.MinItems, *kind.MaxItems, *kind.Type:
		return l.known(e.InstanceLocation)
	}
	if len(e.Causes) > 0 {
		return slices.ContainsFunc(e.Causes, l.real)
	}
	return l.complete(e.InstanceLocation)
}

func (l *literal) node(loc []string) ast.Node {
	for i := len(loc); i >= 0; i-- {
		if n := l.nodes[path(loc[:i])]; n != nil {
			return n
		}
	}
	return nil
}

func (l *literal) mismatch(e *jsonschema.ValidationError, loc []string) bool {
	switch e.ErrorKind.(type) {
	case *kind.Type, *kind.Const, *kind.Enum:
		if (slices.Equal(e.InstanceLocation, loc) || slices.Equal(e.InstanceLocation, append(slices.Clip(loc), "type"))) && l.real(e) {
			return true
		}
	}
	return slices.ContainsFunc(e.Causes, func(c *jsonschema.ValidationError) bool { return l.mismatch(c, loc) })
}

func pointer(loc []string) string {
	if len(loc) == 0 {
		return ""
	}
	escaped := make([]string, len(loc))
	for i, token := range loc {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
	}
	return fmt.Sprintf(atPointer, strings.Join(escaped, "/"))
}

func (a *analyzer) explain(l *literal, e *jsonschema.ValidationError, what string, fallback ast.Node) {
	loc := e.InstanceLocation
	where := func(n ast.Node) ast.Node {
		if n == nil {
			return fallback
		}
		return n
	}
	switch k := e.ErrorKind.(type) {
	case *kind.AnyOf, *kind.OneOf:
		if one, ok := k.(*kind.OneOf); !ok || len(one.Subschemas) == 0 {
			var picked []*jsonschema.ValidationError
			for _, c := range e.Causes {
				if !l.mismatch(c, loc) {
					picked = append(picked, c)
				}
			}
			if len(picked) == 1 && l.real(picked[0]) {
				a.explain(l, picked[0], what, fallback)
				return
			}
			if len(picked) == 0 {
				typeLoc := append(slices.Clip(loc), "type")
				values := &kind.Enum{}
				types := &kind.Type{}
				var collect func(e *jsonschema.ValidationError)
				collect = func(e *jsonschema.ValidationError) {
					var want []any
					switch k := e.ErrorKind.(type) {
					case *kind.Const:
						want = []any{k.Want}
					case *kind.Enum:
						want = k.Want
					case *kind.Type:
						if slices.Equal(e.InstanceLocation, loc) {
							types.Got = k.Got
							for _, t := range k.Want {
								if !slices.Contains(types.Want, t) {
									types.Want = append(types.Want, t)
								}
							}
						}
					}
					if slices.Equal(e.InstanceLocation, typeLoc) {
						for _, v := range want {
							if !slices.ContainsFunc(values.Want, func(w any) bool { return reflect.DeepEqual(w, v) }) {
								values.Want = append(values.Want, v)
							}
						}
					}
					for _, c := range e.Causes {
						collect(c)
					}
				}
				for _, c := range e.Causes {
					collect(c)
				}
				if len(values.Want) > 0 {
					a.report(where(l.node(typeLoc)), schemaMessage, what+pointer(typeLoc), values.LocalizedString(printer))
					return
				}
				if len(types.Want) > 0 {
					a.report(where(l.node(loc)), schemaMessage, what+pointer(loc), types.LocalizedString(printer))
					return
				}
			}
			a.report(where(l.node(loc)), schemaMessage, what+pointer(loc), shapesMessage)
			return
		}
	case *kind.AdditionalProperties:
		extra := l.extra(e, k)
		a.report(where(l.keys[path(append(slices.Clip(loc), extra[0]))]), schemaMessage, what+pointer(loc), (&kind.AdditionalProperties{Properties: extra}).LocalizedString(printer))
		return
	}
	if len(e.Causes) > 0 {
		for _, c := range e.Causes {
			if l.real(c) {
				a.explain(l, c, what, fallback)
			}
		}
		return
	}
	a.report(where(l.node(loc)), schemaMessage, what+pointer(loc), e.ErrorKind.LocalizedString(printer))
}

func (a *analyzer) schema(c *ast.CallExpression, sch *jsonschema.Schema, what string, params bool) {
	if !a.final || sch == nil {
		return
	}
	l := &literal{nodes: map[string]ast.Node{}, keys: map[string]ast.Node{}, maybe: map[string]bool{}}
	var instance any = map[string]any{}
	var fallback ast.Node = c
	if len(c.ArgumentList) > 0 {
		fallback = c.ArgumentList[0]
		v, ok := l.value(c.ArgumentList[0], nil)
		if ok && (v != nil || !params) {
			instance = v
		}
		if !ok && !params {
			return
		}
	}
	if !l.known(nil) {
		return
	}
	err := sch.Validate(instance)
	e, ok := err.(*jsonschema.ValidationError)
	if !ok || !l.real(e) {
		return
	}
	a.explain(l, e, what, fallback)
}
