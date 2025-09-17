package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

var ErrTimeout = errors.New("rpc timeout")

type FindNodeHandler func(target node.NodeID) []node.Contact

type Service struct {
	udp *transport.UDPServer

	// in-flight requests: RPCID -> response chan
	mu      sync.Mutex
	waiters map[wire.RPCID]chan wire.Envelope

	// might not be needed but kinda slick to have?????
	SelfID     [20]byte
	SelfAddr   string
	OnFindNode FindNodeHandler
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

func (service *Service) FindNode(ctx context.Context, to string, target node.NodeID) ([]node.Contact, error) {
	request := wire.Envelope{
		ID:      wire.NewRPCID(),
		Type:    "FIND_NODE",
		Payload: target[:],
	}
	response, err := service.sendAndWait(ctx, to, request)
	if err != nil {
		return nil, err
	}
	if response.Type != "FIND_NODE_RESP" {
		return nil, ErrTimeout
	}
	return node.UnmarshalContactList(response.Payload)
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
	case "FIND_NODE":
		var target node.NodeID
		if len(env.Payload) >= node.IDBytes {
			copy(target[:], env.Payload[:node.IDBytes])
		}
		var contacts []node.Contact
		if service.OnFindNode != nil {
			contacts = service.OnFindNode(target)
		}
		pl := node.MarshalContactList(contacts)
		_ = service.udp.Reply(from, wire.Envelope{
			ID:      env.ID,
			Type:    "FIND_NODE_RESP",
			Payload: pl,
		})
	case "FIND_NODE_RESP":
		// exactly as pong. maybe create function which both can call upon?
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
