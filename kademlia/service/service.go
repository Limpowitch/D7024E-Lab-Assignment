package service

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/internal/transport"
	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

var ErrTimeout = errors.New("rpc timeout")

// callbacks for server
type NodeID = [20]byte // local alias; avoids importing node
type FindNodeHandler func(target NodeID) []byte
type SeenHook func(addr string, peerID [20]byte) // added it just for qualifying later on
type StoreHandler func(key [20]byte, val []byte)
type FindValueHandler func(key [20]byte) (val []byte, contactsPayload []byte)
type DumpRTHandler func() []byte
type ExitHandler func()

type Service struct {
	udp *transport.UDPServer

	mu      sync.Mutex
	waiters map[wire.RPCID]chan wire.Envelope

	SelfID      [20]byte
	OnSeen      SeenHook //just call this when we learn another nodes id
	SelfAddr    string
	OnFindNode  FindNodeHandler
	OnStore     StoreHandler
	OnFindValue FindValueHandler
	OnDumpRT    DumpRTHandler
	OnExit      ExitHandler

	OnAdminPut func(value []byte) (key [20]byte, err error)
	OnAdminGet func(ctx context.Context, key [20]byte) (value []byte, ok bool)
}

// Creates a new Service listening on bind (UDP addr) and identifying as selfID
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

func (s *Service) AdminExit(ctx context.Context, to string) error {
	req := wire.Envelope{ID: wire.NewRPCID(), Type: "ADMIN_EXIT"}
	resp, err := s.sendAndWait(ctx, to, req)
	if err != nil {
		return err
	}
	if resp.Type != "ADMIN_EXIT_OK" {
		return errors.New("bad ADMIN_EXIT response: " + resp.Type)
	}
	return nil
}

// low-level RPCs (used by node layer)
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

// STORE RPC that store a value and returns acks that it was stored
func (service *Service) Store(ctx context.Context, to string, key [20]byte, value []byte) error {
	// build payload: key(20) + len(2) + value
	if len(value) > 65535 {
		return errors.New("value too large (>65535)")
	}
	payload := make([]byte, 20+2+len(value))
	copy(payload[:20], key[:])
	payload[20] = byte(len(value) >> 8)
	payload[21] = byte(len(value))
	copy(payload[22:], value)

	req := wire.Envelope{ID: wire.NewRPCID(), Type: "STORE", Payload: payload}
	_, err := service.sendAndWait(ctx, to, req)
	return err
}

type FindValueResult struct {
	Value    []byte // if non-nil, we got the value
	Contacts []byte // encoded contacts payload; decode in node layer (UnmarshalContactList)
}

// AdminRT asks a running node to dump its routing table
func (s *Service) AdminRT(ctx context.Context, to string) ([]byte, error) {
	req := wire.Envelope{ID: wire.NewRPCID(), Type: "ADMIN_RT"}
	resp, err := s.sendAndWait(ctx, to, req)
	if err != nil {
		return nil, err
	}
	if resp.Type != "ADMIN_RT_RESP" {
		return nil, errors.New("unexpected response: " + resp.Type)
	}
	return resp.Payload, nil
}

// FindValue looks for a value or contacts for a given key
func (service *Service) FindValue(ctx context.Context, to string, key [20]byte) (FindValueResult, error) {
	req := wire.Envelope{ID: wire.NewRPCID(), Type: "FIND_VALUE", Payload: key[:]}
	resp, err := service.sendAndWait(ctx, to, req)
	if err != nil {
		return FindValueResult{}, err
	}
	switch resp.Type {
	case "FIND_VALUE_VAL":
		return FindValueResult{Value: resp.Payload}, nil
	case "FIND_VALUE_CONT":
		return FindValueResult{Contacts: resp.Payload}, nil
	default:
		return FindValueResult{}, errors.New("unexpected response: " + resp.Type)
	}
}

