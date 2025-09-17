package service

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestFindNode_RoundTrip(t *testing.T) {
	var idA, idB [20]byte // local type alias in service is also [20]byte

	// A (client)
	a, err := New("127.0.0.1:0", idA, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Start()

	// B (server) — stub handler returns a fixed payload
	b, err := New("127.0.0.1:0", idB, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Whatever bytes you want—service layer treats it as opaque payload.
	want := []byte{0x00, 0x02, 'x', ':', '1', 'y', ':', '2'}

	b.OnFindNode = func(target [20]byte) []byte {
		// We could assert target is 20 bytes here if we wanted.
		return want
	}

	b.Start()

	var target [20]byte // zero target is fine; service just forwards it
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := a.FindNode(ctx, b.Addr(), target)
	if err != nil {
		t.Fatalf("FindNode failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected payload.\n got=%v\nwant=%v", got, want)
	}
}
