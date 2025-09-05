package node

import (
	"sync"
	"time"
)

type Contact struct {
	ID       [20]byte
	Host     string
	LastSeen time.Time
}

type Kbucket struct {
	Capacity   int
	LowerLimit int
	UpperLimit int
	Contacts   []Contact
	mu         sync.RWMutex
}

func NewKBucket(k, lower, upper int, collection []Contact) Kbucket {
	return Kbucket{
		Capacity:   k,
		LowerLimit: lower,
		UpperLimit: upper,
		Contacts:   collection,
	}
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

//TODO
