// Managed by GoX v0.4.0

//line chips.gox:1
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
	"neui.dev/nekit/internal/chat/params"
)

type Chips = *chips

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

func NewChips(p params.Params, state api.Indexed[api.Content], done bool) Chips {
	c := &chips{
		params: p,
		callNames: make(map[int]*strings.Builder),
	}
	for index, content := range state {
		call, ok := content.(api.ContentToolUse)
		if !ok {
			continue
		}
		c.tools = toolsOpen
		b := new(strings.Builder)
		b.WriteString(call.Name)
		c.callNames[index] = b
	}
	c.toolsIcon.Outer(context.Background(), toolsRunning())
	if done {
		c.Close(context.Background())
		c.Done(context.Background())
	}
	return c
}

type chips struct {
	params params.Params
	recordsFrame doors.Door
	reasoningFrame doors.Door
	reasoningIcon doors.Door
	reasoningTally doors.Door
	thoughtsBody doors.Door
	toolsFrame doors.Door
	toolsIcon doors.Door
	summaryFrame doors.Door
	summaryIcon doors.Door
	summaryBody doors.Door
	mu sync.Mutex
	records int
	reasoning int
	reasoningOpen bool
	reasoningIndex int
	thoughts streamer[partialText]
	summary streamer[partialText]
	pendingSummary map[int]string
	expandable bool
	summarized bool
	tools toolsState
	callNames map[int]*strings.Builder
	modules []toolModule
}

func (c Chips) HasCalls() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools != toolsNone
}

func (c Chips) Records(ctx context.Context, count int) {
	if count == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records += count
	c.recordsFrame.Reload(ctx)
}

func (c Chips) Reason(ctx context.Context, e api.IndexedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	part, text, isSummary := 0, "", false
	switch e := e.(type) {
	case api.EventThinking:
		part, text = e.Part, e.Text
	case api.EventSummary:
		part, text, isSummary = e.Part, e.Text, true
	case api.EventClose:
		if c.reasoningOpen && c.reasoningIndex == e.Index {
			c.closeReasoning(ctx)
		}
		return
	}
	if text == "" {
		return
	}
	reload := false
	if !c.reasoningOpen || c.reasoningIndex != e.At() {
		if strings.TrimSpace(text) == "" {
			return
		}
		c.closeReasoning(ctx)
		c.reasoningOpen = true
		c.reasoningIndex = e.At()
		c.thoughts = nil
		c.summary = nil
		c.pendingSummary = make(map[int]string)
		c.reasoning++
		c.reasoningIcon.Outer(ctx, thinkingRunning())
		if c.reasoning == 1 {
			reload = true
		} else {
			c.reasoningTally.Inner(ctx, tally(c.reasoning))
		}
	}
	switch {
	case isSummary && c.summary != nil:
		apply(ctx, c.summary.Append(partialText{part, text}))
	case isSummary && !c.summarized:
		c.summary = newPartialRower(map[int]string{part: text})
		c.summaryIcon.Outer(ctx, summaryRunning())
		c.summaryBody.Inner(ctx, c.summary)
		c.summarized = true
		c.summaryFrame.Reload(ctx)
	case isSummary:
		c.pendingSummary[part] += text
	case c.thoughts != nil:
		apply(ctx, c.thoughts.Append(partialText{part, text}))
	default:
		c.thoughts = newPartialRower(map[int]string{part: text})
		c.thoughtsBody.Inner(ctx, c.thoughts)
		reload = reload || !c.expandable
		c.expandable = true
	}
	if reload {
		c.reasoningFrame.Reload(ctx)
	}
}

func (c *chips) closeReasoning(ctx context.Context) {
	if !c.reasoningOpen {
		return
	}
	c.reasoningOpen = false
	c.reasoningIcon.Outer(ctx, thinkingDone())
	if c.thoughts != nil {
		apply(ctx, c.thoughts.Close())
	}
	if c.summary != nil {
		apply(ctx, c.summary.Close())
		c.summaryIcon.Outer(ctx, summaryDone())
		return
	}
	for _, part := range c.pendingSummary {
		if strings.TrimSpace(part) == "" {
			continue
		}
		c.summary = newPartialRower(c.pendingSummary)
		apply(ctx, c.summary.Close())
		c.summaryIcon.Outer(ctx, summaryDone())
		c.summaryBody.Inner(ctx, c.summary)
		return
	}
}

func apply(ctx context.Context, replacements []replacement) {
	for _, rp := range replacements {
		rp.door.Static(ctx, rp.content)
	}
}

func (c *chips) destruct(ctx context.Context) ([]toolModule, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tools < toolsClosed {
		panic("can't merge unfinished tool call")
	}
	running := c.tools == toolsClosed
	c.tools = toolsMerged
	c.toolsFrame.Static(ctx, nil)
	return c.modules, running
}

func (c Chips) Merge(ctx context.Context, n Chips) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tools == toolsOpen {
		panic("can't merge into unfinished tools")
	}
	modules, running := n.destruct(ctx)
	for _, module := range modules {
		c.count(module.name, module.count)
	}
	if running && c.tools != toolsClosed {
		c.tools = toolsClosed
		c.toolsIcon.Outer(ctx, toolsRunning())
	} else if !running && c.tools == toolsNone {
		c.tools = toolsResolved
		c.toolsIcon.Outer(ctx, toolsDone())
	}
	c.toolsFrame.Reload(ctx)
}

