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
	"cmp"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/zeebo/xxh3"
	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type Space = *space

type space struct {
	root     *page
	parts    []*part
	children []*space
	path     string
	name     string
	desc     string
	tools    []*tool
	data     []*data
	island   island.Island
	within   island.Island
	digest   [4]uint64
	counts   [4]map[uint64]int
	inputs
}

type part struct {
	desc string
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
	name = utils.SanitizeName(name)
	path := name
	if s.path != "" {
		path = s.path + "." + name
	}
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return nil, context.Canceled
	}
	index, found := s.search(name)
	if !found || island != nil || s.children[index].island != nil {
		fresh := &space{
			root:   s.root,
			name:   name,
			path:   path,
			island: island,
			within: cmp.Or(island, s.within),
		}
		if found {
			if (island == nil) != (s.children[index].island == nil) {
				doors.Logger(ctx).Warn("neui: a space and an island share a name, the later render replaces the other", "space", path)
			}
			s.children[index] = fresh
		} else {
			s.children = slices.Insert(s.children, index, fresh)
		}
		s.rehash(3)
	}
	space := s.children[index]
	added := &part{desc: desc}
	space.parts = append(space.parts, added)
	if desc != "" {
		space.desc = desc
	}
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		space.parts = slices.DeleteFunc(space.parts, func(p *part) bool { return p == added })
		space.desc = ""
		for _, p := range space.parts {
			space.desc = cmp.Or(p.desc, space.desc)
		}
		if len(space.parts) > 0 {
			return
		}
		if i, found := s.search(name); found && s.children[i] == space {
			s.children = slices.Delete(s.children, i, i+1)
			s.rehash(3)
		}
	})
	return context.WithValue(ctx, common.KeySpace, space), nil
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

func (s *space) find(path string) (*space, error) {
	if path == "" {
		return s, nil
	}
	for _, name := range strings.Split(strings.ReplaceAll(path, "/", "."), ".") {
		index, ok := s.search(name)
		if !ok {
			if s.path == "" {
				return nil, fmt.Errorf("no space %q at the page root: %w", name, errSpaceNotFound)
			}
			return nil, fmt.Errorf("no space %q under %q: %w", name, s.path, errSpaceNotFound)
		}
		s = s.children[index]
	}
	return s, nil
}

func (s *space) findIsland(path string) (island.Island, string, error) {
	space, err := s.find(path)
	if err != nil {
		return nil, "", err
	}
	if space.island == nil {
		return nil, "", fmt.Errorf("space %q: %w", space.path, errIslandNotFound)
	}
	return space.island, space.path, nil
}

func (s *space) count(set int, hash uint64, delta int) {
	if s.counts[set] == nil {
		s.counts[set] = map[uint64]int{}
	}
	n := s.counts[set][hash] + delta
	if n == 0 {
		delete(s.counts[set], hash)
	} else {
		s.counts[set][hash] = n
	}
	if n == 0 || (n == 1 && delta > 0) {
		s.rehash(set)
	}
}

func (s *space) rehash(set int) {
	sum := func(hashes []uint64) uint64 {
		if len(hashes) == 0 {
			return 0
		}
		slices.Sort(hashes)
		h := xxh3.New()
		for _, hash := range hashes {
			h.Write(binary.LittleEndian.AppendUint64(nil, hash))
		}
		return h.Sum64()
	}
	var hashes []uint64
	switch set {
	case 0:
		for _, t := range s.tools {
			hashes = append(hashes, t.hash)
		}
	case 3:
		for _, c := range s.children {
			hashes = append(hashes, xxh3.HashString(c.name))
		}
	default:
		hashes = slices.Collect(maps.Keys(s.counts[set]))
	}
	s.digest[set] = sum(hashes)
}

func (s *space) digests(out map[string][4]uint64) {
	out[s.path] = s.digest
	for _, c := range s.children {
		c.digests(out)
	}
}

