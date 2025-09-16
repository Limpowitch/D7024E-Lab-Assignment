package main

import (
	"sync"
	"time"
)

type Value struct {
	Data      []byte
	ExpiresAt time.Time
}

type Object struct {
	Value []byte
}

type ObjectStore struct {
	mu   sync.RWMutex
	data map[[20]byte]Object
}

func NewObjectStore() *ObjectStore {
	return &ObjectStore{data: make(map[[20]byte]Object)}
}

func (s *ObjectStore) Put(key [20]byte, val []byte) {
	s.mu.Lock()
	cp := make([]byte, len(val))
	copy(cp, val)
	s.data[key] = Object{Value: cp}
	s.mu.Unlock()
}

func (s *ObjectStore) Get(key [20]byte) ([]byte, bool) {

	s.mu.RLock()
	obj, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(obj.Value))
	copy(cp, obj.Value)
	return cp, true
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
