// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/containerd/ttrpc"

	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

var logger = log.New(log.Writer(), "[helper/hypervisor] ", log.LstdFlags|log.Lmsgprefix)


type server struct {
	socketPath string

	ttRpc   *ttrpc.Server
	service pb.HypervisorService

	workerNode podnetwork.WorkerNode

	readyCh  chan struct{}
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewServer(cfg hypervisor.Config, cloudCfg Config, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {

     logger.Printf("hypervisor config %v", cfg)
     logger.Printf("cloud config %v", cloudCfg)
     vpcV1, err := NewVpcV1(cloudCfg.ApiKey)
     if err != nil {
          return nil
     }

    return &server{
		socketPath: cfg.SocketPath,
		service:    newService(vpcV1, &cloudCfg, workerNode, cfg.PodsDir, daemonPort),
		workerNode: workerNode,
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

func (s *server) Start(ctx context.Context) error {

	ttRpc, err := ttrpc.NewServer()
	if err != nil {
		return err
	}
	s.ttRpc = ttRpc
	if err := os.MkdirAll(filepath.Dir(s.socketPath), os.ModePerm); err != nil {
		return err
	}
	pb.RegisterHypervisorService(s.ttRpc, s.service)
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
	defer s.ttRpc.Shutdown(context.Background())

	close(s.readyCh)

	select {
	case <-ctx.Done():
		s.Shutdown()
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
