// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package adaptor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/cloud"
	daemon "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder/interceptor"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/containerd/containerd/pkg/cri/annotations"
	"github.com/containerd/ttrpc"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	agent "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestShim(t *testing.T) {

	dir, err := os.MkdirTemp("", "shimtest-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	helperSocketPath := os.Getenv("SHIMTEST_HELPER_SOCKET")
	if helperSocketPath != "" {
		if err := os.Remove(helperSocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	} else {
		helperSocketPath = filepath.Join(dir, "helper.sock")
	}

	podsDir := filepath.Join(dir, "pods")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentDone := make(chan struct{})
	agentSocketPath := os.Getenv("SHIMTEST_AGENT_SOCKET")
	if agentSocketPath != "" {
		t.Logf("SHIMTEST_AGENT_SOCKET is specified\nthe agent protocol forwarder will connect to kata agent at %s", agentSocketPath)
	} else {
		agentSocketPath = "@" + filepath.Join(dir, "agent.sock")
		go startTestAgent(t, ctx, agentSocketPath, agentDone)
	}

	daemonDone := make(chan struct{})
	daemonAddr := os.Getenv("AGENT_PROTOCOL_FORWARDER_ADDRESS")
	if daemonAddr == "" {
		addrCh := make(chan string)
		go startDaemon(t, ctx, agentSocketPath, addrCh, daemonDone)
		daemonAddr = <-addrCh
	} else {
		defer close(daemonDone)
	}

	primaryIP, port, err := net.SplitHostPort(daemonAddr)
	if err != nil {
		t.Fatal(err)
	}
	secondaryIP := os.Getenv("SHIMTEST_SECONDARY_POD_NODE_IP")

	var workerNode podnetwork.WorkerNode

	switch tun := os.Getenv("SHIMTEST_TUNNEL_TYPE"); tun {
	case "", "mock":
		workerNode = &mockWorkerNode{}
	default:
		workerNode, err = podnetwork.NewWorkerNode(&tunneler.NetworkConfig{TunnelType: tun})
		if err != nil {
			t.Fatal(err)
		}
	}

	serverConfig := &cloud.ServerConfig{
		SocketPath:              helperSocketPath,
		PodsDir:                 podsDir,
		ForwarderPort:           port,
		ProxyTimeout:            5 * time.Second,
		EnableCloudConfigVerify: false,
		PeerPodsLimitPerNode:    -1,
	}

	provider := &mockProvider{primaryIP: primaryIP, secondaryIP: secondaryIP}
	srv := NewServer(provider, serverConfig, workerNode)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		if err := srv.Start(ctx); err != nil {
			t.Error(err)
		}
	}()

	<-srv.Ready()

	clientDone := make(chan error)

	if os.Getenv("SHIMTEST_USE_REAL_SHIM") != "" {
		defer close(clientDone)

		t.Logf("SHIMTEST_USE_REAL_SHIM is enabled\nhelper daemon socket is %s\nUse crictl, containerd, and kata shim to test the helper daemon", helperSocketPath)
	} else {

		agentForwarderSocketPath := runMockShim(t, ctx, helperSocketPath)

		if os.Getenv("SHIMTEST_USE_AGENT_CTL") != "" {
			defer close(clientDone)
			t.Logf("SHIMTEST_USE_AGENT_CTL is enabled\nagent forwarder socket is %s\nUse kata-agent-ctl to test the helper daemon", agentForwarderSocketPath)

		} else {

			go func() {
				defer close(clientDone)

				conn, err := net.Dial("unix", agentForwarderSocketPath)
				if err != nil {
					t.Error(err)
					return
				}
				ttrpcClient := ttrpc.NewClient(conn)
				defer ttrpcClient.Close()

				client := agent.NewAgentServiceClient(ttrpcClient)

				if _, err := client.GetGuestDetails(ctx, &agent.GuestDetailsRequest{}); err != nil {
					t.Error(err)
					return
				}
			}()
		}
	}

	select {
	case <-ctx.Done():
	case <-agentDone:
	case <-daemonDone:
	case <-clientDone:
	case <-serverDone:
	}
}

func startDaemon(t *testing.T, ctx context.Context, agentSocketPath string, portCh chan string, done chan struct{}) {

	defer close(done)

	config := &daemon.Config{}

	nsPath := os.Getenv("AGENT_PROTOCOL_FORWARDER_NAMESPACE")
	interceptor := interceptor.NewInterceptor(agentSocketPath, nsPath)

	d := daemon.NewDaemon(config, "127.0.0.1:0", nil, interceptor, &mockPodNode{})

	daemonErr := make(chan error)
	go func() {
		defer close(daemonErr)

		if err := d.Start(ctx); err != nil {
			daemonErr <- err
		}
	}()

	portCh <- d.Addr()

	select {
	case <-ctx.Done():
	case err := <-daemonErr:
		if err != nil {
			t.Error(err)
		}
	}
}

