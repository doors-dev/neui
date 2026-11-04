// Managed by GoX v0.3.3

//line block.gox:1
package rower

import (
	"context"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/kchat/icons"
)

func newSlot(running gox.Elem) *doors.Door {
	slot := new(doors.Door)
	slot.Outer(context.Background(), running)
	return slot
}

//line block.gox:17
func notepadLines() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line block.gox:18
			__e = __c.Set("xmlns", "http://www.w3.org/2000/svg"); if __e != nil { return }
//line block.gox:18
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
//line block.gox:18
			__e = __c.Set("fill", "currentColor"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:19
				__e = __c.Set("d", "M212,64V208a16,16,0,0,1-16,16H56a16,16,0,0,1-16-16V64A16,16,0,0,1,56,48H88V40a8,8,0,0,1,16,0v8h48V40a8,8,0,0,1,16,0v8h32A16,16,0,0,1,216,64ZM56,64V208H200V64H168v8a8,8,0,0,1-16,0V64H104v8a8,8,0,0,1-16,0V64Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:20
				__e = __c.Set("class", "line"); if __e != nil { return }
//line block.gox:20
				__e = __c.Set("d", "M168,112a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,112Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:21
				__e = __c.Set("class", "line"); if __e != nil { return }
//line block.gox:21
				__e = __c.Set("d", "M160,136H96a8,8,0,0,0,0,16h64a8,8,0,0,0,0-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:23
}

//line block.gox:25
func textRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:26
		__e = (doors.Class("kchat-ic kchat-ic_line kchat-build")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:26
			__e = __c.Any(icons.TextAlignLeftLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:27
}

//line block.gox:29
func textDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:30
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:30
			__e = __c.Any(icons.TextAlignLeft()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:31
}

//line block.gox:33
func thinkingRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line block.gox:34
			__e = __c.Set("class", "kchat-blink"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line block.gox:35
			__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line block.gox:35
				__e = __c.Any(icons.Lightbulb()); if __e != nil { return }
			return })); if __e != nil { return }
//line block.gox:36
			__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line block.gox:36
				__e = __c.Any(icons.LightbulbFill()); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:38
}

//line block.gox:40
func thinkingDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:41
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:41
			__e = __c.Any(icons.Lightbulb()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:42
}

//line block.gox:44
func summaryRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:45
		__e = (doors.Class("kchat-ic kchat-pulse")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:45
			__e = __c.Any(notepadLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:46
}

//line block.gox:48
func summaryDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:49
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:49
			__e = __c.Any(notepadLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:50
}

//line block.gox:52
func toolsRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:53
		__e = (doors.Class("kchat-ic kchat-ic_spin")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:53
			__e = __c.Any(icons.Gear()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:54
}

//line block.gox:56
func toolsDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:57
		__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:57
			__e = __c.Any(icons.Terminal()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:58
}

type block struct {
	title string
	icon any
	body any
}

//line block.gox:66
func (b block) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line block.gox:67
			__e = __c.Set("class", "kchat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line block.gox:68
				__e = __c.Set("class", "kchat-gut"); if __e != nil { return }
//line block.gox:68
				__e = __c.Set("title", b.title); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line block.gox:68
				__e = __c.Any(b.icon); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line block.gox:69
			__e = __c.Any(b.body); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:71
}
