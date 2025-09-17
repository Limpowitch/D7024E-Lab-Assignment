package main

import (
	"sync"
)

type Contact struct {
	ID      [20]byte
	Address string
	//LastSeen time.Time		//commented out for now
}
type Kbucket struct {
	Capacity   int
	LowerLimit [20]byte
	UpperLimit [20]byte
	Contacts   []Contact
	mu         sync.RWMutex
}

func NewContact(node Node) (Contact, error) {
	return Contact{
		ID:      node.NodeID,
		Address: node.Hostname,
	}, nil
}

func NewKBucket(k int, lower, upper [20]byte, collection []Contact) (Kbucket, error) {
	return Kbucket{
		Capacity:   k,
		LowerLimit: lower,
		UpperLimit: upper,
		Contacts:   collection,
	}, nil
}

func (kb *Kbucket) AddToKBucket(id Contact) { // We simply append here, no capacity check needed (will be handled elsewere)
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.Contacts = append(kb.Contacts, id)
}

func (kb *Kbucket) ListIDs() []Contact {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	return append([]Contact(nil), kb.Contacts...)
}

func SplitBucket(originBucket Kbucket) (Kbucket, Kbucket) {

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
	return kb1, kb2
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

//TODO
