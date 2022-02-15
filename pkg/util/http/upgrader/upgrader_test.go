// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestUpgrader(t *testing.T) {

	var message = "Hello"

	handler := NewHandler()

	mux := http.NewServeMux()
	mux.Handle("/upgrade", handler)
	httpServer := http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	addr := listener.Addr().String()

	errCh := make(chan error)
	go func() {
		defer close(errCh)

		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
	}()

	conn, err := SendUpgradeRequest(context.Background(), &url.URL{Scheme: "http", Host: addr, Path: "/upgrade"}, "test", WithLogger(log.Default()))
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	serverConn, err := handler.Accept()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	if _, err := serverConn.Write([]byte(message)); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	if err := serverConn.Close(); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	output, err := ioutil.ReadAll(conn)
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	if e, a := message, string(output); e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	if err := handler.Close(); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	if e, a := net.ErrClosed, handler.Close(); e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
}
