package service

import (
	"context"
	"testing"
	"time"
)

func TestFindValue_ValueAndContacts(t *testing.T) {
	var idA, idB [20]byte
	a, _ := New("127.0.0.1:0", idA, "")
	defer a.Close()
	a.Start()
	b, _ := New("127.0.0.1:0", idB, "")
	defer b.Close()
	b.Start()

	key := [20]byte{9, 9}

	// value case
	b.OnFindValue = func(k [20]byte) ([]byte, []byte) {
		if k == key {
			return []byte("V"), nil
		}
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := a.FindValue(ctx, b.Addr(), key)
	if err != nil || string(res.Value) != "V" {
		t.Fatalf("value path: %v %q", err, res.Value)
	}

	// contacts case
	b.OnFindValue = func(k [20]byte) ([]byte, []byte) {
		return nil, []byte{0x00, 0x00} // your encoding: an empty contact list
	}
	res2, err := a.FindValue(ctx, b.Addr(), key)
	if err != nil || res2.Value != nil || len(res2.Contacts) == 0 {
		t.Fatalf("contacts path: %+v %v", res2, err)
	}
}
