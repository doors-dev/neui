// Managed by GoX v0.3.4

//line tools.gox:1
package rower

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

type Tools = *tools

type toolModule struct {
	name string
	count int
}

type toolsState int

const (
	toolsNone toolsState = iota
	toolsOpen
	toolsClosed
	toolsResolved
	toolsMerged
)

func NewTools(state api.Indexed[api.Content], done bool) Tools {
	t := &tools{
		names: make(map[int]*strings.Builder),
	}
	for index, c := range state {
		call, ok := c.(api.ContentToolUse)
		if !ok {
			continue
		}
		t.state = toolsOpen
		b := new(strings.Builder)
		b.WriteString(call.Name)
		t.names[index] = b
	}
	t.icon.Outer(context.Background(), toolsRunning())
	if done {
		t.Close(context.Background())
		t.Done()
	}
	return t
}

type tools struct {
	frame doors.Door
	icon doors.Door
	mu sync.Mutex
	state toolsState
	names map[int]*strings.Builder
	counts []toolModule
}

func (t Tools) IsEmpty() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == toolsNone
}

func (t *tools) destruct(ctx context.Context) ([]toolModule, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state < toolsClosed {
		panic("can't merge unfinished tool call")
	}
	running := t.state == toolsClosed
	t.state = toolsMerged
	t.frame.Static(ctx, nil)
	return t.counts, running
}

func (t Tools) Merge(ctx context.Context, n Tools) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state < toolsClosed {
		panic("can't merge into unfinished tools")
	}
	counts, running := n.destruct(ctx)
	for _, c := range counts {
		t.count(c.name, c.count)
	}
	if running && t.state == toolsResolved {
		t.state = toolsClosed
		t.icon.Outer(ctx, toolsRunning())
	}
	t.frame.Reload(ctx)
}

func (t Tools) Done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == toolsNone || t.state == toolsResolved {
		return
	}
	t.state = toolsResolved
	t.icon.Outer(context.Background(), toolsDone())
}

func (t Tools) Close(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == toolsNone {
		t.frame.Static(ctx, nil)
		return
	}
	if t.state != toolsOpen {
		return
	}
	t.state = toolsClosed
	for _, index := range slices.Sorted(maps.Keys(t.names)) {
		name := t.names[index].String()
		module, _, _ := strings.Cut(name, "__")
		if module == "chat" {
			continue
		}
		t.count(module, 1)
	}
	t.names = nil
}

func (t Tools) Reload(ctx context.Context) {
	t.frame.Reload(ctx)
}

func (t *tools) count(module string, count int) {
	index := slices.IndexFunc(t.counts, func(m toolModule) bool {
		return m.name == module
	})
	if index >= 0 {
		t.counts[index].count += count
		return
	}
	t.counts = append(t.counts, toolModule{name: module, count: count})
}

func (t Tools) Append(e api.EventTool) {
	if e.Name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == toolsNone {
		t.state = toolsOpen
	}
	if t.state > toolsOpen {
		panic("unexpected tools state")
	}
	b, ok := t.names[e.Index]
	if !ok {
		b = new(strings.Builder)
		t.names[e.Index] = b
	}
	b.WriteString(e.Name)
}

//line tools.gox:166
func (t Tools) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line tools.gox:167
		__e = (t.frame).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line tools.gox:167
			__e = __c.Any(func() any {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.state == toolsOpen {
			return nil
		}
		if len(t.counts) == 0 {
			return nil
		}
		counts := slices.Clone(t.counts)
		return block{
			"Tool calls",
			&t.icon,
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line tools.gox:180
					__e = __c.Set("class", "nechat-col nechat-tools-flat"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line tools.gox:181
					for _, c := range counts {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line tools.gox:182
							__e = __c.Set("class", "nechat-tool-flat"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line tools.gox:183
							__e = __c.Any(c.name); if __e != nil { return }
//line tools.gox:184
							if c.count > 1 {
								__e = __c.Init("span"); if __e != nil { return }
								{
//line tools.gox:185
									__e = __c.Set("class", "nechat-tool-count"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line tools.gox:185
									__e = __c.Any("×" + strconv.Itoa(c.count)); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line tools.gox:189
			return }),
		}
	}()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line tools.gox:192
}
