package node

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// hasPeers returns true if we know at least one contact.
func (n *Node) hasPeers() bool {
	return len(n.RoutingTable.Closest(n.NodeID, 1)) > 0
}

// PutValue stores 'value' at the K closest to key.
// 'bootstrap' may be empty; we use it only if our RT is empty.
func (n *Node) PutValue(ctx context.Context, bootstrap string, value string) ([20]byte, error) {
	key := SHA1ID([]byte(value))

	// If we don’t know anyone yet, try to learn a seed.
	if !n.hasPeers() {
		if bootstrap == "" {
			return key, fmt.Errorf("no known peers; provide -to seed:port once to bootstrap")
		}
		c1, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		_ = n.Svc.Ping(c1, bootstrap) // populates RT via OnSeen (best-effort)
		cancel()
	}

	// Iterative lookup for the key and fan-out STORE to K closest (as in the version I sent).
	closest, err := n.LookupNode(ctx, key)
	if err != nil {
		return key, fmt.Errorf("lookup for key failed: %w", err)
	}
	if len(closest) == 0 && bootstrap != "" {
		// last resort: store to the seed if lookup found nothing
		if err := n.Svc.Store(ctx, bootstrap, key, []byte(value)); err != nil {
			return key, fmt.Errorf("no peers & seed store failed: %w", err)
		}
		return key, nil
	}
	if len(closest) > K {
		closest = closest[:K]
	}

	alpha := 3
	ok := 0
	for i := 0; i < len(closest); i += alpha {
		end := i + alpha
		if end > len(closest) {
			end = len(closest)
		}
		ch := make(chan error, end-i)
		for _, c := range closest[i:end] {
			go func(addr string) {
				c2, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
				defer cancel()
				ch <- n.Svc.Store(c2, addr, key, []byte(value))
			}(c.Addr)
		}
		for j := i; j < end; j++ {
			if <-ch == nil {
				ok++
			}
		}
	}

	if ok == 0 {
		return key, fmt.Errorf("store fan-out failed: no acks")
	}
	return key, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// basically the same as the iterative find. this one can exit early, though, if we find the value!
func (n *Node) GetValue(ctx context.Context, key [20]byte) (string, []Contact, error) {
	// 0) quick local check (allowed by course delimitations)
	n.mu.RLock()
	if v, ok := n.Store[string(key[:])]; ok {
		n.mu.RUnlock()
		return string(v.Data), nil, nil
	}
	n.mu.RUnlock()

	// 1) seed shortlist with what we already know
	sl := newShortlist(key, K)
	sl.add(n.RoutingTable.Closest(key, K))

	prevBest := sl.best()

	for {
		select {
		case <-ctx.Done():
			return "", sl.contacts(), ctx.Err()
		default:
		}

		// 2) pick up to α closest unqueried peers
		batch := sl.nextBatch(alpha) // alpha = 3 in your shortlist file
		if len(batch) == 0 {
			break
		}

		// 3) run this round in parallel; cancel whole round if someone returns the value
		roundCtx, cancelRound := context.WithCancel(ctx)
		foundValCh := make(chan string, 1)

		var wg sync.WaitGroup
		var mu sync.Mutex // protects sl.add + RT updates + progressed
		progressed := false

		wg.Add(len(batch))
		for _, c := range batch {
			c := c
			go func() {
				defer wg.Done()

				// per-peer timeout inside the round
				rpcCtx, cancel := context.WithTimeout(roundCtx, 800*time.Millisecond)
				defer cancel()

				res, err := n.Svc.FindValue(rpcCtx, c.Addr, key)
				if err != nil {
					return // timeout/network err → ignore
				}

				// Early exit path: someone has the value
				if res.Value != nil {
					select {
					case foundValCh <- string(res.Value):
						// signal other goroutines in this round to stop quickly
						cancelRound()
					default:
						// another goroutine already sent; no-op
					}
					return
				}

				// Otherwise, merge returned contacts and update RT
				if res.Contacts != nil {
					cs, derr := UnmarshalContactList(res.Contacts)
					if derr == nil {
						mu.Lock()
						if sl.add(cs) { // returns true if shortlist gained/changed
							progressed = true
						}
						for _, rc := range cs {
							n.RoutingTable.Update(rc)
						}
						mu.Unlock()
					}
				}
			}()
		}

		// 4) wait for either a value or all goroutines done
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()

		select {
		case v := <-foundValCh:
			cancelRound()
			<-done // let goroutines wind down
			return v, nil, nil
		case <-done:
			// no value this round; continue if progressed or best improved
		case <-ctx.Done():
			cancelRound()
			<-done
			return "", sl.contacts(), ctx.Err()
		}

		cancelRound()

		// 5) convergence check (either new candidates appeared OR best distance improved)
		if !progressed && !sl.improved(prevBest) {
			break
		}
		prevBest = sl.best()
	}

	// not found—return closest contacts so caller can try elsewhere
	return "", sl.contacts(), fmt.Errorf("not found")
}
