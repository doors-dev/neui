// Managed by GoX v0.4.0

//line icons.gox:1
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

package icons

import (
	"context"
	
	"github.com/doors-dev/gox"
)

var svgAttrs = gox.ModifyFunc(func(ctx context.Context, tag string, attrs gox.Attrs) error {
	attrs.Get("xmlns").Set("http://www.w3.org/2000/svg")
	attrs.Get("fill").Set("currentColor")
	attrs.Get("viewBox").Set("0 0 256 256")
	return nil
})

//line icons.gox:30
func Submit() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:31
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:31
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:31
				__e = __c.Set("d", "M200,32V176a8,8,0,0,1-8,8H67.31l34.35,34.34a8,8,0,0,1-11.32,11.32l-48-48a8,8,0,0,1,0-11.32l48-48a8,8,0,0,1,11.32,11.32L67.31,168H184V32a8,8,0,0,1,16,0Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:32
}

//line icons.gox:34
func Clock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:35
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:35
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:35
				__e = __c.Set("d", "M232,136.66A104.12,104.12,0,1,1,119.34,24,8,8,0,0,1,120.66,40,88.12,88.12,0,1,0,216,135.34,8,8,0,0,1,232,136.66ZM120,72v56a8,8,0,0,0,8,8h56a8,8,0,0,0,0-16H136V72a8,8,0,0,0-16,0Zm40-24a12,12,0,1,0-12-12A12,12,0,0,0,160,48Zm36,24a12,12,0,1,0-12-12A12,12,0,0,0,196,72Zm24,36a12,12,0,1,0-12-12A12,12,0,0,0,220,108Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:36
}

//line icons.gox:38
func Pencil() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:39
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:39
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:39
				__e = __c.Set("d", "M227.31,73.37,182.63,28.68a16,16,0,0,0-22.63,0L36.69,152A15.86,15.86,0,0,0,32,163.31V208a16,16,0,0,0,16,16H92.69A15.86,15.86,0,0,0,104,219.31L227.31,96a16,16,0,0,0,0-22.63ZM92.69,208H48V163.31l88-88L180.69,120ZM192,108.68,147.31,64l24-24L216,84.68Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:40
}

//line icons.gox:42
func Fork() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:43
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:43
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:43
				__e = __c.Set("d", "M229.66,189.66l-32,32a8,8,0,0,1-11.32,0l-32-32a8,8,0,0,1,11.32-11.32L184,196.69V139.31l-56-56-56,56v57.38l18.34-18.35a8,8,0,0,1,11.32,11.32l-32,32a8,8,0,0,1-11.32,0l-32-32a8,8,0,0,1,11.32-11.32L56,196.69V136a8,8,0,0,1,2.34-5.66L120,68.69V24a8,8,0,0,1,16,0V68.69l61.66,61.65A8,8,0,0,1,200,136v60.69l18.34-18.35a8,8,0,0,1,11.32,11.32Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:44
}

//line icons.gox:46
func Attach() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:47
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:47
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:47
				__e = __c.Set("d", "M209.66,122.34a8,8,0,0,1,0,11.32l-82.05,82a56,56,0,0,1-79.2-79.21L147.67,35.73a40,40,0,1,1,56.61,56.55L105,193A24,24,0,1,1,71,159L154.3,74.38A8,8,0,1,1,165.7,85.6L82.39,170.31a8,8,0,1,0,11.27,11.36L192.93,81A24,24,0,1,0,159,47L59.76,147.68a40,40,0,1,0,56.53,56.62l82.06-82A8,8,0,0,1,209.66,122.34Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:48
}

//line icons.gox:50
func Copy() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:51
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:51
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:51
				__e = __c.Set("d", "M216,32H88a8,8,0,0,0-8,8V80H40a8,8,0,0,0-8,8V216a8,8,0,0,0,8,8H168a8,8,0,0,0,8-8V176h40a8,8,0,0,0,8-8V40A8,8,0,0,0,216,32ZM160,208H48V96H160Zm48-48H176V88a8,8,0,0,0-8-8H96V48H208Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:52
}

