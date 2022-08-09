// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/gogo/protobuf/types"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/agentproto"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/netops"
)

var logger = log.New(log.Writer(), "[forwarder/interceptor] ", log.LstdFlags|log.Lmsgprefix)

type Interceptor interface {
	agentproto.Redirector
}

type interceptor struct {
	agentproto.Redirector
}

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {

		if nsPath == "" {
			return (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
		}

		ns, err := netops.NewNSFromPath(nsPath)
		if err != nil {
			err = fmt.Errorf("failed to open network namespace %q: %w", nsPath, err)
			logger.Print(err)
			return nil, err
		}

		var conn net.Conn
		if err := ns.Run(func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to call dialer at namespace %q: %w", nsPath, err)
		}

		return conn, nil
	}

	redirector := agentproto.NewRedirector(agentDialer)

	return &interceptor{
		Redirector: redirector,
	}
}

func (i *interceptor) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*types.Empty, error) {

	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.CreateContainer(ctx, req)

	if err != nil {
		logger.Printf("CreateContainer failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*types.Empty, error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.StartContainer(ctx, req)

	if err != nil {
		logger.Printf("StartContainer failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*types.Empty, error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.RemoveContainer(ctx, req)

	if err != nil {
		logger.Printf("RemoveContainer failed with error: %v", err)
	}
	return res, err
}

func (i *interceptor) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*types.Empty, error) {

	logger.Printf("CreateSandbox: hostname:%s sandboxId:%s", req.Hostname, req.SandboxId)

	res, err := i.Redirector.CreateSandbox(ctx, req)

	if err != nil {
		logger.Printf("CreateSandbox failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*types.Empty, error) {

	logger.Printf("DestroySandbox")

	res, err := i.Redirector.DestroySandbox(ctx, req)

	if err != nil {
		logger.Printf("DestroySandbox failed with error: %v", err)
	}

	return res, err
}
