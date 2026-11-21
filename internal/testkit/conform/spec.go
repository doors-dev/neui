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

package conform

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed testdata/specs testdata/limits
var testdata embed.FS

type pinned struct {
	File       string `json:"file"`
	Source     string `json:"source"`
	Commit     string `json:"commit"`
	APIVersion string `json:"apiVersion"`
	Upstream   string `json:"upstreamSha256"`
	Sha256     string `json:"sha256"`
}

type schemas struct {
	pin      pinned
	raw      []byte
	compiler *jsonschema.Compiler
	mu       sync.Mutex
	compiled map[string]*jsonschema.Schema
}

var specCache sync.Map

func loadPins() (map[string]pinned, error) {
	data, err := testdata.ReadFile("testdata/specs/SPECS")
	if err != nil {
		return nil, err
	}
	var file struct {
		Specs []pinned `json:"specs"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("SPECS: %w", err)
	}
	pins := map[string]pinned{}
	for _, p := range file.Specs {
		pins[p.File] = p
	}
	return pins, nil
}

func loadSpec(file string, loose bool, accept []Accept) (*schemas, error) {
	key := fmt.Sprint(file, loose, accept)
	if s, ok := specCache.Load(key); ok {
		return s.(*schemas), nil
	}
	pins, err := loadPins()
	if err != nil {
		return nil, err
	}
	pin, ok := pins[file]
	if !ok {
		return nil, fmt.Errorf("%s is not pinned in SPECS", file)
	}
	gz, err := testdata.ReadFile("testdata/specs/" + file)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if loose {
		loosen(doc)
	}
	for _, a := range accept {
		segs := splitPointer(a.At)
		if len(segs) == 0 {
			return nil, fmt.Errorf("accept at %q: not a schema inside the spec", a.At)
		}
		parent := doc
		for _, s := range segs[:len(segs)-1] {
			m, ok := parent.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("accept at %q: %s is not an object", a.At, s)
			}
			parent = m[s]
		}
		m, ok := parent.(map[string]any)
		last := segs[len(segs)-1]
		if !ok || m[last] == nil {
			return nil, fmt.Errorf("accept at %q: no schema there", a.At)
		}
		m[last] = map[string]any{"anyOf": []any{m[last], a.Also}}
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseRegexpEngine(func(pattern string) (jsonschema.Regexp, error) {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return anyPattern(pattern), nil
		}
		return re, nil
	})
	if err := c.AddResource(specURL(file), doc); err != nil {
		return nil, err
	}
	s := &schemas{pin: pin, raw: raw, compiler: c, compiled: map[string]*jsonschema.Schema{}}
	actual, _ := specCache.LoadOrStore(key, s)
	return actual.(*schemas), nil
}

func loosen(v any) {
	switch n := v.(type) {
	case map[string]any:
		if one, ok := n["oneOf"]; ok {
			delete(n, "oneOf")
			n["anyOf"] = one
		}
		for _, child := range n {
			loosen(child)
		}
	case []any:
		for _, child := range n {
			loosen(child)
		}
	}
}

func specURL(file string) string {
	return "file:///" + file
}

func (s *schemas) sum() string {
	sum := sha256.Sum256(s.raw)
	return hex.EncodeToString(sum[:])
}

func (s *schemas) validate(ptr string, body []byte, private []string, defaults map[string]any) error {
	s.mu.Lock()
	sch, ok := s.compiled[ptr]
	if !ok {
		var err error
		if sch, err = s.compiler.Compile(specURL(s.pin.File) + "#" + ptr); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("compile %s: %w", ptr, err)
		}
		s.compiled[ptr] = sch
	}
	s.mu.Unlock()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("body is not JSON: %w", err)
	}
	for _, p := range private {
		v = deletePointer(v, splitPointer(p))
	}
	for p, d := range defaults {
		fillPointer(v, splitPointer(p), d)
	}
	return sch.Validate(v)
}

type anyPattern string

func (anyPattern) MatchString(string) bool { return true }

func (a anyPattern) String() string { return string(a) }

func splitPointer(ptr string) []string {
	if ptr == "" || ptr == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts
}

func deletePointer(v any, segs []string) any {
	if len(segs) == 0 {
		return v
	}
	switch n := v.(type) {
	case map[string]any:
		if len(segs) == 1 {
			delete(n, segs[0])
			return n
		}
		if child, ok := n[segs[0]]; ok {
			n[segs[0]] = deletePointer(child, segs[1:])
		}
	case []any:
		for i := range n {
			if segs[0] == "*" || segs[0] == strconv.Itoa(i) {
				if len(segs) == 1 {
					continue
				}
				n[i] = deletePointer(n[i], segs[1:])
			}
		}
	}
	return v
}

func fillPointer(v any, segs []string, value any) {
	if len(segs) == 0 {
		return
	}
	switch n := v.(type) {
	case map[string]any:
		if len(segs) == 1 {
			if _, ok := n[segs[0]]; !ok {
				n[segs[0]] = value
			}
			return
		}
		if child, ok := n[segs[0]]; ok {
			fillPointer(child, segs[1:], value)
		}
	case []any:
		for i := range n {
			if segs[0] == "*" || segs[0] == strconv.Itoa(i) {
				fillPointer(n[i], segs[1:], value)
			}
		}
	}
}

func lookup(body []byte, ptr string) (any, bool) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, false
	}
	for _, s := range splitPointer(ptr) {
		switch n := v.(type) {
		case map[string]any:
			child, ok := n[s]
			if !ok {
				return nil, false
			}
			v = child
		case []any:
			i, err := strconv.Atoi(s)
			if err != nil || i < 0 || i >= len(n) {
				return nil, false
			}
			v = n[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func sameJSON(a, b any) bool {
	ja, err := json.Marshal(a)
	if err != nil {
		return false
	}
	jb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var va, vb any
	json.Unmarshal(ja, &va)
	json.Unmarshal(jb, &vb)
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return bytes.Equal(na, nb)
}