//line icons.gox:54
func Rewind() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:55
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:55
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:55
				__e = __c.Set("d", "M232,200a8,8,0,0,1-16,0,88.1,88.1,0,0,0-88-88H51.31l34.35,34.34a8,8,0,0,1-11.32,11.32l-48-48a8,8,0,0,1,0-11.32l48-48A8,8,0,0,1,85.66,61.66L51.31,96H128A104.11,104.11,0,0,1,232,200Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:56
}

//line icons.gox:58
func Retry() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:59
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:59
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:59
				__e = __c.Set("d", "M88,104H40a8,8,0,0,1-8-8V48a8,8,0,0,1,16,0V76.69L62.63,62.06A95.43,95.43,0,0,1,130,33.94h.53a95.36,95.36,0,0,1,67.07,27.33,8,8,0,0,1-11.18,11.44,79.52,79.52,0,0,0-55.89-22.77h-.45A79.56,79.56,0,0,0,73.94,73.37L59.31,88H88a8,8,0,0,1,0,16Zm128,48H168a8,8,0,0,0,0,16h28.69l-14.63,14.63a79.56,79.56,0,0,1-56.13,23.43h-.45a79.52,79.52,0,0,1-55.89-22.77,8,8,0,1,0-11.18,11.44,95.36,95.36,0,0,0,67.07,27.33H126a95.43,95.43,0,0,0,67.36-28.12L208,179.31V208a8,8,0,0,0,16,0V160A8,8,0,0,0,216,152Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:60
}

//line icons.gox:62
func Warning() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:63
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:63
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:63
				__e = __c.Set("d", "M236.8,188.09,149.35,36.22h0a24.76,24.76,0,0,0-42.7,0L19.2,188.09a23.51,23.51,0,0,0,0,23.72A24.35,24.35,0,0,0,40.55,224h174.9a24.35,24.35,0,0,0,21.33-12.19A23.51,23.51,0,0,0,236.8,188.09ZM222.93,203.8a8.5,8.5,0,0,1-7.48,4.2H40.55a8.5,8.5,0,0,1-7.48-4.2,7.59,7.59,0,0,1,0-7.72L120.52,44.21a8.75,8.75,0,0,1,15,0l87.45,151.87A7.59,7.59,0,0,1,222.93,203.8ZM120,144V104a8,8,0,0,1,16,0v40a8,8,0,0,1-16,0Zm20,36a12,12,0,1,1-12-12A12,12,0,0,1,140,180Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:64
}

//line icons.gox:66
func WarningFill() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:67
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:67
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:67
				__e = __c.Set("d", "M236.8,188.09,149.35,36.22a24.76,24.76,0,0,0-42.7,0L19.2,188.09a23.51,23.51,0,0,0,0,23.72A24.35,24.35,0,0,0,40.55,224h174.9a24.35,24.35,0,0,0,21.33-12.19A23.51,23.51,0,0,0,236.8,188.09ZM120,104a8,8,0,0,1,16,0v40a8,8,0,0,1-16,0Zm8,88a12,12,0,1,1,12-12A12,12,0,0,1,128,192Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:68
}

//line icons.gox:70
func Stop() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:71
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:71
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:71
				__e = __c.Set("d", "M200,36H56A20,20,0,0,0,36,56V200a20,20,0,0,0,20,20H200a20,20,0,0,0,20-20V56A20,20,0,0,0,200,36Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:72
}

//line icons.gox:74
func Square() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:75
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:75
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:75
				__e = __c.Set("d", "M208,32H48A16,16,0,0,0,32,48V208a16,16,0,0,0,16,16H208a16,16,0,0,0,16-16V48A16,16,0,0,0,208,32Zm0,176H48V48H208V208Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:76
}

//line icons.gox:78
func Prohibit() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:79
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:79
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:79
				__e = __c.Set("d", "M128,24A104,104,0,1,0,232,128,104.11,104.11,0,0,0,128,24ZM40,128a87.61,87.61,0,0,1,20.41-56.28L184.28,195.59A87.85,87.85,0,0,1,40,128Zm155.59,56.28L71.72,60.41a88,88,0,0,1,123.87,123.87Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:80
}

//line icons.gox:82
func Check() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:83
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:83
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:83
				__e = __c.Set("d", "M229.66,77.66l-128,128a8,8,0,0,1-11.32,0l-56-56a8,8,0,0,1,11.32-11.32L96,188.69,218.34,66.34a8,8,0,0,1,11.32,11.32Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:84
}

