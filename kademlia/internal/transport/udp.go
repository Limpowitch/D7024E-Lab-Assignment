package main

import (
	"net"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

type Handler func(from *net.UDPAddr)

type UDPServer struct {
	pc            net.PacketConn
	addressString string
	handler       Handler
	down          chan struct{}
}

func (server *UDPServer) Addr() string { return server.addressString }

// needs envelope?
func (server *UDPServer) Start() {}

// subject to change depending on the payload envelope (needed?) //samme
func (server *UDPServer) Send(target string, env wire.Envelope) error {
	conn, err := net.Dial("udp", target) // open udp socket
	if err != nil {
		return err
	}
	defer conn.Close()                 // always defer before action to ensure we release socket (straight from tutorial)
	_, err = conn.Write(env.Marshal()) // send msg
	return err
}

func (server *UDPServer) Reply(target *net.UDPAddr, env wire.Envelope) error {
	_, err := server.pc.WriteTo(env.Marshal(), target) // change payload to envelope once struct and functinos are implemented
	return err
}

func (server *UDPServer) Close() error {
	close(server.down)

	return server.pc.Close()
}
