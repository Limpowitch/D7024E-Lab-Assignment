package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

func main() {
	var bind string
	flag.StringVar(&bind, "bind", ":9999", "udp bind address")
	flag.Parse()

	// Skapa noden
	node, err := NewNode("")
	if err != nil {
		fmt.Println("NewNode error:", err)
		os.Exit(1)
	}

	// Starta UDP-servern med handlern som anropar dina Node-handlers
	srv, err := transport.NewUDP(bind, func(from *net.UDPAddr, env wire.Envelope) {
		switch env.Type {
		case wire.TypeStore: // eller "STORE" om du inte lagt till konstanten än
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
				Type:    wire.TypeStoreAck,              // eller "STORE_ACK"
				Payload: wire.PackStoreAck(node.NodeID), // enkel ack
			})

		case wire.TypeFindValue: // eller "FIND_VALUE"
			fromID, key, err := wire.UnpackFindValue(env.Payload)
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
				Type:    wire.TypeFindValueReply, // eller "FIND_VALUE_REPLY"
				Payload: payload,
			})
		}
	})
	if err != nil {
		fmt.Println("udp listen error:", err)
		os.Exit(1)
	}

	// Gör så Node kan skicka synkrona RPC:er via UDP
	node.Hostname = srv.Addr()
	node.Trans = &UDPTransportAdapter{Srv: srv}

	srv.Start()
	fmt.Printf("node listening on %s — Ctrl+C to stop\n", srv.Addr())

	// Kör tills Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nbye")
	_ = srv.Close()
}
