package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/juju/loggo"
	"github.com/remerge/rex/rollbar"
)

type Connection struct {
	net.Conn
	Server      *Server
	Id          string
	RemoteAddr  string
	Log         loggo.Logger
	LimitReader *io.LimitedReader
	Buffer      *bufio.ReadWriter
	tlsState    *tls.ConnectionState
}

func (c *Connection) Serve() {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case error:
				rollbar.Error(rollbar.CRIT, err.(error), &rollbar.Field{
					Name: "person",
					Data: map[string]string{
						"id": c.RemoteAddr,
					},
				})
			default:
				rollbar.Message(rollbar.CRIT, fmt.Sprintf("unhandled error: %v", err), &rollbar.Field{
					Name: "person",
					Data: map[string]string{
						"id": c.RemoteAddr,
					},
				})
			}
		}
		c.Server.closeRate.Update(1)
		c.Close()
	}()

	if tlsConn, ok := c.Conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			rollbar.Error(rollbar.WARN, err, &rollbar.Field{
				Name: "person",
				Data: map[string]string{
					"id": c.RemoteAddr,
				},
			})
			return
		}
		c.tlsState = new(tls.ConnectionState)
		*c.tlsState = tlsConn.ConnectionState()
		//if proto := c.tlsState.NegotiatedProtocol; validNPN(proto) {
		//    if fn := c.server.TLSNextProto[proto]; fn != nil {
		//        h := initNPNRequest{tlsConn, serverHandler{c.server}}
		//        fn(c.server, tlsConn, h)
		//    }
		//    return
		//}
	}

	c.Server.Handler.Handle(c)
}

// rstAvoidanceDelay is the amount of time we sleep after closing the
// write side of a TCP connection before closing the entire socket.
// By sleeping, we increase the chances that the client sees our FIN
// and processes its final data before they process the subsequent RST
// from closing a connection with known unread data.
// This RST seems to occur mostly on BSD systems. (And Windows?)
// This timeout is somewhat arbitrary (~latency around the planet).
const rstAvoidanceDelay = 500 * time.Millisecond

type closeWriter interface {
	CloseWrite() error
}

var _ closeWriter = (*net.TCPConn)(nil)

// closeWrite flushes any outstanding data and sends a FIN packet (if
// client is connected via TCP), signalling that we're done.  We then
// pause for a bit, hoping the client processes it before any
// subsequent RST.
//
// See https://golang.org/issue/3595
func (c *Connection) CloseWriteAndWait() {
	c.finalFlush()
	if tcp, ok := c.Conn.(closeWriter); ok {
		tcp.CloseWrite()
	}
	time.Sleep(rstAvoidanceDelay)
}

func (c *Connection) finalFlush() {
	if c.Buffer != nil {
		c.Buffer.Flush()
		putBufioReader(c.Buffer.Reader)
		putBufioWriter(c.Buffer.Writer)
		c.Buffer = nil
	}
}
