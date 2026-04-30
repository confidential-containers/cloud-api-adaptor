// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/containerd/ttrpc"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockAgentService is a shared mock implementation of the agent service
// for testing purposes. It implements both AgentServiceService and HealthService interfaces.
type mockAgentService struct{}

func (m *mockAgentService) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) ExecProcess(ctx context.Context, req *pb.ExecProcessRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) SignalProcess(ctx context.Context, req *pb.SignalProcessRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) WaitProcess(ctx context.Context, req *pb.WaitProcessRequest) (*pb.WaitProcessResponse, error) {
	return &pb.WaitProcessResponse{}, nil
}

func (m *mockAgentService) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) UpdateEphemeralMounts(ctx context.Context, req *pb.UpdateEphemeralMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) StatsContainer(ctx context.Context, req *pb.StatsContainerRequest) (*pb.StatsContainerResponse, error) {
	return &pb.StatsContainerResponse{}, nil
}

func (m *mockAgentService) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) ResumeContainer(ctx context.Context, req *pb.ResumeContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) WriteStdin(ctx context.Context, req *pb.WriteStreamRequest) (*pb.WriteStreamResponse, error) {
	return &pb.WriteStreamResponse{}, nil
}

func (m *mockAgentService) ReadStdout(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{}, nil
}

func (m *mockAgentService) ReadStderr(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{}, nil
}

func (m *mockAgentService) CloseStdin(ctx context.Context, req *pb.CloseStdinRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) TtyWinResize(ctx context.Context, req *pb.TtyWinResizeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) UpdateInterface(ctx context.Context, req *pb.UpdateInterfaceRequest) (*protocols.Interface, error) {
	return &protocols.Interface{}, nil
}

func (m *mockAgentService) UpdateRoutes(ctx context.Context, req *pb.UpdateRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}

func (m *mockAgentService) ListInterfaces(ctx context.Context, req *pb.ListInterfacesRequest) (*pb.Interfaces, error) {
	return &pb.Interfaces{}, nil
}

func (m *mockAgentService) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}

func (m *mockAgentService) AddARPNeighbors(ctx context.Context, req *pb.AddARPNeighborsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (*pb.GetIPTablesResponse, error) {
	return &pb.GetIPTablesResponse{}, nil
}

func (m *mockAgentService) SetIPTables(ctx context.Context, req *pb.SetIPTablesRequest) (*pb.SetIPTablesResponse, error) {
	return &pb.SetIPTablesResponse{}, nil
}

func (m *mockAgentService) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.Metrics, error) {
	return &pb.Metrics{}, nil
}

func (m *mockAgentService) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) OnlineCPUMem(ctx context.Context, req *pb.OnlineCPUMemRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) ReseedRandomDev(ctx context.Context, req *pb.ReseedRandomDevRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) GetGuestDetails(ctx context.Context, req *pb.GuestDetailsRequest) (*pb.GuestDetailsResponse, error) {
	return &pb.GuestDetailsResponse{}, nil
}

func (m *mockAgentService) MemHotplugByProbe(ctx context.Context, req *pb.MemHotplugByProbeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) SetGuestDateTime(ctx context.Context, req *pb.SetGuestDateTimeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) CopyFile(ctx context.Context, req *pb.CopyFileRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) GetOOMEvent(ctx context.Context, req *pb.GetOOMEventRequest) (*pb.OOMEvent, error) {
	return &pb.OOMEvent{}, nil
}

func (m *mockAgentService) AddSwap(ctx context.Context, req *pb.AddSwapRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) GetVolumeStats(ctx context.Context, req *pb.VolumeStatsRequest) (*pb.VolumeStatsResponse, error) {
	return &pb.VolumeStatsResponse{}, nil
}

func (m *mockAgentService) ResizeVolume(ctx context.Context, req *pb.ResizeVolumeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) RemoveStaleVirtiofsShareMounts(ctx context.Context, req *pb.RemoveStaleVirtiofsShareMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) Check(ctx context.Context, req *pb.CheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{}, nil
}

func (m *mockAgentService) Version(ctx context.Context, req *pb.CheckRequest) (*pb.VersionCheckResponse, error) {
	return &pb.VersionCheckResponse{}, nil
}

func (m *mockAgentService) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) AddSwapPath(ctx context.Context, req *pb.AddSwapPathRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) MemAgentMemcgSet(ctx context.Context, req *pb.MemAgentMemcgConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) MemAgentCompactSet(ctx context.Context, req *pb.MemAgentCompactConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockAgentService) GetDiagnosticData(ctx context.Context, req *pb.GetDiagnosticDataRequest) (*pb.GetDiagnosticDataResponse, error) {
	return &pb.GetDiagnosticDataResponse{}, nil
}

// errorReturningMockAgent is a mock that returns errors for testing error handling
type errorReturningMockAgent struct {
	mockAgentService
}

func (m *errorReturningMockAgent) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	return nil, errors.New("mock agent error")
}

// setupMockAgent is a shared helper function to create and start a mock agent server
// It returns the server and listener for use in tests
func setupMockAgent(t *testing.T) (*ttrpc.Server, net.Listener) {
	agentServer, err := ttrpc.NewServer()
	require.NoError(t, err, "failed to create ttrpc server")

	pb.RegisterAgentServiceService(agentServer, &mockAgentService{})
	pb.RegisterHealthService(agentServer, &mockAgentService{})

	agentListener, err := net.Listen("tcp", testListenAddressProxy)
	require.NoError(t, err, "failed to create listener")

	go func() {
		_ = agentServer.Serve(context.Background(), agentListener)
	}()

	return agentServer, agentListener
}
