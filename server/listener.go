package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"github.com/juju/loggo"
	"github.com/remerge/rex/log"
)

type Listener struct {
	net.Listener
	wg      sync.WaitGroup
	log     loggo.Logger
	stopped bool
}

func NewListener(port int) (listener *Listener, err error) {
	listener = &Listener{}

	listener.log = log.GetLogger(fmt.Sprintf("listener:%d", port))
	listener.log.Infof("start listen on port %d", port)

	listener.Listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func NewTlsListener(port int, config *tls.Config) (listener *Listener, err error) {
	listener, err = NewListener(port)
	if err != nil {
		return nil, err
	}

	listener.Listener = tls.NewListener(listener.Listener, config)
	return listener, nil
}

func (listener *Listener) Accept() (conn net.Conn, err error) {
	return listener.Listener.Accept()
}

func (listener *Listener) Run(callback func(*Listener) error) error {
	listener.wg.Add(1)
	defer listener.wg.Done()
	return callback(listener)
}

func (listener *Listener) Stop() {
	listener.stopped = true
	_ = listener.Listener.Close()
}

func (listener *Listener) IsStopped() bool {
	return listener.stopped
}

func (listener *Listener) Wait() {
	listener.wg.Wait()
}
