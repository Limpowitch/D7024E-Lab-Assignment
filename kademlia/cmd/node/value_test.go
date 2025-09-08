package main

import (
	"bytes"
	"testing"
	"time"
)

func TestNewValue(t *testing.T) {
	ttl := 20 * time.Second
	data := []byte{71, 111}

	n, err := NewValue(data, ttl)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !bytes.Equal(n.Data, data) {
		t.Errorf("Expected data value was %v, got %v", data, n.Data)
	}

	expectedExpiry := time.Now().Add(ttl)
	if n.ExpiresAt.Before(expectedExpiry.Add(-time.Second)) || n.ExpiresAt.After(expectedExpiry.Add(time.Second)) {
		t.Errorf("expected ExpiresAt around %v, got %v", expectedExpiry, n.ExpiresAt)
	}

}