func startTestAgent(t *testing.T, ctx context.Context, agentSocketPath string, done chan struct{}) {

	defer close(done)

	ttrpcServer, err := ttrpc.NewServer()
	if err != nil {
		t.Error(err)
		return
	}

	agent.RegisterAgentServiceService(ttrpcServer, newAgentService())
	agent.RegisterHealthService(ttrpcServer, &healthService{})

	socket := agentSocketPath

	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Error(err)
		return
	}

	ttrpcServerErr := make(chan error)
	go func() {
		defer close(ttrpcServerErr)

		if err := ttrpcServer.Serve(ctx, listener); err != nil {
			ttrpcServerErr <- err
		}
	}()
	defer func() {
		if err := ttrpcServer.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()

	select {
	case <-ctx.Done():
	case err = <-ttrpcServerErr:
	}
	if err != nil {
		t.Error(err)
	}
}

func runMockShim(t *testing.T, ctx context.Context, helperSocketPath string) string {

	conn, err := net.Dial("unix", helperSocketPath)
	if err != nil {
		t.Error(err)
	}
	ttrpcClient := ttrpc.NewClient(conn)
	defer ttrpcClient.Close()

	client := pb.NewHypervisorClient(ttrpcClient)

	podID := "123"

	req1 := &pb.CreateVMRequest{
		Id: podID,
		Annotations: map[string]string{
			annotations.SandboxNamespace: "default",
			annotations.SandboxName:      "pod1",
		},
	}

	res1, err := client.CreateVM(ctx, req1)
	if err != nil {
		t.Fatal(err)
	}

	agentForwarderSocketPath := res1.AgentSocketPath

	req2 := &pb.StartVMRequest{
		Id: podID,
	}

	if _, err := client.StartVM(ctx, req2); err != nil {
		t.Fatal(err)
	}

	return agentForwarderSocketPath
}

type containerID string

type container struct {
	done      chan struct{}
	readCount int64
	once      sync.Once
}

type agentService struct {
	containers map[containerID]*container
	mutex      sync.Mutex
}

func newAgentService() *agentService {
	return &agentService{
		containers: make(map[containerID]*container),
	}
}

func (s *agentService) get(cid string) (*container, error) {

	s.mutex.Lock()
	c := s.containers[containerID(cid)]
	s.mutex.Unlock()

	if c == nil {
		return nil, fmt.Errorf("container %q not found", cid)
	}

	return c, nil
}

func (s *agentService) CreateContainer(ctx context.Context, req *agent.CreateContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	cid := containerID(req.ContainerId)
	c := &container{
		done: make(chan struct{}),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.containers[cid] = c

	return &emptypb.Empty{}, nil
}
func (s *agentService) StartContainer(ctx context.Context, req *agent.StartContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) RemoveContainer(ctx context.Context, req *agent.RemoveContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	cid := containerID(req.ContainerId)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.containers, cid)

	return &emptypb.Empty{}, nil
}
func (s *agentService) ExecProcess(ctx context.Context, req *agent.ExecProcessRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) SignalProcess(ctx context.Context, req *agent.SignalProcessRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	c, err := s.get(req.ContainerId)
	if err != nil {
		return nil, err
	}

	c.once.Do(func() {
		close(c.done)
	})

	return &emptypb.Empty{}, nil
}
func (s *agentService) WaitProcess(ctx context.Context, req *agent.WaitProcessRequest) (*agent.WaitProcessResponse, error) {
	log.Printf("agent call: %T %#v", req, req)

	c, err := s.get(req.ContainerId)
	if err != nil {
		return nil, err
	}

	<-c.done

	return &agent.WaitProcessResponse{}, nil
}
func (s *agentService) UpdateContainer(ctx context.Context, req *agent.UpdateContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) UpdateEphemeralMounts(ctx context.Context, req *agent.UpdateEphemeralMountsRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) StatsContainer(ctx context.Context, req *agent.StatsContainerRequest) (*agent.StatsContainerResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.StatsContainerResponse{}, nil
}
func (s *agentService) PauseContainer(ctx context.Context, req *agent.PauseContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) ResumeContainer(ctx context.Context, req *agent.ResumeContainerRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) WriteStdin(ctx context.Context, req *agent.WriteStreamRequest) (*agent.WriteStreamResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.WriteStreamResponse{Len: uint32(len(req.Data))}, nil
}

func (s *agentService) ReadStdout(ctx context.Context, req *agent.ReadStreamRequest) (*agent.ReadStreamResponse, error) {
	log.Printf("agent call: ReadStdout %#v", req)

	c, err := s.get(req.ContainerId)
	if err != nil {
		return nil, err
	}

	count := atomic.AddInt64(&c.readCount, 1) - 1
	sleep := 0
	if count > 0 {
		sleep = 1 << count
	}
	timer := time.NewTimer(time.Duration(sleep) * time.Second)

	select {
	case <-c.done:
		return nil, io.EOF

	case <-timer.C:
		str := fmt.Sprintf("data from agent (sleeping for %d seconds)\n", 2<<count)
		return &agent.ReadStreamResponse{Data: []byte(str)}, nil
	}
}

