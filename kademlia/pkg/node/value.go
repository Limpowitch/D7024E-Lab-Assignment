package node

import (
	"time"
)

type Value struct {
	Data      []byte
	ExpiresAt time.Time
}

func NewValue(data []byte, ttl time.Duration) Value {
	return Value{
		Data:      append([]byte(nil), data...), // copy the data and append in new []byte
		ExpiresAt: time.Now().Add(ttl),          // ttl for value object
	}
}

func (v Value) Expired(now time.Time) bool { // Future implimentation...

	return false // change this
}
