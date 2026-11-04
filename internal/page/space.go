package page

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type Space = *space

type space struct {
	seq      uint
	deltaSeq uint
	root     *page
	ctx      context.Context
	children []*space
	path     string
	name     string
	desc     string
	actions  tools.Module
	dataID   uint
	data     []dataEntry
	island   island.Island
	inputs
}

type dataEntry struct {
	id   uint
	data syncData
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
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	name = utils.SanitizeName(name)
	seq := s.root.adv()
	space := &space{
		root:     s.root,
		seq:      seq,
		deltaSeq: seq,
		name:     name,
		path: func() string {
			if s.path == "" {
				return name
			}
			return s.path + "." + name
		}(),
		desc:    desc,
		actions: tools.NewModule(name, ""),
		island:  island,
	}
	defer doors.OnClean(ctx, func() {
		s.removeSpace(space)
	})
	defer s.root.mu.Unlock()
	index, found := s.search(name)
	ctx = context.WithValue(ctx, common.KeySpace, space)
	space.ctx = doors.DetachedContext(ctx)
	s.deltaSeq = seq
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
		s.deltaSeq = s.root.adv()
		return
	}
}

func (s *space) bump() {
	s.seq = s.root.adv()
	s.deltaSeq = s.seq
}

func (s *space) search(name string) (int, bool) {
	return slices.BinarySearchFunc(s.children, name, func(s *space, name string) int {
		return strings.Compare(s.name, name)
	})
}

func (s *space) sync(path []string, ctx context.Context, depth uint) (*syncSpace, error) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		return s.describe(depth), nil
	}
	child := path[0]
	index, ok := s.search(child)
	if !ok {
		return nil, fmt.Errorf("space %q under %q: %w", child, s.path, errSpaceNotFound)
	}
	return s.children[index].sync(slices.Delete(path, 0, 1), ctx, depth)
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

func (s *space) findIsland(path []string) (island.Island, uint, error) {
	space, err := s.find(path)
	if err != nil {
		return nil, 0, err
	}
	if space.island == nil {
		return nil, 0, fmt.Errorf("space %q: %w", space.path, errIslandNotFound)
	}
	return space.island, space.seq, nil
}

func (s *space) collectChanged(since uint, out *[]*syncSpace) {
	if s.deltaSeq > since {
		*out = append(*out, s.describe(1))
	}
	for _, child := range s.children {
		child.collectChanged(since, out)
	}
}

func (s *space) describe(depth uint) *syncSpace {
	out := &syncSpace{
		Path: s.path,
		Desc: s.desc,
	}
	if depth == 0 {
		return out
	}
	out.Seq = s.seq
	out.Inputs = append([]input(nil), s.list...)
	for _, e := range s.data {
		out.Data = append(out.Data, e.data)
	}
	out.Island = s.island != nil
	tools := s.actions.List()
	out.Tools = make([]syncTool, len(tools))
	for i, tool := range tools {
		out.Tools[i] = syncTool{
			Name:  tool.Name(),
			Desc:  tool.Summary(),
			Input: tool.Params(),
		}
	}
	out.Children = make(map[string]*syncSpace, len(s.children))
	for _, child := range s.children {
		out.Children[child.name] = child.describe(depth - 1)
	}
	return out
}

func (s *space) getTool(path []string, seq uint, name string) (common.Tool, error) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		if s.seq != seq {
			return nil, fmt.Errorf("seq of space %q: %w", s.path, errSeqMismatch)
		}
		tool, ok := s.actions.Get(name)
		if !ok {
			return nil, fmt.Errorf("tool %q in space %q: %w", name, s.path, errToolNotFound)
		}
		return tool, nil
	}
	child := path[0]
	index, ok := s.search(child)
	if !ok {
		return nil, fmt.Errorf("space %q under %q: %w", child, s.path, errSpaceNotFound)
	}
	return s.children[index].getTool(slices.Delete(path, 0, 1), seq, name)
}

func (s Space) AddData(ctx context.Context, text string, object any) error {
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return context.Canceled
	}
	s.dataID += 1
	id := s.dataID
	s.data = append(s.data, dataEntry{
		id:   id,
		data: syncData{Text: text, Object: object},
	})
	s.bump()
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		if i, ok := slices.BinarySearchFunc(s.data, id, func(e dataEntry, id uint) int {
			return cmp.Compare(e.id, id)
		}); ok {
			s.data = slices.Delete(s.data, i, i+1)
		}
		s.bump()
	})
	return nil
}

func (s Space) AddTool[I any, O any](ctx context.Context, name string, desc string, f func(ctx context.Context, input I) (O, error)) error {
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return context.Canceled
	}
	base := doors.DetachedContext(ctx)
	id, err := s.actions.Add(name, desc, func(ctx context.Context, input I) (toolResult[O], error) {
		s.root.mu.Lock()
		before := s.root.counter
		s.root.mu.Unlock()
		var out O
		var err error
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
			return toolResult[O]{}, context.Cause(ctx)
		}
		if err != nil {
			return toolResult[O]{}, err
		}
		s.root.mu.Lock()
		seq := uint(0)
		if s.ctx.Err() == nil {
			seq = s.seq
		}
		changes, err := s.root.collectChanges(changesArg{SincePageSeq: before})
		s.root.mu.Unlock()
		return toolResult[O]{Output: out, Seq: seq, Changes: changes}, err
	})
	if err != nil {
		s.root.mu.Unlock()
		return err
	}
	s.bump()
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		s.actions.Remove(id)
		s.bump()
	})
	return nil
}
