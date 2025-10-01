package service

import (
	"context"
	"testing"
	"time"
)

func TestAdminGet_PayloadRoundTrip(t *testing.T) {
	sSrv, _ := New("127.0.0.1:0", [20]byte{9}, "")
	sCli, _ := New("127.0.0.1:0", [20]byte{8}, "")
	sSrv.Start()
	defer sSrv.Close()
	sCli.Start()
	defer sCli.Close()

	// Capture key server receives.
	var gotKey [20]byte
	done := make(chan struct{})
	sSrv.SetOnAdminGet(func(ctx context.Context, key [20]byte) ([]byte, bool) {
		gotKey = key
		close(done)
		return nil, false
	})

	// Known key.
	var want [20]byte
	copy(want[:], []byte{0x67, 0xaf, 0x47, 0x74, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, _ = sCli.AdminGet(ctx, sSrv.Addr(), want)

	<-done

	if gotKey != want {
		t.Fatalf("payload mangled: got %x want %x", gotKey, want)
	}
}
