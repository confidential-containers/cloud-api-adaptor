// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

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

func dial(ctx context.Context, agentSocket string) (net.Conn, error) {

	var conn net.Conn

	maxRetries := 30
	retryInterval := 5 * time.Second

	count := 1
	for {
		var err error

		func() {
			ctx, cancel := context.WithTimeout(ctx, retryInterval)
			defer cancel()

			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)

			if err == nil || count == maxRetries {
				return
			}
			<-ctx.Done()
		}()

		if err == nil {
			break
		}
		if count == maxRetries {
			err := fmt.Errorf("reaches max retry count. gave up establishing agent connection to %s: %w", agentSocket, err)
			logger.Print(err)
			return nil, err
		}
		logger.Printf("failed to establish agent connection to %s: %v. (retrying... %d/%d)", agentSocket, err, count, maxRetries)

		count++
	}

	logger.Printf("established agent connection to %s", agentSocket)

	return conn, nil
}

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {
		return dial(ctx, agentSocket)
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

	if len(req.OCI.Mounts) > 0 {
		for _, m := range req.OCI.Mounts {
			if _, err := os.Stat(m.Source); os.IsNotExist(err) && m.Type == "bind" {
				logger.Printf("mount source %s doesn't exist, try to create", m.Source)
				if err = os.MkdirAll(m.Source, os.ModePerm); err != nil {
					logger.Printf("Failed to create dir: %v", err)
				}
			}
		}
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

func (i *interceptor) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {

	logger.Printf("PullImage: image: %q, containerID: %q", req.Image, req.ContainerId)

	res, err := i.Redirector.PullImage(ctx, req)

	if err != nil {
		logger.Printf("PullImage failed with error: %v", err)
	}

	return res, err
}