func (s *agentService) ReadStderr(ctx context.Context, req *agent.ReadStreamRequest) (*agent.ReadStreamResponse, error) {
	log.Printf("agent call: ReadStderr %#v", req)

	c, err := s.get(req.ContainerId)
	if err != nil {
		return nil, err
	}

	<-c.done

	return nil, io.EOF
}
func (s *agentService) CloseStdin(ctx context.Context, req *agent.CloseStdinRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) TtyWinResize(ctx context.Context, req *agent.TtyWinResizeRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) UpdateInterface(ctx context.Context, req *agent.UpdateInterfaceRequest) (*protocols.Interface, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &protocols.Interface{}, nil
}
func (s *agentService) UpdateRoutes(ctx context.Context, req *agent.UpdateRoutesRequest) (*agent.Routes, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.Routes{}, nil
}
func (s *agentService) ListInterfaces(ctx context.Context, req *agent.ListInterfacesRequest) (*agent.Interfaces, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.Interfaces{}, nil
}
func (s *agentService) ListRoutes(ctx context.Context, req *agent.ListRoutesRequest) (*agent.Routes, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.Routes{}, nil
}
func (s *agentService) AddARPNeighbors(ctx context.Context, req *agent.AddARPNeighborsRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) GetIPTables(ctx context.Context, req *agent.GetIPTablesRequest) (*agent.GetIPTablesResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.GetIPTablesResponse{}, nil
}
func (s *agentService) SetIPTables(ctx context.Context, req *agent.SetIPTablesRequest) (*agent.SetIPTablesResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.SetIPTablesResponse{}, nil
}
func (s *agentService) GetMetrics(ctx context.Context, req *agent.GetMetricsRequest) (*agent.Metrics, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.Metrics{}, nil
}
func (s *agentService) CreateSandbox(ctx context.Context, req *agent.CreateSandboxRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) DestroySandbox(ctx context.Context, req *agent.DestroySandboxRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) OnlineCPUMem(ctx context.Context, req *agent.OnlineCPUMemRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) ReseedRandomDev(ctx context.Context, req *agent.ReseedRandomDevRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) GetGuestDetails(ctx context.Context, req *agent.GuestDetailsRequest) (*agent.GuestDetailsResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.GuestDetailsResponse{}, nil
}
func (s *agentService) MemHotplugByProbe(ctx context.Context, req *agent.MemHotplugByProbeRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) SetGuestDateTime(ctx context.Context, req *agent.SetGuestDateTimeRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) CopyFile(ctx context.Context, req *agent.CopyFileRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) GetOOMEvent(ctx context.Context, req *agent.GetOOMEventRequest) (*agent.OOMEvent, error) {
	log.Printf("agent call: %T %#v", req, req)
	select {}
	//return &agent.OOMEvent{}, nil
}
func (s *agentService) AddSwap(ctx context.Context, req *agent.AddSwapRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) GetVolumeStats(ctx context.Context, req *agent.VolumeStatsRequest) (*agent.VolumeStatsResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.VolumeStatsResponse{}, nil
}
func (s *agentService) ResizeVolume(ctx context.Context, req *agent.ResizeVolumeRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) RemoveStaleVirtiofsShareMounts(ctx context.Context, req *agent.RemoveStaleVirtiofsShareMountsRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}
func (s *agentService) SetPolicy(ctx context.Context, req *agent.SetPolicyRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: SetPolicy %#v", req)
	return &emptypb.Empty{}, nil
}

func (s *agentService) AddSwapPath(ctx context.Context, req *agent.AddSwapPathRequest) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}

func (s *agentService) MemAgentMemcgSet(ctx context.Context, req *agent.MemAgentMemcgConfig) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}

func (s *agentService) MemAgentCompactSet(ctx context.Context, req *agent.MemAgentCompactConfig) (*emptypb.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &emptypb.Empty{}, nil
}

type healthService struct{}

func (s *healthService) Check(ctx context.Context, req *agent.CheckRequest) (*agent.HealthCheckResponse, error) {
	//log.Printf("health call: %T %#v", req, req)
	return &agent.HealthCheckResponse{}, nil
}

func (s *healthService) Version(ctx context.Context, req *agent.CheckRequest) (*agent.VersionCheckResponse, error) {
	log.Printf("health call: %T %#v", req, req)
	return &agent.VersionCheckResponse{}, nil
}

type mockPodNode struct{}

func (n *mockPodNode) Setup() error {
	return nil
}

func (n *mockPodNode) Teardown() error {
	return nil
}
