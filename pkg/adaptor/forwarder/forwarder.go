// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/confidential-containers/peer-pod-opensource/pkg/util/http/upgrader"
)

const SocketName = "agent.ttrpc"

var logger = log.New(log.Writer(), "[helper/forwarder] ", log.LstdFlags|log.Lmsgprefix)

type SocketForwarder interface {
	Start(ctx context.Context) error
	Ready() chan struct{}
	Shutdown() error
}

type socketForwarder struct {
	socketPath string
	serverURL  *url.URL
	readyCh    chan struct{}
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewSocketForwarder(socketPath string, serverURL *url.URL) SocketForwarder {

	return &socketForwarder{
		socketPath: socketPath,
		serverURL:  serverURL,
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

func (f *socketForwarder) Start(ctx context.Context) error {

	if err := os.MkdirAll(filepath.Dir(f.socketPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create parent directories for socket: %s", f.socketPath)
	}
	if err := os.Remove(f.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", f.socketPath, err)
	}

	logger.Printf("Listening on %s\n", f.socketPath)

	listener, err := net.Listen("unix", f.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", f.socketPath, err)
	}

	close(f.readyCh)

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

			startForwarding(ctx, conn, f.serverURL)
		}
	}()
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Printf("error closing upgraded connection listener: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		f.Shutdown()
	case <-f.stopCh:
	case err := <-listenerErr:
		return err
	}

	return nil
}

func (f *socketForwarder) Ready() chan struct{} {
	return f.readyCh
}

func (f *socketForwarder) Shutdown() error {
	f.stopOnce.Do(func() {
		close(f.stopCh)
	})
	return nil
}

func startForwarding(ctx context.Context, shimConn net.Conn, serverURL *url.URL) {

	go func() {
		defer func() {
			if err := shimConn.Close(); err != nil {
				logger.Printf("error closing shim connection: %v", err)
			}
		}()

		var serverConn net.Conn

		for {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

			var err error
			serverConn, err = upgrader.SendUpgradeRequest(ctx, serverURL, "ttrpc")
			if err == nil {
				logger.Printf("Upgrade is done.")
				cancel()
				break
			}

			logger.Printf("failed to establish an upgraded connection to %s: %v. (retrying...)", serverURL, err)
			<-ctx.Done()
		}
		defer func() {
			if err := serverConn.Close(); err != nil {
				logger.Printf("error closing server connection: %v", err)
			}
		}()

		done := make(chan struct{})
		go func() {
			defer close(done)

			logger.Printf("Ready to copy server to shim.")
			if _, err := io.Copy(serverConn, shimConn); err != nil {
				logger.Printf("error copying connection from shim to server: %v", err)
			}
		}()

		logger.Printf("Ready to copy shim to server.")
		if _, err := io.Copy(shimConn, serverConn); err != nil {
			logger.Printf("error copying connection from agent to server: %v", err)
		}

		<-done
	}()
}
