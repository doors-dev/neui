package page

import (
	"encoding/binary"
	"slices"

	"github.com/zeebo/xxh3"
)

func (s *space) fresh() [4]uint64 {
	var sets [4][]uint64
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
	}
	var out [4]uint64
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
	return out
}

func StaleDigests(s Space) []string {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()
	var stale []string
	var walk func(s *space)
	walk = func(s *space) {
		if s.fresh() != s.digest {
			stale = append(stale, s.path)
		}
		for _, c := range s.children {
			walk(c)
		}
	}
	walk(s.root.space)
	return stale
}
