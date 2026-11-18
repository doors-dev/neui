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

// Package precheck finds what would surely fail in a part of an island
// script before it runs.
package precheck

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Env is what an island script reaches besides what its parts declare.
type Env struct {
	// Widgets are the views and controls under ui, by name.
	Widgets map[string]Widget
	// Layouts are the params schemas of the layouts under ui, by name.
	Layouts map[string]*jsonschema.Schema
	// Apps are the names of the app functions under app.
	Apps map[string]bool
	// Globals are the script's other globals: its tools and the built-ins.
	Globals map[string]bool
}

// Widget is a view or control.
type Widget struct {
	Params *jsonschema.Schema
	Patch  *jsonschema.Schema
}

// Globals is what the accepted parts of one script left in its global scope.
// The zero value is the scope before the first part.
type Globals struct {
	values  map[string]value
	written map[string]bool
	open    bool
	loose   bool
}

// Finding is one reason to refuse a part.
type Finding struct {
	Line    int
	Column  int
	Message string
}

// Error formats the finding like a syntax error of the part.
func (f Finding) Error() string {
	return fmt.Sprintf(findingFormat, f.Line, f.Column, f.Message)
}

// Check checks one part of a script against env and the globals the accepted
// parts before it left. It returns why the part must be refused, nothing if
// it may run, and the globals once it is accepted. A part it can't parse
// passes.
func Check(code string, env Env, globals Globals) ([]Finding, Globals) {
	prog, err := parser.ParseFile(nil, "", code, parser.IgnoreRegExpErrors, parser.WithDisableSourceMaps)
	if err != nil {
		return nil, Globals{written: globals.written, open: true, loose: true}
	}
	a := &analyzer{
		env:     env,
		prior:   globals,
		src:     code,
		scopes:  map[ast.Node]*scope{},
		fns:     map[ast.Node]*function{},
		written: map[string]bool{},
		typeofs: map[string]bool{},
		probed:  map[string]bool{},
		global:  &scope{vars: map[string]*binding{}},
	}
	for name, v := range globals.values {
		a.global.vars[name] = &binding{value: v}
	}
	a.declare(a.global, prog.Body)
	converged := false
	for range maxPasses {
		a.changed = false
		a.program(prog)
		if !a.changed {
			converged = true
			break
		}
	}
	a.final = true
	a.program(prog)
	out := Globals{
		values:  map[string]value{},
		written: maps.Clone(globals.written),
		open:    globals.open || a.open || a.blind || !converged,
		loose:   globals.loose || a.loose || a.blind || !converged,
	}
	if out.written == nil {
		out.written = map[string]bool{}
	}
	maps.Copy(out.written, a.written)
	for name, b := range a.global.vars {
		v := b.value
		if v.kind == funcKind {
			v.fn = &function{ret: value{kind: anyKind}, endless: v.fn != nil && v.fn.loops, loops: v.fn != nil && v.fn.loops}
		}
		out.values[name] = v
	}
	if a.blind || !converged {
		return nil, out
	}
	sort.Slice(a.findings, func(i, j int) bool {
		return cmp.Or(cmp.Compare(a.findings[i].Line, a.findings[j].Line), cmp.Compare(a.findings[i].Column, a.findings[j].Column)) < 0
	})
	return a.findings, out
}

const maxPasses = 16

type shape uint8

const (
	unset shape = iota
	anyKind
	uiKind
	appKind
	slotKind
	widgetKind
	layoutKind
	drawKind
	funcKind
)

type value struct {
	kind  shape
	names string
	fn    *function
}

type function struct {
	ret     value
	endless bool
	loops   bool
	run     bool
}

func join(a, b value) value {
	switch {
	case a.kind == unset:
		return b
	case b.kind == unset || a == b:
		return a
	case a.kind != b.kind:
		return value{kind: anyKind}
	case a.kind == widgetKind || a.kind == layoutKind:
		names := slices.Compact(slices.Sorted(slices.Values(append(strings.Split(a.names, ","), strings.Split(b.names, ",")...))))
		return value{kind: a.kind, names: strings.Join(names, ",")}
	case a.kind == funcKind:
		return value{kind: funcKind}
	}
	return a
}

type binding struct {
	value value
}

type scope struct {
	parent *scope
	vars   map[string]*binding
}

func (s *scope) lookup(name string) (*binding, *scope) {
	for c := s; c != nil; c = c.parent {
		if b, ok := c.vars[name]; ok {
			return b, c
		}
	}
	return nil, nil
}

func (s *scope) add(name string) *binding {
	if b, ok := s.vars[name]; ok {
		return b
	}
	b := &binding{}
	s.vars[name] = b
	return b
}

