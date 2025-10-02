package node

import (
	"context"
	"testing"
	"time"
)

func TestGetValueIterative_BasicScenarios(t *testing.T) {
	// Setup 3 nodes: A queries, B stores, C is just extra routing.
	nA, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	nB, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	nC, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	for _, n := range []*Node{nA, nB, nC} {
		n.Start()
		t.Cleanup(func() { _ = n.Close() })
	}

	// Bootstrap RTs so they all know each other
	nA.RoutingTable.Update(Contact{ID: nB.NodeID, Addr: nB.Svc.Addr()})
	nA.RoutingTable.Update(Contact{ID: nC.NodeID, Addr: nC.Svc.Addr()})
	nB.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})
	nC.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})

	// Store value on B
	key := SHA1ID([]byte("iter-test"))
	nB.mu.Lock()
	nB.Store[string(key[:])] = Value{Data: []byte("xyz"), ExpiresAt: time.Now().Add(nB.ttl)}
	nB.mu.Unlock()

	// Case 1: A finds the value iteratively
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, _, err := nA.GetValueIterative(ctx, key, nA.RoutingTable.Closest(key, K))
	if err != nil {
		t.Fatal(err)
	}
	if val != "xyz" {
		t.Fatalf("expected xyz, got %q", val)
	}

	// Case 2: Key not stored anywhere
	badKey := SHA1ID([]byte("missing"))
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	val, _, err = nA.GetValueIterative(ctx2, badKey, nA.RoutingTable.Closest(badKey, K))
	if err == nil && val != "" {
		t.Fatalf("expected miss, got %q", val)
	}
}
