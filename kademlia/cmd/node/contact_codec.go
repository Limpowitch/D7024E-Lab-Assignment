package node

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
)

// Encodes a contact to a byte slice and decodes a byte slice to a contact
func (c Contact) MarshalBinary() []byte {
	addr := []byte(c.Addr)
	if len(addr) > 255 {
		addr = addr[:255]
	} // 1-byte length
	out := make([]byte, 0, IDBytes+1+len(addr))
	out = append(out, c.ID[:]...)      // 20B
	out = append(out, byte(len(addr))) // 1B
	out = append(out, addr...)         // len(addr)
	return out
}

// Decodes a byte slice to a contact
func (c *Contact) UnmarshalBinary(b []byte) error {
	if len(b) < IDBytes+1 {
		return errors.New("short contact")
	}
	copy(c.ID[:], b[:IDBytes])
	l := int(b[IDBytes])
	if len(b) < IDBytes+1+l {
		return errors.New("short addr")
	}
	c.Addr = string(b[IDBytes+1 : IDBytes+1+l])
	return nil
}

// Encodes a list of contacts
func MarshalContactList(list []Contact) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out[:2], uint16(len(list)))
	for _, c := range list {
		out = append(out, c.MarshalBinary()...)
	}
	return out
}

// Decodes a byte slice to a list of contacts
func UnmarshalContactList(b []byte) ([]Contact, error) {
	if len(b) < 2 {
		return nil, errors.New("short list")
	}
	n := int(binary.BigEndian.Uint16(b[:2]))
	b = b[2:]
	res := make([]Contact, 0, n)
	for i := 0; i < n; i++ {
		if len(b) < IDBytes+1 {
			return nil, errors.New("short entry")
		}
		l := int(b[IDBytes])
		need := IDBytes + 1 + l
		if len(b) < need {
			return nil, errors.New("short entry addr")
		}
		var c Contact
		if err := c.UnmarshalBinary(b[:need]); err != nil {
			return nil, err
		}
		res = append(res, c)
		b = b[need:]
	}
	return res, nil
}

// Returns the SHA-1 hash of the input bytes
func SHA1ID(b []byte) [20]byte {
	h := sha1.Sum(b)
	return h
}
