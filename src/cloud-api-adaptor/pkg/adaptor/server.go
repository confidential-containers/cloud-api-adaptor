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

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/vminfo"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/sshutil"
	pbPodVMInfo "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/proto/podvminfo"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

var logger = log.New(log.Writer(), "[adaptor] ", log.LstdFlags|log.Lmsgprefix)

const (
	DefaultSocketPath = "/run/peerpod/hypervisor.sock"
	DefaultPodsDir    = "/run/peerpod/pods"
)

type Server interface {
	Start(ctx context.Context) error
	Shutdown() error
	Ready() chan struct{}
}

type server struct {
	cloudService            cloud.Service
	vmInfoService           pbPodVMInfo.PodVMInfoService
	workerNode              podnetwork.WorkerNode
	ttRPC                   *ttrpc.Server
	readyCh                 chan struct{}
	stopCh                  chan struct{}
	socketPath              string
	stopOnce                sync.Once
	enableCloudConfigVerify bool
	PeerPodsLimitPerNode    int
}

func NewServer(provider provider.Provider, cfg *cloud.ServerConfig, workerNode podnetwork.WorkerNode) Server {

	logger.Printf("server config: %#v", cfg)

	agentFactory := proxy.NewFactory(cfg.PauseImage, cfg.TLSConfig, cfg.ProxyTimeout)
	cloudService := cloud.NewService(provider, agentFactory, workerNode, cfg, sshutil.SSHPORT)
	vmInfoService := vminfo.NewService(cloudService)

	return &server{
		socketPath:              cfg.SocketPath,
		cloudService:            cloudService,
		vmInfoService:           vmInfoService,
		workerNode:              workerNode,
		readyCh:                 make(chan struct{}),
		stopCh:                  make(chan struct{}),
		enableCloudConfigVerify: cfg.EnableCloudConfigVerify,
		PeerPodsLimitPerNode:    cfg.PeerPodsLimitPerNode,
	}
}

func (s *server) Start(ctx context.Context) (err error) {
	if s.enableCloudConfigVerify {
		verifierErr := s.cloudService.ConfigVerifier()
		if verifierErr != nil {
			return err
		}
	}
	// Advertise node resources
	if k8sops.IsKubernetesEnvironment() {
		err = k8sops.AdvertiseExtendedResources(s.PeerPodsLimitPerNode)
		if err != nil {
			return err
		}
	}

	ttRPC, err := ttrpc.NewServer()
	if err != nil {
		return err
	}
	s.ttRPC = ttRPC
	if err := os.MkdirAll(filepath.Dir(s.socketPath), os.ModePerm); err != nil {
		return err
	}
	if err := os.RemoveAll(s.socketPath); err != nil { // just in case socket wasn't cleaned
		return err
	}
	pbHypervisor.RegisterHypervisorService(s.ttRPC, s.cloudService)
	pbPodVMInfo.RegisterPodVMInfoService(s.ttRPC, s.vmInfoService)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}

	ttRPCErr := make(chan error)
	go func() {
		defer close(ttRPCErr)
		if err := s.ttRPC.Serve(ctx, listener); err != nil {
			ttRPCErr <- err
		}
	}()
	defer func() {
		ttRPCShutdownErr := s.ttRPC.Shutdown(context.Background())
		if ttRPCShutdownErr != nil && err == nil {
			err = ttRPCShutdownErr
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
	case err = <-ttRPCErr:
	}
	return err
}

func (s *server) Shutdown() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})

	_ = k8sops.RemoveExtendedResources()

	return s.cloudService.Teardown()
}

func (s *server) Ready() chan struct{} {
	return s.readyCh
}
