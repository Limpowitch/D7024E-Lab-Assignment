package node

// import (
// 	"context"
// 	"testing"
// 	"time"
// )

// func TestLookupNode_SmallNetwork(t *testing.T) {
// 	// Make three services (B, C, D) that answer FIND_NODE from their RT.
// 	// newSvc := func(id [20]byte) *service.Service {
// 	// 	s, err := service.New("127.0.0.1:0", id, "")
// 	// 	if err != nil {
// 	// 		t.Fatal(err)
// 	// 	}
// 	// 	s.OnFindNode = func(target [20]byte) []byte {
// 	// 		// in a real node this would use the node’s RT; here we fake via table we control
// 	// 		return MarshalContactList(nil) // set later if you wrap a Node
// 	// 	}
// 	// 	return s
// 	// }

// 	// It’s much simpler to use your Node wrapper:
// 	nA, _ := NewNode("127.0.0.1:0", "")
// 	nB, _ := NewNode("127.0.0.1:0", "")
// 	nC, _ := NewNode("127.0.0.1:0", "")
// 	nD, _ := NewNode("127.0.0.1:0", "")
// 	for _, n := range []*Node{nA, nB, nC, nD} {
// 		n.Start()
// 		defer n.Close()
// 	}

// 	// Bootstrap: have everyone learn everyone (or ping ring).
// 	_ = nA.PingPeer(nB.Svc.Addr())
// 	_ = nA.PingPeer(nC.Svc.Addr())
// 	_ = nB.PingPeer(nD.Svc.Addr())
// 	_ = nC.PingPeer(nD.Svc.Addr())

// 	// Manually seed routing tables a bit so Closest has something:
// 	nA.RoutingTable.Update(Contact{ID: nB.NodeID, Addr: nB.Svc.Addr()})
// 	nA.RoutingTable.Update(Contact{ID: nC.NodeID, Addr: nC.Svc.Addr()})
// 	nB.RoutingTable.Update(Contact{ID: nD.NodeID, Addr: nD.Svc.Addr()})
// 	nC.RoutingTable.Update(Contact{ID: nD.NodeID, Addr: nD.Svc.Addr()})

// 	// Pick a target near D’s ID
// 	target := nD.NodeID

// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 	defer cancel()
// 	contacts, err := nA.LookupNode(ctx, target)
// 	if err != nil {
// 		t.Fatalf("lookup failed: %v", err)
// 	}
// 	if len(contacts) == 0 {
// 		t.Fatalf("expected some contacts, got none")
// 	}
// 	// Sanity: D should be among the closest (often first)
// 	foundD := false
// 	for _, c := range contacts {
// 		if c.ID == nD.NodeID {
// 			foundD = true
// 			break
// 		}
// 	}
// 	if !foundD {
// 		t.Fatalf("expected to find D among closest, got: %+v", contacts)
// 	}
// }
