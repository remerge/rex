package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/remerge/rex/rollbar"
)

type Connection struct {
	net.Conn
	Server      *Server
	LimitReader io.LimitedReader
	Buffer      bufio.ReadWriter
}

// NoLimit is an effective infinite upper bound for io.LimitedReader
const NoLimit int64 = (1 << 63) - 1

var connectionPool sync.Pool

func (server *Server) NewConnection(conn net.Conn) *Connection {
	c := newConnection()
	c.Conn = conn
	c.Server = server

	c.LimitReader.R = conn
	c.LimitReader.N = NoLimit

	br := newBufioReader(&c.LimitReader)
	bw := newBufioWriter(conn)
	c.Buffer.Reader = br
	c.Buffer.Writer = bw

	c.Server.numConns.Update(1)
	return c
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

	if c.Buffer.Reader != nil {
		putBufioReader(c.Buffer.Reader)
		c.Buffer.Reader = nil
	}

	if c.Buffer.Writer != nil {
		putBufioWriter(c.Buffer.Writer)
		c.Buffer.Writer = nil
	}

	c.Server.numConns.Update(-1)
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
		c.Close()
	}()

	if tlsConn, ok := c.Conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			c.Server.tlsErrors.Update(1)
			return
		}
	}

	c.Server.Handler.Handle(c)
}

func (c *Connection) Close() {
	if c.Server != nil {
		c.Server.closeRate.Update(1)
	}

	// flush write buffer before close
	if c.Buffer.Writer != nil {
		c.Buffer.Writer.Flush()
	}

	// close socket
	if c.Conn != nil {
		c.Conn.Close()
	}

	// put connection back into pool
	putConnection(c)
}
