package main

import (
	"flag"
	"net"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

func main() {
	var bind string
	flag.StringVar(&bind, "bind", ":9999", "udp bind address")
	flag.Parse()

	node, err := NewNode("")
	if err != nil { /* ... */
	}

	srv, err := transport.NewUDP(bind, nil)
	if err != nil { /* ... */
	}

	node.Hostname = srv.Addr()
	node.Trans = &UDPTransportAdapter{Srv: srv}

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
			_ = node.handleSTORE(req)
			_ = srv.Reply(from, wire.Envelope{
				ID:      env.ID,
				Type:    wire.TypeStoreAck,
				Payload: wire.PackStoreAck(node.NodeID, node.NodeID),
			})

		case wire.TypeFindValue: // <-- REQUEST
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
			resp := node.handleFIND_VALUE(req)
			if resp == nil {
				return
			}

			var payload []byte
			if len(resp.Value) > 0 {
				payload = wire.PackFindValueReplyValue(node.NodeID, resp.Value)
			} else {
				ws := make([]wire.Contact, 0, len(resp.Contacts))
				for _, c := range resp.Contacts {
					ws = append(ws, wire.Contact{ID: c.ID, Address: c.Address})
				}
				payload = wire.PackFindValueReplyContacts(node.NodeID, ws)
			}
			_ = srv.Reply(from, wire.Envelope{
				ID:      env.ID,
				Type:    wire.TypeFindValueReply,
				Payload: payload,
			})
		}

	})
}
