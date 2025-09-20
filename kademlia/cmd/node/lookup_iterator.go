package node

import (
	"context"
	"sync"
	"time"
)

// iterative lookup for target. returns K-closest contacts it discovers
func (n *Node) LookupNode(ctx context.Context, target [20]byte) ([]Contact, error) {
	sl := newShortlist(target, K)

	// seed with current routing table
	sl.add(n.RoutingTable.Closest(target, K))

	// if empty, we can still try any bootstrap peers you may have separately.
	//prevBest := sl.best()

	progressed := false

	for {
		batch := sl.nextBatch(alpha)
		if len(batch) == 0 {
			// nothing left to ask
			break
		}

		progressed = false

		// alpha parallel. each response adds more contacts.
		var wg sync.WaitGroup
		var mu sync.Mutex // guard merging into shortlist
		wg.Add(len(batch))

		for _, c := range batch {
			c := c
			go func() {
				defer wg.Done()

				// one-RPC timeout per peer (don’t block the whole lookup)
				rpcCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
				defer cancel()

				raw, err := n.Svc.FindNode(rpcCtx, c.Addr, target)
				if err != nil {
					return // timeout or network error → skip
				}
				contacts, err := UnmarshalContactList(raw)
				if err != nil {
					return
				}

				// after you unmarshal contacts from a FIND_NODE_RESP
				for _, c := range contacts {
					n.RoutingTable.Update(c) // add/refresh each suggested contact
				}

				// update our routing table when we talk to someone.
				n.RoutingTable.Update(Contact{ID: c.ID, Addr: c.Addr})

				// finally merge into shortlist
				mu.Lock()
				if sl.add(contacts) {
					progressed = true
				}
				mu.Unlock()
			}()
		}
		wg.Wait()

		if !progressed {
			break
		}

		// // stop once converged
		// if !sl.improved(prevBest) {
		// 	break
		// }
		// prevBest = sl.best()
	}

	return sl.contacts(), nil
}

// func (n *Node) GetValueIterative(ctx context.Context, key [20]byte, seeds []Contact) (string, []Contact, error) {
// 	sl := newShortlist(key, K)

// 	// seed with RT + optional seeds we just learned
// 	sl.add(n.RoutingTable.Closest(key, K))
// 	if len(seeds) > 0 {
// 		sl.add(seeds)
// 	}

// 	prevBest := sl.best()

// 	for {
// 		batch := sl.nextBatch(alpha)
// 		if len(batch) == 0 {
// 			break
// 		}

// 		var wg sync.WaitGroup
// 		var mu sync.Mutex
// 		var foundVal *string
// 		wg.Add(len(batch))

// 		for _, c := range batch {
// 			c := c
// 			go func() {
// 				defer wg.Done()

// 				rpcCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
// 				defer cancel()

// 				res, err := n.Svc.FindValue(rpcCtx, c.Addr, key)
// 				if err != nil {
// 					return
// 				}

// 				// we successfully talked to c → keep table warm
// 				n.RoutingTable.Update(Contact{ID: c.ID, Addr: c.Addr})

// 				if res.Value != nil {
// 					s := string(res.Value)
// 					mu.Lock()
// 					foundVal = &s
// 					mu.Unlock()
// 					return
// 				}

// 				if res.Contacts != nil {
// 					cs, err := UnmarshalContactList(res.Contacts)
// 					if err == nil && len(cs) > 0 {
// 						mu.Lock()
// 						sl.add(cs)
// 						mu.Unlock()
// 					}
// 				}
// 			}()
// 		}
// 		wg.Wait()

// 		if foundVal != nil {
// 			return *foundVal, nil, nil
// 		}
// 		if !sl.improved(prevBest) {
// 			break
// 		}
// 		prevBest = sl.best()
// 	}

// 	// not found → return the best contacts we have
// 	return "", sl.contacts(), nil
// }
