package node

import (
	"sort"
)

const (
	alpha = 3
)

// An entry in the shortlist
type slEntry struct {
	c   Contact
	d   [20]byte // XOR distance to target
	ask bool     // already queried?
}

type shortlist struct {
	target [20]byte
	k      int
	max    int
	idx    map[[20]byte]int // ID -> index in 'list'
	list   []slEntry
}

// creates a new shortlist for given target and k
func newShortlist(target [20]byte, k int) *shortlist {
	return &shortlist{
		target: target,
		k:      k,
		idx:    make(map[[20]byte]int),
		list:   make([]slEntry, 0, k*2),
		max:    k,
	}
}

// add merges contacts, dedups by ID, resorts by XOR, truncates to k.
func (s *shortlist) add(cs []Contact) (changed bool) {
	oldBest := s.best()
	for _, c := range cs {
		if _, ok := s.idx[c.ID]; ok {
			continue
		}
		s.list = append(s.list, slEntry{c: c, d: xor(s.target, c.ID)})
		changed = true
	}
	if changed {
		sort.Slice(s.list, func(i, j int) bool { return less160(s.list[i].d, s.list[j].d) })
		if len(s.list) > s.max {
			s.list = s.list[:s.max]
		}
		s.rebuildIndex()
		if !changed {
			// if only reorder happened (rare), also treating “best improved” as change just in case!
			changed = s.improved(oldBest)
		} else {
			// definitely changed if best improved
			if s.improved(oldBest) {
				changed = true
			}
		}
	}
	return changed
}

// Rebuilds the index map from the list
func (s *shortlist) rebuildIndex() {
	s.idx = make(map[[20]byte]int, len(s.list))
	for i := range s.list {
		s.idx[s.list[i].c.ID] = i
	}
}

// nextBatch returns alpha closest UNQUIERIED contacts and marks them
func (s *shortlist) nextBatch(a int) []Contact {
	out := make([]Contact, 0, a)
	for i := range s.list {
		if !s.list[i].ask {
			out = append(out, s.list[i].c)
			s.list[i].ask = true
			if len(out) == a {
				break
			}
		}
	}
	return out
}

// improved returns true if the best distance became strictly smaller.
func (s *shortlist) improved(prevBest [20]byte) bool {
	if len(s.list) == 0 {
		return false
	}
	return less160(s.list[0].d, prevBest)
}

// best returns current best distance (or all-FF if empty).
func (s *shortlist) best() [20]byte {
	if len(s.list) == 0 {
		var ff [20]byte
		for i := range ff {
			ff[i] = 0xff
		}
		return ff
	}
	return s.list[0].d
}

// returns all contacts in the shortlist
func (s *shortlist) contacts() []Contact {
	out := make([]Contact, len(s.list))
	for i := range s.list {
		out[i] = s.list[i].c
	}
	return out
}
