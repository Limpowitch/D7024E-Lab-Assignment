// node/get_value_iterative.go
package node

import (
	"context"
	"log"
	"sync"
	"time"
)

func (n *Node) GetValueIterative(ctx context.Context, key [20]byte, seeds []Contact) (string, []Contact, error) {
	sl := newShortlist(key, K)

	if len(seeds) == 0 {
		seeds = n.RoutingTable.Closest(key, K)
	}
	if len(seeds) > 0 {
		sl.add(seeds)
	}

	for {
		batch := sl.nextBatch(alpha)
		if len(batch) == 0 {
			break
		}

		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			foundVal string
			progress bool
		)

		wg.Add(len(batch))
		for _, c := range batch {
			c := c
			go func() {
				defer wg.Done()

				rctx, cancel := context.WithTimeout(ctx, 1100*time.Millisecond)
				defer cancel()

				// DEBUG: prove we’re sending
				log.Printf("[iter-get] FIND_VALUE → %s (%x)", c.Addr, c.ID[:4])

				res, err := n.Svc.FindValue(rctx, c.Addr, key)
				if err != nil {
					return
				}

				// refresh the contact we just talked to
				n.RoutingTable.Update(Contact{ID: c.ID, Addr: c.Addr})

				if res.Value != nil {
					mu.Lock()
					foundVal = string(res.Value)
					mu.Unlock()
					return
				}

				if len(res.Contacts) > 0 {
					contacts, err := UnmarshalContactList(res.Contacts)
					if err != nil {
						return
					}
					// merge into shortlist; track whether we made progress
					mu.Lock()
					if sl.add(contacts) {
						progress = true
					}
					mu.Unlock()

					// put suggested contacts into our RT too
					for _, sc := range contacts {
						n.RoutingTable.Update(sc)
					}
				}
			}()
		}
		wg.Wait()

		if foundVal != "" {
			return foundVal, nil, nil
		}
		if !progress {
			break
		}
	}

	// Last-chance direct sweep over whatever we believe is closest
	for _, c := range sl.contacts() {
		rctx, cancel := context.WithTimeout(ctx, 1100*time.Millisecond)
		res, err := n.Svc.FindValue(rctx, c.Addr, key)
		cancel()
		if err == nil && res.Value != nil {
			return string(res.Value), nil, nil
		}
	}

	return "", sl.contacts(), context.DeadlineExceeded
}
