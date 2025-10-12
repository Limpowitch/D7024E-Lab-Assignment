package node

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func newCovNode(t *testing.T) *Node {
	t.Helper()
	n, err := NewNode("127.0.0.1:0", "", 2*time.Second, 1*time.Second)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	n.Start()
	t.Cleanup(func() { _ = n.Close() })
	return n
}

// Covers: n.Svc.SetOnDumpRT (AdminRT -> OnDumpRT)
func TestAdminRT_DumpRTIncludesAllContacts(t *testing.T) {
	n := newCovNode(t)

	// Seed the RT with two contacts living at different addrs/IDs.
	var id1, id2 [20]byte
	id1[0] = 0x01 // lands in lower range
	id2[0] = 0x80 // lands in upper range
	c1 := Contact{ID: id1, Addr: "127.0.0.1:11111"}
	c2 := Contact{ID: id2, Addr: "127.0.0.1:22222"}
	n.RoutingTable.Update(c1)
	n.RoutingTable.Update(c2)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	payload, err := n.Svc.AdminRT(ctx, n.AdvertisedAddr())
	if err != nil {
		t.Fatalf("AdminRT: %v", err)
	}
	contacts, err := UnmarshalContactList(payload)
	if err != nil {
		t.Fatalf("UnmarshalContactList: %v", err)
	}

	// Check both IDs are present.
	found1, found2 := false, false
	for _, c := range contacts {
		if c.ID == id1 && c.Addr == c1.Addr {
			found1 = true
		}
		if c.ID == id2 && c.Addr == c2.Addr {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("DumpRT missing contacts: id1=%v id2=%v (got %d entries)", found1, found2, len(contacts))
	}
}

// Covers: OnAdminPut branch where Closest(...) returns empty (len(cs)==0).
func TestAdminPut_LocalStoreWhenRTEmpty(t *testing.T) {
	n := newCovNode(t)

	// Ensure RT has no peers; with a single node, LookupNode won't discover anyone.
	data := []byte("branch-coverage")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key, err := n.Svc.AdminPut(ctx, n.AdvertisedAddr(), data)
	if err != nil {
		t.Fatalf("AdminPut: %v", err)
	}

	// Verify the value was stored locally and marked as origin with timestamps.
	n.mu.RLock()
	v, ok := n.Store[string(key[:])]
	n.mu.RUnlock()
	if !ok {
		t.Fatalf("expected key stored locally in early-return branch")
	}
	if string(v.Data) != string(data) {
		t.Fatalf("stored data mismatch: got %q want %q", string(v.Data), string(data))
	}
	if !v.Origin {
		t.Fatalf("expected Origin=true for locally published value")
	}
	if v.LastPublish.IsZero() || v.ExpiresAt.IsZero() {
		t.Fatalf("expected LastPublish/ExpiresAt to be set, got LastPublish=%v ExpiresAt=%v", v.LastPublish, v.ExpiresAt)
	}
}

func newAdminTestNode(t *testing.T) *Node {
	t.Helper()
	n, err := NewNode("127.0.0.1:0", "", 2*time.Second, 1*time.Second)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	n.Start()
	t.Cleanup(func() { _ = n.Close() })
	return n
}

func TestNode_AdminForget_Handler(t *testing.T) {
	n := newAdminTestNode(t)

	// Put a value directly in local store so AdminForget(true-path) can delete it.
	key := SHA1ID([]byte("cover-admin-forget"))
	n.mu.Lock()
	n.Store[string(key[:])] = Value{Data: []byte("v"), ExpiresAt: time.Now().Add(1 * time.Minute)}
	n.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1) Present case => handler returns true and deletes the entry.
	if err := n.Svc.AdminForget(ctx, n.AdvertisedAddr(), key); err != nil {
		t.Fatalf("AdminForget present: %v", err)
	}
	n.mu.RLock()
	_, ok := n.Store[string(key[:])]
	n.mu.RUnlock()
	if ok {
		t.Fatalf("AdminForget did not remove key from local store")
	}

	// 2) Missing case => handler returns false (we can only observe that it doesn't panic and store stays missing).
	if err := n.Svc.AdminForget(ctx, n.AdvertisedAddr(), key); err != nil {
		t.Fatalf("AdminForget missing: %v", err)
	}
	n.mu.RLock()
	_, ok = n.Store[string(key[:])]
	n.mu.RUnlock()
	if ok {
		t.Fatalf("missing AdminForget unexpectedly created the key")
	}
}

