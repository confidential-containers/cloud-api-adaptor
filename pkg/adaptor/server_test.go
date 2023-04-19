// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package adaptor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"github.com/containerd/containerd/pkg/cri/annotations"
	"github.com/containerd/ttrpc"
	"github.com/google/uuid"

	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
	agent "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

func TestServerStartAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, dir, socketPath, client, serverErrCh := testServerStart(t, ctx)
	defer testServerShutdown(t, s, socketPath, dir, serverErrCh)
	if _, err := client.Version(context.Background(), &pb.VersionRequest{}); err != nil {
		t.Error(err)
	}
}

func TestCreateStartAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, dir, socketPath, client, serverErrCh := testServerStart(t, ctx)
	defer testServerShutdown(t, s, socketPath, dir, serverErrCh)
	id := uuid.New().String()
	if _, err := client.CreateVM(
		context.Background(),
		&pb.CreateVMRequest{
			Id: id,
			Annotations: map[string]string{
				annotations.SandboxName:      "test",
				annotations.SandboxNamespace: "test",
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := client.StartVM(context.Background(), &pb.StartVMRequest{Id: id}); err != nil {
		t.Fatal(err)
	}

	forwarderSocket := filepath.Join(dir, "pods", id, proxy.SocketName)
	conn, err := net.Dial("unix", forwarderSocket)
	if err != nil {
		t.Fatal(err)
	}

	ttrpcClient := ttrpc.NewClient(conn)
	defer ttrpcClient.Close()

	agentClient := agent.NewAgentServiceClient(ttrpcClient)

	if _, err := agentClient.GetGuestDetails(ctx, &agent.GuestDetailsRequest{}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.StopVM(context.Background(), &pb.StopVMRequest{Id: id}); err != nil {
		t.Fatal(err)
	}
}

func testServerStart(t *testing.T, ctx context.Context) (Server, string, string, pb.HypervisorService, chan error) {

	dir := t.TempDir()

	socketPath := filepath.Join(dir, "hypervisor.sock")
	s := newServer(t, socketPath, filepath.Join(dir, "pods"))

	serverErrCh := make(chan error)
	go func() {
		defer close(serverErrCh)
		if err := s.Start(ctx); err != nil {
			serverErrCh <- err
		}
	}()

	<-s.Ready()

	select {
	case err := <-serverErrCh:
		t.Fatal(err)
	default:
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	client := pb.NewHypervisorClient(ttrpc.NewClient(conn))
	return s, dir, socketPath, client, serverErrCh
}

func startAgentServer(t *testing.T) string {

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Expect no error, got %q", err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	ttrpcServer, err := ttrpc.NewServer()
	if err != nil {
		t.Fatal(err)
	}

	agent.RegisterAgentServiceService(ttrpcServer, newAgentService())
	agent.RegisterHealthService(ttrpcServer, &healthService{})

	ctx := context.Background()

	go func() {
		if err := ttrpcServer.Serve(ctx, listener); err != nil {
			if !errors.Is(err, ttrpc.ErrServerClosed) {
				t.Error(err)
			}
		}
	}()
	t.Cleanup(func() {
		if err := ttrpcServer.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})

	return port
}

func newServer(t *testing.T, socketPath, podsDir string) Server {

	port := startAgentServer(t)
	provider := &mockProvider{}
	serverConfig := &ServerConfig{
		SocketPath:    socketPath,
		PodsDir:       podsDir,
		ForwarderPort: port,
		ProxyTimeout:  5 * time.Second,
	}
	return NewServer(provider, serverConfig, &mockWorkerNode{})
}

func testServerShutdown(t *testing.T, s Server, socketPath, dir string, serverErrCh chan error) {

	if err := s.Shutdown(); err != nil {
		t.Error(err)
	}
	if err := <-serverErrCh; err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(socketPath); err == nil {
		t.Errorf("Unix domain socket %s still remains\n", socketPath)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Error(err)
	}
}

type mockWorkerNode struct{}

func (n *mockWorkerNode) Inspect(nsPath string) (*tunneler.Config, error) {
	return &tunneler.Config{}, nil
}

func (n *mockWorkerNode) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {
	return nil
}

func (n *mockWorkerNode) Teardown(nsPath string, config *tunneler.Config) error {
	return nil
}

type mockProvider struct {
	primaryIP   string
	secondaryIP string
}

func (p *mockProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*cloud.Instance, error) {

	primaryIP := p.primaryIP
	if primaryIP == "" {
		primaryIP = "127.0.0.1"
	}

	secondaryIP := p.secondaryIP
	if secondaryIP == "" {
		secondaryIP = "127.0.0.1"
	}

	ips := make([]net.IP, 2)
	ips[0] = net.ParseIP(primaryIP)
	ips[1] = net.ParseIP(secondaryIP)

	if ips[0] == nil || ips[1] == nil {
		return nil, fmt.Errorf("Could not parse IPs: primary IP %s and secondary IP %s", primaryIP, secondaryIP)
	}

	instance := &cloud.Instance{
		ID:   "mock",
		Name: "mock",
		IPs:  ips,
	}

	return instance, nil
}

func (p *mockProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (p *mockProvider) Teardown() error {
	return nil
}
