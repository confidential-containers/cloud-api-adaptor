// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder/agent"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
)

var logger = log.New(log.Writer(), "[agent-protocol-forwarder] ", log.LstdFlags|log.Lmsgprefix)

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
	listenAddr     string
	agentForwarder agent.Forwarder
	podNode        podnetwork.PodNode

	readyCh  chan struct{}
	stopCh   chan struct{}
	stopOnce sync.Once
}

func New(spec *Config, listenAddr, kataAgentSocketPath, kataAgentNamespace string, podNode podnetwork.PodNode) Daemon {

	agentForwarder := agent.NewForwarder(kataAgentSocketPath, kataAgentNamespace)

	daemon := &daemon{
		listenAddr:     listenAddr,
		agentForwarder: agentForwarder,
		podNode:        podNode,
		readyCh:        make(chan struct{}),
		stopCh:         make(chan struct{}),
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

	agentForwarderErrCh := make(chan error)
	go func() {
		defer close(agentForwarderErrCh)

		if err := d.agentForwarder.Start(ctx, listener); err != nil {
			agentForwarderErrCh <- fmt.Errorf("error running kata agent forwarder: %w", err)
		}
	}()
	defer func() {
		if err := d.agentForwarder.Shutdown(); err != nil {
			logger.Printf("error shutting down kata agent forwarder: %v", err)
		}
	}()

	close(d.readyCh)

	select {
	case <-ctx.Done():
		return d.Shutdown()
	case <-d.stopCh:
	case err := <-agentForwarderErrCh:
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
