// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"testing"

	cri "github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
	"github.com/stretchr/testify/assert"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

type mockProvider struct{}

func (p *mockProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*Instance, error) {
	return &Instance{
		Name: "abc",
		ID:   fmt.Sprintf("%s-%.8s", podName, sandboxID),
		IPs: []net.IP{
			net.ParseIP("192.0.2.1"),
		},
	}, nil
}

func (p *mockProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (p *mockProvider) Teardown() error {
	return nil
}

type mockProxy struct {
	socketPath string
	readyCh    chan struct{}
	stopCh     chan struct{}
}

func (p *mockProxy) Start(ctx context.Context, serverURL *url.URL) error {
	close(p.readyCh)
	<-p.stopCh
	return nil
}

func (p *mockProxy) Ready() chan struct{} {
	return p.readyCh
}

func (p *mockProxy) Shutdown() error {
	close(p.stopCh)
	return nil
}

type mockProxyFactory struct {
	podsDir string
}

func (f *mockProxyFactory) New(sandboxID string) proxy.AgentProxy {
	return &mockProxy{
		socketPath: filepath.Join(f.podsDir, sandboxID, proxy.SocketName),
		readyCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
	}
}

type mockWorkerNode struct{}

func (n mockWorkerNode) Inspect(nsPath string) (*tunneler.Config, error) {
	return nil, nil
}

func (n *mockWorkerNode) Setup(nsPath string, podNodeIPs []net.IP, config *tunneler.Config) error {
	return nil
}

func (n *mockWorkerNode) Teardown(nsPath string, config *tunneler.Config) error {
	return nil
}

func TestCloudService(t *testing.T) {

	ctx := context.Background()
	dir := t.TempDir()

	proxyFactory := &mockProxyFactory{
		podsDir: dir,
	}

	s := NewService(&mockProvider{}, proxyFactory, &mockWorkerNode{}, dir, forwarder.DefaultListenPort)

	assert.NotNil(t, s)

	sandboxID := "123"
	sandboxNS := "default"
	sandboxName := "mypod"

	req := &pb.CreateVMRequest{
		Id: sandboxID,
		Annotations: map[string]string{
			cri.SandboxNamespace: sandboxNS,
			cri.SandboxName:      sandboxName,
		},
	}

	res1, err := s.CreateVM(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, res1)
	assert.Contains(t, res1.AgentSocketPath, dir)

	res2, err := s.StartVM(ctx, &pb.StartVMRequest{Id: sandboxID})

	assert.NoError(t, err)
	assert.NotNil(t, res2)

	res3, err := s.StopVM(ctx, &pb.StopVMRequest{Id: sandboxID})

	assert.NoError(t, err)
	assert.NotNil(t, res3)
}
