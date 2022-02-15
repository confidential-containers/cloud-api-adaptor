// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

var logger = log.New(log.Writer(), "[http/upgrader] ", log.LstdFlags|log.Lmsgprefix)

type Conn interface {
	net.Conn

	Buffer() *bufio.Reader
	Wait()
}

type upgradedConn struct {
	net.Conn

	buf    *bufio.Reader
	waitCh chan struct{}
	once   sync.Once
}

func (conn *upgradedConn) Close() error {
	conn.once.Do(func() {
		close(conn.waitCh)
	})
	err := conn.Conn.Close()
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			err = fmt.Errorf("upgraded conn already closed: %w", net.ErrClosed)
		}
		if !errors.Is(err, net.ErrClosed) {
			logger.Printf("error closing upgraded conn: %v", err)
		}
	}
	return err
}

func (conn *upgradedConn) Read(b []byte) (int, error) {
	return conn.buf.Read(b)
}

func (conn *upgradedConn) Wait() {
	<-conn.waitCh
}

func (conn *upgradedConn) Buffer() *bufio.Reader {
	return conn.buf
}

func (conn *upgradedConn) File() (*os.File, error) {

	fileConn, ok := conn.Conn.(interface{ File() (*os.File, error) })
	if !ok {
		return nil, fmt.Errorf("%T does not implement File method", conn.Conn)
	}
	return fileConn.File()
}