var objectMembers = map[string]bool{
	"constructor":          true,
	"hasOwnProperty":       true,
	"isPrototypeOf":        true,
	"propertyIsEnumerable": true,
	"toString":             true,
	"toLocaleString":       true,
	"valueOf":              true,
	"__proto__":            true,
	"__defineGetter__":     true,
	"__defineSetter__":     true,
	"__lookupGetter__":     true,
	"__lookupSetter__":     true,
}

var uiMembers = map[string]bool{"root": true, "slot": true, "draw": true, "notify": true, "end": true}

var extenders = map[string]map[string]bool{
	"Object":  {"assign": true, "defineProperty": true, "defineProperties": true, "setPrototypeOf": true},
	"Reflect": {"set": true, "defineProperty": true, "setPrototypeOf": true},
}

type analyzer struct {
	env      Env
	prior    Globals
	src      string
	global   *scope
	scopes   map[ast.Node]*scope
	fns      map[ast.Node]*function
	stack    []*function
	written  map[string]bool
	typeofs  map[string]bool
	probed   map[string]bool
	depth    int
	funcs    int
	masked   int
	open     bool
	loose    bool
	blind    bool
	changed  bool
	final    bool
	findings []Finding
}

func (a *analyzer) set(b *binding, v value) {
	if joined := join(b.value, v); joined != b.value {
		b.value = joined
		a.changed = true
	}
}

func (a *analyzer) flag(f *bool) {
	if !*f {
		*f = true
		a.changed = true
	}
}

func (a *analyzer) fnOf(n ast.Node) *function {
	if f, ok := a.fns[n]; ok {
		return f
	}
	f := &function{}
	a.fns[n] = f
	return f
}

func (a *analyzer) report(n ast.Node, format string, args ...any) {
	if !a.final || a.masked > 0 {
		return
	}
	offset := min(max(int(n.Idx0())-1, 0), len(a.src))
	start := strings.LastIndexByte(a.src[:offset], '\n') + 1
	f := Finding{
		Line:    strings.Count(a.src[:start], "\n") + 1,
		Column:  utf8.RuneCountInString(a.src[start:offset]) + 1,
		Message: fmt.Sprintf(format, args...),
	}
	if !slices.Contains(a.findings, f) {
		a.findings = append(a.findings, f)
	}
}

func (a *analyzer) text(n ast.Node) string {
	from, to := int(n.Idx0())-1, int(n.Idx1())-1
	if from < 0 || to > len(a.src) || from >= to || to-from > 60 {
		return ""
	}
	text := a.src[from:to]
	if strings.ContainsAny(text, "\r\n") {
		return ""
	}
	return text
}

func targets(t ast.Expression, out *[]*ast.Identifier) {
	switch t := t.(type) {
	case *ast.Identifier:
		*out = append(*out, t)
	case *ast.ArrayPattern:
		for _, e := range t.Elements {
			targets(e, out)
		}
		targets(t.Rest, out)
	case *ast.ObjectPattern:
		for _, p := range t.Properties {
			switch p := p.(type) {
			case *ast.PropertyShort:
				*out = append(*out, &p.Name)
			case *ast.PropertyKeyed:
				targets(p.Value, out)
			}
		}
		targets(t.Rest, out)
	case *ast.AssignExpression:
		targets(t.Left, out)
	}
}

func hoist(list []ast.Statement, vars *[]*ast.Identifier, fns *[]*ast.FunctionLiteral, top bool) {
	for _, st := range list {
		switch st := st.(type) {
		case *ast.VariableStatement:
			for _, b := range st.List {
				targets(b.Target, vars)
			}
		case *ast.FunctionDeclaration:
			if !top && st.Function.Name != nil {
				*fns = append(*fns, st.Function)
			}
		case *ast.BlockStatement:
			hoist(st.List, vars, fns, false)
		case *ast.IfStatement:
			hoist([]ast.Statement{st.Consequent, st.Alternate}, vars, fns, false)
		case *ast.WhileStatement:
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.DoWhileStatement:
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.LabelledStatement:
			hoist([]ast.Statement{st.Statement}, vars, fns, false)
		case *ast.WithStatement:
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.ForStatement:
			if init, ok := st.Initializer.(*ast.ForLoopInitializerVarDeclList); ok {
				for _, b := range init.List {
					targets(b.Target, vars)
				}
			}
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.ForInStatement:
			if into, ok := st.Into.(*ast.ForIntoVar); ok {
				targets(into.Binding.Target, vars)
			}
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.ForOfStatement:
			if into, ok := st.Into.(*ast.ForIntoVar); ok {
				targets(into.Binding.Target, vars)
			}
			hoist([]ast.Statement{st.Body}, vars, fns, false)
		case *ast.TryStatement:
			hoist(st.Body.List, vars, fns, false)
			if st.Catch != nil {
				hoist(st.Catch.Body.List, vars, fns, false)
			}
			if st.Finally != nil {
				hoist(st.Finally.List, vars, fns, false)
			}
		case *ast.SwitchStatement:
			for _, c := range st.Body {
				hoist(c.Consequent, vars, fns, false)
			}
		}
	}
}

