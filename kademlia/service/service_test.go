package service

import (
	"context"
	"testing"
	"time"
)

func TestPingPong(t *testing.T) {
	var idA, idB [20]byte

	a, err := New("127.0.0.1:0", idA, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Start()

	b, err := New("127.0.0.1:0", idB, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	b.Start()

	// optional: log addresses to ensure they're dialable
	t.Logf("A=%s  B=%s", a.Addr(), b.Addr())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := a.Ping(ctx, b.Addr()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

}
