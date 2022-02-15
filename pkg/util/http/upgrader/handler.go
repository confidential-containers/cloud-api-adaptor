// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"net"
	"net/http"
	"sync"
)

type Handler interface {
	net.Listener
	http.Handler
}
type handler struct {
	connCh chan Conn
	closed bool
	mutex  sync.Mutex
}

func NewHandler() Handler {
	return &handler{
		connCh: make(chan Conn),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	conn, err := NewConn(w, r)
	if err != nil {
		logger.Printf("%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.mutex.Lock()
	closed := h.closed
	if !closed {
		h.connCh <- conn
	}
	h.mutex.Unlock()

	if closed {
		conn.Close()
		logger.Print("upgrader listener is closed")
		return
	}

	conn.Wait()
}

func (h *handler) Accept() (net.Conn, error) {
	conn := <-h.connCh
	if conn == nil {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (h *handler) Close() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.closed {
		return net.ErrClosed
	}
	close(h.connCh)
	h.closed = true

	return nil
}

type upgraderAddr struct{}

func (a *upgraderAddr) Network() string {
	return "unknown network"
}

func (a *upgraderAddr) String() string {
	return "unknown address"
}

func (h *handler) Addr() net.Addr {
	return &upgraderAddr{}
}