func (a *analyzer) lexical(s *scope, list []ast.Statement) {
	for _, st := range list {
		switch st := st.(type) {
		case *ast.LexicalDeclaration:
			var ids []*ast.Identifier
			for _, b := range st.List {
				targets(b.Target, &ids)
			}
			for _, id := range ids {
				s.add(id.Name.String())
			}
		case *ast.ClassDeclaration:
			if st.Class.Name != nil {
				s.add(st.Class.Name.Name.String())
			}
		}
	}
}

func (a *analyzer) declare(s *scope, list []ast.Statement) {
	var vars []*ast.Identifier
	var fns []*ast.FunctionLiteral
	hoist(list, &vars, &fns, true)
	for _, id := range vars {
		s.add(id.Name.String())
	}
	for _, f := range fns {
		b := s.add(f.Name.Name.String())
		b.value = join(b.value, value{kind: funcKind, fn: a.fnOf(f)})
	}
	a.lexical(s, list)
	for _, st := range list {
		if fd, ok := st.(*ast.FunctionDeclaration); ok && fd.Function.Name != nil {
			s.add(fd.Function.Name.Name.String()).value = value{kind: funcKind, fn: a.fnOf(fd.Function)}
		}
	}
}

func (a *analyzer) scopeOf(n ast.Node, parent *scope, setup func(s *scope)) *scope {
	if s, ok := a.scopes[n]; ok {
		return s
	}
	s := &scope{parent: parent, vars: map[string]*binding{}}
	a.scopes[n] = s
	setup(s)
	return s
}

func (a *analyzer) program(prog *ast.Program) {
	dead := false
	for _, st := range prog.Body {
		if !dead {
			a.statement(st, a.global)
			dead = a.endless(st, a.global, true)
			continue
		}
		if fd, ok := st.(*ast.FunctionDeclaration); ok {
			a.function(fd.Function, a.global, false)
		}
	}
}

func truthy(e ast.Expression) bool {
	switch e := e.(type) {
	case *ast.BooleanLiteral:
		return e.Value
	case *ast.NumberLiteral:
		return e.Value != int64(0) && e.Value != float64(0)
	}
	return false
}

func exits(st ast.Statement, inner bool) bool {
	switch st := st.(type) {
	case *ast.ReturnStatement:
		return true
	case *ast.BranchStatement:
		return st.Token == token.BREAK && (st.Label != nil || !inner)
	case *ast.BlockStatement:
		return slices.ContainsFunc(st.List, func(st ast.Statement) bool { return exits(st, inner) })
	case *ast.IfStatement:
		return exits(st.Consequent, inner) || exits(st.Alternate, inner)
	case *ast.LabelledStatement:
		return exits(st.Statement, inner)
	case *ast.WithStatement:
		return exits(st.Body, inner)
	case *ast.WhileStatement:
		return exits(st.Body, true)
	case *ast.DoWhileStatement:
		return exits(st.Body, true)
	case *ast.ForStatement:
		return exits(st.Body, true)
	case *ast.ForInStatement:
		return exits(st.Body, true)
	case *ast.ForOfStatement:
		return exits(st.Body, true)
	case *ast.SwitchStatement:
		return slices.ContainsFunc(st.Body, func(c *ast.CaseStatement) bool {
			return slices.ContainsFunc(c.Consequent, func(st ast.Statement) bool { return exits(st, true) })
		})
	case *ast.TryStatement:
		return exits(st.Body, inner) || st.Catch != nil && exits(st.Catch.Body, inner) || st.Finally != nil && exits(st.Finally, inner)
	}
	return false
}

func (a *analyzer) endless(st ast.Statement, s *scope, calls bool) bool {
	switch st := st.(type) {
	case *ast.LabelledStatement:
		return a.endless(st.Statement, s, calls)
	case *ast.WhileStatement:
		return truthy(st.Test) && !exits(st.Body, false)
	case *ast.DoWhileStatement:
		return truthy(st.Test) && !exits(st.Body, false)
	case *ast.ForStatement:
		return (st.Test == nil || truthy(st.Test)) && !exits(st.Body, false)
	case *ast.ExpressionStatement:
		c, ok := st.Expression.(*ast.CallExpression)
		if !ok {
			return false
		}
		switch callee := c.Callee.(type) {
		case *ast.Identifier:
			b, bs := s.lookup(callee.Name.String())
			return calls && b != nil && !(bs == a.global && a.openScope()) && b.value.kind == funcKind && b.value.fn != nil && b.value.fn.endless
		case *ast.DotExpression:
			left, ok := callee.Left.(*ast.Identifier)
			if !ok || left.Name.String() != "ui" || callee.Identifier.Name.String() != "end" {
				return false
			}
			b, _ := s.lookup("ui")
			return b == nil
		}
	}
	return false
}

