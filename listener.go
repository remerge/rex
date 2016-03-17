package rex

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
	"github.com/remerge/rex/rollbar"
	"github.com/tylerb/graceful"
)

type Listener struct {
	net.Listener
	TCPListener *net.TCPListener
	wg          sync.WaitGroup
	log         loggo.Logger
	accepts     *instruments.Rate
}

func NewListener(port int) (listener *Listener, err error) {
	listener = &Listener{}
	listener.log = loggo.GetLogger(fmt.Sprintf("listener:%d", port))
	listener.log.Infof("start listen on port %d", port)

	listener.accepts = reporter.NewRegisteredRate(fmt.Sprintf("listener.accepts:%d", port))

	listener.TCPListener, err = net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return nil, err
	}

	listener.Listener = listener.TCPListener
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

func (listener *Listener) Accept() (conn net.Conn, err error) {
	defer listener.accepts.Update(1)
	return listener.Listener.Accept()
}

func (listener *Listener) Serve(server *http.Server) {
	listener.wg.Add(1)
	go func() {
		defer listener.wg.Done()
		server.Handler = &recoveryHandler{h: server.Handler}
		if err := server.Serve(listener); !IsTimeout(err) {
			MayPanic(err)
		}
	}()
}

func (listener *Listener) ServeGraceful(server *graceful.Server) {
	listener.wg.Add(1)
	go func() {
		defer listener.wg.Done()
		server.Handler = &recoveryHandler{h: server.Handler}
		if err := server.Serve(listener); !IsTimeout(err) {
			MayPanic(err)
		}
	}()
}

func (listener *Listener) Stop() {
	err := listener.Listener.Close()
	if err != nil {
		listener.log.Warningf("listener shutdown error %s", err)
	}

}

func (listener *Listener) Done() {
	listener.wg.Done()
}

func (listener *Listener) Wait() {
	listener.wg.Wait()
}

type recoveryHandler struct {
	h http.Handler
}

var recoveryLock sync.Mutex

func (h *recoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			recoveryLock.Lock()
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("unknown error: %v", r)
			}
			rollbar.Error(rollbar.CRIT, err)
			rollbar.Wait()
		}
	}()
	h.h.ServeHTTP(w, r)
}
