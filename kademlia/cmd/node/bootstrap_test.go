package node

// import (
// 	"context"
// 	"fmt"
// 	"testing"
// 	"time"
// )

// // waitUntil polls a predicate until it returns true or timeout elapses.
// func waitUntil(t *testing.T, d time.Duration, pred func() bool) bool {
// 	deadline := time.Now().Add(d)
// 	for time.Now().Before(deadline) {
// 		if pred() {
// 			return true
// 		}
// 		time.Sleep(20 * time.Millisecond)
// 	}
// 	return false
// }

// // Ensures that N nodes, when bootstrapped against a seed, all learn at least one contact.
// func TestBootstrap_AllNodesHaveContacts(t *testing.T) {
// 	const N = 20
// 	ttl := 30 * time.Second
// 	refresh := 15 * time.Second

// 	// Start a seed node.
// 	seed, err := NewNode("127.0.0.1:0", "", ttl, refresh)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	seed.Start()
// 	t.Cleanup(func() { _ = seed.Close() })
// 	seedAddr := seed.Svc.Addr()

// 	// Helpers to assert a non-broken advertised addr in FindNode responses.
// 	// (If you’ve already fixed OnFindNode to use n.AdvertisedAddr(), this will pass.)
// 	if seed.AdvertisedAddr() == "" || seed.AdvertisedAddr()[0] == ':' {
// 		t.Fatalf("seed advertised addr looks bad: %q (listen=%s)", seed.AdvertisedAddr(), seedAddr)
// 	}

// 	// Start the remaining nodes and bootstrap them to the seed.
// 	nodes := make([]*Node, 0, N)
// 	nodes = append(nodes, seed)

// 	for i := 1; i < N; i++ {
// 		n, err := NewNode("127.0.0.1:0", "", ttl, refresh)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		n.Start()
// 		t.Cleanup(func() { _ = n.Close() })

// 		// Minimal bootstrap similar to your CLI: PING then a few lookups.
// 		// 1) Learn seed’s ID (PONG -> OnSeen -> Update).
// 		{
// 			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
// 			_ = n.Svc.Ping(ctx, seedAddr)
// 			cancel()
// 		}
// 		// 2) A couple of lookups to diversify buckets.
// 		for j := 0; j < 3; j++ {
// 			ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
// 			_, _ = n.LookupNode(ctx, n.NodeID) // near-self target is fine here
// 			cancel()
// 		}

// 		nodes = append(nodes, n)
// 	}

// 	// Assert: each node (including the seed) has at least one contact with a usable addr.
// 	ok := waitUntil(t, 2*time.Second, func() bool {
// 		for _, n := range nodes {
// 			cs := n.RoutingTable.Closest(n.NodeID, 1)
// 			if len(cs) == 0 {
// 				return false
// 			}
// 			// Check addr sanity (no “:port” only; not empty).
// 			if cs[0].Addr == "" || cs[0].Addr[0] == ':' {
// 				return false
// 			}
// 		}
// 		return true
// 	})

// 	if !ok {
// 		// Print per-node diagnostics to see who failed and why.
// 		for idx, n := range nodes {
// 			cs := n.RoutingTable.Closest(n.NodeID, 5)
// 			s := "—"
// 			if len(cs) > 0 {
// 				s = fmt.Sprintf("%s …", cs[0].Addr)
// 			}
// 			t.Logf("node[%02d] listen=%s adv=%q contacts=%d first=%s",
// 				idx, n.Svc.Addr(), n.AdvertisedAddr(), len(cs), s)
// 		}
// 		t.Fatal("some nodes failed to learn any contacts after bootstrap (see logs above)")
// 	}
// }