func (a *analyzer) openScope() bool {
	return a.open || a.prior.open
}

func (a *analyzer) running() bool {
	return len(a.stack) == 0 || a.stack[len(a.stack)-1].run
}

func (a *analyzer) block(n ast.Node, list []ast.Statement, s *scope) {
	bs := a.scopeOf(n, s, func(bs *scope) { a.lexical(bs, list) })
	for _, st := range list {
		a.statement(st, bs)
	}
}

func (a *analyzer) bindings(list []*ast.Binding, s *scope) {
	for _, b := range list {
		if b.Initializer != nil {
			a.assign(b.Target, a.expr(b.Initializer, s), s)
		}
	}
}

func (a *analyzer) statement(st ast.Statement, s *scope) {
	switch st := st.(type) {
	case *ast.ExpressionStatement:
		a.expr(st.Expression, s)
	case *ast.VariableStatement:
		a.bindings(st.List, s)
	case *ast.LexicalDeclaration:
		a.bindings(st.List, s)
	case *ast.FunctionDeclaration:
		a.function(st.Function, s, false)
	case *ast.ClassDeclaration:
		if st.Class.Name != nil {
			b, _ := s.lookup(st.Class.Name.Name.String())
			if b != nil {
				a.set(b, value{kind: anyKind})
			}
		}
		a.class(st.Class, s)
	case *ast.BlockStatement:
		a.block(st, st.List, s)
	case *ast.IfStatement:
		a.expr(st.Test, s)
		a.statement(st.Consequent, s)
		a.statement(st.Alternate, s)
	case *ast.WhileStatement:
		a.expr(st.Test, s)
		a.statement(st.Body, s)
	case *ast.DoWhileStatement:
		a.statement(st.Body, s)
		a.expr(st.Test, s)
	case *ast.ForStatement:
		fs := a.scopeOf(st, s, func(fs *scope) {
			if init, ok := st.Initializer.(*ast.ForLoopInitializerLexicalDecl); ok {
				a.lexical(fs, []ast.Statement{&init.LexicalDeclaration})
			}
		})
		switch init := st.Initializer.(type) {
		case *ast.ForLoopInitializerExpression:
			a.expr(init.Expression, fs)
		case *ast.ForLoopInitializerVarDeclList:
			a.bindings(init.List, fs)
		case *ast.ForLoopInitializerLexicalDecl:
			a.bindings(init.LexicalDeclaration.List, fs)
		}
		a.expr(st.Test, fs)
		a.expr(st.Update, fs)
		a.statement(st.Body, fs)
	case *ast.ForInStatement:
		a.forInto(st, st.Into, st.Source, st.Body, s)
	case *ast.ForOfStatement:
		a.forInto(st, st.Into, st.Source, st.Body, s)
	case *ast.ReturnStatement:
		var v value
		if st.Argument != nil {
			v = a.expr(st.Argument, s)
		}
		if len(a.stack) > 0 {
			f := a.stack[len(a.stack)-1]
			if joined := join(f.ret, v); joined != f.ret {
				f.ret = joined
				a.changed = true
			}
		}
	case *ast.ThrowStatement:
		a.expr(st.Argument, s)
	case *ast.TryStatement:
		a.block(st.Body, st.Body.List, s)
		if st.Catch != nil {
			cs := a.scopeOf(st.Catch, s, func(cs *scope) {
				var ids []*ast.Identifier
				targets(st.Catch.Parameter, &ids)
				for _, id := range ids {
					cs.add(id.Name.String()).value = value{kind: anyKind}
				}
			})
			a.block(st.Catch.Body, st.Catch.Body.List, cs)
		}
		if st.Finally != nil {
			a.block(st.Finally, st.Finally.List, s)
		}
	case *ast.SwitchStatement:
		a.expr(st.Discriminant, s)
		ss := a.scopeOf(st, s, func(ss *scope) {
			for _, c := range st.Body {
				a.lexical(ss, c.Consequent)
			}
		})
		for _, c := range st.Body {
			a.expr(c.Test, ss)
			for _, x := range c.Consequent {
				a.statement(x, ss)
			}
		}
	case *ast.LabelledStatement:
		a.statement(st.Statement, s)
	case *ast.WithStatement:
		a.expr(st.Object, s)
		a.masked++
		a.statement(st.Body, s)
		a.masked--
	}
}

