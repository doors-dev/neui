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
	"encoding/binary"
	"slices"

	"github.com/zeebo/xxh3"
)

func (s *space) fresh() ([4]uint64, uint64) {
	var sets [5][]uint64
	for _, t := range s.tools {
		sets[0] = append(sets[0], t.hash)
	}
	for _, in := range s.list {
		sets[1] = append(sets[1], in.hash)
	}
	for _, d := range s.data {
		sets[2] = append(sets[2], d.hash)
	}
	for _, c := range s.children {
		sets[3] = append(sets[3], xxh3.HashString(c.name))
		if c.island == nil {
			sets[4] = append(sets[4], xxh3.HashString(c.name))
		}
	}
	var out [5]uint64
	for i, set := range sets {
		if len(set) == 0 {
			continue
		}
		slices.Sort(set)
		h := xxh3.New()
		for _, hash := range slices.Compact(set) {
			h.Write(binary.LittleEndian.AppendUint64(nil, hash))
		}
		out[i] = h.Sum64()
	}
	return [4]uint64(out[:4]), out[4]
}

func StaleDigests(s Space) []string {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()
	var stale []string
	var walk func(s *space)
	walk = func(s *space) {
		if digest, plain := s.fresh(); digest != s.digest || plain != s.plain {
			stale = append(stale, s.path)
		}
		for _, c := range s.children {
			walk(c)
		}
	}
	walk(s.root.space)
	return stale
}
