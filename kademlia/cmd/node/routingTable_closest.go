package node

import "sort"

// Returns the k closest contacts to the target ID from the routing table
func xor(a, b [20]byte) (out [20]byte) {
	for i := 0; i < 20; i++ {
		out[i] = a[i] ^ b[i]
	}
	return
}

// Compares two 160 bit values
func less160(a, b [20]byte) bool { // moved it here because it made more sense in my view
	for i := 0; i < 20; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// Returns the k closest contacts to the target ID
func (rt *RoutingTable) Closest(target [20]byte, k int) []Contact {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	type pair struct {
		c Contact
		d [20]byte
	}
	tmp := make([]pair, 0, 64)

	for _, b := range rt.BucketList {
		b.mu.RLock()
		for _, c := range b.Contacts {
			tmp = append(tmp, pair{c: c, d: xor(target, c.ID)})
		}
		b.mu.RUnlock()
	}

	sort.Slice(tmp, func(i, j int) bool { return less160(tmp[i].d, tmp[j].d) })
	if k > len(tmp) {
		k = len(tmp)
	}
	out := make([]Contact, k)
	for i := 0; i < k; i++ {
		out[i] = tmp[i].c
	}
	return out
}
