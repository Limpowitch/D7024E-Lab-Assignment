package node

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type RoutingTable struct {
	SelfID     [20]byte
	BucketList []*Kbucket
	mu         sync.RWMutex
}

// Creates a new routing table with a single kbucket that is covering the entire id space
func NewRoutingTable(SelfId, lower, upper [20]byte, K, b_val int) (RoutingTable, error) {

	rt := RoutingTable{
		SelfID:     SelfId,
		BucketList: make([]*Kbucket, 1),
	}

	kb, err := NewKBucket(K, lower, upper, nil, b_val)
	if err != nil {
		return RoutingTable{}, errors.New("failed to create initial kbucket")
	}

	rt.BucketList[0] = &kb
	return rt, nil
}

// adds a bucket to the routing table
func (rt *RoutingTable) addBucketLocked(kb *Kbucket) error {

	lower := kb.LowerLimit
	upper := kb.UpperLimit

	if len(rt.BucketList) == 0 {
		rt.BucketList = append(rt.BucketList, kb)
		return nil
	}

	insertAt := len(rt.BucketList)
	for i := 0; i < len(rt.BucketList); i++ {
		b := rt.BucketList[i]

		if b.LowerLimit == lower && b.UpperLimit == upper {
			return errors.New("kbucket already exists in routing table")
		}

		// find first bucket whose lower >= new.lower -> insert before it
		if !less160(b.LowerLimit, lower) {
			insertAt = i
			break
		}
	}

	rt.BucketList = append(rt.BucketList, &Kbucket{})
	copy(rt.BucketList[insertAt+1:], rt.BucketList[insertAt:])
	rt.BucketList[insertAt] = kb
	return nil
}

