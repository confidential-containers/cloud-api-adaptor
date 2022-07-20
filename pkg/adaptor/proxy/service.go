// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containerd/ttrpc"
	"github.com/gogo/protobuf/types"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

const (
	defaultMaxRetries    = 20
	defaultRetryInterval = 10 * time.Second
)

type client struct {
	pb.AgentServiceService
	pb.ImageService
	pb.HealthService
}

func newClient(conn net.Conn) *client {

	ttrpcClient := ttrpc.NewClient(conn)

	return &client{
		AgentServiceService: pb.NewAgentServiceClient(ttrpcClient),
		ImageService:        pb.NewImageClient(ttrpcClient),
		HealthService:       pb.NewHealthClient(ttrpcClient),
	}
}

type proxyService struct {
	agentClient   *client
	maxRetries    int
	retryInterval time.Duration
}

func newProxyService() *proxyService {
	return &proxyService{
		maxRetries:    defaultMaxRetries,
		retryInterval: defaultRetryInterval,
	}
}

func (s *proxyService) connect(ctx context.Context, address string) error {

	var conn net.Conn

	maxRetries := s.maxRetries
	count := 1
	for {
		var err error

		func() {
			ctx, cancel := context.WithTimeout(ctx, s.retryInterval)
			defer cancel()

			// TODO: Support TLS
			conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)

			if err == nil || count == maxRetries {
				return
			}
			<-ctx.Done()
		}()

		if err == nil {
			break
		}
		if count == maxRetries {
			err := fmt.Errorf("reaches max retry count. gave up establishing agent proxy connection to %s: %w", address, err)
			logger.Print(err)
			return err
		}
		logger.Printf("failed to establish agent proxy connection to %s: %v. (retrying... %d/%d)", address, err, count, maxRetries)

		count++
	}

	s.agentClient = newClient(conn)

	logger.Printf("established agent proxy connection to %s", address)

	return nil
}

func (s *proxyService) redirect(ctx context.Context, fn func(c *client)) {

	fn(s.agentClient)
}

// AgentServiceService methods

func (s *proxyService) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (res *types.Empty, err error) {

	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)
	if len(req.OCI.Annotations) > 0 {
		logger.Print("    annotations:")
		for k, v := range req.OCI.Annotations {
			logger.Printf("        %s: %s", k, v)
		}
	}
	if len(req.OCI.Mounts) > 0 {
		logger.Print("    mounts:")
		for _, m := range req.OCI.Mounts {
			logger.Printf("        destination:%s source:%s type:%s", m.Destination, m.Source, m.Type)
		}
	}
	if len(req.Storages) > 0 {
		logger.Print("    storages:")
		for _, s := range req.Storages {
			logger.Printf("        mount_point:%s source:%s fstype:%s driver:%s", s.MountPoint, s.Source, s.Fstype, s.Driver)
		}
	}
	if len(req.Devices) > 0 {
		logger.Print("    devices:")
		for _, d := range req.Devices {
			logger.Printf("        container_path:%s vm_path:%s type:%s", d.ContainerPath, d.VmPath, d.Type)
		}
	}

	s.redirect(ctx, func(c *client) {
		res, err = c.CreateContainer(ctx, req)
	})

	if err != nil {
		logger.Printf("CreateContainer fails: %v", err)
	}
	return
}

func (s *proxyService) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (res *types.Empty, err error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	s.redirect(ctx, func(c *client) {
		res, err = c.StartContainer(ctx, req)
	})

	if err != nil {
		logger.Printf("StartContainer fails: %v", err)
	}
	return
}

func (s *proxyService) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (res *types.Empty, err error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	s.redirect(ctx, func(c *client) {
		res, err = c.RemoveContainer(ctx, req)
	})

	if err != nil {
		logger.Printf("RemoveContainer fails: %v", err)
	}
	return
}

func (s *proxyService) ExecProcess(ctx context.Context, req *pb.ExecProcessRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ExecProcess(ctx, req)
	})
	return
}

func (s *proxyService) SignalProcess(ctx context.Context, req *pb.SignalProcessRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.SignalProcess(ctx, req)
	})
	return
}

func (s *proxyService) WaitProcess(ctx context.Context, req *pb.WaitProcessRequest) (res *pb.WaitProcessResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.WaitProcess(ctx, req)
	})
	return
}

func (s *proxyService) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.UpdateContainer(ctx, req)
	})
	return
}

func (s *proxyService) StatsContainer(ctx context.Context, req *pb.StatsContainerRequest) (res *pb.StatsContainerResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.StatsContainer(ctx, req)
	})
	return
}

func (s *proxyService) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.PauseContainer(ctx, req)
	})
	return
}

func (s *proxyService) ResumeContainer(ctx context.Context, req *pb.ResumeContainerRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ResumeContainer(ctx, req)
	})
	return
}

func (s *proxyService) WriteStdin(ctx context.Context, req *pb.WriteStreamRequest) (res *pb.WriteStreamResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.WriteStdin(ctx, req)
	})
	return
}

func (s *proxyService) ReadStdout(ctx context.Context, req *pb.ReadStreamRequest) (res *pb.ReadStreamResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ReadStdout(ctx, req)
	})
	return
}

