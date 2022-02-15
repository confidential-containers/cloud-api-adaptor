// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/confidential-containers/peer-pod-opensource/pkg/util/http/upgrader"
)

func TestNewForwarder(t *testing.T) {

	socketPath := "/run/dummy.sock"
	serverURL, err := url.Parse("http://192.0.2.1:15150")
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	forwarder := NewSocketForwarder(socketPath, serverURL)
	f, ok := forwarder.(*socketForwarder)
	if !ok {
		t.Fatalf("expect %T, got %T", &socketForwarder{}, forwarder)
	}
	if e, a := socketPath, f.socketPath; e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}
	if e, a := serverURL.String(), f.serverURL.String(); e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}
}

func TestStartStop(t *testing.T) {

	dir := t.TempDir()

	socketPath := filepath.Join(dir, "test.sock")

	handler := upgrader.NewHandler()

	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	serverURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}
	path := "/agent"
	mux.Handle(path, handler)
	serverURL.Path = path

	forwarder := NewSocketForwarder(socketPath, serverURL)
	f, ok := forwarder.(*socketForwarder)
	if !ok {
		t.Fatalf("expect %T, got %T", &socketForwarder{}, forwarder)
	}

	forwarderErrCh := make(chan error)
	go func() {
		defer close(forwarderErrCh)

		if err := forwarder.Start(context.Background()); err != nil {
			forwarderErrCh <- err
		}
	}()

	<-f.Ready()

	clientMsg := "hello"
	serverMsg := "good bye"

	handlerErrCh := make(chan error)
	receivedMsgCh := make(chan string)
	go func() {
		defer close(handlerErrCh)

		err := func() error {
			for {
				conn, err := handler.Accept()
				if err != nil {
					if !errors.Is(err, net.ErrClosed) {
						return err
					}
					return nil
				}

				buf := make([]byte, len(clientMsg)+1)

				n, err := conn.Read(buf)
				if err != nil {
					return err
				}
				receivedMsgCh <- string(buf[:n])

				if _, err := conn.Write([]byte(serverMsg)); err != nil {
					return err
				}
				if err := conn.Close(); err != nil {
					return err
				}
			}
		}()
		if err != nil {
			handlerErrCh <- err
			return
		}
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	if _, err := conn.Write([]byte(clientMsg)); err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	msgFromClient := <-receivedMsgCh

	if e, a := clientMsg, msgFromClient; e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}

	t.Logf("msgFromClient=%q", msgFromClient)

	buf := make([]byte, len(serverMsg)+1)

	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	msgFromServer := string(buf[:n])
	if e, a := serverMsg, msgFromServer; e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}

	t.Logf("msgFromServer=%q", msgFromServer)

	if err := conn.Close(); err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	select {
	case err := <-handlerErrCh:
		t.Fatalf("expect no error, got %q", err)
	case err := <-forwarderErrCh:
		t.Fatalf("expect no error, got %q", err)
	default:
	}

	if err := f.Shutdown(); err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	if err := <-forwarderErrCh; err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	if err := handler.Close(); err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	if err := <-handlerErrCh; err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

}
