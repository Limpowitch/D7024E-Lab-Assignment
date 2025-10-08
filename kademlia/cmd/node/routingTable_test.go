package node

import (
	"testing"
)

/******** helpers ********/

func zeroID() [20]byte { return [20]byte{} }

func maxID() [20]byte {
	var x [20]byte
	for i := range x {
		x[i] = 0xFF
	}
	return x
}

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

func contactWithFirstByte(b byte, addr string) Contact {
	return Contact{ID: idWithFirstByte(b), Addr: addr}
}

/******** tests ********/

func TestNewRoutingTable_SingleFullRange(t *testing.T) {
	var self [20]byte
	lower := zeroID()
	upper := maxID()
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, lower, upper, K, b)
	if err != nil {
		t.Fatalf("expected no error at NewRoutingTable, got %v", err)
	}

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
	lower := zeroID()
	upper := maxID()
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, lower, upper, K, b)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	// Remove initial full-range for a clean slate.
	if err := rt.removeBucketLocked(rt.BucketList[0]); err != nil {
		t.Fatalf("remove initial failed: %v", err)
	}

	kbA, _ := NewKBucket(20, idWithFirstByte(0x80), upperWithFirstByte(0x80), nil, b)
	kbB, _ := NewKBucket(20, idWithFirstByte(0x00), upperWithFirstByte(0x3F), nil, b)
	kbC, _ := NewKBucket(20, idWithFirstByte(0x40), upperWithFirstByte(0x7F), nil, b)

	if err := rt.addBucketLocked(&kbA); err != nil {
		t.Fatalf("AddBucket A failed: %v", err)
	}
	if err := rt.addBucketLocked(&kbB); err != nil {
		t.Fatalf("AddBucket B failed: %v", err)
	}
	if err := rt.addBucketLocked(&kbC); err != nil {
		t.Fatalf("AddBucket C failed: %v", err)
	}

	if len(rt.BucketList) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(rt.BucketList))
	}

	if rt.BucketList[0].LowerLimit != kbB.LowerLimit {
		t.Errorf("bucket[0] lower = %v, want %v", rt.BucketList[0].LowerLimit, kbB.LowerLimit)
	}
	if rt.BucketList[1].LowerLimit != kbC.LowerLimit {
		t.Errorf("bucket[1] lower = %v, want %v", rt.BucketList[1].LowerLimit, kbC.LowerLimit)
	}
	if rt.BucketList[2].LowerLimit != kbA.LowerLimit {
		t.Errorf("bucket[2] lower = %v, want %v", rt.BucketList[2].LowerLimit, kbA.LowerLimit)
	}

	// Duplicate insert should error
	if err := rt.addBucketLocked(&kbB); err == nil {
		t.Errorf("expected duplicate AddBucket(kbB) to error, got nil")
	}
}

func TestRoutingTable_RemoveBucket(t *testing.T) {
	var self [20]byte
	lower := zeroID()
	upper := maxID()
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, lower, upper, K, b)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	initial := rt.BucketList[0]
	if err := rt.removeBucketLocked(initial); err != nil {
		t.Fatalf("RemoveBucket(initial) failed: %v", err)
	}
	if len(rt.BucketList) != 0 {
		t.Fatalf("expected 0 buckets after removing initial, got %d", len(rt.BucketList))
	}

	if err := rt.removeBucketLocked(initial); err == nil {
		t.Errorf("expected error when removing non-existent bucket, got nil")
	}
}

func TestUpdate_Splits_WhenBucketContainsSelf(t *testing.T) {
	// Self is 0x80..., bucket is [0x80..00, 0x80..FF] so it contains self.
	self := idWithFirstByte(0x80)
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, zeroID(), maxID(), K, b)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	// Replace initial with a small, self-containing bucket (capacity 2).
	if err := rt.removeBucketLocked(rt.BucketList[0]); err != nil {
		t.Fatalf("remove initial failed: %v", err)
	}
	lower := idWithFirstByte(0x80)
	upper := upperWithFirstByte(0x84)
	kb, _ := NewKBucket(2, lower, upper, nil, b)
	if err := rt.addBucketLocked(&kb); err != nil {
		t.Fatalf("add self bucket failed: %v", err)
	}

	// Fill to capacity
	rt.Update(contactWithFirstByte(0x80, "h1"))
	rt.Update(contactWithFirstByte(0x81, "h2"))

	// Inserting a third should trigger split (contains self).
	rt.Update(contactWithFirstByte(0x83, "h3"))

	// Expect split into two contiguous buckets
	if len(rt.BucketList) != 2 {
		t.Fatalf("expected 2 buckets after split, got %d", len(rt.BucketList))
	}
	mid := midpoint(lower, upper)
	left := rt.BucketList[0]
	right := rt.BucketList[1]

	if left.LowerLimit != lower || left.UpperLimit != mid {
		t.Errorf("left bounds want [%v..%v], got [%v..%v]", lower, mid, left.LowerLimit, left.UpperLimit)
	}
	if right.LowerLimit != addOne(mid) || right.UpperLimit != upper {
		t.Errorf("right bounds want [%v..%v], got [%v..%v]", addOne(mid), upper, right.LowerLimit, right.UpperLimit)
	}
}

