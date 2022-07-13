// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestConnectSuccess(t *testing.T) {
	s := &proxyService{
		maxRetries:    20,
		retryInterval: 100 * time.Millisecond,
	}

	for {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}

		address := listener.Addr().String()

		if err := listener.Close(); err != nil {
			t.Fatalf("expect no error, got %q", err)
		}

		listenerErrCh := make(chan error)
		go func() {
			defer close(listenerErrCh)

			time.Sleep(250 * time.Millisecond)

			var err error
			// Open the same port
			listener, err = net.Listen("tcp", address)
			if err != nil {
				listenerErrCh <- err
			}
		}()

		err = s.connect(context.Background(), address)
		if err == nil {
			listener.Close()
			break
		}
		if e := <-listenerErrCh; e != nil {
			// A rare case occurs. Retry the test.
			t.Logf("%v", e)
			continue
		}

		listener.Close()
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}
		break
	}
}

func TestConnectFailure(t *testing.T) {
	s := &proxyService{
		maxRetries:    5,
		retryInterval: 100 * time.Millisecond,
	}

	address := "0.0.0.0:0"
	err := s.connect(context.Background(), address)
	if err == nil {
		t.Fatal("expect error, got nil")
	}
	if e, a := "reaches max retry count", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expect %q, got %q", e, a)
	}
}