// handles the incomgin packets and the contact the right handlers
func (service *Service) onPacket(from *net.UDPAddr, env wire.Envelope) {
	//fmt.Println("onPacket:", env.Type, "from", from)

	switch env.Type {
	case "PING":
		//log.Printf("[service] PING from %s id=%x", from.String(), env.ID[:4])
		var pid [20]byte
		if len(env.Payload) >= 20 {
			copy(pid[:], env.Payload[:20])
			if service.OnSeen != nil {
				service.OnSeen(from.String(), pid)
			}
		}
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "PONG", Payload: service.SelfID[:]})

	case "PONG":
		var pid [20]byte
		if len(env.Payload) >= 20 {
			copy(pid[:], env.Payload[:20])
			if service.OnSeen != nil {
				service.OnSeen(from.String(), pid)
			}
		}

		// wake up waiters
		//log.Printf("[service] PONG from %s id=%x", from.String(), env.ID[:4])
		service.wake(env.ID, env)
	case "FIND_NODE":
		//log.Printf("[service] FIND_NODE from %s id=%x", from.String(), env.ID[:4])
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
		//log.Printf("[service] FIND_NODE_RESP from %s id=%x", from.String(), env.ID[:4])
		// exactly as pong. maybe create function which both can call upon?
		service.wake(env.ID, env)

	case "STORE":
		log.Printf("[service] STORE from %s id=%x", from.String(), env.ID[:4])

		// 20 + 2, so if less, it must be a invalid/bad request
		if len(env.Payload) < 22 {
			return
		}

		var key [20]byte
		copy(key[:], env.Payload[:20])
		l := int(env.Payload[20])<<8 | int(env.Payload[21])
		if 22+l > len(env.Payload) {
			return
		}
		val := make([]byte, l)
		copy(val, env.Payload[22:22+l])

		if service.OnStore != nil {
			service.OnStore(key, val)
		}
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "STORE_ACK"})

	case "STORE_ACK":
		log.Printf("[service] STORE_ACK from %s id=%x", from.String(), env.ID[:4])
		// exactly as pong. maybe create function which both can call upon?
		service.wake(env.ID, env)

	case "FIND_VALUE":
		log.Printf("[service] FIND_VALUE from %s id=%x", from.String(), env.ID[:4])

		var key [20]byte
		if len(env.Payload) >= 20 {
			copy(key[:], env.Payload[:20])
		}
		var reply wire.Envelope
		reply.ID = env.ID

		if service.OnFindValue != nil {
			val, contactsPayload := service.OnFindValue(key)
			if val != nil {
				reply.Type = "FIND_VALUE_VAL"
				reply.Payload = val
			} else {
				reply.Type = "FIND_VALUE_CONT"
				reply.Payload = contactsPayload // can be nil/empty?
			}
		} else {
			// default handle: no value or contacts
			reply.Type = "FIND_VALUE_CONT"
			reply.Payload = nil
		}
		_ = service.udp.Reply(from, reply)

	case "FIND_VALUE_VAL":
		log.Printf("[service] FIND_VALUE_VAL from %s id=%x", from.String(), env.ID[:4])
		// exactly as pong. maybe create function which both can call upon?
		service.wake(env.ID, env)
	case "FIND_VALUE_CONT":
		log.Printf("[service] FIND_VALUE_CONT from %s id=%x", from.String(), env.ID[:4])
		// exactly as pong. maybe create function which both can call upon?
		service.wake(env.ID, env)
	case "ADMIN_RT":
		var pl []byte
		if service.OnDumpRT != nil {
			pl = service.OnDumpRT()
		}
		_ = service.udp.Reply(from, wire.Envelope{
			ID:      env.ID,
			Type:    "ADMIN_RT_RESP",
			Payload: pl,
		})

	case "ADMIN_RT_RESP":
		service.wake(env.ID, env)
	case "ADMIN_PUT":
		go service.handleAdminPut(from, env)

	case "ADMIN_GET":
		go service.handleAdminGet(from, env)

	case "ADMIN_PUT_RESP":
		service.wake(env.ID, env)

	case "ADMIN_GET_VAL":
		service.wake(env.ID, env)

	case "ADMIN_GET_NOTFOUND":
		service.wake(env.ID, env)

	case "ADMIN_EXIT":
		// Reply first so the client doesn't hang, then terminate asynchronously.
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_EXIT_OK"})

		go func() {
			// If the node installed a hook, call it; otherwise self-signal.
			if service.OnExit != nil {
				service.OnExit()
				return
			}
			// Default: self-signal to unblock waitForSignal() in cmdServe
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Signal(syscall.SIGTERM)
		}()
	case "ADMIN_EXIT_OK":
		service.wake(env.ID, env)

	default:
	}
}

