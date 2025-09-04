// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package agentproto

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/containerd/ttrpc"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

type Redirector interface {
	pb.AgentServiceService
	pb.HealthService

	Connect(ctx context.Context) error
	Close() error
}

type redirector struct {
	agentClient *client
	ttrpcClient *ttrpc.Client
	dialer      func(context.Context) (net.Conn, error)
	once        sync.Once
}

type client struct {
	pb.AgentServiceService
	pb.HealthService
}

func NewRedirector(dialer func(context.Context) (net.Conn, error)) Redirector {

	return &redirector{
		dialer: dialer,
	}
}

func (s *redirector) Connect(ctx context.Context) error {

	var err error

	s.once.Do(func() {

		conn, e := s.dialer(ctx)
		if e != nil {
			err = e
			return
		}

		s.ttrpcClient = ttrpc.NewClient(conn)

		s.agentClient = &client{
			AgentServiceService: pb.NewAgentServiceClient(s.ttrpcClient),
			HealthService:       pb.NewHealthClient(s.ttrpcClient),
		}
	})

	if err != nil {
		return fmt.Errorf("agent connection is not established: %w", err)
	}

	if s.agentClient == nil {
		return errors.New("agent connection is not established")
	}

	return nil
}

func (s *redirector) Close() error {
	client := s.ttrpcClient
	if client == nil {
		return nil
	}
	return client.Close()
}

// AgentServiceService methods

func (s *redirector) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.CreateContainer(ctx, req)
}

func (s *redirector) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.StartContainer(ctx, req)
}

func (s *redirector) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.RemoveContainer(ctx, req)
}

func (s *redirector) ExecProcess(ctx context.Context, req *pb.ExecProcessRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ExecProcess(ctx, req)
}

func (s *redirector) SignalProcess(ctx context.Context, req *pb.SignalProcessRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.SignalProcess(ctx, req)
}

func (s *redirector) WaitProcess(ctx context.Context, req *pb.WaitProcessRequest) (res *pb.WaitProcessResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.WaitProcess(ctx, req)
}

func (s *redirector) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.UpdateContainer(ctx, req)
}

func (s *redirector) UpdateEphemeralMounts(ctx context.Context, req *pb.UpdateEphemeralMountsRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.UpdateEphemeralMounts(ctx, req)
}

func (s *redirector) StatsContainer(ctx context.Context, req *pb.StatsContainerRequest) (res *pb.StatsContainerResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.StatsContainer(ctx, req)
}

func (s *redirector) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.PauseContainer(ctx, req)
}

func (s *redirector) ResumeContainer(ctx context.Context, req *pb.ResumeContainerRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ResumeContainer(ctx, req)
}

func (s *redirector) RemoveStaleVirtiofsShareMounts(ctx context.Context, req *pb.RemoveStaleVirtiofsShareMountsRequest) (res *emptypb.Empty, err error) {
	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.RemoveStaleVirtiofsShareMounts(ctx, req)
}

func (s *redirector) WriteStdin(ctx context.Context, req *pb.WriteStreamRequest) (res *pb.WriteStreamResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.WriteStdin(ctx, req)
}

func (s *redirector) ReadStdout(ctx context.Context, req *pb.ReadStreamRequest) (res *pb.ReadStreamResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ReadStdout(ctx, req)
}

func (s *redirector) ReadStderr(ctx context.Context, req *pb.ReadStreamRequest) (res *pb.ReadStreamResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ReadStderr(ctx, req)
}

func (s *redirector) CloseStdin(ctx context.Context, req *pb.CloseStdinRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.CloseStdin(ctx, req)
}

func (s *redirector) TtyWinResize(ctx context.Context, req *pb.TtyWinResizeRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.TtyWinResize(ctx, req)
}

func (s *redirector) UpdateInterface(ctx context.Context, req *pb.UpdateInterfaceRequest) (res *protocols.Interface, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.UpdateInterface(ctx, req)
}

func (s *redirector) UpdateRoutes(ctx context.Context, req *pb.UpdateRoutesRequest) (res *pb.Routes, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.UpdateRoutes(ctx, req)
}

func (s *redirector) ListInterfaces(ctx context.Context, req *pb.ListInterfacesRequest) (res *pb.Interfaces, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ListInterfaces(ctx, req)
}

func (s *redirector) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (res *pb.Routes, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ListRoutes(ctx, req)
}

func (s *redirector) AddARPNeighbors(ctx context.Context, req *pb.AddARPNeighborsRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.AddARPNeighbors(ctx, req)
}

func (s *redirector) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (res *pb.GetIPTablesResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.GetIPTables(ctx, req)
}

func (s *redirector) SetIPTables(ctx context.Context, req *pb.SetIPTablesRequest) (res *pb.SetIPTablesResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.SetIPTables(ctx, req)
}

func (s *redirector) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (res *pb.Metrics, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.GetMetrics(ctx, req)
}

func (s *redirector) MemAgentMemcgSet(ctx context.Context, req *pb.MemAgentMemcgConfig) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.MemAgentMemcgSet(ctx, req)
}

func (s *redirector) MemAgentCompactSet(ctx context.Context, req *pb.MemAgentCompactConfig) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.MemAgentCompactSet(ctx, req)
}

func (s *redirector) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.CreateSandbox(ctx, req)
}

func (s *redirector) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.DestroySandbox(ctx, req)
}

func (s *redirector) OnlineCPUMem(ctx context.Context, req *pb.OnlineCPUMemRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.OnlineCPUMem(ctx, req)
}

func (s *redirector) ReseedRandomDev(ctx context.Context, req *pb.ReseedRandomDevRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ReseedRandomDev(ctx, req)
}

func (s *redirector) GetGuestDetails(ctx context.Context, req *pb.GuestDetailsRequest) (res *pb.GuestDetailsResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.GetGuestDetails(ctx, req)
}

func (s *redirector) MemHotplugByProbe(ctx context.Context, req *pb.MemHotplugByProbeRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.MemHotplugByProbe(ctx, req)
}

func (s *redirector) SetGuestDateTime(ctx context.Context, req *pb.SetGuestDateTimeRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.SetGuestDateTime(ctx, req)
}

func (s *redirector) CopyFile(ctx context.Context, req *pb.CopyFileRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.CopyFile(ctx, req)
}

func (s *redirector) GetOOMEvent(ctx context.Context, req *pb.GetOOMEventRequest) (res *pb.OOMEvent, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.GetOOMEvent(ctx, req)
}

func (s *redirector) AddSwap(ctx context.Context, req *pb.AddSwapRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.AddSwap(ctx, req)
}

func (s *redirector) AddSwapPath(ctx context.Context, req *pb.AddSwapPathRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.AddSwapPath(ctx, req)
}

func (s *redirector) GetVolumeStats(ctx context.Context, req *pb.VolumeStatsRequest) (res *pb.VolumeStatsResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.GetVolumeStats(ctx, req)
}

func (s *redirector) ResizeVolume(ctx context.Context, req *pb.ResizeVolumeRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.ResizeVolume(ctx, req)
}

func (s *redirector) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (res *emptypb.Empty, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.SetPolicy(ctx, req)
}

// HealthService methods

func (s *redirector) Check(ctx context.Context, req *pb.CheckRequest) (res *pb.HealthCheckResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.Check(ctx, req)
}

func (s *redirector) Version(ctx context.Context, req *pb.CheckRequest) (res *pb.VersionCheckResponse, err error) {

	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s.agentClient.Version(ctx, req)
}
