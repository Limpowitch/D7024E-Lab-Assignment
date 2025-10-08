package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/service"
)

const K = 2 // bucket size

// A Kademlia node
type Node struct {
	NodeID       [20]byte
	Addr         string // bind addr we listen on (e.g. "127.0.0.1:9999")
	RoutingTable *RoutingTable
	Store        map[string]Value
	Svc          *service.Service
	adv          string
	ttl          time.Duration // how long values live
	refreshEvery time.Duration // how often origin republisher runs

	mu sync.RWMutex
}

// Creates a new node
func NewNode(bind string, adv string, ttl time.Duration, refreshEvery time.Duration) (*Node, error) {
	if refreshEvery <= 0 {
		refreshEvery = ttl / 2
		if refreshEvery <= 0 {
			refreshEvery = 30 * time.Second // should be a safe floor (for demos at least)
		}
	}

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

	/// IMPORTANT, HERE IS WHERE WE DECIDE OUR SPLIT METRIC
	var b = 5

	rt, err := NewRoutingTable(id, lower, upper, K, b)
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
		ttl:          ttl,
		refreshEvery: refreshEvery,
	}

	n.Svc.SetOnRefresh(func(key [20]byte) {
		n.mu.Lock()
		if v, ok := n.Store[string(key[:])]; ok {
			v.ExpiresAt = time.Now().Add(n.ttl)
			n.Store[string(key[:])] = v
		}
		n.mu.Unlock()
	})

	n.Svc.SetOnAdminForget(func(key [20]byte) bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, ok := n.Store[string(key[:])]; !ok {
			return false
		}
		delete(n.Store, string(key[:]))
		return true
	})

	n.Svc.SetOnExit(func() {
		// need to unblock signal is serve. so we self signal:
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})

	// ADMIN_PUT: compute key, do lookup(key), store to K closest, return key.
	n.Svc.SetOnAdminPut(func(value []byte) ([20]byte, error) {
		key := SHA1ID(value)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = n.LookupNode(ctx, key)

		cs := n.RoutingTable.Closest(key, K)
		if len(cs) == 0 {
			n.mu.Lock()
			n.Store[string(key[:])] = Value{
				Data:        append([]byte(nil), value...),
				Origin:      true,
				LastPublish: time.Now(),
				ExpiresAt:   time.Now().Add(n.ttl),
			}
			n.mu.Unlock()
			return key, nil
		}

		var wg sync.WaitGroup
		for _, c := range cs {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
				_ = n.Svc.Store(ctx2, addr, key, value)
				cancel2()
			}(c.Addr)
		}
		wg.Wait()

		// keep a local origin copy too (optional but convenient)
		n.mu.Lock()
		n.Store[string(key[:])] = Value{
			Data:        append([]byte(nil), value...),
			Origin:      true,
			LastPublish: time.Now(),
			ExpiresAt:   time.Now().Add(n.ttl),
		}
		n.mu.Unlock()

		return key, nil
	})

	// ADMIN_GET: iterative get using our RT (and any seeds already known).
	// node/node.go (inside NewNode)
	n.Svc.SetOnAdminGet(func(ctx context.Context, key [20]byte) ([]byte, bool) {
		// Local fast path
		n.mu.RLock()
		if v, ok := n.Store[string(key[:])]; ok && len(v.Data) > 0 {
			out := append([]byte(nil), v.Data...)
			n.mu.RUnlock()
			return out, true
		}
		n.mu.RUnlock()

		seeds := n.RoutingTable.Closest(key, K) // fine if empty
		val, _, err := n.GetValueIterative(ctx, key, seeds)
		if err == nil && val != "" {
			return []byte(val), true
		}
		// Optional: log for clarity — you already have similar logs.
		// log.Printf("[admin-get] MISS err=%v", err)
		return nil, false
	})

	// node/node.go (inside NewNode after n.Svc is created)
	n.Svc.SetOnDumpRT(func() []byte {
		var all []Contact
		n.RoutingTable.mu.RLock()
		for _, b := range n.RoutingTable.BucketList {
			b.mu.RLock()
			all = append(all, b.Contacts...)
			b.mu.RUnlock()
		}
		n.RoutingTable.mu.RUnlock()
		return MarshalContactList(all) // node’s own marshal
	})

	// when we learn another nodes id (from ping i guess?) we update our routing table. done here initially
	n.Svc.SetOnSeen(func(addr string, peerID [20]byte) {
		if !isZero(peerID) {
			n.RoutingTable.Update(Contact{ID: peerID, Addr: addr})
		}
	})

	// when asked FIND_NODE we reply with k closest from our own routing table
	n.Svc.SetOnFindNode(func(target [20]byte) []byte {
		// start with our own contact so callers learn at least one node
		self := Contact{ID: n.NodeID, Addr: n.AdvertisedAddr()}
		cs := n.RoutingTable.Closest(target, K)

		// avoid duplicate if we already have ourselves in RT (unlikely early)
		out := []Contact{self}
		for _, c := range cs {
			if c.ID != self.ID {
				out = append(out, c)
			}
		}
		return MarshalContactList(out)
	})

	n.Svc.SetOnStore(func(key [20]byte, val []byte) {
		n.mu.Lock()
		n.Store[string(key[:])] = Value{
			Data:      append([]byte(nil), val...),
			ExpiresAt: time.Now().Add(n.ttl),
		}
		n.mu.Unlock()
		log.Printf("[node] STORED key=%x len=%d at %s", key[:], len(val), n.Svc.Addr())
	})

	n.Svc.SetOnFindValue(func(key [20]byte) ([]byte, []byte) {
		n.mu.RLock()
		v, ok := n.Store[string(key[:])]
		n.mu.RUnlock()
		if ok {
			n.mu.Lock()
			v.ExpiresAt = time.Now().Add(n.ttl)
			n.Store[string(key[:])] = v
			n.mu.Unlock()
			return append([]byte(nil), v.Data...), nil
		}
		cs := n.RoutingTable.Closest(key, K)
		return nil, MarshalContactList(cs)
	})

	return n, nil
}

