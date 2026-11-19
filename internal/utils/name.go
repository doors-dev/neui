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

package utils

import (
	"strconv"
	"strings"

	"github.com/zeebo/xxh3"
)

func SanitizeName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	pending := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			if pending && b.Len() > 0 {
				b.WriteByte('_')
			}
			pending = false
			b.WriteRune(r)
		default:
			pending = true
		}
	}
	s := b.String()
	if s == "" {
		return "x" + nameHash(name)
	}
	if len(s) > 31 {
		return strings.TrimRight(s[:24], "_") + "_" + nameHash(s[24:])
	}
	return s
}

func nameHash(s string) string {
	h := strconv.FormatUint(xxh3.HashString(s)&0xffffff, 16)
	return strings.Repeat("0", 6-len(h)) + h
}
