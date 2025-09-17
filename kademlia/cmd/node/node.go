package main

import (
	"crypto/rand"
	"fmt"
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

func NewNode(hostname string) (*Node, error) { // skapar en ny Node med slumpmässigt ID
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

func (n *Node) UpdateStorage(key string, value Value) { // trådsäker uppdatering av NodeStorage
	n.mu.Lock()
	defer n.mu.Unlock()
	n.NodeStorage[key] = value
}

func (n *Node) LookupValue(key string) (Value, bool) { // trådsäker läsning av NodeStorage
	val, ok := n.NodeStorage[key]
	return val, ok
}

func (n *Node) handleSTORE(req *RPC) *RPC { // hantera STORE-förfrågan
	if len(req.Value) > 0 {
		n.Store.Put(req.Key, req.Value)
	}
	n.RoutingTable.AddContact(Contact{ID: req.FromID, Address: req.FromAddr})
	return &RPC{Type: MSG_STORE_ACK, RPCID: req.RPCID}
}

func (n *Node) handleFIND_VALUE(req *RPC) *RPC { // hantera FIND_VALUE-förfrågan
	if val, ok := n.Store.Get(req.Key); ok {
		return &RPC{Type: MSG_FIND_VALUE_REPLY, RPCID: req.RPCID, Value: val}
	}
	closest := n.RoutingTable.FindClosestContacts(req.Key, n.Beta) // returnera närmaste kontakter
	return &RPC{Type: MSG_FIND_VALUE_REPLY, RPCID: req.RPCID, Contacts: closest}
}
