package wire

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const SizeOfID = 20

type RPCID [SizeOfID]byte

var (
	seedOnce   sync.Once
	seedPrefix [12]byte
	idCtr      uint64
)

func initSeeds() {
	if _, err := rand.Read(seedPrefix[:]); err != nil {
		panic("crypto/rand failed in wire.NewRPCID seed")
	}
	atomic.StoreUint64(&idCtr, uint64(time.Now().UnixNano()))
}

//   - 12 random bytes (per-process)  ||  8-byte big-endian counter
//
// we do this so that collisinos never happen
func NewRPCID() RPCID {
	seedOnce.Do(initSeeds)

	n := atomic.AddUint64(&idCtr, 1)

	var id RPCID
	// prefix (12B)
	copy(id[:12], seedPrefix[:])
	// suffix counter (8B)
	binary.BigEndian.PutUint64(id[12:], n)
	return id
}

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
