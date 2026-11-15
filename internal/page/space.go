package page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/zeebo/xxh3"
	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type Space = *space

type space struct {
	root     *page
	ctx      context.Context
	children []*space
	path     string
	name     string
	desc     string
	tools    []*tool
	data     []*data
	island   island.Island
	inputs
}

func (s Space) AddSpace(ctx context.Context, name string, desc string) (context.Context, error) {
	return s.addSpace(ctx, name, desc, nil)
}

func (s Space) AddIsland(ctx context.Context, name string, desc string, island island.Island) (context.Context, error) {
	ctx, err := s.addSpace(ctx, name, desc, island)
	if err != nil {
		return nil, err
	}
	doors.OnClean(ctx, island.Unmount)
	return ctx, nil
}

func (s Space) addSpace(ctx context.Context, name string, desc string, island island.Island) (context.Context, error) {
	if name == "" {
		return nil, errors.New("space name is empty")
	}
	sanitized := utils.SanitizeName(name)
	if sanitized == "" {
		sanitized = fmt.Sprintf("space_%016x", xxh3.HashString(name))
		doors.Logger(ctx).Warn("neui: space name has no letter or digit, named from its hash", "name", name, "as", sanitized)
	}
	name = sanitized
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	space := &space{
		root: s.root,
		name: name,
		path: func() string {
			if s.path == "" {
				return name
			}
			return s.path + "." + name
		}(),
		desc:   desc,
		island: island,
	}
	defer doors.OnClean(ctx, func() {
		s.removeSpace(space)
	})
	defer s.root.mu.Unlock()
	index, found := s.search(name)
	ctx = context.WithValue(ctx, common.KeySpace, space)
	space.ctx = doors.DetachedContext(ctx)
	if found {
		s.children[index] = space
		return ctx, nil
	}
	s.children = slices.Insert(s.children, index, space)
	return ctx, nil
}

func (s *space) removeSpace(space *space) {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	for i, c := range s.children {
		if c != space {
			continue
		}
		s.children = slices.Delete(s.children, i, i+1)
		return
	}
}

func (s *space) search(name string) (int, bool) {
	return slices.BinarySearchFunc(s.children, name, func(s *space, name string) int {
		return strings.Compare(s.name, name)
	})
}

func (s *space) searchTool(name string) (int, bool) {
	return slices.BinarySearchFunc(s.tools, name, func(t *tool, name string) int {
		return strings.Compare(t.Name, name)
	})
}

func (s *space) find(path []string) (*space, error) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		return s, nil
	}
	child := path[0]
	index, ok := s.search(child)
	if !ok {
		return nil, fmt.Errorf("space %q under %q: %w", child, s.path, errSpaceNotFound)
	}
	return s.children[index].find(slices.Delete(path, 0, 1))
}

func (s *space) findIsland(path []string) (island.Island, string, error) {
	space, err := s.find(path)
	if err != nil {
		return nil, "", err
	}
	if space.island == nil {
		return nil, "", fmt.Errorf("space %q: %w", space.path, errIslandNotFound)
	}
	return space.island, space.path, nil
}

func (s *space) digest() [4]uint64 {
	var d [4]uint64
	for _, t := range s.tools {
		d[0] ^= t.hash
	}
	for _, in := range s.list {
		d[1] ^= in.hash
	}
	seen := map[uint64]bool{}
	for _, e := range s.data {
		if !seen[e.hash] {
			seen[e.hash] = true
			d[2] ^= e.hash
		}
	}
	for _, c := range s.children {
		d[3] ^= xxh3.HashString(c.name)
	}
	return d
}

func (s *space) digests(out map[string][4]uint64) {
	out[s.path] = s.digest()
	for _, c := range s.children {
		c.digests(out)
	}
}

func (s *space) state() state {
	out := state{Path: s.path, Desc: s.desc, Island: s.island != nil, Inputs: slices.Clone(s.list)}
	for _, t := range s.tools {
		out.Tools = append(out.Tools, *t)
	}
	seen := map[uint64]bool{}
	for _, e := range s.data {
		if !seen[e.hash] {
			seen[e.hash] = true
			out.Data = append(out.Data, *e)
		}
	}
	for _, c := range s.children {
		out.Children = append(out.Children, child{Name: c.name, Desc: c.desc, Island: c.island != nil})
	}
	return out
}

func (s *space) snapshot(by uint64) snapshot {
	return snapshot{by: by, digest: s.digest(), state: s.state()}
}

