// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
)

var logger = log.New(log.Writer(), "[forwarder] ", log.LstdFlags|log.Lmsgprefix)

const (
	DefaultListenHost          = "0.0.0.0"
	DefaultListenPort          = "15150"
	DefaultListenAddr          = DefaultListenHost + ":" + DefaultListenPort
	DefaultConfigPath          = "/run/peerpod/apf.json"
	DefaultPodNetworkSpecPath  = "/run/peerpod/podnetwork.json"
	DefaultKataAgentSocketPath = "/run/kata-containers/agent.sock"
	DefaultPodNamespace        = "/run/netns/podns"
	AgentURLPath               = "/agent"
)

type Config struct {
	PodNetwork   *tunneler.Config `json:"pod-network"`
	PodNamespace string           `json:"pod-namespace"`
	PodName      string           `json:"pod-name"`

	TLSServerKey  string `json:"tls-server-key,omitempty"`
	TLSServerCert string `json:"tls-server-cert,omitempty"`
	TLSClientCA   string `json:"tls-client-ca,omitempty"`

	PpPrivateKey []byte `json:"sc-pp-prv,omitempty"`
	WnPublicKey  []byte `json:"sc-wn-pub,omitempty"`
}

type Daemon interface {
	Start(ctx context.Context) error
	Shutdown() error
	Ready() chan struct{}
	Addr() string
}

type daemon struct {
	tlsConfig           *tlsutil.TLSConfig
	interceptor         interceptor.Interceptor
	podNode             podnetwork.PodNode
	readyCh             chan struct{}
	stopCh              chan struct{}
	listenAddr          string
	stopOnce            sync.Once
	externalNetViaPodVM bool
}

func NewDaemon(spec *Config, listenAddr string, tlsConfig *tlsutil.TLSConfig, interceptor interceptor.Interceptor, podNode podnetwork.PodNode) Daemon {

	if tlsConfig != nil && !tlsConfig.HasCertAuth() {
		tlsConfig.CertData = []byte(spec.TLSServerCert)
		tlsConfig.KeyData = []byte(spec.TLSServerKey)
	}

	if tlsConfig != nil && !tlsConfig.HasCA() {
		tlsConfig.CAData = []byte(spec.TLSClientCA)
	}

	daemon := &daemon{
		listenAddr:  listenAddr,
		tlsConfig:   tlsConfig,
		interceptor: interceptor,
		podNode:     podNode,
		readyCh:     make(chan struct{}),
		stopCh:      make(chan struct{}),
	}

	if spec.PodNetwork != nil {
		daemon.externalNetViaPodVM = spec.PodNetwork.ExternalNetViaPodVM
	}

	return daemon
}

func (d *daemon) Start(ctx context.Context) error {

	// Set up pod network

	if err := d.podNode.Setup(); err != nil {
		return fmt.Errorf("failed to set up pod network: %w", err)
	}
	defer func() {
		if err := d.podNode.Teardown(); err != nil {
			logger.Printf("failed to tear down pod network: %v", err)
		}
	}()

	// Set up agent protocol interceptor

	var listener net.Listener

	logger.Printf("Starting agent-protocol-forwarder listener on address %v", d.listenAddr)
	if d.tlsConfig != nil {
		logger.Printf("TLS is configured. Configure TLS listener")

		// Create a TLS configuration object
		tlsConfig, err := tlsutil.GetTLSConfigFor(d.tlsConfig)
		if err != nil {
			return fmt.Errorf("Failed to create tls config: %v", err)
		}

		listener, err = tls.Listen("tcp", d.listenAddr, tlsConfig)
		if err != nil {
			logger.Printf("failed to create tls agent-protocol-forwarder listener: %v", err)
			return err
		}
	} else {
		var err error

		listener, err = net.Listen("tcp", d.listenAddr)
		if err != nil {
			logger.Printf("failed to create agent-protocol-forwarder listener: %v", err)
			return err
		}
	}

	d.listenAddr = listener.Addr().String()

	ttrpcServer, err := ttrpc.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create TTRPC server: %w", err)
	}

	pb.RegisterAgentServiceService(ttrpcServer, d.interceptor)
	pb.RegisterHealthService(ttrpcServer, d.interceptor)

	ttrpcServerErr := make(chan error)
	go func() {
		defer close(ttrpcServerErr)

		if err := ttrpcServer.Serve(ctx, listener); err != nil && !errors.Is(err, ttrpc.ErrServerClosed) {
			ttrpcServerErr <- fmt.Errorf("error running TTRPC server for kata agent interceptor: %w", err)
		}
	}()
	defer func() {
		if err := ttrpcServer.Shutdown(ctx); err != nil {
			logger.Printf("error shutting down TTRPC server: %v", err)
		}
		if err := d.interceptor.Close(); err != nil {
			logger.Printf("error shutting down kata agent interceptor: %v", err)
		}
	}()

	close(d.readyCh)

	select {
	case <-ctx.Done():
		return d.Shutdown()
	case <-d.stopCh:
	case err := <-ttrpcServerErr:
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

func (d *daemon) Ready() chan struct{} {
	return d.readyCh
}

func (d *daemon) Addr() string {
	<-d.readyCh
	return d.listenAddr
}
