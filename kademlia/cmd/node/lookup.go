package node

import (
	"sort"
)

// Tuning constants. Adjust later if you like.
const (
	alpha = 3 // parallelism
)

type slEntry struct {
	c   Contact
	d   [20]byte // XOR distance to target
	ask bool     // already queried?
}

type shortlist struct {
	target [20]byte
	k      int
	idx    map[[20]byte]int // ID -> index in 'list'
	list   []slEntry
}

func newShortlist(target [20]byte, k int) *shortlist {
	return &shortlist{
		target: target,
		k:      k,
		idx:    make(map[[20]byte]int),
		list:   make([]slEntry, 0, k*2),
	}
}

// add merges contacts, dedups by ID, resorts by XOR, truncates to k.
func (s *shortlist) add(cs []Contact) {
	for _, c := range cs {
		if _, ok := s.idx[c.ID]; ok {
			continue
		}
		s.list = append(s.list, slEntry{c: c, d: xor(s.target, c.ID)})
	}
	sort.Slice(s.list, func(i, j int) bool { return less160(s.list[i].d, s.list[j].d) })
	if len(s.list) > s.k {
		s.list = s.list[:s.k]
	}
	s.rebuildIndex()
}

func (s *shortlist) rebuildIndex() {
	s.idx = make(map[[20]byte]int, len(s.list))
	for i := range s.list {
		s.idx[s.list[i].c.ID] = i
	}
}

// nextBatch returns up to α closest *unqueried* contacts and marks them 'ask'.
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

func (s *shortlist) contacts() []Contact {
	out := make([]Contact, len(s.list))
	for i := range s.list {
		out[i] = s.list[i].c
	}
	return out
}
