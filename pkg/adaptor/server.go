// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package adaptor

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/ttrpc"
	pbHypervisor "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/vminfo"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/tlsutil"
	pbPodVMInfo "github.com/confidential-containers/cloud-api-adaptor/proto/podvminfo"
)

var logger = log.New(log.Writer(), "[adaptor] ", log.LstdFlags|log.Lmsgprefix)

const (
	DefaultSocketPath = "/run/peerpod/hypervisor.sock"
	DefaultPodsDir    = "/run/peerpod/pods"
)

type ServerConfig struct {
	TLSConfig     tlsutil.TLSConfig
	SocketPath    string
	CriSocketPath string
	PauseImage    string
	PodsDir       string
	ForwarderPort string
}

type Server interface {
	Start(ctx context.Context) error
	Shutdown() error
	Ready() chan struct{}
}

type server struct {
	cloudService  cloud.Service
	vmInfoService pbPodVMInfo.PodVMInfoService
	workerNode    podnetwork.WorkerNode
	ttRpc         *ttrpc.Server
	readyCh       chan struct{}
	stopCh        chan struct{}
	socketPath    string
	stopOnce      sync.Once
}

func NewServer(provider cloud.Provider, cfg *ServerConfig, workerNode podnetwork.WorkerNode) Server {

	logger.Printf("server config: %#v", cfg)

	agentFactory := proxy.NewFactory(cfg.PauseImage, cfg.CriSocketPath, &cfg.TLSConfig)
	cloudService := cloud.NewService(provider, agentFactory, workerNode, cfg.PodsDir, cfg.ForwarderPort)
	vmInfoService := vminfo.NewService(cloudService)

	return &server{
		socketPath:    cfg.SocketPath,
		cloudService:  cloudService,
		vmInfoService: vmInfoService,
		workerNode:    workerNode,
		readyCh:       make(chan struct{}),
		stopCh:        make(chan struct{}),
	}
}

func (s *server) Start(ctx context.Context) (err error) {

	ttRpc, err := ttrpc.NewServer()
	if err != nil {
		return err
	}
	s.ttRpc = ttRpc
	if err := os.MkdirAll(filepath.Dir(s.socketPath), os.ModePerm); err != nil {
		return err
	}
	if err := os.RemoveAll(s.socketPath); err != nil { // just in case socket wasn't cleaned
		return err
	}
	pbHypervisor.RegisterHypervisorService(s.ttRpc, s.cloudService)
	pbPodVMInfo.RegisterPodVMInfoService(s.ttRpc, s.vmInfoService)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}

	ttRpcErr := make(chan error)
	go func() {
		defer close(ttRpcErr)
		if err := s.ttRpc.Serve(ctx, listener); err != nil {
			ttRpcErr <- err
		}
	}()
	defer func() {
		ttRpcShutdownErr := s.ttRpc.Shutdown(context.Background())
		if ttRpcShutdownErr != nil && err == nil {
			err = ttRpcShutdownErr
		}
	}()

	close(s.readyCh)

	logger.Printf("server started")

	select {
	case <-ctx.Done():
		shutdownErr := s.Shutdown()
		if shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	case <-s.stopCh:
	case err = <-ttRpcErr:
	}
	return err
}

func (s *server) Shutdown() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})

	return s.cloudService.Teardown()
}

func (s *server) Ready() chan struct{} {
	return s.readyCh
}