//line icons.gox:86
func Terminal() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:87
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:87
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:87
				__e = __c.Set("d", "M117.31,134l-72,64a8,8,0,1,1-10.63-12L100,128,34.69,70A8,8,0,1,1,45.32,58l72,64a8,8,0,0,1,0,12ZM216,184H120a8,8,0,0,0,0,16h96a8,8,0,0,0,0-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:88
}

//line icons.gox:90
func Gear() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:91
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:91
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:91
				__e = __c.Set("d", "M128,80a48,48,0,1,0,48,48A48.05,48.05,0,0,0,128,80Zm0,80a32,32,0,1,1,32-32A32,32,0,0,1,128,160Zm88-29.84q.06-2.16,0-4.32l14.92-18.64a8,8,0,0,0,1.48-7.06,107.21,107.21,0,0,0-10.88-26.25,8,8,0,0,0-6-3.93l-23.72-2.64q-1.48-1.56-3-3L186,40.54a8,8,0,0,0-3.94-6,107.71,107.71,0,0,0-26.25-10.87,8,8,0,0,0-7.06,1.49L130.16,40Q128,40,125.84,40L107.2,25.11a8,8,0,0,0-7.06-1.48A107.6,107.6,0,0,0,73.89,34.51a8,8,0,0,0-3.93,6L67.32,64.27q-1.56,1.49-3,3L40.54,70a8,8,0,0,0-6,3.94,107.71,107.71,0,0,0-10.87,26.25,8,8,0,0,0,1.49,7.06L40,125.84Q40,128,40,130.16L25.11,148.8a8,8,0,0,0-1.48,7.06,107.21,107.21,0,0,0,10.88,26.25,8,8,0,0,0,6,3.93l23.72,2.64q1.49,1.56,3,3L70,215.46a8,8,0,0,0,3.94,6,107.71,107.71,0,0,0,26.25,10.87,8,8,0,0,0,7.06-1.49L125.84,216q2.16.06,4.32,0l18.64,14.92a8,8,0,0,0,7.06,1.48,107.21,107.21,0,0,0,26.25-10.88,8,8,0,0,0,3.93-6l2.64-23.72q1.56-1.48,3-3L215.46,186a8,8,0,0,0,6-3.94,107.71,107.71,0,0,0,10.87-26.25,8,8,0,0,0-1.49-7.06Zm-16.1-6.5a73.93,73.93,0,0,1,0,8.68,8,8,0,0,0,1.74,5.48l14.19,17.73a91.57,91.57,0,0,1-6.23,15L187,173.11a8,8,0,0,0-5.1,2.64,74.11,74.11,0,0,1-6.14,6.14,8,8,0,0,0-2.64,5.1l-2.51,22.58a91.32,91.32,0,0,1-15,6.23l-17.74-14.19a8,8,0,0,0-5-1.75h-.48a73.93,73.93,0,0,1-8.68,0,8,8,0,0,0-5.48,1.74L100.45,215.8a91.57,91.57,0,0,1-15-6.23L82.89,187a8,8,0,0,0-2.64-5.1,74.11,74.11,0,0,1-6.14-6.14,8,8,0,0,0-5.1-2.64L46.43,170.6a91.32,91.32,0,0,1-6.23-15l14.19-17.74a8,8,0,0,0,1.74-5.48,73.93,73.93,0,0,1,0-8.68,8,8,0,0,0-1.74-5.48L40.2,100.45a91.57,91.57,0,0,1,6.23-15L69,82.89a8,8,0,0,0,5.1-2.64,74.11,74.11,0,0,1,6.14-6.14A8,8,0,0,0,82.89,69L85.4,46.43a91.32,91.32,0,0,1,15-6.23l17.74,14.19a8,8,0,0,0,5.48,1.74,73.93,73.93,0,0,1,8.68,0,8,8,0,0,0,5.48-1.74L155.55,40.2a91.57,91.57,0,0,1,15,6.23L173.11,69a8,8,0,0,0,2.64,5.1,74.11,74.11,0,0,1,6.14,6.14,8,8,0,0,0,5.1,2.64l22.58,2.51a91.32,91.32,0,0,1,6.23,15l-14.19,17.74A8,8,0,0,0,199.87,123.66Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:92
}

