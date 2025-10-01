package service

import (
	"bytes"
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

func TestAdminRT_RoundTrip(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	want := []byte{0x01, 0x02, 0x03}
	b.SetOnDumpRT(func() []byte { return want })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := a.AdminRT(ctx, b.Addr())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAdminPut_AdminGet_Value(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	// Server: OnAdminPut returns a fixed key, OnAdminGet returns the value for that key.
	var key [20]byte
	copy(key[:], []byte("abcdefghijklmnopqrst")[:20])
	b.SetOnAdminPut(func(v []byte) ([20]byte, error) {
		if string(v) != "hello" {
			t.Fatalf("unexpected put value %q", v)
		}
		return key, nil
	})
	b.SetOnAdminGet(func(ctx context.Context, k [20]byte) ([]byte, bool) {
		if k == key {
			return []byte("hello"), true
		}
		return nil, false
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	gotKey, err := a.AdminPut(ctx, b.Addr(), []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != key {
		t.Fatalf("key mismatch")
	}

	val, ok, err := a.AdminGet(ctx, b.Addr(), key)
	if err != nil || !ok {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if string(val) != "hello" {
		t.Fatalf("got %q", val)
	}
}

func TestAdminGet_NotFound(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	b.SetOnAdminGet(func(ctx context.Context, k [20]byte) ([]byte, bool) { return nil, false })

	var key [20]byte
	key[0] = 0x42
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	val, ok, err := a.AdminGet(ctx, b.Addr(), key)
	if err != nil {
		t.Fatal(err)
	}
	if ok || val != nil {
		t.Fatalf("expected not found, got ok=%v val=%v", ok, val)
	}
}

func TestRefresh_RoundTrip(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	key := [20]byte{7, 7, 7}
	called := make(chan struct{}, 1)
	b.SetOnRefresh(func(k [20]byte) {
		if k != key {
			t.Fatalf("key mismatch")
		}
		called <- struct{}{}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Refresh(ctx, b.Addr(), key); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("refresh handler not called")
	}
}

func TestAdminForget_RoundTrip(t *testing.T) {
	t.Parallel()

	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	t.Cleanup(func() { _ = a.Close() })
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	t.Cleanup(func() { _ = b.Close() })
	b.Start()

	key := [20]byte{0xAA}

	called := make(chan struct{}, 1) // signal channel
	b.SetOnAdminForget(func(k [20]byte) bool {
		if k != key {
			t.Errorf("key mismatch")
		}
		called <- struct{}{} // signal safely
		return true
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := a.AdminForget(ctx, b.Addr(), key); err != nil {
		t.Fatal(err)
	}

	select {
	case <-called:
		// ok
	case <-time.After(time.Second):
		t.Fatal("forget handler not called")
	}
}

func TestAdminExit_RoundTrip(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	called := make(chan struct{}, 1)
	b.SetOnExit(func() { called <- struct{}{} })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.AdminExit(ctx, b.Addr()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("exit handler not called")
	}
}

func TestSendAndWait_Timeout(t *testing.T) {
	var idA [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := a.FindNode(ctx, "127.0.0.1:65534", [20]byte{}) // nobody listens
	if err == nil {
		t.Fatal("expected timeout")
	}
}
