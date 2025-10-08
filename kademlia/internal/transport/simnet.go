//go:build sim
// +build sim

package transport

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

// ---------------------- SIM config knobs ----------------------

var (
	simRNG       = rand.New(rand.NewSource(1))                                           // global RNG for sim
	simDropProb  float64                                                                 // packet drop probability [0..1]
	simLatencyFn = func(from, to string) time.Duration { return 200 * time.Microsecond } // latency function
	simSent      uint64                                                                  // number of packets sent
	simDelivered uint64                                                                  // number of packets delivered
	simDropped   uint64                                                                  // number of packets dropped
)

func ResetSimStats() { // Function for reseting the stats
	atomic.StoreUint64(&simSent, 0)
	atomic.StoreUint64(&simDelivered, 0)
	atomic.StoreUint64(&simDropped, 0)
}

func SimStats() (sent, delivered, dropped uint64) { // Function for getting the stats
	return atomic.LoadUint64(&simSent),
		atomic.LoadUint64(&simDelivered),
		atomic.LoadUint64(&simDropped)
}

func SetSeed(seed int64) { simRNG.Seed(seed) } // Function for setting the seed of the RNG
func SetDropProbability(p float64) { // Function for setting the drop probability
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	simDropProb = p
}
func SetLatency(f func(from, to string) time.Duration) { // Setter for the latency function
	if f != nil {
		simLatencyFn = f
	}
}

// Simulated network bus
type simBus struct {
	mu sync.RWMutex
	ep map[string]*UDPServer // addr -> endpoint
}

var bus = &simBus{ep: map[string]*UDPServer{}} // global singleton bus

func (b *simBus) register(addr string, s *UDPServer) error { // register a new endpoint
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.ep[addr]; ok {
		return errors.New("address already in use (sim)")
	}
	b.ep[addr] = s
	return nil
}
func (b *simBus) unregister(addr string) { // unregister an endpoint
	b.mu.Lock()
	delete(b.ep, addr)
	b.mu.Unlock()
}
func (b *simBus) get(addr string) *UDPServer { // get an endpoint by addr
	b.mu.RLock()
	s := b.ep[addr]
	b.mu.RUnlock()
	return s
}

// Simulated UDP transport implementation that matches the real UDPServer API

type handlerFn func(from *net.UDPAddr, env wire.Envelope)

type UDPServer struct {
	addr    string
	handler handlerFn
	closed  bool
	mu      sync.RWMutex
}

// Creates a new simulated UDP transport server
func NewUDP(bind string, handler func(from *net.UDPAddr, env wire.Envelope)) (*UDPServer, error) {
	s := &UDPServer{addr: bind, handler: handler}
	if err := bus.register(bind, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Function to start the server
func (s *UDPServer) Start() error { return nil }

// service.go expects string
func (s *UDPServer) Addr() string { return s.addr }

// Closes the server and unregisters it from the bus
func (s *UDPServer) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	bus.unregister(s.addr)
	return nil
}

// Sets the handler function for incoming packets
func (s *UDPServer) SetHandler(h func(from *net.UDPAddr, env wire.Envelope)) {
	s.mu.Lock()
	s.handler = h
	s.mu.Unlock()
}

// Sends a request and waits for a reply or timeout
func (s *UDPServer) Request(ctx context.Context, to string, env wire.Envelope) (wire.Envelope, error) {
	replyCh := make(chan wire.Envelope, 1)

	_ = s.enqueue(to, env, func(resp wire.Envelope) {
		select {
		case replyCh <- resp:
		default:
		}
	})

	select {
	case <-ctx.Done():
		return wire.Envelope{}, ctx.Err()
	case resp := <-replyCh:
		return resp, nil
	}
}

// Function to send a message from the listener
func (s *UDPServer) SendFromListener(to string, env wire.Envelope) error {
	_ = s.enqueue(to, env, nil)
	return nil
}

// Replies to a message from the listener
func (s *UDPServer) Reply(to *net.UDPAddr, env wire.Envelope) error {
	if to == nil {
		return errors.New("nil reply addr")
	}
	_ = s.enqueue(to.String(), env, nil)
	return nil
}

// Enqueues a message to be sent with simulated latency and drop
func (s *UDPServer) enqueue(to string, env wire.Envelope, onReply func(wire.Envelope)) bool {
	atomic.AddUint64(&simSent, 1)

	dst := bus.get(to)
	if dst == nil {
		atomic.AddUint64(&simDropped, 1) // treat unknown destination as a dropped packet
		return false
	}

	if simRNG.Float64() < simDropProb {
		atomic.AddUint64(&simDropped, 1)
		return true // silent drop; caller eventually times out
	}

	delay := simLatencyFn(s.addr, to)
	time.AfterFunc(delay, func() {
		dst.mu.RLock()
		h := dst.handler
		closed := dst.closed
		dst.mu.RUnlock()
		if closed || h == nil {
			atomic.AddUint64(&simDropped, 1)
			return
		}

		atomic.AddUint64(&simDelivered, 1)
		fromAddr, _ := net.ResolveUDPAddr("udp", s.addr)
		h(fromAddr, env)

		if onReply != nil {
			onReply(env)
		}
	})
	return true
}