//line icons.gox:94
func Lightbulb() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:95
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:95
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:95
				__e = __c.Set("d", "M176,232a8,8,0,0,1-8,8H88a8,8,0,0,1,0-16h80A8,8,0,0,1,176,232Zm40-128a87.55,87.55,0,0,1-33.64,69.21A16.24,16.24,0,0,0,176,186v6a16,16,0,0,1-16,16H96a16,16,0,0,1-16-16v-6a16,16,0,0,0-6.23-12.66A87.59,87.59,0,0,1,40,104.49C39.74,56.83,78.26,17.14,125.88,16A88,88,0,0,1,216,104Zm-16,0a72,72,0,0,0-73.74-72c-39,.92-70.47,33.39-70.26,72.39a71.65,71.65,0,0,0,27.64,56.3A32,32,0,0,1,96,186v6h64v-6a32.15,32.15,0,0,1,12.47-25.35A71.65,71.65,0,0,0,200,104Zm-16.11-9.34a57.6,57.6,0,0,0-46.56-46.55,8,8,0,0,0-2.66,15.78c16.57,2.79,30.63,16.85,33.44,33.45A8,8,0,0,0,176,104a9,9,0,0,0,1.35-.11A8,8,0,0,0,183.89,94.66Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:96
}

//line icons.gox:98
func TextAlignLeft() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:99
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:99
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:99
				__e = __c.Set("d", "M32,64a8,8,0,0,1,8-8H216a8,8,0,0,1,0,16H40A8,8,0,0,1,32,64Zm8,48H168a8,8,0,0,0,0-16H40a8,8,0,0,0,0,16Zm176,24H40a8,8,0,0,0,0,16H216a8,8,0,0,0,0-16Zm-48,40H40a8,8,0,0,0,0,16H168a8,8,0,0,0,0-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:100
}

//line icons.gox:102
func LightbulbFill() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:103
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:103
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:103
				__e = __c.Set("d", "M176,232a8,8,0,0,1-8,8H88a8,8,0,0,1,0-16h80A8,8,0,0,1,176,232Zm40-128a87.55,87.55,0,0,1-33.64,69.21A16.24,16.24,0,0,0,176,186v6a16,16,0,0,1-16,16H96a16,16,0,0,1-16-16v-6a16,16,0,0,0-6.23-12.66A87.59,87.59,0,0,1,40,104.49C39.74,56.83,78.26,17.14,125.88,16A88,88,0,0,1,216,104Zm-32.11-9.34a57.6,57.6,0,0,0-46.56-46.55,8,8,0,0,0-2.66,15.78c16.57,2.79,30.63,16.85,33.44,33.45A8,8,0,0,0,176,104a9,9,0,0,0,1.35-.11A8,8,0,0,0,183.89,94.66Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:104
}

//line icons.gox:106
func TextAlignLeftLines() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:107
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:107
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
//line icons.gox:107
			__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:107
			__e = __c.Set("stroke-width", "16"); if __e != nil { return }
