package node

type RoutingTable struct {
	BucketList []Kbucket
}

func NewRoutingTable() RoutingTable {
	return RoutingTable{
		BucketList: make([]Kbucket, 0),
	}
}

func (rt *RoutingTable) AddToRT(value Kbucket) {
	rt.BucketList = append(rt.BucketList, value)
}

func (rt *RoutingTable) DeleteFromRT(value Kbucket) {
	for i, v := range rt.BucketList {
		if v.LowerLimit == value.LowerLimit && v.UpperLimit == value.UpperLimit {
			rt.BucketList = append(rt.BucketList[:i], rt.BucketList[i+1:]...)
			return
		}
	}
}
