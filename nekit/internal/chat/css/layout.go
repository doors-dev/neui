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

package css

import (
	"bytes"
	"regexp"
	"strconv"

	"github.com/zeebo/xxh3"
)

type Side uint8

const (
	Right Side = iota
	Left
)

type Layout struct {
	// Side is the page edge the chat sits at; the zero value is the right edge.
	Side Side
	// Width is the floating window's width; default 25rem.
	Width string
	// Height is the floating window's height; default 40rem.
	Height string
	// DockWidth is the docked panel's width, which the page makes room for; default 28rem.
	DockWidth string
}

var length = regexp.MustCompile(`^(\d+|\d*\.\d+)(px|rem|em|vw|vh|%)$`)

func (l Layout) valid() Layout {
	for _, v := range []*string{&l.Width, &l.Height, &l.DockWidth} {
		if !length.MatchString(*v) {
			*v = ""
		}
	}
	if l.Side != Left {
		l.Side = Right
	}
	return l
}

func LayoutKey(l Layout) string {
	l = l.valid()
	if l == (Layout{}) {
		return ""
	}
	return strconv.FormatUint(xxh3.HashString(strconv.Itoa(int(l.Side))+"|"+l.Width+"|"+l.Height+"|"+l.DockWidth), 36)
}

func (l Layout) write(b *bytes.Buffer, key string) {
	l = l.valid()
	widget := `.nechat-widget[data-nechat-layout="` + key + `"]`
	left := l.Side == Left
	b.WriteString("@media (width >= 40rem){")
	if left || l.Width != "" || l.Height != "" {
		b.WriteString(widget)
		b.WriteString(`[data-mode]:not([data-mode="dock"]){`)
		if left {
			b.WriteString("right:auto;left:1rem;transform-origin:bottom left;")
		}
		if l.Width != "" {
			b.WriteString("width:" + l.Width + ";")
		}
		if l.Height != "" {
			b.WriteString("height:" + l.Height + ";")
		}
		b.WriteString("}")
	}
	if left || l.DockWidth != "" {
		b.WriteString(widget)
		b.WriteString(`[data-mode="dock"]{`)
		if left {
			b.WriteString("right:auto;left:0;border-inline-start-width:0;border-inline-end-width:1px;")
		}
		if l.DockWidth != "" {
			b.WriteString("width:" + l.DockWidth + ";")
		}
		b.WriteString("}")
		if left {
			b.WriteString("@starting-style{" + widget + `[data-mode="dock"]{translate:-100% 0}}`)
		}
		dock := l.DockWidth
		if dock == "" {
			dock = "28rem"
		}
		b.WriteString("html:has(" + widget + `[data-mode="dock"]){`)
		if left {
			b.WriteString("padding-right:0;padding-left:" + dock + ";")
		} else {
			b.WriteString("padding-right:" + dock + ";")
		}
		b.WriteString("}")
	}
	if left {
		b.WriteString(widget + " > .nechat-launcher{right:auto;left:0}")
	}
	b.WriteString("}")
	if left {
		b.WriteString("@media (width < 40rem){" + widget + " > .nechat-launcher{right:auto;left:1rem}}")
	}
}
