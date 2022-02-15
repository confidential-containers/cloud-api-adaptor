// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package agent

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

	socketName := "dummy.sock"

	ret := NewForwarder(socketName, "")
	if ret == nil {
		t.Fatal("Expect non nil, got nil")
	}
	if _, ok := ret.(http.Handler); !ok {
		t.Fatalf("Expect http.Handler, got %T", ret)
	}
	f, ok := ret.(*forwarder)
	if !ok {
		t.Fatalf("Expect *forwarder, got %T", f)
	}
	if f.agentDialer == nil {
		t.Fatal("Expect non nil, got nil")
	}
	if f.httpUpgrader == nil {
		t.Fatal("Expect non nil, got nil")
	}
	if _, ok := f.httpUpgrader.(upgrader.Handler); !ok {
		t.Fatalf("Expect upgrader.Handler, got %T", f.httpUpgrader)
	}
	if f.listener == nil {
		t.Fatal("Expect non nil, got nil")
	}
	if _, ok := f.listener.(upgrader.Handler); !ok {
		t.Fatalf("Expect upgrader.Handler, got %T", f.listener)
	}
	if f.stopCh == nil {
		t.Fatal("Expect non nil, got nil")
	}
	select {
	case <-f.stopCh:
		t.Fatal("channel is closed")
	default:
	}
}

type mockBackend struct{}

func (*mockBackend) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
func (*mockBackend) Shutdown() error {
	return nil
}
func (*mockBackend) AddHandler(pattern string, handler http.Handler) error {
	return nil
}

func TestStart(t *testing.T) {

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	f := NewForwarder(socketPath, "")
	if f == nil {
		t.Fatal("Expect non nil, got nil")
	}

	httpServer := httptest.NewServer(f)

	serverURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	forwarderErrCh := make(chan error)
	go func() {
		defer close(forwarderErrCh)

		if err := f.Start(context.Background()); err != nil {
			forwarderErrCh <- err
		}
	}()

	msg := "Hello"

	// Launch a dummy agent
	agentErr := make(chan error)
	go func() {
		defer close(agentErr)

		err := func() error {
			agentListener, err := net.Listen("unix", socketPath)
			if err != nil {
				return err
			}

			for {
				conn, err := agentListener.Accept()
				if err != nil {
					if !errors.Is(err, net.ErrClosed) {
						return nil
					}
					return err
				}
				buffer := make([]byte, len(msg))
				n, err := conn.Read(buffer)
				if err != nil {
					return err
				}
				if _, err := conn.Write(buffer[0:n]); err != nil {
					return err
				}
				if err := conn.Close(); err != nil {
					return err
				}
			}
		}()
		if err != nil {
			agentErr <- err
		}
	}()

	select {
	case err := <-forwarderErrCh:
		t.Fatalf("Expect no error, got %q", err)
	default:
	}

	conn, err := upgrader.SendUpgradeRequest(context.Background(), serverURL, "agent")
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	buffer := make([]byte, len(msg))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	if e, a := msg, string(buffer[0:n]); e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}

	if err := f.Shutdown(); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	select {
	case err := <-forwarderErrCh:
		if err != nil {
			t.Fatalf("Expect no error, got %q", err)
		}
	}
}
