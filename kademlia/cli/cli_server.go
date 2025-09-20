package cli

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
)

// cmdServe
func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	// was ":9999"
	bind := fs.String("bind", "0.0.0.0:9999", "UDP bind address")
	seeds := fs.String("seeds", "", "comma-separated bootstrap peers host:port")
	adv := fs.String("adv", "", "advertised addr host:port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	n, err := node.NewNode(*bind, *adv) // <— pass adv
	if err != nil {
		return err
	}
	n.Start()
	fmt.Println("node listening on", n.Svc.Addr())

	// bootstrap: ping each seed and do one FindNode to kick-start RT
	for _, s := range splitCSV(*seeds) {
		ctx1, c1 := context.WithTimeout(context.Background(), time.Second)
		_ = n.Svc.Ping(ctx1, s)
		c1()

		ctx2, c2 := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		// ask the seed directly once; it will include itself now
		raw, err := n.Svc.FindNode(ctx2, s, n.NodeID)
		c2()
		if err == nil && len(raw) > 0 {
			if cs, e := node.UnmarshalContactList(raw); e == nil {
				for _, c := range cs {
					n.RoutingTable.Update(c)
				}
			}
		}
	}

	waitForSignal()
	return n.Close()
}

func cmdPing(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	to := fs.String("to", "", "peer host:port")
	bind := fs.String("bind", ":0", "local UDP bind (use :0 for random)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("-to is required")
	}

	n, err := node.NewNode(*bind, "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := n.Svc.Ping(ctx, *to); err != nil {
		return err
	}
	fmt.Println("PING ok →", *to)
	return nil
}

func cmdJoin(args []string) error {
	fs := flag.NewFlagSet("join", flag.ContinueOnError)
	to := fs.String("to", "", "bootstrap peer host:port")
	bind := fs.String("bind", ":0", "local bind")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("-to is required")
	}

	n, err := node.NewNode(*bind, "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	// ping → join via LookupNode(self)
	{
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = n.Svc.Ping(ctx, *to)
		cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = n.LookupNode(ctx, n.NodeID)
	fmt.Println("joined; closest known peers:", len(n.RoutingTable.Closest(n.NodeID, node.K)))
	return nil
}

func cmdPut(args []string) error {
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	to := fs.String("to", "", "bootstrap peer host:port (optional)")
	seed := fs.String("seed", "", "bootstrap peer host:port (optional)")
	val := fs.String("value", "", "UTF-8 string to store (required)")
	bind := fs.String("bind", ":0", "local bind (client ephemeral)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *val == "" {
		return errors.New("-value is required")
	}
	// We’ll pick one bootstrap if the client RT is empty.
	bootstrap := *to
	if bootstrap == "" {
		bootstrap = *seed
	}

	n, err := node.NewNode(*bind, "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Use node.PutValue which:
	//  - hashes value → key
	//  - if RT empty and bootstrap provided → PING bootstrap
	//  - iterative lookup(key) → STORE fan-out to K closest
	key, err := n.PutValue(ctx, bootstrap, *val)
	if err != nil {
		return err
	}

	fmt.Printf("%x\n", key[:])
	return nil
}

func cmdGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	bind := fs.String("bind", ":0", "local bind (client ephemeral)")
	seed := fs.String("seed", "", "bootstrap peer host:port (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: get keyhex[@host:port] [-seed host:port]")
	}

	// Parse key and optional @host:port
	keyStr, at, _ := strings.Cut(fs.Arg(0), "@")
	keyBytes, err := hex.DecodeString(keyStr)
	if err != nil || len(keyBytes) != 20 {
		return errors.New("bad key hex (need 40 hex chars)")
	}
	var key [20]byte
	copy(key[:], keyBytes)

	// Create a temporary node (client) bound on :0
	n, err := node.NewNode(*bind, "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	// Case A: direct ask (key@host:port)
	if at != "" {
		// Short direct attempt
		{
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()
			res, err := n.Svc.FindValue(ctx, at, key)
			if err == nil && res.Value != nil {
				fmt.Println(string(res.Value))
				return nil
			}
			// If we got closer contacts back, kick off full iterative
			if err == nil && len(res.Contacts) > 0 {
				seeds, _ := node.UnmarshalContactList(res.Contacts)
				ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel2()
				val, _, ierr := n.GetValueIterative(ctx2, key, seeds)
				if ierr == nil && val != "" {
					fmt.Println(val)
					return nil
				}
			}
		}
		// Fallback: iterative from our current RT (may be empty, still okay)
		{
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			val, contacts, err := n.GetValueIterative(ctx, key, nil)
			if err == nil && val != "" {
				fmt.Println(val)
				return nil
			}
			for _, c := range contacts { // debug
				fmt.Printf("%x %s\n", c.ID[:4], c.Addr)
			}
			if err != nil {
				return err
			}
			return errors.New("not found")
		}
	}

	// Case B: no @host:port → bootstrap iterative.
	// If -seed is provided, ping it to learn its NodeID (PONG carries ID), which OnSeen stores in RT.
	var seeds []node.Contact
	if *seed != "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = n.Svc.Ping(ctx, *seed) // even if PING fails, we still try lookup from RT
		cancel()
		// build a seed contact if we learned it
		// optional: expose helper to read last-seen from your service, or
		// just rely on RT.Update inside OnSeen (already wired in your Node constructor).
		// If you want to be explicit, you can do:
		// seeds = []node.Contact{{ID: learnedID, Addr: *seed}}
		// For now we pass nil and let the iterative walk use RT (which includes the seed if PING worked).
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	val, contacts, err := n.GetValueIterative(ctx, key, seeds)
	if err == nil && val != "" {
		fmt.Println(val)
		return nil
	}
	for _, c := range contacts { // debug
		fmt.Printf("%x %s\n", c.ID[:4], c.Addr)
	}
	if err != nil {
		return err
	}
	return errors.New("not found")
}

func cmdRT(args []string) error {
	fs := flag.NewFlagSet("rt", flag.ContinueOnError)
	to := fs.String("to", "", "host:port of running node to inspect")
	bind := fs.String("bind", ":0", "local bind for the client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("-to is required")
	}

	n, err := node.NewNode(*bind, "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	raw, err := n.Svc.AdminRT(ctx, *to)
	if err != nil {
		return err
	}

	cs, err := node.UnmarshalContactList(raw)
	if err != nil {
		return err
	}

	fmt.Printf("contacts=%d\n", len(cs))
	for i, c := range cs {
		fmt.Printf("%02d  %x  %s\n", i, c.ID[:4], c.Addr)
	}
	return nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
