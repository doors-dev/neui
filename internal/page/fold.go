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

package page

import (
	"slices"
	"strings"
)

var groups = [3]string{"tools", "inputs", "data"}

func (s *space) changes(before map[string][4]uint64, called string) []change {
	var out []change
	live := map[string]bool{}
	var walk func(s *space)
	walk = func(s *space) {
		live[s.path] = true
		old, ok := before[s.path]
		if !ok {
			if parent(s.path) != called {
				out = append(out, change{Path: s.path, Added: true, Desc: s.desc})
			}
			return
		}
		if s.path != called {
			c := change{Path: s.path}
			for i, group := range groups {
				if s.digest[i] != old[i] {
					c.Changed = append(c.Changed, group)
				}
			}
			if c.Changed != nil {
				out = append(out, c)
			}
		}
		for _, c := range s.children {
			walk(c)
		}
	}
	walk(s)
	for path := range before {
		if !live[path] && path != called && live[parent(path)] && parent(path) != called {
			out = append(out, change{Path: path, Removed: true})
		}
	}
	slices.SortFunc(out, func(a, b change) int {
		return slices.Compare(segs(a.Path), segs(b.Path))
	})
	if len(out) <= 50 {
		return out
	}
	units := make([]unit, len(out))
	var added, rest []unit
	for i, c := range out {
		units[i] = unit{change: c, own: true, n: 1}
		if c.Added {
			added = append(added, units[i])
		} else {
			rest = append(rest, units[i])
		}
	}
	if len(added) > 25 {
		added, rest = nil, units
	}
	units = append(fold(rest, 50-len(added)), added...)
	slices.SortFunc(units, func(a, b unit) int {
		return slices.Compare(segs(a.Path), segs(b.Path))
	})
	out = out[:0]
	for _, u := range units {
		out = append(out, u.change)
	}
	return out
}

type unit struct {
	change
	own bool
	n   int
}

func fold(u []unit, limit int) []unit {
	cp := make([]int, len(u)+1)
	cp[0], cp[len(u)] = -1, -1
	for i := 1; i < len(u); i++ {
		cp[i] = shared(u[i-1].Path, u[i].Path)
	}
	for len(u) > limit {
		var runs [][3]int
		size := 0
		for a := 0; a < len(u)-1; {
			c, b := cp[a+1], a+1
			for cp[b+1] == c {
				b++
			}
			if cp[a] < c && cp[b+1] < c {
				runs = append(runs, [3]int{a, b, c})
				size = slices.Max([]int{size, b - a + 1})
			}
			a = b
		}
		out, right := []unit{}, []int{-1}
		next := 0
		for _, r := range runs {
			if r[1]-r[0]+1 < size {
				continue
			}
			out = append(out, u[next:r[0]]...)
			right = append(right, cp[next+1:r[0]+1]...)
			out = append(out, join(strings.Join(segs(u[r[0]].Path)[:r[2]], "."), u[r[0]:r[1]+1]))
			right = append(right, cp[r[1]+1])
			next = r[1] + 1
		}
		u, cp = append(out, u[next:]...), append(right, cp[next+1:]...)
	}
	return u
}

func join(path string, members []unit) unit {
	out := unit{change: change{Path: path}}
	for _, m := range members {
		out.n += m.n
		if m.Path == path && (m.own || m.Spaces > 0) {
			out.own = m.own
			out.Changed = slices.Clone(m.Changed)
		}
	}
	if !slices.Contains(out.Changed, "children") {
		out.Changed = append(out.Changed, "children")
	}
	out.Spaces = out.n
	if out.own {
		out.Spaces--
	}
	return out
}

func segs(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func shared(a, b string) int {
	x, y := segs(a), segs(b)
	n := 0
	for n < len(x) && n < len(y) && x[n] == y[n] {
		n++
	}
	return n
}

func parent(path string) string {
	return path[:max(strings.LastIndexByte(path, '.'), 0)]
}
