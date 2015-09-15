package rex

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/juju/loggo"
)

type Listener struct {
	net.Listener
	TCPListener *net.TCPListener
	stop        chan int
	wg          sync.WaitGroup
	log         loggo.Logger
}

func NewListener(port int) (listener *Listener, err error) {
	listener = &Listener{}
	listener.log = loggo.GetLogger(fmt.Sprintf("listener:%d", port))
	listener.log.Infof("start listen on port %d", port)

	listener.TCPListener, err = net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return nil, err
	}

	listener.Listener = listener.TCPListener
	listener.stop = make(chan int)
	return listener, nil
}

func NewTlsListener(port int, key string, cert string) (listener *Listener, err error) {
	config := &tls.Config{}
	config.NextProtos = []string{"http/1.1"}

	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	listener, err = NewListener(port)
	listener.log.Infof("using TLS certificate at %s", cert)
	listener.Listener = tls.NewListener(listener.TCPListener, config)
	return listener, nil
}

var StoppedError = errors.New("listener stopped")

func (listener *Listener) Accept() (net.Conn, error) {
	for {
		listener.TCPListener.SetDeadline(time.Now().Add(1 * time.Second))

		newConn, err := listener.Listener.Accept()

		select {
		case <-listener.stop:
			listener.Listener.Close()
			return nil, StoppedError
		default:
		}

		if err != nil {
			if IsTimeout(err) {
				continue
			}
		}
		return newConn, err
	}
}

func (listener *Listener) Serve(server http.Server) {
	listener.wg.Add(1)
	defer listener.wg.Done()
	go func() {
		if err := server.Serve(listener); err != StoppedError {
			MayPanic(err)
		}
	}()
}

func (listener *Listener) Stop() {
	close(listener.stop)
}

func (listener *Listener) Done() {
	listener.wg.Done()
}

func (listener *Listener) Wait() {
	listener.wg.Wait()
}
