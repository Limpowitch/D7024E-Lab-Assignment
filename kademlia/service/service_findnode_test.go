package service

import (
	"context"
	"testing"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
)

func TestFindNode_RoundTrip(t *testing.T) {
	var idA, idB node.NodeID

	// A
	a, err := New("127.0.0.1:0", idA, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Start()

	// B with a stubbed handler: always return 2 contacts
	b, err := New("127.0.0.1:0", idB, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	b.OnFindNode = func(target node.NodeID) []node.Contact {
		return []node.Contact{
			{ID: node.RandomNodeID(), Addr: "x:1"},
			{ID: node.RandomNodeID(), Addr: "y:2"},
		}
	}
	b.Start()

	target := node.RandomNodeID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	cs, err := a.FindNode(ctx, b.Addr(), target)
	if err != nil {
		t.Fatalf("FindNode failed: %v", err)
	}
	if len(cs) != 2 || cs[0].Addr == "" || cs[1].Addr == "" {
		t.Fatalf("bad contacts: %+v", cs)
	}
}
