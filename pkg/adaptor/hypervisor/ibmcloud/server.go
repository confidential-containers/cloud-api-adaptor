// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0
//go:build ibmcloud

package ibmcloud

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"

	"github.com/containerd/ttrpc"
	pbHypervisor "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	pbPodVMInfo "github.com/confidential-containers/cloud-api-adaptor/proto/podvminfo"
)

var logger = log.New(log.Writer(), "[helper/hypervisor] ", log.LstdFlags|log.Lmsgprefix)

type server struct {
	socketPath string

	ttRpc         *ttrpc.Server
	service       pbHypervisor.HypervisorService
	vmInfoService pbPodVMInfo.PodVMInfoService

	workerNode podnetwork.WorkerNode

	readyCh  chan struct{}
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewServer(cfg hypervisor.Config, cloudCfg Config, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {

	logger.Printf("hypervisor config %v", cfg)
	logger.Printf("cloud config %v", cloudCfg.Redact())

	var vpcV1 VpcV1
	if cloudCfg.ApiKey != "" {
		//FIXME: Null ApiKey is used in unit tests
		var err error
		vpcV1, err = NewVpcV1(cloudCfg.ApiKey, cloudCfg.IamServiceURL, cloudCfg.VpcServiceURL)
		if err != nil {
			panic(err)
		}
	}

	s := &server{
		socketPath: cfg.SocketPath,
		service:    newService(vpcV1, &cloudCfg, &cfg, workerNode, cfg.PodsDir, daemonPort),
		workerNode: workerNode,
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}

	s.vmInfoService = newPodVMInfoService(s.service.(*hypervisorService)) // TODO: refactor not to use type casting

	return s
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
	pbHypervisor.RegisterHypervisorService(s.ttRpc, s.service)
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
	return nil
}

func (s *server) Ready() chan struct{} {
	return s.readyCh
}
