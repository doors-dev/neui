package page

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

type Rights string

const (
	RW Rights = "RW"
	R  Rights = "R"
	W  Rights = "W"
)

type input struct {
	ctx       context.Context `json:"-"`
	ID        uint            `json:"id" description:"pass to read, write, click and focus"`
	Kind      kind            `json:"kind" description:"button, link, input or radio; button and link take click, input and radio take write; a radio entry stands for its whole same-name group"`
	elementID string          `json:"-"`
	members   int             `json:"-"`
	Desc      string          `json:"desc"`
	Type      string          `json:"type" description:"html flavor of the element"`
	Name      string          `json:"name,omitempty" description:"html name attribute"`
	Rights    Rights          `json:"rights" description:"RW allows every call, R only read and focus, W only click, write and focus"`
	Disabled  bool            `json:"disabled,omitempty" description:"present and true when the element is disabled"`
}

type kind string

const (
	kindButton kind = "button"
	kindLink   kind = "link"
	kindInput  kind = "input"
	kindRadio  kind = "radio"
)

type inputs struct {
	counter uint
	list    []input
}

func (s *space) elementID() (uint, string) {
	s.counter += 1
	return s.counter, fmt.Sprintf("%s-%d", s.path, s.counter)
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
		if s.list[i].Kind == kindRadio && s.list[i].Name == name {
			group = i
			break
		}
	}
	var id uint
	var elementID string
	first := group < 0
	if first {
		id, elementID = s.elementID()
		s.bump()
		s.list = append(s.list, input{
			ctx:       doors.DetachedContext(ctx),
			ID:        id,
			Kind:      kindRadio,
			elementID: elementID,
			members:   1,
			Desc:      desc,
			Type:      "radio",
			Name:      name,
			Rights:    permissios,
		})
	} else {
		id = s.list[group].ID
		elementID = s.list[group].elementID
		s.list[group].members += 1
	}
	idAttr.Set(elementID)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i, ok := searchInput(s.list, id); ok {
			s.list[i].members -= 1
			if s.list[i].members == 0 {
				s.list = slices.Delete(s.list, i, i+1)
				s.bump()
			}
		}
	})
	if first {
		return radioGroup(elementID), nil
	}
	return gox.Elem(func(gox.Cursor) error { return nil }), nil
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
	s.bump()
	if t, ok := attrs.Get("type").Value().(string); ok {
		typ = t
	}
	name, _ := attrs.Get("name").Value().(string)
	s.list = append(s.list, input{
		ctx:       doors.DetachedContext(ctx),
		ID:        id,
		Kind:      kind,
		elementID: elementID,
		Desc:      desc,
		Rights:    permissios,
		Type:      typ,
		Name:      name,
		Disabled:  attrs.Get("disabled").IsSet(),
	})
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i, ok := searchInput(s.list, id); ok {
			s.list = slices.Delete(s.list, i, i+1)
			s.bump()
		}
	})
	return script(elementID), nil
}

func (s *space) getInput(path []string, seq uint, id uint) (*space, input, error) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		if s.seq != seq {
			return nil, input{}, fmt.Errorf("seq of space %q: %w", s.path, errSeqMismatch)
		}
		i, ok := searchInput(s.list, id)
		if !ok {
			return nil, input{}, fmt.Errorf("input %d in space %q: %w", id, s.path, errInputNotFound)
		}
		return s, s.list[i], nil
	}
	child := path[0]
	index, ok := s.search(child)
	if !ok {
		return nil, input{}, fmt.Errorf("space %q under %q: %w", child, s.path, errSpaceNotFound)
	}
	return s.children[index].getInput(slices.Delete(path, 0, 1), seq, id)
}