func (a *analyzer) forInto(n ast.Node, into ast.ForInto, source ast.Expression, body ast.Statement, s *scope) {
	fs := a.scopeOf(n, s, func(fs *scope) {
		if d, ok := into.(*ast.ForDeclaration); ok {
			var ids []*ast.Identifier
			targets(d.Target, &ids)
			for _, id := range ids {
				fs.add(id.Name.String()).value = value{kind: anyKind}
			}
		}
	})
	a.expr(source, fs)
	switch into := into.(type) {
	case *ast.ForIntoVar:
		if into.Binding.Initializer != nil {
			a.expr(into.Binding.Initializer, fs)
		}
		a.assign(into.Binding.Target, value{kind: anyKind}, fs)
	case *ast.ForDeclaration:
		a.assign(into.Target, value{kind: anyKind}, fs)
	case *ast.ForIntoExpression:
		a.assign(into.Expression, value{kind: anyKind}, fs)
	}
	a.statement(body, fs)
}

func (a *analyzer) params(fs *scope, list *ast.ParameterList) {
	if list == nil {
		return
	}
	var ids []*ast.Identifier
	for _, b := range list.List {
		targets(b.Target, &ids)
	}
	targets(list.Rest, &ids)
	for _, id := range ids {
		fs.add(id.Name.String()).value = value{kind: anyKind}
	}
}

func (a *analyzer) defaults(fs *scope, list *ast.ParameterList) {
	if list == nil {
		return
	}
	for _, b := range list.List {
		if b.Initializer != nil {
			a.expr(b.Initializer, fs)
		}
		if _, ok := b.Target.(*ast.Identifier); !ok {
			a.assign(b.Target, value{kind: anyKind}, fs)
		}
	}
}

func (a *analyzer) function(f *ast.FunctionLiteral, s *scope, named bool) value {
	fn := a.fnOf(f)
	fs := a.scopeOf(f, s, func(fs *scope) {
		if named && f.Name != nil {
			fs.add(f.Name.Name.String()).value = value{kind: funcKind, fn: fn}
		}
		fs.add("arguments").value = value{kind: anyKind}
		a.params(fs, f.ParameterList)
		a.declare(fs, f.Body.List)
	})
	a.funcs++
	a.stack = append(a.stack, fn)
	a.defaults(fs, f.ParameterList)
	for _, st := range f.Body.List {
		a.statement(st, fs)
	}
	a.stack = a.stack[:len(a.stack)-1]
	a.funcs--
	returns := f.Async || f.Generator || slices.ContainsFunc(f.Body.List, func(st ast.Statement) bool { return exits(st, true) })
	endless := !returns && slices.ContainsFunc(f.Body.List, func(st ast.Statement) bool { return a.endless(st, fs, true) })
	loops := !returns && slices.ContainsFunc(f.Body.List, func(st ast.Statement) bool { return a.endless(st, fs, false) })
	if endless != fn.endless || loops != fn.loops {
		fn.endless, fn.loops = endless, loops
		a.changed = true
	}
	return value{kind: funcKind, fn: fn}
}

func (a *analyzer) arrow(f *ast.ArrowFunctionLiteral, s *scope) value {
	fn := a.fnOf(f)
	body, isBlock := f.Body.(*ast.BlockStatement)
	fs := a.scopeOf(f, s, func(fs *scope) {
		a.params(fs, f.ParameterList)
		if isBlock {
			a.declare(fs, body.List)
		}
	})
	a.stack = append(a.stack, fn)
	a.defaults(fs, f.ParameterList)
	if isBlock {
		for _, st := range body.List {
			a.statement(st, fs)
		}
	} else if e, ok := f.Body.(*ast.ExpressionBody); ok {
		if joined := join(fn.ret, a.expr(e.Expression, fs)); joined != fn.ret {
			fn.ret = joined
			a.changed = true
		}
	}
	a.stack = a.stack[:len(a.stack)-1]
	return value{kind: funcKind, fn: fn}
}

func (a *analyzer) class(c *ast.ClassLiteral, s *scope) {
	cs := a.scopeOf(c, s, func(cs *scope) {
		if c.Name != nil {
			cs.add(c.Name.Name.String()).value = value{kind: anyKind}
		}
	})
	if c.SuperClass != nil {
		a.expr(c.SuperClass, cs)
	}
	a.depth++
	for _, el := range c.Body {
		switch el := el.(type) {
		case *ast.MethodDefinition:
			if el.Computed {
				a.expr(el.Key, cs)
			}
			a.function(el.Body, cs, false)
		case *ast.FieldDefinition:
			if el.Computed {
				a.expr(el.Key, cs)
			}
			if el.Initializer != nil && el.Static {
				a.expr(el.Initializer, cs)
			} else if el.Initializer != nil {
				a.stack = append(a.stack, &function{})
				a.expr(el.Initializer, cs)
				a.stack = a.stack[:len(a.stack)-1]
			}
		case *ast.ClassStaticBlock:
			a.block(el.Block, el.Block.List, cs)
		}
	}
	a.depth--
}

