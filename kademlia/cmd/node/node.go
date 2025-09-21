package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/service"
)

const K = 20 // bucket size

type Node struct {
	NodeID       [20]byte
	Addr         string // bind addr we listen on (e.g. "127.0.0.1:9999")
	RoutingTable *RoutingTable
	Store        map[string]Value
	Svc          *service.Service
	adv          string

	mu sync.RWMutex
}

func NewNode(bind string, adv string) (*Node, error) {
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
	svc, err := service.New(bind, id, adv)
	if err != nil {
		return nil, err
	}

	n := &Node{
		NodeID:       id,
		Addr:         bind,
		RoutingTable: &rt,
		Store:        make(map[string]Value),
		Svc:          svc,
		adv:          adv,
	}

	// Advertised address already handled in your constructor; not shown here.

	// ADMIN_PUT: compute key, do lookup(key), store to K closest, return key.
	n.Svc.OnAdminPut = func(value []byte) ([20]byte, error) {
		key := SHA1ID(value)

		// Populate RT around key (uses your iterative FindNode walk)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = n.LookupNode(ctx, key)

		// Fan-out STORE to K closest we currently know
		cs := n.RoutingTable.Closest(key, K)

		log.Printf("[admin-put] key=%x K=%d closest=%d", key[:4], K, len(cs))

		if len(cs) == 0 {
			// Still allow storing locally (optionally), but Kademlia usually wants at least some peers.
			// Store locally for demo ergonomics:
			n.mu.Lock()
			n.Store[string(key[:])] = Value{Data: append([]byte(nil), value...)}
			n.mu.Unlock()
			return key, nil
		}
		// Send STORE concurrently (best-effort)
		var wg sync.WaitGroup
		for _, c := range cs {
			c := c
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
				_ = n.Svc.Store(ctx2, c.Addr, key, value)
				cancel2()
			}()
		}
		wg.Wait()
		return key, nil
	}

	// ADMIN_GET: iterative get using our RT (and any seeds already known).
	n.Svc.OnAdminGet = func(key [20]byte) ([]byte, bool) {
		n.mu.RLock()
		if v, ok := n.Store[string(key[:])]; ok && len(v.Data) > 0 {
			out := append([]byte(nil), v.Data...)
			n.mu.RUnlock()
			return out, true
		}
		n.mu.RUnlock()

		seeds := n.RoutingTable.Closest(key, K)
		if len(seeds) == 0 {
			return nil, false
		}

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		val, _, err := n.GetValueIterative(ctx, key, seeds)
		if err == nil && val != "" {
			return []byte(val), true
		}
		return nil, false
	}

	// node/node.go (inside NewNode after n.Svc is created)
	n.Svc.OnDumpRT = func() []byte {
		var all []Contact
		n.RoutingTable.mu.RLock()
		for _, b := range n.RoutingTable.BucketList {
			b.mu.RLock()
			all = append(all, b.Contacts...)
			b.mu.RUnlock()
		}
		n.RoutingTable.mu.RUnlock()
		return MarshalContactList(all) // node’s own marshal
	}

	// when we learn another nodes id (from ping i guess?) we update our routing table. done here initially
	n.Svc.OnSeen = func(addr string, peerID [20]byte) {
		if !isZero(peerID) {
			n.RoutingTable.Update(Contact{ID: peerID, Addr: addr})
		}
	}

	// when asked FIND_NODE we reply with k closest from our own routing table
	n.Svc.OnFindNode = func(target [20]byte) []byte {
		// start with our own contact so callers learn at least one node
		self := Contact{ID: n.NodeID, Addr: n.adv}
		cs := n.RoutingTable.Closest(target, K)

		// avoid duplicate if we already have ourselves in RT (unlikely early)
		out := []Contact{self}
		for _, c := range cs {
			if c.ID != self.ID {
				out = append(out, c)
			}
		}
		return MarshalContactList(out)
	}

	n.Svc.OnStore = func(key [20]byte, val []byte) {
		n.mu.Lock()
		n.Store[string(key[:])] = Value{Data: append([]byte(nil), val...)} // copy for safety
		n.mu.Unlock()

		log.Printf("[node] STORED key=%x len=%d at %s", key[:], len(val), n.Svc.Addr())
	}

	n.Svc.OnFindValue = func(key [20]byte) (val []byte, contactsPayload []byte) {
		n.mu.RLock()
		v, ok := n.Store[string(key[:])]
		n.mu.RUnlock()
		if ok {
			// DEBUG
			log.Printf("[node] FIND_VALUE HIT key=%x at %s", key[:], n.Svc.Addr())
			return append([]byte(nil), v.Data...), nil
		}
		cs := n.RoutingTable.Closest(key, K)
		// DEBUG
		log.Printf("[node] FIND_VALUE MISS key=%x returning %d contacts at %s",
			key[:], len(cs), n.Svc.Addr())
		return nil, MarshalContactList(cs)
	}

	return n, nil
}

func (n *Node) AdvertisedAddr() string {
	if n.Svc.SelfAddr != "" {
		return n.Svc.SelfAddr
	}
	return n.Svc.Addr() // fallback to real bound address
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

func (n *Node) Start() {
	n.Svc.Start()
	go n.bootstrap()
}

func (n *Node) bootstrap() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = n.LookupNode(ctx, n.NodeID)
}

func (n *Node) Close() error {
	return n.Svc.Close()
}

func isZero(id [20]byte) bool {
	var z [20]byte
	return id == z
}

// storag helpers
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
