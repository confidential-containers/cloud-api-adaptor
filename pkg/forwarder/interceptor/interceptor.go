// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	// TODO: Handle agent proto
	_ "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[forwarder/interceptor] ", log.LstdFlags|log.Lmsgprefix)

type Interceptor interface {
	Start(ctx context.Context, listener net.Listener) error
	Shutdown() error
}

type interceptor struct {
	agentDialer dialer

	stopCh   chan struct{}
	stopOnce sync.Once
}

type dialer func(context.Context) (net.Conn, error)

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {

		if nsPath == "" {
			return (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
		}

		ns, err := netops.NewNSFromPath(nsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
		}

		var conn net.Conn
		if err := ns.Run(func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to call dialer at namespace %q: %w", nsPath, err)
		}

		return conn, nil
	}

	return &interceptor{
		agentDialer: agentDialer,
		stopCh:      make(chan struct{}),
	}
}

func (i *interceptor) Start(ctx context.Context, listener net.Listener) error {

	listenerErr := make(chan error)
	go func() {
		defer close(listenerErr)

		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					listenerErr <- err
				}
				return
			}

			if err := startForwarding(ctx, conn, i.agentDialer); err != nil {
				listenerErr <- err
				return
			}
		}
	}()
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Printf("error closing connection listener: %v", err)
		}
	}()

	select {
	case <-i.stopCh:
	case err := <-listenerErr:
		return err
	}

	return nil
}

func (i *interceptor) Shutdown() error {
	i.stopOnce.Do(func() {
		close(i.stopCh)
	})
	return nil
}

func startForwarding(ctx context.Context, shimConn net.Conn, agentDialer dialer) error {

	var agentConn net.Conn
	for {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

		var err error
		agentConn, err = agentDialer(ctx)
		if err == nil {
			cancel()
			break
		}
		log.Printf("error connecting to kata agent socket: %v (retrying)", err)

		<-ctx.Done()
	}

	go func() {
		defer func() {
			shimConn.Close()
			agentConn.Close()
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)

			_, err := io.Copy(agentConn, shimConn)
			if err != nil {
				logger.Printf("error copying connection from shim to agent: %v", err)
			}
		}()

		_, err := io.Copy(shimConn, agentConn)
		if err != nil {
			logger.Printf("error copying connection from agent to shim: %v", err)
		}

		<-done
	}()

	return nil
}
