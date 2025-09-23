package node

import (
	"errors"
	"sync"
)

type Kbucket struct {
	Capacity   int
	LowerLimit [20]byte
	UpperLimit [20]byte
	Contacts   []Contact
	mu         sync.RWMutex
}

// Creates a new kbucket
func NewKBucket(k int, lower, upper [20]byte, collection []Contact) (Kbucket, error) {
	return Kbucket{
		Capacity:   k,
		LowerLimit: lower,
		UpperLimit: upper,
		Contacts:   collection,
	}, nil
}

// Adds a contact to the kbucket
func (kb *Kbucket) AddToKBucket(id Contact) { // We simply append here, no capacity check needed (will be handled elsewere)
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.Contacts = append(kb.Contacts, id)
}

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

// Upserts a contact to the kbucket
func (kb *Kbucket) Upsert(c Contact) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	if kb.moveToTailIfExist(c) {
		return
	}

	if len(kb.Contacts) < kb.Capacity {
		kb.Contacts = append(kb.Contacts, c)
		return
	}

	copy(kb.Contacts, kb.Contacts[1:])
	kb.Contacts[len(kb.Contacts)-1] = c
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

//TODO