func member(e ast.Expression) (ast.Expression, string, bool) {
	switch e := e.(type) {
	case *ast.DotExpression:
		return e.Left, e.Identifier.Name.String(), true
	case *ast.BracketExpression:
		if name, ok := e.Member.(*ast.StringLiteral); ok {
			return e.Left, name.Value.String(), true
		}
	}
	return nil, "", false
}

func (a *analyzer) isGlobal(e ast.Expression, s *scope) bool {
	switch e := e.(type) {
	case *ast.ThisExpression:
		return a.depth == 0 && a.funcs == 0
	case *ast.Identifier:
		if e.Name.String() != "globalThis" {
			return false
		}
		b, _ := s.lookup("globalThis")
		return b == nil
	}
	return false
}

func (a *analyzer) object(left ast.Expression, s *scope) value {
	if _, this := left.(*ast.ThisExpression); this || a.isGlobal(left, s) {
		return value{kind: anyKind}
	}
	return a.expr(left, s)
}

func (a *analyzer) assign(target ast.Expression, v value, s *scope) {
	switch t := target.(type) {
	case *ast.Identifier:
		name := t.Name.String()
		if b, _ := s.lookup(name); b != nil {
			a.set(b, v)
			return
		}
		if name == "ui" || name == "app" || a.env.Globals[name] {
			return
		}
		b := a.global.add(name)
		a.changed = true
		a.set(b, v)
	case *ast.DotExpression, *ast.BracketExpression:
		left, name, ok := member(t)
		if bracket, isBracket := t.(*ast.BracketExpression); isBracket {
			left = bracket.Left
			a.expr(bracket.Member, s)
		}
		if a.isGlobal(left, s) {
			a.flag(&a.open)
			return
		}
		if _, this := left.(*ast.ThisExpression); this && a.depth == 0 {
			if !ok {
				a.flag(&a.open)
				return
			}
			a.set(a.global.add(name), value{kind: anyKind})
		}
		lv := a.object(left, s)
		if !ok {
			if lv.kind == slotKind || lv.kind == widgetKind || lv.kind == layoutKind || lv.kind == drawKind {
				a.flag(&a.loose)
			}
			return
		}
		if !a.written[name] {
			a.written[name] = true
			a.changed = true
		}
	case *ast.ObjectPattern:
		if v.kind != drawKind {
			v = value{kind: anyKind}
		}
		for _, p := range t.Properties {
			switch p := p.(type) {
			case *ast.PropertyShort:
				a.access(p, "", v, p.Name.Name.String())
				if p.Initializer != nil {
					a.expr(p.Initializer, s)
				}
				a.assign(&p.Name, value{kind: anyKind}, s)
			case *ast.PropertyKeyed:
				if p.Computed {
					a.expr(p.Key, s)
				} else if key, ok := p.Key.(*ast.StringLiteral); ok {
					a.access(p, "", v, key.Value.String())
				} else if key, ok := p.Key.(*ast.Identifier); ok {
					a.access(p, "", v, key.Name.String())
				}
				a.assign(p.Value, value{kind: anyKind}, s)
			}
		}
		if t.Rest != nil {
			a.assign(t.Rest, value{kind: anyKind}, s)
		}
	case *ast.ArrayPattern:
		for _, e := range t.Elements {
			if e != nil {
				a.assign(e, value{kind: anyKind}, s)
			}
		}
		if t.Rest != nil {
			a.assign(t.Rest, value{kind: anyKind}, s)
		}
	case *ast.AssignExpression:
		a.expr(t.Right, s)
		a.assign(t.Left, value{kind: anyKind}, s)
	default:
		a.expr(target, s)
	}
}

