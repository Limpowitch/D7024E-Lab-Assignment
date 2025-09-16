package wire

import (
	"crypto/rand"
	"errors"
)

// we need messages that travel over udp.
// each message must carry the rpc id (160bit) so that req. and reply can be correlated
// need to know message type
// actual payload bytes (arbitrary size?)

// envelope thoughts:

// ========
// 160-bit id
// label, like ping, find_node, msg, etc.
// actual message
// ========

const SizeOfID = 20

type RPCID [SizeOfID]byte // follow the same principle as in 'node'

func NewRPCID() (id RPCID) { _, _ = rand.Read(id[:]); return } //useful for tests later

type Envelope struct {
	ID      RPCID
	Type    string
	Payload []byte
}

// Marshal: [20B ID][1B typLen][typ][payload]
func (e Envelope) Marshal() []byte {
	typ := []byte(e.Type)
	if len(typ) > 255 {
		typ = typ[:255]
	}
	out := make([]byte, 0, SizeOfID+1+len(typ)+len(e.Payload))
	out = append(out, e.ID[:]...)
	out = append(out, byte(len(typ)))
	out = append(out, typ...)
	out = append(out, e.Payload...)
	return out
}

func Unmarshal(b []byte) (Envelope, error) {
	if len(b) < SizeOfID+1 {
		return Envelope{}, errors.New("short")
	}
	var id RPCID
	copy(id[:], b[:SizeOfID])
	typLen := int(b[SizeOfID])
	if len(b) < SizeOfID+1+typLen {
		return Envelope{}, errors.New("short type")
	}
	typ := string(b[SizeOfID+1 : SizeOfID+1+typLen])
	pl := b[SizeOfID+1+typLen:]
	return Envelope{ID: id, Type: typ, Payload: pl}, nil
}
