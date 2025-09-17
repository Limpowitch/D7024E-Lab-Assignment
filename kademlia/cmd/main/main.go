// kademlia/cmd/main/main.go
package main

// full disclosure: this is entirely chat-gpt:ed based upon my earlier main for sprint 0. Just wanted to test how docker comes into play once again!

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	knode "github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

func main() {
	bind := flag.String("bind", ":9999", "udp bind (ip:port or :port)")
	ping := flag.String("ping", "", "client-only: send a PING to addr (host:port) and exit")
	idHex := flag.String("id", "", "optional 40-hex NodeID for --ping client (random if empty)")
	flag.Parse()

	if *ping != "" {
		// client-only mode: NO listener, use ephemeral :0 and exit after PONG (or timeout)
		var selfID [20]byte
		if *idHex != "" {
			b, err := hex.DecodeString(*idHex)
			if err != nil || len(b) != 20 {
				log.Fatalf("invalid --id (need 40 hex chars): %v", *idHex)
			}
			copy(selfID[:], b)
		} else {
			if _, err := rand.Read(selfID[:]); err != nil {
				log.Fatal("rand ID:", err)
			}
		}
		if err := clientPing(*ping, selfID); err != nil {
			log.Fatal("PING failed:", err)
		}
		fmt.Println("PING ok →", *ping)
		return
	}

	// server mode
	n, err := knode.NewNode(*bind)
	if err != nil {
		log.Fatal(err)
	}
	defer n.Close()
	n.Start()
	log.Println("node up on", n.Svc.Addr())
	select {} // keep running
}

// clientPing sends a wire.Envelope{Type: PING, Payload: 20B selfID}
// from an ephemeral UDP port and waits for matching PONG.
func clientPing(to string, selfID [20]byte) error {
	conn, err := net.Dial("udp", to) // binds to :0 automatically
	if err != nil {
		return err
	}
	defer conn.Close()

	req := wire.Envelope{
		ID:      wire.NewRPCID(),
		Type:    "PING",
		Payload: selfID[:],
	}
	if _, err := conn.Write(req.Marshal()); err != nil {
		return err
	}

	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}

	resp, err := wire.Unmarshal(buf[:n])
	if err != nil {
		return err
	}
	if resp.Type != "PONG" || resp.ID != req.ID {
		return fmt.Errorf("unexpected response: type=%q idmatch=%v", resp.Type, resp.ID == req.ID)
	}
	return nil
}
