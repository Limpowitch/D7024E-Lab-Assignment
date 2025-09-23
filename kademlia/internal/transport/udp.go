package transport

import (
	"net"
	"time"

	"github.com/Limpowitch/D7024E-Lab-Assignment/kademlia/wire"
)

type Handler func(from *net.UDPAddr, env wire.Envelope)

type UDPServer struct {
	pc            net.PacketConn
	addressString string
	handler       Handler
	down          chan struct{}
}

// Creates a new UDP transport server
func NewUDP(bind string, h Handler) (*UDPServer, error) {
	pc, err := net.ListenPacket("udp", bind)
	if err != nil {
		return nil, err
	}
	return &UDPServer{
		pc:            pc,
		addressString: pc.LocalAddr().String(),
		handler:       h,
		down:          make(chan struct{}),
	}, nil
}

// Returns the address the server is listening on
func (server *UDPServer) Addr() string { return server.addressString }

// needs envelope?
// Starts listening for incoming packets
func (server *UDPServer) Start() {
	go func() {
		buf := make([]byte, 2048)
		for {
			_ = server.pc.SetReadDeadline(time.Now().Add(750 * time.Millisecond))
			n, from, err := server.pc.ReadFrom(buf)
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-server.down:
					return
				default:
					continue
				}
			}
			if err != nil {
				return
			}
			//env, err := wire.Unmarshal(buf[:n])

			env, err := wire.Unmarshal(buf[:n])
			if err == nil && server.handler != nil {
				server.handler(from.(*net.UDPAddr), env)
			}
		}
	}()
}

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

// send from listener used for replies
func (server *UDPServer) SendFromListener(to string, env wire.Envelope) error {
	raddr, err := net.ResolveUDPAddr("udp", to)
	if err != nil {
		return err
	}
	_, err = server.pc.WriteTo(env.Marshal(), raddr)
	return err
}

// reply from listener used for replies
func (server *UDPServer) Reply(target *net.UDPAddr, env wire.Envelope) error {
	_, err := server.pc.WriteTo(env.Marshal(), target) // change payload to envelope once struct and functinos are implemented
	return err
}

// Stops the server and closes the underlying socket
func (server *UDPServer) Close() error {
	close(server.down)

	return server.pc.Close()
}
