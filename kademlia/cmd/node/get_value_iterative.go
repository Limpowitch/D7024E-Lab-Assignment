package node

import (
	"context"
	"log"
	"sync"
	"time"
)

// Iterative lookup for a value by its key
func (n *Node) GetValueIterative(ctx context.Context, key [20]byte, seeds []Contact) (string, []Contact, error) {
	foundCh := make(chan string, 1)

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

		// filter bad/self addrs
		tmp := batch[:0]
		for _, c := range batch {
			if c.ID == n.NodeID || c.Addr == "" || c.Addr[0] == ':' {
				continue
			}
			tmp = append(tmp, c)
		}
		batch = tmp
		if len(batch) == 0 {
			break
		}

		var (
			wg     sync.WaitGroup
			mu     sync.Mutex // protects shortlist.add
			doneCh = make(chan struct{})
		)

		wg.Add(len(batch))
		for _, c := range batch {
			c := c
			go func() {
				defer wg.Done()

				// Per-RPC timeout bounded by caller’s ctx
				rctx, rcancel := context.WithTimeout(ctx, 4*time.Second)
				defer rcancel()

				log.Printf("[iter] QUERY  -> %s key=%x", c.Addr, key[:4])
				res, err := n.Svc.FindValue(rctx, c.Addr, key)
				if err != nil {
					log.Printf("[iter] ERROR <- %s key=%x err=%v", c.Addr, key[:4], err)
					return
				}

				n.RoutingTable.Update(Contact{ID: c.ID, Addr: c.Addr})

				if res.Value != nil {
					log.Printf("[iter] VALUE <- %s key=%x len=%d", c.Addr, key[:4], len(res.Value))
					select {
					case foundCh <- string(res.Value):
					default:
					}
					return
				}

				if len(res.Contacts) > 0 {
					contacts, err := UnmarshalContactList(res.Contacts)
					if err != nil {
						return
					}
					mu.Lock()
					_ = sl.add(contacts) // we don't need the bool anymore
					mu.Unlock()
					for _, sc := range contacts {
						if sc.ID == n.NodeID || sc.Addr == "" || sc.Addr[0] == ':' {
							continue
						}
						n.RoutingTable.Update(sc)
					}
				}
			}()
		}

		go func() { wg.Wait(); close(doneCh) }()

		select {
		case v := <-foundCh:
			log.Printf("[iter] DELIVER len=%d", len(v))
			return v, nil, nil

		case <-doneCh:
			// fire next batch (if any); nextBatch(alpha) will be empty if no progress
			continue

		case <-ctx.Done():
			select {
			case v := <-foundCh:
				return v, nil, nil
			default:
			}
			return "", sl.contacts(), ctx.Err()
		}
	}

	return "", sl.contacts(), context.DeadlineExceeded
}
