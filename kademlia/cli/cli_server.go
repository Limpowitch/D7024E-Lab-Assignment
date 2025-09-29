package cli

import (
	"context"
	"crypto/rand"
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

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	ttlStr := fs.String("ttl", "24h", "TTL for stored values (e.g. 30s, 10m, 24h)")
	refreshStr := fs.String("refresh", "", "Refresh interval for origin (default: ttl/2)")
	bind := fs.String("bind", "0.0.0.0:9999", "UDP bind address")
	seeds := fs.String("seeds", "", "comma-separated bootstrap peers host:port")
	adv := fs.String("adv", "", "advertised addr host:port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ttl, err := time.ParseDuration(*ttlStr)
	if err != nil {
		return fmt.Errorf("bad -ttl: %w", err)
	}

	var refresh time.Duration
	if *refreshStr != "" {
		refresh, err = time.ParseDuration(*refreshStr)
		if err != nil {
			return fmt.Errorf("bad -refresh: %w", err)
		}
	}

	n, err := node.NewNode(*bind, *adv, ttl, refresh)
	if err != nil {
		return err
	}
	n.Start()
	fmt.Println("node listening on", n.Svc.Addr())

	// bootstrap: ping each seed and do one FindNode to kick-start RT
	// after n.Start() in cmdServe
	for _, s := range splitCSV(*seeds) {
		// 1) learn seed’s ID
		ctx, c := context.WithTimeout(context.Background(), time.Second)
		_ = n.Svc.Ping(ctx, s)
		c()

		// 2) several lookups to diversify buckets
		for i := 0; i < 4; i++ {
			ctx2, c2 := context.WithTimeout(context.Background(), 1200*time.Millisecond)
			// random target near self on first pass; fully random afterwards
			var t [20]byte
			if i == 0 {
				t = n.NodeID
			} else {
				if _, err := rand.Read(t[:]); err == nil { /* ok */
				}
			}
			_, err = n.LookupNode(ctx2, t)
			if err != nil {
				fmt.Println("error: ", err)
			}
			c2()
		}
	}

	waitForSignal()
	return n.Close()
}

func cmdForget(args []string) error {
	fs := flag.NewFlagSet("forget", flag.ContinueOnError)
	to := fs.String("to", "127.0.0.1:9999", "address of local daemon")
	bind := fs.String("bind", ":0", "local bind for the client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: forget <keyhex>")
	}

	keyb, err := hex.DecodeString(fs.Arg(0))
	if err != nil || len(keyb) != 20 {
		return errors.New("bad key (need 40 hex chars)")
	}
	var key [20]byte
	copy(key[:], keyb)

	n, err := node.NewNode(*bind, "", 24*time.Hour, 0)
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.Svc.AdminForget(ctx, *to, key); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func cmdExit(args []string) error {
	fs := flag.NewFlagSet("exit", flag.ContinueOnError)
	to := fs.String("to", "127.0.0.1:9999", "address of local daemon")
	bind := fs.String("bind", ":0", "local bind for the client")
	if err := fs.Parse(args); err != nil {
		return err
	}

	n, err := node.NewNode(*bind, "", 24*time.Hour, 0)
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := n.Svc.AdminExit(ctx, *to); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

// local-put: talk to 127.0.0.1:9999 (or override) and ask daemon to store.
func cmdLocalPut(args []string) error {
	fs := flag.NewFlagSet("local-put", flag.ContinueOnError)
	value := fs.String("value", "", "UTF-8 string to store")
	to := fs.String("to", "127.0.0.1:9999", "local daemon addr")
	bind := fs.String("bind", ":0", "ephemeral client bind")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *value == "" {
		return errors.New("-value is required")
	}

	// small client node just to send the admin RPC:
	n, err := node.NewNode(*bind, "", 24*time.Hour, 0)
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	key, err := n.Svc.AdminPut(ctx, *to, []byte(*value))
	if err != nil {
		return err
	}
	fmt.Printf("%x\n", key[:])
	return nil
}

// local-get: ask local daemon to resolve using its RT
func cmdLocalGet(args []string) error {
	fs := flag.NewFlagSet("local-get", flag.ContinueOnError)
	to := fs.String("to", "127.0.0.1:9999", "address of local daemon")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: get <keyhex>")
	}

	keyb, err := hex.DecodeString(fs.Arg(0))
	if err != nil || len(keyb) != 20 {
		return errors.New("bad key (need 40 hex chars)")
	}
	var key [20]byte
	copy(key[:], keyb)

	// client binds :0 (DO NOT BIND :9999)
	//n, err := node.NewNode(":0", "")
	n, err := node.NewNode(":0", "", 24*time.Hour, 0)
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

	// can probably be 5 seconds at this point (maybe less) but tried with 15 when we had race conditions
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	val, ok, err := n.Svc.AdminGet(ctx, *to, key)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("not found")
	}
	fmt.Println(string(val))
	fmt.Printf("%q\n", val)
	fmt.Printf("[len=%d]\n", len(val))
	return nil
}

// RT command: ask local node for its RT and print it out
func cmdRT(args []string) error {
	fs := flag.NewFlagSet("rt", flag.ContinueOnError)
	to := fs.String("to", "127.0.0.1:9999", "host:port of running node to inspect") // should probably hard-code this in AdminRT but i'll keep it here for now (we always just call our own address)
	bind := fs.String("bind", ":0", "local bind for the client")
	if err := fs.Parse(args); err != nil {
		return err
	}

	n, err := node.NewNode(*bind, "", 24*time.Hour, 0)
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

// Splits comma separated values and trims spaces
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

// Waiting for signal
func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
