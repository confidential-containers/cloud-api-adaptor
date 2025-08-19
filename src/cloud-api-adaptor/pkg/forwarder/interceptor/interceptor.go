// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	retry "github.com/avast/retry-go/v4"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/moby/sys/mountinfo"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
)

const (
	volumeTargetPathKey = "io.confidentialcontainers.org.peerpodvolumes.target_path"
	volumeCheckInterval = 5 * time.Second
	volumeCheckTimeout  = 3 * time.Minute
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

	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	logger.Printf("Trying to establish agent connection to %s", agentSocket)
	err := retry.Do(
		func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
			return err
		},
		retry.Context(ctx),
	)

	if err != nil {
		err = fmt.Errorf("failed to establish agent connection to %s: %w", agentSocket, err)
		logger.Print(err)
		return nil, err
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

func (i *interceptor) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)

	// Specify the network namespace path in the container spec
	req.OCI.Linux.Namespaces = append(req.OCI.Linux.Namespaces, &pb.LinuxNamespace{
		Type: string(specs.NetworkNamespace),
		Path: i.nsPath,
	})

	logger.Printf("    namespaces:")
	for _, ns := range req.OCI.Linux.Namespaces {
		logger.Printf("    %s: %q", ns.Type, ns.Path)
	}

	volumeTargetPath := req.OCI.Annotations[volumeTargetPathKey]
	volumeTargetPathSlice := strings.Split(volumeTargetPath, ",")
	if len(req.OCI.Mounts) > 0 {
		for _, m := range req.OCI.Mounts {
			if _, err := os.Stat(m.Source); os.IsNotExist(err) && m.Type == "bind" {
				logger.Printf("mount source %s doesn't exist, try to create", m.Source)
				if err = os.MkdirAll(m.Source, os.ModePerm); err != nil {
					logger.Printf("Failed to create dir: %v", err)
				}
			}
			for _, s := range volumeTargetPathSlice {
				if isTargetPath(m.Source, strings.TrimSpace(s)) {
					logger.Printf("Waiting for device mounted to: %s", m.Source)
					err := waitForDeviceMounted(ctx, m.Source)
					if err != nil {
						return nil, err
					}
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

func isTargetPath(path, targetPath string) bool {
	return targetPath != "" && targetPath == path
}

func waitForDeviceMounted(ctx context.Context, path string) error {

	ctx, cancel := context.WithTimeout(ctx, volumeCheckTimeout)
	defer cancel()

	err := retry.Do(
		func() error {
			isMounted, err := mountinfo.Mounted(path)
			if err != nil {
				logger.Printf("Mounted check error: %v", err)
				return err
			}

			if isMounted {
				logger.Printf("Device has been mounted to %s", path)
				return nil
			} else {
				err = fmt.Errorf("device has not been mounted to %s", path)
				logger.Print(err)
				return err
			}
		},
		retry.Attempts(0),
		retry.Context(ctx),
		retry.MaxDelay(volumeCheckInterval),
	)

	if err != nil {
		err = fmt.Errorf("timeout waiting for device to mount to %s: %w", path, err)
		logger.Print(err)
		return err
	}

	return nil

}

func (i *interceptor) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.StartContainer(ctx, req)

	if err != nil {
		logger.Printf("StartContainer failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.RemoveContainer(ctx, req)

	if err != nil {
		logger.Printf("RemoveContainer failed with error: %v", err)
	}
	return res, err
}

func (i *interceptor) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {

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

func (i *interceptor) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {

	logger.Printf("DestroySandbox")

	res, err := i.Redirector.DestroySandbox(ctx, req)

	if err != nil {
		logger.Printf("DestroySandbox failed with error: %v", err)
	}

	return res, err
}
