// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/ttrpc"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNewAgentProxy(t *testing.T) {

	socketPath := "/run/dummy.sock"

	proxy := NewAgentProxy("podvm", socketPath, "", nil, nil, 0)
	p, ok := proxy.(*agentProxy)
	if !ok {
		t.Fatalf("expect %T, got %T", &agentProxy{}, proxy)
	}
	if e, a := socketPath, p.socketPath; e != a {
		t.Fatalf("expect %q, got %q", e, a)
	}
}

func TestStartStop(t *testing.T) {

	dir := t.TempDir()

	socketPath := filepath.Join(dir, "test.sock")

	agentServer, err := ttrpc.NewServer()
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}
	pb.RegisterAgentServiceService(agentServer, &agentMock{})
	pb.RegisterHealthService(agentServer, &agentMock{})

	agentListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}
	defer func() {
		err := agentListener.Close()
		if e, a := net.ErrClosed, err; !errors.Is(a, e) {
			t.Fatalf("expect %q, got %q", e, a)
		}
	}()

	agentServerErrCh := make(chan error)
	go func() {
		defer close(agentServerErrCh)

		err := agentServer.Serve(context.Background(), agentListener)
		if err != nil {
			agentServerErrCh <- err
			return
		}
	}()
	defer func() {
		err := agentServer.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}
	}()

	serverURL := &url.URL{
		Scheme: "grpc",
		Host:   agentListener.Addr().String(),
	}

	proxy := NewAgentProxy("podvm", socketPath, "", nil, nil, 5*time.Second)
	p, ok := proxy.(*agentProxy)
	if !ok {
		t.Fatalf("expect %T, got %T", &agentProxy{}, proxy)
	}

	proxyErrCh := make(chan error)
	go func() {
		defer close(proxyErrCh)

		if err := proxy.Start(context.Background(), serverURL); err != nil {
			proxyErrCh <- err
		}
	}()
	defer func() {
		if err := p.Shutdown(); err != nil {
			t.Fatalf("expect no error, got %q", err)
		}
	}()

	select {
	case err := <-proxyErrCh:
		t.Fatalf("expect no error, got %q", err)
	case <-proxy.Ready():
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("expect no error, got %q", err)
	}

	ttrpcClient := ttrpc.NewClient(conn)

	client := struct {
		pb.AgentServiceService
		pb.HealthService
	}{
		AgentServiceService: pb.NewAgentServiceClient(ttrpcClient),
		HealthService:       pb.NewHealthClient(ttrpcClient),
	}

	{
		res, err := client.CreateContainer(context.Background(), &pb.CreateContainerRequest{ContainerId: "123", OCI: &pb.Spec{Annotations: map[string]string{"aaa": "111"}}})
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}
		if res == nil {
			t.Fatal("expect non nil, got nil")
		}
	}

	select {
	case err := <-agentServerErrCh:
		t.Fatalf("expect no error, got %q", err)
	case err := <-proxyErrCh:
		t.Fatalf("expect no error, got %q", err)
	default:
	}
}

func TestDialerSuccess(t *testing.T) {
	p := &agentProxy{
		proxyTimeout: 5 * time.Second,
	}

	for {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}

		address := listener.Addr().String()

		if err := listener.Close(); err != nil {
			t.Fatalf("expect no error, got %q", err)
		}

		listenerErrCh := make(chan error)
		go func() {
			defer close(listenerErrCh)

			time.Sleep(250 * time.Millisecond)

			var err error
			// Open the same port
			listener, err = net.Listen("tcp", address)
			if err != nil {
				listenerErrCh <- err
			}
		}()

		conn, err := p.dial(context.Background(), address)
		if err == nil {
			listener.Close()
			break
		}
		defer conn.Close()

		if e := <-listenerErrCh; e != nil {
			// A rare case occurs. Retry the test.
			t.Logf("%v", e)
			continue
		}

		listener.Close()
		if err != nil {
			t.Fatalf("expect no error, got %q", err)
		}
		break
	}
}

func TestDialerFailure(t *testing.T) {
	p := &agentProxy{
		proxyTimeout: 5 * time.Second,
	}

	address := "0.0.0.0:0"
	conn, err := p.dial(context.Background(), address)
	if err == nil {
		conn.Close()
		t.Fatal("expect error, got nil")
	}

	if e, a := "failed to establish agent proxy connection", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("expect %q, got %q", e, a)
	}
}

type agentMock struct{}

