package main

import (
	"testing"
)

func TestNewRoutingTable(t *testing.T) {

	rt, err := NewRoutingTable()
	if err != nil {
		t.Errorf("Expected no error at NewRoutingTable, got %v", err)
	}

	capacity := 3
	var lower [20]byte // defaults to all zeros
	var upper [20]byte
	for i := range upper {
		upper[i] = 0xFF // max value for upper limit
	}

	node, err := NewNode("localhost")
	if err != nil {
		t.Fatalf("expected no error at NewNode creation, got %v", err)
	}

	contact1, err := NewContact(*node)
	if err != nil {
		t.Fatalf("expected no error at NewContact creation, got %v", err)
	}

	contacts := []Contact{contact1}

	kb, err := NewKBucket(capacity, lower, upper, contacts)
	if err != nil {
		t.Fatalf("expected no error at NewKBucket creation, got %v", err)
	}

	rt.AddToRT(kb)

	if len(rt.BucketList) <= 0 {
		t.Errorf("expected bucketList size of 1, got %v", rt.BucketList)
	}
}
