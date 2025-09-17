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

func TestUDP_SendFromListener_UsesListeningPort(t *testing.T) {
	// Server B: capture the sender address and the envelope
	gotFrom := make(chan *net.UDPAddr, 1)
	gotEnv := make(chan wire.Envelope, 1)

	srvB, err := NewUDP("127.0.0.1:0", func(from *net.UDPAddr, env wire.Envelope) {
		gotFrom <- from
		gotEnv <- env
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srvB.Close()
	srvB.Start()

	// Server A: will send FROM its listening socket
	srvA, err := NewUDP("127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srvA.Close()
	srvA.Start()

	env := wire.Envelope{ID: wire.NewRPCID(), Type: "probe", Payload: []byte("hello")}
	if err := srvA.SendFromListener(srvB.Addr(), env); err != nil {
		t.Fatalf("SendFromListener error: %v", err)
	}

	// say B received the packet and that the source port == A's local port
	select {
	case from := <-gotFrom:
		_, aPortStr, _ := net.SplitHostPort(srvA.Addr())
		if from.Port == 0 {
			t.Fatalf("unexpected zero source port")
		}
		if got := from.String(); got == "" {
			t.Fatalf("empty from address")
		}
		// make sure ports are correct
		_, fromPortStr, _ := net.SplitHostPort(from.String())
		if fromPortStr != aPortStr {
			t.Fatalf("packet did not originate from A's listening port: got %s want %s", fromPortStr, aPortStr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for packet at B")
	}

	select {
	case e := <-gotEnv:
		if e.Type != "probe" || string(e.Payload) != "hello" {
			t.Fatalf("unexpected envelope: %+v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for envelope at B")
	}
}

func TestUDP_Reply_GoesBackToSender(t *testing.T) {
	// A waits for ack
	ackCh := make(chan wire.Envelope, 1)

	srvA, err := NewUDP("127.0.0.1:0", func(from *net.UDPAddr, env wire.Envelope) {
		if env.Type == "ack" {
			ackCh <- env
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srvA.Close()
	srvA.Start()

	// B: when it receives "probe", it replies to 'from'
	var srvB *UDPServer
	srvB, err = NewUDP("127.0.0.1:0", func(from *net.UDPAddr, env wire.Envelope) {
		if env.Type == "probe" {
			_ = srvB.Reply(from, wire.Envelope{
				ID:      env.ID, // reuse same ID
				Type:    "ack",
				Payload: []byte("ok"),
			})
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srvB.Close()
	srvB.Start()

	// A -> B (from A's listening port)
	req := wire.Envelope{ID: wire.NewRPCID(), Type: "probe", Payload: []byte("ping")}
	if err := srvA.SendFromListener(srvB.Addr(), req); err != nil {
		t.Fatalf("send error: %v", err)
	}

	select {
	case ack := <-ackCh:
		if ack.Type != "ack" || string(ack.Payload) != "ok" {
			t.Fatalf("bad ack: %+v", ack)
		}
		// Optional: verify ID correlation
		if ack.ID != req.ID {
			t.Fatalf("ack ID mismatch: got %x want %x", ack.ID, req.ID)
		}
	case <-time.After(700 * time.Millisecond):
		t.Fatal("timeout waiting for ack at A")
	}
}

func TestUDP_SendFromListener_InvalidAddr(t *testing.T) {
	srv, err := NewUDP("127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	srv.Start()

	err = srv.SendFromListener("not-a-host:abc", wire.Envelope{ID: wire.NewRPCID(), Type: "x"})
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
}
