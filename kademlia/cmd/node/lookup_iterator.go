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
