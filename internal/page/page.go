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
	"strings"
	"sync"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/replay"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type Page = *page

func NewPage(ctx context.Context) Page {
	root := &page{ctx: ctx}
	root.space = &space{root: root}
	return root
}

type page struct {
	mu      sync.Mutex
	ctx     context.Context
	counter uint
	space   *space
}

func (s Page) Space() Space {
	return s.space
}

func (s Page) Context() context.Context {
	return s.ctx
}

type Module = *module

type module struct {
	tools.Module
	page  *page
	snaps map[string]snapshot
	tier  common.ModelTier
}

type snapshot struct {
	digest [4]uint64
	state  state
}

type permit[T any] struct {
	module *module
	item   T
	old    snapshot
	before map[string][4]uint64
	island island.Island
}

func (p *page) Module(ctx context.Context) Module {
	tier := common.Tier(ctx)
	m := &module{
		Module: tools.NewModule(moduleName, moduleDesc(tier)),
		page:   p,
		snaps:  map[string]snapshot{},
		tier:   tier,
	}
	_, err := m.Add(toolName, toolDesc, func(ctx context.Context, arg toolArg) (toolResult, error) {
		a, err := m.act(ctx, arg.Path, func(s *space) (*tool, error) {
			i, ok := s.searchTool(utils.SanitizeName(arg.Tool))
			if !ok {
				return nil, fmt.Errorf("tool %q in space %q: %w", arg.Tool, s.path, errToolNotFound)
			}
			return s.tools[i], nil
		})
		if err != nil {
			return toolResult{}, err
		}
		input := arg.Arg
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		in, err := a.item.decode(input)
		var text string
		if err != nil && json.Unmarshal(input, &text) == nil && json.Valid([]byte(text)) {
			if unwrapped, again := a.item.decode(json.RawMessage(text)); again == nil {
				in, err = unwrapped, nil
			}
		}
		if err != nil {
			return toolResult{}, err
		}
		a.enter()
		defer a.leave()
		out, err := a.item.call(ctx, in)
		if err != nil {
			var res toolResult
			var value any
			var container api.Container
			if errors.As(err, &container) {
				value = container
			}
			if raw, merr := json.Marshal(value); value != nil && merr == nil {
				res.Output = json.RawMessage(raw)
				if !fits(ctx, "", res.Output) {
					res.Output = replay.OmittedNote(len(raw), tier)
				}
			}
			return toolResult{}, a.fail(ctx, arg.Path, err, &res, &res.report)
		}
		raw, err := json.Marshal(out)
		if err != nil {
			var res toolResult
			return toolResult{}, a.fail(ctx, arg.Path, err, &res, &res.report)
		}
		res := toolResult{Output: json.RawMessage(raw)}
		if !fits(ctx, "", res.Output) {
			res.Output = replay.OmittedNote(len(raw), tier)
		}
		err = a.report(ctx, arg.Path, "", &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(readName, readDesc, func(ctx context.Context, arg inputRef) (readResult, error) {
		a, err := m.act(ctx, arg.Path, func(s *space) (input, error) {
			return s.getInput(arg.ID, "read")
		})
		if err != nil {
			return readResult{}, err
		}
		return dispatch[readResult](ctx, a.item, "read", nil)
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(writeName, writeDesc, func(ctx context.Context, arg writeArg) (actionReport, error) {
		a, err := m.act(ctx, arg.Path, func(s *space) (input, error) {
			return s.getInput(arg.ID, "write")
		})
		if err != nil {
			return actionReport{}, err
		}
		a.enter()
		defer a.leave()
		res, err := dispatch[actionReport](ctx, a.item, "input", arg.Value)
		if err != nil {
			return actionReport{}, a.fail(ctx, arg.Path, err, &res.report, &res.report)
		}
		err = a.report(ctx, arg.Path, "", &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(clickName, clickDesc, func(ctx context.Context, arg inputRef) (actionReport, error) {
		a, err := m.act(ctx, arg.Path, func(s *space) (input, error) {
			return s.getInput(arg.ID, "click")
		})
		if err != nil {
			return actionReport{}, err
		}
		a.enter()
		defer a.leave()
		res, err := dispatch[actionReport](ctx, a.item, "click", nil)
		if err != nil {
			return actionReport{}, a.fail(ctx, arg.Path, err, &res.report, &res.report)
		}
		err = a.report(ctx, arg.Path, "", &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(focusName, focusDesc, func(ctx context.Context, arg inputRef) (report, error) {
		a, err := m.act(ctx, arg.Path, func(s *space) (input, error) {
			return s.getInput(arg.ID, "focus")
		})
		if err != nil {
			return report{}, err
		}
		var res report
		if _, err := dispatch[struct{}](ctx, a.item, "focus", nil); err != nil {
			return report{}, a.fail(ctx, arg.Path, err, &res, &res)
		}
		err = a.report(ctx, arg.Path, "", &res, &res)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(treeName, treeDesc, func(ctx context.Context, arg treeArg) (node, error) {
		p.mu.Lock()
		defer p.mu.Unlock()
		if ctx.Err() != nil {
			return node{}, context.Cause(ctx)
		}
		s, err := p.space.find(arg.Path)
		if err != nil {
			return node{}, err
		}
		depth := uint(3)
		if arg.Depth != nil {
			depth = *arg.Depth
		}
		return s.tree(depth, tier), nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(syncName, syncDesc, func(ctx context.Context, arg syncArg) (state, error) {
		p.mu.Lock()
		defer p.mu.Unlock()
		if ctx.Err() != nil {
			return state{}, context.Cause(ctx)
		}
		s, err := p.space.find(arg.Path)
		if err != nil {
			return state{}, err
		}
		live := s.snapshot(tier)
		if !fits(ctx, "", live.state) {
			return state{}, fmt.Errorf("space %q: %w", s.path, spaceTooLarge(tier))
		}
		m.store(s, live)
		return live.state, nil
	})
	if err != nil {
		panic(err)
	}
	if tier != common.TierDeveloper {
		return m
	}
	_, err = m.Add(islandGuideName, islandGuideDesc, func(ctx context.Context, _ struct{}) (string, error) {
		if tools.FromJS(ctx) {
			return "", errIslandFromJS
		}
		return island.Guide(), nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandSpecName, islandSpecDesc, func(ctx context.Context, arg islandSpecArg) (string, error) {
		if tools.FromJS(ctx) {
			return "", errIslandFromJS
		}
		island, path, err := p.Island(ctx, arg.Path)
		if err != nil {
			return "", err
		}
		spec, ends, items := island.Spec(ctx, path)
		revision := island.Revision(ctx, path)
		ctrl := common.GetController(ctx)
		limit := replay.RoundItemBudget(ctrl.Config(), ctrl.Model()) / 2
		want := max(arg.Page, 1)
		whole := fmt.Sprintf("revision %s\n\n", revision)
		if len(whole)+len(spec) <= limit {
			if want > 1 {
				return "", fmt.Errorf("page %d, the spec has 1 in all: %w", want, errIslandSpecPage)
			}
			return whole + spec, nil
		}
		head := func(i int, n int) string {
			if i < n {
				return fmt.Sprintf("revision %s\npage %d of %d: call island_spec with page %d for the rest\n\n", revision, i, n, i+1)
			}
			return fmt.Sprintf("revision %s\npage %d of %d\n\n", revision, i, n)
		}
		room := limit - len(head(len(ends)-1, len(ends)))
		cuts := []int{0}
		prev := 0
		for i, end := range ends {
			if end-prev > room {
				return "", fmt.Errorf("%s takes %d bytes, a page holds %d: %w", items[i], end-prev, room, errIslandSpecItem)
			}
			if end-cuts[len(cuts)-1] > room {
				cuts = append(cuts, prev)
			}
			prev = end
		}
		cuts = append(cuts, len(spec))
		pages := len(cuts) - 1
		if want > uint(pages) {
			return "", fmt.Errorf("page %d, the spec has %d in all: %w", want, pages, errIslandSpecPage)
		}
		return head(int(want), pages) + spec[cuts[want-1]:cuts[want]], nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandSpecSchemaName, islandSpecSchemaDesc, func(ctx context.Context, arg islandSpecSchemaArg) (string, error) {
		if tools.FromJS(ctx) {
			return "", errIslandFromJS
		}
		island, path, err := p.Island(ctx, arg.Path)
		if err != nil {
			return "", err
		}
		if island.Revision(ctx, path) != arg.Revision {
			return "", errIslandRevision
		}
		schema, err := island.Schema(arg.Items)
		if err != nil {
			return "", err
		}
		ctrl := common.GetController(ctx)
		if limit := replay.RoundItemBudget(ctrl.Config(), ctrl.Model()) / 2; len(schema) > limit {
			return "", fmt.Errorf("%d bytes, a result holds %d: %w", len(schema), limit, errIslandSchemaSize)
		}
		return schema, nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandBeginName, islandBeginDesc, func(ctx context.Context, arg islandBeginArg) (islandRunResult, error) {
		if tools.FromJS(ctx) {
			return islandRunResult{}, errIslandFromJS
		}
		if (arg.File == "") == (arg.Code == "") {
			return islandRunResult{}, errFileOrCode
		}
		island, path, err := p.Island(ctx, arg.Path)
		if err != nil {
			return islandRunResult{}, err
		}
		if island.Revision(ctx, path) != arg.Revision {
			return islandRunResult{}, errIslandRevision
		}
		var task, script uint
		if arg.File != "" {
			task, script, err = island.StartFile(ctx, path, arg.Comment, arg.File)
		} else {
			task, script, err = island.StartCode(ctx, path, arg.Comment, arg.Code)
		}
		return islandRunResult{Task: task, Script: script}, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandAppendName, islandAppendDesc, func(ctx context.Context, arg islandNextArg) (islandRunResult, error) {
		if tools.FromJS(ctx) {
			return islandRunResult{}, errIslandFromJS
		}
		if (arg.File == "") == (arg.Code == "") {
			return islandRunResult{}, errFileOrCode
		}
		island, _, err := p.Island(ctx, arg.Path)
		if err != nil {
			return islandRunResult{}, err
		}
		var task, script uint
		if arg.File != "" {
			task, script, err = island.ContinueFile(ctx, arg.File, false)
		} else {
			task, script, err = island.ContinueCode(arg.Code, false)
		}
		return islandRunResult{Task: task, Script: script}, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandInterruptName, islandInterruptDesc, func(ctx context.Context, arg islandNextArg) (islandRunResult, error) {
		if tools.FromJS(ctx) {
			return islandRunResult{}, errIslandFromJS
		}
		if (arg.File == "") == (arg.Code == "") {
			return islandRunResult{}, errFileOrCode
		}
		island, _, err := p.Island(ctx, arg.Path)
		if err != nil {
			return islandRunResult{}, err
		}
		var task, script uint
		if arg.File != "" {
			task, script, err = island.ContinueFile(ctx, arg.File, true)
		} else {
			task, script, err = island.ContinueCode(arg.Code, true)
		}
		return islandRunResult{Task: task, Script: script}, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandEndName, islandEndDesc, func(ctx context.Context, arg islandEndArg) (struct{}, error) {
		if tools.FromJS(ctx) {
			return struct{}{}, errIslandFromJS
		}
		island, _, err := p.Island(ctx, arg.Path)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, island.Done(ctx)
	})
	if err != nil {
		panic(err)
	}
	return m
}

func (m Module) Reset() {
	m.page.mu.Lock()
	defer m.page.mu.Unlock()
	clear(m.snaps)
}

func (s Page) Island(ctx context.Context, path string) (island.Island, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return nil, "", context.Cause(ctx)
	}
	return s.space.findIsland(path)
}

func (m *module) act[T any](ctx context.Context, path string, find func(s *space) (T, error)) (permit[T], error) {
	m.page.mu.Lock()
	defer m.page.mu.Unlock()
	if ctx.Err() != nil {
		return permit[T]{}, context.Cause(ctx)
	}
	s, err := m.page.space.find(path)
	if err != nil {
		return permit[T]{}, err
	}
	old, ok := m.snaps[s.path]
	if ok && old.digest == s.digest {
		item, err := find(s)
		if err != nil {
			return permit[T]{}, err
		}
		before := map[string][4]uint64{}
		m.page.space.digests(before)
		return permit[T]{module: m, item: item, old: old, before: before, island: s.within}, nil
	}
	if ok {
		return permit[T]{}, errRefusedChanged
	}
	return permit[T]{}, errRefusedUnread
}

func (m *module) store(s *space, snap snapshot) {
	m.snaps[s.path] = snap
	prefix := s.path + "."
	if s.path == "" {
		prefix = ""
	}
	for path := range m.snaps {
		rest, ok := strings.CutPrefix(path, prefix)
		if !ok || rest == "" {
			continue
		}
		name, _, _ := strings.Cut(rest, ".")
		if _, found := s.search(name); !found {
			delete(m.snaps, path)
		}
	}
}

func (a permit[T]) enter() {
	if a.island != nil {
		a.island.Enter()
	}
}

func (a permit[T]) leave() {
	if a.island != nil {
		a.island.Leave()
	}
}

func (a permit[T]) report(ctx context.Context, path string, message string, res any, rep *report) error {
	p, tier := a.module.page, a.module.tier
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	s, err := p.space.find(path)
	if err != nil {
		rep.Diff = &diff{Removed: true}
	} else {
		rep.Diff = s.diff(a.old, tier)
	}
	rep.Changes = p.space.changes(a.before, path)
	if !fits(ctx, message, res) {
		*rep = report{}
		return nil
	}
	if s != nil {
		a.module.store(s, s.snapshot(tier))
	}
	return nil
}

func (a permit[T]) fail(ctx context.Context, path string, err error, res any, rep *report) error {
	f := failure{result: res, err: err}
	if a.report(ctx, path, f.Error(), res, rep) != nil || rep.Diff == nil {
		return err
	}
	return f
}

func fits(ctx context.Context, message string, v any) bool {
	if !tools.FromAgent(ctx) {
		return true
	}
	raw, err := json.Marshal(v)
	ctrl := common.GetController(ctx)
	return err == nil && len(message)+len(raw) <= replay.RoundItemBudget(ctrl.Config(), ctrl.Model())
}

func dispatch[T any](ctx context.Context, in input, action string, value any) (T, error) {
	var res T
	callCtx, cancel := context.WithCancel(in.ctx)
	defer cancel()
	ch := doors.Call(callCtx, doors.ActionEmit[T]{
		Name: in.elementID,
		Arg:  inputArg{Action: action, Value: value},
	}.Into(&res))
	scheduled := false
	for {
		select {
		case <-ctx.Done():
			var t T
			return t, context.Cause(ctx)
		case err, ok := <-ch:
			var t T
			if !ok {
				return t, context.Canceled
			}
			if err != nil {
				return t, err
			}
			if scheduled {
				return res, nil
			}
			scheduled = true
		}
	}
}
