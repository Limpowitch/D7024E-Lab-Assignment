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

func (server *UDPServer) Addr() string { return server.addressString }

// NEW: så vi kan sätta/ändra handler efter NewUDP()
func (server *UDPServer) SetHandler(h Handler) { server.handler = h }

// needs envelope?
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

func (server *UDPServer) Reply(target *net.UDPAddr, env wire.Envelope) error {
	_, err := server.pc.WriteTo(env.Marshal(), target) // change payload to envelope once struct and functinos are implemented
	return err
}

func (server *UDPServer) Request(target string, env wire.Envelope, timeout time.Duration) (wire.Envelope, error) {
	conn, err := net.Dial("udp", target)
	if err != nil {
		return wire.Envelope{}, err
	}
	defer conn.Close()

	if _, err := conn.Write(env.Marshal()); err != nil {
		return wire.Envelope{}, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return wire.Envelope{}, err
	}
	reply, err := wire.Unmarshal(buf[:n])
	if err != nil {
		return wire.Envelope{}, err
	}
	return reply, nil
}
