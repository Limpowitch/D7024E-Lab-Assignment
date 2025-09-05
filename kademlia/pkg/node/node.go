package node

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
)

type Node struct {
	NodeID      *big.Int         `json:"node_id"` // 160bit id
	Hostname    string           `json:"hostname"`
	NodeStorage map[string]Value `json:"node_storage"` // Store Value struct in dictionary

	mu sync.RWMutex
}

func NewNode(hostname string) (*Node, error) {
	// Generate 160-bit random number (20 bytes)
	randomBytes := make([]byte, 20)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	// Convert bytes to big.Int
	nodeID := new(big.Int).SetBytes(randomBytes)

	return &Node{
		NodeID:      nodeID,
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
