package main

import (
	"testing"
)

func TestNewKbucket(t *testing.T) {
	capacity := 3
	var lower [20]byte // defaults to all zeros
	var upper [20]byte
	for i := range upper {
		upper[i] = 0xFF // max value for upper limit
	}

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
