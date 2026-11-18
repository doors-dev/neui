// Managed by GoX v0.4.0

//line block.gox:1
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
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/nekit/internal/chat/icons"
)

func newSlot(running gox.Elem) *doors.Door {
	slot := new(doors.Door)
	slot.Outer(context.Background(), running)
	return slot
}

//line block.gox:31
func notepadLines() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line block.gox:32
			__e = __c.Set("xmlns", "http://www.w3.org/2000/svg"); if __e != nil { return }
//line block.gox:32
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
//line block.gox:32
			__e = __c.Set("fill", "currentColor"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:33
				__e = __c.Set("d", "M212,64V208a16,16,0,0,1-16,16H56a16,16,0,0,1-16-16V64A16,16,0,0,1,56,48H88V40a8,8,0,0,1,16,0v8h48V40a8,8,0,0,1,16,0v8h32A16,16,0,0,1,216,64ZM56,64V208H200V64H168v8a8,8,0,0,1-16,0V64H104v8a8,8,0,0,1-16,0V64Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:34
				__e = __c.Set("class", "line"); if __e != nil { return }
//line block.gox:34
				__e = __c.Set("d", "M168,112a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,112Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line block.gox:35
				__e = __c.Set("class", "line"); if __e != nil { return }
//line block.gox:35
				__e = __c.Set("d", "M160,136H96a8,8,0,0,0,0,16h64a8,8,0,0,0,0-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:37
}

//line block.gox:39
func textRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:40
		__e = (doors.Class("nechat-ic nechat-ic_line nechat-build")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:40
			__e = __c.Any(icons.TextAlignLeftLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:41
}

//line block.gox:43
func textDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:44
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:44
			__e = __c.Any(icons.TextAlignLeft()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:45
}

//line block.gox:47
func thinkingRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line block.gox:48
			__e = __c.Set("class", "nechat-blink"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line block.gox:49
			__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line block.gox:49
				__e = __c.Any(icons.Lightbulb()); if __e != nil { return }
			return })); if __e != nil { return }
//line block.gox:50
			__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line block.gox:50
				__e = __c.Any(icons.LightbulbFill()); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:52
}

//line block.gox:54
func thinkingDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:55
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:55
			__e = __c.Any(icons.Lightbulb()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:56
}

//line block.gox:58
func summaryRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:59
		__e = (doors.Class("nechat-ic nechat-pulse")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:59
			__e = __c.Any(notepadLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:60
}

//line block.gox:62
func summaryDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:63
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:63
			__e = __c.Any(notepadLines()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:64
}

//line block.gox:66
func toolsRunning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:67
		__e = (doors.Class("nechat-ic nechat-ic_spin")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:67
			__e = __c.Any(icons.Gear()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:68
}

//line block.gox:70
func toolsDone() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line block.gox:71
		__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line block.gox:71
			__e = __c.Any(icons.Terminal()); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line block.gox:72
}

type block struct {
	title string
	icon any
	body any
}

//line block.gox:80
func (b block) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line block.gox:81
			__e = __c.Set("class", "nechat-blk"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line block.gox:82
				__e = __c.Set("class", "nechat-gut"); if __e != nil { return }
//line block.gox:82
				__e = __c.Set("title", b.title); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line block.gox:82
				__e = __c.Any(b.icon); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line block.gox:83
			__e = __c.Any(b.body); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line block.gox:85
}
