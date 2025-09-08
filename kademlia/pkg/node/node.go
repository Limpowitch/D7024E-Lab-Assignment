package node

import (
	"crypto/rand"
	"fmt"
	"sync"
)

type Node struct {
	NodeID      [20]byte         `json:"node_id"` // 160bit id
	Hostname    string           `json:"hostname"`
	NodeStorage map[string]Value `json:"node_storage"` // Store Value struct in dictionary

	mu sync.RWMutex
}

func NewNode(hostname string) (*Node, error) {
	var id [20]byte

	_, err := rand.Read(id[:]) // fills all 160 bits
	if err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	return &Node{
		NodeID:      id,
		Hostname:    hostname,
		NodeStorage: make(map[string]Value),
	}, nil
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