//line icons.gox:107
			__e = __c.Set("stroke-linecap", "round"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:108
				__e = __c.Set("x1", "40"); if __e != nil { return }
//line icons.gox:108
				__e = __c.Set("y1", "64"); if __e != nil { return }
//line icons.gox:108
				__e = __c.Set("x2", "216"); if __e != nil { return }
//line icons.gox:108
				__e = __c.Set("y2", "64"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:109
				__e = __c.Set("x1", "40"); if __e != nil { return }
//line icons.gox:109
				__e = __c.Set("y1", "104"); if __e != nil { return }
//line icons.gox:109
				__e = __c.Set("x2", "168"); if __e != nil { return }
//line icons.gox:109
				__e = __c.Set("y2", "104"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:110
				__e = __c.Set("x1", "40"); if __e != nil { return }
//line icons.gox:110
				__e = __c.Set("y1", "144"); if __e != nil { return }
//line icons.gox:110
				__e = __c.Set("x2", "216"); if __e != nil { return }
//line icons.gox:110
				__e = __c.Set("y2", "144"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:111
				__e = __c.Set("x1", "40"); if __e != nil { return }
//line icons.gox:111
				__e = __c.Set("y1", "184"); if __e != nil { return }
//line icons.gox:111
				__e = __c.Set("x2", "168"); if __e != nil { return }
//line icons.gox:111
				__e = __c.Set("y2", "184"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:113
}

//line icons.gox:115
func Notepad() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:116
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:116
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:116
				__e = __c.Set("d", "M168,128a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,128Zm-8,24H96a8,8,0,0,0,0,16h64a8,8,0,0,0,0-16ZM216,40V200a32,32,0,0,1-32,32H72a32,32,0,0,1-32-32V40a8,8,0,0,1,8-8H72V24a8,8,0,0,1,16,0v8h32V24a8,8,0,0,1,16,0v8h32V24a8,8,0,0,1,16,0v8h24A8,8,0,0,1,216,40Zm-16,8H184v8a8,8,0,0,1-16,0V48H136v8a8,8,0,0,1-16,0V48H88v8a8,8,0,0,1-16,0V48H56V200a16,16,0,0,0,16,16H184a16,16,0,0,0,16-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:117
}

//line icons.gox:119
func NotepadFill() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:120
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:120
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:120
				__e = __c.Set("d", "M208,32H184V24a8,8,0,0,0-16,0v8H136V24a8,8,0,0,0-16,0v8H88V24a8,8,0,0,0-16,0v8H48a8,8,0,0,0-8,8V200a32,32,0,0,0,32,32H184a32,32,0,0,0,32-32V40A8,8,0,0,0,208,32ZM120,56a8,8,0,0,1,16,0v8a8,8,0,0,1-16,0ZM80,72a8,8,0,0,1-8-8V56a8,8,0,0,1,16,0v8A8,8,0,0,1,80,72Zm80,96H96a8,8,0,0,1,0-16h64a8,8,0,0,1,0,16Zm0-32H96a8,8,0,0,1,0-16h64a8,8,0,0,1,0,16Zm24-72a8,8,0,0,1-16,0V56a8,8,0,0,1,16,0Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:121
}

//line icons.gox:123
func Beams() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:124
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:124
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
//line icons.gox:124
			__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:124
			__e = __c.Set("stroke-width", "24"); if __e != nil { return }
//line icons.gox:124
			__e = __c.Set("stroke-linecap", "round"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:125
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:125
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:125
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:125
				__e = __c.Set("y2", "72"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:126
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:126
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:126
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:126
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:126
				__e = __c.Set("transform", "rotate(45 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:127
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:127
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:127
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:127
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:127
				__e = __c.Set("transform", "rotate(90 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:128
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:128
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:128
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:128
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:128
				__e = __c.Set("transform", "rotate(135 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:129
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:129
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:129
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:129
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:129
				__e = __c.Set("transform", "rotate(180 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:130
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:130
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:130
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:130
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:130
				__e = __c.Set("transform", "rotate(225 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:131
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:131
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:131
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:131
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:131
				__e = __c.Set("transform", "rotate(270 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("line"); if __e != nil { return }
			{
//line icons.gox:132
				__e = __c.Set("x1", "128"); if __e != nil { return }
//line icons.gox:132
				__e = __c.Set("y1", "32"); if __e != nil { return }
//line icons.gox:132
				__e = __c.Set("x2", "128"); if __e != nil { return }
//line icons.gox:132
				__e = __c.Set("y2", "72"); if __e != nil { return }
//line icons.gox:132
				__e = __c.Set("transform", "rotate(315 128 128)"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:134
}

//line icons.gox:136
func EnvelopeSimple() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:137
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:137
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:137
				__e = __c.Set("d", "M224,48H32a8,8,0,0,0-8,8V192a16,16,0,0,0,16,16H216a16,16,0,0,0,16-16V56A8,8,0,0,0,224,48ZM203.43,64,128,133.15,52.57,64ZM216,192H40V74.19l82.59,75.71a8,8,0,0,0,10.82,0L216,74.19V192Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:138
}

//line icons.gox:140
func Spinner() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:141
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:141
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:141
				__e = __c.Set("d", "M232,128a104,104,0,0,1-208,0c0-41,23.81-78.36,60.66-95.27a8,8,0,0,1,6.68,14.54C60.15,61.59,40,93,40,128a88,88,0,0,0,176,0c0-35-20.15-66.41-51.34-80.73a8,8,0,0,1,6.68-14.54C208.19,49.64,232,87,232,128Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:142
}

//line icons.gox:144
func File() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:145
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:145
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:145
				__e = __c.Set("d", "M213.66,82.34l-56-56A8,8,0,0,0,152,24H56A16,16,0,0,0,40,40V216a16,16,0,0,0,16,16H200a16,16,0,0,0,16-16V88A8,8,0,0,0,213.66,82.34ZM160,51.31,188.69,80H160ZM200,216H56V40h88V88a8,8,0,0,0,8,8h48V216Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:146
}

//line icons.gox:148
func Close() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:149
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:149
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:149
				__e = __c.Set("d", "M205.66,194.34a8,8,0,0,1-11.32,11.32L128,139.31,61.66,205.66a8,8,0,0,1-11.32-11.32L116.69,128,50.34,61.66A8,8,0,0,1,61.66,50.34L128,116.69l66.34-66.35a8,8,0,0,1,11.32,11.32L139.31,128Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:150
}

//line icons.gox:152
func Minus() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:153
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:153
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:153
				__e = __c.Set("d", "M228,128a12,12,0,0,1-12,12H40a12,12,0,0,1,0-24H216A12,12,0,0,1,228,128Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:154
}

//line icons.gox:156
func FileImage() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:157
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:157
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:157
				__e = __c.Set("d", "M216,40H40A16,16,0,0,0,24,56V200a16,16,0,0,0,16,16H216a16,16,0,0,0,16-16V56A16,16,0,0,0,216,40Zm0,16V158.75l-26.07-26.06a16,16,0,0,0-22.63,0l-20,20-44-44a16,16,0,0,0-22.62,0L40,149.37V56ZM40,172l52-52,80,80H40Zm176,28H194.63l-36-36,20-20L216,181.38V200ZM144,100a12,12,0,1,1,12,12A12,12,0,0,1,144,100Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:158
}

//line icons.gox:160
func FileText() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:161
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:161
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:161
				__e = __c.Set("d", "M213.66,82.34l-56-56A8,8,0,0,0,152,24H56A16,16,0,0,0,40,40V216a16,16,0,0,0,16,16H200a16,16,0,0,0,16-16V88A8,8,0,0,0,213.66,82.34ZM160,51.31,188.69,80H160ZM200,216H56V40h88V88a8,8,0,0,0,8,8h48V216Zm-32-80a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,136Zm0,32a8,8,0,0,1-8,8H96a8,8,0,0,1,0-16h64A8,8,0,0,1,168,168Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:162
}

//line icons.gox:164
func FileArchive() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:165
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:165
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:165
				__e = __c.Set("d", "M213.66,82.34l-56-56A8,8,0,0,0,152,24H56A16,16,0,0,0,40,40V216a16,16,0,0,0,16,16H200a16,16,0,0,0,16-16V88A8,8,0,0,0,213.66,82.34ZM160,51.31,188.69,80H160ZM200,216H56V40h32v8a8,8,0,0,0,16,0V40h40V88a8,8,0,0,0,8,8h48V216ZM96,64v8a8,8,0,0,0,16,0V64a8,8,0,0,0-16,0Zm16,40a8,8,0,0,0-16,0v8a8,8,0,0,0,16,0Zm0,32v32a8,8,0,0,0,16,0V136a8,8,0,0,0-16,0Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:166
}

//line icons.gox:168
func Files() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:169
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:169
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:169
				__e = __c.Set("d", "M213.66,66.34l-40-40A8,8,0,0,0,168,24H88A16,16,0,0,0,72,40V56H56A16,16,0,0,0,40,72V216a16,16,0,0,0,16,16H168a16,16,0,0,0,16-16V200h16a16,16,0,0,0,16-16V72A8,8,0,0,0,213.66,66.34ZM168,216H56V72h76.69L168,107.31v84.53c0,.06,0,.11,0,.16s0,.1,0,.16V216Zm32-32H184V104a8,8,0,0,0-2.34-5.66l-40-40A8,8,0,0,0,136,56H88V40h76.69L200,75.31Zm-56-32a8,8,0,0,1-8,8H88a8,8,0,0,1,0-16h48A8,8,0,0,1,144,152Zm0,32a8,8,0,0,1-8,8H88a8,8,0,0,1,0-16h48A8,8,0,0,1,144,184Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:170
}

//line icons.gox:172
func Plus() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:173
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:173
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:173
				__e = __c.Set("d", "M224,128a8,8,0,0,1-8,8H136v80a8,8,0,0,1-16,0V136H40a8,8,0,0,1,0-16h80V40a8,8,0,0,1,16,0v80h80A8,8,0,0,1,224,128Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:174
}

//line icons.gox:176
func ArrowLeft() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:177
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:177
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:177
				__e = __c.Set("d", "M224,128a8,8,0,0,1-8,8H59.31l58.35,58.34a8,8,0,0,1-11.32,11.32l-72-72a8,8,0,0,1,0-11.32l72-72a8,8,0,0,1,11.32,11.32L59.31,120H216A8,8,0,0,1,224,128Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:178
}

//line icons.gox:180
func UploadSimple() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:181
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:181
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:181
				__e = __c.Set("d", "M74.34,85.66a8,8,0,0,1,0-11.32l48-48a8,8,0,0,1,11.32,0l48,48a8,8,0,0,1-11.32,11.32L136,51.31V144a8,8,0,0,1-16,0V51.31L85.66,85.66A8,8,0,0,1,74.34,85.66ZM224,144a8,8,0,0,0-8,8v56H40V152a8,8,0,0,0-16,0v56a16,16,0,0,0,16,16H216a16,16,0,0,0,16-16V152A8,8,0,0,0,224,144Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:182
}

//line icons.gox:184
func DownloadSimple() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:185
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:185
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:185
				__e = __c.Set("d", "M74.34,133.66a8,8,0,0,1,11.32-11.32L120,156.69V40a8,8,0,0,1,16,0V156.69l34.34-34.35a8,8,0,0,1,11.32,11.32l-48,48a8,8,0,0,1-11.32,0ZM240,152a8,8,0,0,0-8,8v48H24V160a8,8,0,0,0-16,0v48a16,16,0,0,0,16,16H232a16,16,0,0,0,16-16V160A8,8,0,0,0,240,152Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:186
}

//line icons.gox:188
func Trash() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:189
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:189
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:189
				__e = __c.Set("d", "M216,48H176V40a24,24,0,0,0-24-24H104A24,24,0,0,0,80,40v8H40a8,8,0,0,0,0,16h8V208a16,16,0,0,0,16,16H192a16,16,0,0,0,16-16V64h8a8,8,0,0,0,0-16ZM96,40a8,8,0,0,1,8-8h48a8,8,0,0,1,8,8v8H96Zm96,168H64V64H192ZM112,104v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Zm48,0v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:190
}

//line icons.gox:192
func DotsThreeVertical() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:193
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:193
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:193
				__e = __c.Set("d", "M140,128a12,12,0,1,1-12-12A12,12,0,0,1,140,128ZM128,76a12,12,0,1,0-12-12A12,12,0,0,0,128,76Zm0,104a12,12,0,1,0,12,12A12,12,0,0,0,128,180Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:194
}

//line icons.gox:196
func ArrowRight() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:197
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:197
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:197
				__e = __c.Set("d", "M221.66,133.66l-72,72a8,8,0,0,1-11.32-11.32L196.69,136H40a8,8,0,0,1,0-16H196.69L138.34,61.66a8,8,0,0,1,11.32-11.32l72,72A8,8,0,0,1,221.66,133.66Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:198
}

//line icons.gox:200
func MagnifyingGlass() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:201
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:201
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:201
				__e = __c.Set("d", "M229.66,218.34l-50.07-50.06a88.11,88.11,0,1,0-11.31,11.31l50.06,50.07a8,8,0,0,0,11.32-11.32ZM40,112a72,72,0,1,1,72,72A72.08,72.08,0,0,1,40,112Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:202
}

//line icons.gox:204
func List() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:205
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:205
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:205
				__e = __c.Set("d", "M224,128a8,8,0,0,1-8,8H40a8,8,0,0,1,0-16H216A8,8,0,0,1,224,128ZM40,72H216a8,8,0,0,0,0-16H40a8,8,0,0,0,0,16ZM216,184H40a8,8,0,0,0,0,16H216a8,8,0,0,0,0-16Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:206
}

//line icons.gox:208
func CheckSquareFill() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:209
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:209
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:209
				__e = __c.Set("d", "M208,32H48A16,16,0,0,0,32,48V208a16,16,0,0,0,16,16H208a16,16,0,0,0,16-16V48A16,16,0,0,0,208,32Zm-34.34,77.66-56,56a8,8,0,0,1-11.32,0l-24-24a8,8,0,0,1,11.32-11.32L112,148.69l50.34-50.35a8,8,0,0,1,11.32,11.32Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:210
}

//line icons.gox:212
func ChatCircle() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:213
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:213
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:213
				__e = __c.Set("d", "M128,24A104,104,0,0,0,36.18,176.88L24.83,210.93a16,16,0,0,0,20.24,20.24l34.05-11.35A104,104,0,1,0,128,24Zm0,192a87.87,87.87,0,0,1-44.06-11.81,8,8,0,0,0-6.54-.67L40,216,52.47,178.6a8,8,0,0,0-.66-6.54A88,88,0,1,1,128,216Z"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:214
}

//line icons.gox:216
func DockRight() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:217
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:217
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:217
				__e = __c.Set("x", "28"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("width", "200"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("rx", "16"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("fill", "none"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("stroke-width", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:217
				__e = __c.Set("x", "152"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("width", "76"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:217
				__e = __c.Set("rx", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:218
}

//line icons.gox:220
func DockLeft() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:221
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:221
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:221
				__e = __c.Set("x", "28"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("width", "200"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("rx", "16"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("fill", "none"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("stroke-width", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:221
				__e = __c.Set("x", "28"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("width", "76"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:221
				__e = __c.Set("rx", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:222
}

//line icons.gox:224
func Undock() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:225
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:225
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:225
				__e = __c.Set("x", "28"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("width", "200"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("rx", "16"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("fill", "none"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("stroke-width", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:225
				__e = __c.Set("x", "124"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("y", "116"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("width", "80"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("height", "72"); if __e != nil { return }
//line icons.gox:225
				__e = __c.Set("rx", "8"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:226
}

//line icons.gox:228
func UndockLeft() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:229
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:229
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:229
				__e = __c.Set("x", "28"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("y", "44"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("width", "200"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("height", "168"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("rx", "16"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("fill", "none"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("stroke-width", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("rect"); if __e != nil { return }
			{
//line icons.gox:229
				__e = __c.Set("x", "52"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("y", "116"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("width", "80"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("height", "72"); if __e != nil { return }
//line icons.gox:229
				__e = __c.Set("rx", "8"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:230
}

//line icons.gox:232
func Exclamation() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("svg"); if __e != nil { return }
		{
//line icons.gox:233
			__e = __c.Modify(svgAttrs); if __e != nil { return }
//line icons.gox:233
			__e = __c.Set("viewBox", "0 0 256 256"); if __e != nil { return }
//line icons.gox:233
			__e = __c.Set("fill", "none"); if __e != nil { return }
//line icons.gox:233
			__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line icons.gox:233
			__e = __c.Set("stroke-width", "32"); if __e != nil { return }
//line icons.gox:233
			__e = __c.Set("stroke-linecap", "round"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("path"); if __e != nil { return }
			{
//line icons.gox:233
				__e = __c.Set("d", "M128 56v92"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("circle"); if __e != nil { return }
			{
//line icons.gox:233
				__e = __c.Set("cx", "128"); if __e != nil { return }
//line icons.gox:233
				__e = __c.Set("cy", "200"); if __e != nil { return }
//line icons.gox:233
				__e = __c.Set("r", "6"); if __e != nil { return }
//line icons.gox:233
				__e = __c.Set("fill", "currentColor"); if __e != nil { return }
//line icons.gox:233
				__e = __c.Set("stroke-width", "16"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line icons.gox:234
}