func (m *agentMock) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) ExecProcess(ctx context.Context, req *pb.ExecProcessRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) SignalProcess(ctx context.Context, req *pb.SignalProcessRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) WaitProcess(ctx context.Context, req *pb.WaitProcessRequest) (*pb.WaitProcessResponse, error) {
	return &pb.WaitProcessResponse{}, nil
}
func (m *agentMock) UpdateContainer(ctx context.Context, req *pb.UpdateContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) UpdateEphemeralMounts(ctx context.Context, req *pb.UpdateEphemeralMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) StatsContainer(ctx context.Context, req *pb.StatsContainerRequest) (*pb.StatsContainerResponse, error) {
	return &pb.StatsContainerResponse{}, nil
}
func (m *agentMock) PauseContainer(ctx context.Context, req *pb.PauseContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) ResumeContainer(ctx context.Context, req *pb.ResumeContainerRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) WriteStdin(ctx context.Context, req *pb.WriteStreamRequest) (*pb.WriteStreamResponse, error) {
	return &pb.WriteStreamResponse{}, nil
}
func (m *agentMock) ReadStdout(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{}, nil
}
func (m *agentMock) ReadStderr(ctx context.Context, req *pb.ReadStreamRequest) (*pb.ReadStreamResponse, error) {
	return &pb.ReadStreamResponse{}, nil
}
func (m *agentMock) CloseStdin(ctx context.Context, req *pb.CloseStdinRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) TtyWinResize(ctx context.Context, req *pb.TtyWinResizeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) UpdateInterface(ctx context.Context, req *pb.UpdateInterfaceRequest) (*protocols.Interface, error) {
	return &protocols.Interface{}, nil
}
func (m *agentMock) UpdateRoutes(ctx context.Context, req *pb.UpdateRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}
func (m *agentMock) ListInterfaces(ctx context.Context, req *pb.ListInterfacesRequest) (*pb.Interfaces, error) {
	return &pb.Interfaces{}, nil
}
func (m *agentMock) ListRoutes(ctx context.Context, req *pb.ListRoutesRequest) (*pb.Routes, error) {
	return &pb.Routes{}, nil
}
func (m *agentMock) AddARPNeighbors(ctx context.Context, req *pb.AddARPNeighborsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) GetIPTables(ctx context.Context, req *pb.GetIPTablesRequest) (*pb.GetIPTablesResponse, error) {
	return &pb.GetIPTablesResponse{}, nil
}
func (m *agentMock) SetIPTables(ctx context.Context, req *pb.SetIPTablesRequest) (*pb.SetIPTablesResponse, error) {
	return &pb.SetIPTablesResponse{}, nil
}
func (m *agentMock) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.Metrics, error) {
	return &pb.Metrics{}, nil
}
func (m *agentMock) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) OnlineCPUMem(ctx context.Context, req *pb.OnlineCPUMemRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) ReseedRandomDev(ctx context.Context, req *pb.ReseedRandomDevRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) GetGuestDetails(ctx context.Context, req *pb.GuestDetailsRequest) (*pb.GuestDetailsResponse, error) {
	return &pb.GuestDetailsResponse{}, nil
}
func (m *agentMock) MemHotplugByProbe(ctx context.Context, req *pb.MemHotplugByProbeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) SetGuestDateTime(ctx context.Context, req *pb.SetGuestDateTimeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) CopyFile(ctx context.Context, req *pb.CopyFileRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) GetOOMEvent(ctx context.Context, req *pb.GetOOMEventRequest) (*pb.OOMEvent, error) {
	return &pb.OOMEvent{}, nil
}
func (m *agentMock) AddSwap(ctx context.Context, req *pb.AddSwapRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) GetVolumeStats(ctx context.Context, req *pb.VolumeStatsRequest) (*pb.VolumeStatsResponse, error) {
	return &pb.VolumeStatsResponse{}, nil
}
func (m *agentMock) ResizeVolume(ctx context.Context, req *pb.ResizeVolumeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (p *agentMock) RemoveStaleVirtiofsShareMounts(ctx context.Context, req *pb.RemoveStaleVirtiofsShareMountsRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) Check(ctx context.Context, req *pb.CheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{}, nil
}
func (m *agentMock) Version(ctx context.Context, req *pb.CheckRequest) (*pb.VersionCheckResponse, error) {
	return &pb.VersionCheckResponse{}, nil
}
func (m *agentMock) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) AddSwapPath(ctx context.Context, req *pb.AddSwapPathRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) MemAgentMemcgSet(ctx context.Context, req *pb.MemAgentMemcgConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func (m *agentMock) MemAgentCompactSet(ctx context.Context, req *pb.MemAgentCompactConfig) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