func TestNode_AdminExit_SendsSIGTERM(t *testing.T) {
	n := newAdminTestNode(t)

	// Capture SIGTERM so the process isn't killed when the node’s OnExit self-signals.
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGTERM)
	defer signal.Stop(termCh)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := n.Svc.AdminExit(ctx, n.AdvertisedAddr()); err != nil {
		t.Fatalf("AdminExit: %v", err)
	}

	select {
	case <-termCh:
		// got the SIGTERM sent by node’s OnExit closure -> covered
	case <-time.After(1 * time.Second):
		t.Fatal("expected SIGTERM after AdminExit, none received")
	}
}

// newTestNode is a tiny helper to keep tests tidy.
func newTestNode(t *testing.T) *Node {
	t.Helper()
	n, err := NewNode("127.0.0.1:0" /*adv*/, "", 2*time.Second, 1*time.Second)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	n.Start()
	t.Cleanup(func() { _ = n.Close() })
	return n
}

func TestNewNodeAndAdvertisedAddr(t *testing.T) {
	n := newTestNode(t)

	// NodeID should not be all zeros.
	var zero [20]byte
	if n.NodeID == zero {
		t.Fatalf("expected non-zero NodeID")
	}

	// With no advertised address, we should fall back to the bound addr.
	got := n.AdvertisedAddr()
	if got == "" {
		t.Fatalf("expected a non-empty AdvertisedAddr()")
	}

	// If SelfAddr is set, AdvertisedAddr must return it.
	n.Svc.SelfAddr = "203.0.113.1:9999"
	if n.AdvertisedAddr() != "203.0.113.1:9999" {
		t.Fatalf("AdvertisedAddr() did not use SelfAddr override")
	}
}

func TestFindNode_ReturnsSelfFirst(t *testing.T) {
	a := newTestNode(t)
	b := newTestNode(t)

	// Cross-seed routing tables so they know about each other.
	a.RoutingTable.Update(Contact{ID: b.NodeID, Addr: b.Svc.Addr()})
	b.RoutingTable.Update(Contact{ID: a.NodeID, Addr: a.Svc.Addr()})

	// Ask B about (any) target; when the target is B itself, the handler should
	// return B as the first contact (self-first behavior in node.go).
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	contacts, err := a.FindNode(b.Svc.Addr(), b.NodeID)
	if err != nil {
		t.Fatalf("FindNode: %v", err)
	}
	if len(contacts) == 0 {
		t.Fatalf("FindNode returned no contacts")
	}
	if contacts[0].ID != b.NodeID {
		t.Fatalf("expected first contact to be B (self), got %x", contacts[0].ID[:4])
	}
}

func TestAdminPut_AdminGet_RoundTripAcrossPeers(t *testing.T) {
	a := newTestNode(t)
	b := newTestNode(t)

	// Make sure A knows B so A's AdminPut can store to B (among K closest).
	a.RoutingTable.Update(Contact{ID: b.NodeID, Addr: b.Svc.Addr()})
	b.RoutingTable.Update(Contact{ID: a.NodeID, Addr: a.Svc.Addr()})

	// Ask A (locally) to publish a value into the DHT.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	value := []byte("hello-node")
	key, err := a.Svc.AdminPut(ctx, a.AdvertisedAddr(), value)
	if err != nil {
		t.Fatalf("AdminPut: %v", err)
	}

	// Now ask B (locally) to resolve that key. Since A stored to the K closest
	// and we seeded B in A's RT, B should have the value and return it locally.
	got, ok, err := b.Svc.AdminGet(ctx, b.AdvertisedAddr(), key)
	if err != nil {
		t.Fatalf("AdminGet: %v", err)
	}
	if !ok {
		t.Fatalf("AdminGet reported miss; expected a value")
	}
	if string(got) != string(value) {
		t.Fatalf("AdminGet value mismatch: got %q, want %q", string(got), string(value))
	}
}
