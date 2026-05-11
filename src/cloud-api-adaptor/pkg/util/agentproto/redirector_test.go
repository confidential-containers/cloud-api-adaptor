package agentproto

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto/testutil"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

// setupTestRedirector creates a test redirector with the given mock clients
func setupTestRedirector(t *testing.T, mockAgent *testutil.MockAgentServiceClient, mockHealth *testutil.MockHealthServiceClient) *redirector {
	t.Helper()

	r := &redirector{
		agentClient: &client{
			AgentServiceService: mockAgent,
			HealthService:       mockHealth,
		},
		dialer: func(ctx context.Context) (net.Conn, error) {
			return testutil.NewMockConn(), nil
		},
	}
	r.once.Do(func() {}) // Pre-initialize to avoid connection logic
	return r
}

// TestNewRedirector tests the NewRedirector constructor
func TestNewRedirector(t *testing.T) {
	tests := []struct {
		name   string
		dialer func(context.Context) (net.Conn, error)
	}{
		{
			name: "valid dialer",
			dialer: func(ctx context.Context) (net.Conn, error) {
				return testutil.NewMockConn(), nil
			},
		},
		{
			name:   "nil dialer",
			dialer: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRedirector(tt.dialer)
			if r == nil {
				t.Fatal("NewRedirector returned nil")
			}

			redirectorImpl, ok := r.(*redirector)
			if !ok {
				t.Fatal("NewRedirector did not return *redirector type")
			}

			if tt.dialer != nil && redirectorImpl.dialer == nil {
				t.Error("dialer was not set correctly")
			}
		})
	}
}

