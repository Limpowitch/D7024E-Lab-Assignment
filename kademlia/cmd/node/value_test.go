package node

import (
	"bytes"
	"testing"
	"time"
)

func TestNewValue_SetsFieldsAndCopiesData(t *testing.T) {
	data := []byte{0x71, 0x6f, 0x21} // "qo!"
	ttl := 500 * time.Millisecond

	callStart := time.Now()
	v := NewValue(data, ttl)
	callEnd := time.Now()

	// Deep copy of data
	if !bytes.Equal(v.Data, data) {
		t.Fatalf("data mismatch: got %v, want %v", v.Data, data)
	}
	// Mutate input; stored value must not change
	data[0] = 0x00
	if v.Data[0] != 0x71 {
		t.Fatalf("value.Data was not copied; got %v", v.Data)
	}

	// Defaults
	if v.Origin {
		t.Fatalf("Origin should default to false")
	}
	if !v.LastPublish.IsZero() {
		t.Fatalf("LastPublish should be zero by default")
	}

	// ExpiresAt should be within [callStart+ttl, callEnd+ttl]
	min := callStart.Add(ttl)
	max := callEnd.Add(ttl)
	if v.ExpiresAt.Before(min) || v.ExpiresAt.After(max) {
		t.Fatalf("ExpiresAt not within expected window. got=%v, want=[%v..%v]", v.ExpiresAt, min, max)
	}
}

func TestExpired_Boundaries(t *testing.T) {
	now := time.Now()
	v := Value{ExpiresAt: now.Add(100 * time.Millisecond)}

	if v.Expired(now) {
		t.Fatalf("should not be expired before ExpiresAt")
	}
	if !v.Expired(now.Add(100 * time.Millisecond)) { // exactly at ExpiresAt
		t.Fatalf("should be expired at ExpiresAt")
	}
	if !v.Expired(now.Add(101 * time.Millisecond)) { // after ExpiresAt
		t.Fatalf("should be expired after ExpiresAt")
	}
}

func TestNewValue_ZeroTTL(t *testing.T) {
	v := NewValue([]byte("x"), 0)

	// With zero TTL, ExpiresAt ~= creation time; at ExpiresAt it's expired.
	if !v.Expired(v.ExpiresAt) {
		t.Fatalf("zero-TTL value should be expired at its ExpiresAt")
	}
}
