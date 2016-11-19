package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"runtime/debug"
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

	br := newBufioReader(&c.LimitReader, server.BufferSize)
	bw := newBufioWriter(conn, server.BufferSize)
	c.Buffer.Reader = br
	c.Buffer.Writer = bw

	c.Server.numConns.Inc(1)
	return c
}

func newConnection() *Connection {
	if v := connectionPool.Get(); v != nil {
		return v.(*Connection)
	}
	return &Connection{}
}

func putConnection(c *Connection) {
	c.Server.numConns.Dec(1)

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

	connectionPool.Put(c)
}

var (
	bufioReaderPool sync.Pool
	bufioWriterPool sync.Pool
)

func newBufioReader(r io.Reader, size int) *bufio.Reader {
	if v := bufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReaderSize(r, size)
}

func putBufioReader(br *bufio.Reader) {
	br.Reset(nil)
	bufioReaderPool.Put(br)
}

func newBufioWriter(w io.Writer, size int) *bufio.Writer {
	if v := bufioWriterPool.Get(); v != nil {
		bw := v.(*bufio.Writer)
		bw.Reset(w)
		return bw
	}
	return bufio.NewWriterSize(w, size)
}

func putBufioWriter(bw *bufio.Writer) {
	bw.Reset(nil)
	bufioWriterPool.Put(bw)
}

func (c *Connection) Serve() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("unhandled panic: %v\n", err)
			debug.PrintStack()

			switch err.(type) {
			case error:
				rollbar.Error(rollbar.CRIT, err.(error), &rollbar.Field{
					Name: "person",
					Data: map[string]string{
						"id": c.Conn.RemoteAddr().String(),
					},
				})
			default:
				rollbar.Message(rollbar.CRIT,
					"unhandled server connection error",
					&rollbar.Field{Name: "error", Data: err},
					&rollbar.Field{Name: "person", Data: map[string]string{
						"id": c.Conn.RemoteAddr().String(),
					}},
				)
			}
		}
		c.Close()
	}()

	if tlsConn, ok := c.Conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			c.Server.tlsErrors.Inc(1)
			return
		}
	}

	c.Server.Handler.Handle(c)
}

func (c *Connection) Close() {
	// prevent double close
	if c.Conn == nil {
		return
	}

	if c.Server != nil {
		c.Server.closes.Inc(1)
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
