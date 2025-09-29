package node

import (
	"time"
)

type Value struct {
	Data        []byte
	Origin      bool
	LastPublish time.Time
	ExpiresAt   time.Time
}

func NewValue(data []byte, ttl time.Duration) Value {
	return Value{
		Data:      append([]byte(nil), data...),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (v Value) Expired(now time.Time) bool { // Future implimentation...

	return !now.Before(v.ExpiresAt) // this should work for the tests. change if needed//samme
}
