package page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/doors-dev/doors"
	"github.com/zeebo/xxh3"
	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/replay"
	"neui.dev/internal/tools"
	"neui.dev/internal/utils"
)

type Page = *page

func NewPage(ctx context.Context) Page {
	root := &page{}
	root.space = &space{
		root: root,
		ctx:  ctx,
	}
	return root
}

type page struct {
	mu      sync.Mutex
	counter uint
	space   *space
}

func (s Page) Space() Space {
	return s.space
}

func (s Page) Context() context.Context {
	return s.space.ctx
}

type Module = *module

type module struct {
	tools.Module
	page  *page
	snaps map[string]snapshot
}

type snapshot struct {
	by     uint64
	digest [4]uint64
	state  state
}

type permit struct {
	module *module
	path   string
	old    snapshot
	before map[string][4]uint64
}

func (p *page) Module() Module {
	m := &module{
		Module: tools.NewModule(moduleName, moduleDesc+"\n\n"+islandsDesc),
		page:   p,
		snaps:  map[string]snapshot{},
	}
	_, err := m.Add(toolName, toolDesc, func(ctx context.Context, arg toolArg) (toolResult, error) {
		var t *tool
		a, err := m.act(ctx, arg.Path, true, func(s *space) error {
			i, ok := s.searchTool(utils.SanitizeName(arg.Tool))
			if !ok {
				return fmt.Errorf("tool %q in space %q: %w", arg.Tool, s.path, errToolNotFound)
			}
			t = s.tools[i]
			return nil
		})
		if err != nil {
			return toolResult{}, err
		}
		out, err := t.call(ctx, arg.Arg)
		if err != nil {
			return toolResult{}, err
		}
		raw, err := json.Marshal(out)
		if err != nil {
			return toolResult{}, err
		}
		res := toolResult{Output: json.RawMessage(raw)}
		if !fits(ctx, res.Output) {
			res.Output = replay.OmittedNote(len(raw))
		}
		err = a.report(ctx, &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(readName, readDesc, func(ctx context.Context, arg inputRef) (readResult, error) {
		var in input
		_, err := m.act(ctx, arg.Path, false, func(s *space) (err error) {
			in, err = s.getInput(arg.ID, "read")
			return err
		})
		if err != nil {
			return readResult{}, err
		}
		return dispatch[readResult](ctx, in, "read", nil)
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(writeName, writeDesc, func(ctx context.Context, arg writeArg) (hookResult, error) {
		var in input
		a, err := m.act(ctx, arg.Path, true, func(s *space) (err error) {
			in, err = s.getInput(arg.ID, "input")
			return err
		})
		if err != nil {
			return hookResult{}, err
		}
		res, err := dispatch[hookResult](ctx, in, "input", arg.Value)
		if err != nil {
			return hookResult{}, err
		}
		err = a.report(ctx, &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(clickName, clickDesc, func(ctx context.Context, arg inputRef) (hookResult, error) {
		var in input
		a, err := m.act(ctx, arg.Path, true, func(s *space) (err error) {
			in, err = s.getInput(arg.ID, "click")
			return err
		})
		if err != nil {
			return hookResult{}, err
		}
		res, err := dispatch[hookResult](ctx, in, "click", nil)
		if err != nil {
			return hookResult{}, err
		}
		err = a.report(ctx, &res, &res.report)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(focusName, focusDesc, func(ctx context.Context, arg inputRef) (report, error) {
		var in input
		a, err := m.act(ctx, arg.Path, true, func(s *space) (err error) {
			in, err = s.getInput(arg.ID, "focus")
			return err
		})
		if err != nil {
			return report{}, err
		}
		if _, err := dispatch[struct{}](ctx, in, "focus", nil); err != nil {
			return report{}, err
		}
		var res report
		err = a.report(ctx, &res, &res)
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
		s, err := p.space.find(strings.Split(arg.Path, "."))
		if err != nil {
			return node{}, err
		}
		depth := uint(3)
		if arg.Depth != nil {
			depth = *arg.Depth
		}
		return s.tree(depth), nil
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
		s, err := p.space.find(strings.Split(arg.Path, "."))
		if err != nil {
			return state{}, err
		}
		live := s.snapshot(tools.RunID(ctx))
		if !fits(ctx, live.state) {
			return state{}, fmt.Errorf("space %q: %w", s.path, errSpaceTooLarge)
		}
		m.snaps[s.path] = live
		return live.state, nil
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
		revision := xxh3.HashString(spec)
		ctrl := common.GetController(ctx)
		limit := replay.RoundItemBudget(ctrl.Config(), ctrl.Model()) / 2
		part := max(arg.Part, 1)
		whole := fmt.Sprintf("revision %016x\n\n", revision)
		if len(whole)+len(spec) <= limit {
			if part > 1 {
				return "", fmt.Errorf("part %d, the spec has 1 in all: %w", part, errIslandSpecPart)
			}
			return whole + spec, nil
		}
		head := func(i int, n int) string {
			if i < n {
				return fmt.Sprintf("revision %016x\npart %d of %d: call island_spec with part %d for the rest\n\n", revision, i, n, i+1)
			}
			return fmt.Sprintf("revision %016x\npart %d of %d\n\n", revision, i, n)
		}
		room := limit - len(head(len(ends)-1, len(ends)))
		cuts := []int{0}
		prev := 0
		for i, end := range ends {
			if end-prev > room {
				return "", fmt.Errorf("%s takes %d bytes, a part holds %d: %w", items[i], end-prev, room, errIslandSpecItem)
			}
			if end-cuts[len(cuts)-1] > room {
				cuts = append(cuts, prev)
			}
			prev = end
		}
		cuts = append(cuts, len(spec))
		parts := len(cuts) - 1
		if part > uint(parts) {
			return "", fmt.Errorf("part %d, the spec has %d in all: %w", part, parts, errIslandSpecPart)
		}
		return head(int(part), parts) + spec[cuts[part-1]:cuts[part]], nil
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandStartName, islandStartDesc, func(ctx context.Context, arg islandStartArg) (islandRunResult, error) {
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
		if spec, _, _ := island.Spec(ctx, path); fmt.Sprintf("%016x", xxh3.HashString(spec)) != arg.Revision {
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
	_, err = m.Add(islandContinueName, islandContinueDesc, func(ctx context.Context, arg islandContinueArg) (islandRunResult, error) {
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
			task, script, err = island.ContinueFile(ctx, arg.File, arg.Interrupt)
		} else {
			task, script, err = island.ContinueCode(arg.Code, arg.Interrupt)
		}
		return islandRunResult{Task: task, Script: script}, err
	})
	if err != nil {
		panic(err)
	}
	_, err = m.Add(islandStopName, islandStopDesc, func(ctx context.Context, arg islandStopArg) (struct{}, error) {
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
	return s.space.findIsland(strings.Split(path, "."))
}

func (m *module) act(ctx context.Context, path string, reports bool, find func(s *space) error) (permit, error) {
	m.page.mu.Lock()
	defer m.page.mu.Unlock()
	if ctx.Err() != nil {
		return permit{}, context.Cause(ctx)
	}
	s, err := m.page.space.find(strings.Split(path, "."))
	if err != nil {
		return permit{}, err
	}
	by := tools.RunID(ctx)
	old, ok := m.snaps[s.path]
	if ok && old.by == by && old.digest == s.digest() {
		if err := find(s); err != nil {
			return permit{}, err
		}
		a := permit{module: m, path: s.path, old: old}
		if reports {
			a.before = map[string][4]uint64{}
			m.page.space.digests(a.before)
		}
		return a, nil
	}
	live := s.snapshot(by)
	r := refusal{Refused: refusedUnreadMessage, Space: &live.state}
	if ok && old.by == by {
		r = refusal{Refused: refusedChangedMessage, Diff: s.diff(old)}
	}
	if !fits(ctx, r) {
		return permit{}, refusal{Refused: refusedTooLargeMessage}
	}
	m.snaps[s.path] = live
	return permit{}, r
}

func (a permit) report(ctx context.Context, res any, rep *report) error {
	p := a.module.page
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	s, err := p.space.find(strings.Split(a.path, "."))
	if err != nil {
		rep.Diff = &diff{Removed: true}
	} else {
		rep.Diff = s.diff(a.old)
	}
	rep.Changes = p.space.changes(a.before, a.path)
	if !fits(ctx, res) {
		*rep = report{}
		return nil
	}
	if s == nil {
		delete(a.module.snaps, a.path)
	} else {
		a.module.snaps[a.path] = s.snapshot(a.old.by)
	}
	return nil
}

func fits(ctx context.Context, v any) bool {
	if !tools.FromAgent(ctx) {
		return true
	}
	raw, err := json.Marshal(v)
	ctrl := common.GetController(ctx)
	return err == nil && len(raw) <= replay.RoundItemBudget(ctrl.Config(), ctrl.Model())
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
