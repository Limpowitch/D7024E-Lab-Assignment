package node

import (
	"testing"
)

// helpers for readable 160-bit bounds (big-endian)
func idWithFirstByte(b byte) [20]byte {
	var x [20]byte
	x[0] = b
	return x
}
func upperWithFirstByte(b byte) [20]byte {
	var x [20]byte
	x[0] = b
	for i := 1; i < 20; i++ {
		x[i] = 0xFF
	}
	return x
}

func TestNewRoutingTable_SingleFullRange(t *testing.T) {
	var self [20]byte

	var lower [20]byte // 00..00
	var upper [20]byte // FF..FF
	for i := range upper {
		upper[i] = 0xFF
	}

	rt, err := NewRoutingTable(self, lower, upper)
	if err != nil {
		t.Fatalf("expected no error at NewRoutingTable, got %v", err)
	}

	// should start with exactly one full-range bucket
	if len(rt.BucketList) != 1 {
		t.Fatalf("expected 1 KBucket, got %d", len(rt.BucketList))
	}
	got := rt.BucketList[0]
	if got.LowerLimit != lower {
		t.Errorf("initial bucket lower limit mismatch\nwant %v\n got %v", lower, got.LowerLimit)
	}
	if got.UpperLimit != upper {
		t.Errorf("initial bucket upper limit mismatch\nwant %v\n got %v", upper, got.UpperLimit)
	}
}

func TestRoutingTable_AddBucket_SortedInsertAndDuplicate(t *testing.T) {
	var self [20]byte

	// Start with full range
	var lower [20]byte
	var upper [20]byte
	for i := range upper {
		upper[i] = 0xFF
	}

	rt, err := NewRoutingTable(self, lower, upper)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	// Remove the full-range bucket so we can test non-overlapping inserts cleanly
	if err := rt.RemoveBucket(rt.BucketList[0]); err != nil {
		t.Fatalf("RemoveBucket (initial) failed: %v", err)
	}
	if len(rt.BucketList) != 0 {
		t.Fatalf("expected 0 buckets after removal, got %d", len(rt.BucketList))
	}

	// Create three disjoint buckets with out-of-order lowers to test sorted insertion
	kbA, _ := NewKBucket(20, idWithFirstByte(0x80), upperWithFirstByte(0x80), nil) // [0x80.., 0x80..FF]
	kbB, _ := NewKBucket(20, idWithFirstByte(0x00), upperWithFirstByte(0x3F), nil) // [0x00.., 0x3F..FF]
	kbC, _ := NewKBucket(20, idWithFirstByte(0x40), upperWithFirstByte(0x7F), nil) // [0x40.., 0x7F..FF]

	// Insert unsorted; list should be kept sorted by LowerLimit
	if err := rt.AddBucket(kbA); err != nil {
		t.Fatalf("AddBucket(kbA) failed: %v", err)
	}
	if err := rt.AddBucket(kbB); err != nil {
		t.Fatalf("AddBucket(kbB) failed: %v", err)
	}
	if err := rt.AddBucket(kbC); err != nil {
		t.Fatalf("AddBucket(kbC) failed: %v", err)
	}

	if len(rt.BucketList) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(rt.BucketList))
	}

	// Expect order by LowerLimit: kbB (0x00), kbC (0x40), kbA (0x80)
	if rt.BucketList[0].LowerLimit != kbB.LowerLimit {
		t.Errorf("bucket[0] lower = %v, want %v", rt.BucketList[0].LowerLimit, kbB.LowerLimit)
	}
	if rt.BucketList[1].LowerLimit != kbC.LowerLimit {
		t.Errorf("bucket[1] lower = %v, want %v", rt.BucketList[1].LowerLimit, kbC.LowerLimit)
	}
	if rt.BucketList[2].LowerLimit != kbA.LowerLimit {
		t.Errorf("bucket[2] lower = %v, want %v", rt.BucketList[2].LowerLimit, kbA.LowerLimit)
	}

	// Duplicate insert should be rejected
	if err := rt.AddBucket(kbB); err == nil {
		t.Errorf("expected duplicate AddBucket(kbB) to error, got nil")
	}
}

func TestRoutingTable_RemoveBucket(t *testing.T) {
	var self [20]byte

	// Initial full range
	var lower [20]byte
	var upper [20]byte
	for i := range upper {
		upper[i] = 0xFF
	}

	rt, err := NewRoutingTable(self, lower, upper)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	initial := rt.BucketList[0]

	// Remove the initial bucket
	if err := rt.RemoveBucket(initial); err != nil {
		t.Fatalf("RemoveBucket(initial) failed: %v", err)
	}
	if len(rt.BucketList) != 0 {
		t.Fatalf("expected 0 buckets after removing initial, got %d", len(rt.BucketList))
	}

	// Removing again should error
	if err := rt.RemoveBucket(initial); err == nil {
		t.Errorf("expected error when removing non-existent bucket, got nil")
	}
}

func TestRoutingTable_SplitBucket(t *testing.T) {
	var self [20]byte

	// Full range [0x00..00, 0xFF..FF]
	var lower [20]byte
	var upper [20]byte
	for i := range upper {
		upper[i] = 0xFF
	}

	rt, err := NewRoutingTable(self, lower, upper)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	origin := rt.BucketList[0]
	mid := midpoint(lower, upper)
	midPlus := addOne(mid)

	if err := rt.SplitBucket(origin); err != nil {
		t.Fatalf("SplitBucket error: %v", err)
	}

	// Expect two buckets, sorted, covering exactly [lower..mid] and [mid+1..upper]
	if got := len(rt.BucketList); got != 2 {
		t.Fatalf("expected 2 buckets after split, got %d", got)
	}

	b0 := rt.BucketList[0]
	b1 := rt.BucketList[1]

	// Sorted by LowerLimit
	if compare(b0.LowerLimit, b1.LowerLimit) > 0 {
		t.Fatalf("buckets not sorted by LowerLimit: %v then %v", b0.LowerLimit, b1.LowerLimit)
	}

	// First bucket is [lower, mid]
	if b0.LowerLimit != lower {
		t.Errorf("bucket0 lower want %v, got %v", lower, b0.LowerLimit)
	}
	if b0.UpperLimit != mid {
		t.Errorf("bucket0 upper want %v, got %v", mid, b0.UpperLimit)
	}

	// Second bucket is [mid+1, upper]
	if b1.LowerLimit != midPlus {
		t.Errorf("bucket1 lower want %v, got %v", midPlus, b1.LowerLimit)
	}
	if b1.UpperLimit != upper {
		t.Errorf("bucket1 upper want %v, got %v", upper, b1.UpperLimit)
	}

	// No gaps / overlaps
	if addOne(b0.UpperLimit) != b1.LowerLimit {
		t.Errorf("expected contiguous buckets: addOne(b0.Upper) == b1.Lower; got %v vs %v", addOne(b0.UpperLimit), b1.LowerLimit)
	}
}