func (a *analyzer) access(n ast.Node, left string, v value, name string) {
	if !a.final {
		return
	}
	label := name
	if text := a.text(n); text != "" && left != "" {
		label = text
	}
	switch v.kind {
	case uiKind:
		_, widget := a.env.Widgets[name]
		_, layout := a.env.Layouts[name]
		if !uiMembers[name] && !widget && !layout && !objectMembers[name] && !a.probed[name] {
			a.report(n, uiMemberMessage, label, name)
		}
		return
	case appKind:
		if !a.env.Apps[name] && !objectMembers[name] && !a.probed[name] {
			a.report(n, appMemberMessage, label, name)
		}
		return
	}
	if a.loose || a.prior.loose || a.written[name] || a.prior.written[name] || objectMembers[name] || a.probed[name] {
		return
	}
	names := fmt.Sprintf(widgetName, strings.ReplaceAll(v.names, ",", widgetNames))
	switch v.kind {
	case slotKind:
		switch name {
		case "set":
		case "update":
			a.report(n, slotUpdateMessage, label)
		default:
			a.report(n, slotMemberMessage, label, name)
		}
	case widgetKind:
		switch name {
		case "update", "id":
		case "set":
			a.report(n, widgetSetMessage, label, names)
		default:
			a.report(n, widgetMemberMessage, label, names, name)
		}
	case layoutKind:
		if name != "name" && name != "params" && name != "children" {
			a.report(n, layoutMemberMessage, label, names, name)
		}
	case drawKind:
		if name == "id" || name == "output" {
			return
		}
		field := fmt.Sprintf(outputField, left, name)
		if _, ok := n.(*ast.BracketExpression); ok {
			field = fmt.Sprintf(outputIndex, left, strconv.Quote(name))
		}
		a.report(n, drawMemberMessage, label, strings.TrimPrefix(field, "."))
	}
}

func (a *analyzer) ident(id *ast.Identifier, s *scope, guarded bool) value {
	name := id.Name.String()
	if b, bs := s.lookup(name); b != nil {
		if bs == a.global && a.openScope() {
			return value{kind: anyKind}
		}
		return b.value
	}
	switch name {
	case "ui":
		return value{kind: uiKind}
	case "app":
		return value{kind: appKind}
	case "undefined":
		return value{}
	case "eval":
		a.flag(&a.blind)
	case "globalThis", "Function":
		a.flag(&a.open)
	}
	if guarded {
		if !a.typeofs[name] {
			a.typeofs[name] = true
			a.changed = true
		}
		return value{kind: anyKind}
	}
	if a.running() && !a.env.Globals[name] && !a.typeofs[name] && !a.openScope() {
		message := undefinedMessage
		if before, after, ok := strings.Cut(name, "__"); ok && before != "" && after != "" {
			message = undefinedToolMessage
		}
		a.report(id, message, name)
	}
	return value{kind: anyKind}
}

func (a *analyzer) read(e ast.Expression, s *scope, guarded bool) value {
	left, name, ok := member(e)
	if !ok {
		if b, isBracket := e.(*ast.BracketExpression); isBracket {
			a.object(b.Left, s)
			a.expr(b.Member, s)
		}
		return value{kind: anyKind}
	}
	lv := a.object(left, s)
	switch lv.kind {
	case uiKind, appKind, slotKind, widgetKind, layoutKind:
		if !a.probed[name] {
			a.probed[name] = true
			a.changed = true
		}
	case drawKind:
		if !guarded {
			a.access(e, a.text(left), lv, name)
		}
	}
	if lv.kind == uiKind && name == "root" {
		return value{kind: slotKind}
	}
	return value{kind: anyKind}
}

func (a *analyzer) call(c *ast.CallExpression, s *scope) value {
	callee := c.Callee
	guarded := false
	if opt, ok := callee.(*ast.Optional); ok {
		callee = opt.Expression
		guarded = true
	}
	left, name, ok := member(callee)
	if !ok {
		cv := a.expr(callee, s)
		for _, arg := range c.ArgumentList {
			a.expr(arg, s)
		}
		if cv.kind == funcKind && cv.fn != nil {
			if a.running() && !cv.fn.run {
				cv.fn.run = true
				a.changed = true
			}
			return cv.fn.ret
		}
		return value{kind: anyKind}
	}
	lv := a.object(left, s)
	if !guarded {
		a.access(callee, a.text(left), lv, name)
	}
	for _, arg := range c.ArgumentList {
		a.expr(arg, s)
	}
	if id, ok := left.(*ast.Identifier); ok && extenders[id.Name.String()][name] && len(c.ArgumentList) > 0 {
		if b, _ := s.lookup(id.Name.String()); b == nil {
			switch c.ArgumentList[0].(type) {
			case *ast.ObjectLiteral, *ast.ArrayLiteral:
			default:
				a.flag(&a.loose)
			}
		}
	}
	switch lv.kind {
	case uiKind:
		switch name {
		case "slot":
			return value{kind: slotKind}
		case "draw":
			return value{kind: drawKind}
		case "notify", "end":
			return value{}
		}
		if w, ok := a.env.Widgets[name]; ok {
			a.schema(c, w.Params, fmt.Sprintf(paramsWhat, name), true)
			return value{kind: widgetKind, names: name}
		}
		if l, ok := a.env.Layouts[name]; ok {
			a.schema(c, l, fmt.Sprintf(paramsWhat, name), true)
			return value{kind: layoutKind, names: name}
		}
	case widgetKind:
		if name != "update" || a.loose || a.prior.loose || a.written[name] || a.prior.written[name] {
			break
		}
		if len(c.ArgumentList) == 0 {
			a.report(c, noPatchMessage, cmp.Or(a.text(callee), name))
		} else if w, ok := a.env.Widgets[lv.names]; ok {
			a.schema(c, w.Patch, fmt.Sprintf(patchWhat, lv.names), false)
		}
	}
	return value{kind: anyKind}
}

