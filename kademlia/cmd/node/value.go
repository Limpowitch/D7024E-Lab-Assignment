package node

import (
	"time"
)

type Value struct {
	Data      []byte
	ExpiresAt time.Time
}

func NewValue(data []byte, ttl time.Duration) (Value, error) {
	return Value{
		Data:      append([]byte(nil), data...),
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (v Value) Expired(now time.Time) bool { // Future implimentation...

	return !now.Before(v.ExpiresAt) // this should work for the tests. change if needed//samme
}
