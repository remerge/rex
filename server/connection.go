package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"

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
			rollbar.Message(rollbar.CRIT, fmt.Sprintf("unhandled error: %v", err), &rollbar.Field{
				Name: "person",
				Data: map[string]string{
					"id": c.RemoteAddr,
				},
			})
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