func (a *analyzer) expr(e ast.Expression, s *scope) value {
	switch e := e.(type) {
	case nil:
		return value{}
	case *ast.Identifier:
		return a.ident(e, s, false)
	case *ast.NullLiteral:
		return value{}
	case *ast.DotExpression, *ast.BracketExpression:
		return a.read(e, s, false)
	case *ast.Optional:
		switch e.Expression.(type) {
		case *ast.DotExpression, *ast.BracketExpression:
			return a.read(e.Expression, s, true)
		}
		return a.expr(e.Expression, s)
	case *ast.OptionalChain:
		return a.expr(e.Expression, s)
	case *ast.PrivateDotExpression:
		a.expr(e.Left, s)
	case *ast.CallExpression:
		return a.call(e, s)
	case *ast.NewExpression:
		if cv := a.expr(e.Callee, s); cv.kind == funcKind && cv.fn != nil && a.running() && !cv.fn.run {
			cv.fn.run = true
			a.changed = true
		}
		for _, arg := range e.ArgumentList {
			a.expr(arg, s)
		}
	case *ast.TemplateLiteral:
		a.expr(e.Tag, s)
		for _, x := range e.Expressions {
			a.expr(x, s)
		}
	case *ast.AssignExpression:
		if e.Operator == token.ASSIGN {
			v := a.expr(e.Right, s)
			a.assign(e.Left, v, s)
			return v
		}
		a.expr(e.Right, s)
		a.assign(e.Left, value{kind: anyKind}, s)
	case *ast.BinaryExpression:
		l := a.expr(e.Left, s)
		r := a.expr(e.Right, s)
		switch e.Operator {
		case token.LOGICAL_OR, token.LOGICAL_AND, token.COALESCE:
			return join(l, r)
		}
	case *ast.UnaryExpression:
		switch e.Operator {
		case token.TYPEOF:
			switch operand := e.Operand.(type) {
			case *ast.Identifier:
				a.ident(operand, s, true)
				return value{kind: anyKind}
			case *ast.DotExpression, *ast.BracketExpression:
				a.read(operand, s, true)
				return value{kind: anyKind}
			}
		case token.DELETE:
			switch operand := e.Operand.(type) {
			case *ast.DotExpression, *ast.BracketExpression:
				a.assign(operand, value{kind: anyKind}, s)
				return value{kind: anyKind}
			}
		case token.INCREMENT, token.DECREMENT:
			a.assign(e.Operand, value{kind: anyKind}, s)
			return value{kind: anyKind}
		}
		a.expr(e.Operand, s)
	case *ast.ConditionalExpression:
		a.expr(e.Test, s)
		return join(a.expr(e.Consequent, s), a.expr(e.Alternate, s))
	case *ast.SequenceExpression:
		var v value
		for _, x := range e.Sequence {
			v = a.expr(x, s)
		}
		return v
	case *ast.ObjectLiteral:
		for _, p := range e.Value {
			switch p := p.(type) {
			case *ast.PropertyKeyed:
				if p.Computed {
					a.expr(p.Key, s)
				}
				a.expr(p.Value, s)
			case *ast.PropertyShort:
				a.ident(&p.Name, s, false)
			case *ast.SpreadElement:
				a.expr(p.Expression, s)
			}
		}
	case *ast.ArrayLiteral:
		for _, x := range e.Value {
			a.expr(x, s)
		}
	case *ast.SpreadElement:
		a.expr(e.Expression, s)
	case *ast.FunctionLiteral:
		return a.function(e, s, true)
	case *ast.ArrowFunctionLiteral:
		return a.arrow(e, s)
	case *ast.ClassLiteral:
		a.class(e, s)
	case *ast.ThisExpression:
		if a.depth == 0 {
			a.flag(&a.open)
		}
	case *ast.YieldExpression:
		a.expr(e.Argument, s)
	case *ast.AwaitExpression:
		a.expr(e.Argument, s)
	case *ast.ObjectPattern, *ast.ArrayPattern:
		a.assign(e, value{kind: anyKind}, s)
	}
	return value{kind: anyKind}
}
