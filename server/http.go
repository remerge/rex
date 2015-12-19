package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"
)

type HTTPHandler struct {
}

func (h *HTTPHandler) Handle(c *Connection) {
	for {
		c.LimitReader.N = (1 << 20) + 4096
		req, err := h.ReadRequest(c)

		if err != nil {
			if c.LimitReader.N == 0 {
				// Their HTTP client may or may not be
				// able to read this if we're
				// responding to them and hanging up
				// while they're still writing their
				// request.  Undefined behavior.
				io.WriteString(c.Conn, "HTTP/1.1 413 Request Entity Too Large\r\n\r\n")
				c.Close()
				break
			} else if err == io.EOF {
				break // Don't reply
			} else if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				break // Don't reply
			}
			io.WriteString(c.Conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
			break
		}

		c.LimitReader.N = noLimit

		req.RemoteAddr = c.RemoteAddr
		req.TLS = c.tlsState

		//if body, ok := req.Body.(*body); ok {
		//    body.doEarlyClose = true
		//}

		//w = &response{
		//    conn:          c,
		//    req:           req,
		//    handlerHeader: make(Header),
		//    contentLength: -1,
		//}
		//w.cw.res = w
		//w.w = newBufioWriterSize(&w.cw, bufferBeforeChunkingSize)
		//return w, nil

		//// Expect 100 Continue support
		//req := w.req
		//if req.expectsContinue() {
		//    if req.ProtoAtLeast(1, 1) && req.ContentLength != 0 {
		//        // Wrap the Body reader with one that replies on the connection
		//        req.Body = &expectContinueReader{readCloser: req.Body, resp: w}
		//    }
		//    req.Header.Del("Expect")
		//} else if req.Header.get("Expect") != "" {
		//    w.sendExpectationFailed()
		//    break
		//}

		//// HTTP cannot have multiple simultaneous active requests.[*]
		//// Until the server replies to this request, it can't read another,
		//// so we might as well run the handler in this goroutine.
		//// [*] Not strictly true: HTTP pipelining.  We could let them all process
		//// in parallel even if their responses need to be serialized.
		//serverHandler{c.server}.ServeHTTP(w, w.req)
		//if c.hijacked() {
		//    return
		//}
		//w.finishRequest()
		//if !w.shouldReuseConnection() {
		//    if w.requestBodyLimitHit || w.closedRequestBodyEarly() {
		//        c.closeWriteAndWait()
		//    }
		//    break
		//}
		//c.setState(c.rwc, StateIdle)
		c.Conn.Write(responseOK)
	}
}

var responseOK = []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")

type badStringError struct {
	what string
	str  string
}

func (e *badStringError) Error() string { return fmt.Sprintf("%s %q", e.what, e.str) }

var contentLength = []byte("Content-Length")

func (h *HTTPHandler) ReadRequest(c *Connection) (req *http.Request, err error) {
	req = new(http.Request)
	req.ContentLength = -1

	// First line: GET / HTTP/1.1
	line, isPrefix, err := c.Buffer.ReadLine()
	if err != nil {
		return nil, err
	}

	if isPrefix {
		return nil, &badStringError{"request line too large", string(line)}
	}

	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	var ok bool
	req.Method, req.RequestURI, req.Proto, ok = parseRequestLine(line)
	if !ok {
		return nil, &badStringError{"malformed HTTP request", string(line)}
	}

	//rawurl := req.RequestURI
	//if req.ProtoMajor, req.ProtoMinor, ok = ParseHTTPVersion(req.Proto); !ok {
	//    return nil, &badStringError{"malformed HTTP version", req.Proto}
	//}

	//if req.URL, err = url.ParseRequestURI(rawurl); err != nil {
	//    return nil, err
	//}

	// ignore headers except content length
	for {
		line, isPrefix, err := c.Buffer.ReadLine()
		if err != nil {
			return nil, err
		}

		if isPrefix {
			return nil, &badStringError{"header line too large", string(line)}
		}

		if len(line) == 0 {
			break
		}

		if line[0] != 'C' {
			continue
		}

		if bytes.HasPrefix(line, contentLength) {
			i := bytes.IndexByte(line, ':')
			if i < 0 {
				return nil, &badStringError{"malformed Content-Length header", string(line)}
			}

			i++ // skip colon
			for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
				i++
			}

			req.ContentLength, err = strconv.ParseInt(string(line[i:]), 10, 64)
			if err != nil {
				return nil, &badStringError{"invalid Content-Length", string(line)}
			}
		}
	}

	switch {
	case req.ContentLength == 0:
		req.Body = eofReader
	case req.ContentLength > 0:
		req.Body = &body{src: io.LimitReader(c.Buffer.Reader, req.ContentLength), closing: req.Close}
	default:
		// Content-Length not mentioned in header
		if req.Close {
			// Close semantics (i.e. HTTP/1.0)
			req.Body = &body{src: c.Buffer.Reader, closing: req.Close}
		} else {
			// Persistent connection (i.e. HTTP/1.1)
			req.Body = eofReader
		}
	}

	return req, nil
}

