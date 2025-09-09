package node

import (
	"bytes"
	"testing"
)

func TestNewRoutingTable(t *testing.T) {
	nodeA, err := NewNode("localhost")
	if err != nil {
		t.Fatalf("failed to create nodeA: %v", err)
	}

	rt, err := NewRoutingTable(nodeA.NodeID)
	if err != nil {
		t.Fatalf("expected no error at NewRoutingTable, got %v", err)
	}

	// Check that we have exactly 160 KBuckets
	if len(rt.BucketList) != 160 {
		t.Fatalf("expected 160 KBuckets, got %d", len(rt.BucketList))
	}

	// Check that each bucket's upper limit has exactly one bit set
	for i, kb := range rt.BucketList {
		countSetBits := 0
		for _, b := range kb.UpperLimit {
			for j := 0; j < 8; j++ {
				if (b>>j)&1 == 1 {
					countSetBits++
				}
			}
		}
		if countSetBits != 1 {
			t.Errorf("bucket %d: expected exactly one bit set in upper limit, got %d", i, countSetBits)
		}
	}

	// Check that lower limits are all zeros
	for i, kb := range rt.BucketList {
		for j, b := range kb.LowerLimit {
			if b != 0 {
				t.Errorf("bucket %d, byte %d: expected lower limit 0, got %x", i, j, b)
			}
		}
	}
}

func TestAddNodeToCorrectBucket(t *testing.T) {
	nodeA, err := NewNode("localhost")
	if err != nil {
		t.Fatalf("failed to create nodeA: %v", err)
	}
	nodeB, err := NewNode("remotehost")
	if err != nil {
		t.Fatalf("failed to create nodeB: %v", err)
	}

	nodeA.NodeID = [20]byte{0b00000001}
	nodeB.NodeID = [20]byte{0b10000000}

	rt, _ := NewRoutingTable(nodeA.NodeID)
	nodeA.RoutingTable = rt

	if err := nodeA.RoutingTable.AddNode(nodeB); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	msb, err := nodeA.RoutingTable.CalcMostSigBit(nodeB)
	if err != nil {
		t.Fatalf("CalcMostSigBit failed: %v", err)
	}

	// check that we've correctly added nodeB to nodeA
	kb := nodeA.RoutingTable.BucketList[msb]
	found := false
	for _, c := range kb.Contacts {
		if bytes.Equal(c.ID[:], nodeB.NodeID[:]) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("nodeB not found in expected KBucket %d (MSB %d)", msb, msb)
	}
}
