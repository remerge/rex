package server

import (
	"crypto/tls"
	"fmt"

	"github.com/juju/loggo"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/remerge/rex"
	"github.com/remerge/rex/log"
)

type Server struct {
	Id         string
	Port       int
	TlsPort    int
	TlsConfig  *tls.Config
	MaxConns   int64
	BufferSize int

	Log     loggo.Logger
	Handler Handler

	listener    *Listener
	tlsListener *Listener

	accepts      metrics.Counter
	tooManyConns metrics.Counter
	closes       metrics.Counter
	numConns     metrics.Counter
	tlsErrors    metrics.Counter
}

func NewServer(port int) (server *Server, err error) {
	server = &Server{}

	server.Id = fmt.Sprintf("server:%d", port)
	server.Port = port
	server.BufferSize = 32768

	server.Log = log.GetLogger(server.Id)
	server.Log.Infof("new server on port %d", port)

	server.accepts = metrics.GetOrRegisterCounter(fmt.Sprintf("rex.server,port=%d accept", port), nil)
	server.tooManyConns = metrics.GetOrRegisterCounter(fmt.Sprintf("rex.server,port=%d too_many_connection", port), nil)
	server.closes = metrics.GetOrRegisterCounter(fmt.Sprintf("rex.server,port=%d close", port), nil)
	server.numConns = metrics.GetOrRegisterCounter(fmt.Sprintf("rex.server,port=%d connection", port), nil)
	server.tlsErrors = metrics.GetOrRegisterCounter(fmt.Sprintf("rex.server,port=%d tls_error", port), nil)

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

func (server *Server) serve(listener *Listener) error {
	defer listener.Close()

	for {
		if listener.IsStopped() {
			return nil
		}

		conn, err := listener.Accept()
		if err != nil {
			if listener.IsStopped() {
				return nil
			}
			return err
		}

		server.accepts.Inc(1)

		if server.MaxConns > 0 && server.numConns.Count() >= server.MaxConns {
			// too many connections
			server.tooManyConns.Inc(1)
			conn.Close()
		} else {
			go server.NewConnection(conn).Serve()
		}
	}
}
