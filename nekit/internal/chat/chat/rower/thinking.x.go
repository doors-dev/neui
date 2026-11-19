// Managed by GoX v0.4.0

//line thinking.gox:1
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
	"errors"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

func newThinking(thoughts map[int]string, summary map[int]string) streamer[api.IndexedEvent] {
	t := &thinking{
		slot: newSlot(thinkingRunning()),
		summarySlot: newSlot(summaryRunning()),
	}
	if !emptyMap(thoughts) {
		t.thoughtsStreamer = newPartialRower(thoughts)
	}
	if !emptyMap(summary) {
		t.summaryStreamer = newPartialRower(summary)
	}
	return t
}

type thinking struct {
	mu sync.Mutex
	closed bool
	rendered bool
	summaryDoor *doors.Door
	thoughtsStreamer streamer[partialText]
	thoughtsDoor *doors.Door
	summaryStreamer streamer[partialText]
	slot *doors.Door
	summarySlot *doors.Door
}

const summary = "reasoning"

//line thinking.gox:54
func (t *thinking) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:56
		t.mu.Lock()
	defer t.mu.Unlock()
	if t.rendered {
		return errors.New("thinking can be rendered only once")
	}
	t.rendered = true
	var thoughts gox.Elem
	if t.thoughtsStreamer != nil {
		thoughts = t.thoughtsBlock()
	} else if !t.closed {
		t.thoughtsDoor = new(doors.Door)
		thoughts = t.thoughtsPlaceholder()
	} else {
		thoughts = thoughtsLabel()
	}

//line thinking.gox:72
		__e = __c.Any(block{"Reasoning", t.slot, thoughts}); if __e != nil { return }
//line thinking.gox:73
		__e = __c.Any(func() any {
		if t.summaryStreamer != nil {
			return t.summaryBlock()
		}
		if t.closed {
			return nil
		}
		t.summaryDoor = new(doors.Door)
		return t.summaryDoor
	}()); if __e != nil { return }
	return })
//line thinking.gox:83
}

//line thinking.gox:85
func (t *thinking) thoughtsPlaceholder() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:86
		__e = (t.thoughtsDoor).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:86
				__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:86
				__e = __c.Any(summary); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line thinking.gox:87
}

//line thinking.gox:89
func thoughtsLabel() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line thinking.gox:90
			__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:90
			__e = __c.Any(summary); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line thinking.gox:91
}

//line thinking.gox:93
func (t *thinking) summaryBlock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:94
		__e = __c.Any(block{
		"Summary",
		t.summarySlot,
		gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:97
				__e = __c.Set("class", "nechat-col nechat-summary"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:98
				__e = __c.Any(t.summaryStreamer); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line thinking.gox:99
		return }),
	}); if __e != nil { return }
	return })
//line thinking.gox:101
}

//line thinking.gox:103
func (t *thinking) thoughtsBlock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("details"); if __e != nil { return }
		{
//line thinking.gox:104
			__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("summary"); if __e != nil { return }
			{
//line thinking.gox:105
				__e = __c.Set("class", "nechat-think-head"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:105
				__e = __c.Any(summary); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:106
				__e = __c.Set("class", "nechat-think-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:107
				__e = __c.Any(t.thoughtsStreamer); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line thinking.gox:110
}

func (t *thinking) Append(e api.IndexedEvent) []replacement {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	switch e := e.(type) {
	case api.EventThinking:
		if e.Text == "" {
			return nil
		}
		if t.thoughtsStreamer == nil {
			t.thoughtsStreamer = newPartialRower(map[int]string{
				e.Part: e.Text,
			})
			if t.rendered {
				door := t.thoughtsDoor
				t.thoughtsDoor = nil
				return []replacement{{door, t.thoughtsBlock()}}
			}
		} else {
			return t.thoughtsStreamer.Append(partialText{e.Part, e.Text})
		}
	case api.EventSummary:
		if e.Text == "" {
			return nil
		}
		if t.summaryStreamer == nil {
			t.summaryStreamer = newPartialRower(map[int]string{
				e.Part: e.Text,
			})
			if t.rendered {
				door := t.summaryDoor
				t.summaryDoor = nil
				return []replacement{{door, t.summaryBlock()}}
			}
		} else {
			return t.summaryStreamer.Append(partialText{e.Part, e.Text})
		}
	case api.EventClose:
		return t.close()
	}
	return nil
}

func (t *thinking) Close() []replacement {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.close()
}

func (t *thinking) close() []replacement {
	if t.closed {
		return nil
	}
	t.closed = true
	replacements := []replacement{{t.slot, thinkingDone()}, {t.summarySlot, summaryDone()}}
	if t.summaryStreamer != nil {
		replacements = append(replacements, t.summaryStreamer.Close()...)
	}
	if t.thoughtsStreamer != nil {
		replacements = append(replacements, t.thoughtsStreamer.Close()...)
	}
	if t.summaryDoor != nil {
		replacements = append(replacements, replacement{t.summaryDoor, nil})
	}
	return replacements
}

func emptyMap(m map[int]string) bool {
	for _, s := range m {
		if len(s) == 0 {
			continue
		}
		return false
	}
	return true
}
