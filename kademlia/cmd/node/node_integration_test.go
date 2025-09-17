//go:build !race
// +build !race

package main

import (
	"net"
	"testing"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

// Samma handler som i main – kopplar UDP <-> dina RPC-handlers.
func attachHandler(n *Node, srv *transport.UDPServer) {
	srv.SetHandler(func(from *net.UDPAddr, env wire.Envelope) {
		switch env.Type {
		case wire.TypeStore:
			fromID, key, val, err := wire.UnpackStore(env.Payload)
			if err != nil {
				return
			}
			req := &RPC{
				Type:     MSG_STORE,
				RPCID:    fromWireID(env.ID),
				FromID:   fromID,
				FromAddr: from.String(),
				Key:      key,
				Value:    val,
			}
			_ = n.handleSTORE(req)
			_ = srv.Reply(from, wire.Envelope{
				ID:      env.ID,
				Type:    wire.TypeStoreAck,
				Payload: wire.PackStoreAck(n.NodeID, n.NodeID),
			})

		case wire.TypeFindValue:
			fromID, key, err := wire.UnpackFindValue(env.Payload) // samma layout som FindNode
			if err != nil {
				return
			}
			req := &RPC{
				Type:     MSG_FIND_VALUE,
				RPCID:    fromWireID(env.ID),
				FromID:   fromID,
				FromAddr: from.String(),
				Key:      key,
			}
			resp := n.handleFIND_VALUE(req)
			if resp == nil {
				return
			}
			var payload []byte
			if len(resp.Value) > 0 {
				payload = wire.PackFindValueReplyValue(n.NodeID, resp.Value)
			} else {
				ws := make([]wire.Contact, 0, len(resp.Contacts))
				for _, c := range resp.Contacts {
					ws = append(ws, wire.Contact{ID: c.ID, Address: c.Address})
				}
				payload = wire.PackFindValueReplyContacts(n.NodeID, ws)
			}
			_ = srv.Reply(from, wire.Envelope{
				ID:      env.ID,
				Type:    wire.TypeFindValueReply,
				Payload: payload,
			})
		}
	})
}

func newTestNode(t *testing.T, bind string) (*Node, *transport.UDPServer) {
	t.Helper()

	// node
	n, err := NewNode("")
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	// UDP server
	srv, err := transport.NewUDP(bind, nil)
	if err != nil {
		t.Fatalf("NewUDP: %v", err)
	}
	attachHandler(n, srv)
	srv.Start()

	n.Hostname = srv.Addr() // e.g. "127.0.0.1:9001"
	n.Trans = &UDPTransportAdapter{Srv: srv}
	return n, srv
}

func TestStoreAndFindValue(t *testing.T) {
	// starta två noder på olika portar
	a, srvA := newTestNode(t, "127.0.0.1:9001")
	defer srvA.Close()
	b, srvB := newTestNode(t, "127.0.0.1:9002")
	defer srvB.Close()

	time.Sleep(100 * time.Millisecond) // ge servrarna tid att starta

	// A känner till B (för routing + last seen)
	a.RoutingTable.AddContact(Contact{ID: b.NodeID, Address: b.Hostname})
	b.RoutingTable.AddContact(Contact{ID: a.NodeID, Address: a.Hostname})

	// === STORE: A -> B ===
	key := SHA1Key([]byte("hello"))
	val := []byte("world")

	storeReq := &RPC{
		Type:     MSG_STORE,
		RPCID:    NewRPCID(),
		FromID:   a.NodeID,
		FromAddr: a.Hostname,
		Key:      key,
		Value:    val,
	}
	storeResp, err := a.Trans.Request(b.Hostname, storeReq, 5*time.Second)
	if err != nil {
		t.Fatalf("STORE request error: %v", err)
	}
	if storeResp.Type != MSG_STORE_ACK {
		t.Fatalf("expected STORE_ACK, got %v", storeResp.Type)
	}

	// Verifiera att B faktiskt lagrat värdet lokalt
	got, ok := b.Store.Get(key)
	if !ok {
		t.Fatalf("server B saknar objektet efter STORE")
	}
	if string(got) != "world" {
		t.Fatalf("fel värde i B: %q", string(got))
	}

	// === FIND_VALUE: A -> B ===
	findReq := &RPC{
		Type:     MSG_FIND_VALUE,
		RPCID:    NewRPCID(),
		FromID:   a.NodeID,
		FromAddr: a.Hostname,
		Key:      key,
	}
	findResp, err := a.Trans.Request(b.Hostname, findReq, 5*time.Second)
	if err != nil {
		t.Fatalf("FIND_VALUE request error: %v", err)
	}
	if findResp.Type != MSG_FIND_VALUE_REPLY {
		t.Fatalf("expected FIND_VALUE_REPLY, got %v", findResp.Type)
	}
	if string(findResp.Value) != "world" {
		t.Fatalf("expected value 'world', got %q", string(findResp.Value))
	}
}
