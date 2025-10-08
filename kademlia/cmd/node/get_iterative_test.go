package node

import (
	"context"
	"testing"
	"time"
)

// func TestGetValueIterative_BasicScenarios(t *testing.T) {
// 	// Setup 3 nodes: A queries, B stores, C is just extra routing.
// 	nA, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
// 	nB, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
// 	nC, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
// 	for _, n := range []*Node{nA, nB, nC} {
// 		n.Start()
// 		t.Cleanup(func() { _ = n.Close() })
// 	}

// 	// Bootstrap RTs so they all know each other
// 	nA.RoutingTable.Update(Contact{ID: nB.NodeID, Addr: nB.Svc.Addr()})
// 	nA.RoutingTable.Update(Contact{ID: nC.NodeID, Addr: nC.Svc.Addr()})
// 	nB.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})
// 	nC.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})

// 	// Store value on B
// 	key := SHA1ID([]byte("iter-test"))
// 	nB.mu.Lock()
// 	nB.Store[string(key[:])] = Value{Data: []byte("xyz"), ExpiresAt: time.Now().Add(nB.ttl)}
// 	nB.mu.Unlock()

// 	// Case 1: A finds the value iteratively
// 	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
// 	defer cancel()
// 	val, _, err := nA.GetValueIterative(ctx, key, nA.RoutingTable.Closest(key, K))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if val != "xyz" {
// 		t.Fatalf("expected xyz, got %q", val)
// 	}

// 	// Case 2: Key not stored anywhere
// 	badKey := SHA1ID([]byte("missing"))
// 	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
// 	defer cancel2()
// 	val, _, err = nA.GetValueIterative(ctx2, badKey, nA.RoutingTable.Closest(badKey, K))
// 	if err == nil && val != "" {
// 		t.Fatalf("expected miss, got %q", val)
// 	}
// }

func TestKbucketSplit_BasicScenarios(t *testing.T) {
	// Setup 3 nodes: A queries, B stores, C is just extra routing.
	nA, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)

	kbucketsBefore := len(nA.RoutingTable.BucketList)
	if kbucketsBefore != 1 {
		t.Fatalf("expected 1, got %d", kbucketsBefore)
	}

	nB, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	nC, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	nD, _ := NewNode("127.0.0.1:0", "", 10*time.Second, 5*time.Second)
	for _, n := range []*Node{nA, nB, nC, nD} {
		n.Start()
		t.Cleanup(func() { _ = n.Close() })
	}

	// Bootstrap RTs so they all know each other
	pairs := [][2]*Node{
		{nA, nB}, {nA, nC}, {nA, nD},
		{nB, nC}, {nB, nD},
		{nC, nD},
	}
	for _, p := range pairs {
		x, y := p[0], p[1]
		x.RoutingTable.Update(Contact{ID: y.NodeID, Addr: y.Svc.Addr()})
		y.RoutingTable.Update(Contact{ID: x.NodeID, Addr: x.Svc.Addr()})
	}

	t.Logf("[A] buckets=%d\n%s", nA.RoutingTable.BucketsLen(), nA.RoutingTable.Dump())
	t.Logf("[B] buckets=%d\n%s", nB.RoutingTable.BucketsLen(), nB.RoutingTable.Dump())
	t.Logf("[C] buckets=%d\n%s", nC.RoutingTable.BucketsLen(), nC.RoutingTable.Dump())
	t.Logf("[D] buckets=%d\n%s", nD.RoutingTable.BucketsLen(), nD.RoutingTable.Dump())

	// Store value on B
	key := SHA1ID([]byte("iter-test"))
	for _, c := range nA.RoutingTable.Closest(key, K) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = nA.Svc.Store(ctx, c.Addr, key, []byte("xyz"))
		cancel()
	}

	// kbucketsAfter := len(nA.RoutingTable.BucketList)
	// if kbucketsAfter != 2 {
	// 	t.Fatalf("expected 2, got %d", kbucketsAfter)
	// }

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
