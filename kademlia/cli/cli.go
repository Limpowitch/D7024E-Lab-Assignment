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
	case "ping":
		return cmdPing(args[1:])
	case "put":
		return cmdPut(args[1:])
	case "get":
		return cmdGet(args[1:])
	case "join":
		return cmdJoin(args[1:])
	case "rt":
		return cmdRT(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	case "local-put":
		return cmdLocalPut(args[1:])
	case "local-get":
		return cmdLocalGet(args[1:])

	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Println(`kademlia

Usage:
  serve   [-bind :9999] [-seeds host:port,host:port]
  join    -to host:port  [-bind :0]
  ping    -to host:port  [-bind :0]
  put     -to host:port  -value "UTF-8 string"
  get     keyhex@host:port
  local-put  [-to 127.0.0.1:9999] -value "..."
  local-get  keyhex [-to 127.0.0.1:9999]


Examples:
  kademlia serve -bind :9999 -seeds node1:9999,node2:9999
  kademlia join -to node1:9999
  kademlia ping -to node2:9999
  kademlia put -to node2:9999 -value "hello world"
  kademlia get  5e884898da28047151d0e56f8dc6292773603d0d@node2:9999`)
}
