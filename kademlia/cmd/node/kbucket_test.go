package node

import (
	"testing"
)

// small helpers
func zeroID() [20]byte { return [20]byte{} }
func maxID() [20]byte {
	var x [20]byte
	for i := range x {
		x[i] = 0xFF
	}
	return x
}

func TestNewKbucket(t *testing.T) {
	capacity := 3
	lower := zeroID()
	upper := maxID()

	node, err := NewNode("localhost")
	if err != nil {
		t.Fatalf("expected no error at NewNode creation, got %v", err)
	}

	contact1, err := NewContact(*node)
	if err != nil {
		t.Fatalf("expected no error at NewContact creation, got %v", err)
	}

	contacts := []Contact{contact1}

	kb, err := NewKBucket(capacity, lower, upper, contacts)
	if err != nil {
		t.Fatalf("expected no error at NewKBucket creation, got %v", err)
	}

	if kb.Capacity != capacity {
		t.Errorf("expected capacity %d, got %d", capacity, kb.Capacity)
	}
	if len(kb.Contacts) != 1 {
		t.Errorf("expected 1 contact, got %d", len(kb.Contacts))
	}
	if kb.Contacts[0].Host != "localhost" {
		t.Errorf("expected contact host 'localhost', got %s", kb.Contacts[0].Host)
	}
}

func TestAddToKBucket_AppendAndList(t *testing.T) {
	kb, _ := NewKBucket(5, zeroID(), maxID(), nil)

	// two deterministic contacts
	c1 := Contact{ID: idWithFirstByte(0x01), Host: "a"}
	c2 := Contact{ID: idWithFirstByte(0xFE), Host: "b"}

	kb.AddToKBucket(c1)
	kb.AddToKBucket(c2)

	ids := kb.Contacts
	if len(ids) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(ids))
	}
	if ids[0].ID != c1.ID || ids[1].ID != c2.ID {
		t.Errorf("contacts not in append order: got %v then %v", ids[0].ID, ids[1].ID)
	}
}
