package cli

import (
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "put":
		return cmdLocalPut(args[1:])
	case "get":
		return cmdLocalGet(args[1:])
	case "rt":
		return cmdRT(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Println(`kademlia

Usage:
  serve   [-bind :9999] [-seeds host:port,host:port]
  put  [-to 127.0.0.1:9999] -value "..."
  get  keyhex [-to 127.0.0.1:9999]


Examples:
  docker exec d7024e-lab-assignment-node-# /app/node serve -bind :9999 -seeds node1:9999,node2:9999
  docker exec d7024e-lab-assignment-node-# /app/node put -to node2:9999 -value "hello world"
  docker exec d7024e-lab-assignment-node-# /app/node get  5e884898da28047151d0e56f8dc6292773603d0d@node2:9999`)
}