// AdminPut asks a running node (daemon) to store a value using its RT.
// Request:  value bytes
// Response: 20B key (SHA-1)
func (s *Service) AdminPut(ctx context.Context, to string, value []byte) ([20]byte, error) {
	req := wire.Envelope{ID: wire.NewRPCID(), Type: "ADMIN_PUT", Payload: value}
	resp, err := s.sendAndWait(ctx, to, req)
	if err != nil {
		return [20]byte{}, err
	}
	if resp.Type != "ADMIN_PUT_RESP" || len(resp.Payload) != 20 {
		return [20]byte{}, errors.New("bad ADMIN_PUT response")
	}
	var key [20]byte
	copy(key[:], resp.Payload[:20])
	return key, nil
}

// AdminGet asks a running node (daemon) to resolve a key using its RT.
// Response: value (if found) or notfound.
func (s *Service) AdminGet(ctx context.Context, to string, key [20]byte) ([]byte, bool, error) {
	// derive remaining budget from ctx
	timeoutMs := uint32(10000)
	if dl, ok := ctx.Deadline(); ok {
		left := time.Until(dl)
		if left <= 0 {
			return nil, false, ctx.Err()
		}
		if left > 60*time.Second {
			left = 60 * time.Second
		}
		timeoutMs = uint32(left / time.Millisecond)
	}

	payload := make([]byte, 24)
	copy(payload[:20], key[:])
	binary.BigEndian.PutUint32(payload[20:], timeoutMs)

	req := wire.Envelope{ID: wire.NewRPCID(), Type: "ADMIN_GET", Payload: payload}
	resp, err := s.sendAndWait(ctx, to, req)
	if err != nil {
		return nil, false, err
	}

	log.Printf("[admin-get/client] resp=%s payload=%dB", resp.Type, len(resp.Payload))

	switch resp.Type {
	case "ADMIN_GET_VAL":
		return resp.Payload, true, nil
	case "ADMIN_GET_NOTFOUND":
		return nil, false, nil
	default:
		return nil, false, errors.New("bad ADMIN_GET response")
	}
}

// Helper to wake up a waiter for a given RPC ID
func (s *Service) wake(id wire.RPCID, env wire.Envelope) {
	s.mu.Lock()
	if ch, ok := s.waiters[id]; ok {
		delete(s.waiters, id)
		ch <- env
	}
	s.mu.Unlock()
}

// Handles an incoming ADMIN_PUT request
func (service *Service) handleAdminPut(from *net.UDPAddr, env wire.Envelope) {
	log.Printf("[service] ADMIN_PUT from %s", from.String())
	if service.OnAdminPut == nil {
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_PUT_RESP"})
		return
	}

	val := append([]byte(nil), env.Payload...)

	key, err := service.OnAdminPut(val)
	if err != nil {
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_PUT_RESP"})
		return
	}
	_ = service.udp.Reply(from, wire.Envelope{
		ID:      env.ID,
		Type:    "ADMIN_PUT_RESP",
		Payload: key[:],
	})
}

func (service *Service) handleAdminGet(from *net.UDPAddr, env wire.Envelope) {
	log.Printf("[service] ADMIN_GET from %s", from.String())
	if len(env.Payload) < 20 {
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_GET_NOTFOUND"})
		return
	}

	var key [20]byte
	copy(key[:], env.Payload[:20])

	// derive timeout from client payload (or default)
	timeoutMs := uint32(10000)
	if len(env.Payload) >= 24 {
		timeoutMs = binary.BigEndian.Uint32(env.Payload[20:])
		if timeoutMs == 0 {
			timeoutMs = 1
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if service.OnAdminGet == nil {
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_GET_NOTFOUND"})
		return
	}

	val, ok := service.OnAdminGet(ctx, key)
	if ok {
		log.Printf("[admin-get] FOUND -> replying ADMIN_GET_VAL with value=%q len=%d", string(val), len(val))
		_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_GET_VAL", Payload: val})
		return
	}
	log.Printf("[admin-get] NOTFOUND -> replying ADMIN_GET_NOTFOUND")
	_ = service.udp.Reply(from, wire.Envelope{ID: env.ID, Type: "ADMIN_GET_NOTFOUND"})
}
