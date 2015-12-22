package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/heroku/instruments"
	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
	"github.com/remerge/rex"
	"github.com/remerge/rex/rollbar"
)

type Server struct {
	Id              string
	Port            int
	TlsPort         int
	TlsConfig       *tls.Config
	Log             loggo.Logger
	listener        *Listener
	tlsListener     *Listener
	Handler         Handler
	acceptRate      *instruments.Rate
	acceptErrorRate *instruments.Rate
	closeRate       *instruments.Rate
}

func NewServer(port int) (server *Server, err error) {
	server = &Server{
		Id:   fmt.Sprintf("server:%d", port),
		Port: port,
	}

	server.Log = loggo.GetLogger(server.Id)
	server.Log.Infof("new server on port %d", port)

	server.acceptRate = reporter.NewRegisteredRate(fmt.Sprintf("server.accept:%d", port))
	server.acceptErrorRate = reporter.NewRegisteredRate(fmt.Sprintf("server.accept.error:%d", port))
	server.closeRate = reporter.NewRegisteredRate(fmt.Sprintf("server.close:%d", port))

	return server, nil
}

func NewServerWithTLS(port int, tlsPort int, key string, cert string) (server *Server, err error) {
	server, err = NewServer(port)
	if err != nil {
		return nil, err
	}

	if tlsPort < 1 {
		return server, nil
	}

	server.Log.Infof("using TLS key=%v cert=%v", key, cert)
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	server.TlsConfig = &tls.Config{Certificates: []tls.Certificate{pair}}
	server.TlsPort = tlsPort

	return server, nil
}

func (server *Server) HasTLS() bool {
	return server.TlsPort > 0 && server.TlsConfig != nil
}

func (server *Server) Listen() (err error) {
	server.listener, err = NewListener(server.Port)
	if err != nil {
		return err
	}

	if server.HasTLS() {
		server.tlsListener, err = NewTlsListener(server.TlsPort, server.TlsConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (server *Server) Run() error {
	if err := server.Listen(); err != nil {
		return err
	}

	go server.Serve()
	if server.HasTLS() {
		go server.ServeTLS()
	}

	return nil
}

func (server *Server) Stop() {
	if server == nil {
		return
	}

	if server.HasTLS() && server.tlsListener != nil {
		server.Log.Infof("shutting down TLS listener")
		server.tlsListener.Stop()
		server.Log.Infof("waiting for requests to finish")
		server.tlsListener.Wait()
	}

	if server.listener != nil {
		server.Log.Infof("shutting down listener")
		server.listener.Stop()
		server.Log.Infof("waiting for requests to finish")
		server.listener.Wait()
	}
}

func (server *Server) Serve() {
	rex.MayPanic(server.listener.Run(server.serve))
}

func (server *Server) ServeTLS() {
	rex.MayPanic(server.tlsListener.Run(server.serve))
}

func (server *Server) serve(l *Listener) error {
	defer l.Close()

	b := &rex.BackoffDuration{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	for {
		conn, err := l.Accept()
		server.acceptRate.Update(1)

		if err != nil {
			server.acceptErrorRate.Update(1)
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				rollbar.Error(rollbar.ERR, err)
				time.Sleep(b.Duration())
				continue
			}
			// ignore stray shutdown error
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			return err
		}

		b.Reset()

		c, err := server.NewConnection(conn)
		if err != nil {
			continue
		}

		go c.Serve()
	}
}
