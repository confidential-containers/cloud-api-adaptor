// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/confidential-containers/cloud-api-adapter/pkg/forwarder/agent"
	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork/tunneler"
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
	httpServer     *http.Server
	agentForwarder agent.Forwarder
	podNode        podnetwork.PodNode

	readyCh  chan struct{}
	stopCh   chan struct{}
	stopOnce sync.Once
}

func New(spec *Config, listenAddr, kataAgentSocketPath, kataAgentNamespace string, podNode podnetwork.PodNode) Daemon {

	mux := http.NewServeMux()
	httpServer := &http.Server{
		Handler: mux,
	}

	agentForwarder := agent.NewForwarder(kataAgentSocketPath, kataAgentNamespace)

	mux.Handle(AgentURLPath, agentForwarder)

	daemon := &daemon{
		listenAddr:     listenAddr,
		httpServer:     httpServer,
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

	agentForwarder := make(chan error)
	go func() {
		defer close(agentForwarder)

		if err := d.agentForwarder.Start(ctx); err != nil {
			agentForwarder <- fmt.Errorf("error running kata agent forwarder: %w", err)
		}
	}()

	listener, err := net.Listen("tcp", d.listenAddr)
	if err != nil {
		return err
	}
	d.listenAddr = listener.Addr().String()

	httpServerErrCh := make(chan error)
	go func() {
		defer close(httpServerErrCh)

		if err := d.httpServer.Serve(listener); err != nil {
			httpServerErrCh <- fmt.Errorf("error running an http server: %w", err)
		}
	}()
	defer func() {
		if err := d.httpServer.Shutdown(context.Background()); err != nil {
			logger.Printf("error shutting down http server: %v", err)
		}
	}()

	close(d.readyCh)

	select {
	case <-ctx.Done():
		d.Shutdown()
	case <-d.stopCh:
	case err = <-agentForwarder:
	case err = <-httpServerErrCh:
	}

	return err
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
