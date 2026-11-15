package page

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

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
	ID        uint                     `json:"id" description:"pass to read, write, click and focus"`
	Kind      kind                     `json:"kind" description:"button, link, input or radio; button and link take click, input and radio take write; a radio entry stands for its whole same-name group"`
	Type      string                   `json:"type" description:"html flavor of the element"`
	Desc      string                   `json:"desc"`
	Rights    Rights                   `json:"rights" description:"RW allows every call, R only read and focus, W only click, write and focus"`
	ctx       context.Context          `json:"-"`
	elementID string                   `json:"-"`
	members   map[uint]context.Context `json:"-"`
	name      string                   `json:"-"`
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

func (s *space) elementID() (uint, string) {
	s.root.counter += 1
	return s.root.counter, fmt.Sprintf("%s-%d", s.path, s.root.counter)
}

func searchInput(list []input, id uint) (int, bool) {
	return slices.BinarySearchFunc(list, id, func(in input, id uint) int {
		return cmp.Compare(in.ID, id)
	})
}

func (s Space) Button(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, attrs, "button", kindButton, element)
}

func (s Space) Link(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, attrs, "link", kindLink, element)
}

func (s Space) Input(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, attrs, "text", kindInput, element)
}

func (s Space) Textarea(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, attrs, "textarea", kindInput, element)
}

func (s Space) Select(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	return s.element(ctx, permissios, desc, attrs, "select", kindInput, element)
}

func (s Space) Radio(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs) (gox.Elem, error) {
	idAttr := attrs.Get("data-kid")
	if idAttr.IsSet() {
		return nil, errors.New("element is already used")
	}
	s.root.mu.Lock()
	if s.ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	name, _ := attrs.Get("name").Value().(string)
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
	var id uint
	var elementID string
	if group < 0 {
		id, elementID = s.elementID()
		in := input{
			ctx:       memberCtx,
			ID:        id,
			Kind:      kindRadio,
			elementID: elementID,
			members:   map[uint]context.Context{member: memberCtx},
			Desc:      desc,
			Type:      "radio",
			name:      name,
			Rights:    permissios,
		}
		type bare input
		raw, err := json.Marshal(bare(in))
		if err != nil {
			s.root.mu.Unlock()
			return nil, err
		}
		in.raw, in.hash = raw, xxh3.Hash(raw)
		s.list = append(s.list, in)
	} else {
		id = s.list[group].ID
		elementID = s.list[group].elementID
		s.list[group].members[member] = memberCtx
	}
	idAttr.Set(elementID)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		i, ok := searchInput(s.list, id)
		if !ok {
			return
		}
		delete(s.list[i].members, member)
		if len(s.list[i].members) == 0 {
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

func (s Space) element(ctx context.Context, permissios Rights, desc string, attrs gox.Attrs, typ string, kind kind, script func(string) gox.Elem) (gox.Elem, error) {
	idAttr := attrs.Get("data-kid")
	if idAttr.IsSet() {
		return nil, errors.New("element is already used")
	}
	s.root.mu.Lock()
	if s.ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	id, elementID := s.elementID()
	idAttr.Set(elementID)
	if t, ok := attrs.Get("type").Value().(string); ok {
		typ = t
	}
	in := input{
		ctx:       doors.DetachedContext(ctx),
		ID:        id,
		Kind:      kind,
		elementID: elementID,
		Desc:      desc,
		Rights:    permissios,
		Type:      typ,
	}
	type bare input
	raw, err := json.Marshal(bare(in))
	if err != nil {
		s.root.mu.Unlock()
		return nil, err
	}
	in.raw, in.hash = raw, xxh3.Hash(raw)
	s.list = append(s.list, in)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i, ok := searchInput(s.list, id); ok {
			s.list = slices.Delete(s.list, i, i+1)
		}
	})
	return script(elementID), nil
}

func (s *space) getInput(id uint, action string) (input, error) {
	i, ok := searchInput(s.list, id)
	if !ok {
		return input{}, fmt.Errorf("input %d in space %q: %w", id, s.path, errInputNotFound)
	}
	in := s.list[i]
	switch action {
	case "read":
		if in.Rights == W {
			return input{}, fmt.Errorf("%q on input %d with rights %s: %w", action, id, in.Rights, errRights)
		}
	case "click", "input":
		if in.Rights == R {
			return input{}, fmt.Errorf("%q on input %d with rights %s: %w", action, id, in.Rights, errRights)
		}
	}
	return in, nil
}
