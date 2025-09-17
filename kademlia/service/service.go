package service

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

var ErrTimeout = errors.New("rpc timeout")

type NodeID = [20]byte // local alias; avoids importing node
type FindNodeHandler func(target NodeID) []byte
type SeenHook func(addr string, peerID [20]byte)

type Service struct {
	udp *transport.UDPServer

	// in-flight requests: RPCID -> response chan
	mu      sync.Mutex
	waiters map[wire.RPCID]chan wire.Envelope

	SelfID     [20]byte
	OnSeen     SeenHook //just call this when we learn another nodes id
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
		Payload: service.SelfID[:], // this is [20]byte size
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

	// send from listening socket (source port is the server port (us))
	if err := service.udp.SendFromListener(to, env); err != nil {
		service.mu.Lock()
		delete(service.waiters, env.ID)
		service.mu.Unlock()
		return wire.Envelope{}, err
	}

	// await (just block on waiter channel or context timeout if that can even happen)
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

func (s *Service) FindNode(ctx context.Context, to string, target NodeID) ([]byte, error) {
	req := wire.Envelope{
		ID:      wire.NewRPCID(),
		Type:    "FIND_NODE",
		Payload: target[:], // 20B
	}
	resp, err := s.sendAndWait(ctx, to, req)
	if err != nil {
		return nil, err
	}
	if resp.Type != "FIND_NODE_RESP" {
		return nil, errors.New("unexpected response type: " + resp.Type)
	}
	return resp.Payload, nil // raw bytes; node layer will decode
}

func (service *Service) onPacket(from *net.UDPAddr, env wire.Envelope) {
	//fmt.Println("onPacket:", env.Type, "from", from)

	switch env.Type {
	case "PING":
		var pid [20]byte
		if len(env.Payload) >= 20 {
			copy(pid[:], env.Payload[:20])
			if service.OnSeen != nil {
				service.OnSeen(from.String(), pid)
			}
		}
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "PONG"})

	case "PONG":
		// wake up waiters
		service.mu.Lock()
		if ch, ok := service.waiters[env.ID]; ok {
			delete(service.waiters, env.ID)
			ch <- env
		}
		service.mu.Unlock()
	case "FIND_NODE":
		var target NodeID
		if len(env.Payload) >= 20 {
			copy(target[:], env.Payload[:20])
		}
		var payload []byte
		if service.OnFindNode != nil {
			payload = service.OnFindNode(target) // already encoded contact list
		}
		_ = service.udp.Reply(from, wire.Envelope{
			ID:      env.ID,
			Type:    "FIND_NODE_RESP",
			Payload: payload,
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
