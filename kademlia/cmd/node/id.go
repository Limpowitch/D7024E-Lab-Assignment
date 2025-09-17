package node

import (
	"crypto/rand"
	"math/big"
)

const IDBytes = 20 // 160 bits

type NodeID [IDBytes]byte

// random 160-bit ID for testing purposes
func RandomNodeID() (id NodeID) { _, _ = rand.Read(id[:]); return }

// returns XOR distance for comparisons
func Distance(a, b NodeID) *big.Int {
	var x [IDBytes]byte
	for i := 0; i < IDBytes; i++ {
		x[i] = a[i] ^ b[i]
	}
	return new(big.Int).SetBytes(x[:])
}

// return true if x is closer to target than y
func Closer(target, x, y NodeID) bool {
	dx := Distance(target, x)
	dy := Distance(target, y)
	return dx.Cmp(dy) < 0
}

func (id NodeID) IsZero() bool {
	var z NodeID
	return id == z
}
