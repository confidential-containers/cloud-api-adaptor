package testutil

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MockConn implements net.Conn for testing
type MockConn struct {
	net.Conn
	closed bool
	mu     sync.Mutex
}

func NewMockConn() *MockConn {
	return &MockConn{}
}

func (m *MockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *MockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *MockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (m *MockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}
}

func (m *MockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// MockAgentServiceClient implements pb.AgentServiceService for testing
type MockAgentServiceClient struct {
	pb.AgentServiceService
	CreateContainerErr    error
	StartContainerErr     error
	RemoveContainerErr    error
	ExecProcessErr        error
	SignalProcessErr      error
	WaitProcessErr        error
	UpdateContainerErr    error
	CreateSandboxErr      error
	DestroySandboxErr     error
	CreateContainerCalled bool
	StartContainerCalled  bool
	RemoveContainerCalled bool
	CreateSandboxCalled   bool
	DestroySandboxCalled  bool
}

// Container operations
func (m *MockAgentServiceClient) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	m.CreateContainerCalled = true
	if m.CreateContainerErr != nil {
		return nil, m.CreateContainerErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {
	m.StartContainerCalled = true
	if m.StartContainerErr != nil {
		return nil, m.StartContainerErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {
	m.RemoveContainerCalled = true
	if m.RemoveContainerErr != nil {
		return nil, m.RemoveContainerErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (*emptypb.Empty, error) {
	if m.UpdateContainerErr != nil {
		return nil, m.UpdateContainerErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) StatsContainer(ctx context.Context, req *pb.StatsContainerRequest) (*pb.StatsContainerResponse, error) {
	return &pb.StatsContainerResponse{}, nil
}

func (m *MockAgentServiceClient) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) ResumeContainer(ctx context.Context, req *pb.ResumeContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Process operations
func (m *MockAgentServiceClient) ExecProcess(ctx context.Context, req *pb.ExecProcessRequest) (*emptypb.Empty, error) {
	if m.ExecProcessErr != nil {
		return nil, m.ExecProcessErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) SignalProcess(ctx context.Context, req *pb.SignalProcessRequest) (*emptypb.Empty, error) {
	if m.SignalProcessErr != nil {
		return nil, m.SignalProcessErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) WaitProcess(ctx context.Context, req *pb.WaitProcessRequest) (*pb.WaitProcessResponse, error) {
	if m.WaitProcessErr != nil {
		return nil, m.WaitProcessErr
	}
	return &pb.WaitProcessResponse{Status: 0}, nil
}

// Stream operations
func (m *MockAgentServiceClient) WriteStdin(ctx context.Context, req *pb.WriteStreamRequest) (*pb.WriteStreamResponse, error) {
	return &pb.WriteStreamResponse{Len: uint32(len(req.Data))}, nil
}

func (m *MockAgentServiceClient) ReadStdout(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{Data: []byte("stdout data")}, nil
}

func (m *MockAgentServiceClient) ReadStderr(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{Data: []byte("stderr data")}, nil
}

func (m *MockAgentServiceClient) CloseStdin(ctx context.Context, req *pb.CloseStdinRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) TtyWinResize(ctx context.Context, req *pb.TtyWinResizeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Network operations
func (m *MockAgentServiceClient) UpdateInterface(ctx context.Context, req *pb.UpdateInterfaceRequest) (*protocols.Interface, error) {
	return &protocols.Interface{}, nil
}

func (m *MockAgentServiceClient) UpdateRoutes(ctx context.Context, req *pb.UpdateRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}

func (m *MockAgentServiceClient) ListInterfaces(ctx context.Context, req *pb.ListInterfacesRequest) (*pb.Interfaces, error) {
	return &pb.Interfaces{}, nil
}

func (m *MockAgentServiceClient) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}

func (m *MockAgentServiceClient) AddARPNeighbors(ctx context.Context, req *pb.AddARPNeighborsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Sandbox operations
func (m *MockAgentServiceClient) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {
	m.CreateSandboxCalled = true
	if m.CreateSandboxErr != nil {
		return nil, m.CreateSandboxErr
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {
	m.DestroySandboxCalled = true
	if m.DestroySandboxErr != nil {
		return nil, m.DestroySandboxErr
	}
	return &emptypb.Empty{}, nil
}

// Mount operations
func (m *MockAgentServiceClient) UpdateEphemeralMounts(ctx context.Context, req *pb.UpdateEphemeralMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) RemoveStaleVirtiofsShareMounts(ctx context.Context, req *pb.RemoveStaleVirtiofsShareMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// IPTables operations
func (m *MockAgentServiceClient) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (*pb.GetIPTablesResponse, error) {
	return &pb.GetIPTablesResponse{}, nil
}

func (m *MockAgentServiceClient) SetIPTables(ctx context.Context, req *pb.SetIPTablesRequest) (*pb.SetIPTablesResponse, error) {
	return &pb.SetIPTablesResponse{}, nil
}

// Memory operations
func (m *MockAgentServiceClient) OnlineCPUMem(ctx context.Context, req *pb.OnlineCPUMemRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) MemHotplugByProbe(ctx context.Context, req *pb.MemHotplugByProbeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) MemAgentMemcgSet(ctx context.Context, req *pb.MemAgentMemcgConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) MemAgentCompactSet(ctx context.Context, req *pb.MemAgentCompactConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Storage operations
func (m *MockAgentServiceClient) GetVolumeStats(ctx context.Context, req *pb.VolumeStatsRequest) (*pb.VolumeStatsResponse, error) {
	return &pb.VolumeStatsResponse{}, nil
}

func (m *MockAgentServiceClient) ResizeVolume(ctx context.Context, req *pb.ResizeVolumeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) AddSwap(ctx context.Context, req *pb.AddSwapRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) AddSwapPath(ctx context.Context, req *pb.AddSwapPathRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Miscellaneous operations
func (m *MockAgentServiceClient) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.Metrics, error) {
	return &pb.Metrics{}, nil
}

func (m *MockAgentServiceClient) ReseedRandomDev(ctx context.Context, req *pb.ReseedRandomDevRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) GetGuestDetails(ctx context.Context, req *pb.GuestDetailsRequest) (*pb.GuestDetailsResponse, error) {
	return &pb.GuestDetailsResponse{}, nil
}

func (m *MockAgentServiceClient) SetGuestDateTime(ctx context.Context, req *pb.SetGuestDateTimeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) CopyFile(ctx context.Context, req *pb.CopyFileRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) GetOOMEvent(ctx context.Context, req *pb.GetOOMEventRequest) (*pb.OOMEvent, error) {
	return &pb.OOMEvent{}, nil
}

func (m *MockAgentServiceClient) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *MockAgentServiceClient) GetDiagnosticData(ctx context.Context, req *pb.GetDiagnosticDataRequest) (*pb.GetDiagnosticDataResponse, error) {
	return &pb.GetDiagnosticDataResponse{}, nil
}

// MockHealthServiceClient implements pb.HealthService for testing
type MockHealthServiceClient struct {
	pb.HealthService
	CheckErr   error
	VersionErr error
}

func (m *MockHealthServiceClient) Check(ctx context.Context, req *pb.CheckRequest) (*pb.HealthCheckResponse, error) {
	if m.CheckErr != nil {
		return nil, m.CheckErr
	}
	return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_SERVING}, nil
}

func (m *MockHealthServiceClient) Version(ctx context.Context, req *pb.CheckRequest) (*pb.VersionCheckResponse, error) {
	if m.VersionErr != nil {
		return nil, m.VersionErr
	}
	return &pb.VersionCheckResponse{
		GrpcVersion:  "1.0.0",
		AgentVersion: "2.0.0",
	}, nil
}
