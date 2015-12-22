package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/remerge/rex/rollbar"
)

type Connection struct {
	net.Conn
	Server      *Server
	LimitReader io.LimitedReader
	Buffer      bufio.ReadWriter
	tlsState    *tls.ConnectionState
}

// NoLimit is an effective infinite upper bound for io.LimitedReader
const NoLimit int64 = (1 << 63) - 1

var connectionPool sync.Pool

func (server *Server) NewConnection(conn net.Conn) (*Connection, error) {
	c := newConnection()
	c.Conn = conn
	c.Server = server

	c.LimitReader.R = conn
	c.LimitReader.N = NoLimit

	br := newBufioReader(&c.LimitReader)
	bw := newBufioWriter(conn)
	c.Buffer.Reader = br
	c.Buffer.Writer = bw

	c.tlsState = nil

	return c, nil
}

func newConnection() *Connection {
	if v := connectionPool.Get(); v != nil {
		return v.(*Connection)
	}
	return &Connection{}
}

func putConnection(c *Connection) {
	c.Conn = nil
	c.Server = nil
	c.LimitReader.R = nil
	c.LimitReader.N = 0
	c.Buffer.Reader = nil
	c.Buffer.Writer = nil
	c.tlsState = nil
	connectionPool.Put(c)
}

var (
	bufioReaderPool sync.Pool
	bufioWriterPool sync.Pool
)

func newBufioReader(r io.Reader) *bufio.Reader {
	if v := bufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReader(r)
}

func putBufioReader(br *bufio.Reader) {
	br.Reset(nil)
	bufioReaderPool.Put(br)
}

func newBufioWriter(w io.Writer) *bufio.Writer {
	if v := bufioWriterPool.Get(); v != nil {
		bw := v.(*bufio.Writer)
		bw.Reset(w)
		return bw
	}
	return bufio.NewWriterSize(w, 4096)
}

func putBufioWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	bufioWriterPool.Put(bw)
}
func (c *Connection) Serve() {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case error:
				rollbar.Error(rollbar.CRIT, err.(error), &rollbar.Field{
					Name: "person",
					Data: map[string]string{
						"id": c.Conn.RemoteAddr().String(),
					},
				})
			default:
				rollbar.Message(rollbar.CRIT, fmt.Sprintf("unhandled error: %v", err), &rollbar.Field{
					Name: "person",
					Data: map[string]string{
						"id": c.Conn.RemoteAddr().String(),
					},
				})
			}
		}
		c.Server.closeRate.Update(1)
		c.Close()
		putConnection(c)
	}()

	if tlsConn, ok := c.Conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			rollbar.Error(rollbar.WARN, err, &rollbar.Field{
				Name: "person",
				Data: map[string]string{
					"id": c.Conn.RemoteAddr().String(),
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
	if c.Buffer.Writer != nil {
		c.Buffer.Flush()
		putBufioReader(c.Buffer.Reader)
		c.Buffer.Reader = nil
		putBufioWriter(c.Buffer.Writer)
		c.Buffer.Writer = nil
	}
}
