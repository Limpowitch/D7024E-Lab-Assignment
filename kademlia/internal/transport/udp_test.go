// kademlia/transport/udp_test.go
package transport

import (
	"net"
	"testing"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

func TestUDP_SendRecv(t *testing.T) {
	got := make(chan wire.Envelope, 1)
	s, err := NewUDP(":0", func(from *net.UDPAddr, env wire.Envelope) {
		got <- env
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Start()

	env := wire.Envelope{ID: wire.NewRPCID(), Type: "msg", Payload: []byte("hello")}
	if err := s.Send(s.Addr(), env); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-got:
		if e.Type != "msg" || string(e.Payload) != "hello" {
			t.Fatalf("unexpected %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
