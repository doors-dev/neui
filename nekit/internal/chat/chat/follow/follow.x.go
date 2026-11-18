// Managed by GoX v0.4.0

//line follow.gox:1
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

package follow

import (
	"github.com/doors-dev/gox"
)

//line follow.gox:21
func Mark() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line follow.gox:22
			__e = __c.Set("class", "nechat-follow"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const mark = document.currentScript.parentElement\n\t\t\tconst log = mark.closest(\".nechat-log\")\n\t\t\tif (!log) return\n\t\t\tconst shown = mark.getClientRects().length !== 0\n\t\t\tlet end = mark.getBoundingClientRect().top\n\t\t\tfor (const span of log.querySelectorAll(\".nechat-span\")) {\n\t\t\t\tend = Math.max(end, span.getBoundingClientRect().bottom)\n\t\t\t}\n\t\t\tconst below = end - log.getBoundingClientRect().bottom\n\t\t\tmark.remove()\n\t\t\tif (!shown || below > 48) return\n\t\t\trequestAnimationFrame(() => log.scrollTo({ top: log.scrollHeight, behavior: \"smooth\" }))"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line follow.gox:38
}