// TestConnect tests the Connect method with various scenarios
func TestConnect(t *testing.T) {
	tests := []struct {
		name      string
		dialer    func(context.Context) (net.Conn, error)
		cancelCtx bool
		wantErr   bool
	}{
		{
			name: "successful connection",
			dialer: func(ctx context.Context) (net.Conn, error) {
				return testutil.NewMockConn(), nil
			},
			wantErr: false,
		},
		{
			name: "connection failure",
			dialer: func(ctx context.Context) (net.Conn, error) {
				return nil, errors.New("connection failed")
			},
			wantErr: true,
		},
		{
			name: "context cancellation",
			dialer: func(ctx context.Context) (net.Conn, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			cancelCtx: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &redirector{dialer: tt.dialer}

			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := r.Connect(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConnectConcurrency tests concurrent Connect calls
func TestConnectConcurrency(t *testing.T) {
	mockAgent := &testutil.MockAgentServiceClient{}
	r := setupTestRedirector(t, mockAgent, nil)

	ctx := context.Background()
	const numGoroutines = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Connect(ctx); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent Connect failed: %v", err)
	}
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	mockAgent := &testutil.MockAgentServiceClient{}
	r := setupTestRedirector(t, mockAgent, nil)

	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestContainerOperations tests container operations
func TestContainerOperations(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*testutil.MockAgentServiceClient)
		operation func(*redirector, context.Context) error
		wantErr   bool
	}{
		{
			name: "CreateContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.CreateContainerErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.CreateContainer(ctx, &pb.CreateContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name: "CreateContainer failure",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.CreateContainerErr = errors.New("create failed")
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.CreateContainer(ctx, &pb.CreateContainerRequest{})
				return err
			},
			wantErr: true,
		},
		{
			name: "StartContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.StartContainerErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.StartContainer(ctx, &pb.StartContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name: "RemoveContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.RemoveContainerErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.RemoveContainer(ctx, &pb.RemoveContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name: "UpdateContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.UpdateContainerErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.UpdateContainer(ctx, &pb.UpdateContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name:      "StatsContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.StatsContainer(ctx, &pb.StatsContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name:      "PauseContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.PauseContainer(ctx, &pb.PauseContainerRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name:      "ResumeContainer success",
			setupMock: func(m *testutil.MockAgentServiceClient) {},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.ResumeContainer(ctx, &pb.ResumeContainerRequest{})
				return err
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgent := &testutil.MockAgentServiceClient{}
			tt.setupMock(mockAgent)
			r := setupTestRedirector(t, mockAgent, nil)

			ctx := context.Background()
			err := tt.operation(r, ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// TestProcessOperations tests process operations
func TestProcessOperations(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*testutil.MockAgentServiceClient)
		operation func(*redirector, context.Context) error
		wantErr   bool
	}{
		{
			name: "ExecProcess success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.ExecProcessErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.ExecProcess(ctx, &pb.ExecProcessRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name: "SignalProcess success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.SignalProcessErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				_, err := r.SignalProcess(ctx, &pb.SignalProcessRequest{})
				return err
			},
			wantErr: false,
		},
		{
			name: "WaitProcess success",
			setupMock: func(m *testutil.MockAgentServiceClient) {
				m.WaitProcessErr = nil
			},
			operation: func(r *redirector, ctx context.Context) error {
				resp, err := r.WaitProcess(ctx, &pb.WaitProcessRequest{})
				if err == nil && resp == nil {
					return errors.New("nil response")
				}
				return err
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgent := &testutil.MockAgentServiceClient{}
			tt.setupMock(mockAgent)
			r := setupTestRedirector(t, mockAgent, nil)

			ctx := context.Background()
			err := tt.operation(r, ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// TestStreamOperations tests I/O stream operations
func runRedirectorTests(t *testing.T, tests []struct {
	name string
	run  func(*redirector, context.Context) error
}) {
	t.Helper()

	mockAgent := &testutil.MockAgentServiceClient{}
	r := setupTestRedirector(t, mockAgent, nil)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(r, ctx); err != nil {
				t.Errorf("%s() error = %v", tt.name, err)
			}
		})
	}
}

// TestStreamOperations tests I/O stream operations
func TestStreamOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "WriteStdin",
			run: func(r *redirector, ctx context.Context) error {
				resp, err := r.WriteStdin(ctx, &pb.WriteStreamRequest{Data: []byte("test")})
				if err == nil && resp == nil {
					return errors.New("nil response")
				}
				return err
			},
		},
		{
			name: "ReadStdout",
			run: func(r *redirector, ctx context.Context) error {
				resp, err := r.ReadStdout(ctx, &pb.ReadStreamRequest{})
				if err == nil && resp == nil {
					return errors.New("nil response")
				}
				return err
			},
		},
		{
			name: "ReadStderr",
			run: func(r *redirector, ctx context.Context) error {
				resp, err := r.ReadStderr(ctx, &pb.ReadStreamRequest{})
				if err == nil && resp == nil {
					return errors.New("nil response")
				}
				return err
			},
		},
		{
			name: "CloseStdin",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.CloseStdin(ctx, &pb.CloseStdinRequest{})
				return err
			},
		},
		{
			name: "TtyWinResize",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.TtyWinResize(ctx, &pb.TtyWinResizeRequest{})
				return err
			},
		},
	})
}

// TestNetworkOperations tests network operations
func TestNetworkOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "UpdateInterface",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.UpdateInterface(ctx, &pb.UpdateInterfaceRequest{})
				return err
			},
		},
		{
			name: "UpdateRoutes",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.UpdateRoutes(ctx, &pb.UpdateRoutesRequest{})
				return err
			},
		},
		{
			name: "ListInterfaces",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.ListInterfaces(ctx, &pb.ListInterfacesRequest{})
				return err
			},
		},
		{
			name: "ListRoutes",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.ListRoutes(ctx, &pb.ListRoutesRequest{})
				return err
			},
		},
		{
			name: "AddARPNeighbors",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.AddARPNeighbors(ctx, &pb.AddARPNeighborsRequest{})
				return err
			},
		},
	})
}

// TestSandboxOperations tests sandbox operations
func TestSandboxOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "CreateSandbox",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.CreateSandbox(ctx, &pb.CreateSandboxRequest{})
				return err
			},
		},
		{
			name: "DestroySandbox",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.DestroySandbox(ctx, &pb.DestroySandboxRequest{})
				return err
			},
		},
	})
}

// TestMountOperations tests mount operations
func TestMountOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "UpdateEphemeralMounts",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.UpdateEphemeralMounts(ctx, &pb.UpdateEphemeralMountsRequest{})
				return err
			},
		},
		{
			name: "RemoveStaleVirtiofsShareMounts",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.RemoveStaleVirtiofsShareMounts(ctx, &pb.RemoveStaleVirtiofsShareMountsRequest{})
				return err
			},
		},
	})
}

