package node

import (
	"time"
)

type Contact struct {
	ID       NodeID
	Addr     string    // "host:port"
	LastSeen time.Time // updated on successful RPC if we want to do as we talked about in sprint 0 review//samme
}

func NewContact(id NodeID, addr string) Contact {
	return Contact{ID: id, Addr: addr}
}

// sets LastSeen to now.
func (c *Contact) Touch() { c.LastSeen = time.Now() }
