// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	cri "github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
	"github.com/stretchr/testify/assert"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/tlsutil"
)

type mockProvider struct{}

func (p *mockProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec InstanceTypeSpec) (*Instance, error) {
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

func (p *mockProvider) SelectInstanceType(ctx context.Context, vCPU int64, memory int64) (instanceType string, err error) {
	return "", nil
}

type mockProxy struct {
	readyCh    chan struct{}
	stopCh     chan struct{}
	socketPath string
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

func (p *mockProxy) ClientCA() (certPEM []byte) {
	return nil
}

func (p *mockProxy) CAService() tlsutil.CAService {
	return nil
}

type mockProxyFactory struct {
	podsDir string
}

func (f *mockProxyFactory) New(serverName, socketPath string) proxy.AgentProxy {
	return &mockProxy{
		socketPath: socketPath,
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

func TestVerifyCloudInstanceType(t *testing.T) {
	type args struct {
		instanceType        string
		instanceTypes       []string
		defaultInstanceType string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// Add test case with instanceType="t2.small", instanceTypes=["t2.small, t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[t2.small, t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{"t2.small", "t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.small",
			wantErr: false,
		},
		// Add test case with instanceType="t2.small", instanceTypes=["t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{"t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "",
			wantErr: true,
		},
		// Add test case with instanceType="", instanceTypes=["t2.medium"], defaultInstanceType="t2.medium"
		{
			name: "instanceType=, instanceTypes=[t2.medium], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "",
				instanceTypes:       []string{"t2.medium"},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with instanceType="", instanceTypes=[], defaultInstanceType="t2.medium"
		{
			name: "instanceType=, instanceTypes=[], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "",
				instanceTypes:       []string{},
				defaultInstanceType: "t2.medium",
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with instanceType="t2.small", instanceTypes=[], defaultInstanceType="t2.medium"
		{
			name: "instanceType=t2.small, instanceTypes=[], defaultInstanceType=t2.medium",
			args: args{
				instanceType:        "t2.small",
				instanceTypes:       []string{},
				defaultInstanceType: "t2.medium",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyCloudInstanceType(tt.args.instanceType, tt.args.instanceTypes, tt.args.defaultInstanceType)

			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyCloudInstanceType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VerifyCloudInstanceType() = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestSortInstanceTypesOnMemory(t *testing.T) {
	type args struct {
		instanceTypeSpecList []InstanceTypeSpec
	}
	tests := []struct {
		name string
		args args
		want []InstanceTypeSpec
	}{

		// Add test case with instanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}]
		{
			name: "instanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},
		// Add test case with instanceTypeSpecList=[{t2.small, 2, 6}, {t2.large, 8, 16}, {t2.medium, 4, 8}]
		{
			name: "instanceTypeSpecList=[{t2.small, 2, 6}, {t2.large, 8, 16}, {t2.medium, 4, 8}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},
		// Add test case with instanceTypeSpecList=[{t2.medium, 4, 8}, {t2.small, 2, 6}, {t2.large, 8, 16}]
		{
			name: "instanceTypeSpecList=[{t2.medium, 4, 8}, {t2.small, 2, 6}, {t2.large, 8, 16}]",
			args: args{
				instanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
			},
			want: []InstanceTypeSpec{
				{
					InstanceType: "t2.small",
					VCPUs:        2,
					Memory:       6,
				},
				{
					InstanceType: "t2.medium",
					VCPUs:        4,
					Memory:       8,
				},
				{
					InstanceType: "t2.large",
					VCPUs:        8,
					Memory:       16,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add benchmark
			start := time.Now()
			if got := SortInstanceTypesOnMemory(tt.args.instanceTypeSpecList); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortInstanceTypesOnMemory() = %v, want %v", got, tt.want)
			}
			elapsed := time.Since(start)
			fmt.Printf("SortInstanceTypesOnMemory() took %s\n", elapsed)
		})
	}
}

func TestGetBestFitInstanceType(t *testing.T) {
	type args struct {
		sortedInstanceTypeSpecList []InstanceTypeSpec
		vcpus                      int64
		memory                     int64
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=2, memory=6
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=2, memory=6",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  2,
				memory: 6,
			},
			want:    "t2.small",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=8
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=8",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 8,
			},
			want:    "t2.medium",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=16
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=16",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 16,
			},
			want:    "t2.large",
			wantErr: false,
		},
		// Add test case with sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=32
		{
			name: "sortedInstanceTypeSpecList=[{t2.small, 2, 6}, {t2.medium, 4, 8}, {t2.large, 8, 16}], vcpus=4, memory=32",
			args: args{
				sortedInstanceTypeSpecList: []InstanceTypeSpec{
					{
						InstanceType: "t2.small",
						VCPUs:        2,
						Memory:       6,
					},
					{
						InstanceType: "t2.medium",
						VCPUs:        4,
						Memory:       8,
					},
					{
						InstanceType: "t2.large",
						VCPUs:        8,
						Memory:       16,
					},
				},
				vcpus:  4,
				memory: 32,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add benchmark
			start := time.Now()
			got, err := GetBestFitInstanceType(tt.args.sortedInstanceTypeSpecList, tt.args.vcpus, tt.args.memory)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBestFitInstanceType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			elapsed := time.Since(start)
			fmt.Printf("GetBestFitInstanceType() took %s\n", elapsed)
			if got != tt.want {
				t.Errorf("GetBestFitInstanceType() = %v, want %v", got, tt.want)
			}
		})
	}
}
