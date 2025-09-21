package cli

import (
	"context"
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
