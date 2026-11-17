// Managed by GoX v0.4.0

//line thinking.gox:1
package rower

import (
	"context"
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

//line thinking.gox:41
func (t *thinking) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:43
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
	}

//line thinking.gox:57
		__e = __c.Any(block{"Reasoning", t.slot, thoughts}); if __e != nil { return }
//line thinking.gox:58
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
//line thinking.gox:68
}

//line thinking.gox:70
func (t *thinking) thoughtsPlaceholder() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:71
		__e = (t.thoughtsDoor).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:71
				__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:71
				__e = __c.Any(summary); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line thinking.gox:72
}

//line thinking.gox:74
func (t *thinking) summaryBlock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line thinking.gox:75
		__e = __c.Any(block{
		"Summary",
		t.summarySlot,
		gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:78
				__e = __c.Set("class", "nechat-col nechat-summary"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:79
				__e = __c.Any(t.summaryStreamer); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line thinking.gox:80
		return }),
	}); if __e != nil { return }
	return })
//line thinking.gox:82
}

//line thinking.gox:84
func (t *thinking) thoughtsBlock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("details"); if __e != nil { return }
		{
//line thinking.gox:85
			__e = __c.Set("class", "nechat-col nechat-think"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("summary"); if __e != nil { return }
			{
//line thinking.gox:86
				__e = __c.Set("class", "nechat-think-head"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:86
				__e = __c.Any(summary); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line thinking.gox:87
				__e = __c.Set("class", "nechat-think-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line thinking.gox:88
				__e = __c.Any(t.thoughtsStreamer); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line thinking.gox:91
}

func (t *thinking) Append(e api.IndexedEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	switch e := e.(type) {
	case api.EventThinking:
		if e.Text == "" {
			return
		}
		if t.thoughtsStreamer == nil {
			t.thoughtsStreamer = newPartialRower(map[int]string{
				e.Part: e.Text,
			})
			if t.rendered {
				t.thoughtsDoor.Static(context.Background(), t.thoughtsBlock())
				t.thoughtsDoor = nil
			}
		} else {
			t.thoughtsStreamer.Append(partialText{e.Part, e.Text})
		}
	case api.EventSummary:
		if e.Text == "" {
			return
		}
		if t.summaryStreamer == nil {
			t.summaryStreamer = newPartialRower(map[int]string{
				e.Part: e.Text,
			})
			if t.rendered {
				t.summaryDoor.Static(context.Background(), t.summaryBlock())
				t.summaryDoor = nil
			}
		} else {
			t.summaryStreamer.Append(partialText{e.Part, e.Text})
		}
	case api.EventClose:
		t.close()
	}
}

func (t *thinking) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.close()
}

func (t *thinking) close() {
	if t.closed {
		return
	}
	t.closed = true
	t.slot.Static(context.Background(), thinkingDone())
	t.summarySlot.Static(context.Background(), summaryDone())
	if t.summaryStreamer != nil {
		t.summaryStreamer.Close()
	}
	if t.thoughtsStreamer != nil {
		t.thoughtsStreamer.Close()
	}
	if t.thoughtsDoor != nil {
		t.thoughtsDoor.Static(context.Background(), nil)
	}
	if t.summaryDoor != nil {
		t.summaryDoor.Static(context.Background(), nil)
	}
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
