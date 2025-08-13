package testhelpers

import (
	"log"
	"net"
	"sync/atomic"
)

// TCPProxy is used for intercepting WebSocket connections and counting
// the number of bytes transferred.
type TCPProxy struct {
	// Destination endpoint to connect to.
	destHostPort string
	// Incoming endpoint to accept connections on.
	incomingHostPort string

	stopSignal chan struct{}

	// Byte counters in both directions.
	clientToServerBytes int64
	serverToClientBytes int64
}

func NewProxy(destHostPort string) *TCPProxy {
	return &TCPProxy{destHostPort: destHostPort}
}

func (p *TCPProxy) Start() error {
	// Begin listening on an available TCP port.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return err
	}

	// Remember the port that we listen on.
	p.incomingHostPort = ln.Addr().String()

	p.stopSignal = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.stopSignal:
				ln.Close()
				return
			default:
			}
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("Failed to Accept TCP connection: %v\n", err.Error())
				return
			}
			go p.forwardBothWays(conn)
		}
	}()

	return nil
}

func (p *TCPProxy) Stop() {
	close(p.stopSignal)
}

func (p *TCPProxy) IncomingEndpoint() string {
	return p.incomingHostPort
}

func (p *TCPProxy) forwardBothWays(in net.Conn) {
	// We have an incoming connection. Establish an outgoing connection
	// to the destination endpoint.
	out, err := net.Dial("tcp", p.destHostPort)
	if err != nil {
		return
	}

	defer out.Close()
	defer in.Close()

	// Forward TCP stream bytes from incoming to outgoing connection direction.
	go p.forwardConn(in, out, &p.clientToServerBytes)

	// Forward TCP stream bytes in the reverse direction.
	p.forwardConn(out, in, &p.serverToClientBytes)
}

func (p *TCPProxy) ServerToClientBytes() int {
	return int(atomic.LoadInt64(&p.serverToClientBytes))
}

func (p *TCPProxy) ClientToServerBytes() int {
	return int(atomic.LoadInt64(&p.clientToServerBytes))
}

func (p *TCPProxy) forwardConn(in, out net.Conn, byteCounter *int64) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-p.stopSignal:
			return
		default:
		}

		n, err := in.Read(buf)
		if err != nil {
			break
		}
		n, err = out.Write(buf[:n])
		if err != nil {
			break
		}
		atomic.AddInt64(byteCounter, int64(n))
	}
}
