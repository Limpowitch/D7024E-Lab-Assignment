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
func NewRoutingTable(SelfId, lower, upper [20]byte) (RoutingTable, error) {
	const KBucketCapacity = 20

	rt := RoutingTable{
		SelfID:     SelfId,
		BucketList: make([]*Kbucket, 1),
	}

	kb, err := NewKBucket(KBucketCapacity, lower, upper, nil)
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

// call everytime we succeed with RPC. if contact exist, move to tail. if bucket has room, append. bucket full? drop it or remove head. add LRU logic perhaps?
func (rt *RoutingTable) Update(c Contact) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	i := rt.bucketIndexFor(c.ID)
	if i < 0 {
		return
	}
	rt.BucketList[i].Upsert(c)
}

// Splits a bucket into two new buckets
func (rt *RoutingTable) SplitBucket(originBucket *Kbucket) error {

	rt.mu.Lock()
	defer rt.mu.Unlock()

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

	kb1, _ := NewKBucket(originBucket.Capacity, kb1Lower, kb1Upper, kb1Contacts) // Bucket1 = [originbucket.lower, mid]
	kb2, _ := NewKBucket(originBucket.Capacity, kb2Lower, kb2Upper, kb2Contacts) // Bucket2 = [mid + 1, originbucket.upper]

	rt.removeBucketLocked(originBucket)

	rt.addBucketLocked(&kb1)
	rt.addBucketLocked(&kb2)

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

func midpoint(a, b [20]byte) [20]byte { // midpoint returns floor((a+b)/2)
	var out [20]byte
	var carry uint16
	for i := 19; i >= 0; i-- {
		sum := uint16(a[i]) + uint16(b[i]) + carry
		out[i] = byte((sum >> 1) & 0xff) // divide by 2
		carry = (sum & 1) << 8           // carry remainder to next higher byte
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
		fmt.Fprintf(&sb, "bucket[%02d] [%x..%x] %d contacts\n",
			i, b.LowerLimit[:2], b.UpperLimit[:2], len(b.Contacts))
		for _, c := range b.Contacts {
			fmt.Fprintf(&sb, "  - %x %s\n", c.ID[:4], c.Addr)
		}
		b.mu.RUnlock()
	}
	return sb.String()
}
