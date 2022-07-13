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
)

const (
	SocketName = "agent.ttrpc"
	maxRetries = 20
)

var logger = log.New(log.Writer(), "[helper/forwarder] ", log.LstdFlags|log.Lmsgprefix)

type SocketForwarder interface {
	Start(ctx context.Context, serverURL *url.URL) error
	Ready() chan struct{}
	Shutdown() error
}

type socketForwarder struct {
	readyCh    chan struct{}
	stopCh     chan struct{}
	stopOnce   sync.Once
	socketPath string
}

func NewSocketForwarder(socketPath string) SocketForwarder {

	return &socketForwarder{
		socketPath: socketPath,
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

func (f *socketForwarder) Start(ctx context.Context, serverURL *url.URL) error {

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

			startForwarding(ctx, conn, serverURL)
		}
	}()
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Printf("error closing upgraded connection listener: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		if shutdownErr := f.Shutdown(); shutdownErr != nil {
			logger.Printf("error on shutdown: %v", shutdownErr)
		}
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
	logger.Printf("shutting down socket forwarder")
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

		count := 1
		for {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

			var err error
			// TODO: Support TLS
			serverConn, err = net.Dial("tcp", serverURL.Host)
			if err == nil {
				logger.Printf("connection established: %s", serverURL.Host)
				cancel()
				break
			}

			logger.Printf("failed to connect to peer pod VM %s: %v. (retrying... %d/%d)", serverURL, err, count, maxRetries)
			<-ctx.Done()

			if count >= maxRetries {
				cancel()
				logger.Printf("reaches max retry count. gave up establishing connection to %s", serverURL)
				return
			}
			count++
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