// TestIPTablesOperations tests IPTables operations
func TestIPTablesOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "GetIPTables",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetIPTables(ctx, &pb.GetIPTablesRequest{})
				return err
			},
		},
		{
			name: "SetIPTables",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.SetIPTables(ctx, &pb.SetIPTablesRequest{})
				return err
			},
		},
	})
}

// TestMemoryOperations tests memory operations
func TestMemoryOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "OnlineCPUMem",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.OnlineCPUMem(ctx, &pb.OnlineCPUMemRequest{})
				return err
			},
		},
		{
			name: "MemHotplugByProbe",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.MemHotplugByProbe(ctx, &pb.MemHotplugByProbeRequest{})
				return err
			},
		},
		{
			name: "MemAgentMemcgSet",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.MemAgentMemcgSet(ctx, &pb.MemAgentMemcgConfig{})
				return err
			},
		},
		{
			name: "MemAgentCompactSet",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.MemAgentCompactSet(ctx, &pb.MemAgentCompactConfig{})
				return err
			},
		},
	})
}

// TestStorageOperations tests storage operations
func TestStorageOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "GetVolumeStats",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetVolumeStats(ctx, &pb.VolumeStatsRequest{})
				return err
			},
		},
		{
			name: "ResizeVolume",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.ResizeVolume(ctx, &pb.ResizeVolumeRequest{})
				return err
			},
		},
		{
			name: "AddSwap",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.AddSwap(ctx, &pb.AddSwapRequest{})
				return err
			},
		},
		{
			name: "AddSwapPath",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.AddSwapPath(ctx, &pb.AddSwapPathRequest{})
				return err
			},
		},
	})
}

// TestMiscellaneousOperations tests miscellaneous operations
func TestMiscellaneousOperations(t *testing.T) {
	runRedirectorTests(t, []struct {
		name string
		run  func(*redirector, context.Context) error
	}{
		{
			name: "GetMetrics",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetMetrics(ctx, &pb.GetMetricsRequest{})
				return err
			},
		},
		{
			name: "ReseedRandomDev",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.ReseedRandomDev(ctx, &pb.ReseedRandomDevRequest{})
				return err
			},
		},
		{
			name: "GetGuestDetails",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetGuestDetails(ctx, &pb.GuestDetailsRequest{})
				return err
			},
		},
		{
			name: "SetGuestDateTime",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.SetGuestDateTime(ctx, &pb.SetGuestDateTimeRequest{})
				return err
			},
		},
		{
			name: "CopyFile",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.CopyFile(ctx, &pb.CopyFileRequest{})
				return err
			},
		},
		{
			name: "GetOOMEvent",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetOOMEvent(ctx, &pb.GetOOMEventRequest{})
				return err
			},
		},
		{
			name: "SetPolicy",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.SetPolicy(ctx, &pb.SetPolicyRequest{})
				return err
			},
		},
		{
			name: "GetDiagnosticData",
			run: func(r *redirector, ctx context.Context) error {
				_, err := r.GetDiagnosticData(ctx, &pb.GetDiagnosticDataRequest{})
				return err
			},
		},
	})
}

// TestHealthService tests health service operations
func TestHealthService(t *testing.T) {
	tests := []struct {
		name       string
		checkErr   error
		versionErr error
	}{
		{
			name:       "successful health check",
			checkErr:   nil,
			versionErr: nil,
		},
		{
			name:       "health check failure",
			checkErr:   errors.New("health check failed"),
			versionErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgent := &testutil.MockAgentServiceClient{}
			mockHealth := &testutil.MockHealthServiceClient{
				CheckErr:   tt.checkErr,
				VersionErr: tt.versionErr,
			}
			r := setupTestRedirector(t, mockAgent, mockHealth)
			ctx := context.Background()

			_, err := r.Check(ctx, &pb.CheckRequest{})
			if (err != nil) != (tt.checkErr != nil) {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.checkErr != nil)
			}

			_, err = r.Version(ctx, &pb.CheckRequest{})
			if (err != nil) != (tt.versionErr != nil) {
				t.Errorf("Version() error = %v, wantErr %v", err, tt.versionErr != nil)
			}
		})
	}
}

// TestInterfaceCompliance verifies interface implementation
func TestInterfaceCompliance(t *testing.T) {
	var _ Redirector = (*redirector)(nil)
	var _ pb.AgentServiceService = (*client)(nil)
	var _ pb.HealthService = (*client)(nil)
}
