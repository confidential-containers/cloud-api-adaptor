// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

const (
	SocketName = "agent.ttrpc"
)

var logger = log.New(log.Writer(), "[adaptor/proxy] ", log.LstdFlags|log.Lmsgprefix)

type AgentProxy interface {
	Start(ctx context.Context, serverURL *url.URL) error
	Ready() chan struct{}
	Shutdown() error
}

type agentProxy struct {
	readyCh    chan struct{}
	stopCh     chan struct{}
	stopOnce   sync.Once
	socketPath string
}

func NewAgentProxy(socketPath string) AgentProxy {

	return &agentProxy{
		socketPath: socketPath,
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

func (p *agentProxy) Start(ctx context.Context, serverURL *url.URL) error {

	if err := os.MkdirAll(filepath.Dir(p.socketPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create parent directories for socket: %s", p.socketPath)
	}
	if err := os.Remove(p.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", p.socketPath, err)
	}

	logger.Printf("Listening on %s\n", p.socketPath)

	listener, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.socketPath, err)
	}

	close(p.readyCh)

	proxyService := newProxyService()

	if err := proxyService.connect(ctx, serverURL.Host); err != nil {
		return err
	}

	ttrpcServer, err := ttrpc.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create TTRPC server: %w", err)
	}

	pb.RegisterAgentServiceService(ttrpcServer, proxyService)
	pb.RegisterImageService(ttrpcServer, proxyService)
	pb.RegisterHealthService(ttrpcServer, proxyService)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ttrpcServerErr := make(chan error)
	go func() {
		defer close(ttrpcServerErr)

		if err := ttrpcServer.Serve(ctx, listener); err != nil && !errors.Is(err, ttrpc.ErrServerClosed) {
			ttrpcServerErr <- err
		}
	}()
	defer func() {
		if err := ttrpcServer.Shutdown(ctx); err != nil {
			logger.Printf("error shutting down TTRPC server: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		if err := p.Shutdown(); err != nil {
			logger.Printf("error on shutdown: %v", err)
		}
	case <-p.stopCh:
	case err := <-ttrpcServerErr:
		return err
	}

	return nil
}

func (p *agentProxy) Ready() chan struct{} {
	return p.readyCh
}

func (p *agentProxy) Shutdown() error {
	logger.Printf("shutting down socket forwarder")
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
	return nil
}
