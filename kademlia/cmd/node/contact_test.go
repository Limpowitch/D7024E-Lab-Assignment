package node

import (
	"testing"
	"time"
)

func TestContactTouch(t *testing.T) {
	c := Contact{ID: RandomNodeID(), Addr: "127.0.0.1:9999"}
	if !c.LastSeen.IsZero() {
		t.Fatalf("expected zero LastSeen initially, got %v", c.LastSeen)
	}
	c.Touch()
	if c.LastSeen.IsZero() {
		t.Fatal("Touch should set LastSeen")
	}
	if time.Since(c.LastSeen) > time.Second {
		t.Fatal("LastSeen looks too old; Touch likely not called")
	}
}
