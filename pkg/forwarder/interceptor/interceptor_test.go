// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"testing"
)

func TestNewInterceptor(t *testing.T) {

	socketName := "dummy.sock"

	ret := NewInterceptor(socketName, "")
	if ret == nil {
		t.Fatal("Expect non nil, got nil")
	}
	f, ok := ret.(*interceptor)
	if !ok {
		t.Fatalf("Expect *interceptor, got %T", f)
	}
	if f.agentDialer == nil {
		t.Fatal("Expect non nil, got nil")
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

func TestStart(t *testing.T) {

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	i := NewInterceptor(socketPath, "")
	if i == nil {
		t.Fatal("Expect non nil, got nil")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	serverURL := url.URL{
		Scheme: "grpc",
		Host:   listener.Addr().String(),
	}

	interceptorErrCh := make(chan error)
	go func() {
		defer close(interceptorErrCh)

		if err := i.Start(context.Background(), listener); err != nil {
			interceptorErrCh <- err
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
	case err := <-interceptorErrCh:
		t.Fatalf("Expect no error, got %q", err)
	default:
	}

	conn, err := net.Dial("tcp", serverURL.Host)
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

	if err := i.Shutdown(); err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}

	select {
	case err := <-interceptorErrCh:
		if err != nil {
			t.Fatalf("Expect no error, got %q", err)
		}
	default:
	}
}