// removes a bucket from the routing table
func (rt *RoutingTable) removeBucketLocked(kb *Kbucket) error {

	if len(rt.BucketList) == 0 {
		return errors.New("routing table contains no kbuckets")
	}

	idx := -1
	for i := range rt.BucketList {
		b := rt.BucketList[i]
		if b.LowerLimit == kb.LowerLimit && b.UpperLimit == kb.UpperLimit {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("kbucket not found in routing table")
	}

	copy(rt.BucketList[idx:], rt.BucketList[idx+1:])
	rt.BucketList = rt.BucketList[:len(rt.BucketList)-1]
	return nil
}

// containsSelf reports whether SelfID lies in kb's [lower, upper] inclusive.
func (rt *RoutingTable) containsSelf(kb *Kbucket) bool {
	return compare(rt.SelfID, kb.LowerLimit) >= 0 &&
		compare(rt.SelfID, kb.UpperLimit) <= 0
}

// Update should be called whenever an RPC succeeds for contact c.
func (rt *RoutingTable) Update(c Contact) int {
	didSplit := false
	for {
		// (1) Locate the leaf and grab its lock while we still hold rt.mu.RLock.
		rt.mu.RLock()
		i := rt.bucketIndexFor(c.ID)
		if i < 0 {
			rt.mu.RUnlock()
			return 0
		}
		kb := rt.BucketList[i]
		kb.mu.Lock()
		rt.mu.RUnlock()

		// (2) Upsert under the bucket lock.
		needsPolicy := kb.upsertNoLock(c)
		kb.mu.Unlock()
		if !needsPolicy {
			if didSplit {
				return 5
			}
			return 1
		}

		// (3) FULL → consider split under writer lock.
		rt.mu.Lock()
		// If someone else already split/removed this bucket, just continue.
		stillThere := false
		for _, b := range rt.BucketList {
			if b == kb {
				stillThere = true
				break
			}
		}
		if !stillThere {
			rt.mu.Unlock()
			didSplit = true
			continue
		}

		// Re-check eligibility while holding rt.mu.
		eligible := rt.containsSelf(kb) || (kb.Depth()%kb.b != 0)
		if !eligible {
			rt.mu.Unlock()
			return 4
		}

		// (4) Perform the split while protecting the bucket’s Contacts.
		_ = rt.splitBucketLocked(kb)
		rt.mu.Unlock()
		didSplit = true
		// (5) Loop to re-locate the correct child and try again.
	}
}

// Splits a bucket into two new buckets
func (rt *RoutingTable) SplitBucket(originBucket *Kbucket) error {

	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.splitBucketLocked(originBucket)
}

func (rt *RoutingTable) splitBucketLocked(originBucket *Kbucket) error {
	// Protect access to Contacts during partitioning.
	originBucket.mu.Lock()
	mid := midpoint(originBucket.LowerLimit, originBucket.UpperLimit)

	kb1Lower := originBucket.LowerLimit
	kb1Upper := mid
	kb2Lower := addOne(mid)
	kb2Upper := originBucket.UpperLimit

	var kb1Contacts, kb2Contacts []Contact
	for _, c := range originBucket.Contacts {
		if compare(c.ID, kb1Lower) >= 0 && compare(c.ID, kb1Upper) <= 0 {
			kb1Contacts = append(kb1Contacts, c)
		} else {
			kb2Contacts = append(kb2Contacts, c)
		}
	}
	originBucket.mu.Unlock()

	kb1, _ := NewKBucket(originBucket.Capacity, kb1Lower, kb1Upper, kb1Contacts, originBucket.b) // Bucket1 = [originbucket.lower, mid]
	kb2, _ := NewKBucket(originBucket.Capacity, kb2Lower, kb2Upper, kb2Contacts, originBucket.b) // Bucket2 = [mid + 1, originbucket.upper]

	// Remove the old one; ignore error if already removed by a concurrent splitter.
	_ = rt.removeBucketLocked(originBucket)
	_ = rt.addBucketLocked(&kb1)
	_ = rt.addBucketLocked(&kb2)
	return nil
}

// following 3 functions below are simple helper functions, since we cant do simple arithmatic on [20]byte

func compare(a, b [20]byte) int { // compare returns -1 if a<b, 0 if a==b, 1 if a>b
	for i := 0; i < 20; i++ {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func addOne(x [20]byte) [20]byte { // addOne returns x+1 (mod 2^160)
	var out [20]byte
	carry := byte(1)
	for i := 19; i >= 0; i-- {
		sum := uint16(x[i]) + uint16(carry)
		out[i] = byte(sum & 0xff)
		carry = byte(sum >> 8)
	}
	return out
}

func midpoint(a, b [20]byte) [20]byte {
	// diff = b - a  (big-endian)
	var diff [20]byte
	var borrow uint16
	for i := 19; i >= 0; i-- {
		bi := uint16(b[i])
		ai := uint16(a[i])
		if bi < ai+borrow {
			diff[i] = byte(256 + bi - ai - borrow)
			borrow = 1
		} else {
			diff[i] = byte(bi - ai - borrow)
			borrow = 0
		}
	}

	// half = diff >> 1  (big-endian right shift by 1)
	var half [20]byte
	var carry byte
	for i := 0; i < 20; i++ {
		v := (uint16(carry) << 8) | uint16(diff[i])
		half[i] = byte(v >> 1)
		carry = byte(v & 1)
	}

	// out = a + half
	var out [20]byte
	var c uint16
	for i := 19; i >= 0; i-- {
		sum := uint16(a[i]) + uint16(half[i]) + c
		out[i] = byte(sum & 0xff)
		c = sum >> 8
	}
	return out
}

func (rt *RoutingTable) bucketIndexFor(id [20]byte) int {
	for i := range rt.BucketList {
		b := rt.BucketList[i]
		if compare(id, b.LowerLimit) >= 0 && compare(id, b.UpperLimit) <= 0 {
			return i
		}
	}
	return -1
}

// in node/routingtable.go
func (rt *RoutingTable) Dump() string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var sb strings.Builder
	fmt.Fprintf(&sb, "self=%x\n", rt.SelfID[:4])
	for i, b := range rt.BucketList {
		b.mu.RLock()
		fmt.Fprintf(&sb, "bucket[%02d] [%x..%x] depth=%d b=%d contacts=%d\n",
			i, b.LowerLimit[:2], b.UpperLimit[:2], b.Depth(), b.b, len(b.Contacts))
		for _, c := range b.Contacts {
			fmt.Fprintf(&sb, "  - %x %s\n", c.ID[:4], c.Addr)
		}
		b.mu.RUnlock()
	}
	return sb.String()
}

func (rt *RoutingTable) BucketsLen() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.BucketList)
}
