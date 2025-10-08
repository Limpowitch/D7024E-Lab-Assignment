package node

import (
	"errors"
	"math/bits"
	"sync"
)

type Kbucket struct {
	Capacity   int
	LowerLimit [20]byte
	UpperLimit [20]byte
	Contacts   []Contact
	mu         sync.RWMutex
	b          int
}

// Creates a new kbucket
func NewKBucket(k int, lower, upper [20]byte, collection []Contact, b_val int) (Kbucket, error) {
	return Kbucket{
		Capacity:   k,
		LowerLimit: lower,
		UpperLimit: upper,
		Contacts:   collection,
		b:          b_val,
	}, nil
}

// Adds a contact to the kbucket. Probably dont need this
// func (kb *Kbucket) AddToKBucket(id Contact) { // We simply append here, no capacity check needed (will be handled elsewere)
// 	kb.mu.Lock()
// 	defer kb.mu.Unlock()
// 	kb.Contacts = append(kb.Contacts, id)
// }

// Removes a contact from the kbucket
func (kb *Kbucket) RemoveFromKBucket(c Contact) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	idx := -1
	for i := 0; i < len(kb.Contacts); i++ {
		if kb.Contacts[i].ID == c.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("contact is not inside kbucket")
	}

	copy(kb.Contacts[idx:], kb.Contacts[idx+1:])
	kb.Contacts = kb.Contacts[:len(kb.Contacts)-1]
	return nil
}

// Upsert tries to add or touch.
// Returns true if the bucket is FULL and the caller must apply policy.
func (kb *Kbucket) Upsert(c Contact) bool {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	return kb.upsertNoLock(c)
}

// upsertNoLock performs the Upsert assuming kb.mu is already held.
func (kb *Kbucket) upsertNoLock(c Contact) bool {
	// already present → move to tail (LRU)
	if kb.moveToTailIfExist(c) {
		return false
	}
	// space available → append
	if len(kb.Contacts) < kb.Capacity {
		kb.Contacts = append(kb.Contacts, c)
		return false
	}
	// FULL → caller must apply policy (split/ping/replace)
	return true
}

// if the contact is already present in bucket, place it last (update for LRU-standard, essentially)
func (kb *Kbucket) moveToTailIfExist(c Contact) bool {
	for i := range kb.Contacts {
		if kb.Contacts[i].ID == c.ID {
			temp := kb.Contacts[i]
			copy(kb.Contacts[i:], kb.Contacts[i+1:])
			kb.Contacts[len(kb.Contacts)-1] = temp
			return true
		}
	}
	return false
}

func (kb *Kbucket) Depth() int {
	a := kb.LowerLimit
	b := kb.UpperLimit
	depth := 0
	for i := 0; i < 20; i++ {
		if a[i] == b[i] {
			depth += 8
			continue
		}
		// first differing byte: count equal leading bits inside this byte
		x := a[i] ^ b[i]
		depth += bits.LeadingZeros8(x) // 0..8
		return depth
	}
	// identical bounds → degenerate range (all 160 bits fixed)
	return 160
}
