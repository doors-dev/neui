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

package page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/zeebo/xxh3"
)

type Rights string

const (
	RW Rights = "RW"
	R  Rights = "R"
	W  Rights = "W"
)

type input struct {
	ID        string                   `json:"id"`
	Kind      kind                     `json:"kind"`
	Type      string                   `json:"type"`
	Desc      string                   `json:"desc"`
	Rights    Rights                   `json:"rights"`
	ctx       context.Context          `json:"-"`
	elementID string                   `json:"-"`
	members   map[uint]context.Context `json:"-"`
	name      string                   `json:"-"`
	highlight bool                     `json:"-"`
	raw       json.RawMessage          `json:"-"`
	hash      uint64                   `json:"-"`
}

func (in input) MarshalJSON() ([]byte, error) {
	return in.raw, nil
}

type kind string

const (
	kindButton kind = "button"
	kindLink   kind = "link"
	kindInput  kind = "input"
	kindRadio  kind = "radio"
)

type inputs struct {
	member uint
	list   []input
}

func (s *space) elementID() string {
	s.root.counter += 1
	return fmt.Sprintf("%s-%d", s.path, s.root.counter)
}

func (s Space) Button(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, highlight, attrs, "button", kindButton, element)
}

func (s Space) Link(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, highlight, attrs, "link", kindLink, element)
}

func (s Space) Input(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, highlight, attrs, "text", kindInput, element)
}

func (s Space) Textarea(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, highlight, attrs, "textarea", kindInput, element)
}

func (s Space) Select(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, highlight, attrs, "select", kindInput, element)
}

func (s Space) Radio(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs) (gox.Elem, error) {
	idAttr := attrs.Get("data-neid")
	if idAttr.IsSet() {
		return nil, errors.New("element is already used")
	}
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	name, _ := attrs.Get("name").Value().(string)
	value := ""
	if name == "" {
		value, _ = attrs.Get("value").Value().(string)
	}
	group := -1
	for i := range s.list {
		if name != "" && s.list[i].Kind == kindRadio && s.list[i].name == name {
			group = i
			break
		}
	}
	s.member += 1
	member := s.member
	memberCtx := doors.DetachedContext(ctx)
	var elementID string
	if group < 0 {
		elementID = s.elementID()
		in := input{
			ctx:       memberCtx,
			ID:        fmt.Sprintf("%08x", uint32(xxh3.HashString(string(kindRadio)+"\x00radio\x00"+desc+"\x00"+name+"\x00"+value))),
			Kind:      kindRadio,
			elementID: elementID,
			members:   map[uint]context.Context{member: memberCtx},
			Desc:      desc,
			Type:      "radio",
			name:      name,
			Rights:    permissios,
			highlight: highlight,
		}
		type bare input
		raw, err := json.Marshal(bare(in))
		if err != nil {
			s.root.mu.Unlock()
			return nil, err
		}
		in.raw, in.hash = raw, xxh3.Hash(raw)
		s.list = append(s.list, in)
		s.count(1, in.hash, 1)
	} else {
		elementID = s.list[group].elementID
		s.list[group].members[member] = memberCtx
	}
	idAttr.Set(elementID)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		i := slices.IndexFunc(s.list, func(in input) bool { return in.elementID == elementID })
		if i < 0 {
			return
		}
		delete(s.list[i].members, member)
		if len(s.list[i].members) == 0 {
			s.count(1, s.list[i].hash, -1)
			s.list = slices.Delete(s.list, i, i+1)
			return
		}
		for _, memberCtx := range s.list[i].members {
			s.list[i].ctx = memberCtx
			break
		}
	})
	return radioGroup(elementID), nil
}

func (s Space) element(ctx context.Context, permissios Rights, desc string, highlight bool, attrs gox.Attrs, typ string, kind kind, script func(string) gox.Elem) (gox.Elem, error) {
	idAttr := attrs.Get("data-neid")
	if idAttr.IsSet() {
		return nil, errors.New("element is already used")
	}
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	elementID := s.elementID()
	idAttr.Set(elementID)
	if t, ok := attrs.Get("type").Value().(string); ok {
		typ = t
	}
	name, _ := attrs.Get("name").Value().(string)
	value := ""
	switch {
	case kind == kindButton || strings.EqualFold(typ, "checkbox"):
		value, _ = attrs.Get("value").Value().(string)
	case kind == kindLink:
		value, _ = attrs.Get("href").Value().(string)
	}
	in := input{
		ctx:       doors.DetachedContext(ctx),
		ID:        fmt.Sprintf("%08x", uint32(xxh3.HashString(string(kind)+"\x00"+typ+"\x00"+desc+"\x00"+name+"\x00"+value))),
		Kind:      kind,
		elementID: elementID,
		Desc:      desc,
		Rights:    permissios,
		Type:      typ,
		highlight: highlight,
	}
	type bare input
	raw, err := json.Marshal(bare(in))
	if err != nil {
		s.root.mu.Unlock()
		return nil, err
	}
	in.raw, in.hash = raw, xxh3.Hash(raw)
	s.list = append(s.list, in)
	s.count(1, in.hash, 1)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if i := slices.IndexFunc(s.list, func(in input) bool { return in.elementID == elementID }); i >= 0 {
			s.count(1, s.list[i].hash, -1)
			s.list = slices.Delete(s.list, i, i+1)
		}
	})
	return script(elementID), nil
}

func (s *space) getInput(id string, action string) (input, error) {
	var in input
	n := 0
	for _, c := range s.list {
		if c.ID == id && c.ctx.Err() == nil {
			in = c
			n++
		}
	}
	switch {
	case n == 0:
		return input{}, fmt.Errorf("input %q in space %q: %w", id, s.path, errInputNotFound)
	case n > 1:
		doors.Logger(s.root.ctx).Error("neui: inputs in a space cannot be told apart, a call on them was refused", "space", s.path, "id", id, "desc", in.Desc)
		return input{}, fmt.Errorf("input %q in space %q: %w", id, s.path, errRefusedDuplicate)
	}
	switch action {
	case "read":
		if in.Rights == W {
			return input{}, fmt.Errorf("%q on input %q with rights %s: %w", action, id, in.Rights, errRights)
		}
	case "click", "write":
		if in.Rights == R {
			return input{}, fmt.Errorf("%q on input %q with rights %s: %w", action, id, in.Rights, errRights)
		}
	}
	return in, nil
}
