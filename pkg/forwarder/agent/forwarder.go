// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	// TODO: Handle agent proto
	_ "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"

	"github.com/confidential-containers/cloud-api-adapter/pkg/util/http/upgrader"
	"github.com/confidential-containers/cloud-api-adapter/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[daemon/agent] ", log.LstdFlags|log.Lmsgprefix)

type Forwarder interface {
	Start(ctx context.Context) error
	Shutdown() error
	ServeHTTP(w http.ResponseWriter, req *http.Request)
}

type forwarder struct {
	agentDialer  dialer
	listener     net.Listener
	httpUpgrader http.Handler

	stopCh   chan struct{}
	stopOnce sync.Once
}

type dialer func(context.Context) (net.Conn, error)

func NewForwarder(agentSocket, nsPath string) Forwarder {

	// agent-ctl assumes a trailing zero in an abstract Unix domain socket path.
	// https://github.com/kata-containers/kata-containers/blob/af0fbb94602a23501e2e8a17a5c98974ff0dc325/tools/agent-ctl/src/client.rs#L397-L404
	socket := agentSocket
	if len(socket) > 0 && socket[0] == '@' {
		socket = socket + "\x00"
	}

	agentDialer := func(ctx context.Context) (net.Conn, error) {

		if nsPath == "" {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}

		ns, err := netops.NewNSFromPath(nsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
		}

		var conn net.Conn
		if err := ns.Run(func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", socket)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to call dialer at namespace %q: %w", nsPath, err)
		}

		return conn, nil
	}

	httpUpgrader := upgrader.NewHandler()

	return &forwarder{
		agentDialer:  agentDialer,
		listener:     httpUpgrader,
		httpUpgrader: httpUpgrader,
		stopCh:       make(chan struct{}),
	}
}

func (f *forwarder) Start(ctx context.Context) error {

	listenerErr := make(chan error)
	go func() {
		defer close(listenerErr)

		for {
			conn, err := f.listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					listenerErr <- err
				}
				return
			}

			if err := startForwarding(ctx, conn, f.agentDialer); err != nil {
				listenerErr <- err
				return
			}
		}
	}()
	defer func() {
		if err := f.listener.Close(); err != nil {
			logger.Printf("error closing upgraded connection listener: %v", err)
		}
	}()

	select {
	case <-f.stopCh:
	case err := <-listenerErr:
		return err
	}

	return nil
}

func (f *forwarder) Shutdown() error {
	f.stopOnce.Do(func() {
		close(f.stopCh)
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

func (f *forwarder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f.httpUpgrader.ServeHTTP(w, req)
}