// parseRequestLine parses "GET /foo HTTP/1.1" into its three parts.
func parseRequestLine(line []byte) (method, requestURI, proto string, ok bool) {
	s1 := bytes.IndexByte(line, ' ')
	s2 := bytes.IndexByte(line[s1+1:], ' ')
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return string(line[:s1]), string(line[s1+1 : s2]), string(line[s2+1:]), true
}

type eofReaderWithWriteTo struct{}

func (eofReaderWithWriteTo) WriteTo(io.Writer) (int64, error) { return 0, nil }
func (eofReaderWithWriteTo) Read([]byte) (int, error)         { return 0, io.EOF }

// eofReader is a non-nil io.ReadCloser that always returns EOF.
// It has a WriteTo method so io.Copy won't need a buffer.
var eofReader = &struct {
	eofReaderWithWriteTo
	io.Closer
}{
	eofReaderWithWriteTo{},
	ioutil.NopCloser(nil),
}

// body turns a Reader into a ReadCloser.
// Close ensures that the body has been fully read
// and then reads the trailer if necessary.
type body struct {
	src     io.Reader
	hdr     interface{}   // non-nil (Response or Request) value means read trailer
	r       *bufio.Reader // underlying wire-format reader for the trailer
	closing bool          // is the connection to be closed after reading body?
	//doEarlyClose bool          // whether Close should stop early

	mu         sync.Mutex // guards closed, and calls to Read and Close
	sawEOF     bool
	closed     bool
	earlyClose bool // Close called and we didn't read to the end of src
}

// ErrBodyReadAfterClose is returned when reading a Request or Response
// Body after the body has been closed. This typically happens when the body is
// read after an HTTP Handler calls WriteHeader or Write on its
// ResponseWriter.
var ErrBodyReadAfterClose = errors.New("http: invalid Read on closed Body")

func (b *body) Read(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, ErrBodyReadAfterClose
	}
	return b.readLocked(p)
}

// Must hold b.mu.
func (b *body) readLocked(p []byte) (n int, err error) {
	if b.sawEOF {
		return 0, io.EOF
	}
	n, err = b.src.Read(p)

	if err == io.EOF {
		b.sawEOF = true
		// Chunked case. Read the trailer.
		if b.hdr != nil {
			if e := b.readTrailer(); e != nil {
				err = e
				// Something went wrong in the trailer, we must not allow any
				// further reads of any kind to succeed from body, nor any
				// subsequent requests on the server connection. See
				// golang.org/issue/12027
				b.sawEOF = false
				b.closed = true
			}
			b.hdr = nil
		} else {
			// If the server declared the Content-Length, our body is a LimitedReader
			// and we need to check whether this EOF arrived early.
			if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > 0 {
				err = io.ErrUnexpectedEOF
			}
		}
	}

	// If we can return an EOF here along with the read data, do
	// so. This is optional per the io.Reader contract, but doing
	// so helps the HTTP transport code recycle its connection
	// earlier (since it will see this EOF itself), even if the
	// client doesn't do future reads or Close.
	if err == nil && n > 0 {
		if lr, ok := b.src.(*io.LimitedReader); ok && lr.N == 0 {
			err = io.EOF
			b.sawEOF = true
		}
	}

	return n, err
}

