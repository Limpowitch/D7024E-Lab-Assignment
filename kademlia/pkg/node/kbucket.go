package node

import (
	"math/big"
	"sync"
	"time"
)

type Contact struct {
	ID       *big.Int
	Host     string
	LastSeen time.Time
}

type Kbucket struct {
	Capacity   int
	LowerLimit *big.Int
	UpperLimit *big.Int
	Contacts   []Contact
	mu         sync.RWMutex
}

func NewKBucket(k int, lower, upper *big.Int, collection []Contact) Kbucket {
	return Kbucket{
		Capacity:   k,
		LowerLimit: new(big.Int).Set(lower),
		UpperLimit: new(big.Int).Set(upper),
		Contacts:   collection,
	}
}

func (kb *Kbucket) AddToKBucket(contact Contact) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.Contacts = append(kb.Contacts, contact)
}

func (kb *Kbucket) ListIDs() []Contact {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	return append([]Contact(nil), kb.Contacts...)
}

func SplitBucket(originBucket Kbucket, newValue Contact) (Kbucket, Kbucket) {
	// Calculate midpoint: (upper + lower) / 2
	sum := new(big.Int).Add(originBucket.LowerLimit, originBucket.UpperLimit)
	midPoint := new(big.Int).Div(sum, big.NewInt(2))

	// Second bucket starts at midPoint + 1
	nextPoint := new(big.Int).Add(midPoint, big.NewInt(1))

	var kb1Contacts, kb2Contacts []Contact

	// Split existing contacts
	for _, contact := range originBucket.Contacts {
		if contact.ID.Cmp(midPoint) <= 0 {
			kb1Contacts = append(kb1Contacts, contact)
		} else {
			kb2Contacts = append(kb2Contacts, contact)
		}
	}

	// Add new contact to appropriate bucket
	if newValue.ID.Cmp(midPoint) <= 0 {
		kb1Contacts = append(kb1Contacts, newValue)
	} else {
		kb2Contacts = append(kb2Contacts, newValue)
	}

	kb1 := NewKBucket(originBucket.Capacity, originBucket.LowerLimit, midPoint, kb1Contacts)
	kb2 := NewKBucket(originBucket.Capacity, nextPoint, originBucket.UpperLimit, kb2Contacts)

	return kb1, kb2
}

// Helper function to create max 160-bit value (2^160 - 1)
func Max160BitInt() *big.Int {
	max := new(big.Int)
	max.Exp(big.NewInt(2), big.NewInt(160), nil) // 2^160
	max.Sub(max, big.NewInt(1))                  // 2^160 - 1
	return max
}

// Helper function to create zero value
func Zero160BitInt() *big.Int {
	return big.NewInt(0)
}
