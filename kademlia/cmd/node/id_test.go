package node

import (
	"encoding/hex"
	"math/big"
	"testing"
)

// helper to build a NodeID from hex in tests (kept local to tests)
func idFromHex(t *testing.T, s string) NodeID {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	if len(b) != IDBytes {
		t.Fatalf("want %d bytes, got %d", IDBytes, len(b))
	}
	var id NodeID
	copy(id[:], b)
	return id
}

func TestIsZero(t *testing.T) {
	var z NodeID
	if !z.IsZero() {
		t.Fatal("zero id should report IsZero=true")
	}
	var nz NodeID
	nz[0] = 1
	if nz.IsZero() {
		t.Fatal("non-zero id should report IsZero=false")
	}
}

func TestDistance_XOR(t *testing.T) {
	// a = 0000..00
	a := idFromHex(t, "0000000000000000000000000000000000000000")
	// b = ffff..ff
	b := idFromHex(t, "ffffffffffffffffffffffffffffffffffffffff")

	want := new(big.Int).SetBytes(make([]byte, IDBytes)) // start at 0
	// XOR(0, f...f) == f...f
	want.SetBytes(make([]byte, IDBytes))
	want.SetBytes([]byte{
		0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff,
	})

	got := Distance(a, b)
	if got.Cmp(want) != 0 {
		t.Fatalf("distance mismatch:\n got=%x\nwant=%x", got.Bytes(), want.Bytes())
	}

	// Symmetry: d(a,b) == d(b,a)
	if Distance(b, a).Cmp(got) != 0 {
		t.Fatal("distance should be symmetric")
	}
}

func TestCloser(t *testing.T) {
	target := idFromHex(t, "0000000000000000000000000000000000000000")
	// x is very close (differs by 1 in LSB)
	x := idFromHex(t, "0000000000000000000000000000000000000001")
	// y is far (all ones)
	y := idFromHex(t, "ffffffffffffffffffffffffffffffffffffffff")

	if !Closer(target, x, y) {
		t.Fatal("expected x to be closer to target than y")
	}
	if Closer(target, y, x) {
		t.Fatal("expected y NOT to be closer than x")
	}
}

func TestRandomNodeID_NotZero_And_NotAllEqual(t *testing.T) {
	const n = 5
	seen := make(map[[IDBytes]byte]struct{})
	for i := 0; i < n; i++ {
		id := RandomNodeID()
		if id.IsZero() {
			t.Fatal("RandomNodeID returned zero id (extremely unlikely)")
		}
		seen[id] = struct{}{}
	}
	if len(seen) == 1 {
		t.Fatal("RandomNodeID returned identical ids repeatedly (extremely unlikely)")
	}
}
