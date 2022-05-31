// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConnNoProtocol(t *testing.T) {
	rawURL := ""
	req, _ := http.NewRequest("POST", rawURL, nil)
	w := httptest.NewRecorder()
	_, err := NewConn(w, req)

	if assert.NotNil(t, err) {
		assert.Equal(t, err, errors.New("Upgrade header is not specified"))
	}

}

type mockresponseWriter struct {
	w http.ResponseWriter
}

func (w *mockresponseWriter) Header() http.Header {
	return w.w.Header()
}
func (w *mockresponseWriter) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *mockresponseWriter) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

func (w *mockresponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("Not support Hijack.")
}

func TestNewConnNoHijacker(t *testing.T) {
	rawURL := ""
	req, _ := http.NewRequest("POST", rawURL, nil)
	req.Header.Set("Upgrade", "ok")
	w := &mockresponseWriter{
		w: httptest.NewRecorder(),
	}
	_, err := NewConn(w, req)
	if assert.NotNil(t, err) {
		assert.EqualError(t, err, "Failed to get a raw connection for http upgrade: Not support Hijack.", "test no hijack")
	}

}

type mockresponseWriter2 struct {
	w http.ResponseWriter
}

func (w *mockresponseWriter2) Header() http.Header {
	return w.w.Header()
}
func (w *mockresponseWriter2) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *mockresponseWriter2) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

func (w *mockresponseWriter2) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	reader := bufio.NewReader(strings.NewReader("test"))

	buf := bytes.NewBuffer([]byte(""))
	writer := bufio.NewWriterSize(buf, 1024)

	_, err := writer.WriteString("non-empty buffer")
	rw := bufio.NewReadWriter(reader, writer)

	return nil, rw, err
}

func TestNewConnWriter(t *testing.T) {
	rawURL := ""
	req, _ := http.NewRequest("POST", rawURL, nil)
	req.Header.Set("Upgrade", "ok")
	w := &mockresponseWriter2{
		w: httptest.NewRecorder(),
	}
	_, err := NewConn(w, req)
	if assert.NotNil(t, err) {
		assert.EqualError(t, err, "Write buffer is not empty", "test writer is non-empty.")
	}

}
