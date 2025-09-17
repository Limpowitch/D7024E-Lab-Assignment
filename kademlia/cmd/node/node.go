package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/service"
)

const K = 20 // bucket size / #contacts to return; keep consistent across the project

type Node struct {
	NodeID       [20]byte
	Addr         string // bind addr we listen on (e.g. "127.0.0.1:9999")
	RoutingTable *RoutingTable
	Store        map[string]Value // your existing in-memory value store
	Svc          *service.Service

	mu sync.RWMutex
}

func NewNode(bind string) (*Node, error) {
	// generate a random 160-bit node ID
	var id [20]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	// full ID space [0..2^160-1]
	var lower, upper [20]byte
	for i := range upper {
		upper[i] = 0xff
	}

	rt, err := NewRoutingTable(id, lower, upper)
	if err != nil {
		return nil, err
	}

	// create the RPC service bound to UDP
	svc, err := service.New(bind, id, "")
	if err != nil {
		return nil, err
	}

	n := &Node{
		NodeID:       id,
		Addr:         bind,
		RoutingTable: &rt,
		Store:        make(map[string]Value),
		Svc:          svc,
	}

	// When we learn a peer’s ID (from PING payload), update our routing table.
	n.Svc.OnSeen = func(addr string, peerID [20]byte) {
		if !isZero(peerID) {
			n.RoutingTable.Update(Contact{ID: peerID, Addr: addr})
		}
	}

	// When asked FIND_NODE(target), reply with k closest from our RT.
	n.Svc.OnFindNode = func(target [20]byte) []byte {
		cs := n.RoutingTable.Closest(target, K)
		return MarshalContactList(cs) // your node-side encoder
	}

	return n, nil
}

func (n *Node) FindNode(to string, target [20]byte) ([]Contact, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	payload, err := n.Svc.FindNode(ctx, to, target)
	if err != nil {
		return nil, err
	}
	return UnmarshalContactList(payload)
}

func (n *Node) Start()       { n.Svc.Start() }
func (n *Node) Close() error { return n.Svc.Close() }

func isZero(id [20]byte) bool { var z [20]byte; return id == z }

// --- convenience methods the CLI / higher layers can use ---

func (n *Node) PingPeer(targetAddr string) error {
	// service.Ping sends our NodeID in the payload and waits for PONG
	ctx := defaultTimeoutContext()
	defer ctx.Cancel()
	return n.Svc.Ping(ctx.Ctx, targetAddr)
}

// Storage helpers you already had (renamed for brevity)
func (n *Node) Put(key string, value Value) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Store[key] = value
}
func (n *Node) Get(key string) (Value, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	v, ok := n.Store[key]
	return v, ok
}

// tiny helper for timeouts (keeps service calls uniform)
type cancelCtx struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func defaultTimeoutContext() cancelCtx {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	return cancelCtx{Ctx: ctx, Cancel: cancel}
}
