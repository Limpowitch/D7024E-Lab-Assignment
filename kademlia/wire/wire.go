package wire

import (
	"bytes"
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

const (
	TypeStore          = "STORE"
	TypeStoreAck       = "STORE_ACK"
	TypeFindNode       = "FIND_NODE"
	TypeFindValueReply = "FIND_VALUE_REPLY"
)

type Contact struct {
	ID      [20]byte
	Address string
}

//Store

func PackStore(fromID, key [20]byte, value []byte) []byte {
	out := make([]byte, 0, 20+20+len(value))
	out = append(out, fromID[:]...)
	out = append(out, key[:]...)
	out = append(out, value...)
	return out
}

func UnpackStore(pl []byte) (fromID, key [20]byte, value []byte, err error) {
	if len(pl) < 40 {
		return fromID, key, nil, errors.New("short store")
	}
	copy(fromID[:], pl[:20])
	copy(key[:], pl[20:40])
	value = pl[40:]
	return
}

// Ack
func PackStoreAck(fromID, key [20]byte) []byte {
	out := make([]byte, 0, 20)
	copy(out, fromID[:])
	return out
}

func UnpackStoreAck(pl []byte) (fromID, key [20]byte, err error) {
	if len(pl) < 20 {
		return fromID, key, errors.New("short store ack")
	}
	copy(fromID[:], pl[:20])
	return
}

// FindNode
func PackFindNode(fromID, key [20]byte) []byte {
	out := make([]byte, 0, 20+20)
	copy(out[:20], fromID[:])
	copy(out[20:40], key[:])
	return out
}

func UnpackFindNode(pl []byte) (fromID, key [20]byte, err error) {
	if len(pl) < 40 {
		return fromID, key, errors.New("short find node")
	}
	copy(fromID[:], pl[:20])
	copy(key[:], pl[20:40])
	return
}

// FindValueReply
func PackFindValueReplyValue(fromID [20]byte, value []byte) []byte {
	out := make([]byte, 0, 21+len(value))
	out = append(out, fromID[:]...)
	out = append(out, byte(1)) // 1 indicates value present
	out = append(out, value...)
	return out
}

func PackFindValueReplyContacts(fromID [20]byte, contacts []Contact) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	buf.Write(fromID[:])
	buf.WriteByte(0)
	if len(contacts) > 255 {
		contacts = contacts[:255]
	}
	buf.WriteByte(byte(len(contacts)))
	for _, c := range contacts {
		buf.Write(c.ID[:])
		addr := c.Address
		if len(addr) > 255 {
			addr = addr[:255]
		}
		buf.WriteByte(byte(len(addr)))
		buf.Write([]byte(addr))
	}
	return buf.Bytes()
}

func UnpackFindValueReply(p []byte) (fromID [20]byte, hasValue bool, value []byte, contacts []Contact, err error) {
	if len(p) < 21 {
		err = errors.New("short FIND_VALUE_REPLY")
		return
	}
	copy(fromID[:], p[:20])
	flag := p[20]
	if flag == 1 {
		hasValue = true
		value = p[21:]
		return
	}
	if len(p) < 22 {
		err = errors.New("short contacts header")
		return
	}
	n := int(p[21])
	off := 22
	contacts = make([]Contact, 0, n)
	for i := 0; i < n; i++ {
		if len(p) < off+20+1 {
			err = errors.New("short contact entry")
			return
		}
		var id [20]byte
		copy(id[:], p[off:off+20])
		off += 20
		alen := int(p[off])
		off++
		if len(p) < off+alen {
			err = errors.New("short contact addr")
			return
		}
		addr := string(p[off : off+alen])
		off += alen
		contacts = append(contacts, Contact{ID: id, Address: addr})
	}
	return
}
