// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/tlsutil"
	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	criapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

const (
	SocketName          = "agent.ttrpc"
	defaultCriTimeout   = 1 * time.Second
	DefaultProxyTimeout = 5 * time.Minute

	// The server TLS certificate must have this as SAN
	// TODO: Avoid hard coding of server name
	podvmServername = "podvm-server"
)

var logger = log.New(log.Writer(), "[adaptor/proxy] ", log.LstdFlags|log.Lmsgprefix)

type criClient struct {
	criapi.ImageServiceClient
}

type AgentProxy interface {
	Start(ctx context.Context, serverURL *url.URL) error
	Ready() chan struct{}
	Shutdown() error
	CAService() tlsutil.CAService
	ClientCA() (certPEM []byte)
}

type agentProxy struct {
	tlsConfig     *tlsutil.TLSConfig
	caService     tlsutil.CAService
	readyCh       chan struct{}
	stopCh        chan struct{}
	serverName    string
	socketPath    string
	criSocketPath string
	pauseImage    string
	proxyTimeout  time.Duration
	criTimeout    time.Duration
	stopOnce      sync.Once
}

func NewAgentProxy(serverName, socketPath, criSocketPath string, pauseImage string, tlsConfig *tlsutil.TLSConfig, caService tlsutil.CAService, proxyTimeout time.Duration) AgentProxy {

	return &agentProxy{
		serverName:    serverName,
		socketPath:    socketPath,
		criSocketPath: criSocketPath,
		readyCh:       make(chan struct{}),
		stopCh:        make(chan struct{}),
		proxyTimeout:  proxyTimeout,
		criTimeout:    defaultCriTimeout,
		pauseImage:    pauseImage,
		tlsConfig:     tlsConfig,
		caService:     caService,
	}
}

func (p *agentProxy) dial(ctx context.Context, address string) (net.Conn, error) {

	var conn net.Conn

	var dialer interface {
		DialContext(ctx context.Context, network, address string) (net.Conn, error)
	}

	if p.tlsConfig != nil {

		// Create a TLS configuration object
		config, err := tlsutil.GetTLSConfigFor(p.tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("Failed to create tls config: %v", err)
		}
		// This is important otherwise you'll hit the following error
		// cannot validate certificate for <IP> because it doesn't contain any IP SAN
		// Since it's not possible to know the IP address of the pod VM apriori,
		// we are using a well-defined hostname here. Other option is to create
		// certificates with IP SAN having all the IPs in the network range
		// When CA service is enabled, a server certificate is automatically generated for
		// the instance VM name.
		if p.caService != nil {
			config.ServerName = p.serverName
		} else {
			config.ServerName = podvmServername
		}

		dialer = &tls.Dialer{
			Config: config,
		}
	} else {
		dialer = &net.Dialer{}
	}

	ctx, cancel := context.WithTimeout(ctx, p.proxyTimeout)
	defer cancel()

	logger.Printf("Trying to establish agent proxy connection to %s", address)
	err := retry.Do(
		func() error {
			var err error
			conn, err = dialer.DialContext(ctx, "tcp", address)
			return err
		},
		retry.Attempts(0),
		retry.Context(ctx),
	)

	if err != nil {
		err = fmt.Errorf("failed to establish agent proxy connection to %s: %w", address, err)
		logger.Print(err)
		return nil, err
	}

	logger.Printf("established agent proxy connection to %s", address)
	return conn, nil
}

func (p *agentProxy) initCriClient(ctx context.Context) (*criClient, error) {
	if p.criSocketPath != "" {
		timeout, cancel := context.WithTimeout(ctx, p.criTimeout)
		defer cancel()
		conn, err := grpc.DialContext(timeout, p.criSocketPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", target)
			}),
			grpc.FailOnNonTempDialError(true),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to established cri uds connection to %s: %v", p.criSocketPath, err)
		}

		criClient := &criClient{
			ImageServiceClient: criapi.NewImageServiceClient(conn),
		}
		logger.Printf("established cri uds connection to %s", p.criSocketPath)
		return criClient, err
	}

	return nil, fmt.Errorf("cri runtime endpoint is not specified, it is used to get the image name from image digest")
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

	dialer := func(ctx context.Context) (net.Conn, error) {
		return p.dial(ctx, serverURL.Host)
	}

	criClient, err := p.initCriClient(ctx)
	if err != nil {
		// cri client is optional currently, we ignore any errors here
		logger.Printf("failed to init cri client, the err: %v", err)
	}

	proxyService := newProxyService(dialer, criClient, p.pauseImage)
	defer func() {
		if err := proxyService.Close(); err != nil {
			logger.Printf("error closing agent proxy connection: %v", err)
		}
	}()

	if err := proxyService.Connect(ctx); err != nil {
		return fmt.Errorf("error connecting to agent: %v", err)
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

	close(p.readyCh)

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

func (p *agentProxy) CAService() tlsutil.CAService {
	return p.caService
}

func (p *agentProxy) ClientCA() (certPEM []byte) {

	if p.tlsConfig == nil {
		return nil
	}
	if p.tlsConfig.CAFile != "" {
		// When a client CA file is explicitly specified, we don't need to put it in cloud-init data
		return nil
	}

	return p.tlsConfig.CertData
}