func (n *Node) startRepublisher() {
	tick := time.NewTicker(n.refreshEvery)
	go func() {
		for range tick.C {
			now := time.Now()
			var keys [][20]byte
			n.mu.RLock()
			for kStr, v := range n.Store {
				if !v.Origin {
					continue
				}
				if v.LastPublish.IsZero() || now.Sub(v.LastPublish) >= n.refreshEvery {
					var key [20]byte
					copy(key[:], []byte(kStr)[:20])
					keys = append(keys, key)
				}
			}
			n.mu.RUnlock()

			for _, key := range keys {
				cs := n.RoutingTable.Closest(key, K)
				var wg sync.WaitGroup
				for _, c := range cs {
					wg.Add(1)
					go func(addr string, key [20]byte) {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
						_ = n.Svc.Refresh(ctx, addr, key)
						cancel()
					}(c.Addr, key)
				}
				wg.Wait()

				n.mu.Lock()
				v := n.Store[string(key[:])]
				v.LastPublish = now
				n.Store[string(key[:])] = v
				n.mu.Unlock()
			}
		}
	}()
}

// Returns the adress thats being advertised to other nodes
func (n *Node) AdvertisedAddr() string {
	if n.Svc.SelfAddr != "" {
		return n.Svc.SelfAddr
	}
	return n.Svc.Addr() // fallback to real bound address
}

// Finds the given node ID
func (n *Node) FindNode(to string, target [20]byte) ([]Contact, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	payload, err := n.Svc.FindNode(ctx, to, target)
	if err != nil {
		return nil, err
	}
	return UnmarshalContactList(payload)
}

// Starts the service and bootstraps the node
func (n *Node) Start() {
	// GC ticker (U1)
	gc := time.NewTicker(1 * time.Minute)
	go func() {
		for range gc.C {
			now := time.Now()
			n.mu.Lock()
			for k, v := range n.Store {
				if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
					delete(n.Store, k)
				}
			}
			n.mu.Unlock()
		}
	}()

	// Republisher ticker (U2)
	n.startRepublisher()

	n.Svc.Start()
	go n.bootstrap()
}

// Bootstraps the node and populates its routing table
func (n *Node) bootstrap() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = n.LookupNode(ctx, n.NodeID)
}

// Closes the node and its service
func (n *Node) Close() error {
	return n.Svc.Close()
}

// Checks if a NodeID is all zeroes
func isZero(id [20]byte) bool {
	var z [20]byte
	return id == z
}