var (
	singleCRLF = []byte("\r\n")
	doubleCRLF = []byte("\r\n\r\n")
)

func seeUpcomingDoubleCRLF(r *bufio.Reader) bool {
	for peekSize := 4; ; peekSize++ {
		// This loop stops when Peek returns an error,
		// which it does when r's buffer has been filled.
		buf, err := r.Peek(peekSize)
		if bytes.HasSuffix(buf, doubleCRLF) {
			return true
		}
		if err != nil {
			break
		}
	}
	return false
}

var errTrailerEOF = errors.New("http: unexpected EOF reading trailer")

func (b *body) readTrailer() error {
	// The common case, since nobody uses trailers.
	buf, err := b.r.Peek(2)
	if bytes.Equal(buf, singleCRLF) {
		b.r.Discard(2)
		return nil
	}
	if len(buf) < 2 {
		return errTrailerEOF
	}
	if err != nil {
		return err
	}

	// Make sure there's a header terminator coming up, to prevent
	// a DoS with an unbounded size Trailer.  It's not easy to
	// slip in a LimitReader here, as textproto.NewReader requires
	// a concrete *bufio.Reader.  Also, we can't get all the way
	// back up to our conn's LimitedReader that *might* be backing
	// this bufio.Reader.  Instead, a hack: we iteratively Peek up
	// to the bufio.Reader's max size, looking for a double CRLF.
	// This limits the trailer to the underlying buffer size, typically 4kB.
	if !seeUpcomingDoubleCRLF(b.r) {
		return errors.New("http: suspiciously long trailer after chunked body")
	}

	hdr, err := textproto.NewReader(b.r).ReadMIMEHeader()
	if err != nil {
		if err == io.EOF {
			return errTrailerEOF
		}
		return err
	}
	switch rr := b.hdr.(type) {
	case *http.Request:
		mergeSetHeader(&rr.Trailer, http.Header(hdr))
	case *http.Response:
		mergeSetHeader(&rr.Trailer, http.Header(hdr))
	}
	return nil
}

func mergeSetHeader(dst *http.Header, src http.Header) {
	if *dst == nil {
		*dst = src
		return
	}
	for k, vv := range src {
		(*dst)[k] = vv
	}
}

// unreadDataSizeLocked returns the number of bytes of unread input.
// It returns -1 if unknown.
// b.mu must be held.
func (b *body) unreadDataSizeLocked() int64 {
	if lr, ok := b.src.(*io.LimitedReader); ok {
		return lr.N
	}
	return -1
}

func (b *body) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	var err error
	switch {
	case b.sawEOF:
		// Already saw EOF, so no need going to look for it.
	case b.hdr == nil && b.closing:
		// no trailer and closing the connection next.
		// no point in reading to EOF.
	//case b.doEarlyClose:
	//    // Read up to maxPostHandlerReadBytes bytes of the body, looking for
	//    // for EOF (and trailers), so we can re-use this connection.
	//    if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > maxPostHandlerReadBytes {
	//        // There was a declared Content-Length, and we have more bytes remaining
	//        // than our maxPostHandlerReadBytes tolerance. So, give up.
	//        b.earlyClose = true
	//    } else {
	//        var n int64
	//        // Consume the body, or, which will also lead to us reading
	//        // the trailer headers after the body, if present.
	//        n, err = io.CopyN(ioutil.Discard, bodyLocked{b}, maxPostHandlerReadBytes)
	//        if err == io.EOF {
	//            err = nil
	//        }
	//        if n == maxPostHandlerReadBytes {
	//            b.earlyClose = true
	//        }
	//    }
	default:
		// Fully consume the body, which will also lead to us reading
		// the trailer headers after the body, if present.
		_, err = io.Copy(ioutil.Discard, bodyLocked{b})
	}
	b.closed = true
	return err
}

func (b *body) didEarlyClose() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.earlyClose
}

// bodyLocked is a io.Reader reading from a *body when its mutex is
// already held.
type bodyLocked struct {
	b *body
}

func (bl bodyLocked) Read(p []byte) (n int, err error) {
	if bl.b.closed {
		return 0, ErrBodyReadAfterClose
	}
	return bl.b.readLocked(p)
}
