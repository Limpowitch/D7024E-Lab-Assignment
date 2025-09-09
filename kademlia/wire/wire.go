package wire

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