func (s *space) state(tier common.ModelTier) state {
	developer := tier == common.TierDeveloper
	out := state{Path: s.path, Desc: s.desc, Island: developer && s.island != nil}
	for _, t := range s.tools {
		t := *t
		if tier == common.TierAssistant {
			t.raw = t.lean
		}
		out.Tools = append(out.Tools, t)
	}
	seen := map[uint64]bool{}
	for _, in := range s.list {
		if !seen[in.hash] {
			seen[in.hash] = true
			out.Inputs = append(out.Inputs, in)
		}
	}
	for _, e := range s.data {
		if !seen[e.hash] {
			seen[e.hash] = true
			out.Data = append(out.Data, *e)
		}
	}
	for _, c := range s.children {
		out.Children = append(out.Children, child{Name: c.name, Desc: c.desc, Island: developer && c.island != nil})
	}
	return out
}

func (s *space) snapshot(tier common.ModelTier) snapshot {
	return snapshot{digest: s.digest, state: s.state(tier)}
}

func (s *space) diff(old snapshot, tier common.ModelTier) *diff {
	now := s.digest
	if now == old.digest {
		return &diff{}
	}
	cur := s.state(tier)
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

func (s *space) tree(depth uint, tier common.ModelTier) node {
	out := node{child: child{Name: s.name, Desc: s.desc, Island: tier == common.TierDeveloper && s.island != nil}}
	for _, t := range s.tools {
		out.Tools = append(out.Tools, t.Name)
	}
	seen := map[uint64]bool{}
	for _, in := range s.list {
		if !seen[in.hash] {
			seen[in.hash] = true
			out.Inputs++
		}
		if in.highlight && in.Desc != "" && !slices.Contains(out.Highlights, in.Desc) {
			out.Highlights = append(out.Highlights, in.Desc)
		}
	}
	for _, e := range s.data {
		if !seen[e.hash] {
			seen[e.hash] = true
			out.Data++
		}
	}
	for _, c := range s.children {
		if depth == 0 || len(out.Children) == 20 {
			out.More++
			continue
		}
		out.Children = append(out.Children, c.tree(depth-1, tier))
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
	s.count(2, d.hash, 1)
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if i := slices.Index(s.data, d); i >= 0 {
			s.data = slices.Delete(s.data, i, i+1)
			s.count(2, d.hash, -1)
		}
	})
	return nil
}

func (s Space) AddTool(ctx context.Context, from common.Tool) error {
	base := doors.DetachedContext(ctx)
	t := &tool{
		Name:   from.Name(),
		Desc:   from.Desc(),
		Input:  from.Input().Raw,
		decode: from.Decode,
		call: func(ctx context.Context, input any) (any, error) {
			var out any
			var err error
			settled := make(chan struct{})
			doors.OnSettle(base, func(context.Context) {
				close(settled)
			}, func(opCtx context.Context) {
				opCtx, cancel := context.WithCancelCause(opCtx)
				defer cancel(nil)
				stop := context.AfterFunc(ctx, func() {
					cancel(context.Cause(ctx))
				})
				defer stop()
				opCtx = tools.Attach(opCtx, ctx)
				opCtx = common.AttachThread(opCtx, ctx)
				out, err = from.Call(opCtx, input)
			})
			select {
			case <-settled:
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
			if err != nil && ctx.Err() != nil {
				return nil, context.Cause(ctx)
			}
			return out, err
		},
	}
	type bare tool
	short := bare(*t)
	if brief := from.Output().Brief; brief != "" {
		short.Desc += "\n" + brief
	}
	lean, err := json.Marshal(short)
	if err != nil {
		return err
	}
	full := bare(*t)
	full.Desc += "\nOutput schema: " + string(from.Output().Raw)
	raw, err := json.Marshal(full)
	if err != nil {
		return err
	}
	t.raw, t.lean, t.hash = raw, lean, xxh3.Hash(raw)
	s.root.mu.Lock()
	if ctx.Err() != nil {
		s.root.mu.Unlock()
		return context.Canceled
	}
	switch i, found := s.searchTool(t.Name); {
	case !found:
		s.tools = slices.Insert(s.tools, i, t)
		s.rehash(0)
	case s.tools[i].hash != t.hash:
		s.tools[i] = t
		s.rehash(0)
	default:
		s.tools[i] = t
	}
	s.root.mu.Unlock()
	doors.OnClean(ctx, func() {
		s.root.mu.Lock()
		defer s.root.mu.Unlock()
		if i, found := s.searchTool(t.Name); found && s.tools[i] == t {
			s.tools = slices.Delete(s.tools, i, i+1)
			s.rehash(0)
		}
	})
	return nil
}
