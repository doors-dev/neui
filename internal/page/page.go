package page

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/doors-dev/doors"
	"strings"
	"sync"
	"sync/atomic"

	"neui.dev/internal/common"
	"neui.dev/internal/island"
	"neui.dev/internal/replay"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
)

type Page = *page

func NewPage(ctx context.Context) Page {
	root := &page{}
	seq := root.adv()
	root.space = &space{
		root:     root,
		ctx:      ctx,
		seq:      seq,
		deltaSeq: seq,
		actions:  tools.NewModule("root", ""),
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

func (s *page) Module(ctx context.Context) common.Module {
	pageModule := tools.NewModule(moduleName, moduleDesc+"\n\n"+islandsDesc)
	synced := atomic.Bool{}
	toolInput, err := schema.NewSchema[toolArg]()
	if err != nil {
		panic(err)
	}
	toolOutput, err := schema.NewSchema[toolResult[any]]()
	if err != nil {
		panic(err)
	}
	toolInputRaw, err := schema.NewRawSchema(json.RawMessage(toolInput.Raw()))
	if err != nil {
		panic(err)
	}
	toolOutputRaw, err := schema.NewRawSchema(json.RawMessage(toolOutput.Raw()))
	if err != nil {
		panic(err)
	}
	_, err = pageModule.AddRaw(toolName, toolDesc, toolInputRaw, toolOutputRaw, func(ctx context.Context, raw json.RawMessage) (any, error) {
		if !synced.Load() {
			return nil, errSync
		}
		input, err := toolInput.Decode(raw)
		if err != nil {
			return nil, err
		}
		return s.tool(ctx, input)
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(readName, readDesc, func(ctx context.Context, arg inputRef) (readResult, error) {
		if !synced.Load() {
			return readResult{}, errSync
		}
		res, _, _, err := dispatch[readResult](s, ctx, arg, "read", nil)
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(writeName, writeDesc, func(ctx context.Context, arg writeArg) (hookResult, error) {
		if !synced.Load() {
			return hookResult{}, errSync
		}
		res, target, before, err := dispatch[hookResult](s, ctx, inputRef{Path: arg.Path, Seq: arg.Seq, ID: arg.ID}, "input", arg.Value)
		if err != nil {
			return hookResult{}, err
		}
		s.mu.Lock()
		if target.ctx.Err() == nil {
			res.Seq = target.seq
		}
		res.Changes, err = s.collectChanges(changesArg{SincePageSeq: before})
		s.mu.Unlock()
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(clickName, clickDesc, func(ctx context.Context, arg inputRef) (hookResult, error) {
		if !synced.Load() {
			return hookResult{}, errSync
		}
		res, target, before, err := dispatch[hookResult](s, ctx, arg, "click", nil)
		if err != nil {
			return hookResult{}, err
		}
		s.mu.Lock()
		if target.ctx.Err() == nil {
			res.Seq = target.seq
		}
		res.Changes, err = s.collectChanges(changesArg{SincePageSeq: before})
		s.mu.Unlock()
		return res, err
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(focusName, focusDesc, func(ctx context.Context, arg inputRef) (struct{}, error) {
		if !synced.Load() {
			return struct{}{}, errSync
		}
		_, _, _, err := dispatch[struct{}](s, ctx, arg, "focus", nil)
		return struct{}{}, err
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(syncName, syncDesc, func(ctx context.Context, arg syncArg) (syncResult, error) {
		out, err := s.sync(ctx, arg)
		if err != nil {
			return syncResult{}, err
		}
		synced.Store(true)
		return out, nil
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(changesName, changesDesc, func(ctx context.Context, arg changesArg) (changesResult, error) {
		if !synced.Load() {
			return changesResult{}, errSync
		}
		return s.changes(ctx, arg)
	})
	if err != nil {
		panic(err)
	}
	_, err = pageModule.Add(islandSpecName, islandSpecDesc, func(ctx context.Context, arg islandSpecArg) (string, error) {
		if tools.FromJS(ctx) {
			return "", errIslandFromJS
		}
		if !synced.Load() {
			return "", errSync
		}
		island, seq, path, err := s.island(ctx, arg.Path)
		if err != nil {
			return "", err
		}
		spec, ends, items := island.Spec(ctx, path)
		ctrl := common.GetController(ctx)
		limit := replay.ResultTextCap(ctrl.Config(), ctrl.Model())
		part := max(arg.Part, 1)
		whole := fmt.Sprintf("seq %d\n\n", seq)
		if limit <= 0 || len(whole)+len(spec) <= limit {
			if part > 1 {
				return "", fmt.Errorf("part %d, the spec has 1 in all: %w", part, errIslandSpecPart)
			}
			return whole + spec, nil
		}
		head := func(i int, n int) string {
			if i < n {
				return fmt.Sprintf("seq %d\npart %d of %d: call island_spec with part %d for the rest\n\n", seq, i, n, i+1)
			}
			return fmt.Sprintf("seq %d\npart %d of %d\n\n", seq, i, n)
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
	_, err = pageModule.Add(islandStartName, islandStartDesc, func(ctx context.Context, arg islandStartArg) (islandRunResult, error) {
		if tools.FromJS(ctx) {
			return islandRunResult{}, errIslandFromJS
		}
		if !synced.Load() {
			return islandRunResult{}, errSync
		}
		if (arg.File == "") == (arg.Code == "") {
			return islandRunResult{}, errFileOrCode
		}
		island, seq, path, err := s.island(ctx, arg.Path)
		if err != nil {
			return islandRunResult{}, err
		}
		if seq != arg.Seq {
			return islandRunResult{}, errIslandSeqMismatch
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
	_, err = pageModule.Add(islandContinueName, islandContinueDesc, func(ctx context.Context, arg islandContinueArg) (islandRunResult, error) {
		if tools.FromJS(ctx) {
			return islandRunResult{}, errIslandFromJS
		}
		if !synced.Load() {
			return islandRunResult{}, errSync
		}
		if (arg.File == "") == (arg.Code == "") {
			return islandRunResult{}, errFileOrCode
		}
		island, _, _, err := s.island(ctx, arg.Path)
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
	_, err = pageModule.Add(islandStopName, islandStopDesc, func(ctx context.Context, arg islandStopArg) (struct{}, error) {
		if tools.FromJS(ctx) {
			return struct{}{}, errIslandFromJS
		}
		if !synced.Load() {
			return struct{}{}, errSync
		}
		island, _, _, err := s.island(ctx, arg.Path)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, island.Done(ctx)
	})
	if err != nil {
		panic(err)
	}
	return pageModule
}

func (s Page) Island(ctx context.Context, path string) (island.Island, string, error) {
	island, _, path, err := s.island(ctx, path)
	return island, path, err
}

func (s *page) island(ctx context.Context, path string) (island.Island, uint, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return nil, 0, "", context.Cause(ctx)
	}
	return s.space.findIsland(strings.Split(path, "."))
}

func (s *page) tool(ctx context.Context, arg toolArg) (any, error) {
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return nil, context.Cause(ctx)
	}
	tool, err := s.space.getTool(strings.Split(arg.Path, "."), arg.Seq, arg.Tool)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return tool.Dispatched(ctx, arg.Arg)
}

func dispatch[T any](s *page, ctx context.Context, ref inputRef, action string, value any) (T, *space, uint, error) {
	var res T
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return res, nil, 0, context.Cause(ctx)
	}
	target, in, err := s.space.getInput(strings.Split(ref.Path, "."), ref.Seq, ref.ID)
	before := s.counter
	s.mu.Unlock()
	if err != nil {
		return res, nil, 0, err
	}
	switch action {
	case "read":
		if in.Rights == W {
			return res, nil, 0, fmt.Errorf("%q on input %d with rights %s: %w", action, ref.ID, in.Rights, errRights)
		}
	case "click", "input":
		if in.Rights == R {
			return res, nil, 0, fmt.Errorf("%q on input %d with rights %s: %w", action, ref.ID, in.Rights, errRights)
		}
	}
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
			return t, nil, 0, context.Cause(ctx)
		case err, ok := <-ch:
			var t T
			if !ok {
				return t, nil, 0, context.Canceled
			}
			if err != nil {
				return t, nil, 0, err
			}
			if scheduled {
				return res, target, before, nil
			}
			scheduled = true
		}
	}
}

func (s *page) sync(ctx context.Context, arg syncArg) (syncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, err := s.space.sync(strings.Split(arg.Path, "."), arg.Depth+1)
	if err != nil {
		return syncResult{}, err
	}
	return syncResult{PageSeq: s.counter, Space: target}, nil
}

func (s *page) changes(ctx context.Context, arg changesArg) (changesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collectChanges(arg)
}

func (s *page) collectChanges(arg changesArg) (changesResult, error) {
	node, err := s.space.find(strings.Split(arg.Path, "."))
	if err != nil {
		return changesResult{}, err
	}
	spaces := []*syncSpace{}
	node.collectChanged(arg.SincePageSeq, &spaces)
	return changesResult{PageSeq: s.counter, Spaces: spaces}, nil
}

func (s *page) adv() uint {
	s.counter += 1
	return s.counter
}
