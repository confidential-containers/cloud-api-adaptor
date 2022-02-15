// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"errors"
	"fmt"
	"net/http"
)

func NewConn(w http.ResponseWriter, req *http.Request) (Conn, error) {
	protocol := req.Header.Get("Upgrade")
	if protocol == "" {
		return nil, errors.New("Upgrade header is not specified")
	}

	w.Header().Add("Connection", "Upgrade")
	w.Header().Add("Upgrade", protocol)
	w.WriteHeader(http.StatusSwitchingProtocols)

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("Not implement http.Hijacker: %T", w)
	}
	rawConn, rw, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("Failed to get a raw connection for http upgrade: %w", err)
	}

	if rw.Writer.Buffered() > 0 {
		return nil, errors.New("Write buffer is not empty")
	}

	conn := &upgradedConn{
		Conn:   rawConn,
		buf:    rw.Reader,
		waitCh: make(chan struct{}),
	}

	return conn, nil
}