func (c Chips) Done(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tools == toolsNone || c.tools == toolsResolved {
		return
	}
	c.tools = toolsResolved
	c.toolsIcon.Outer(ctx, toolsDone())
}

func (c Chips) Close(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeReasoning(ctx)
	if c.tools != toolsOpen {
		return
	}
	c.tools = toolsClosed
	for _, index := range slices.Sorted(maps.Keys(c.callNames)) {
		name := c.callNames[index].String()
		module, _, found := strings.Cut(name, "__")
		if !found || module == "" || module == "chat" || strings.ContainsFunc(module, func(r rune) bool {
			return r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
		}) {
			continue
		}
		c.count(module, 1)
	}
	c.callNames = nil
}

func (c Chips) Reload(ctx context.Context) {
	c.toolsFrame.Reload(ctx)
}

func (c *chips) count(module string, count int) {
	index := slices.IndexFunc(c.modules, func(m toolModule) bool {
		return m.name == module
	})
	if index >= 0 {
		c.modules[index].count += count
		return
	}
	c.modules = append(c.modules, toolModule{name: module, count: count})
}

func (c Chips) Append(e api.EventTool) {
	if e.Name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tools == toolsNone {
		c.tools = toolsOpen
	}
	if c.tools > toolsOpen {
		panic("unexpected tools state")
	}
	b, ok := c.callNames[e.Index]
	if !ok {
		b = new(strings.Builder)
		c.callNames[e.Index] = b
	}
	b.WriteString(e.Name)
}

//line chips.gox:305
func (c Chips) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chips.gox:306
		__e = (c.recordsFrame).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line chips.gox:306
			__e = __c.Any(func() any {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.records == 0 || c.params.Off(params.Tools) {
			return nil
		}
		records := c.records
		return block{
			"Events",
			recordsIcon(),
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chips.gox:316
					__e = __c.Set("class", "nechat-col nechat-events"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("event "); if __e != nil { return }
//line chips.gox:316
					__e = __c.Any(tally(records)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line chips.gox:316
			return }),
		}
	}()); if __e != nil { return }
		return })); if __e != nil { return }
//line chips.gox:319
		__e = (c.reasoningFrame).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line chips.gox:319
			__e = __c.Any(func() any {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.reasoning == 0 {
			return nil
		}
		if !c.expandable {
			return block{
				"Reasoning",
				&c.reasoningIcon,
				gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chips.gox:329
						__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("reasoning "); if __e != nil { return }
//line chips.gox:329
						__e = __c.Any(&c.reasoningTally); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line chips.gox:329
				return }),
			}
		}
		return block{
			"Reasoning",
			&c.reasoningIcon,
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("details"); if __e != nil { return }
				{
//line chips.gox:335
					__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("summary"); if __e != nil { return }
					{
//line chips.gox:336
						__e = __c.Set("class", "nechat-think-head"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("reasoning "); if __e != nil { return }
//line chips.gox:336
						__e = __c.Any(&c.reasoningTally); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chips.gox:337
						__e = __c.Set("class", "nechat-think-body"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chips.gox:337
						__e = __c.Any(&c.thoughtsBody); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line chips.gox:338
			return }),
		}
	}()); if __e != nil { return }
		return })); if __e != nil { return }
//line chips.gox:341
		__e = (c.toolsFrame).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line chips.gox:341
			__e = __c.Any(func() any {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.tools == toolsOpen || c.tools == toolsMerged {
			return nil
		}
		if len(c.modules) == 0 || c.params.Off(params.Tools) {
			return nil
		}
		modules := slices.Clone(c.modules)
		return block{
			"Tool calls",
			&c.toolsIcon,
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chips.gox:354
					__e = __c.Set("class", "nechat-col nechat-tools-flat"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line chips.gox:355
					for _, module := range modules {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line chips.gox:356
							__e = __c.Set("class", "nechat-tool-flat"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line chips.gox:357
							__e = __c.Any(module.name); if __e != nil { return }
//line chips.gox:358
							__e = __c.Any(tally(module.count)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
//line chips.gox:361
			return }),
		}
	}()); if __e != nil { return }
		return })); if __e != nil { return }
//line chips.gox:364
		__e = (c.summaryFrame).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line chips.gox:364
			__e = __c.Any(func() any {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.summarized {
			return nil
		}
		return block{
			"Summary",
			&c.summaryIcon,
			gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chips.gox:373
					__e = __c.Set("class", "nechat-col nechat-summary"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line chips.gox:373
					__e = __c.Any(&c.summaryBody); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line chips.gox:373
			return }),
		}
	}()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line chips.gox:376
}

//line chips.gox:378
func tally(count int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chips.gox:379
		if count > 1 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line chips.gox:380
				__e = __c.Set("class", "nechat-tool-count"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chips.gox:380
				__e = __c.Any("×" + strconv.Itoa(count)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line chips.gox:382
}
