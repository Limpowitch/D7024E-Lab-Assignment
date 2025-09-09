package node

import "errors"

type RoutingTable struct {
	SelfID     [20]byte
	BucketList []Kbucket
}

func NewRoutingTable(SelfId [20]byte) (RoutingTable, error) {
	const Bits = 160
	const KBucketCapacity = 20

	rt := RoutingTable{
		SelfID:     SelfId,
		BucketList: make([]Kbucket, Bits),
	}

	var zeroID [20]byte

	for i := 0; i < Bits; i++ {
		// Compute lower limit as 0
		lower := zeroID

		// Compute upper limit by setting the (i)th bit to 1
		var upper [20]byte
		byteIndex := i / 8
		bitIndex := 7 - (i % 8) // MSB-first
		upper[byteIndex] = 1 << bitIndex

		kb, err := NewKBucket(KBucketCapacity, lower, upper, []Contact{})
		if err != nil {
			return RoutingTable{}, errors.New("failed to create kbucket")
		}

		rt.BucketList[i] = kb
	}

	return rt, nil
}

func (rt *RoutingTable) AddNode(node *Node) error {
	msb, err := rt.CalcMostSigBit(node)
	if err != nil {
		return err
	}

	if msb < 0 {
		return errors.New("cannot add node with identical ID")
	}

	contact, _ := NewContact(*node)

	targetEntry := &rt.BucketList[msb]
	targetEntry.AddToKBucket(contact)

	return nil
}

// func (rt *RoutingTable) AddBucketToRT(value Kbucket) { // might be used???
// 	rt.BucketList = append(rt.BucketList, value)
// }

// func (rt *RoutingTable) DeleteBucketFromRT(value Kbucket) { // Also might be used???
// 	for i, v := range rt.BucketList {
// 		if v.LowerLimit == value.LowerLimit && v.UpperLimit == value.UpperLimit {
// 			rt.BucketList = append(rt.BucketList[:i], rt.BucketList[i+1:]...)
// 			return
// 		}
// 	}
// }

func (rt *RoutingTable) CalcMostSigBit(RemoteNode *Node) (int, error) {
	var distance [20]byte
	for i := 0; i < 20; i++ {
		distance[i] = rt.SelfID[i] ^ RemoteNode.NodeID[i]
	}

	// Iterate over each byte from the most significant byte
	for byteIndex := 0; byteIndex < 20; byteIndex++ {
		b := distance[byteIndex]
		if b != 0 {
			// Find the most significant bit in this byte
			for bit := 7; bit >= 0; bit-- {
				if (b>>bit)&1 == 1 {
					// MSB index: 0 is most significant bit, 159 is least
					return 159 - byteIndex*8 + (7 - bit), nil
				}
			}
		}
	}

	// IDs are identical
	return -1, nil
}
