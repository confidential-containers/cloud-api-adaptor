// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/forwarder"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/http/upgrader"
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

	forwarderSocket := filepath.Join(dir, "pods", id, forwarder.SocketName)
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
	dir, err := ioutil.TempDir("", "helper")
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(dir, "hypervisor.sock")
	s := newServer(t, socketPath, filepath.Join(dir, "pods"))

	serverErrCh := make(chan error)
	go func() {
		defer close(serverErrCh)
		if err := s.Start(ctx); err != nil {
			serverErrCh <- err
		}
	}()
	time.Sleep(1 * time.Millisecond)
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
	handler := upgrader.NewHandler()

	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	serverURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle(daemon.AgentURLPath, handler)
	_, port, err := net.SplitHostPort(serverURL.Host)
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
		if ttrpcServer.Serve(ctx, handler); err != nil {
			if err != nil {
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
	switch strings.ToLower(os.Getenv("USE_IBM_CLOUD")) {
	case "", "no", "false", "0":
		port := startAgentServer(t)
		return NewServer(socketPath, &mockVpcV1{}, &ServiceConfig{}, &mockWorkerNode{}, podsDir, port)
	}
	log.Print("Using IBM Cloud...")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		t.Fatal("Specify the API key as API_KEY")
	}
	vpcv1, err := NewVpcV1(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	keyId := os.Getenv("KEY_ID")
	if keyId == "" {
		t.Fatal("Specify the SSH key ID as KEY_ID")
	}
	serviceConfig := &ServiceConfig{
		ProfileName:              "bx2-2x8",
		ZoneName:                 "us-south-2",
		ImageID:                  "r134-d2090805-5652-4845-b287-46232e1098c3",
		PrimarySubnetID:          "0726-698b0a57-02db-49ee-a965-fa5f4d802fce",
		PrimarySecurityGroupID:   "r134-bace3bd1-6936-4126-ba6f-ff6d38775e9f",
		SecondarySubnetID:        "0726-bcb377d2-fccf-48c3-acd1-056d229ceb76",
		SecondarySecurityGroupID: "r134-2d59206c-ac4d-4488-b9a4-a086bef59ee5",
		KeyID:                    keyId,
		VpcID:                    "r134-c199bf26-ec6d-4c5d-a0a2-1e74d312891f",
	}

	return NewServer(socketPath, vpcv1, serviceConfig, &mockWorkerNode{}, podsDir, daemon.DefaultListenPort)
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

type mockVpcV1 struct {
	primaryIP   string
	secondaryIP string
}

func (v *mockVpcV1) GetInstance(getInstanceOptions *vpcv1.GetInstanceOptions) (result *vpcv1.Instance, response *core.DetailedResponse, err error) {
	return &vpcv1.Instance{}, &core.DetailedResponse{}, nil
}

func (v *mockVpcV1) CreateInstance(createInstanceOptions *vpcv1.CreateInstanceOptions) (result *vpcv1.Instance, response *core.DetailedResponse, err error) {

	primaryIP := v.primaryIP
	if primaryIP == "" {
		primaryIP = "127.0.0.1"
	}

	secondaryIP := v.secondaryIP
	if secondaryIP == "" {
		secondaryIP = "127.0.0.1"
	}

	strptr := func(s string) *string { return &s }
	return &vpcv1.Instance{
		ID:    strptr("mock"),
		Name:  strptr("mock"),
		Image: &vpcv1.ImageReference{Name: strptr("mock")},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfaceInstanceContextReference{
			ID:                 strptr("mockNIC1"),
			Name:               strptr("mockNIC1"),
			PrimaryIpv4Address: strptr(primaryIP),
		},
		NetworkInterfaces: []vpcv1.NetworkInterfaceInstanceContextReference{
			{
				ID:                 strptr("mockNIC1"),
				Name:               strptr("mockNIC1"),
				PrimaryIpv4Address: strptr(primaryIP),
			},
			{
				ID:                 strptr("mockNIC2"),
				Name:               strptr("mockNIC2"),
				PrimaryIpv4Address: strptr(secondaryIP),
			},
		},
	}, &core.DetailedResponse{}, nil
}

func (v *mockVpcV1) DeleteInstance(deleteInstanceOptions *vpcv1.DeleteInstanceOptions) (response *core.DetailedResponse, err error) {
	return &core.DetailedResponse{}, nil
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