func TestUpdate_Splits_OnNonSelf_WhenDepthModBNotZero(t *testing.T) {
	// Self far away (0xA0...), bucket is [0x60..00, 0x7F..FF] → prefix 011x → depth=3 (non-self), b=2 → 3%2!=0 → can split.
	self := idWithFirstByte(0xA0)
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, zeroID(), maxID(), K, b)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	if err := rt.removeBucketLocked(rt.BucketList[0]); err != nil {
		t.Fatalf("remove initial failed: %v", err)
	}
	lower := idWithFirstByte(0x60)    // 0110 0000
	upper := upperWithFirstByte(0x7F) // 0111 1111
	kb, _ := NewKBucket(2, lower, upper, nil, b)
	if err := rt.addBucketLocked(&kb); err != nil {
		t.Fatalf("add non-self odd-depth bucket failed: %v", err)
	}

	// Fill to capacity
	rt.Update(contactWithFirstByte(0x60, "h1"))
	rt.Update(contactWithFirstByte(0x61, "h2"))

	// Insert third → should split (d=3, b=2)
	rt.Update(contactWithFirstByte(0x7F, "h3"))

	if len(rt.BucketList) != 2 {
		t.Fatalf("expected 2 buckets after split, got %d", len(rt.BucketList))
	}

	mid := midpoint(lower, upper)
	left := rt.BucketList[0]
	right := rt.BucketList[1]

	if left.LowerLimit != lower || left.UpperLimit != mid {
		t.Errorf("left bounds want [%v..%v], got [%v..%v]", lower, mid, left.LowerLimit, left.UpperLimit)
	}
	if right.LowerLimit != addOne(mid) || right.UpperLimit != upper {
		t.Errorf("right bounds want [%v..%v], got [%v..%v]", addOne(mid), upper, right.LowerLimit, right.UpperLimit)
	}
}

func TestUpdate_NoSplit_OnNonSelf_WhenDepthModBZero(t *testing.T) {
	// Self far away (0xA0...), bucket is [0x00..00, 0x3F..FF] → prefix 00xx → depth=2 (non-self), b=2 → 2%2==0 → do NOT split.
	self := idWithFirstByte(0xA0)
	b := 2
	K := 2

	rt, err := NewRoutingTable(self, zeroID(), maxID(), K, b)
	if err != nil {
		t.Fatalf("NewRoutingTable error: %v", err)
	}

	if err := rt.removeBucketLocked(rt.BucketList[0]); err != nil {
		t.Fatalf("remove initial failed: %v", err)
	}
	lower := idWithFirstByte(0x00)    // 0000 0000
	upper := upperWithFirstByte(0x3F) // 0011 1111
	kb, _ := NewKBucket(2, lower, upper, nil, b)
	if err := rt.addBucketLocked(&kb); err != nil {
		t.Fatalf("add non-self even-depth bucket failed: %v", err)
	}

	// Fill to capacity
	rt.Update(contactWithFirstByte(0x00, "h1"))
	rt.Update(contactWithFirstByte(0x3F, "h2"))

	// Insert third → should NOT split; bucket remains at capacity (replacement cache or drop).
	rt.Update(contactWithFirstByte(0x01, "h3"))

	if len(rt.BucketList) != 1 {
		t.Fatalf("expected no split (1 bucket), got %d", len(rt.BucketList))
	}
	if got := len(rt.BucketList[0].Contacts); got != 2 {
		t.Fatalf("expected bucket capacity 2 unchanged, got %d", got)
	}

	// Ensure new contact was NOT inserted (since policy said no split).
	ids := rt.BucketList[0].Contacts
	for _, c := range ids {
		if c.ID == idWithFirstByte(0x01) {
			t.Fatalf("unexpected insertion of 0x01 in non-self bucket at depth multiple of b")
		}
	}
}
