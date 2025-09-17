package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
)

// OBS: Contact ska redan finnas i ditt projekt (från kbucket/routingTable).
// Den används här i RPC. Om din Contact heter något annat, byt namnet nedan.

type MsgType uint8

const (
	MSG_PING MsgType = iota
	MSG_PONG
	MSG_FIND_NODE
	MSG_FIND_NODE_REPLY

	// --- M2 (nya) ---
	MSG_STORE
	MSG_STORE_ACK
	MSG_FIND_VALUE
	MSG_FIND_VALUE_REPLY
)

// RPC är ditt trådmeddelande över UDP.
// Lägg till fält här om du behöver mer senare.
type RPC struct {
	Type     MsgType
	RPCID    [20]byte
	FromID   [20]byte
	FromAddr string

	// Payload (exakt en “gren” används åt gången)
	Key      [20]byte  // STORE, FIND_VALUE
	Value    []byte    // STORE, FIND_VALUE_REPLY (om hittad)
	Contacts []Contact // *_REPLY när värde saknas (eller FIND_NODE_REPLY)
}

// ==== Hjälpfunktioner ====

func NewRPCID() (id [20]byte) { // slumpmässigt RPC-ID
	_, _ = rand.Read(id[:])
	return
}

func SHA1Key(b []byte) (k [20]byte) { // SHA-1 hash av godtycklig data
	sum := sha1.Sum(b)
	copy(k[:], sum[:])
	return
}

func KeyToHex(k [20]byte) string { // hex-sträng av nyckel
	return hex.EncodeToString(k[:])
}

func HexToKey(s string) (k [20]byte, ok bool) { // nyckel från hex-sträng
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 20 {
		return k, false
	}
	copy(k[:], b)
	return k, true
}