func (s *proxyService) ReadStderr(ctx context.Context, req *pb.ReadStreamRequest) (res *pb.ReadStreamResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ReadStderr(ctx, req)
	})
	return
}

func (s *proxyService) CloseStdin(ctx context.Context, req *pb.CloseStdinRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.CloseStdin(ctx, req)
	})
	return
}

func (s *proxyService) TtyWinResize(ctx context.Context, req *pb.TtyWinResizeRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.TtyWinResize(ctx, req)
	})
	return
}

func (s *proxyService) UpdateInterface(ctx context.Context, req *pb.UpdateInterfaceRequest) (res *protocols.Interface, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.UpdateInterface(ctx, req)
	})
	return
}

func (s *proxyService) UpdateRoutes(ctx context.Context, req *pb.UpdateRoutesRequest) (res *pb.Routes, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.UpdateRoutes(ctx, req)
	})
	return
}

func (s *proxyService) ListInterfaces(ctx context.Context, req *pb.ListInterfacesRequest) (res *pb.Interfaces, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ListInterfaces(ctx, req)
	})
	return
}

func (s *proxyService) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (res *pb.Routes, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ListRoutes(ctx, req)
	})
	return
}

func (s *proxyService) AddARPNeighbors(ctx context.Context, req *pb.AddARPNeighborsRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.AddARPNeighbors(ctx, req)
	})
	return
}

func (s *proxyService) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (res *pb.GetIPTablesResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.GetIPTables(ctx, req)
	})
	return
}

func (s *proxyService) SetIPTables(ctx context.Context, req *pb.SetIPTablesRequest) (res *pb.SetIPTablesResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.SetIPTables(ctx, req)
	})
	return
}

func (s *proxyService) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (res *pb.Metrics, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.GetMetrics(ctx, req)
	})
	return
}

func (s *proxyService) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (res *types.Empty, err error) {

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
	if len(req.Storages) > 0 {
		logger.Print("    storages:")
		for _, s := range req.Storages {
			logger.Printf("        mountpoint:%s source:%s fstype:%s driver:%s", s.MountPoint, s.Source, s.Fstype, s.Driver)
		}
	}

	s.redirect(ctx, func(c *client) {
		res, err = c.CreateSandbox(ctx, req)
	})

	if err != nil {
		logger.Printf("CreateSandbox fails: %v", err)
	}

	return
}

func (s *proxyService) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (res *types.Empty, err error) {

	logger.Printf("DestroySandbox")

	s.redirect(ctx, func(c *client) {
		res, err = c.DestroySandbox(ctx, req)
	})

	if err != nil {
		logger.Printf("DestroySandbox fails: %v", err)
	}

	return
}

func (s *proxyService) OnlineCPUMem(ctx context.Context, req *pb.OnlineCPUMemRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.OnlineCPUMem(ctx, req)
	})
	return
}

func (s *proxyService) ReseedRandomDev(ctx context.Context, req *pb.ReseedRandomDevRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ReseedRandomDev(ctx, req)
	})
	return
}

func (s *proxyService) GetGuestDetails(ctx context.Context, req *pb.GuestDetailsRequest) (res *pb.GuestDetailsResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.GetGuestDetails(ctx, req)
	})
	return
}

func (s *proxyService) MemHotplugByProbe(ctx context.Context, req *pb.MemHotplugByProbeRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.MemHotplugByProbe(ctx, req)
	})
	return
}

func (s *proxyService) SetGuestDateTime(ctx context.Context, req *pb.SetGuestDateTimeRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.SetGuestDateTime(ctx, req)
	})
	return
}

func (s *proxyService) CopyFile(ctx context.Context, req *pb.CopyFileRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.CopyFile(ctx, req)
	})
	return
}

func (s *proxyService) GetOOMEvent(ctx context.Context, req *pb.GetOOMEventRequest) (res *pb.OOMEvent, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.GetOOMEvent(ctx, req)
	})
	return
}

func (s *proxyService) AddSwap(ctx context.Context, req *pb.AddSwapRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.AddSwap(ctx, req)
	})
	return
}

func (s *proxyService) GetVolumeStats(ctx context.Context, req *pb.VolumeStatsRequest) (res *pb.VolumeStatsResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.GetVolumeStats(ctx, req)
	})
	return
}

func (s *proxyService) ResizeVolume(ctx context.Context, req *pb.ResizeVolumeRequest) (res *types.Empty, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.ResizeVolume(ctx, req)
	})
	return
}

// ImageService method

func (s *proxyService) PullImage(ctx context.Context, req *pb.PullImageRequest) (res *pb.PullImageResponse, err error) {

	logger.Printf("PullImage: image:%s containerID:%s", req.Image, req.ContainerId)

	s.redirect(ctx, func(c *client) {
		res, err = c.PullImage(ctx, req)
	})

	if err != nil {
		logger.Printf("PullImage fails: %v", err)
	}
	return
}

// HealthService methods

func (s *proxyService) Check(ctx context.Context, req *pb.CheckRequest) (res *pb.HealthCheckResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.Check(ctx, req)
	})
	return
}

func (s *proxyService) Version(ctx context.Context, req *pb.CheckRequest) (res *pb.VersionCheckResponse, err error) {

	s.redirect(ctx, func(c *client) {
		res, err = c.Version(ctx, req)
	})
	return
}
