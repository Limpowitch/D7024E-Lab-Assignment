package main

import (
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

type UDPTransportAdapter struct {
	Srv *transport.UDPServer
}

func toWireID(id [20]byte) (w wire.RPCID) {
	copy(w[:], id[:])
	return
}
func fromWireID(w wire.RPCID) (id [20]byte) {
	copy(id[:], w[:])
	return
}
func (a *UDPTransportAdapter) Request(addr string, req *RPC, timeout time.Duration) (*RPC, error) {
	env := wire.Envelope{ID: toWireID(req.RPCID)}

	switch req.Type {
	case MSG_STORE:
		env.Type = wire.TypeStore
		env.Payload = wire.PackStore(req.FromID, req.Key, req.Value)

	case MSG_FIND_VALUE:
		env.Type = wire.TypeFindValue                         // <-- REQUEST, inte Reply
		env.Payload = wire.PackFindValue(req.FromID, req.Key) // (fromID, key)
	}

	reply, err := a.Srv.Request(addr, env, timeout)
	if err != nil {
		return nil, err
	}

	out := &RPC{RPCID: fromWireID(reply.ID)}
	switch reply.Type {
	case wire.TypeStoreAck:
		out.Type = MSG_STORE_ACK

	case wire.TypeFindValueReply:
		fromID, hasValue, value, contacts, err := wire.UnpackFindValueReply(reply.Payload)
		if err != nil {
			return nil, err
		}
		out.Type = MSG_FIND_VALUE_REPLY
		out.FromID = fromID
		out.FromAddr = addr
		if hasValue {
			out.Value = value
		} else {
			out.Contacts = make([]Contact, 0, len(contacts))
			for _, c := range contacts {
				out.Contacts = append(out.Contacts, Contact{ID: c.ID, Address: c.Address})
			}
		}
	}
	return out, nil
}
