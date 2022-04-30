// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

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

	daemon "github.com/confidential-containers/cloud-api-adapter/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adapter/pkg/podnetwork"
	"github.com/containerd/containerd/pkg/cri/annotations"
	"github.com/containerd/ttrpc"
	"github.com/gogo/protobuf/types"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	agent "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
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

	switch t := os.Getenv("SHIMTEST_TUNNEL_TYPE"); t {
	case "", "mock":
		workerNode = &mockWorkerNode{}
	case "routing":
		workerNode = podnetwork.NewWorkerNode("routing", "ens4")
	default:
		workerNode = podnetwork.NewWorkerNode(t, "")
	}

	server := NewServer(helperSocketPath, &mockVpcV1{primaryIP: primaryIP, secondaryIP: secondaryIP}, &ServiceConfig{}, workerNode, podsDir, port)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		if err := server.Start(ctx); err != nil {
			t.Error(err)
		}
	}()

	<-server.Ready()

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

	d := daemon.New(config, "127.0.0.1:0", agentSocketPath, nsPath, &mockPodNode{})

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
	if len(socket) > 0 && socket[0] == '@' {
		socket = socket + "\x00"
	}

	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Error(err)
		return
	}

	ttrpcServerErr := make(chan error)
	go func() {
		defer close(ttrpcServerErr)

		if ttrpcServer.Serve(ctx, listener); err != nil {
			if err != nil {
				ttrpcServerErr <- err
			}
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
	readCount int64
	done      chan struct{}
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

func (s *agentService) CreateContainer(ctx context.Context, req *agent.CreateContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	cid := containerID(req.ContainerId)
	c := &container{
		done: make(chan struct{}),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.containers[cid] = c

	return &types.Empty{}, nil
}
func (s *agentService) StartContainer(ctx context.Context, req *agent.StartContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) RemoveContainer(ctx context.Context, req *agent.RemoveContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	cid := containerID(req.ContainerId)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.containers, cid)

	return &types.Empty{}, nil
}
func (s *agentService) ExecProcess(ctx context.Context, req *agent.ExecProcessRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) SignalProcess(ctx context.Context, req *agent.SignalProcessRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)

	c, err := s.get(req.ContainerId)
	if err != nil {
		return nil, err
	}

	c.once.Do(func() {
		close(c.done)
	})

	return &types.Empty{}, nil
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
func (s *agentService) UpdateContainer(ctx context.Context, req *agent.UpdateContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) StatsContainer(ctx context.Context, req *agent.StatsContainerRequest) (*agent.StatsContainerResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.StatsContainerResponse{}, nil
}
func (s *agentService) PauseContainer(ctx context.Context, req *agent.PauseContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) ResumeContainer(ctx context.Context, req *agent.ResumeContainerRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
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
func (s *agentService) CloseStdin(ctx context.Context, req *agent.CloseStdinRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) TtyWinResize(ctx context.Context, req *agent.TtyWinResizeRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
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
func (s *agentService) AddARPNeighbors(ctx context.Context, req *agent.AddARPNeighborsRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) StartTracing(ctx context.Context, req *agent.StartTracingRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) StopTracing(ctx context.Context, req *agent.StopTracingRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) GetMetrics(ctx context.Context, req *agent.GetMetricsRequest) (*agent.Metrics, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.Metrics{}, nil
}
func (s *agentService) CreateVM(ctx context.Context, req *agent.CreateVMRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) DestroyVM(ctx context.Context, req *agent.DestroyVMRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) OnlineCPUMem(ctx context.Context, req *agent.OnlineCPUMemRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) ReseedRandomDev(ctx context.Context, req *agent.ReseedRandomDevRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) GetGuestDetails(ctx context.Context, req *agent.GuestDetailsRequest) (*agent.GuestDetailsResponse, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &agent.GuestDetailsResponse{}, nil
}
func (s *agentService) MemHotplugByProbe(ctx context.Context, req *agent.MemHotplugByProbeRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) SetGuestDateTime(ctx context.Context, req *agent.SetGuestDateTimeRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) CopyFile(ctx context.Context, req *agent.CopyFileRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}
func (s *agentService) GetOOMEvent(ctx context.Context, req *agent.GetOOMEventRequest) (*agent.OOMEvent, error) {
	log.Printf("agent call: %T %#v", req, req)
	select {}
	//return &agent.OOMEvent{}, nil
}
func (s *agentService) AddSwap(ctx context.Context, req *agent.AddSwapRequest) (*types.Empty, error) {
	log.Printf("agent call: %T %#v", req, req)
	return &types.Empty{}, nil
}

func (s *agentService) PullImage(ctx context.Context, req *agent.PullImageRequest) (*types.Empty, error) {
	log.Printf("agent call: PullImage %#v", req)
	return &types.Empty{}, nil
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
