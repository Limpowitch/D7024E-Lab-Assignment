package node

import (
	"context"
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

func TestFindValue_Contacts(t *testing.T) {
	nA, _ := NewNode("127.0.0.1:0", "")
	nA.Start()
	defer nA.Close()
	nB, _ := NewNode("127.0.0.1:0", "")
	nB.Start()
	defer nB.Close()

	// Seed nB's RT with a contact (e.g., itself or nA) so it returns something
	nB.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})

	var key [20]byte // not stored anywhere
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	v, contacts, err := nA.FindValue(ctx, nB.Svc.Addr(), key)
	if err != nil || v != nil || len(contacts) == 0 {
		t.Fatalf("FindValue contacts branch failed: v=%v contacts=%v err=%v", v, contacts, err)
	}
}
