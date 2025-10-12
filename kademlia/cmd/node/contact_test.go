package node

import (
	"testing"
	"time"
)

func TestNewContact_InitializesFields(t *testing.T) {
	// Build a deterministic NodeID (no helper funcs needed)
	var id NodeID
	id[0] = 0xAA
	id[19] = 0x55

	addr := "127.0.0.1:4242"
	c := NewContact(id, addr)

	if c.ID != id {
		t.Fatalf("ID mismatch: got %x, want %x", c.ID[:4], id[:4])
	}
	if c.Addr != addr {
		t.Fatalf("Addr mismatch: got %q, want %q", c.Addr, addr)
	}
	if !c.LastSeen.IsZero() {
		t.Fatalf("LastSeen should be zero on construction, got %v", c.LastSeen)
	}
}

func TestContact_TouchSetsTimestampWithinWindow(t *testing.T) {
	var id NodeID
	c := NewContact(id, "127.0.0.1:1")

	before := time.Now()
	c.Touch()
	after := time.Now()

	if c.LastSeen.IsZero() {
		t.Fatal("Touch should set LastSeen, but it is zero")
	}
	// LastSeen should be between before and after (inclusive)
	if c.LastSeen.Before(before) || c.LastSeen.After(after) {
		t.Fatalf("LastSeen not within expected window: got=%v, window=[%v .. %v]", c.LastSeen, before, after)
	}
}

func TestContact_TouchIsMonotonic(t *testing.T) {
	var id NodeID
	c := NewContact(id, "127.0.0.1:1")

	c.Touch()
	first := c.LastSeen

	// Small sleep to ensure time advances on systems with coarse clock granularity
	time.Sleep(2 * time.Millisecond)

	c.Touch()
	second := c.LastSeen

	if !second.After(first) {
		t.Fatalf("Touch should advance LastSeen: first=%v second=%v", first, second)
	}
}
