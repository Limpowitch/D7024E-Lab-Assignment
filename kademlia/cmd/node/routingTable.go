package main

import "sort"

type RoutingTable struct {
	BucketList []Kbucket
}

// Skapa en bucket som täcker hela keyspacet [0x00..00, 0xFF..FF] med capacity 20 (K).
func NewRoutingTable() (RoutingTable, error) {
	var lower, upper [20]byte
	for i := 0; i < 20; i++ {
		upper[i] = 0xFF
	}
	b, _ := NewKBucket(20, lower, upper, nil) // 20 = default K (matcha gärna n.K senare)
	return RoutingTable{
		BucketList: []Kbucket{b},
	}, nil
}

// ---- Publika metoder som Node anropar ----

// Lägger in/uppdaterar en kontakt. Split:ar bucket om full.
func (rt *RoutingTable) AddContact(c Contact) {
	for {
		idx := rt.bucketIndexForID(c.ID)
		if idx < 0 {
			return // borde inte hända
		}
		kb := &rt.BucketList[idx]

		// Uppdatera om redan finns
		for i := range kb.Contacts {
			if kb.Contacts[i].ID == c.ID {
				kb.Contacts[i].Address = c.Address
				return
			}
		}

		// Får plats? Lägg till.
		if len(kb.Contacts) < kb.Capacity {
			kb.AddToKBucket(c)
			return
		}

		// Full bucket -> split
		left, right := SplitBucket(*kb)
		// ersätt bucket med de två nya
		rt.BucketList = append(rt.BucketList[:idx], append([]Kbucket{left, right}, rt.BucketList[idx+1:]...)...)
		// loopa och försök igen; nu hamnar kontakten i rätt delbucket
	}
}

// Returnerar de n närmaste kontakterna (XOR-distans) till nyckeln över alla buckets.
func (rt *RoutingTable) FindClosestContacts(key [20]byte, n int) []Contact {
	var all []Contact
	for _, b := range rt.BucketList {
		all = append(all, b.ListIDs()...)
	}
	sort.Slice(all, func(i, j int) bool {
		return xorLess(all[i].ID, all[j].ID, key)
	})
	if n > len(all) {
		n = len(all)
	}
	// kopia för säkerhets skull
	out := append([]Contact(nil), all[:n]...)
	return out
}

// ---- Hjälpare ----

func (rt *RoutingTable) bucketIndexForID(id [20]byte) int {
	for i, b := range rt.BucketList {
		if compare(id, b.LowerLimit) >= 0 && compare(id, b.UpperLimit) <= 0 {
			return i
		}
	}
	return -1
}

// true om dist(a,key) < dist(b,key) i lexikografisk XOR-jämförelse
func xorLess(a, b, key [20]byte) bool {
	for i := 0; i < 20; i++ {
		da := a[i] ^ key[i]
		db := b[i] ^ key[i]
		if da != db {
			return da < db
		}
	}
	return false
}
