package node

import (
	"errors"
	"sync"
)

type Contact struct {
	ID   [20]byte
	Host string
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
		ID:   node.NodeID,
		Host: node.Hostname,
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

//TODO
