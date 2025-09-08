package node

import (
	"testing"
)

func TestNewNode(t *testing.T) {
	hostname := "localhost"

	node, err := NewNode(hostname)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if node.Hostname != hostname {
		t.Errorf("expected hostname %s, got %s", hostname, node.Hostname)
	}

	var zeroID [20]byte
	if node.NodeID == zeroID {
		t.Errorf("expected random NodeID, got all zeros")
	}

	if node.NodeStorage == nil {
		t.Errorf("expected NodeStorage to be initialized, got nil")
	}

	if node.RoutingTable.BucketList == nil {
		t.Errorf("expected RoutingTable.BucketList to be initialized, got nil")
	}
}
