package service

import (
	"context"
	"testing"
	"time"
)

func TestStore_RoundTrip(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	var gotKey [20]byte
	var gotVal []byte
	done := make(chan struct{}, 1)
	b.SetOnStore(func(k [20]byte, v []byte) { gotKey = k; gotVal = append([]byte(nil), v...); done <- struct{}{} })

	key := [20]byte{1, 2, 3}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Store(ctx, b.Addr(), key, []byte("hello")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	<-done
	if gotKey != key || string(gotVal) != "hello" {
		t.Fatalf("bad store: %x %q", gotKey, gotVal)
	}
}
