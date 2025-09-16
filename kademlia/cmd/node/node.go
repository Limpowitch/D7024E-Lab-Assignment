package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Transport interface { // interface for network transport layer
	Request(addr string, req *RPC, timeout time.Duration) (*RPC, error)
}

type Node struct {
	NodeID       [20]byte // 160bit id
	Hostname     string
	NodeStorage  map[string]Value // Store Value struct in dictionary
	RoutingTable RoutingTable

	mu sync.RWMutex

	Store *ObjectStore // trådsäker key->[value] (nyckel = [20]byte)

	Trans Transport // din UDP-transport

	K     int // replikationsfaktor (typ 20)
	Alpha int // parallellism i lookup (typ 3)
	Beta  int // hur många kontakter en nod returnerar (3–5)
}

func NewNode(hostname string) (*Node, error) {
	var id [20]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}
	rt, _ := NewRoutingTable()

	n := &Node{
		NodeID:       id,
		Hostname:     hostname,
		RoutingTable: rt,

		NodeStorage: make(map[string]Value),

		Store: NewObjectStore(),

		// sätt standardparametrar för Kademlia
		K:     20,
		Alpha: 3,
		Beta:  5,
	}
	return n, nil
}

func (n *Node) UpdateStorage(key string, value Value) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.NodeStorage[key] = value
}

func (n *Node) LookupValue(key string) (Value, bool) {
	val, ok := n.NodeStorage[key]
	return val, ok
}

// handleSTORE lagrar värdet lokalt och kvitterar.
func (n *Node) handleSTORE(req *RPC) *RPC {
	if len(req.Value) > 0 {
		n.Store.Put(req.Key, req.Value)
	}
	// uppdatera routingtabell med avsändaren om du vill
	n.RoutingTable.AddContact(Contact{ID: req.FromID, Address: req.FromAddr})

	return &RPC{
		Type:  MSG_STORE_ACK,
		RPCID: req.RPCID,
	}
}

// handleFIND_VALUE returnerar värdet om det finns; annars närmaste kontakter.
func (n *Node) handleFIND_VALUE(req *RPC) *RPC {
	if val, ok := n.Store.Get(req.Key); ok {
		return &RPC{
			Type:  MSG_FIND_VALUE_REPLY,
			RPCID: req.RPCID,
			Value: val,
		}
	}
	closest := n.RoutingTable.FindClosestContacts(req.Key, n.Beta)
	return &RPC{
		Type:     MSG_FIND_VALUE_REPLY,
		RPCID:    req.RPCID,
		Contacts: closest,
	}
}

// ===================== M2: KLIENT-API =====================

// PutObject: hashar data -> key, hittar K närmaste och skickar STORE parallellt.
// Returnerar key + antal lyckade ACKs.
func (n *Node) PutObject(data []byte) (key [20]byte, acks int, err error) {
	key = SHA1Key(data)

	// Antingen använd din IterativeFindNode(key) här, eller börja med RT:
	targets := n.RoutingTable.FindClosestContacts(key, n.K)
	if len(targets) == 0 {
		return key, 0, errors.New("no contacts")
	}

	type res struct{ ok bool }
	ch := make(chan res, len(targets))
	for _, c := range targets {
		go func(addr string) {
			req := &RPC{
				Type:     MSG_STORE,
				RPCID:    NewRPCID(),
				FromID:   n.NodeID,
				FromAddr: n.Hostname,
				Key:      key,
				Value:    data,
			}
			_, e := n.Trans.Request(addr, req, 1500*time.Millisecond)
			ch <- res{ok: e == nil}
		}(c.Address)
	}

	for range targets {
		if (<-ch).ok {
			acks++
		}
	}
	if acks == 0 {
		return key, 0, errors.New("store failed")
	}
	return key, acks, nil
}

// GetObject: iterativ FIND_VALUE med α-parallellism och "short-circuit" när värde hittas.
func (n *Node) GetObject(key [20]byte) (value []byte, fromAddr string, err error) {
	shortlist := n.RoutingTable.FindClosestContacts(key, n.K)
	if len(shortlist) == 0 {
		return nil, "", errors.New("no contacts")
	}

	// sortera shortlist på XOR-distans
	sort.Slice(shortlist, func(i, j int) bool {
		di, dj := xor(shortlist[i].ID, key), xor(shortlist[j].ID, key)
		return lessBytes(di[:], dj[:])
	})

	queried := map[string]bool{}
	for {
		// plocka α o-queried att fråga parallellt
		batch := make([]Contact, 0, n.Alpha)
		for _, c := range shortlist {
			if !queried[c.Address] {
				batch = append(batch, c)
				if len(batch) == n.Alpha {
					break
				}
			}
		}
		if len(batch) == 0 {
			break
		}

		type rep struct {
			from string
			val  []byte
			cont []Contact
			ok   bool
		}
		ch := make(chan rep, len(batch))
		for _, c := range batch {
			queried[c.Address] = true
			go func(addr string) {
				req := &RPC{
					Type:     MSG_FIND_VALUE,
					RPCID:    NewRPCID(),
					FromID:   n.NodeID,
					FromAddr: n.Hostname,
					Key:      key,
				}
				resp, e := n.Trans.Request(addr, req, 1200*time.Millisecond)
				if e != nil || resp == nil {
					ch <- rep{ok: false}
					return
				}
				if len(resp.Value) > 0 {
					ch <- rep{from: addr, val: resp.Value, ok: true}
					return
				}
				ch <- rep{from: addr, cont: resp.Contacts, ok: true}
			}(c.Address)
		}

		improved := false
		for range batch {
			r := <-ch
			if !r.ok {
				continue
			}
			if len(r.val) > 0 {
				// cache on path (frivilligt)
				go n.Store.Put(key, r.val)
				return r.val, r.from, nil
			}
			if len(r.cont) > 0 {
				before := encodeAddrs(shortlist)
				shortlist = mergeAndSort(shortlist, r.cont, key, n.K)
				if before != encodeAddrs(shortlist) {
					improved = true
				}
			}
		}
		if !improved {
			break
		}
	}
	return nil, "", errors.New("not found")
}

// ===================== Små hjälpfunktioner =====================

func xor(a, b [20]byte) (r [20]byte) {
	for i := 0; i < 20; i++ {
		r[i] = a[i] ^ b[i]
	}
	return
}

func lessBytes(a, b []byte) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return len(a) < len(b)
}

func encodeAddrs(cs []Contact) string {
	buf := make([]byte, 0, len(cs)*6)
	for _, c := range cs {
		buf = append(buf, c.Address...)
		buf = append(buf, ';')
	}
	return string(buf)
}

func mergeAndSort(base, add []Contact, key [20]byte, k int) []Contact {
	seen := map[string]bool{}
	out := make([]Contact, 0, len(base)+len(add))
	for _, c := range append(base, add...) {
		if c.Address == "" {
			continue
		}
		if !seen[c.Address] {
			seen[c.Address] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := xor(out[i].ID, key), xor(out[j].ID, key)
		return lessBytes(di[:], dj[:])
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}
