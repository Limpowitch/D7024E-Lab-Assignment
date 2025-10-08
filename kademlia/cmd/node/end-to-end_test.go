package node

import (
	"context"
	"testing"
	"time"
)

func TestAdminPutGet_EndToEnd(t *testing.T) {
	nA, _ := NewNode("127.0.0.1:0", "", 30, 15) // client-ish
	nB, _ := NewNode("127.0.0.1:0", "", 30, 15) // daemon-ish
	for _, n := range []*Node{nA, nB} {
		n.Start()
		t.Cleanup(func() { _ = n.Close() })
	}

	// Minimal bootstrap: ensure A knows B so AdminPut populates around B quickly.
	nA.RoutingTable.Update(Contact{ID: nB.NodeID, Addr: nB.Svc.Addr()})
	nB.RoutingTable.Update(Contact{ID: nA.NodeID, Addr: nA.Svc.Addr()})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key, err := nA.Svc.AdminPut(ctx, nB.Svc.Addr(), []byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("put key=%x", key[:4]) // should print 57650b31 like your logs

	got, ok, err := nA.Svc.AdminGet(ctx, nB.Svc.Addr(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("AdminGet: not found")
	}
	if string(got) != "hello world" {
		t.Fatalf("got %q", got)
	}

}
