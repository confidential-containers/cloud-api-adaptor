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

	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adapter/pkg/adaptor/hypervisor"
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

type Config struct {
        socketPath        string
        podsDir           string
        helperDaemonRoot  string
        httpTunnelTimeout string
        apiKey            string
        TunnelType        string
        HostInterface     string
        hypProvider       string
        serviceConfig     hypervisor.ServiceConfig
}

func NewServer(cfg Config, workerNode podnetwork.WorkerNode, daemonPort string) hypervisor.Server {

     vpcV1, err := NewVpcV1(cfg.apiKey)
     if err != nil {
          return nil
     }

    return &server{
		socketPath: cfg.socketPath,
		service:    newService(vpcV1, &cfg.serviceConfig, workerNode, cfg.podsDir, daemonPort),
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
