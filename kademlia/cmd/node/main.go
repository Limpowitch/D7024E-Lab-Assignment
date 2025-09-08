// cmd/node/main.go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	var bind string
	var sendTo string
	var msg string

	var reply bool
	var instance string

	flag.StringVar(&bind, "bind", ":9999", "udp bind address (something like :9999, 0.0.0.0:9999)")
	flag.StringVar(&sendTo, "send", "", "choose target host:port to send a message then exit")
	flag.StringVar(&msg, "msg", "", "send a message like --send (hostname)")

	flag.BoolVar(&reply, "reply", false, "if true auto replies 'ack' ")
	flag.StringVar(&instance, "instance", "", "simple label for each log")
	flag.Parse()

	//if non empty we build message and parse
	if sendTo != "" {
		if msg == "" {
			host, _ := os.Hostname()
			msg = "hello from " + host
		}
		if err := sendOnce(sendTo, "msg:"+msg); err != nil {
			fmt.Println("send error:", err)
			os.Exit(1)
		}
		fmt.Printf("[sent] to=%s msg=%q\n", sendTo, msg)
		return
	}

	// otherwise use the bind address and instance label and reply bool
	if err := listenAndLog(bind, instance, reply); err != nil {
		fmt.Println("listen error:", err)
		os.Exit(1)
	}
}

func sendOnce(target, payload string) error {
	conn, err := net.Dial("udp", target) // open udp socket
	if err != nil {
		return err
	}
	defer conn.Close()                   // always defer before action to ensure we release socket (straight from tutorial)
	_, err = conn.Write([]byte(payload)) // send msg
	return err
}

func listenAndLog(bind, inst string, doReply bool) error {
	pc, err := net.ListenPacket("udp", bind)
	if err != nil {
		return err
	}
	defer pc.Close()

	local := pc.LocalAddr().String() // print the actual bound address
	if inst != "" {
		fmt.Printf("listening on %s (%s) — ctrl-c to stop\n", local, inst)
	} else {
		fmt.Printf("listening on %s — ctrl-c to stop\n", local)
	}

	// this should listen to if we do ctrl-c but it doesnt rly work....
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	// ---------- improve this perhaps... ---------------

	buf := make([]byte, 2048)
	for {
		select {
		case <-sig:
			fmt.Println("\nbye")
			return nil
		default:
			n, from, err := pc.ReadFrom(buf) // n is amount of bytes received and from is sender addres
			if err != nil {
				return err
			}

			// good format, like: "type:payload" structure
			body := string(buf[:n])
			typ, content := splitTypeBody(body)

			// just added this for container debugging.... it works more or less the same at this point, since it's working!
			if inst != "" {
				fmt.Printf("[recv] inst=%s from=%s type=%s len=%d msg=%q\n", inst, from.String(), typ, n, content)
			} else {
				fmt.Printf("[recv] from=%s type=%s len=%d msg=%q\n", from.String(), typ, n, content)
			}

			if doReply {
				_, _ = pc.WriteTo([]byte("ack:ok"), from) // only if -reply is set
			}
		}
	}
}

func splitTypeBody(s string) (typ, body string) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "default", s
	}
	return s[:i], s[i+1:]
}
