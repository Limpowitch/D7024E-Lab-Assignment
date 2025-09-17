package node

import (
	"context"
	"testing"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/service"
)

func TestFindNode_Integration_WithContactCodec(t *testing.T) {
	var idA, idB [20]byte

	a, err := service.New("127.0.0.1:0", idA, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Start()

	b, err := service.New("127.0.0.1:0", idB, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	contacts := []Contact{
		{ID: RandomNodeID(), Addr: "x:1"},
		{ID: RandomNodeID(), Addr: "y:2"},
	}
	b.OnFindNode = func(target [20]byte) []byte {
		return MarshalContactList(contacts)
	}
	b.Start()

	var target [20]byte
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	raw, err := a.FindNode(ctx, b.Addr(), target)
	if err != nil {
		t.Fatalf("FindNode failed: %v", err)
	}

	got, err := UnmarshalContactList(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if len(got) != 2 || got[0].Addr == "" || got[1].Addr == "" {
		t.Fatalf("bad contacts: %+v", got)
	}
}
