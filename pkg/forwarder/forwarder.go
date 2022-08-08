// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
)

var logger = log.New(log.Writer(), "[forwarder] ", log.LstdFlags|log.Lmsgprefix)

const (
	DefaultListenHost          = "0.0.0.0"
	DefaultListenPort          = "15150"
	DefaultListenAddr          = DefaultListenHost + ":" + DefaultListenPort
	DefaultConfigPath          = "/peerpod/daemon.json"
	DefaultPodNetworkSpecPath  = "/peerpod/podnetwork.json"
	DefaultKataAgentSocketPath = "@/run/kata-containers/agent.sock"
	DefaultKataAgentNamespace  = ""
	AgentURLPath               = "/agent"
)

type Config struct {
	PodNamespace string           `json:"pod-namespace"`
	PodName      string           `json:"pod-name"`
	PodNetwork   *tunneler.Config `json:"pod-network"`
}

type Daemon interface {
	Start(ctx context.Context) error
	Shutdown() error
	Addr() string
}

type daemon struct {
	listenAddr  string
	interceptor interceptor.Interceptor
	podNode     podnetwork.PodNode

	readyCh  chan struct{}
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewDaemon(spec *Config, listenAddr, kataAgentSocketPath, kataAgentNamespace string, podNode podnetwork.PodNode) Daemon {

	interceptor := interceptor.NewInterceptor(kataAgentSocketPath, kataAgentNamespace)

	daemon := &daemon{
		listenAddr:  listenAddr,
		interceptor: interceptor,
		podNode:     podNode,
		readyCh:     make(chan struct{}),
		stopCh:      make(chan struct{}),
	}

	return daemon
}

func (d *daemon) Start(ctx context.Context) error {

	if err := d.podNode.Setup(); err != nil {
		return fmt.Errorf("failed to set up pod network: %w", err)
	}
	defer func() {
		if err := d.podNode.Teardown(); err != nil {
			logger.Printf("failed to tear down pod network: %v", err)
		}
	}()

	listener, err := net.Listen("tcp", d.listenAddr)
	if err != nil {
		return err
	}
	d.listenAddr = listener.Addr().String()

	interceptorErrCh := make(chan error)
	go func() {
		defer close(interceptorErrCh)

		if err := d.interceptor.Start(ctx, listener); err != nil {
			interceptorErrCh <- fmt.Errorf("error running kata agent interceptor: %w", err)
		}
	}()
	defer func() {
		if err := d.interceptor.Shutdown(); err != nil {
			logger.Printf("error shutting down kata agent interceptor: %v", err)
		}
	}()

	close(d.readyCh)

	select {
	case <-ctx.Done():
		return d.Shutdown()
	case <-d.stopCh:
	case err := <-interceptorErrCh:
		return err
	}

	return nil
}

func (d *daemon) Shutdown() error {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
	return nil
}

func (d *daemon) Addr() string {
	<-d.readyCh
	return d.listenAddr
}
