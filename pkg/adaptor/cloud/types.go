// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"net"
	"sync"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

type Provider interface {
	CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec InstanceTypeSpec) (instance *Instance, err error)
	DeleteInstance(ctx context.Context, instanceID string) error
	Teardown() error
}

type Instance struct {
	ID   string
	Name string
	IPs  []net.IP
}

type Service interface {
	pb.HypervisorService
	GetInstanceID(ctx context.Context, podNamespace, podName string, wait bool) (string, error)
	Teardown() error
}

type cloudService struct {
	provider     Provider
	proxyFactory proxy.Factory
	workerNode   podnetwork.WorkerNode
	sandboxes    map[sandboxID]*sandbox
	cond         *sync.Cond
	podsDir      string
	daemonPort   string
	mutex        sync.Mutex
	ppService    *k8sops.PeerPodService
}

type InstanceTypeSpec struct {
	InstanceType string
	VCPUs        int64
	Memory       int64
	Arch         string
	GPUs         int64
}

type sandboxID string

type sandbox struct {
	agentProxy   proxy.AgentProxy
	podNetwork   *tunneler.Config
	cloudConfig  *cloudinit.CloudConfig
	id           sandboxID
	podName      string
	podNamespace string
	instanceName string
	instanceID   string
	netNSPath    string
	spec         InstanceTypeSpec
}
