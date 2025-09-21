package cli

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/cmd/node"
)

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
	n, err := node.NewNode(*bind, "")
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
// cli_local.go
func cmdLocalGet(args []string) error {
	fs := flag.NewFlagSet("local-get", flag.ContinueOnError)
	to := fs.String("to", "127.0.0.1:9999", "address of local daemon")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: local-get <keyhex> [-to host:port]")
	}

	keyb, err := hex.DecodeString(fs.Arg(0))
	if err != nil || len(keyb) != 20 {
		return errors.New("bad key (need 40 hex chars)")
	}
	var key [20]byte
	copy(key[:], keyb)

	// client binds :0 (DO NOT BIND :9999)
	n, err := node.NewNode(":0", "")
	if err != nil {
		return err
	}
	n.Start()
	defer n.Close()

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
