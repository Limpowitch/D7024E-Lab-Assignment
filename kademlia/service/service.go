package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

var ErrTimeout = errors.New("rpc timeout")

type Service struct {
	udp *transport.UDPServer

	// in-flight requests: RPCID -> response chan
	mu      sync.Mutex
	waiters map[wire.RPCID]chan wire.Envelope

	// might not be needed but kinda slick to have?????
	SelfID   [20]byte
	SelfAddr string
}

func New(bind string, selfID [20]byte, selfAddr string) (*Service, error) {
	s := &Service{
		waiters:  make(map[wire.RPCID]chan wire.Envelope),
		SelfID:   selfID,
		SelfAddr: selfAddr,
	}
	udp, err := transport.NewUDP(bind, s.onPacket)
	if err != nil {
		return nil, err
	}
	s.udp = udp
	return s, nil
}

func (s *Service) Start()           { s.udp.Start() }
func (s *Service) Addr() string     { return s.udp.Addr() }
func (s *Service) Close() error     { return s.udp.Close() }
func (s *Service) DialAddr() string { return s.udp.Addr() }
func (service *Service) Ping(ctx context.Context, to string) error {
	request := wire.Envelope{
		ID:      wire.NewRPCID(),
		Type:    "PING",
		Payload: nil, // nil for ping should be reasonable?
	}

	_, err := service.sendAndWait(ctx, to, request)
	return err
}

// ---- core request/respons functionality ----

func (service *Service) sendAndWait(ctx context.Context, to string, env wire.Envelope) (wire.Envelope, error) {
	// register waiter
	ch := make(chan wire.Envelope, 1)
	service.mu.Lock()
	service.waiters[env.ID] = ch
	service.mu.Unlock()

	// send
	if err := service.udp.SendFromListener(to, env); err != nil {
		service.mu.Lock()
		delete(service.waiters, env.ID)
		service.mu.Unlock()
		return wire.Envelope{}, err
	}

	// await
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		service.mu.Lock()
		delete(service.waiters, env.ID)
		service.mu.Unlock()
		return wire.Envelope{}, ErrTimeout
	}
}

func (service *Service) onPacket(from *net.UDPAddr, env wire.Envelope) {
	fmt.Println("onPacket:", env.Type, "from", from)

	switch env.Type {
	case "PING":
		// immediately reply: PONG (we should reuse same ID?)
		pong := wire.Envelope{ID: env.ID, Type: "PONG", Payload: nil}
		_ = service.udp.Reply(from, pong)

	case "PONG":
		// wake up waiters
		service.mu.Lock()
		if ch, ok := service.waiters[env.ID]; ok {
			delete(service.waiters, env.ID)
			ch <- env
		}
		service.mu.Unlock()

	default:
		// senare: FIND_NODE, STORE, liknande ALBIN HÄR FINNS COOL PLATS HEHE
	}
}
