// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"log"
	"net"

	"github.com/gogo/protobuf/types"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/agentproto"
)

var logger = log.New(log.Writer(), "[forwarder/interceptor] ", log.LstdFlags|log.Lmsgprefix)

type Interceptor interface {
	agentproto.Redirector
}

type interceptor struct {
	agentproto.Redirector

	nsPath string
}

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
	}

	redirector := agentproto.NewRedirector(agentDialer)

	return &interceptor{
		Redirector: redirector,
		nsPath:     nsPath,
	}
}

func (i *interceptor) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*types.Empty, error) {

	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)

	// Specify the network namespace path in the container spec
	req.OCI.Linux.Namespaces = append(req.OCI.Linux.Namespaces, pb.LinuxNamespace{
		Type: string(specs.NetworkNamespace),
		Path: i.nsPath,
	})

	logger.Printf("    namespaces:")
	for _, ns := range req.OCI.Linux.Namespaces {
		logger.Printf("    %s: %q", ns.Type, ns.Path)
	}

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

	if len(req.Dns) > 0 {
		logger.Print("    dns:")
		for _, d := range req.Dns {
			logger.Printf("        %s", d)
		}

		logger.Print("      Eliminated the DNS setting above from CreateSandboxRequest to stop updating /etc/resolv.conf on the peer pod VM")
		logger.Print("      See https://github.com/confidential-containers/cloud-api-adaptor/issues/98 for the details.")
		logger.Println()
		req.Dns = nil
	}

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
