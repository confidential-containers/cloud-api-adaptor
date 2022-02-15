// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufferFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(-1)
		fmt.Fprintln(w, "Mocked test.")
	}))
	defer ts.Close()
	tsHost := ts.Listener.Addr().String()

	rawConn, _ := net.Dial("tcp", tsHost)
	reader := bufio.NewReader(strings.NewReader(""))
	buf := bytes.NewBuffer([]byte(""))
	writer := bufio.NewWriterSize(buf, 1024)
	rw := bufio.NewReadWriter(reader, writer)

	conn := &upgradedConn{
		Conn:   rawConn,
		buf:    rw.Reader,
		waitCh: make(chan struct{}),
	}

	rs := conn.Buffer()
	assert.Equalf(t, "*bufio.Reader", reflect.TypeOf(rs).String(), "verify the Buffer func")

	rsFile, _ := conn.File()
	assert.Equalf(t, "*os.File", reflect.TypeOf(rsFile).String(), "verify the Buffer func")

}

type mockAddr struct {
}

func (addr *mockAddr) Network() string {
	return "tcp"
}

func (addr *mockAddr) String() string {
	return "127.0.0.1:80"
}

type mockConn struct {
	errType string
}

func (conn *mockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}
func (conn *mockConn) Write(b []byte) (n int, err error) {
	return 0, nil
}
func (conn *mockConn) Close() error {
	if conn.errType == "errClosed" {
		return os.ErrClosed
	} else {
		return fmt.Errorf("mocked close error")
	}

}

func (conn *mockConn) LocalAddr() net.Addr {
	addr := &mockAddr{}
	return addr
}
func (conn *mockConn) RemoteAddr() net.Addr {
	return nil
}

func (conn *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (conn *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (conn *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestCloseOSError(t *testing.T) {
	mockConn := &mockConn{}
	mockConn.errType = "errClosed"
	reader := bufio.NewReader(strings.NewReader(""))
	buf := bytes.NewBuffer([]byte(""))
	writer := bufio.NewWriterSize(buf, 1024)
	rw := bufio.NewReadWriter(reader, writer)
	conn := &upgradedConn{
		Conn:   mockConn,
		buf:    rw.Reader,
		waitCh: make(chan struct{}),
	}
	err := conn.Close()
	assert.Contains(t, err.Error(), "upgraded conn already closed", "verify close error is os.ErrClosed")
}

func TestCloseOtherError(t *testing.T) {

	mockConn := &mockConn{}
	mockConn.errType = "unknownErr"

	reader := bufio.NewReader(strings.NewReader(""))
	buf := bytes.NewBuffer([]byte(""))
	writer := bufio.NewWriterSize(buf, 1024)
	rw := bufio.NewReadWriter(reader, writer)

	conn := &upgradedConn{
		Conn:   mockConn,
		buf:    rw.Reader,
		waitCh: make(chan struct{}),
	}
	err := conn.Close()

	assert.Equal(t, err.Error(), "mocked close error", "verify close error is other error")

}