func (s *space) diff(old snapshot) *diff {
	now := s.digest()
	if now == old.digest {
		return nil
	}
	cur := s.state()
	out := &diff{}
	if now[0] != old.digest[0] {
		out.Tools = &toolsDiff{}
		for _, t := range cur.Tools {
			if !slices.ContainsFunc(old.state.Tools, func(o tool) bool { return o.hash == t.hash }) {
				out.Tools.Added = append(out.Tools.Added, t)
			}
		}
		for _, o := range old.state.Tools {
			if !slices.ContainsFunc(cur.Tools, func(t tool) bool { return t.hash == o.hash }) {
				out.Tools.Removed = append(out.Tools.Removed, o.Name)
			}
		}
	}
	if now[1] != old.digest[1] {
		out.Inputs = &inputsDiff{}
		for _, in := range cur.Inputs {
			if !slices.ContainsFunc(old.state.Inputs, func(o input) bool { return o.hash == in.hash }) {
				out.Inputs.Added = append(out.Inputs.Added, in)
			}
		}
		for _, o := range old.state.Inputs {
			if !slices.ContainsFunc(cur.Inputs, func(in input) bool { return in.hash == o.hash }) {
				out.Inputs.Removed = append(out.Inputs.Removed, o.ID)
			}
		}
	}
	if now[2] != old.digest[2] {
		out.Data = append([]data{}, cur.Data...)
	}
	if now[3] != old.digest[3] {
		out.Children = &childrenDiff{}
		for _, c := range cur.Children {
			if !slices.ContainsFunc(old.state.Children, func(o child) bool { return o.Name == c.Name }) {
				out.Children.Added = append(out.Children.Added, c)
			}
		}
		for _, o := range old.state.Children {
			if !slices.ContainsFunc(cur.Children, func(c child) bool { return c.Name == o.Name }) {
				out.Children.Removed = append(out.Children.Removed, o.Name)
			}
		}
	}
	return out
}

func (s *space) tree(depth uint) node {
	out := node{child: child{Name: s.name, Desc: s.desc, Island: s.island != nil}}
	if depth == 0 {
		out.More = len(s.children)
		return out
	}
	for i, c := range s.children {
		if i == 20 {
			out.More = len(s.children) - i
			break
		}
		out.Children = append(out.Children, c.tree(depth-1))
	}
	return out
}

func (s Space) AddData(ctx context.Context, text string, object any) error {
	d := &data{Text: text, Object: object}
	type bare data
	raw, err := json.Marshal(bare(*d))
	if err != nil {
		return err
	}
	d.raw, d.hash = raw, xxh3.Hash(raw)
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return context.Canceled
	}
	s.data = append(s.data, d)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i := slices.Index(s.data, d); i >= 0 {
			s.data = slices.Delete(s.data, i, i+1)
		}
	})
	return nil
}

func (s Space) AddTool[I any, O any](ctx context.Context, name string, desc string, f func(ctx context.Context, input I) (O, error)) error {
	inputSchema, err := schema.NewSchema[I]()
	if err != nil {
		return err
	}
	outputSchema, err := schema.NewSchema[O]()
	if err != nil {
		return err
	}
	base := doors.DetachedContext(ctx)
	t := &tool{
		Name:  utils.SanitizeName(name),
		Desc:  desc + "\nOutput schema: " + string(outputSchema.Raw()),
		Input: inputSchema.Raw(),
		call: func(ctx context.Context, arg json.RawMessage) (any, error) {
			input, err := inputSchema.Decode(arg)
			if err != nil {
				return nil, err
			}
			var out O
			settled := make(chan struct{})
			doors.OnSettle(base, func(context.Context) {
				close(settled)
			}, func(opCtx context.Context) {
				opCtx, cancel := context.WithCancel(opCtx)
				defer cancel()
				stop := context.AfterFunc(ctx, cancel)
				defer stop()
				opCtx = tools.Attach(opCtx, ctx)
				opCtx = common.AttachThread(opCtx, ctx)
				out, err = f(opCtx, input)
			})
			select {
			case <-settled:
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
			return out, err
		},
	}
	type bare tool
	raw, err := json.Marshal(bare(*t))
	if err != nil {
		return err
	}
	t.raw, t.hash = raw, xxh3.Hash(raw)
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return context.Canceled
	}
	if i, found := s.searchTool(t.Name); found {
		s.tools[i] = t
	} else {
		s.tools = slices.Insert(s.tools, i, t)
	}
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i, found := s.searchTool(t.Name); found && s.tools[i] == t {
			s.tools = slices.Delete(s.tools, i, i+1)
		}
	})
	return nil
}
